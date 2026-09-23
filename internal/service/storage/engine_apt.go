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
	"renop/internal/service/apt"
	"renop/internal/service/nativesign"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
)

func aptEngine() repositoryEngine {
	engine := managedNativeEngine()
	engine.handle = handleAPTIndex
	return engine
}

func handleAPTIndex(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, relative string, concrete bool) (bool, error) {
	if !isRepositoryRead(c) || concrete {
		return false, nil
	}
	if relative == "renop.asc" {
		return serveNativePublicKey(c, state, false)
	}
	parts := strings.Split(relative, "/")
	suite, component, architecture := "", "", ""
	filename := path.Base(relative)
	isRelease := false
	switch {
	case len(parts) == 3 && parts[0] == "dists" && (filename == "Release" || filename == "InRelease" || filename == "Release.gpg"):
		suite, isRelease = parts[1], true
	case len(parts) == 5 && parts[0] == "dists" && strings.HasPrefix(parts[3], "binary-") && (filename == "Packages" || filename == "Packages.gz"):
		suite, component, architecture = parts[1], parts[2], strings.TrimPrefix(parts[3], "binary-")
		if architecture == "" {
			return false, nil
		}
	case relative == "Packages" || relative == "Packages.gz":
	default:
		return false, nil
	}
	root := filepath.Join(storagePath, repo.Name)
	if _, exists, err := backendFor(root).Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil || exists {
		return false, err
	}
	// Uploaded release documents own their signatures; never sign a different
	// generated document alongside a publisher's release.
	if isRelease && filename != "Release" {
		for _, sibling := range []string{"Release", "InRelease"} {
			if _, exists, err := backendFor(root).Stat(filepath.Join(root, "dists", suite, sibling)); err != nil || exists {
				if err != nil {
					return true, c.SendStatus(fiber.StatusServiceUnavailable)
				}
				return true, c.SendStatus(fiber.StatusNotFound)
			}
		}
	}
	if filename == "Packages.gz" {
		if _, exists, err := backendFor(root).Stat(filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(relative, ".gz")))); err != nil || exists {
			if err != nil {
				return true, c.SendStatus(fiber.StatusServiceUnavailable)
			}
			return true, c.SendStatus(fiber.StatusNotFound)
		}
	}
	contentType := "text/plain; charset=utf-8"
	if strings.HasSuffix(filename, ".gz") {
		contentType = "application/gzip"
	}
	return serveGeneratedNativeIndex(c, state, repo, nativeIndexSpec{
		root: root, contentType: contentType,
		accept: func(relative string) bool { return strings.HasSuffix(relative, ".deb") },
		parse: func(reader io.ReaderAt, size int64, relative string) ([]byte, error) {
			pkg, err := apt.ReadPackage(reader, size, relative)
			if err != nil {
				return nil, err
			}
			return json.Marshal(pkg)
		},
		render: func(records [][]byte) ([]byte, error) {
			packages := make([]*apt.Package, 0, len(records))
			for _, record := range records {
				var pkg apt.Package
				if err := json.Unmarshal(record, &pkg); err != nil {
					return nil, err
				}
				packages = append(packages, &pkg)
			}
			if isRelease {
				data, err := apt.ReleaseWithIndexes(packages, suite, func(filename string, generated []byte) ([]byte, bool, error) {
					return resolveAPTIndex(filepath.Join(root, "dists", suite, filepath.FromSlash(filename)), generated)
				})
				if err != nil {
					return nil, err
				}
				switch filename {
				case "InRelease":
					return nativesign.Cleartext(state.Inner.Config.Load().NativeSigningKeys, data)
				case "Release.gpg":
					return nativesign.Detached(state.Inner.Config.Load().NativeSigningKeys, data)
				}
				return data, nil
			}
			data, err := apt.PackageIndex(packages, suite, component, architecture)
			if err != nil {
				return nil, err
			}
			if strings.HasSuffix(filename, ".gz") {
				return apt.Gzip(data)
			}
			return data, nil
		},
	})
}

func resolveAPTIndex(filename string, generated []byte) ([]byte, bool, error) {
	backend := backendFor(filename)
	reader, size, exists, err := backend.Open(filename)
	if err != nil {
		return nil, false, err
	}
	if exists {
		defer reader.Close()
		if size < 0 || size > 32<<20 {
			return nil, false, apt.ErrInvalidPackage
		}
		data, err := io.ReadAll(io.LimitReader(reader, size+1))
		if err != nil {
			return nil, false, err
		}
		if int64(len(data)) != size {
			return nil, false, apt.ErrInvalidPackage
		}
		return data, true, nil
	}
	if before, ok := strings.CutSuffix(filename, ".gz"); ok {
		_, plainExists, err := backend.Stat(before)
		if err != nil || plainExists {
			return nil, false, err
		}
	}
	return generated, true, nil
}
