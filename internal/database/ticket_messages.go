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
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"renop/internal/core"
)

// AddTicketMessage commits a bounded comment with live session and ticket authority.
func (db *DB) AddTicketMessage(msg *core.TicketMessage, actor, session string) error {
	if db == nil || db.SQLDB == nil {
		return core.ErrDatabaseUnavailable
	}
	if msg == nil || msg.TaskID == "" {
		return core.ErrReviewInvalidRequest
	}
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	body, valid := normalizeTicketText(msg.Body, 16384)
	if !valid {
		return core.ErrReviewInvalidRequest
	}
	msg.Body = body
	msg.Kind = "comment"
	if msg.CreatedAt <= 0 {
		msg.CreatedAt = time.Now().UnixMilli()
	}
	reviewTaskMutationLock.Lock()
	defer reviewTaskMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	actor = strings.ToLower(strings.TrimSpace(actor))
	account, err := accountEmailSessionTx(tx, actor, session)
	if err != nil {
		return core.ErrReviewPermissionDenied
	}
	if _, err := tx.Exec(`UPDATE review_tasks SET created_at = created_at WHERE id = ?`, msg.TaskID); err != nil {
		return err
	}
	task, err := loadReviewTaskTx(tx, msg.TaskID)
	if err != nil {
		return err
	}
	if task.Kind == core.TicketKindReport && strings.Contains(task.TargetUserIDs, `"`+account.UserID+`"`) {
		return core.ErrReviewPermissionDenied
	}
	msg.AuthorID, msg.AuthorName, msg.AuthorRole = account.UserID, actor, "author"
	if task.RequestedByID != account.UserID {
		user, err := ticketHandlerTx(tx, task, account.UserID)
		if err != nil {
			return err
		}
		msg.AuthorRole = "team_admin"
		if user.IsManager() {
			msg.AuthorRole = "admin"
		} else if user.CheckModeratePermission(task.Repository) {
			msg.AuthorRole = "moderator"
		}
	}
	if task.Status != core.ReviewStatusPending {
		return core.ErrReviewTaskConflict
	}
	var total, recent, burst int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM ticket_messages WHERE task_id = ? AND kind = 'comment'`, msg.TaskID).Scan(&total); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN created_at > ? THEN 1 ELSE 0 END), 0)
		FROM ticket_messages WHERE author_id = ? AND kind = 'comment' AND created_at > ?`,
		msg.CreatedAt-time.Minute.Milliseconds(), account.UserID, msg.CreatedAt-time.Hour.Milliseconds()).Scan(&recent, &burst); err != nil {
		return err
	}
	if total >= 1000 || recent >= 200 || burst >= 20 {
		return core.ErrReviewFileLimit
	}
	_, err = tx.Exec(`INSERT INTO ticket_messages (id, task_id, author_id, author_name, author_role, kind, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.TaskID, msg.AuthorID, msg.AuthorName, msg.AuthorRole, msg.Kind, msg.Body, msg.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ListTicketMessages returns a bounded chronological page, starting with the newest messages.
// The next cursor selects older messages without skipping equal timestamps or concurrent inserts.
func (db *DB) ListTicketMessages(taskID, before string, limit int) ([]*core.TicketMessage, string, error) {
	if db == nil || db.SQLDB == nil {
		return nil, "", core.ErrDatabaseUnavailable
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || len(before) > 64 || limit < 1 || limit > 100 {
		return nil, "", core.ErrReviewInvalidRequest
	}
	where, args := "task_id = ?", []any{taskID}
	if before != "" {
		var createdAt int64
		if err := db.QueryRow(`SELECT created_at FROM ticket_messages WHERE task_id = ? AND id = ?`, taskID, before).Scan(&createdAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = core.ErrReviewInvalidRequest
			}
			return nil, "", err
		}
		where += " AND (created_at < ? OR (created_at = ? AND id < ?))"
		args = append(args, createdAt, createdAt, before)
	}
	args = append(args, limit+1)
	rows, err := db.Query(`SELECT id, task_id, author_id, author_name, author_role, kind, body, created_at
		FROM ticket_messages WHERE `+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	messages := make([]*core.TicketMessage, 0)
	for rows.Next() {
		m := &core.TicketMessage{}
		if err := rows.Scan(&m.ID, &m.TaskID, &m.AuthorID, &m.AuthorName, &m.AuthorRole, &m.Kind, &m.Body, &m.CreatedAt); err != nil {
			return nil, "", err
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(messages) > limit {
		messages = messages[:limit]
		next = messages[len(messages)-1].ID
	}
	slices.Reverse(messages)
	return messages, next, nil
}
