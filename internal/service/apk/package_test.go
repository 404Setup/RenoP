/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package apk

import (
	"archive/tar"
	"bytes"
	"crypto/sha1"
	"io"
	"strings"
	"testing"

	"github.com/emmansun/base64"
	"github.com/klauspost/compress/gzip"
)

func testSegment(t *testing.T, name, contents string) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestAPKControlChecksumAndIndex(t *testing.T) {
	control := testSegment(t, ".PKGINFO", "pkgname = example\npkgver = 1.0-r0\narch = x86_64\nsize = 1024\npkgdesc = Example\nlicense = MIT\ndepend = libc\ndepend = so:libc.so\nprovides = cmd:example=1.0\n")
	payload := testSegment(t, "usr/bin/example", "payload")
	for _, signed := range []bool{false, true} {
		var data []byte
		if signed {
			data = append(data, testSegment(t, ".SIGN.RSA.example.rsa.pub", "signature")...)
		}
		data = append(data, control...)
		data = append(data, payload...)
		pkg, err := ReadPackage(bytes.NewReader(data), int64(len(data)), "example-1.0-r0.apk")
		if err != nil {
			t.Fatalf("signed=%v: %v", signed, err)
		}
		sum := sha1.Sum(control)
		if pkg.Fields["C"] != "Q1"+base64.StdEncoding.EncodeToString(sum[:]) {
			t.Fatalf("wrong control checksum: %s", pkg.Fields["C"])
		}
		if pkg.Fields["D"] != "libc so:libc.so" {
			t.Fatalf("lost dependencies: %+v", pkg.Fields)
		}
		data, err = Index([]*Package{pkg})
		if err != nil {
			t.Fatal(err)
		}
		compressed, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		archive := tar.NewReader(compressed)
		found := false
		for {
			header, err := archive.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if header.Name != "APKINDEX" {
				continue
			}
			index, err := io.ReadAll(archive)
			if err != nil {
				t.Fatal(err)
			}
			found = strings.Contains(string(index), "P:example\nV:1.0-r0\nA:x86_64\n") && strings.Contains(string(index), "D:libc so:libc.so\n")
		}
		compressed.Close()
		if !found {
			t.Fatal("missing native package record")
		}
	}
}

func TestAPKRejectsWrongFilenameAndMalformedMetadata(t *testing.T) {
	for _, metadata := range []string{"pkgname = example\npkgver = 1.0-r0\narch = x86_64\nsize = -1\n", "pkgname = ../example\npkgver = 1.0-r0\narch = x86_64\n", "pkgname = example\npkgname = duplicate\npkgver = 1.0-r0\narch = x86_64\n"} {
		data := append(testSegment(t, ".PKGINFO", metadata), testSegment(t, "data", "body")...)
		if _, err := ReadPackage(bytes.NewReader(data), int64(len(data)), "example-1.0-r0.apk"); err == nil {
			t.Fatalf("accepted malformed metadata: %q", metadata)
		}
	}
	data := append(testSegment(t, ".PKGINFO", "pkgname = example\npkgver = 1.0-r0\narch = x86_64\n"), testSegment(t, "data", "body")...)
	if _, err := ReadPackage(bytes.NewReader(data), int64(len(data)), "renamed.apk"); err == nil {
		t.Fatal("accepted filename clients cannot resolve")
	}
}
