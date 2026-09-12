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
	"path"
	"path/filepath"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/conda"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/klauspost/compress/zstd"
)

func condaEngine() repositoryEngine {
	engine := managedNativeEngine()
	engine.handle = handleCondaIndex
	return engine
}

func handleCondaIndex(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, relative string, concrete bool) (bool, error) {
	if !isRepositoryRead(c) || concrete {
		return false, nil
	}
	parts := strings.Split(relative, "/")
	if len(parts) != 2 {
		return false, nil
	}
	subdir, filename := parts[0], parts[1]
	base := strings.TrimSuffix(filename, ".zst")
	if base != "repodata.json" && base != "current_repodata.json" && base != "repodata_from_packages.json" {
		return false, nil
	}
	root := filepath.Join(storagePath, repo.Name, subdir)
	if _, exists, err := backendFor(root).Stat(filepath.Join(root, filename)); err != nil || exists {
		return false, err
	}
	// A publisher-supplied plain index may contain repodata patches. Never serve
	// an automatically generated compressed variant with different semantics.
	if base != filename {
		if _, exists, err := backendFor(root).Stat(filepath.Join(root, base)); err != nil || exists {
			if err != nil {
				return true, c.SendStatus(fiber.StatusServiceUnavailable)
			}
			return true, c.SendStatus(fiber.StatusNotFound)
		}
	}
	contentType := "application/json"
	if strings.HasSuffix(filename, ".zst") {
		contentType = "application/zstd"
	}
	return serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{
		root:        root,
		contentType: contentType,
		accept: func(relative string) bool {
			return !strings.Contains(relative, "/") && (strings.HasSuffix(relative, ".conda") || strings.HasSuffix(relative, ".tar.bz2"))
		},
		parse: func(reader io.ReaderAt, size int64, relative string) ([]byte, error) {
			pkg, err := conda.ReadPackage(reader, size, path.Base(relative))
			if err != nil {
				return nil, err
			}
			return json.Marshal(pkg)
		},
		render: func(records [][]byte) ([]byte, error) {
			packages := make([]*conda.Package, 0, len(records))
			for _, record := range records {
				var pkg conda.Package
				if err := json.Unmarshal(record, &pkg); err != nil {
					return nil, err
				}
				packages = append(packages, &pkg)
			}
			data, err := conda.Repodata(subdir, packages)
			if err != nil {
				return nil, err
			}
			if strings.HasSuffix(filename, ".zst") {
				encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
				if err != nil {
					return nil, err
				}
				defer encoder.Close()
				data = encoder.EncodeAll(data, nil)
			}
			return data, nil
		},
	})
}
