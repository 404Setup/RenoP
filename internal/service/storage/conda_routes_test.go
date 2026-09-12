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
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/klauspost/compress/zip"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/service/index"
	"renop/pkg/hex"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/klauspost/compress/zstd"
)

func condaStoragePackage(t *testing.T) []byte {
	t.Helper()
	var metadata bytes.Buffer
	encoder, err := zstd.NewWriter(&metadata, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(encoder)
	data := []byte(`{"name":"example","version":"1.0","build":"0","build_number":0,"subdir":"noarch","depends":[]}`)
	if err := archive.WriteHeader(&tar.Header{Name: "info/index.json", Mode: 0644, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(data); err != nil {
		t.Fatal(err)
	}
	payload := []byte("example\n")
	sum := sha256.Sum256(payload)
	paths, err := json.Marshal(map[string]any{"paths_version": 1, "paths": []map[string]any{{"_path": "share/example.txt", "path_type": "hardlink", "sha256": hex.EncodeToString(sum[:]), "size_in_bytes": len(payload)}}})
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"info/files": []byte("share/example.txt\n"), "info/paths.json": paths} {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	zipWriter := zip.NewWriter(&body)
	member, err := zipWriter.CreateHeader(&zip.FileHeader{Name: "info-example.tar.zst", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write(metadata.Bytes()); err != nil {
		t.Fatal(err)
	}
	member, err = zipWriter.CreateHeader(&zip.FileHeader{Name: "metadata.json", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write([]byte(`{"conda_pkg_format_version":2}`)); err != nil {
		t.Fatal(err)
	}
	var packageData bytes.Buffer
	packageEncoder, err := zstd.NewWriter(&packageData, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	packageTar := tar.NewWriter(packageEncoder)
	if err := packageTar.WriteHeader(&tar.Header{Name: "share/example.txt", Mode: 0644, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := packageTar.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := packageTar.Close(); err != nil {
		t.Fatal(err)
	}
	if err := packageEncoder.Close(); err != nil {
		t.Fatal(err)
	}
	member, err = zipWriter.CreateHeader(&zip.FileHeader{Name: "pkg-example-1.0-0.tar.zst", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write(packageData.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestCondaNativeIndexUploadDownloadAndDelete(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	repo := &config.Repository{Name: "channel", Format: config.RepositoryFormatCondaNative, Visibility: "PUBLIC"}
	cfg.Maven.Repositories[repo.Name] = repo
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(storageTestTempDir(t), "native.db"), MaxOpenConns: 1, MaxIdleConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	state.Inner.DB = db
	if err := db.SaveToken(&core.AccessToken{Name: "writer", Permissions: []string{"base", "canupdate:channel"}}); err != nil {
		t.Fatal(err)
	}
	InitS3(cfg)
	app := fiber.New(fiber.Config{StreamRequestBody: true})
	app.Use(func(c fiber.Ctx) error {
		if c.Get("X-Test-Writer") == "yes" {
			c.Locals("user", &config.User{Username: "writer", Roles: []string{"base", "canupdate:channel"}})
		}
		return c.Next()
	})
	SetupRoutes(app, state)
	request := func(method, target string, data []byte, writer bool) (int, []byte, http.Header) {
		t.Helper()
		req := httptest.NewRequest(method, target, bytes.NewReader(data))
		if writer {
			req.Header.Set("X-Test-Writer", "yes")
		}
		response, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body, response.Header
	}
	const indexURL = "/channel/noarch/repodata.json"
	const packageURL = "/channel/noarch/example-1.0-0.conda"
	if code, body, _ := request(http.MethodGet, indexURL, nil, false); code != 200 || !bytes.Contains(body, []byte(`"packages.conda":{}`)) {
		t.Fatalf("empty index: %d %s", code, body)
	}
	archive := condaStoragePackage(t)
	if code, _, _ := request(http.MethodPut, packageURL, archive, false); code != 403 {
		t.Fatalf("anonymous PUT: %d", code)
	}
	if code, _, _ := request(http.MethodPut, packageURL, archive, true); code != 404 {
		t.Fatalf("unreserved resource PUT: %d", code)
	}
	if _, err := db.CreateNativeResource(repo.Name, repo.Engine().Protocol, "example", "writer", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if code, body, _ := request(http.MethodPut, packageURL, archive, true); code != 201 {
		t.Fatalf("PUT: %d %s", code, body)
	}
	if code, _, _ := request(http.MethodPut, packageURL, archive, true); code != 409 {
		t.Fatalf("redeployment: %d", code)
	}
	code, body, headers := request(http.MethodGet, indexURL, nil, false)
	var document struct {
		Packages map[string]json.RawMessage `json:"packages.conda"`
	}
	if code != 200 || json.Unmarshal(body, &document) != nil || len(document.Packages) != 1 {
		t.Fatalf("generated index: %d %s", code, body)
	}
	if headers.Get("ETag") == "" {
		t.Fatal("missing generated ETag")
	}
	if code, body, _ := request(http.MethodGet, packageURL, nil, false); code != 200 || !bytes.Equal(body, archive) {
		t.Fatalf("download changed bytes: %d", code)
	}
	if code, _, _ := request(http.MethodPut, "/channel/noarch/invalid.conda", []byte("invalid"), true); code != 400 {
		t.Fatalf("invalid package: %d", code)
	}
	if _, err := os.Stat(filepath.Join(cfg.StoragePath, "channel", "noarch", "invalid.conda")); !os.IsNotExist(err) {
		t.Fatalf("invalid upload installed: %v", err)
	}
	// A publisher cannot replace generated indexes and bypass publication policy.
	for _, manual := range [][]byte{[]byte("{\n  \"packages\": {}\n}\n"), []byte("{\"packages.conda\":{}}\n")} {
		if code, body, _ := request(http.MethodPut, indexURL, manual, true); code != 400 {
			t.Fatalf("metadata PUT: %d %s", code, body)
		}
		if code, body, _ := request(http.MethodGet, indexURL, nil, false); code != 200 || !bytes.Contains(body, []byte("example-1.0-0.conda")) {
			t.Fatalf("generated metadata was replaced: %d %q", code, body)
		}
	}
	if code, body, _ := request(http.MethodDelete, indexURL, nil, true); code != 404 {
		t.Fatalf("delete generated metadata: %d %s", code, body)
	}
	if code, body, _ := request(http.MethodDelete, packageURL, nil, true); code != 200 && code != 204 {
		t.Fatalf("delete package: %d %s", code, body)
	}
	if code, body, _ := request(http.MethodGet, indexURL, nil, false); code != 200 || !bytes.Contains(body, []byte(`"packages.conda":{}`)) {
		t.Fatalf("stale index after deletion: %d %s", code, body)
	}
}
