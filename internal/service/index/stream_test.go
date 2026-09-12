/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package index

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

type boundedIndexWriter struct{ writes int }

func (w *boundedIndexWriter) Write(data []byte) (int, error) {
	if len(data) > 4096 {
		return 0, fmt.Errorf("buffered an oversized index value: %d", len(data))
	}
	w.writes++
	return len(data), nil
}

func TestIndexStreamBoundsAndPrivateContentRoundTrip(t *testing.T) {
	idx := NewFileIndex()
	for i := range 10000 {
		idx.InsertFile(fmt.Sprintf("repo/file-%05d", i), FileInfo{Size: int64(i), ModTime: 99})
	}
	writer := &boundedIndexWriter{}
	if err := idx.WritePersistentJSONTo(writer); err != nil {
		t.Fatal(err)
	}
	if writer.writes < 10000 {
		t.Fatal("index was not streamed by record")
	}
	digest := strings.Repeat("a", 64)
	key := "scope\x00repo/file-00001"
	idx.PutContent(key, ContentInfo{Digest: digest, ETag: "etag", Size: 65536, ObjectSize: 97, ModTime: 99, Reference: true, Verified: true})
	idx.MarkContentGarbage("scope\x00"+strings.Repeat("b", 64), 10)
	var data bytes.Buffer
	if err := idx.WritePersistentJSONTo(&data); err != nil {
		t.Fatal(err)
	}
	loaded := NewFileIndex()
	if err := loaded.ReadJSONFrom(&data); err != nil {
		t.Fatal(err)
	}
	if info, ok := loaded.GetContent(key); !ok || info.Digest != digest || info.Verified {
		t.Fatalf("restored content: %+v %v", info, ok)
	}
	if loaded.HasContent("scope\x00" + digest) {
		t.Fatal("restored content trusted before remote index validation")
	}
	if got := loaded.ContentGarbage("scope", 16, 20); len(got) != 1 {
		t.Fatalf("lost collection queue: %v", got)
	}
	public, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(public, []byte(digest)) || bytes.Contains(public, []byte("content_garbage")) || bytes.Contains(public, []byte("not_found")) {
		t.Fatal("private storage metadata leaked into public JSON")
	}
	var ordinary bytes.Buffer
	if err := idx.WriteJSONTo(&ordinary); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ordinary.Bytes(), []byte(digest)) {
		t.Fatal("private metadata leaked into ordinary stream")
	}
}

func TestContentReferencesProtectReuseAndNewCollectionWork(t *testing.T) {
	idx := NewFileIndex()
	digest := strings.Repeat("a", 64)
	digestKey := "scope\x00" + digest
	info := ContentInfo{Digest: digest, Reference: true, Verified: true}
	idx.PutContent("scope\x00one", info)
	idx.PutContent("scope\x00two", info)
	idx.RemoveContent("scope\x00one")
	if !idx.HasContent(digestKey) || len(idx.ContentGarbage("scope", 10, 1<<62)) != 0 {
		t.Fatal("removed a shared content reference")
	}
	idx.RemoveContent("scope\x00two")
	queue := idx.ContentGarbage("scope", 10, 1<<62)
	if len(queue) != 1 {
		t.Fatal("last reference did not enqueue collection")
	}
	previous := queue[0].Since
	idx.EnsureContentGarbage(digestKey, previous+5)
	if queue = idx.ContentGarbage("scope", 10, 1<<62); queue[0].Since != previous {
		t.Fatal("rebuild postponed existing collection")
	}
	idx.MarkContentGarbage(digestKey, previous+10)
	idx.ForgetContentGarbage(digestKey, previous)
	if queue = idx.ContentGarbage("scope", 10, 1<<62); len(queue) != 1 || queue[0].Since != previous+10 {
		t.Fatal("old collection discarded new work")
	}
	idx.PutContent("scope\x00new", info)
	if len(idx.ContentGarbage("scope", 10, 1<<62)) != 0 {
		t.Fatal("reused content remained queued")
	}
}
