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

import "renop/internal/core"

// SetUserProfilePrivacy commits an owner-only preference after rechecking the live browser session.
func (db *DB) SetUserProfilePrivacy(username, session, userID string, private bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	account, err := accountEmailSessionTx(tx, username, session)
	if err != nil {
		return err
	}
	if account.UserID != userID {
		return core.ErrUserProfileNotFound
	}
	if _, err = tx.Exec(`UPDATE user_profiles SET is_private = ? WHERE user_id = ?`, boolInt(private), userID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	db.invalidateUserProfileCaches(username)
	return nil
}
