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
	"bytes"
	"crypto/sha256"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"
	"renop/internal/service/nativesign"
	"renop/pkg/hex"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/gofiber/fiber/v3"
)

func TestNativeIndexSigningRoutes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	for _, format := range []string{"apt", "apk", "rpm"} {
		cfg.Maven.Repositories[format] = &config.Repository{Name: format, Format: format, Visibility: "PUBLIC"}
	}
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	if err := nativesign.EnsureKeys(state, filepath.Join(storageTestTempDir(t), "settings.db")); err != nil {
		t.Fatal(err)
	}
	InitS3(state.Inner.Config.Load())
	app := fiber.New()
	SetupRoutes(app, state)
	get := func(path string, status int) []byte {
		t.Helper()
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status {
			t.Fatalf("%s: %d %s", path, response.StatusCode, data)
		}
		return data
	}
	public := get("/apt/renop.asc", 200)
	ring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(public))
	if err != nil {
		t.Fatal(err)
	}
	release := get("/apt/dists/stable/Release", 200)
	detached := get("/apt/dists/stable/Release.gpg", 200)
	if _, err := openpgp.CheckArmoredDetachedSignature(ring, bytes.NewReader(release), bytes.NewReader(detached), nil); err != nil {
		t.Fatal(err)
	}
	clear, rest := clearsign.Decode(get("/apt/dists/stable/InRelease", 200))
	if clear == nil || len(rest) != 0 {
		t.Fatal("invalid InRelease")
	}
	if _, err := openpgp.CheckDetachedSignature(ring, bytes.NewReader(clear.Bytes), clear.ArmoredSignature.Body, nil); err != nil {
		t.Fatal(err)
	}
	rpm := get("/rpm/repodata/repomd.xml", 200)
	rpmSignature := get("/rpm/repodata/repomd.xml.asc", 200)
	if _, err := openpgp.CheckArmoredDetachedSignature(ring, bytes.NewReader(rpm), bytes.NewReader(rpmSignature), nil); err != nil {
		t.Fatal(err)
	}
	get("/rpm/repodata/repomd.xml.key", 200)
	get("/apk/renop.rsa.pub", 200)
	get("/apk/x86_64/APKINDEX.tar.gz", 200)
	// A generated Release must describe the uploaded representation clients get.
	manualPackages := []byte("Package: uploaded\nVersion: 2\nArchitecture: amd64\n\n")
	mustWriteIndexed(t, state, filepath.Join(cfg.StoragePath, "apt", "dists", "stable", "main", "binary-amd64", "Packages"), manualPackages)
	manualHash := sha256.Sum256(manualPackages)
	releaseWithUpload := string(get("/apt/dists/stable/Release", 200))
	if !strings.Contains(releaseWithUpload, hex.EncodeToString(manualHash[:])) || strings.Contains(releaseWithUpload, "main/binary-amd64/Packages.gz") {
		t.Fatal("Release does not match uploaded Packages")
	}
	get("/apt/dists/stable/main/binary-amd64/Packages.gz", 404)
	// Publisher metadata stays byte-for-byte intact and receives no synthetic signature.
	for _, file := range []struct{ path, body string }{
		{"apt/dists/stable/Release", "publisher release"},
		{"rpm/repodata/repomd.xml", "publisher repomd"},
		{"apk/x86_64/APKINDEX.tar.gz", "publisher apk index"},
	} {
		mustWriteIndexed(t, state, filepath.Join(cfg.StoragePath, filepath.FromSlash(file.path)), []byte(file.body))
		if data := get("/"+file.path, 200); string(data) != file.body {
			t.Fatalf("replaced %s", file.path)
		}
	}
	get("/apt/dists/stable/InRelease", 404)
	get("/apt/dists/stable/Release.gpg", 404)
	get("/rpm/repodata/repomd.xml.asc", 404)
}
