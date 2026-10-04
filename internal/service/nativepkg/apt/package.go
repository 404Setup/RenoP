/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package apt implements Debian binary package metadata and repository indexes.
package apt

import (
	"archive/tar"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"io"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/klauspost/compress/gzip"

	"renop/pkg/hex"

	"github.com/klauspost/compress/zstd"
)

const (
	MaxPackageBytes     = int64(8 << 30)
	maxControlBytes     = 64 << 20
	maxControlFileBytes = 1 << 20
)

var ErrInvalidPackage = errors.New("invalid Debian package metadata")

type Package struct {
	Filename string
	Fields   map[string]string
}

func ReadPackage(reader io.ReaderAt, size int64, filename string) (*Package, error) {
	if size <= 0 || size > MaxPackageBytes || strings.ContainsAny(filename, "\x00\r\n") || !strings.HasSuffix(filename, ".deb") {
		return nil, ErrInvalidPackage
	}
	var magic [8]byte
	if _, err := reader.ReadAt(magic[:], 0); err != nil || string(magic[:]) != "!<arch>\n" {
		return nil, ErrInvalidPackage
	}
	position := int64(8)
	var control []byte
	dataFound := false
	for member := 0; member < 64 && position+60 <= size; member++ {
		var header [60]byte
		if _, err := reader.ReadAt(header[:], position); err != nil {
			return nil, err
		}
		if string(header[58:]) != "`\n" {
			return nil, ErrInvalidPackage
		}
		name := strings.TrimSuffix(strings.TrimSpace(string(header[:16])), "/")
		length, err := strconv.ParseInt(strings.TrimSpace(string(header[48:58])), 10, 64)
		if err != nil || length < 0 || length > size-position-60 {
			return nil, ErrInvalidPackage
		}
		position += 60
		section := io.NewSectionReader(reader, position, length)
		switch {
		case member == 0:
			if name != "debian-binary" || length < 4 || length > 64 {
				return nil, ErrInvalidPackage
			}
			version, err := io.ReadAll(section)
			if err != nil || !bytes.HasPrefix(version, []byte("2.")) {
				return nil, ErrInvalidPackage
			}
		case strings.HasPrefix(name, "_"):
		case strings.HasPrefix(name, "control.tar"):
			if control != nil || length > maxControlBytes {
				return nil, ErrInvalidPackage
			}
			control, err = readControl(section, length, strings.TrimPrefix(name, "control.tar"))
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(name, "data.tar"):
			if control == nil {
				return nil, ErrInvalidPackage
			}
			dataFound = true
		default:
			return nil, ErrInvalidPackage
		}
		if dataFound {
			break
		}
		position += length + length%2
	}
	if control == nil || !dataFound {
		return nil, ErrInvalidPackage
	}
	fields, err := parseControl(control)
	if err != nil {
		return nil, err
	}
	md5Hash, sha256Hash, sha512Hash := md5.New(), sha256.New(), sha512.New()
	n, err := io.Copy(io.MultiWriter(md5Hash, sha256Hash, sha512Hash), io.NewSectionReader(reader, 0, size))
	if err != nil || n != size {
		return nil, ErrInvalidPackage
	}
	fields["Filename"], fields["Size"] = filename, strconv.FormatInt(size, 10)
	fields["MD5sum"] = hex.EncodeToString(md5Hash.Sum(nil))
	fields["SHA256"] = hex.EncodeToString(sha256Hash.Sum(nil))
	fields["SHA512"] = hex.EncodeToString(sha512Hash.Sum(nil))
	return &Package{Filename: filename, Fields: fields}, nil
}

func readControl(reader io.ReaderAt, size int64, extension string) ([]byte, error) {
	var input io.Reader = io.NewSectionReader(reader, 0, size)
	switch extension {
	case "":
	case ".gz":
		decoder, err := gzip.NewReader(input)
		if err != nil {
			return nil, ErrInvalidPackage
		}
		defer decoder.Close()
		input = decoder
	case ".xz":
		decoder, err := boundedXZ(reader, size)
		if err != nil {
			return nil, ErrInvalidPackage
		}
		input = decoder
	case ".zst":
		decoder, err := zstd.NewReader(input, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxControlBytes))
		if err != nil {
			return nil, ErrInvalidPackage
		}
		defer decoder.Close()
		input = decoder
	default:
		return nil, ErrInvalidPackage
	}
	archive := tar.NewReader(io.LimitReader(input, maxControlBytes+1))
	for range 4096 {
		header, err := archive.Next()
		if err != nil {
			return nil, ErrInvalidPackage
		}
		if strings.TrimPrefix(header.Name, "./") != "control" {
			continue
		}
		if !header.FileInfo().Mode().IsRegular() || header.Size <= 0 || header.Size > maxControlFileBytes {
			return nil, ErrInvalidPackage
		}
		return io.ReadAll(io.LimitReader(archive, maxControlFileBytes+1))
	}
	return nil, ErrInvalidPackage
}

func parseControl(data []byte) (map[string]string, error) {
	if bytes.ContainsAny(data, "\x00\r") {
		return nil, ErrInvalidPackage
	}
	fields := make(map[string]string)
	seen := make(map[string]bool)
	last := ""
	for line := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			return nil, ErrInvalidPackage
		}
		if line[0] == ' ' || line[0] == '\t' {
			if last == "" {
				return nil, ErrInvalidPackage
			}
			fields[last] += "\n" + line
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || key == "" || seen[strings.ToLower(key)] {
			return nil, ErrInvalidPackage
		}
		for _, char := range key {
			if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				return nil, ErrInvalidPackage
			}
		}
		seen[strings.ToLower(key)] = true
		key = textproto.CanonicalMIMEHeaderKey(key)
		switch strings.ToLower(key) {
		case "md5sum":
			key = "MD5sum"
		case "sha1":
			key = "SHA1"
		case "sha256":
			key = "SHA256"
		case "sha512":
			key = "SHA512"
		}
		fields[key] = strings.TrimSpace(value)
		last = key
	}
	for _, key := range []string{"Package", "Version", "Architecture"} {
		if fields[key] == "" || len(fields[key]) > 1024 || strings.ContainsAny(fields[key], " \t\n/\\") {
			return nil, ErrInvalidPackage
		}
	}
	return fields, nil
}
