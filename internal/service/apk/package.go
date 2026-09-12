/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package apk reads Alpine APK package metadata and emits native repository indexes.
package apk

import (
	"archive/tar"
	"bufio"
	"crypto/sha1"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/emmansun/base64"
	"github.com/klauspost/compress/gzip"
)

const (
	MaxPackageBytes     = int64(8 << 30)
	maxControlBytes     = 64 << 20
	maxPackageInfoBytes = 1 << 20
)

var ErrInvalidPackage = errors.New("invalid Alpine APK package metadata")

type Package struct {
	Filename string
	Fields   map[string]string
}

// ReadPackage hashes the compressed control member, rather than the complete APK.
// APK v2 consists of independent signature, control and payload gzip streams.
func ReadPackage(reader io.ReaderAt, size int64, filename string) (*Package, error) {
	if size <= 0 || size > MaxPackageBytes || path.Base(filename) != filename || !strings.HasSuffix(filename, ".apk") {
		return nil, ErrInvalidPackage
	}
	section := io.NewSectionReader(reader, 0, size)
	buffered := bufio.NewReader(section)
	for range 8 {
		position, _ := section.Seek(0, io.SeekCurrent)
		start := position - int64(buffered.Buffered())
		compressed, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, ErrInvalidPackage
		}
		compressed.Multistream(false)
		bounded := &io.LimitedReader{R: compressed, N: maxControlBytes + 1}
		archive := tar.NewReader(bounded)
		var metadata []byte
		for range 4096 {
			header, err := archive.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				compressed.Close()
				return nil, ErrInvalidPackage
			}
			if strings.TrimPrefix(header.Name, "./") != ".PKGINFO" {
				continue
			}
			if metadata != nil || !header.FileInfo().Mode().IsRegular() || header.Size <= 0 || header.Size > maxPackageInfoBytes {
				compressed.Close()
				return nil, ErrInvalidPackage
			}
			metadata, err = io.ReadAll(io.LimitReader(archive, maxPackageInfoBytes+1))
			if err != nil {
				compressed.Close()
				return nil, err
			}
		}
		_, err = io.Copy(io.Discard, bounded)
		compressed.Close()
		if err != nil || bounded.N <= 0 {
			return nil, ErrInvalidPackage
		}
		position, _ = section.Seek(0, io.SeekCurrent)
		end := position - int64(buffered.Buffered())
		if metadata == nil {
			continue
		}
		if end >= size {
			return nil, ErrInvalidPackage
		}
		fields, err := parsePackageInfo(metadata)
		if err != nil {
			return nil, err
		}
		if filename != fields["P"]+"-"+fields["V"]+".apk" {
			return nil, ErrInvalidPackage
		}
		hash := sha1.New()
		if _, err := io.Copy(hash, io.NewSectionReader(reader, start, end-start)); err != nil {
			return nil, err
		}
		fields["C"] = "Q1" + base64.StdEncoding.EncodeToString(hash.Sum(nil))
		fields["S"] = strconv.FormatInt(size, 10)
		return &Package{Filename: filename, Fields: fields}, nil
	}
	return nil, ErrInvalidPackage
}

func parsePackageInfo(data []byte) (map[string]string, error) {
	fields := make(map[string]string)
	names := map[string]string{
		"pkgname": "P", "pkgver": "V", "arch": "A", "size": "I", "pkgdesc": "T", "url": "U", "license": "L",
		"origin": "o", "maintainer": "m", "builddate": "t", "commit": "c", "provider_priority": "k",
		"depend": "D", "provides": "p", "install_if": "i",
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok || strings.ContainsAny(value, "\x00\r") {
			return nil, ErrInvalidPackage
		}
		field := names[key]
		if field == "" {
			continue
		}
		if old, exists := fields[field]; exists {
			if field != "D" && field != "p" && field != "i" {
				return nil, ErrInvalidPackage
			}
			value = old + " " + value
		}
		fields[field] = value
	}
	for _, field := range []string{"P", "V", "A"} {
		if fields[field] == "" || len(fields[field]) > 1024 || strings.ContainsAny(fields[field], " \t/\\") {
			return nil, ErrInvalidPackage
		}
	}
	for _, field := range []string{"I", "t", "k"} {
		if value, exists := fields[field]; exists {
			if _, err := strconv.ParseUint(value, 10, 63); err != nil {
				return nil, ErrInvalidPackage
			}
		}
	}
	if fields["I"] == "" {
		fields["I"] = "0"
	}
	return fields, nil
}
