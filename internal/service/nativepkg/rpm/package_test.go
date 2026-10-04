/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package rpm

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/xml"
	"io"
	"maps"
	"slices"
	"testing"

	"github.com/klauspost/compress/gzip"

	"renop/pkg/hex"
)

func testRPM(t *testing.T) []byte {
	return testRPMFields(t, nil)
}

func testRPMFields(t *testing.T, extra map[uint32]string) []byte {
	t.Helper()
	fields := map[uint32]string{1000: "example", 1001: "1.0", 1002: "1", 1022: "noarch", 1004: "Example & tools", 1005: "Package description", 1014: "MIT", 1047: "example", 1049: "libc.so.6", 1050: "2.31", 1113: "1.0-1"}
	maps.Copy(fields, extra)
	tags := make([]uint32, 0, len(fields))
	for tag := range fields {
		tags = append(tags, tag)
	}
	slices.Sort(tags)
	var entries, data bytes.Buffer
	for _, tag := range tags {
		kind := uint32(6)
		if tag == 1047 || tag == 1049 || tag == 1050 || tag == 1113 {
			kind = 8
		}
		for _, value := range []uint32{tag, kind, uint32(data.Len()), 1} {
			if err := binary.Write(&entries, binary.BigEndian, value); err != nil {
				t.Fatal(err)
			}
		}
		data.WriteString(fields[tag])
		data.WriteByte(0)
	}
	lead := make([]byte, 96)
	binary.BigEndian.PutUint32(lead, 0xedabeedb)
	lead[4] = 3
	empty := []byte{0x8e, 0xad, 0xe8, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	body := append(lead, empty...)
	header := bytes.Clone(empty)
	binary.BigEndian.PutUint32(header[8:12], uint32(len(tags)))
	binary.BigEndian.PutUint32(header[12:16], uint32(data.Len()))
	body = append(body, header...)
	body = append(body, entries.Bytes()...)
	body = append(body, data.Bytes()...)
	return append(body, []byte("payload")...)
}

func TestRPMMetadataAndNativeIndexChecksums(t *testing.T) {
	body := testRPM(t)
	pkg, err := ReadPackage(bytes.NewReader(body), int64(len(body)), "Packages/example-1.0-1.noarch.rpm")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if pkg.Name != "example" || pkg.SHA256 != hex.EncodeToString(sum[:]) || len(pkg.Requires) != 1 || pkg.Requires[0].Name != "libc.so.6" {
		t.Fatalf("incorrect package: %+v", pkg)
	}
	documents, err := Index([]*Package{pkg})
	if err != nil {
		t.Fatal(err)
	}
	var repomd struct {
		Data []struct {
			Type         string `xml:"type,attr"`
			Checksum     string `xml:"checksum"`
			OpenChecksum string `xml:"open-checksum"`
			Location     struct {
				Href string `xml:"href,attr"`
			} `xml:"location"`
		} `xml:"data"`
	}
	if err := xml.Unmarshal(documents["repodata/repomd.xml"], &repomd); err != nil {
		t.Fatal(err)
	}
	if len(repomd.Data) != 3 {
		t.Fatalf("missing metadata documents: %s", documents["repodata/repomd.xml"])
	}
	for _, entry := range repomd.Data {
		compressed := documents[entry.Location.Href]
		sum := sha256.Sum256(compressed)
		if entry.Checksum != hex.EncodeToString(sum[:]) {
			t.Fatal("compressed checksum mismatch")
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		sum = sha256.Sum256(plain)
		if entry.OpenChecksum != hex.EncodeToString(sum[:]) {
			t.Fatal("open checksum mismatch")
		}
		if entry.Type == "primary" {
			decoder := xml.NewDecoder(bytes.NewReader(plain))
			licenseFound := false
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if element, ok := token.(xml.StartElement); ok && element.Name.Local == "license" {
					licenseFound = element.Name.Space == "http://linux.duke.edu/metadata/rpm"
				}
			}
			if !licenseFound {
				t.Fatalf("incorrect RPM namespace: %s", plain)
			}
			if !bytes.Contains(plain, []byte("Example &amp; tools")) {
				t.Fatal("summary was not XML escaped")
			}
		}
	}
}

func TestRPMRejectsTruncatedAndOversizedHeaders(t *testing.T) {
	body := testRPM(t)
	for _, length := range []int{0, 95, 111, len(body) - 10} {
		if _, err := ReadPackage(bytes.NewReader(body[:length]), int64(length), "package.rpm"); err == nil {
			t.Fatalf("accepted truncation %d", length)
		}
	}
	for _, offset := range []int{104, 108, 120, 124} {
		malformed := bytes.Clone(body)
		binary.BigEndian.PutUint32(malformed[offset:offset+4], ^uint32(0))
		if _, err := ReadPackage(bytes.NewReader(malformed), int64(len(malformed)), "package.rpm"); err == nil {
			t.Fatalf("accepted oversized header at %d", offset)
		}
	}
}
