/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"renop/internal/core"
)

// rememberPrimaryEmailTx runs under the account lock before changing the primary address.
// Keeping every still-protected address prevents consecutive email changes from erasing a recovery window.
func rememberPrimaryEmailTx(tx *Tx, userID, next string, now int64) error {
	if now <= 0 {
		return core.ErrEmailCodeInvalid
	}
	var previous string
	err := tx.QueryRow(`SELECT COALESCE(email, '') FROM user_account_security WHERE user_id = ?`, userID).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if previous == "" || previous == next {
		return nil
	}
	if _, err := tx.Exec(`DELETE FROM user_primary_email_history WHERE user_id = ? AND (email = ? OR expires_at <= ?)`, userID, previous, now); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO user_primary_email_history (user_id, email, expires_at) VALUES (?, ?, ?)`,
		userID, previous, now+core.PrimaryEmailRecoveryPeriod.Milliseconds())
	return err
}

func securityHoldTx(tx *Tx, userID string, now int64) error {
	var until int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(expires_at), 0) FROM user_primary_email_history WHERE user_id = ?`, userID).Scan(&until); err != nil {
		return err
	}
	if until > now {
		return core.ErrSecurityHold
	}
	return nil
}

func securityHoldByNameTx(tx *Tx, username string, now int64) error {
	var userID string
	err := tx.QueryRow(`SELECT user_id FROM user_profiles WHERE username = ?`, username).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return securityHoldTx(tx, userID, now)
}

func (db *DB) loadPrimaryEmailProtection(userID string, security *core.AccountSecurity) error {
	rows, err := db.Query(`SELECT email, expires_at FROM user_primary_email_history
		WHERE user_id = ? AND expires_at > ? ORDER BY expires_at DESC, email LIMIT ?`,
		userID, time.Now().UnixMilli(), core.MaxAccountEmails)
	if err != nil {
		return err
	}
	defer rows.Close()
	security.PreviousPrimaryEmails = []core.PreviousPrimaryEmail{}
	for rows.Next() {
		var entry core.PreviousPrimaryEmail
		if err := rows.Scan(&entry.Email, &entry.ExpiresAt); err != nil {
			return err
		}
		if entry.Email != security.Email {
			security.PreviousPrimaryEmails = append(security.PreviousPrimaryEmails, entry)
		}
		security.SecurityHoldUntil = max(security.SecurityHoldUntil, entry.ExpiresAt)
	}
	return rows.Err()
}

// recoveryPrimaryEmailTx rechecks ownership and the deadline after locking the account.
func recoveryPrimaryEmailTx(tx *Tx, userID, identifier string, now int64) (restore bool, current string, err error) {
	email, valid := core.NormalizeEmail(identifier)
	if !valid || email == "" {
		return false, "", core.ErrRecoveryCodesInvalid
	}
	var owner string
	err = tx.QueryRow(`SELECT address.user_id, COALESCE(security.email, '') FROM user_email_addresses address
		JOIN user_account_security security ON security.user_id = address.user_id
		WHERE address.email = ?`, email).Scan(&owner, &current)
	if errors.Is(err, sql.ErrNoRows) || err == nil && owner != userID {
		return false, "", core.ErrRecoveryCodesInvalid
	}
	if err != nil {
		return false, "", err
	}
	if current == email {
		return false, current, nil
	}
	var until int64
	err = tx.QueryRow(`SELECT expires_at FROM user_primary_email_history WHERE user_id = ? AND email = ?`, userID, email).Scan(&until)
	if errors.Is(err, sql.ErrNoRows) || err == nil && until <= now {
		return false, "", core.ErrRecoveryCodesInvalid
	}
	return true, current, err
}

func restorePrimaryEmailTx(tx *Tx, userID, previous, replaced string) error {
	previous, _ = core.NormalizeEmail(previous)
	if _, err := tx.Exec(`UPDATE user_account_security SET email = ? WHERE user_id = ?`, previous, userID); err != nil {
		return err
	}
	if replaced != "" && !strings.EqualFold(previous, replaced) {
		if _, err := tx.Exec(`DELETE FROM user_email_addresses WHERE user_id = ? AND email = ?`, userID, replaced); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM user_email_addresses WHERE user_id = ? AND retained = 0 AND email <> ?`, userID, previous); err != nil {
		return err
	}
	// Recovery-code possession restores the trusted address without starting another hold.
	if _, err := tx.Exec(`DELETE FROM user_primary_email_history WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_email_changes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM user_password_resets WHERE user_id = ?`, userID)
	return err
}

// cleanupExpiredPrimaryEmails releases recovery-only reservations after their
// deadline. Retired accounts keep their independent retirement reservation.
func (db *DB) cleanupExpiredPrimaryEmails(now int64, limit int) error {
	rows, err := db.Query(`SELECT history.user_id FROM user_primary_email_history history
		JOIN user_profiles profile ON profile.user_id = history.user_id
		JOIN tokens token ON token.name = profile.username
		WHERE history.expires_at <= ? AND token.deleted_at = 0
		GROUP BY history.user_id ORDER BY history.user_id LIMIT ?`, now, limit)
	if err != nil {
		return err
	}
	var users []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			_ = rows.Close()
			return err
		}
		users = append(users, userID)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	for _, userID := range users {
		if err := db.releaseExpiredPrimaryEmails(userID, now); err != nil && !errors.Is(err, core.ErrAccountDeleted) {
			return err
		}
	}
	return nil
}

func (db *DB) releaseExpiredPrimaryEmails(userID string, now int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockAccountLoginMethodsTx(tx, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_email_addresses WHERE user_id = ? AND retained = 0
		AND email <> COALESCE((SELECT email FROM user_account_security WHERE user_id = ?), '')
		AND email IN (SELECT email FROM user_primary_email_history WHERE user_id = ? AND expires_at <= ?)`,
		userID, userID, userID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_primary_email_history WHERE user_id = ? AND expires_at <= ?`, userID, now); err != nil {
		return err
	}
	return tx.Commit()
}
