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
	"path/filepath"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/nativesign"
	"renop/internal/service/rpm"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
)

func rpmEngine() repositoryEngine {
	engine := managedNativeEngine()
	engine.handle = func(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, relative string, concrete bool) (bool, error) {
		if !isRepositoryRead(c) || concrete || !strings.HasPrefix(relative, "repodata/") {
			return false, nil
		}
		if relative == "repodata/repomd.xml.key" {
			return serveNativePublicKey(c, state, false)
		}
		if relative != "repodata/repomd.xml" && relative != "repodata/repomd.xml.asc" && !strings.HasSuffix(relative, "-primary.xml.gz") && !strings.HasSuffix(relative, "-filelists.xml.gz") && !strings.HasSuffix(relative, "-other.xml.gz") {
			return false, nil
		}
		root := filepath.Join(storagePath, repo.Name)
		if _, exists, err := backendFor(root).Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil || exists {
			return false, err
		}
		// A manually supplied root document controls all referenced metadata.
		if relative != "repodata/repomd.xml" {
			if _, exists, err := backendFor(root).Stat(filepath.Join(root, "repodata", "repomd.xml")); err != nil || exists {
				if err != nil {
					return true, c.SendStatus(fiber.StatusServiceUnavailable)
				}
				return false, nil
			}
		}
		contentType := "application/xml"
		if strings.HasSuffix(relative, ".gz") {
			contentType = "application/gzip"
		}
		if strings.HasSuffix(relative, ".asc") {
			contentType = "application/pgp-signature"
		}
		return serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{
			root: root, contentType: contentType,
			accept: func(relative string) bool { return strings.HasSuffix(relative, ".rpm") },
			parse: func(reader io.ReaderAt, size int64, relative string) ([]byte, error) {
				pkg, err := rpm.ReadPackage(reader, size, relative)
				if err != nil {
					return nil, err
				}
				return json.Marshal(pkg)
			},
			render: func(records [][]byte) ([]byte, error) {
				packages := make([]*rpm.Package, 0, len(records))
				for _, record := range records {
					var pkg rpm.Package
					if err := json.Unmarshal(record, &pkg); err != nil {
						return nil, err
					}
					packages = append(packages, &pkg)
				}
				documents, err := rpm.Index(packages)
				if err != nil {
					return nil, err
				}
				if relative == "repodata/repomd.xml.asc" {
					return nativesign.Detached(state.Inner.Config.Load().NativeSigningKeys, documents["repodata/repomd.xml"])
				}
				data, exists := documents[relative]
				if !exists {
					return nil, errNativeIndexNotFound
				}
				return data, nil
			},
		})
	}
	return engine
}
