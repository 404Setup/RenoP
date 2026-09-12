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

	"renop/internal/config"
	"renop/internal/core"
)

func (db *DB) liveTicketUser(userID string) (*config.User, error) {
	// Presentation needs one live authorization snapshot, without a write transaction.
	// Ticket mutations still use ticketActorTx under the account lock.
	var username, permissions string
	now := time.Now().UnixMilli()
	err := db.QueryRow(`SELECT profile.username, token.permissions_json
		FROM user_profiles profile JOIN tokens token ON token.name = profile.username
		WHERE profile.user_id = ? AND token.deleted_at = 0
		AND (token.expires_at IS NULL OR token.expires_at > ?)
		AND (token.banned_at = 0 OR (token.banned_until > 0 AND token.banned_until <= ?))`, userID, now, now).
		Scan(&username, &permissions)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrReviewPermissionDenied
	}
	if err != nil {
		return nil, err
	}
	return reviewUserFromPermissions(username, permissions)
}

func (db *DB) presentTickets(tasks []*core.ReviewTask, userID string, requestedView bool) error {
	user, err := db.liveTicketUser(userID)
	if err != nil {
		return err
	}
	roles := make(map[string]int)
	if !user.IsManager() && !requestedView {
		rows, err := db.Query(`SELECT team_prefix, role_level FROM super_team_members WHERE user_id = ? AND role_level >= ? LIMIT 1001`,
			userID, core.SuperTeamRoleManage)
		if err != nil {
			return err
		}
		for rows.Next() {
			var prefix string
			var role int
			if err := rows.Scan(&prefix, &role); err != nil {
				rows.Close()
				return err
			}
			roles[prefix] = role
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	for _, task := range tasks {
		if task.Kind == core.TicketKindReport && strings.Contains(task.TargetUserIDs, `"`+userID+`"`) {
			return core.ErrReviewPermissionDenied
		}
		requester := task.RequestedByID == userID
		teamAdmin := (task.ReviewTeamPrefix != "" && roles[task.ReviewTeamPrefix] >= core.SuperTeamRoleManage) ||
			(task.Kind != core.ReviewKindPublication && ((task.TargetTeamPrefix != "" && roles[task.TargetTeamPrefix] >= core.SuperTeamRoleManage) ||
				(task.SourceTeamPrefix != "" && roles[task.SourceTeamPrefix] >= core.SuperTeamRoleManage)))
		repoMod := (task.Repository != "" && user.CheckModeratePermission(task.Repository)) || user.CheckModeratePermission("")
		staff := !requestedView && (user.IsManager() || repoMod || teamAdmin)
		if !requester && !staff {
			return core.ErrReviewPermissionDenied
		}
		task.Actions = nil
		if task.Status == core.ReviewStatusPending {
			if requester && task.Kind != core.ReviewKindPublication {
				task.Actions = append(task.Actions, "close")
			}
			handler := staff && (!task.AdminOnly || user.IsManager()) &&
				(user.IsManager() ||
					(task.Kind == core.ReviewKindPublication && task.ReviewTeamPrefix != "" && roles[task.ReviewTeamPrefix] >= core.SuperTeamRoleManage) ||
					(task.Kind == core.ReviewKindPublication && task.ReviewTeamPrefix == "" && repoMod) ||
					(task.Kind != core.ReviewKindPublication && (repoMod || teamAdmin)))
			if handler && (!requester || !supportTicket(task)) {
				switch {
				case task.AssigneeID == userID:
					if supportTicket(task) {
						task.Actions = append(task.Actions, "process", "complete", "close")
					} else {
						task.Actions = append(task.Actions, "decision")
					}
					if task.Escalations < 3 {
						task.Actions = append(task.Actions, "release", "escalate")
					}
				case task.AssigneeID == "" && (!task.AdminOnly || task.EscalatedByID != userID):
					task.Actions = append(task.Actions, "claim", "close")
				case task.AssigneeID != "" && user.IsManager() && !task.AssigneeAdmin && task.Escalations < 3:
					task.Actions = append(task.Actions, "force_claim", "close")
				}
			}
		}
		if requester || !staff {
			task.DecidedByID, task.DecidedBy, task.AssigneeID, task.Assignee, task.EscalatedByID, task.EscalatedBy = "", "", "", "", "", ""
		}
	}
	return nil
}

// GetTicket returns a requester or staff view with server-computed actions and identity redaction.
func (db *DB) GetTicket(id, actor string) (*core.ReviewTask, error) {
	if db == nil || db.SQLDB == nil {
		return nil, core.ErrDatabaseUnavailable
	}
	userID, err := db.userIDForExistingAccount(actor)
	if err != nil {
		return nil, core.ErrReviewPermissionDenied
	}
	task, err := db.GetReviewTask(id)
	if err != nil {
		return nil, err
	}
	if err := db.presentTickets([]*core.ReviewTask{task}, userID, false); err != nil {
		return nil, err
	}
	return task, nil
}
