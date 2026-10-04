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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"renop/pkg/id"
)

// MigrateLegacyUUIDs scans all tables in the database for 36-character legacy UUID strings,
// converting them to 22-character compact IDs while preserving referential integrity.
// It returns the total count of values migrated.
func (db *DB) MigrateLegacyUUIDs(ctx context.Context) (int64, error) {
	if db == nil || (db.SQLDB == nil && db.clickHouse == nil) {
		return 0, nil
	}

	db.InvalidateCaches()
	defer db.InvalidateCaches()

	driver := ""
	if db.Dialect != nil {
		driver = strings.ToLower(db.Dialect.Name())
	}

	if db.SQLDB != nil {
		switch {
		case strings.Contains(driver, "sqlite"):
			_, _ = db.SQLDB.ExecContext(ctx, "PRAGMA foreign_keys = OFF;")
			defer func() {
				_, _ = db.SQLDB.ExecContext(context.Background(), "PRAGMA foreign_keys = ON;")
			}()
		case strings.Contains(driver, "mysql") || strings.Contains(driver, "mariadb"):
			_, _ = db.SQLDB.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0;")
			defer func() {
				_, _ = db.SQLDB.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS = 1;")
			}()
		case strings.Contains(driver, "postgres") || strings.Contains(driver, "pgx") || strings.Contains(driver, "postgresql"):
			_, _ = db.SQLDB.ExecContext(ctx, "SET session_replication_role = 'replica';")
			_, _ = db.SQLDB.ExecContext(ctx, "SET CONSTRAINTS ALL DEFERRED;")
			defer func() {
				_, _ = db.SQLDB.ExecContext(context.Background(), "SET session_replication_role = 'origin';")
			}()
		}
	}

	var totalMigrated int64

	tableExists := func(tableName string) bool {
		switch {
		case strings.Contains(driver, "sqlite"):
			var dummy int
			err := db.QueryRow("SELECT 1 FROM sqlite_master WHERE type='table' AND name = ?", tableName).Scan(&dummy)
			return err == nil
		case strings.Contains(driver, "mysql") || strings.Contains(driver, "mariadb"):
			var dummy int
			err := db.QueryRow("SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&dummy)
			return err == nil
		case strings.Contains(driver, "postgres") || strings.Contains(driver, "pgx") || strings.Contains(driver, "postgresql"):
			var dummy int
			err := db.QueryRow("SELECT 1 FROM information_schema.tables WHERE table_name = ? AND table_schema = CURRENT_SCHEMA()", tableName).Scan(&dummy)
			return err == nil
		case strings.Contains(driver, "clickhouse"):
			var dummy int
			err := db.QueryRow("SELECT 1 FROM system.tables WHERE database = currentDatabase() AND name = ?", tableName).Scan(&dummy)
			return err == nil
		default:
			var dummy int
			err := db.QueryRow("SELECT 1 FROM " + tableName + " LIMIT 1").Scan(&dummy)
			if err == nil || errors.Is(err, sql.ErrNoRows) {
				return true
			}
			errStr := strings.ToLower(err.Error())
			return !strings.Contains(errStr, "no such table") && !strings.Contains(errStr, "doesn't exist") && !strings.Contains(errStr, "does not exist") && !strings.Contains(errStr, "relation")
		}
	}

	execUpdate := func(query string, args ...any) bool {
		aff, err := db.execMigrationUpdate(query, args...)
		if err == nil && aff > 0 {
			totalMigrated++
			return true
		}
		return false
	}

	simpleTables := []string{
		"user_messages",
		"cargo_invitations",
		"docker_invitations",
		"npm_invitations",
		"maven_domain_invitations",
		"gpg_releases",
		"publication_quota_reservations",
		"fido_devices",
		"oauth_revocations",
	}

	for _, table := range simpleTables {
		if !tableExists(table) {
			continue
		}
		col := "id"
		if table == "oauth_revocations" {
			col = "event_id"
		}
		rows, err := db.Query("SELECT " + col + " FROM " + table)
		if err != nil {
			continue
		}
		type item struct{ oldID, newID string }
		var toUpdate []item
		for rows.Next() {
			var rawVal string
			if err := rows.Scan(&rawVal); err != nil {
				continue
			}
			norm, ok := id.Normalize(rawVal)
			if ok && norm != rawVal {
				toUpdate = append(toUpdate, item{oldID: rawVal, newID: norm})
			}
		}
		_ = rows.Close()

		for _, it := range toUpdate {
			execUpdate("UPDATE "+table+" SET "+col+" = ? WHERE "+col+" = ?", it.newID, it.oldID)
		}
	}

	if tableExists("user_profiles") {
		rows, err := db.Query("SELECT user_id FROM user_profiles")
		if err == nil {
			type item struct{ oldID, newID string }
			var toUpdate []item
			for rows.Next() {
				var rawVal string
				if err := rows.Scan(&rawVal); err != nil {
					continue
				}
				norm, ok := id.Normalize(rawVal)
				if ok && norm != rawVal {
					toUpdate = append(toUpdate, item{oldID: rawVal, newID: norm})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_profiles SET user_id = ? WHERE user_id = ?", it.newID, it.oldID)
			}
		}
	}

	if tableExists("user_mfa") {
		rows, err := db.Query("SELECT user_id, revision FROM user_mfa")
		if err == nil {
			type mfaItem struct {
				oldUser, newUser string
				oldRev, newRev   string
			}
			var toUpdate []mfaItem
			for rows.Next() {
				var u, r string
				if err := rows.Scan(&u, &r); err != nil {
					continue
				}
				nu, uOk := id.Normalize(u)
				if !uOk {
					nu = u
				}
				nr, rOk := id.Normalize(r)
				if !rOk {
					nr = r
				}
				if (uOk && nu != u) || (rOk && nr != r) {
					toUpdate = append(toUpdate, mfaItem{oldUser: u, newUser: nu, oldRev: r, newRev: nr})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_mfa SET user_id = ?, revision = ? WHERE user_id = ?", it.newUser, it.newRev, it.oldUser)
			}
		}
	}

	if tableExists("account_ip_bans") {
		rows, err := db.Query("SELECT user_id, ip FROM account_ip_bans")
		if err == nil {
			type banItem struct{ oldUser, newUser, ip string }
			var toUpdate []banItem
			for rows.Next() {
				var u, ip string
				if err := rows.Scan(&u, &ip); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, banItem{oldUser: u, newUser: nu, ip: ip})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE account_ip_bans SET user_id = ? WHERE user_id = ? AND ip = ?", it.newUser, it.oldUser, it.ip)
			}
		}
	}

	if tableExists("user_account_security") {
		rows, err := db.Query("SELECT user_id FROM user_account_security")
		if err == nil {
			type item struct{ oldID, newID string }
			var toUpdate []item
			for rows.Next() {
				var u string
				if err := rows.Scan(&u); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, item{oldID: u, newID: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_account_security SET user_id = ? WHERE user_id = ?", it.newID, it.oldID)
			}
		}
	}

	if tableExists("user_email_addresses") {
		rows, err := db.Query("SELECT email, user_id FROM user_email_addresses")
		if err == nil {
			type item struct{ email, newUser string }
			var toUpdate []item
			for rows.Next() {
				var email, u string
				if err := rows.Scan(&email, &u); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, item{email: email, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_email_addresses SET user_id = ? WHERE email = ?", it.newUser, it.email)
			}
		}
	}

	if tableExists("user_primary_email_history") {
		rows, err := db.Query("SELECT user_id, email FROM user_primary_email_history")
		if err == nil {
			type item struct{ oldUser, newUser, email string }
			var toUpdate []item
			for rows.Next() {
				var u, email string
				if err := rows.Scan(&u, &email); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, item{oldUser: u, newUser: nu, email: email})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_primary_email_history SET user_id = ? WHERE user_id = ? AND email = ?", it.newUser, it.oldUser, it.email)
			}
		}
	}

	if tableExists("user_password_resets") {
		rows, err := db.Query("SELECT email, user_id FROM user_password_resets")
		if err == nil {
			type item struct{ email, newUser string }
			var toUpdate []item
			for rows.Next() {
				var email, u string
				if err := rows.Scan(&email, &u); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, item{email: email, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_password_resets SET user_id = ? WHERE email = ?", it.newUser, it.email)
			}
		}
	}

	if tableExists("user_email_changes") {
		rows, err := db.Query("SELECT user_id FROM user_email_changes")
		if err == nil {
			type item struct{ oldUser, newUser string }
			var toUpdate []item
			for rows.Next() {
				var u string
				if err := rows.Scan(&u); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, item{oldUser: u, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_email_changes SET user_id = ? WHERE user_id = ?", it.newUser, it.oldUser)
			}
		}
	}

	if tableExists("user_recovery_codes") {
		rows, err := db.Query("SELECT user_id, selector_hash FROM user_recovery_codes")
		if err == nil {
			type item struct{ oldUser, newUser, selector string }
			var toUpdate []item
			for rows.Next() {
				var u, sel string
				if err := rows.Scan(&u, &sel); err != nil {
					continue
				}
				if nu, ok := id.Normalize(u); ok && nu != u {
					toUpdate = append(toUpdate, item{oldUser: u, newUser: nu, selector: sel})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_recovery_codes SET user_id = ? WHERE user_id = ? AND selector_hash = ?", it.newUser, it.oldUser, it.selector)
			}
		}
	}

	if tableExists("user_api_tokens") {
		rows, err := db.Query("SELECT id, user_id FROM user_api_tokens")
		if err == nil {
			type tokenItem struct {
				oldID, newID     string
				oldUser, newUser string
			}
			var toUpdate []tokenItem
			for rows.Next() {
				var tid, uid string
				if err := rows.Scan(&tid, &uid); err != nil {
					continue
				}
				nid, idOk := id.Normalize(tid)
				if !idOk {
					nid = tid
				}
				nuid, uOk := id.Normalize(uid)
				if !uOk {
					nuid = uid
				}
				if (idOk && nid != tid) || (uOk && nuid != uid) {
					toUpdate = append(toUpdate, tokenItem{oldID: tid, newID: nid, oldUser: uid, newUser: nuid})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_api_tokens SET id = ?, user_id = ? WHERE id = ?", it.newID, it.newUser, it.oldID)
			}
		}
	}

	if tableExists("download_statistics") {
		rows, err := db.Query("SELECT id, user_id FROM download_statistics")
		if err == nil {
			type statItem struct{ id, newUser string }
			var toUpdate []statItem
			for rows.Next() {
				var did, uid string
				if err := rows.Scan(&did, &uid); err != nil {
					continue
				}
				if nu, ok := id.Normalize(uid); ok && nu != uid {
					toUpdate = append(toUpdate, statItem{id: did, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE download_statistics SET user_id = ? WHERE id = ?", it.newUser, it.id)
			}
		}
	}

	if tableExists("super_teams") {
		rows, err := db.Query("SELECT prefix, created_by FROM super_teams")
		if err == nil {
			type item struct{ prefix, newCreator string }
			var toUpdate []item
			for rows.Next() {
				var prefix, creator string
				if err := rows.Scan(&prefix, &creator); err != nil {
					continue
				}
				if nc, ok := id.Normalize(creator); ok && nc != creator {
					toUpdate = append(toUpdate, item{prefix: prefix, newCreator: nc})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE super_teams SET created_by = ? WHERE prefix = ?", it.newCreator, it.prefix)
			}
		}
	}

	if tableExists("super_team_members") {
		rows, err := db.Query("SELECT team_prefix, user_id FROM super_team_members")
		if err == nil {
			type item struct{ prefix, oldUser, newUser string }
			var toUpdate []item
			for rows.Next() {
				var prefix, uid string
				if err := rows.Scan(&prefix, &uid); err != nil {
					continue
				}
				if nu, ok := id.Normalize(uid); ok && nu != uid {
					toUpdate = append(toUpdate, item{prefix: prefix, oldUser: uid, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE super_team_members SET user_id = ? WHERE team_prefix = ? AND user_id = ?", it.newUser, it.prefix, it.oldUser)
			}
		}
	}

	if tableExists("super_team_invitations") {
		rows, err := db.Query("SELECT id, inviter_id, recipient_id FROM super_team_invitations")
		if err == nil {
			type invItem struct {
				oldID, newID   string
				oldInv, newInv string
				oldRec, newRec string
			}
			var toUpdate []invItem
			for rows.Next() {
				var iid, inv, rec string
				if err := rows.Scan(&iid, &inv, &rec); err != nil {
					continue
				}
				nid, idOk := id.Normalize(iid)
				if !idOk {
					nid = iid
				}
				ninv, invOk := id.Normalize(inv)
				if !invOk {
					ninv = inv
				}
				nrec, recOk := id.Normalize(rec)
				if !recOk {
					nrec = rec
				}
				if (idOk && nid != iid) || (invOk && ninv != inv) || (recOk && nrec != rec) {
					toUpdate = append(toUpdate, invItem{oldID: iid, newID: nid, oldInv: inv, newInv: ninv, oldRec: rec, newRec: nrec})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE super_team_invitations SET id = ?, inviter_id = ?, recipient_id = ? WHERE id = ?", it.newID, it.newInv, it.newRec, it.oldID)
			}
		}
	}

	if tableExists("user_super_team_limits") {
		rows, err := db.Query("SELECT user_id FROM user_super_team_limits")
		if err == nil {
			type item struct{ oldID, newID string }
			var toUpdate []item
			for rows.Next() {
				var uid string
				if err := rows.Scan(&uid); err != nil {
					continue
				}
				if nu, ok := id.Normalize(uid); ok && nu != uid {
					toUpdate = append(toUpdate, item{oldID: uid, newID: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE user_super_team_limits SET user_id = ? WHERE user_id = ?", it.newID, it.oldID)
			}
		}
	}

	type memberTableConfig struct {
		name    string
		keyCols []string
		userCol string
	}
	memberTables := []memberTableConfig{
		{name: "npm_members", keyCols: []string{"repository", "package_name", "username"}, userCol: "user_id"},
		{name: "maven_domain_members", keyCols: []string{"repository", "domain", "username"}, userCol: "user_id"},
		{name: "cargo_members", keyCols: []string{"repository", "normalized_name", "username"}, userCol: "user_id"},
		{name: "docker_members", keyCols: []string{"repository", "image_name", "username"}, userCol: "user_id"},
		{name: "native_members", keyCols: []string{"resource_id", "user_id"}, userCol: "user_id"},
	}

	for _, cfg := range memberTables {
		if !tableExists(cfg.name) {
			continue
		}
		selectCols := strings.Join(append([]string(nil), cfg.keyCols...), ", ")
		query := fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s IS NOT NULL AND %s != ''", selectCols, cfg.userCol, cfg.name, cfg.userCol, cfg.userCol)
		rows, err := db.Query(query)
		if err != nil {
			continue
		}

		type rowUpdate struct {
			keyVals []any
			newUser string
		}
		var updates []rowUpdate

		for rows.Next() {
			vals := make([]any, len(cfg.keyCols)+1)
			valPtrs := make([]any, len(vals))
			for i := range vals {
				valPtrs[i] = &vals[i]
			}
			if err := rows.Scan(valPtrs...); err != nil {
				continue
			}
			rawUser, ok := vals[len(vals)-1].(string)
			if !ok {
				continue
			}
			normUser, changed := id.Normalize(rawUser)
			if changed && normUser != rawUser {
				keys := make([]any, len(cfg.keyCols))
				copy(keys, vals[:len(cfg.keyCols)])
				updates = append(updates, rowUpdate{keyVals: keys, newUser: normUser})
			}
		}
		_ = rows.Close()

		whereClauses := make([]string, len(cfg.keyCols))
		for i, k := range cfg.keyCols {
			whereClauses[i] = k + " = ?"
		}
		updateSQL := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s", cfg.name, cfg.userCol, strings.Join(whereClauses, " AND "))

		for _, up := range updates {
			args := append([]any{up.newUser}, up.keyVals...)
			execUpdate(updateSQL, args...)
		}
	}

	if tableExists("oauth_identities") {
		rows, err := db.Query("SELECT identity_hash, user_id FROM oauth_identities")
		if err == nil {
			type item struct{ hash, newUser string }
			var toUpdate []item
			for rows.Next() {
				var hash, uid string
				if err := rows.Scan(&hash, &uid); err != nil {
					continue
				}
				if nu, ok := id.Normalize(uid); ok && nu != uid {
					toUpdate = append(toUpdate, item{hash: hash, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE oauth_identities SET user_id = ? WHERE identity_hash = ?", it.newUser, it.hash)
			}
		}
	}

	if tableExists("github_identities") {
		rows, err := db.Query("SELECT github_user_id, user_id FROM github_identities")
		if err == nil {
			type item struct {
				ghID    int64
				newUser string
			}
			var toUpdate []item
			for rows.Next() {
				var ghID int64
				var uid string
				if err := rows.Scan(&ghID, &uid); err != nil {
					continue
				}
				if nu, ok := id.Normalize(uid); ok && nu != uid {
					toUpdate = append(toUpdate, item{ghID: ghID, newUser: nu})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE github_identities SET user_id = ? WHERE github_user_id = ?", it.newUser, it.ghID)
			}
		}
	}

	if tableExists("github_principals") {
		rows, err := db.Query("SELECT user_id, principal_type, github_principal_id FROM github_principals")
		if err == nil {
			type item struct {
				oldUser, newUser string
				pType            string
				pID              int64
			}
			var toUpdate []item
			for rows.Next() {
				var uid, pType string
				var pID int64
				if err := rows.Scan(&uid, &pType, &pID); err != nil {
					continue
				}
				if nu, ok := id.Normalize(uid); ok && nu != uid {
					toUpdate = append(toUpdate, item{oldUser: uid, newUser: nu, pType: pType, pID: pID})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE github_principals SET user_id = ? WHERE user_id = ? AND principal_type = ? AND github_principal_id = ?", it.newUser, it.oldUser, it.pType, it.pID)
			}
		}
	}

	if tableExists("review_tasks") {
		rows, err := db.Query("SELECT id, requested_by_id, decided_by_id FROM review_tasks")
		if err == nil {
			type taskItem struct {
				oldID, newID   string
				oldReq, newReq string
				oldDec, newDec string
			}
			var toUpdate []taskItem
			for rows.Next() {
				var tid, req, dec string
				if err := rows.Scan(&tid, &req, &dec); err != nil {
					continue
				}
				nid, idOk := id.Normalize(tid)
				if !idOk {
					nid = tid
				}
				nreq, reqOk := id.Normalize(req)
				if !reqOk {
					nreq = req
				}
				ndec, decOk := id.Normalize(dec)
				if !decOk {
					ndec = dec
				}
				if (idOk && nid != tid) || (reqOk && nreq != req) || (decOk && ndec != dec) {
					toUpdate = append(toUpdate, taskItem{oldID: tid, newID: nid, oldReq: req, newReq: nreq, oldDec: dec, newDec: ndec})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE review_tasks SET id = ?, requested_by_id = ?, decided_by_id = ? WHERE id = ?", it.newID, it.newReq, it.newDec, it.oldID)
			}
		}
	}

	if tableExists("review_task_files") {
		rows, err := db.Query("SELECT task_id, file_id FROM review_task_files")
		if err == nil {
			type item struct{ oldTask, newTask, fileID string }
			var toUpdate []item
			for rows.Next() {
				var tid, fid string
				if err := rows.Scan(&tid, &fid); err != nil {
					continue
				}
				if ntid, ok := id.Normalize(tid); ok && ntid != tid {
					toUpdate = append(toUpdate, item{oldTask: tid, newTask: ntid, fileID: fid})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE review_task_files SET task_id = ? WHERE task_id = ? AND file_id = ?", it.newTask, it.oldTask, it.fileID)
			}
		}
	}

	if tableExists("review_task_payloads") {
		rows, err := db.Query("SELECT task_id FROM review_task_payloads")
		if err == nil {
			type item struct{ oldTask, newTask string }
			var toUpdate []item
			for rows.Next() {
				var tid string
				if err := rows.Scan(&tid); err != nil {
					continue
				}
				if ntid, ok := id.Normalize(tid); ok && ntid != tid {
					toUpdate = append(toUpdate, item{oldTask: tid, newTask: ntid})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE review_task_payloads SET task_id = ? WHERE task_id = ?", it.newTask, it.oldTask)
			}
		}
	}

	if tableExists("ticket_state") {
		rows, err := db.Query("SELECT task_id, assignee_id, escalated_by_id FROM ticket_state")
		if err == nil {
			type item struct {
				oldTask, newTask string
				oldAss, newAss   string
				oldEsc, newEsc   string
			}
			var toUpdate []item
			for rows.Next() {
				var tid, ass, esc string
				if err := rows.Scan(&tid, &ass, &esc); err != nil {
					continue
				}
				ntid, tOk := id.Normalize(tid)
				if !tOk {
					ntid = tid
				}
				nass, aOk := id.Normalize(ass)
				if !aOk {
					nass = ass
				}
				nesc, eOk := id.Normalize(esc)
				if !eOk {
					nesc = esc
				}
				if (tOk && ntid != tid) || (aOk && nass != ass) || (eOk && nesc != esc) {
					toUpdate = append(toUpdate, item{oldTask: tid, newTask: ntid, oldAss: ass, newAss: nass, oldEsc: esc, newEsc: nesc})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE ticket_state SET task_id = ?, assignee_id = ?, escalated_by_id = ? WHERE task_id = ?", it.newTask, it.newAss, it.newEsc, it.oldTask)
			}
		}
	}

	if tableExists("ticket_messages") {
		rows, err := db.Query("SELECT id, task_id, author_id FROM ticket_messages")
		if err == nil {
			type item struct {
				oldID, newID     string
				oldTask, newTask string
				oldAuth, newAuth string
			}
			var toUpdate []item
			for rows.Next() {
				var mid, tid, aid string
				if err := rows.Scan(&mid, &tid, &aid); err != nil {
					continue
				}
				nmid, mOk := id.Normalize(mid)
				if !mOk {
					nmid = mid
				}
				ntid, tOk := id.Normalize(tid)
				if !tOk {
					ntid = tid
				}
				naid, aOk := id.Normalize(aid)
				if !aOk {
					naid = aid
				}
				if (mOk && nmid != mid) || (tOk && ntid != tid) || (aOk && naid != aid) {
					toUpdate = append(toUpdate, item{oldID: mid, newID: nmid, oldTask: tid, newTask: ntid, oldAuth: aid, newAuth: naid})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE ticket_messages SET id = ?, task_id = ?, author_id = ? WHERE id = ?", it.newID, it.newTask, it.newAuth, it.oldID)
			}
		}
	}

	if tableExists("sessions") {
		rows, err := db.Query("SELECT session_token, public_id FROM sessions")
		if err == nil {
			type item struct{ token, newPublic string }
			var toUpdate []item
			for rows.Next() {
				var tok, pub string
				if err := rows.Scan(&tok, &pub); err != nil {
					continue
				}
				if npub, ok := id.Normalize(pub); ok && npub != pub {
					toUpdate = append(toUpdate, item{token: tok, newPublic: npub})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE sessions SET public_id = ? WHERE session_token = ?", it.newPublic, it.token)
			}
		}
	}

	if tableExists("audit_logs") {
		rows, err := db.Query("SELECT id, session_id FROM audit_logs WHERE session_id != ''")
		if err == nil {
			type item struct {
				id     int64
				newSID string
			}
			var toUpdate []item
			for rows.Next() {
				var aid int64
				var sid string
				if err := rows.Scan(&aid, &sid); err != nil {
					continue
				}
				if nsid, ok := id.Normalize(sid); ok && nsid != sid {
					toUpdate = append(toUpdate, item{id: aid, newSID: nsid})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE audit_logs SET session_id = ? WHERE id = ?", it.newSID, it.id)
			}
		}
	}

	if tableExists("mail_jobs") {
		rows, err := db.Query("SELECT id, user_id FROM mail_jobs")
		if err == nil {
			type item struct {
				oldID, newID     string
				oldUser, newUser string
			}
			var toUpdate []item
			for rows.Next() {
				var jid, uid string
				if err := rows.Scan(&jid, &uid); err != nil {
					continue
				}
				njid, jOk := id.Normalize(jid)
				if !jOk {
					njid = jid
				}
				nuid, uOk := id.Normalize(uid)
				if !uOk {
					nuid = uid
				}
				if (jOk && njid != jid) || (uOk && nuid != uid) {
					toUpdate = append(toUpdate, item{oldID: jid, newID: njid, oldUser: uid, newUser: nuid})
				}
			}
			_ = rows.Close()
			for _, it := range toUpdate {
				execUpdate("UPDATE mail_jobs SET id = ?, user_id = ? WHERE id = ?", it.newID, it.newUser, it.oldID)
			}
		}
	}

	log.Printf("[UUID-Migration] Completed full-table scan: migrated %d UUID values to compact IDs", totalMigrated)
	return totalMigrated, nil
}

func (db *DB) execMigrationUpdate(query string, args ...any) (int64, error) {
	if db == nil || (db.SQLDB == nil && db.clickHouse == nil) {
		return 0, fmt.Errorf("database not initialized")
	}
	res, err := db.Exec(query, args...)
	if err == nil {
		if aff, err := res.RowsAffected(); err == nil && aff > 0 {
			return aff, nil
		}
		return 1, nil
	}
	if db.clickHouse != nil {
		if aff := db.clickHousePKMigrationFallback(query, args...); aff > 0 {
			return aff, nil
		}
	}
	return 0, err
}

func (db *DB) clickHousePKMigrationFallback(query string, args ...any) int64 {
	if db == nil || db.clickHouse == nil {
		return 0
	}
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "UPDATE") {
		return 0
	}
	remainder := strings.TrimSpace(trimmed[6:])
	sep := strings.IndexAny(remainder, " \t\r\n")
	if sep <= 0 {
		return 0
	}
	table := strings.Trim(remainder[:sep], "`")
	setIdx := strings.Index(strings.ToUpper(remainder), "SET")
	whereIdx := strings.Index(strings.ToUpper(remainder), "WHERE")
	if setIdx < 0 || whereIdx < 0 || whereIdx <= setIdx {
		return 0
	}

	var schema *clickHouseTableSchema
	for _, s := range clickHouseSchemas() {
		if s.name == table {
			schema = &s
			break
		}
	}
	if schema == nil {
		return 0
	}
	cols, err := schema.dataColumns()
	if err != nil || len(cols) == 0 {
		return 0
	}

	setPart := strings.TrimSpace(remainder[setIdx+3 : whereIdx])
	wherePart := strings.TrimSpace(remainder[whereIdx+5:])

	setClauses := strings.Split(setPart, ",")
	setCols := make([]string, len(setClauses))
	for i, sc := range setClauses {
		parts := strings.Split(sc, "=")
		setCols[i] = strings.Trim(strings.TrimSpace(parts[0]), "`")
	}

	if len(args) < len(setCols) {
		return 0
	}
	setVals := args[:len(setCols)]
	whereVals := args[len(setCols):]

	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = "`" + c + "`"
	}
	selectSQL := fmt.Sprintf("SELECT %s FROM `%s` WHERE %s", strings.Join(quotedCols, ", "), table, wherePart)
	rows, err := db.Query(selectSQL, whereVals...)
	if err != nil {
		return 0
	}
	defer rows.Close()

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", table, strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))
	deleteSQL := fmt.Sprintf("ALTER TABLE `%s` DELETE WHERE %s", table, wherePart)

	var count int64
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		for sIdx, sCol := range setCols {
			for cIdx, colName := range cols {
				if colName == sCol {
					vals[cIdx] = setVals[sIdx]
				}
			}
		}
		if _, err := db.Exec(insertSQL, vals...); err == nil {
			_, _ = db.Exec(deleteSQL, whereVals...)
			count++
		}
	}
	return count
}
