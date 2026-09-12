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
	"fmt"
	"sync"
	"testing"
	"time"

	"renop/internal/testutil"
)

func TestNegativeCacheUsesBackendAndPreservesPositivePaths(t *testing.T) {
	idx := NewFileIndex()
	idx.UseRemoteCache(testutil.RemoteCache(t))
	expires := time.Now().Add(time.Minute).Unix()
	idx.InsertNotFound("repo/missing", expires)
	if !idx.IsNotFound("repo/missing") || idx.Snapshot().NotFound["repo/missing"] != expires {
		t.Fatal("negative lookup or its local invalidation metadata was lost")
	}
	idx.InsertFile("repo/missing", FileInfo{Size: 7})
	idx.InsertNotFound("repo/missing", expires)
	_, info, ok, missing := idx.GetPathState("repo/missing")
	if !ok || missing || info.Size != 7 || idx.IsNotFound("repo/missing") {
		t.Fatal("late negative response hid an installed file")
	}
	idx.InsertDir("repo/directory")
	idx.InsertNotFound("repo/directory", expires)
	if idx.IsNotFound("repo/directory") {
		t.Fatal("negative response hid an indexed directory")
	}
	idx.InsertNotFound("repo/directory/child", expires)
	idx.RemoveDir("repo/directory")
	if idx.IsNotFound("repo/directory/child") {
		t.Fatal("removed directory retained a stale child miss")
	}
	idx.InsertNotFound("repo/expired", time.Now().Add(-time.Second).Unix())
	if idx.IsNotFound("repo/expired") {
		t.Fatal("expired miss was retained")
	}
}

func TestNegativeCacheBoundsConcurrentMisses(t *testing.T) {
	idx := NewFileIndex()
	expires := time.Now().Add(time.Minute).Unix()
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for n := range 3000 {
				idx.InsertNotFound(fmt.Sprintf("repo/%d/%d", worker, n), expires)
			}
		})
	}
	workers.Wait()
	// The shared cache rounds the budget to its 32 bounded shards.
	if size := idx.negativeCache().Len(); size > maxNegativeEntries+31 {
		t.Fatalf("negative cache grew beyond its bound: %d", size)
	}
}

func TestConcurrentIndexMutationsKeepCountsAndClassification(t *testing.T) {
	idx := NewFileIndex()
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for n := range 500 {
				idx.InsertFile("repo/shared", FileInfo{Size: int64(worker + n)})
				idx.InsertDir("repo/shared")
				if n%3 == 0 {
					idx.RemoveFile("repo/shared")
				}
			}
		})
	}
	workers.Wait()
	idx.InsertFile("repo/shared", FileInfo{Size: 42})
	if idx.TotalFileBytes() != 42 || idx.FilesCount.Load() != 1 || idx.HasDir("repo/shared") {
		t.Fatalf("inconsistent index: bytes=%d files=%d directory=%v", idx.TotalFileBytes(), idx.FilesCount.Load(), idx.HasDir("repo/shared"))
	}
	idx.IsDirty.Store(false)
	idx.InsertFile("repo/shared", FileInfo{Size: 42, ModTime: 1})
	if !idx.IsDirty.Load() {
		t.Fatal("metadata-only replacement was not marked for persistence")
	}
}

func TestDeletedPublicationTreeReleasesOnlyItsBlocks(t *testing.T) {
	idx := NewFileIndex()
	for _, path := range []string{"repo/package/file", "repo/package", "repo/package-other/file"} {
		idx.BlockFile(path)
	}
	idx.UnblockTree("repo/package")
	if idx.IsBlocked("repo/package/file") || idx.IsBlocked("repo/package") || !idx.IsBlocked("repo/package-other/file") {
		t.Fatal("publication block cleanup crossed or retained the deleted namespace")
	}
}
