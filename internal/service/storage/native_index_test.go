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
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"

	"github.com/gofiber/fiber/v3"
)

func TestNativeIndexRejectsMutationDuringRecordLoad(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	repo := &config.Repository{Name: "native", Format: config.RepositoryFormatAPK}
	cfg.Maven.Repositories[repo.Name] = repo
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	InitS3(cfg)
	root := filepath.Join(cfg.StoragePath, repo.Name)
	filename := filepath.Join(root, "package.apk")
	mustWriteIndexed(t, state, filename, []byte("package"))
	rendered := false
	app := fiber.New()
	app.Get("/index", func(c fiber.Ctx) error {
		_, err := serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{
			root: root, contentType: "application/octet-stream", accept: func(string) bool { return true },
			parse: func(io.ReaderAt, int64, string) ([]byte, error) {
				state.Inner.FileIndex.RemoveFile(filename)
				return []byte(`{}`), nil
			},
			render: func([][]byte) ([]byte, error) { rendered = true; return nil, nil },
		})
		return err
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/index", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 503 || rendered {
		t.Fatalf("served changed snapshot: status=%d rendered=%v", response.StatusCode, rendered)
	}
}

func TestNativeIndexRejectsOversizedRecordBeforeRendering(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	repo := &config.Repository{Name: "native", Format: config.RepositoryFormatAPK}
	cfg.Maven.Repositories[repo.Name] = repo
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	InitS3(cfg)
	root := filepath.Join(cfg.StoragePath, repo.Name)
	mustWriteIndexed(t, state, filepath.Join(root, "package.apk"), []byte("package"))
	rendered := false
	app := fiber.New()
	app.Get("/index", func(c fiber.Ctx) error {
		_, err := serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{
			root: root, contentType: "application/octet-stream", accept: func(string) bool { return true },
			parse:  func(io.ReaderAt, int64, string) ([]byte, error) { return bytes.Repeat([]byte{'x'}, (32<<20)+1), nil },
			render: func([][]byte) ([]byte, error) { rendered = true; return nil, nil },
		})
		return err
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/index", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 503 || rendered {
		t.Fatalf("rendered oversized index: status=%d rendered=%v", response.StatusCode, rendered)
	}
}
