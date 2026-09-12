/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package conda implements Conda channel metadata without extracting package files.
package conda

import (
	"archive/tar"
	"compress/bzip2"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path"
	"renop/pkg/hex"
	"strings"

	"github.com/klauspost/compress/zip"

	"github.com/goccy/go-json"
	"github.com/klauspost/compress/zstd"
)

const (
	MaxPackageBytes         = int64(8 << 30)
	maxMetadataBytes        = 1 << 20
	maxMetadataArchiveBytes = 64 << 20
)

var ErrInvalidPackage = errors.New("invalid Conda package metadata")

type Package struct {
	Filename string
	Record   map[string]json.RawMessage
}

// ReadPackage supports both .tar.bz2 and .conda containers. Only the metadata
// member is decoded; package data is streamed solely to calculate wire hashes.
func ReadPackage(reader io.ReaderAt, size int64, filename string) (*Package, error) {
	if size <= 0 || size > MaxPackageBytes || path.Base(filename) != filename {
		return nil, ErrInvalidPackage
	}
	var data []byte
	var err error
	switch {
	case strings.HasSuffix(filename, ".tar.bz2"):
		data, err = readIndex(bzip2.NewReader(io.NewSectionReader(reader, 0, size)))
	case strings.HasSuffix(filename, ".conda"):
		data, err = readCondaIndex(reader, size)
	default:
		return nil, ErrInvalidPackage
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPackage, err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(data, &record); err != nil || len(record) == 0 {
		return nil, ErrInvalidPackage
	}
	for _, field := range []string{"name", "version", "build"} {
		var value string
		if json.Unmarshal(record[field], &value) != nil || value == "" || len(value) > 1024 || strings.ContainsAny(value, "\x00\r\n/\\") {
			return nil, ErrInvalidPackage
		}
	}
	var buildNumber int64
	if json.Unmarshal(record["build_number"], &buildNumber) != nil || buildNumber < 0 {
		return nil, ErrInvalidPackage
	}
	for _, field := range []string{"depends", "constrains"} {
		if raw, ok := record[field]; ok {
			var values []string
			if json.Unmarshal(raw, &values) != nil || len(values) > 4096 {
				return nil, ErrInvalidPackage
			}
		}
	}
	md5Hash, shaHash := md5.New(), sha256.New()
	n, err := io.Copy(io.MultiWriter(md5Hash, shaHash), io.NewSectionReader(reader, 0, size))
	if err != nil || n != size {
		return nil, ErrInvalidPackage
	}
	setRecordField(record, "size", size)
	setRecordField(record, "md5", hex.EncodeToString(md5Hash.Sum(nil)))
	setRecordField(record, "sha256", hex.EncodeToString(shaHash.Sum(nil)))
	return &Package{Filename: filename, Record: record}, nil
}

func setRecordField(record map[string]json.RawMessage, field string, value any) {
	record[field], _ = json.Marshal(value)
}

func readCondaIndex(reader io.ReaderAt, size int64) ([]byte, error) {
	archive, err := zip.NewReader(reader, size)
	if err != nil || len(archive.File) > 16 {
		return nil, ErrInvalidPackage
	}
	var info *zip.File
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "info-") && strings.HasSuffix(file.Name, ".tar.zst") && path.Base(file.Name) == file.Name {
			if info != nil || file.UncompressedSize64 > maxMetadataArchiveBytes {
				return nil, ErrInvalidPackage
			}
			info = file
		}
	}
	if info == nil {
		return nil, ErrInvalidPackage
	}
	compressed, err := info.Open()
	if err != nil {
		return nil, err
	}
	defer compressed.Close()
	decoder, err := zstd.NewReader(io.LimitReader(compressed, maxMetadataArchiveBytes+1), zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxMetadataArchiveBytes))
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return readIndex(decoder)
}

func readIndex(reader io.Reader) ([]byte, error) {
	archive := tar.NewReader(io.LimitReader(reader, maxMetadataArchiveBytes+1))
	for range 4096 {
		header, err := archive.Next()
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(header.Name, "./") != "info/index.json" {
			continue
		}
		if !header.FileInfo().Mode().IsRegular() || header.Size <= 0 || header.Size > maxMetadataBytes {
			return nil, ErrInvalidPackage
		}
		return io.ReadAll(io.LimitReader(archive, maxMetadataBytes+1))
	}
	return nil, ErrInvalidPackage
}
