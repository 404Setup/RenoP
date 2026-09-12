/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package configstore persists startup settings independently of the selected application database.
package configstore

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
	_ "modernc.org/sqlite"

	"renop/internal/config"
)

// MaxSnapshotBytes bounds both imported and persisted settings, including private keys.
const MaxSnapshotBytes = 16 << 20

// LegacyPath locates the optional one-time YAML import.
func LegacyPath() string {
	if path := os.Getenv("RENOP_CONFIG"); path != "" {
		return path
	}
	return "config.yaml"
}

// Path locates the private SQLite settings database, next to a custom legacy configuration by default.
func Path() string { return PathForLegacy(LegacyPath()) }

// PathForLegacy preserves the configuration directory used by existing installations.
func PathForLegacy(legacy string) string {
	if path := os.Getenv("RENOP_SETTINGS_DB"); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(legacy), "renop-settings.db")
}

func open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("settings database path is empty")
	}
	// Create with private permissions before SQLite can create the file with its defaults.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open settings database: %w", err)
	}
	if err := errors.Join(f.Chmod(0600), f.Close()); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := u.Query()
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(FULL)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY CHECK (id = 1), payload TEXT NOT NULL
	)`); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return db, nil
}

func decode(data []byte) (*config.Config, error) {
	if len(data) > MaxSnapshotBytes {
		return nil, errors.New("settings exceed the size limit")
	}
	var cfg *config.Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode settings: %w", err)
	}
	if cfg == nil {
		return nil, errors.New("settings must contain a mapping")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("settings must contain exactly one document")
	}
	return cfg, nil
}

// Read returns the stored snapshot, or nil if no snapshot has been committed.
// The bytes contain secrets and must never be returned through a public API.
func Read(path string) (data []byte, err error) {
	db, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	var payload string
	err = db.QueryRow("SELECT payload FROM settings WHERE id = 1 AND length(CAST(payload AS BLOB)) <= ?", MaxSnapshotBytes).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM settings WHERE id = 1").Scan(&count); err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, errors.New("settings exceed the size limit")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	data = []byte(payload)
	_, err = decode(data)
	return data, err
}

// Load imports legacy settings only before the first durable snapshot; it never creates YAML files.
func Load(path, legacyPath string) (*config.Config, error) {
	data, err := Read(path)
	if err != nil {
		return nil, err
	}
	if data != nil {
		return decode(data)
	}
	cfg := config.DefaultConfig()
	imported := false
	if legacyPath != "" {
		file, err := os.Open(legacyPath)
		if err == nil {
			data, err = io.ReadAll(io.LimitReader(file, MaxSnapshotBytes+1))
			err = errors.Join(err, file.Close())
			if err != nil {
				return nil, err
			}
			cfg, err = decode(data)
			if err != nil {
				return nil, err
			}
			imported = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	encoded, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := CompareAndSwap(path, nil, encoded); err != nil && !errors.Is(err, ErrChanged) {
		return nil, err
	}
	stored, err := Read(path)
	if err != nil {
		return nil, err
	}
	if imported && bytes.Equal(stored, encoded) {
		archive := legacyPath + ".migrated." + rand.Text()
		if err := os.Rename(legacyPath, archive); err != nil {
			log.Printf("Settings committed to the database; could not archive the legacy configuration: %v", err)
		}
	}
	return decode(stored)
}

// ErrChanged prevents offline tools from overwriting concurrent administrator changes.
var ErrChanged = errors.New("settings changed concurrently")

// CompareAndSwap atomically installs a snapshot only if the previous bytes still match.
func CompareAndSwap(path string, previous, next []byte) (err error) {
	if _, err := decode(next); err != nil {
		return err
	}
	db, err := open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	var result sql.Result
	if previous == nil {
		result, err = db.Exec("INSERT INTO settings (id, payload) VALUES (1, ?) ON CONFLICT(id) DO NOTHING", string(next))
	} else {
		result, err = db.Exec("UPDATE settings SET payload = ? WHERE id = 1 AND payload = ?", string(next), string(previous))
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrChanged
	}
	return err
}

// Save durably replaces the full private settings snapshot before callers publish it in memory.
func Save(path string, cfg *config.Config) (err error) {
	if cfg == nil {
		return errors.New("settings are unavailable")
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if len(data) > MaxSnapshotBytes {
		return errors.New("settings exceed the size limit")
	}
	db, err := open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	_, err = db.Exec("INSERT INTO settings (id, payload) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET payload = excluded.payload", string(data))
	return err
}
