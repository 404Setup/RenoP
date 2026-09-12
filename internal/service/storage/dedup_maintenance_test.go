/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/repositorygate"
)

func TestDedupMaintenanceSkipsBusyRepositoriesAndMutableMetadata(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	cfg.Maven.Repositories["local"] = &config.Repository{Name: "local", Format: config.RepositoryFormatAPT}
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	root := filepath.Join(cfg.StoragePath, "local")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.deb", "b.deb", "Packages", "conanmanifest.txt", "pending.tmp"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("same body"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	same := func(a, b string) bool {
		t.Helper()
		ai, err := os.Stat(filepath.Join(root, a))
		if err != nil {
			t.Fatal(err)
		}
		bi, err := os.Stat(filepath.Join(root, b))
		if err != nil {
			t.Fatal(err)
		}
		return os.SameFile(ai, bi)
	}
	unblock := repositorygate.AcquireMutation("local")
	DeduplicateExisting(context.Background(), state)
	unblock()
	if same("a.deb", "b.deb") {
		t.Fatal("maintenance ran while repository was busy")
	}
	DeduplicateExisting(context.Background(), state)
	if !same("a.deb", "b.deb") {
		t.Fatal("existing artifacts were not deduplicated")
	}
	for _, name := range []string{"Packages", "conanmanifest.txt", "pending.tmp"} {
		if same("a.deb", name) {
			t.Fatalf("coalesced mutable metadata or staging: %s", name)
		}
	}
}
