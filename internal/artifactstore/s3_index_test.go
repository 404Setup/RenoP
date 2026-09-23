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
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"renop/internal/testutil"

	"github.com/minio/minio-go/v7"
)

type memoryS3Index struct {
	records map[string]S3Record
	pending map[string]int64
}

func (i *memoryS3Index) Get(key string) (S3Record, bool) { v, ok := i.records[key]; return v, ok }
func (i *memoryS3Index) Put(key string, v S3Record) {
	i.records[key] = v
	if v.Reference {
		delete(i.pending, v.Digest)
	}
}
func (i *memoryS3Index) Remove(key string) {
	v := i.records[key]
	delete(i.records, key)
	if v.Reference {
		i.MarkGarbage(v.Digest, 0)
	}
}
func (i *memoryS3Index) Has(digest string) bool {
	for _, v := range i.records {
		if v.Reference && v.Verified && v.Digest == digest {
			return true
		}
	}
	return false
}
func (i *memoryS3Index) MarkGarbage(digest string, since int64) {
	if !i.Has(digest) {
		i.pending[digest] = since
	}
}
func (i *memoryS3Index) ForgetGarbage(digest string, since int64) {
	if i.pending[digest] == since {
		delete(i.pending, digest)
	}
}
func (i *memoryS3Index) Garbage(limit int, before int64) []S3Garbage {
	result := []S3Garbage{}
	for key, since := range i.pending {
		if since <= before {
			result = append(result, S3Garbage{key, since})
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

func TestS3IndexAvoidsPaidReadsAndLearnsDigestsFromDownloads(t *testing.T) {
	backend := &testS3{objects: map[string]testS3Object{}}
	server := httptest.NewServer(backend)
	defer server.Close()
	client, err := minio.New(strings.TrimPrefix(server.URL, "http://"), &minio.Options{Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	metadata := &memoryS3Index{records: map[string]S3Record{}, pending: map[string]int64{}}
	store := S3{Client: client, Bucket: "artifacts", Prefix: "private/content", Index: metadata}
	ctx := context.Background()
	data := bytes.Repeat([]byte("payload"), 10000)
	file := filepath.Join(testutil.TempDir(t), "data")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.PutFile(ctx, "repo/first", file); err != nil {
		t.Fatal(err)
	}
	reads := func() int { backend.Lock(); defer backend.Unlock(); return backend.reads }
	before := reads()
	if err := store.PutFile(ctx, "repo/first", file); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		if _, _, err := store.Resolve(ctx, "repo/first"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Collect(ctx, "", 16); err != nil {
		t.Fatal(err)
	}
	if reads() != before {
		t.Fatalf("indexed replacement/metadata/idle collection performed %d paid reads", reads()-before)
	}
	if err := store.PutRaw(ctx, "repo/legacy", bytes.NewReader(data), int64(len(data)), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	record, _ := metadata.Get("repo/legacy")
	before = reads()
	if err := store.DeduplicateKnown(ctx, "repo/legacy", record); err != nil {
		t.Fatal(err)
	}
	if reads() != before {
		t.Fatal("unknown digest caused a background read")
	}
	physical, info, err := store.Resolve(ctx, "repo/legacy")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := client.GetObject(ctx, store.Bucket, physical, minio.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	tracked := store.TrackRead("repo/legacy", reader, info)
	downloaded, err := io.ReadAll(tracked)
	tracked.Close()
	if err != nil || !bytes.Equal(downloaded, data) {
		t.Fatal("ordinary download failed", err)
	}
	record, _ = metadata.Get("repo/legacy")
	if record.Digest == "" {
		t.Fatal("ordinary download did not record a digest")
	}
	before = reads()
	if err := store.DeduplicateKnown(ctx, "repo/legacy", record); err != nil {
		t.Fatal(err)
	}
	if reads() != before {
		t.Fatalf("known duplicate migration performed %d paid reads", reads()-before)
	}
}
