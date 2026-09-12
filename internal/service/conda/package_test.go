/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package conda

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"renop/pkg/hex"
	"testing"

	"github.com/emmansun/base64"
	"github.com/klauspost/compress/zip"

	"github.com/goccy/go-json"
	"github.com/klauspost/compress/zstd"
)

func TestLegacyBzip2Repodata(t *testing.T) {
	const fixture = "QlpoOTFBWSZTWWxEXQcAAI5bhMyAUAXvkCAK//ffagQACAggAJINRqaaaMppp6RkwQxNGh6ZIJRJqeSek0PUGmRtQAAeocYflUECbRAQr7ZaSNWF6AgoKpw+WG7HvxHjQjpqw0pUsZ+dWAS11MWiEmBJYNcuui2UnueIyG2yjrW5uzphZ6cIlanSjbhG8k9/zBrDKTs8cZKGiJCzk58640qIiOvnBi1BomgiVu+9xjGA/i7kinChINiIug4="
	body, err := base64.StdEncoding.DecodeString(fixture)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := ReadPackage(bytes.NewReader(body), int64(len(body)), "legacy-1.0-0.tar.bz2")
	if err != nil {
		t.Fatal(err)
	}
	data, err := Repodata("noarch", []*Package{pkg})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Packages) != 1 || document.Packages[pkg.Filename] == nil {
		t.Fatalf("legacy package absent: %s", data)
	}
}

func testPackage(t *testing.T, metadata string) []byte {
	t.Helper()
	var info bytes.Buffer
	encoder, err := zstd.NewWriter(&info, zstd.WithEncoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(encoder)
	if err := archive.WriteHeader(&tar.Header{Name: "info/index.json", Mode: 0644, Size: int64(len(metadata))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(metadata)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	zipWriter := zip.NewWriter(&body)
	member, err := zipWriter.CreateHeader(&zip.FileHeader{Name: "info-example-1.0-0.tar.zst", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write(info.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestNativePackageRepodata(t *testing.T) {
	body := testPackage(t, `{"name":"example","version":"1.0","build":"0","build_number":0,"subdir":"noarch","depends":["python >=3.10"],"size":1,"sha256":"untrusted"}`)
	pkg, err := ReadPackage(bytes.NewReader(body), int64(len(body)), "example-1.0-0.conda")
	if err != nil {
		t.Fatal(err)
	}
	data, err := Repodata("noarch", []*Package{pkg})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Packages map[string]json.RawMessage `json:"packages.conda"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Size    int64
		SHA256  string
		Depends []string
	}
	if err := json.Unmarshal(document.Packages[pkg.Filename], &record); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if record.Size != int64(len(body)) || record.SHA256 != hex.EncodeToString(sum[:]) || len(record.Depends) != 1 {
		t.Fatalf("incorrect package record: %+v", record)
	}
	if _, err := Repodata("linux-64", []*Package{pkg}); err == nil {
		t.Fatal("accepted wrong subdir")
	}
	empty, err := Repodata("noarch", nil)
	if err != nil || !bytes.Contains(empty, []byte(`"packages.conda":{}`)) {
		t.Fatalf("empty channel: %s, %v", empty, err)
	}
}

func TestPackageMetadataRejectsMalformedInput(t *testing.T) {
	for _, metadata := range []string{
		`{`,
		`{"name":"../escape","version":"1.0","build":"0","build_number":0}`,
		`{"name":"example","version":"1.0","build":"0","build_number":-1}`,
		`{"name":"example","version":"1.0","build":"0","build_number":0,"depends":"not-an-array"}`,
	} {
		body := testPackage(t, metadata)
		if _, err := ReadPackage(bytes.NewReader(body), int64(len(body)), "example.conda"); err == nil {
			t.Fatalf("accepted %s", metadata)
		}
	}
	if _, err := ReadPackage(bytes.NewReader([]byte("invalid")), 7, "example.tar.bz2"); err == nil {
		t.Fatal("accepted invalid legacy container")
	}
}
