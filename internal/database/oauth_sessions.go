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
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"time"

	"renop/internal/core"
	"renop/pkg/hex"
)

const maxOAuthRevocations = 8192

func initOAuthRevocationTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS oauth_revocations (
		event_hash VARCHAR(64) PRIMARY KEY, provider_id VARCHAR(32) NOT NULL, authority VARCHAR(64) NOT NULL,
		subject VARCHAR(255) NOT NULL, session_id VARCHAR(255) NOT NULL, issued_at BIGINT NOT NULL, expires_at BIGINT NOT NULL)`)
	return err
}

// Provider subjects and session IDs are opaque and case-sensitive, including on case-insensitive SQL collations.
func oauthScopeHash(value string) string {
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// Lock ordering is revocation gate, then account, then session. Both issuance and callbacks use it.
func lockOAuthRevocationsTx(tx *Tx) error {
	query := `INSERT INTO oauth_revocations (event_hash, provider_id, authority, subject, session_id, issued_at, expires_at) VALUES ('lock', '', '', '', '', 0, 9223372036854775807)`
	switch tx.db.Dialect.Name() {
	case "mysql":
		query += ` ON DUPLICATE KEY UPDATE issued_at = issued_at`
	case "clickhouse":
		query = `/* renop:upsert */ ` + query
	default:
		query += ` ON CONFLICT (event_hash) DO UPDATE SET issued_at = 0`
	}
	_, err := tx.Exec(query)
	return err
}

func (db *DB) pruneOAuthRevocations(now int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockOAuthRevocationsTx(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM oauth_revocations WHERE expires_at <= ?`, now); err != nil {
		return err
	}
	return tx.Commit()
}

func validGrantScope(provider, authority, subject, sid string) bool {
	return provider != "" && len(provider) <= 32 && len(authority) == 64 && len(subject) <= 255 && len(sid) <= 255 &&
		(subject != "" || sid != "") && !strings.ContainsAny(provider+authority+subject+sid, "\x00\r\n")
}

func saveSessionOAuthGrantTx(tx *Tx, sessionToken string, grant *core.SessionOAuthGrant, now int64) error {
	if !validGrantScope(grant.ProviderID, grant.Authority, grant.Subject, grant.SessionID) || grant.Subject == "" ||
		grant.AuthorizedAt <= 0 || grant.AuthorizedAt > now+30000 || len(grant.EncryptedTokens) > 60000 || grant.EncryptedTokens == "" {
		return core.ErrMFAInvalid
	}
	// Persisted events also deny an OAuth callback or MFA proof that completes after provider logout.
	var revoked int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM oauth_revocations WHERE provider_id = ? AND authority = ?
		AND (subject = '' OR subject = ?) AND (session_id = '' OR session_id = ?) AND issued_at >= ? AND expires_at > ?`,
		grant.ProviderID, grant.Authority, oauthScopeHash(grant.Subject), oauthScopeHash(grant.SessionID), grant.AuthorizedAt, now).Scan(&revoked); err != nil {
		return err
	}
	if revoked != 0 {
		return core.ErrMFAInvalid
	}
	_, err := tx.Exec(`UPDATE sessions SET oauth_provider = ?, oauth_authority = ?, oauth_subject = ?, oauth_sid = ?, oauth_authorized_at = ?, oauth_grant = ?, oauth_subject_hash = ?, oauth_sid_hash = ? WHERE session_token = ?`,
		grant.ProviderID, grant.Authority, grant.Subject, grant.SessionID, grant.AuthorizedAt, grant.EncryptedTokens, oauthScopeHash(grant.Subject), oauthScopeHash(grant.SessionID), sessionToken)
	return err
}

// GetSessionOAuthGrant reads private authorization material only for a known session secret.
func (db *DB) GetSessionOAuthGrant(sessionToken string) (*core.SessionOAuthGrant, error) {
	var grant core.SessionOAuthGrant
	err := db.QueryRow(`SELECT oauth_provider, oauth_authority, oauth_subject, oauth_sid, oauth_authorized_at, COALESCE(oauth_grant, '')
		FROM sessions WHERE session_token = ? AND oauth_provider <> ''`, sessionToken).
		Scan(&grant.ProviderID, &grant.Authority, &grant.Subject, &grant.SessionID, &grant.AuthorizedAt, &grant.EncryptedTokens)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &grant, err
}

// RevokeOAuthSessions atomically records a verified event and removes only its older matching sessions.
func (db *DB) RevokeOAuthSessions(event core.OAuthRevocation, now int64) ([]string, error) {
	if !validGrantScope(event.ProviderID, event.Authority, event.Subject, event.SessionID) || event.EventID == "" ||
		len(event.EventID) > 255 || event.IssuedAt <= 0 || event.IssuedAt > now+30000 || event.IssuedAt < now-10*60*1000 {
		return nil, core.ErrMFAInvalid
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := lockOAuthRevocationsTx(tx); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM oauth_revocations WHERE expires_at <= ?`, now); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(event.ProviderID + "\x00" + event.Authority + "\x00" + event.EventID))
	eventHash := hex.EncodeToString(digest[:])
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM oauth_revocations WHERE event_hash = ?`, eventHash).Scan(&count); err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, nil
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM oauth_revocations`).Scan(&count); err != nil {
		return nil, err
	}
	if count >= maxOAuthRevocations {
		return nil, core.ErrDatabaseUnavailable
	}
	if _, err := tx.Exec(`INSERT INTO oauth_revocations (event_hash, provider_id, authority, subject, session_id, issued_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		eventHash, event.ProviderID, event.Authority, oauthScopeHash(event.Subject), oauthScopeHash(event.SessionID), event.IssuedAt, now+int64(30*time.Minute/time.Millisecond)); err != nil {
		if uniqueConstraintError(err) {
			return nil, nil
		}
		return nil, err
	}
	where := `oauth_provider = ? AND oauth_authority = ? AND oauth_authorized_at <= ?`
	args := []any{event.ProviderID, event.Authority, event.IssuedAt}
	if event.Subject != "" {
		where += ` AND oauth_subject_hash = ?`
		args = append(args, oauthScopeHash(event.Subject))
	}
	if event.SessionID != "" {
		where += ` AND oauth_sid_hash = ?`
		args = append(args, oauthScopeHash(event.SessionID))
	}
	rows, err := tx.Query(`SELECT session_token FROM sessions WHERE `+where+` LIMIT 10001`, args...)
	if err != nil {
		return nil, err
	}
	tokens := make([]string, 0)
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			rows.Close()
			return nil, err
		}
		tokens = append(tokens, token)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(tokens) > 10000 {
		return nil, core.ErrDatabaseUnavailable
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE oauth_provider = ? AND oauth_authority = ? AND oauth_authorized_at <= ?
		AND (? = '' OR oauth_subject_hash = ?) AND (? = '' OR oauth_sid_hash = ?)`, event.ProviderID, event.Authority, event.IssuedAt,
		oauthScopeHash(event.Subject), oauthScopeHash(event.Subject), oauthScopeHash(event.SessionID), oauthScopeHash(event.SessionID)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for _, token := range tokens {
		db.sessionCache.Delete(token)
	}
	return tokens, nil
}
