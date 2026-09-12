/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package configstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"renop/internal/config"
	"renop/internal/testutil"
)

func TestMigrationPersistsPrivateSettingsAndNeverReimports(t *testing.T) {
	dir := testutil.TempDir(t)
	path, legacy := filepath.Join(dir, "settings.db"), filepath.Join(dir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Server.Host, cfg.Server.Port = "127.0.0.2", 9090
	cfg.MFAEncryptionKey = "private-mfa-key"
	cfg.Mail.EncryptionKey = "private-mail-key"
	cfg.Database.Driver, cfg.Database.Dsn = "postgres", "private-dsn"
	cfg.Legal.PrivacyPolicy = "# Privacy\nExample policy"
	require.NoError(t, cfg.Legal.Normalize())
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(legacy, data, 0600))
	loaded, err := Load(path, legacy)
	require.NoError(t, err)
	require.Equal(t, cfg.Server.Port, loaded.Server.Port)
	require.Equal(t, cfg.MFAEncryptionKey, loaded.MFAEncryptionKey)
	require.Equal(t, cfg.Mail.EncryptionKey, loaded.Mail.EncryptionKey)
	require.Equal(t, cfg.Database, loaded.Database)
	require.Equal(t, cfg.Legal, loaded.Legal)
	require.NoFileExists(t, legacy)
	archives, err := filepath.Glob(legacy + ".migrated.*")
	require.NoError(t, err)
	require.Len(t, archives, 1)
	require.NoError(t, os.WriteFile(legacy, []byte("invalid: : :"), 0600))
	loaded.Server.Port = 8089
	require.NoError(t, Save(path, loaded))
	restarted, err := Load(path, legacy)
	require.NoError(t, err)
	require.EqualValues(t, 8089, restarted.Server.Port)
	require.Equal(t, cfg.MFAEncryptionKey, restarted.MFAEncryptionKey)
}

func TestDefaultsAndFailedSnapshots(t *testing.T) {
	dir := testutil.TempDir(t)
	path, legacy := filepath.Join(dir, "settings.db"), filepath.Join(dir, "config.yaml")
	_, err := Load(path, legacy)
	require.NoError(t, err)
	require.NoFileExists(t, legacy)
	before, err := Read(path)
	require.NoError(t, err)
	cfg := config.DefaultConfig()
	cfg.Legal.PrivacyPolicy = strings.Repeat("x", MaxSnapshotBytes+1)
	require.Error(t, Save(path, cfg))
	after, err := Read(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Error(t, CompareAndSwap(path, before, []byte("null")))
	require.Error(t, CompareAndSwap(path, before, []byte("server: {}\n---\nserver: {}")))
}

func TestOfflineUpdateAndRollbackPreserveConcurrentChanges(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "settings.db")
	_, err := Load(path, "")
	require.NoError(t, err)
	before, err := Read(path)
	require.NoError(t, err)
	next := []byte("server:\n  port: 9090\n")
	require.NoError(t, CompareAndSwap(path, before, next))
	require.ErrorIs(t, CompareAndSwap(path, before, []byte("server: {}")), ErrChanged)
	require.ErrorIs(t, CompareAndSwap(path, nil, before), ErrChanged)
	concurrent := []byte("server:\n  port: 9091\n")
	require.NoError(t, CompareAndSwap(path, next, concurrent))
	require.ErrorIs(t, CompareAndSwap(path, next, before), ErrChanged)
	after, err := Read(path)
	require.NoError(t, err)
	require.Equal(t, concurrent, after)
}

func TestInvalidLegacyDoesNotCommitDefaults(t *testing.T) {
	dir := testutil.TempDir(t)
	path, legacy := filepath.Join(dir, "settings.db"), filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(legacy, []byte("server: ["), 0600))
	_, err := Load(path, legacy)
	require.Error(t, err)
	data, err := Read(path)
	require.NoError(t, err)
	require.Nil(t, data)
	require.FileExists(t, legacy)
}
