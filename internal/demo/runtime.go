/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package demo

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/emmansun/base64"
	"go.yaml.in/yaml/v3"

	"renop/internal/config"
	"renop/internal/configstore"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/mail"
	"renop/internal/service/index"
)

const seedVersion = "1"

func defaultRepositories() config.MavenSettings {
	settings := config.MavenSettings{Repositories: make(map[string]*config.Repository)}
	for _, definition := range []struct{ name, format, visibility string }{
		{"releases", "maven", "PUBLIC"}, {"snapshots", "maven-classic", "PUBLIC"},
		{"cargo", "cargo", "PUBLIC"}, {"npm", "npm", "PUBLIC"}, {"docker", "docker", "PUBLIC"},
		{"downloads", "files", "PUBLIC"}, {"private", "files", "PRIVATE"}, {"mirror", "maven", "PUBLIC"},
		{"review", "maven", "HIDDEN"},
	} {
		settings.Repositories[definition.name] = &config.Repository{Name: definition.name, Format: definition.format,
			Visibility: definition.visibility, AllowRedeployment: definition.format == "files" || definition.name == "snapshots",
			CapacityLimitBytes: 10 << 30}
	}
	settings.Repositories["review"].PublicationReview = config.PublicationReviewEveryVersion
	settings.Repositories["mirror"].Mirrors = []config.Mirror{{Name: "Maven Central", URL: "https://repo.maven.apache.org/maven2", Persist: true}}
	return settings
}

func defaultConfig() (*config.Config, error) {
	cfg := config.DefaultConfig()
	cfg.Frontend.Title = "RenoP Demo"
	cfg.Frontend.Description = "Explore package repositories, teams, reviews and system settings with preset data."
	cfg.Database.Dsn = databasePath()
	cfg.Registration.Enabled = true
	cfg.Mail.Enabled = true
	cfg.Mail.PublicURL = "https://demo.example.com"
	account := mail.Presets()[0].Account
	account.ID, account.Name, account.From = "demo-api", "Demo API sender", "notifications@example.com"
	account.APIKey, account.AccountID = "demo-not-a-real-key", "00000000000000000000000000000001"
	account.Quota = mail.Quota{Limit: 10000, Period: "month"}
	cfg.Mail.Accounts = []mail.Account{account}
	if err := cfg.Mail.EnsureKey(); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	cfg.MFAEncryptionKey = base64.RawStdEncoding.EncodeToString(key)
	return cfg, nil
}

func loadConfig(path string) (*config.Config, error) {
	stored, err := configstore.Read(path)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		cfg, err := defaultConfig()
		if err != nil {
			return nil, err
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		if err := configstore.CompareAndSwap(path, nil, data); err != nil && !errors.Is(err, configstore.ErrChanged) {
			return nil, err
		}
	}
	return configstore.Load(path, "")
}

func verifyDatabase(path string) error {
	db, err := database.OpenReadOnlySQLite(path)
	if err != nil {
		return err
	}
	defer db.Close()
	var version string
	if err := db.QueryRow(`SELECT value FROM demo_state WHERE name = 'seed_version'`).Scan(&version); err != nil {
		return errors.New("existing database is not a RenoP demo dataset")
	}
	if version != seedVersion {
		return errors.New("demo dataset version is unsupported")
	}
	return nil
}

// prepareDatabase publishes only a completely seeded database and never replaces an existing dataset.
func prepareDatabase(path string, cfg *config.Config) error {
	if _, err := os.Stat(path); err == nil {
		return verifyDatabase(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".renop-demo-*.db")
	if err != nil {
		return err
	}
	temporary := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	defer os.Remove(temporary)
	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: temporary, MaxOpenConns: 1, MaxIdleConns: 1})
	if err != nil {
		return err
	}
	err = seed(db, cfg)
	err = errors.Join(err, db.Close())
	if err != nil {
		return err
	}
	if err := os.Link(temporary, path); err == nil {
		return nil
	} else if errors.Is(err, os.ErrExist) {
		return verifyDatabase(path)
	}
	// Filesystems without hard links still get exclusive creation, never replacement.
	destination, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return verifyDatabase(path)
	}
	if err != nil {
		return err
	}
	source, err := os.Open(temporary)
	if err == nil {
		_, err = io.Copy(destination, source)
		err = errors.Join(err, source.Close())
	}
	if err == nil {
		err = destination.Sync()
	}
	err = errors.Join(err, destination.Close())
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

// Open loads isolated presets and attaches ephemeral browser sessions without starting workers or file storage.
func Open(options Options) (*core.AppState, error) {
	if !options.Enabled {
		return nil, errors.New("demo mode is not enabled")
	}
	settingsFile, dataFile := settingsPath(), databasePath()
	cfg, err := loadConfig(settingsFile)
	if err != nil {
		return nil, fmt.Errorf("load demo settings: %w", err)
	}
	if err := prepareDatabase(dataFile, cfg); err != nil {
		return nil, fmt.Errorf("prepare demo database: %w", err)
	}
	db, err := database.OpenReadOnlySQLite(dataFile)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()
	repositories, err := db.GetRepositorySettings()
	if err != nil || repositories == nil {
		return nil, errors.Join(err, errors.New("demo repository snapshot is unavailable"))
	}
	cfg.Maven = *repositories
	cfg.Runtime = config.RuntimeConfig{Demo: true, DemoTemp: options.Temporary, SettingsDatabase: settingsFile,
		DemoConfiguredStoragePath: cfg.StoragePath}
	cfg.StoragePath = filepath.Join(os.TempDir(), ".renop-demo-artifacts-"+rand.Text())
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	sessions := newSessionStore(db, state)
	state.Inner.DB = sessions
	if options.Temporary {
		sessions.repositoryWriter, err = database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: dataFile})
		if err != nil {
			return nil, err
		}
		defer func() {
			if !success {
				_ = sessions.repositoryWriter.Close()
			}
		}()
	}
	state.Inner.FileIndex = index.NewFileIndex()
	state.Inner.FileCache = core.NewFileByteCache(0)
	count, err := db.CountTokens()
	if err != nil {
		return nil, err
	}
	state.Inner.TokensCount.Store(count)
	if err := loadIndex(db, state); err != nil {
		return nil, err
	}
	now := time.Now()
	state.Inner.StartTime = now.Add(-6 * time.Hour).UnixMilli()
	snapshots := make([]core.StatusSnapshot, 15)
	for i := range snapshots {
		snapshots[i] = core.StatusSnapshot{Timestamp: now.Add(time.Duration(i-14) * 20 * time.Second).UnixMilli(),
			UsedMemory: uint64(88+i%5*3) << 20, VssMemory: 384 << 20, UsedThreads: uint64(18 + i%4), OpenFiles: 64}
	}
	state.Inner.StatusSnapshots.Store(&snapshots)
	success = true
	return state, nil
}

func loadIndex(db *database.DB, state *core.AppState) error {
	cfg := state.Inner.Config.Load()
	for name := range cfg.Maven.Repositories {
		state.Inner.FileIndex.InsertDir(filepath.Join(cfg.StoragePath, name))
	}
	rows, err := db.Query(`SELECT repository, path, size, modified_at FROM demo_files ORDER BY repository, path LIMIT 2048`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var repository, path string
		var size, modified int64
		if err := rows.Scan(&repository, &path, &size, &modified); err != nil {
			return err
		}
		if cfg.Maven.Repositories[repository] == nil {
			continue
		}
		full := filepath.Join(cfg.StoragePath, repository, filepath.FromSlash(path))
		state.Inner.FileIndex.EnsureParentDirs(full)
		state.Inner.FileIndex.InsertFile(full, index.FileInfo{Size: size, ModTime: modified * int64(time.Millisecond)})
	}
	return rows.Err()
}
