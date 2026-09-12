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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/mail"
)

func TestMySQLRequestControls(t *testing.T) {
	db, _ := newMySQLTestDatabase(t)
	now := time.Now().UnixMilli()
	require.NoError(t, checkTickets(db, "mysql", now))
	profile, err := db.UpdateUserProfileLinks("ticket-reporter-mysql", core.UserProfileLinks{
		Visibility: &core.ProfileLinkVisibility{GitHub: true, GitLab: true},
	}, now)
	require.NoError(t, err)
	require.True(t, profile.Links.Visibility.GitHub)
	require.True(t, profile.Links.Visibility.GitLab)
	cfg := mail.DefaultConfig()
	require.NoError(t, cfg.EnsureKey())
	job := func(id string, at int64) *mail.Job {
		return &mail.Job{ID: id, Scene: "test", CreatedAt: at, ExpiresAt: at + 600000,
			Message: mail.Message{ID: id, To: "recipient@example.test", Subject: "Test", Text: "Test", CreatedAt: at}}
	}
	created, err := db.QueueMailJob(job("first", now), cfg.EncryptionKey, "192.0.2.1", cfg.ManualRate)
	require.NoError(t, err)
	require.True(t, created)
	_, err = db.QueueMailJob(job("rotated", now), cfg.EncryptionKey, "192.0.2.2", cfg.ManualRate)
	require.ErrorIs(t, err, mail.ErrRateLimited)
	created, err = db.QueueMailJob(job("expired", now+120001), cfg.EncryptionKey, "192.0.2.2", cfg.ManualRate)
	require.NoError(t, err)
	require.True(t, created)
}

// newMySQLTestDatabase owns a fresh utf8mb4 schema and removes only that schema after all handles close.
func newMySQLTestDatabase(t *testing.T) (*DB, config.DatabaseConfig) {
	t.Helper()
	dsn := os.Getenv("RENOP_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("RENOP_TEST_MYSQL_DSN is not configured")
	}
	options, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)
	options.DBName = ""
	admin, err := sql.Open("mysql", options.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, admin.Close()) })
	schema := "renop_mysql_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec("CREATE DATABASE " + schema + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec("DROP DATABASE " + schema)
		require.NoError(t, err)
	})
	options.DBName = schema
	cfg := config.DatabaseConfig{Driver: "mysql", Dsn: options.FormatDSN(), MaxOpenConns: 4}
	db, err := InitDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db, cfg
}
