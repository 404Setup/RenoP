/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package api

import (
	"fmt"
	"path/filepath"
	"testing"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/service/index"
	"renop/internal/testutil"
)

func TestFileSearchRetainsLateExactMatchesAndUnicode(t *testing.T) {
	state := core.NewAppState()
	state.Inner.FileIndex = index.NewFileIndex()
	repo := &config.Repository{Name: "files", Format: config.RepositoryFormatFiles, Visibility: "PUBLIC"}
	root := filepath.Join("storage", repo.Name)
	for i := range 700 {
		path := filepath.Join(root, fmt.Sprintf("prefix-target-%04d", i))
		state.Inner.FileIndex.EnsureParentDirs(path)
		state.Inner.FileIndex.InsertFile(path, index.FileInfo{Size: 1})
	}
	state.Inner.FileIndex.InsertFile(filepath.Join(root, "target"), index.FileInfo{Size: 2})
	state.Inner.FileIndex.InsertFile(filepath.Join(root, "ÄPFEL.jar"), index.FileInfo{Size: 3})
	viewer := &config.User{Username: "guest"}
	response, err := searchFileTreeRepository(state, "storage", repo, viewer, "target", 1)
	if err != nil || len(response.Results) != 1 || response.Results[0].Name != "target" || response.Total != 701 || !response.HasMore {
		t.Fatalf("late exact match lost: %+v, %v", response, err)
	}
	response, err = searchFileTreeRepository(state, "storage", repo, viewer, "äpfel", 1)
	if err != nil || len(response.Results) != 1 || response.Results[0].Name != "ÄPFEL.jar" {
		t.Fatalf("Unicode case-insensitive match lost: %+v, %v", response, err)
	}
}

func BenchmarkFileRepositorySearch(b *testing.B) {
	state := core.NewAppState()
	state.Inner.FileIndex = index.NewFileIndex()
	repo := &config.Repository{Name: "files", Format: config.RepositoryFormatFiles, Visibility: "PUBLIC"}
	for i := range 10000 {
		path := filepath.Join("storage", repo.Name, fmt.Sprintf("group-%02d", i%100), fmt.Sprintf("demo-package-%05d.jar", i))
		state.Inner.FileIndex.EnsureParentDirs(path)
		state.Inner.FileIndex.InsertFile(path, index.FileInfo{Size: 1024})
	}
	viewer := &config.User{Username: "guest"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := searchFileTreeRepository(state, "storage", repo, viewer, "demo", 20); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSearchClassicMavenRepositoryUsesIndexAndOmitsBlockedFiles(t *testing.T) {
	storagePath := testutil.TempDir(t)
	repo := &config.Repository{Name: "releases", Format: config.RepositoryFormatMavenClassic, Visibility: "PUBLIC"}
	state := core.NewAppState()
	initMavenReadTestDB(t, state)
	state.Inner.FileIndex = index.NewFileIndex()
	root := filepath.Join(storagePath, repo.Name)
	artifact := filepath.Join(root, "org", "example", "demo", "1.0.0", "demo-1.0.0.jar")
	blocked := filepath.Join(root, "org", "example", "demo", "1.0.0", "demo-1.0.0.pom")
	state.Inner.FileIndex.EnsureParentDirs(artifact)
	state.Inner.FileIndex.InsertFile(artifact, index.FileInfo{Size: 123, ModTime: 1})
	state.Inner.FileIndex.InsertFile(blocked, index.FileInfo{Size: 45, ModTime: 1})
	state.Inner.FileIndex.BlockFile(blocked)

	response, err := searchFileTreeRepository(state, storagePath, repo, &config.User{Username: "guest", Roles: []string{"base"}}, "demo", 20)
	if err != nil {
		t.Fatal(err)
	}
	if response.Format != config.RepositoryFormatMavenClassic || response.Total != 3 {
		t.Fatalf("unexpected Maven search metadata: %+v", response)
	}
	for _, result := range response.Results {
		if result.Path == "org/example/demo/1.0.0/demo-1.0.0.pom" {
			t.Fatal("blocked Maven file was exposed by repository search")
		}
	}
	foundArtifact := false
	for _, result := range response.Results {
		if result.Path == "org/example/demo/1.0.0/demo-1.0.0.jar" && result.Size == 123 {
			foundArtifact = true
		}
	}
	if !foundArtifact {
		t.Fatalf("Maven artifact missing from search results: %+v", response.Results)
	}
}

func TestSearchModernMavenRepositoryReturnsDomainAndArtifactRoutes(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "maven-search.db"), MaxOpenConns: 1, MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	state := core.NewAppState()
	state.Inner.DB = db
	if err := db.EnsureImportedMavenDomain(&core.MavenDomain{
		Domain: "com.example", VerificationType: "legacy", VerificationHost: "com.example",
		VerificationCode: "search-import", Verified: true, CreatedAt: 10, VerifiedAt: 11,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordMavenPublication(&core.MavenArtifact{
		Repository: "releases", Domain: "com.example", GroupID: "com.example.tools",
		ArtifactID: "demo", Description: "Demo artifact", LatestVersion: "1.2.3", CreatedAt: 12, UpdatedAt: 13,
	}, &core.MavenVersion{
		Repository: "releases", GroupID: "com.example.tools", ArtifactID: "demo",
		Version: "1.2.3", Size: 123, CreatedAt: 12,
	}); err != nil {
		t.Fatal(err)
	}

	response, err := searchModernMavenRepository(state,
		&config.Repository{Name: "releases", Format: config.RepositoryFormatMaven}, &config.User{Username: "guest"}, "example", 20)
	if err != nil {
		t.Fatal(err)
	}
	if response.Format != config.RepositoryFormatMaven || response.Total != 2 || len(response.Results) != 2 {
		t.Fatalf("unexpected modern Maven search response: %+v", response)
	}
	paths := make(map[string]string, len(response.Results))
	for _, result := range response.Results {
		paths[result.Path] = result.Type
	}
	if paths["domains/com.example"] != "DOMAIN" {
		t.Fatalf("modern Maven domain route missing: %+v", response.Results)
	}
	if paths["packages/com.example.tools/demo"] != "PACKAGE" {
		t.Fatalf("modern Maven artifact route missing: %+v", response.Results)
	}
}

func TestSearchCargoRepositoryReturnsNavigablePublicPackage(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "search.db"), MaxOpenConns: 1, MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	state := core.NewAppState()
	state.Inner.DB = db
	if err := db.RecordCargoPublication(
		&core.CargoPackage{Repository: "cargo", Name: "renop-demo", NormalizedName: "renop-demo", Description: "demo crate", CreatedAt: 1, UpdatedAt: 1},
		&core.CargoVersion{Repository: "cargo", Package: "renop-demo", Version: "1.2.3", Publisher: "alice", CreatedAt: 1},
		"alice",
	); err != nil {
		t.Fatal(err)
	}

	response, err := searchCargoRepository(state, &config.User{Username: "guest"},
		&config.Repository{Name: "cargo", Format: config.RepositoryFormatCargo},
		"demo", 20)
	if err != nil {
		t.Fatal(err)
	}
	if response.Total != 1 || len(response.Results) != 1 {
		t.Fatalf("unexpected Cargo search response: %+v", response)
	}
	result := response.Results[0]
	if result.Name != "renop-demo" || result.Path != "packages/renop-demo" ||
		result.Type != "PACKAGE" || result.LatestVersion != "1.2.3" {
		t.Fatalf("unexpected Cargo search result: %+v", result)
	}
}

func TestSearchNPMRepositoryFiltersPrivatePackagesAndBuildsPackageRoutes(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "npm-search.db"), MaxOpenConns: 1, MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.SaveToken(&core.AccessToken{Name: "alice", Permissions: []string{"base"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateNPMPackage("npm", "demo-library", "alice", false, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateNPMPackage("npm", "@team/private-demo", "alice", true, 2); err != nil {
		t.Fatal(err)
	}
	state := core.NewAppState()
	state.Inner.DB = db
	repo := &config.Repository{Name: "npm", Format: config.RepositoryFormatNPM}
	guestResponse, err := searchNPMRepository(state, repo,
		&config.User{Username: "guest", Roles: []string{"base"}}, "demo", 20)
	if err != nil {
		t.Fatal(err)
	}
	if guestResponse.Total != 1 || len(guestResponse.Results) != 1 ||
		guestResponse.Results[0].Path != "packages/demo-library" {
		t.Fatalf("unexpected public npm search response: %+v", guestResponse)
	}
	aliceResponse, err := searchNPMRepository(state, repo,
		&config.User{Username: "alice", Roles: []string{"base"}}, "demo", 20)
	if err != nil {
		t.Fatal(err)
	}
	if aliceResponse.Total != 2 || len(aliceResponse.Results) != 2 {
		t.Fatalf("npm owner could not search private package: %+v", aliceResponse)
	}
}

func TestContainsFold(t *testing.T) {
	cases := []struct {
		s      string
		needle string
		want   bool
	}{
		{"com/example/MyArtifact/1.0/MyArtifact-1.0.jar", "myartifact", true},
		{"com/example/MyArtifact/1.0/MyArtifact-1.0.jar", "MYARTIFACT", false},
		{"com/example/foo/bar.jar", "bar", true},
		{"com/example/foo/bar.jar", "baz", false},
		{"short", "longerneedle", false},
		{"exact", "exact", true},
		{"", "a", false},
		{"anything", "", true},
	}
	for _, c := range cases {
		got := containsFold(c.s, c.needle)
		if got != c.want {
			t.Errorf("containsFold(%q, %q) = %v, want %v", c.s, c.needle, got, c.want)
		}
	}
}

func TestRepositorySearchRank(t *testing.T) {
	if r := repositorySearchRank("exact", "exact"); r != 0 {
		t.Errorf("expected 0 for exact match, got %d", r)
	}
	if r := repositorySearchRank("Exact", "exact"); r != 0 {
		t.Errorf("expected 0 for case-insensitive exact match, got %d", r)
	}
	if r := repositorySearchRank("exact-prefix", "exact"); r != 1 {
		t.Errorf("expected 1 for prefix match, got %d", r)
	}
	if r := repositorySearchRank("other", "exact"); r != 2 {
		t.Errorf("expected 2 for other match, got %d", r)
	}
}

func TestSearchNativeRepositoryReturnsResourcePathAndAccessFilter(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "native-search.db"),
		MaxOpenConns: 1, MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := int64(1000)
	if err := db.SaveToken(&core.AccessToken{Name: "alice", Permissions: []string{"base", "canupdate:apk-repo"}}); err != nil {
		t.Fatal(err)
	}
	res1, err := db.CreateNativeResource("apk-repo", config.RepositoryFormatAPK, "alpine-base", "alice", now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.CreateNativeResource("apk-repo", config.RepositoryFormatAPK, "alpine-extra", "alice", now)
	if err != nil {
		t.Fatal(err)
	}
	// res1 published, res2 reserved only
	if err := db.SaveNativeArtifact(core.NativeArtifact{
		Repository: "apk-repo", ResourceID: res1.ID, Name: res1.Name, Version: "1.0",
		Path: "x86_64/alpine-base-1.0.apk", Size: 100, CreatedAt: now,
	}, "alice", true); err != nil {
		t.Fatal(err)
	}

	state := core.NewAppState()
	state.Inner.DB = db
	repo := &config.Repository{Name: "apk-repo", Format: config.RepositoryFormatAPK}

	// Guest should only find published resource
	guestResp, err := searchNativeRepository(state, repo, &config.User{Username: "guest"}, "alpine", 10)
	if err != nil {
		t.Fatal(err)
	}
	if guestResp.Total != 1 || len(guestResp.Results) != 1 || guestResp.Results[0].Path != "~/alpine-base" {
		t.Fatalf("unexpected guest search response: %+v", guestResp)
	}

	// Alice (owner) should find both published and reserved
	aliceResp, err := searchNativeRepository(state, repo, &config.User{Username: "alice"}, "alpine", 10)
	if err != nil {
		t.Fatal(err)
	}
	if aliceResp.Total != 2 || len(aliceResp.Results) != 2 {
		t.Fatalf("unexpected alice search response: %+v", aliceResp)
	}
}
