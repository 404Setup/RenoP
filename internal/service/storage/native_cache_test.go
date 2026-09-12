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
	"io"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"renop/internal/artifactstore"
	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestNativeIndexCacheRejectsSameMetadataReplacement(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	repo := &config.Repository{Name: "native", Format: config.RepositoryFormatAPK}
	cfg.Maven.Repositories[repo.Name] = repo
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	InitS3(cfg)
	root := filepath.Join(cfg.StoragePath, repo.Name)
	filename := filepath.Join(root, "test.apk")
	mustWriteIndexed(t, state, filename, []byte("old"))
	info, ok := state.Inner.FileIndex.GetFileInfo(filename)
	require.True(t, ok)
	app := fiber.New()
	app.Get("/index", func(c fiber.Ctx) error {
		_, err := serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{root: root, contentType: "text/plain", accept: func(string) bool { return true },
			parse: func(reader io.ReaderAt, size int64, _ string) ([]byte, error) {
				return io.ReadAll(io.NewSectionReader(reader, 0, size))
			},
			render: func(records [][]byte) ([]byte, error) { return records[0], nil }})
		return err
	})
	get := func() string {
		response, err := app.Test(httptest.NewRequest("GET", "/index", nil))
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode)
		return string(body)
	}
	require.Equal(t, "old", get())
	require.NoError(t, artifactstore.DefaultDisk.WriteFile(filename, []byte("new"), 0644))
	// Object stores and restored filesystem timestamps can report the same size
	// and timestamp for two different versions of one logical path.
	state.Inner.FileIndex.InsertFile(filename, info)
	require.Equal(t, "new", get())
}
