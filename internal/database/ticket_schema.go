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
	"strings"
)

func initTicketTable(db *sql.DB, mysql bool) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS ticket_state (
		task_id CHAR(36) PRIMARY KEY,
		status VARCHAR(16) NOT NULL DEFAULT 'unprocessed',
		title VARCHAR(160) NOT NULL DEFAULT '', body TEXT NOT NULL,
		assignee_id VARCHAR(36) NOT NULL DEFAULT '', assignee_admin INT NOT NULL DEFAULT 0,
		admin_only INT NOT NULL DEFAULT 0, escalations INT NOT NULL DEFAULT 0,
		escalated_by_id VARCHAR(36) NOT NULL DEFAULT '', revision BIGINT NOT NULL DEFAULT 0,
		target_user_ids TEXT NOT NULL, outcome VARCHAR(16) NOT NULL DEFAULT '',
		response TEXT NOT NULL, changed_at BIGINT NOT NULL DEFAULT 0
	);`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS ticket_messages (
		id VARCHAR(64) PRIMARY KEY,
		task_id VARCHAR(64) NOT NULL,
		author_id VARCHAR(64) NOT NULL,
		author_name VARCHAR(255) NOT NULL,
		author_role VARCHAR(32) NOT NULL DEFAULT '',
		kind VARCHAR(32) NOT NULL DEFAULT 'comment',
		body TEXT NOT NULL,
		created_at BIGINT NOT NULL
	);`)
	if err != nil {
		return err
	}
	for _, index := range []string{
		"CREATE INDEX idx_ticket_messages_page ON ticket_messages (task_id, created_at, id)",
		"CREATE INDEX idx_ticket_messages_author ON ticket_messages (author_id, created_at)",
	} {
		if !mysql {
			index = strings.Replace(index, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ", 1)
		}
		if _, err := db.Exec(index); err != nil {
			if mysql && strings.Contains(strings.ToLower(err.Error()), "duplicate key name") {
				continue
			}
			return err
		}
	}
	return nil
}
