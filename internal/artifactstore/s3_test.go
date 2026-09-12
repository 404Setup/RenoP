/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package artifactstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"renop/internal/testutil"
	"renop/pkg/hex"

	"github.com/minio/minio-go/v7"
)

type testS3Object struct {
	data     []byte
	header   http.Header
	modified time.Time
}
type testS3 struct {
	sync.Mutex
	objects          map[string]testS3Object
	failPointer      bool
	replaceOnPointer map[string][]byte
	reads            int
	bytesRead        int64
}

func (s *testS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var data []byte
	if r.Method == "PUT" && r.Header.Get("X-Amz-Copy-Source") == "" {
		var err error
		data, err = io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
			data, err = io.ReadAll(httputil.NewChunkedReader(bytes.NewReader(data)))
			if err != nil {
				w.WriteHeader(400)
				return
			}
		}
	}
	s.Lock()
	defer s.Unlock()
	if r.Method == "GET" || r.Method == "HEAD" {
		s.reads++
	}
	key := strings.TrimPrefix(r.URL.Path, "/artifacts/")
	if r.URL.Query().Get("list-type") == "2" {
		prefix, delimiter, after := r.URL.Query().Get("prefix"), r.URL.Query().Get("delimiter"), r.URL.Query().Get("start-after")
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>artifacts</Name><IsTruncated>false</IsTruncated>`)
		keys := make([]string, 0, len(s.objects))
		for key := range s.objects {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		prefixes := map[string]bool{}
		for _, key := range keys {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			if delimiter != "" {
				if first, _, ok := strings.Cut(strings.TrimPrefix(key, prefix), delimiter); ok {
					common := prefix + first + delimiter
					if common > after && !prefixes[common] {
						fmt.Fprint(w, "<CommonPrefixes><Prefix>")
						xml.EscapeText(w, []byte(common))
						fmt.Fprint(w, "</Prefix></CommonPrefixes>")
						prefixes[common] = true
					}
					continue
				}
			}
			if key <= after {
				continue
			}
			object := s.objects[key]
			fmt.Fprint(w, "<Contents><Key>")
			xml.EscapeText(w, []byte(key))
			fmt.Fprintf(w, "</Key><Size>%d</Size><LastModified>%s</LastModified></Contents>", len(object.data), object.modified.UTC().Format(time.RFC3339Nano))
		}
		fmt.Fprint(w, "</ListBucketResult>")
		return
	}
	switch r.Method {
	case "PUT":
		if replacement, ok := s.replaceOnPointer[key]; ok && r.Header.Get("X-Amz-Meta-Renop-Reference") == "1" {
			s.objects[key] = testS3Object{replacement, http.Header{}, time.Now().UTC()}
			delete(s.replaceOnPointer, key)
		}
		if match := r.Header.Get("If-Match"); match != "" {
			object, exists := s.objects[key]
			sum := md5.Sum(object.data)
			if !exists || match != `"`+hex.EncodeToString(sum[:])+`"` {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(412)
				fmt.Fprint(w, "<Error><Code>PreconditionFailed</Code></Error>")
				return
			}
		}
		if source := r.Header.Get("X-Amz-Copy-Source"); source != "" {
			object, ok := s.objects[strings.TrimPrefix(strings.TrimPrefix(source, "/"), "artifacts/")]
			if !ok {
				w.WriteHeader(404)
				return
			}
			if match := r.Header.Get("X-Amz-Copy-Source-If-Match"); match != "" {
				sum := md5.Sum(object.data)
				if match != `"`+hex.EncodeToString(sum[:])+`"` {
					w.WriteHeader(412)
					return
				}
			}
			object.modified = time.Now().UTC()
			s.objects[key] = object
			sum := md5.Sum(object.data)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, "<CopyObjectResult><ETag>\"%s\"</ETag><LastModified>%s</LastModified></CopyObjectResult>", hex.EncodeToString(sum[:]), object.modified.Format(time.RFC3339Nano))
			return
		}
		if s.failPointer && r.Header.Get("X-Amz-Meta-Renop-Reference") == "1" {
			w.WriteHeader(500)
			return
		}
		s.objects[key] = testS3Object{data, r.Header.Clone(), time.Now().UTC()}
		sum := md5.Sum(data)
		w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
		w.WriteHeader(200)
	case "DELETE":
		delete(s.objects, key)
		w.WriteHeader(204)
	case "HEAD", "GET":
		object, ok := s.objects[key]
		if !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(404)
			fmt.Fprint(w, "<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>")
			return
		}
		if r.Method == "GET" {
			s.bytesRead += int64(len(object.data))
		}
		for key, values := range object.header {
			if strings.HasPrefix(strings.ToLower(key), "x-amz-meta-") {
				w.Header()[key] = values
			}
		}
		w.Header().Set("Content-Type", object.header.Get("Content-Type"))
		sum := md5.Sum(object.data)
		w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
		http.ServeContent(w, r, "data", object.modified, bytes.NewReader(object.data))
	default:
		w.WriteHeader(400)
	}
}

func TestS3ContentSharingReplacementRecoveryAndCollection(t *testing.T) {
	backend := &testS3{objects: map[string]testS3Object{}}
	server := httptest.NewServer(backend)
	defer server.Close()
	client, err := minio.New(strings.TrimPrefix(server.URL, "http://"), &minio.Options{Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	store := S3{Client: client, Bucket: "artifacts", Prefix: "private/content"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data := bytes.Repeat([]byte("shared artifact"), 10000)
	file := filepath.Join(testutil.TempDir(t), "artifact")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"repo/a", "repo/b"} {
		if err := store.PutFile(ctx, key, file); err != nil {
			t.Fatal(err)
		}
	}
	physical, info, err := store.Resolve(ctx, "repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != int64(len(data)) {
		t.Fatalf("logical size=%d", info.Size)
	}
	physicalB, _, err := store.Resolve(ctx, "repo/b")
	if err != nil || physicalB != physical {
		t.Fatalf("duplicate did not share payload: %s, %v", physicalB, err)
	}
	reader, err := client.GetObject(ctx, "artifacts", physical, minio.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("shared content differs", err)
	}
	if err := store.PutRaw(ctx, "repo/a", strings.NewReader("replacement"), 11, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if key, info, err := store.Resolve(ctx, "repo/a"); err != nil || key != "repo/a" || info.Size != 11 {
		t.Fatalf("raw replacement: %s %+v %v", key, info, err)
	}
	digest, err := store.oldDigest(ctx, "repo/b")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.collectBlob(ctx, digest, time.Now().Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.StatObject(ctx, "artifacts", physical, minio.StatObjectOptions{}); err != nil {
		t.Fatal("deleted live payload", err)
	}
	if err := store.Delete(ctx, "repo/b"); err != nil {
		t.Fatal(err)
	}
	if err := store.collectBlob(ctx, digest, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.StatObject(ctx, "artifacts", physical, minio.StatObjectOptions{}); err != nil {
		t.Fatal("did not retain read grace", err)
	}
	if err := store.collectBlob(ctx, digest, time.Now().Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.StatObject(ctx, "artifacts", physical, minio.StatObjectOptions{}); !missingS3(err) {
		t.Fatalf("orphan payload retained: %v", err)
	}
	// A lost pointer-write acknowledgement retains both staged bytes and the old
	// logical destination. Stale markers are repaired on a later collection pass.
	backend.Lock()
	backend.failPointer = true
	backend.Unlock()
	if err := store.PutFile(ctx, "repo/a", file); err == nil {
		t.Fatal("expected pointer failure")
	}
	if _, info, err := store.Resolve(ctx, "repo/a"); err != nil || info.Size != 11 {
		t.Fatal("failed write replaced installed object", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("failed write consumed staging")
	}
	backend.Lock()
	backend.failPointer = false
	backend.Unlock()
	if _, err := store.Collect(ctx, "", 16); err != nil {
		t.Fatal(err)
	}
	streamData := bytes.Repeat([]byte("stream payload"), 10000)
	for _, key := range []string{"repo/stream-a", "repo/stream-b"} {
		if err := store.PutStream(ctx, key, bytes.NewReader(streamData), int64(len(streamData)), "application/octet-stream"); err != nil {
			t.Fatal(err)
		}
	}
	streamKey, _, err := store.Resolve(ctx, "repo/stream-a")
	if err != nil {
		t.Fatal(err)
	}
	streamKeyB, _, err := store.Resolve(ctx, "repo/stream-b")
	if err != nil || streamKeyB != streamKey {
		t.Fatal("streamed duplicates did not share content", err)
	}
	var ranged minio.GetObjectOptions
	if err := ranged.SetRange(7, 21); err != nil {
		t.Fatal(err)
	}
	reader, err = client.GetObject(ctx, "artifacts", streamKey, ranged)
	if err != nil {
		t.Fatal(err)
	}
	actual, err = io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(actual, streamData[7:22]) {
		t.Fatal("shared range differs", err)
	}
	if err := store.PutRaw(ctx, "repo/legacy", bytes.NewReader(streamData), int64(len(streamData)), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	_, originalInfo, err := store.Resolve(ctx, "repo/legacy")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(streamData)
	known := func(info minio.ObjectInfo) S3Record {
		return S3Record{Digest: hex.EncodeToString(sum[:]), ETag: info.ETag, Size: info.Size, ObjectSize: info.Size, ModTime: info.LastModified.UnixNano(), Verified: true}
	}
	if err := store.DeduplicateKnown(ctx, "repo/legacy", known(originalInfo)); err != nil {
		t.Fatal(err)
	}
	migrated, migratedInfo, err := store.Resolve(ctx, "repo/legacy")
	if err != nil || migrated != streamKey || !migratedInfo.LastModified.Equal(originalInfo.LastModified) {
		t.Fatal("legacy migration changed data or timestamp", err)
	}
	if err := store.PutRaw(ctx, "repo/changing", bytes.NewReader(streamData), int64(len(streamData)), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	_, changingInfo, err := store.Resolve(ctx, "repo/changing")
	if err != nil {
		t.Fatal(err)
	}
	backend.Lock()
	backend.replaceOnPointer = map[string][]byte{"repo/changing": []byte("concurrent replacement")}
	backend.Unlock()
	if err := store.DeduplicateKnown(ctx, "repo/changing", known(changingInfo)); err == nil {
		t.Fatal("accepted a changed migration source")
	}
	if key, info, err := store.Resolve(ctx, "repo/changing"); err != nil || key != "repo/changing" || info.Size != int64(len("concurrent replacement")) {
		t.Fatal("migration overwrote concurrent replacement", err)
	}
	backend.Lock()
	defer backend.Unlock()
	for key := range backend.objects {
		if strings.Contains(key, "/staging/") {
			t.Fatalf("retained completed staging: %s", key)
		}
	}
}
