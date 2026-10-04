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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/pkg/id"
)

func TestMigrateLegacyUUIDs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "migration_test.db")

	cfg := config.DatabaseConfig{
		Driver: "sqlite3",
		Dsn:    dbPath,
	}
	db, err := InitDB(cfg)
	require.NoError(t, err)
	defer db.Close()

	legacyUserUUID := "550e8400-e29b-41d4-a716-446655440000"
	legacyRevUUID := "a0000000-0000-0000-0000-000000000001"
	legacyMsgUUID := "b0000000-0000-0000-0000-000000000002"
	legacyTaskUUID := "c0000000-0000-0000-0000-000000000003"
	legacySessionUUID := "d0000000-0000-0000-0000-000000000004"

	expectedNewUserID, err := id.FromUUIDString(legacyUserUUID)
	require.NoError(t, err)
	expectedNewRevID, err := id.FromUUIDString(legacyRevUUID)
	require.NoError(t, err)
	expectedNewMsgID, err := id.FromUUIDString(legacyMsgUUID)
	require.NoError(t, err)
	expectedNewTaskID, err := id.FromUUIDString(legacyTaskUUID)
	require.NoError(t, err)
	expectedNewSessionID, err := id.FromUUIDString(legacySessionUUID)
	require.NoError(t, err)

	now := time.Now().UnixMilli()

	_, err = db.Exec(`INSERT INTO user_profiles (user_id, username, nickname, updated_at) VALUES (?, ?, ?, ?)`,
		legacyUserUUID, "alice", "Alice", now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO user_mfa (user_id, secret, revision, passkey_enabled, last_step, window_start, failures) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		legacyUserUUID, "secret123", legacyRevUUID, 0, 0, 0, 0)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO user_messages (id, recipient, sender, kind, severity, title, body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		legacyMsgUUID, "alice", "system", "notice", "info", "Hello", "Welcome", now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO review_tasks (id, kind, resource_type, repository, resource_key, resource_name, source_team_prefix, target_team_prefix, review_team_prefix, requested_by_id, requested_by_name, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		legacyTaskUUID, "publication", "package", "repo", "key", "name", "", "", "team", legacyUserUUID, "alice", "pending", now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO review_task_files (task_id, file_id, path, size, critical, added_at) VALUES (?, ?, ?, ?, ?, ?)`,
		legacyTaskUUID, "file-1", "/path/to/file", 100, 0, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sessions (session_token, public_id, username, ip, user_agent, created_at, last_active) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"token-abc", legacySessionUUID, "alice", "127.0.0.1", "ua", now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO audit_logs (username, operator, action, ip, created_at, session_id, details, auth_method) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"alice", "alice", "login", "127.0.0.1", now, legacySessionUUID, "logged in", "password")
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO super_teams (prefix, name, description, created_by, created_by_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"team_a", "Team A", "desc", legacyUserUUID, "alice", now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO super_team_members (team_prefix, user_id, role_level, added_at) VALUES (?, ?, ?, ?)`,
		"team_a", legacyUserUUID, 1, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO user_super_team_limits (user_id, create_limit, join_limit, updated_at) VALUES (?, ?, ?, ?)`,
		legacyUserUUID, 10, 10, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO oauth_identities (identity_hash, provider_id, subject, authority, user_id, login, namespaces_json, authorized_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"hash123", "github", "sub123", "auth", legacyUserUUID, "alice", "[]", now)
	require.NoError(t, err)

	legacyTokenUUID := "e0000000-0000-0000-0000-000000000005"
	expectedNewTokenID, err := id.FromUUIDString(legacyTokenUUID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_api_tokens (id, user_id, name, secret_hash, scopes_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		legacyTokenUUID, legacyUserUUID, "token1", "sechash123", "[]", now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO download_statistics (id, user_id, username, repository, format, namespace, package_name, version, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"stat-1", legacyUserUUID, "alice", "repo", "npm", "ns", "pkg", "1.0.0", now)
	require.NoError(t, err)

	legacyMailUUID := "f0000000-0000-0000-0000-000000000006"
	expectedNewMailID, err := id.FromUUIDString(legacyMailUUID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO mail_jobs (id, account_id, user_id, actor, scene, status, payload, result_json, checks, created_at, updated_at, next_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		legacyMailUUID, "acc1", legacyUserUUID, "actor", "scene", "pending", "{}", "{}", 0, now, now, now, now)
	require.NoError(t, err)

	db.userIDCache.Set("alice", legacyUserUUID, time.Hour)
	_, cachedBefore := db.userIDCache.Get("alice")
	require.True(t, cachedBefore)

	migratedCount, err := db.MigrateLegacyUUIDs(context.Background())
	require.NoError(t, err)
	require.Greater(t, migratedCount, int64(0))

	_, cachedAfter := db.userIDCache.Get("alice")
	require.False(t, cachedAfter, "in-memory cache must be invalidated during migration")

	var gotUserID string
	err = db.QueryRow(`SELECT user_id FROM user_profiles WHERE username = ?`, "alice").Scan(&gotUserID)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotUserID)
	require.True(t, id.IsValid(gotUserID))

	var gotMFAUser, gotMFARev string
	err = db.QueryRow(`SELECT user_id, revision FROM user_mfa WHERE user_id = ?`, expectedNewUserID).Scan(&gotMFAUser, &gotMFARev)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotMFAUser)
	require.Equal(t, expectedNewRevID, gotMFARev)
	require.True(t, id.IsValid(gotMFARev))

	var gotMsgID string
	err = db.QueryRow(`SELECT id FROM user_messages WHERE recipient = ?`, "alice").Scan(&gotMsgID)
	require.NoError(t, err)
	require.Equal(t, expectedNewMsgID, gotMsgID)
	require.True(t, id.IsValid(gotMsgID))

	var gotTaskID, gotTaskRequester string
	err = db.QueryRow(`SELECT id, requested_by_id FROM review_tasks WHERE id = ?`, expectedNewTaskID).Scan(&gotTaskID, &gotTaskRequester)
	require.NoError(t, err)
	require.Equal(t, expectedNewTaskID, gotTaskID)
	require.Equal(t, expectedNewUserID, gotTaskRequester)

	var gotChildTaskID string
	err = db.QueryRow(`SELECT task_id FROM review_task_files WHERE task_id = ?`, expectedNewTaskID).Scan(&gotChildTaskID)
	require.NoError(t, err)
	require.Equal(t, expectedNewTaskID, gotChildTaskID)

	var gotSessionPublicID string
	err = db.QueryRow(`SELECT public_id FROM sessions WHERE session_token = ?`, "token-abc").Scan(&gotSessionPublicID)
	require.NoError(t, err)
	require.Equal(t, expectedNewSessionID, gotSessionPublicID)

	var gotAuditSessionID string
	err = db.QueryRow(`SELECT session_id FROM audit_logs WHERE username = ?`, "alice").Scan(&gotAuditSessionID)
	require.NoError(t, err)
	require.Equal(t, expectedNewSessionID, gotAuditSessionID)

	var gotTeamCreator string
	err = db.QueryRow(`SELECT created_by FROM super_teams WHERE prefix = ?`, "team_a").Scan(&gotTeamCreator)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotTeamCreator)

	var gotTeamMember string
	err = db.QueryRow(`SELECT user_id FROM super_team_members WHERE team_prefix = ?`, "team_a").Scan(&gotTeamMember)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotTeamMember)

	var gotLimitUser string
	err = db.QueryRow(`SELECT user_id FROM user_super_team_limits WHERE user_id = ?`, expectedNewUserID).Scan(&gotLimitUser)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotLimitUser)

	var gotOAuthUser string
	err = db.QueryRow(`SELECT user_id FROM oauth_identities WHERE identity_hash = ?`, "hash123").Scan(&gotOAuthUser)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotOAuthUser)

	var gotTokenID, gotTokenUser string
	err = db.QueryRow(`SELECT id, user_id FROM user_api_tokens WHERE id = ?`, expectedNewTokenID).Scan(&gotTokenID, &gotTokenUser)
	require.NoError(t, err)
	require.Equal(t, expectedNewTokenID, gotTokenID)
	require.Equal(t, expectedNewUserID, gotTokenUser)

	var gotStatUser string
	err = db.QueryRow(`SELECT user_id FROM download_statistics WHERE id = ?`, "stat-1").Scan(&gotStatUser)
	require.NoError(t, err)
	require.Equal(t, expectedNewUserID, gotStatUser)

	var gotMailJobID, gotMailJobUser string
	err = db.QueryRow(`SELECT id, user_id FROM mail_jobs WHERE id = ?`, expectedNewMailID).Scan(&gotMailJobID, &gotMailJobUser)
	require.NoError(t, err)
	require.Equal(t, expectedNewMailID, gotMailJobID)
	require.Equal(t, expectedNewUserID, gotMailJobUser)

	secondMigrated, err := db.MigrateLegacyUUIDs(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(0), secondMigrated)
}

func TestInvalidateCachesWithoutRemote(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "cache_invalidation_test.db")
	cfg := config.DatabaseConfig{
		Driver: "sqlite3",
		Dsn:    dbPath,
	}
	db, err := InitDB(cfg)
	require.NoError(t, err)
	defer db.Close()

	db.userIDCache.Set("user1", "val1", time.Hour)
	db.InvalidateCaches()
	_, found := db.userIDCache.Get("user1")
	require.False(t, found)
}

func TestExecMigrationUpdateAndFallback(t *testing.T) {
	var nilDB *DB
	require.Equal(t, int64(0), nilDB.clickHousePKMigrationFallback("UPDATE foo SET x = 1 WHERE y = 2"))
	_, err := nilDB.execMigrationUpdate("UPDATE foo SET x = 1 WHERE y = 2")
	require.Error(t, err)

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "update_test.db")
	db, err := InitDB(config.DatabaseConfig{
		Driver: "sqlite3",
		Dsn:    dbPath,
	})
	require.NoError(t, err)
	defer db.Close()

	// Invalid SQL returns error
	_, err = db.execMigrationUpdate("INVALID SQL STATEMENT")
	require.Error(t, err)

	// Valid SQL with 0 affected rows
	aff, err := db.execMigrationUpdate("UPDATE user_profiles SET username = 'nonexistent' WHERE user_id = 'nobody'")
	require.NoError(t, err)
	require.Equal(t, int64(1), aff)
}
