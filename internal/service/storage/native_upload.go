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
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"renop/internal/config"
	"renop/internal/service/apk"
	"renop/internal/service/apt"
	"renop/internal/service/conda"
	"renop/internal/service/rpm"

	"github.com/goccy/go-json"
)

var ErrNativePackageInvalid = errors.New("native package metadata is invalid")

type nativePackageParser struct {
	accepts func(string) bool
	parse   func(io.ReaderAt, int64, string) (any, error)
}

var nativePackageParsers = map[string]nativePackageParser{
	config.RepositoryFormatConda: {
		accepts: func(name string) bool {
			return strings.HasSuffix(name, ".conda") || strings.HasSuffix(name, ".tar.bz2")
		},
		parse: func(reader io.ReaderAt, size int64, relative string) (any, error) {
			parts := strings.Split(relative, "/")
			if len(parts) != 2 {
				return nil, ErrNativePackageInvalid
			}
			pkg, err := conda.ReadPackage(reader, size, path.Base(relative))
			if err != nil {
				return nil, err
			}
			if _, err := conda.Repodata(parts[0], []*conda.Package{pkg}); err != nil {
				return nil, err
			}
			return pkg, nil
		},
	},
	config.RepositoryFormatAPK: {
		accepts: func(name string) bool { return strings.HasSuffix(name, ".apk") },
		parse: func(reader io.ReaderAt, size int64, relative string) (any, error) {
			return apk.ReadPackage(reader, size, path.Base(relative))
		},
	},
	config.RepositoryFormatAPT: {
		accepts: func(name string) bool { return strings.HasSuffix(name, ".deb") },
		parse: func(reader io.ReaderAt, size int64, relative string) (any, error) {
			return apt.ReadPackage(reader, size, relative)
		},
	},
	config.RepositoryFormatRPM: {
		accepts: func(name string) bool { return strings.HasSuffix(name, ".rpm") },
		parse: func(reader io.ReaderAt, size int64, relative string) (any, error) {
			return rpm.ReadPackage(reader, size, relative)
		},
	},
}

// prepareNativePackage validates archive metadata before replacing any installed
// bytes. Uploaded indexes and signatures are opaque and are never rewritten.
func prepareNativePackage(repo *config.Repository, relativePath, staged string) ([]byte, error) {
	relativePath = filepath.ToSlash(relativePath)
	parser, exists := nativePackageParsers[repo.Engine().Protocol]
	if !exists || !parser.accepts(relativePath) {
		return nil, nil
	}
	file, err := os.Open(staged)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	record, err := parser.parse(file, info.Size(), relativePath)
	if err != nil {
		return nil, ErrNativePackageInvalid
	}
	data, err := json.Marshal(record)
	if err != nil || len(data) > 1<<20 {
		return nil, ErrNativePackageInvalid
	}
	return data, nil
}
