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

import "database/sql"

func initNativeResourceTables(db *sql.DB) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS native_resources (
			id CHAR(64) PRIMARY KEY, repository VARCHAR(64) NOT NULL,
			format VARCHAR(16) NOT NULL, name VARCHAR(255) NOT NULL,
			description TEXT NOT NULL, signing_key TEXT NOT NULL, created_at BIGINT NOT NULL, published_at BIGINT NOT NULL DEFAULT 0,
			archived INT NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS native_members (
			resource_id CHAR(64) NOT NULL, user_id VARCHAR(36) NOT NULL,
			permission_level INT NOT NULL, added_at BIGINT NOT NULL,
			PRIMARY KEY (resource_id, user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS native_artifacts (
			id CHAR(64) PRIMARY KEY, resource_id CHAR(64) NOT NULL, repository VARCHAR(64) NOT NULL,
			path VARCHAR(1024) NOT NULL, version VARCHAR(255) NOT NULL, size BIGINT NOT NULL,
			created_at BIGINT NOT NULL, published INT NOT NULL DEFAULT 0
		);`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}
