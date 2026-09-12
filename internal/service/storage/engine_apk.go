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
	"renop/internal/service/apk"
	"renop/internal/service/nativesign"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
)

func apkEngine() repositoryEngine {
	engine := managedNativeEngine()
	engine.handle = func(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, relative string, concrete bool) (bool, error) {
		if !isRepositoryRead(c) || concrete {
			return false, nil
		}
		if relative == "renop.rsa.pub" {
			return serveNativePublicKey(c, state, true)
		}
		if path.Base(relative) != "APKINDEX.tar.gz" {
			return false, nil
		}
		root := filepath.Join(storagePath, repo.Name, filepath.FromSlash(path.Dir(relative)))
		if _, exists, err := backendFor(root).Stat(filepath.Join(root, "APKINDEX.tar.gz")); err != nil || exists {
			return false, err
		}
		return serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{
			root:        root,
			contentType: "application/gzip",
			accept: func(relative string) bool {
				return !strings.Contains(relative, "/") && strings.HasSuffix(relative, ".apk")
			},
			parse: func(reader io.ReaderAt, size int64, relative string) ([]byte, error) {
				pkg, err := apk.ReadPackage(reader, size, path.Base(relative))
				if err != nil {
					return nil, err
				}
				return json.Marshal(pkg)
			},
			render: func(records [][]byte) ([]byte, error) {
				packages := make([]*apk.Package, 0, len(records))
				for _, record := range records {
					var pkg apk.Package
					if err := json.Unmarshal(record, &pkg); err != nil {
						return nil, err
					}
					packages = append(packages, &pkg)
				}
				data, err := apk.Index(packages)
				if err != nil {
					return nil, err
				}
				return nativesign.APK(state.Inner.Config.Load().NativeSigningKeys, data)
			},
		})
	}
	return engine
}
