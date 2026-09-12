/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package apt

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/klauspost/compress/gzip"

	"renop/pkg/hex"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func testDeb(t *testing.T, compression, control string) []byte {
	t.Helper()
	var raw bytes.Buffer
	archive := tar.NewWriter(&raw)
	if err := archive.WriteHeader(&tar.Header{Name: "./control", Mode: 0644, Size: int64(len(control))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(control)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	var writer io.WriteCloser
	var err error
	switch compression {
	case "":
		writer = nopWriteCloser{&encoded}
	case ".gz":
		writer = gzip.NewWriter(&encoded)
	case ".xz":
		writer, err = xz.NewWriter(&encoded)
	case ".zst":
		writer, err = zstd.NewWriter(&encoded, zstd.WithEncoderConcurrency(1))
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	var deb bytes.Buffer
	deb.WriteString("!<arch>\n")
	for _, member := range []struct {
		name string
		body []byte
	}{{"debian-binary", []byte("2.0\n")}, {"control.tar" + compression, encoded.Bytes()}, {"data.tar", make([]byte, 1024)}} {
		fmt.Fprintf(&deb, "%-16s%-12d%-6d%-6d%-8o%-10d`\n", member.name, 0, 0, 0, 0644, len(member.body))
		deb.Write(member.body)
		if len(member.body)%2 != 0 {
			deb.WriteByte('\n')
		}
	}
	return deb.Bytes()
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestDebControlCompressionAndReleaseChecksums(t *testing.T) {
	for _, compression := range []string{"", ".gz", ".xz", ".zst"} {
		t.Run(compression, func(t *testing.T) {
			data := testDeb(t, compression, "Package: example\nVersion: 1.0-1\nArchitecture: amd64\nDepends: libc6 (>= 2.31)\nDescription: Example\n Extended description.\n\n")
			pkg, err := ReadPackage(bytes.NewReader(data), int64(len(data)), "pool/main/e/example/example_1.0-1_amd64.deb")
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			if pkg.Fields["SHA256"] != hex.EncodeToString(sum[:]) || pkg.Fields["Depends"] != "libc6 (>= 2.31)" {
				t.Fatalf("incorrect metadata: %+v", pkg.Fields)
			}
			packages, err := PackageIndex([]*Package{pkg}, "stable", "main", "amd64")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(packages, []byte("Filename: pool/main/e/example/example_1.0-1_amd64.deb\n")) || !bytes.Contains(packages, []byte("Description: Example\n Extended description.\n")) {
				t.Fatalf("bad Packages: %s", packages)
			}
			release, err := Release([]*Package{pkg}, "stable")
			if err != nil {
				t.Fatal(err)
			}
			sum = sha256.Sum256(packages)
			line := fmt.Sprintf(" %s %d main/binary-amd64/Packages\n", hex.EncodeToString(sum[:]), len(packages))
			if !bytes.Contains(release, []byte(line)) {
				t.Fatalf("Release checksum mismatch: %s", release)
			}
			other, err := PackageIndex([]*Package{pkg}, "stable", "main", "arm64")
			if err != nil || len(other) != 0 {
				t.Fatalf("architecture filter: %s %v", other, err)
			}
		})
	}
}

func TestDebRejectsInvalidFieldsAndHugeXZDictionary(t *testing.T) {
	for _, control := range []string{"Package: example\nVersion: 1\nArchitecture: amd64\npackage: injected\n", "Package: example\nVersion: 1\nArchitecture: amd64\n\nPackage: injected\n"} {
		data := testDeb(t, ".gz", control)
		if _, err := ReadPackage(bytes.NewReader(data), int64(len(data)), "example.deb"); err == nil {
			t.Fatal("accepted duplicate or additional paragraph")
		}
	}
	var output bytes.Buffer
	writer, err := xz.NewWriter(&output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("small body")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data := bytes.Clone(output.Bytes())
	// Standard one-filter block: size, flags, filter ID, property length,
	// dictionary property. An attacker-controlled property must be rejected
	// before the library allocates the dictionary, regardless of the CRC.
	if data[14] != 0x21 || data[15] != 1 {
		t.Fatal("unexpected test XZ block layout")
	}
	data[16] = 40
	if _, err := boundedXZ(bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("accepted multi-gigabyte dictionary")
	}
	// A forged index size must not underflow a slice or allocate its size.
	data = bytes.Clone(output.Bytes())
	binary.LittleEndian.PutUint32(data[len(data)-8:], ^uint32(0))
	if _, err := boundedXZ(bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("accepted invalid index size")
	}
	if _, err := parseControl([]byte("package: example\nversion: 1\narchitecture: all\n")); err != nil {
		t.Fatalf("field names must be case-insensitive: %v", err)
	}
}
