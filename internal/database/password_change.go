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
	"strings"

	"renop/internal/core"
)

// ChangeAccountPassword verifies current policy and consumes a fresh proof with the password mutation.
// Passkey signatures are checked by the service; ownership is rechecked here under the account lock.
func (db *DB) ChangeAccountPassword(change core.PasswordChange) error {
	if change.PasswordHash == "" || len(change.PasswordHash) > 255 || change.Session == "" ||
		!validSelectorHash(change.Snapshot) || change.Now <= 0 {
		return core.ErrMFAInvalid
	}
	change.Username = strings.ToLower(change.Username)
	if change.Factor == "email" {
		db.mailWriteMu.Lock()
		defer db.mailWriteMu.Unlock()
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if change.Factor == "email" {
		if err := lockMailTx(tx); err != nil {
			return err
		}
	}
	current, err := accountEmailSessionTx(tx, change.Username, change.Session)
	if err != nil {
		return err
	}
	if current.Snapshot != change.Snapshot {
		return core.ErrMFAInvalid
	}
	switch change.Factor {
	case "":
		if current.Enabled() {
			return core.ErrMFARequired
		}
	case "totp":
		valid, err := consumeMFACodeTx(tx, current, current.Revision, change.TOTPStep, change.Now)
		if err != nil {
			return err
		}
		if !valid {
			if err := tx.Commit(); err != nil {
				return err
			}
			return core.ErrMFAInvalid
		}
	case "passkey":
		if !current.Passkey || len(change.CredentialID) == 0 {
			return core.ErrMFAInvalid
		}
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM fido_devices WHERE username = ? AND credential_id = ?`,
			change.Username, change.CredentialID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return core.ErrMFAInvalid
		}
	case "email":
		_, recordFailure, err := consumePasswordResetProofTx(tx, change.Email, change.EmailCodeHash, current.UserID, change.Now)
		if err != nil {
			if recordFailure {
				if commitErr := tx.Commit(); commitErr != nil {
					return commitErr
				}
			}
			return err
		}
	default:
		return core.ErrMFAInvalid
	}
	if err := resetAccountPasswordTx(tx, current.UserID, change.Username, change.PasswordHash, change.Now, change.Session); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	db.invalidateRecoveredAccount(change.Username)
	return nil
}
