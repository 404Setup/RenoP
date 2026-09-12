/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package artifactstore owns atomic installation of immutable artifact contents.
// Logical paths remain ordinary files. Writers must replace files, never modify
// installed inodes: identical artifacts can share an inode through hard links.
package artifactstore

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"sync"

	"renop/internal/utils"

	"github.com/google/uuid"
)

const candidatesPerShard = 256

type contentKey struct {
	digest [sha256.Size]byte
	mode   os.FileMode
}

type contentShard struct {
	sync.Mutex
	paths map[contentKey]string
}

// Disk keeps only bounded, disposable hints. Neither a reference database nor
// hidden object pool is needed; filesystem link counts reclaim the last copy.
type Disk struct {
	shards [64]contentShard
}

var DefaultDisk Disk

// CommitMutable installs independently timestamped contents, such as expiring
// upstream caches. Their on-disk modification time is the durable freshness
// clock, so sharing an inode would incorrectly share freshness after restart.
func CommitMutable(staged, target string) error {
	return utils.SafeRename(staged, target)
}

func fingerprint(path string) (contentKey, error) {
	return fingerprintContext(context.Background(), path)
}

func fingerprintContext(ctx context.Context, path string) (contentKey, error) {
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return contentKey{}, err
	}
	if !linkInfo.Mode().IsRegular() {
		return contentKey{}, &os.PathError{Op: "fingerprint", Path: path, Err: os.ErrInvalid}
	}
	f, err := os.Open(path)
	if err != nil {
		return contentKey{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return contentKey{}, err
	}
	if !info.Mode().IsRegular() {
		return contentKey{}, &os.PathError{Op: "fingerprint", Path: path, Err: os.ErrInvalid}
	}
	h := sha256.New()
	if _, err := io.Copy(h, contextReader{ctx, f}); err != nil {
		return contentKey{}, err
	}
	key := contentKey{mode: info.Mode().Perm()}
	copy(key.digest[:], h.Sum(nil))
	return key, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

// ReuseExisting coalesces an installed immutable file. The caller must exclude
// writes to target while it runs. Busy content shards are left for a later pass.
func (d *Disk) ReuseExisting(ctx context.Context, target string) (bool, error) {
	key, err := fingerprintContext(ctx, target)
	if err != nil {
		return false, err
	}
	shard := &d.shards[key.digest[0]%byte(len(d.shards))]
	if !shard.TryLock() {
		return false, nil
	}
	defer shard.Unlock()
	if shard.paths == nil {
		shard.paths = make(map[contentKey]string)
	}
	candidate := shard.paths[key]
	if candidate != "" && candidate != target {
		targetInfo, err := os.Stat(target)
		if err != nil {
			return false, err
		}
		candidateInfo, err := os.Stat(candidate)
		if err == nil && os.SameFile(targetInfo, candidateInfo) {
			return false, nil
		}
		linked := target + ".tmp.dedup." + uuid.NewString()
		if err := os.Link(candidate, linked); err == nil {
			defer os.Remove(linked)
			actual, err := fingerprintContext(ctx, linked)
			if err != nil {
				return false, err
			}
			if actual == key {
				if err := utils.SafeRename(linked, target); err != nil {
					return false, err
				}
				shard.paths[key] = target
				return true, nil
			}
		}
	}
	if len(shard.paths) >= candidatesPerShard {
		clear(shard.paths)
	}
	shard.paths[key] = target
	return false, nil
}

// Commit consumes staged after success. A failed replacement preserves both the
// installed destination and the original staged contents. Unsupported hard links
// and cross-volume candidates transparently use the ordinary atomic installer.
func (d *Disk) Commit(staged, target string) error {
	key, err := fingerprint(staged)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	shard := &d.shards[key.digest[0]%byte(len(d.shards))]
	shard.Lock()
	defer shard.Unlock()
	if shard.paths == nil {
		shard.paths = make(map[contentKey]string)
	}
	if candidate := shard.paths[key]; candidate != "" {
		linked := target + ".tmp.dedup." + uuid.NewString()
		if err := os.Link(candidate, linked); err == nil {
			// Verify the linked inode, not the mutable candidate pathname. A stale
			// hint after replacement or an external edit must never change bytes.
			actual, checkErr := fingerprint(linked)
			if checkErr == nil && actual == key {
				err := utils.SafeRename(linked, target)
				_ = os.Remove(linked)
				if err != nil {
					return err
				}
				_ = os.Remove(staged)
				shard.paths[key] = target
				return nil
			}
			_ = os.Remove(linked)
		}
	}
	if err := utils.SafeRename(staged, target); err != nil {
		return err
	}
	if len(shard.paths) >= candidatesPerShard {
		clear(shard.paths)
	}
	shard.paths[key] = target
	return nil
}

// WriteFile replaces a small generated companion without modifying shared bytes.
func (d *Disk) WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	staged := path + ".tmp.content." + uuid.NewString()
	defer os.Remove(staged)
	f, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return d.Commit(staged, path)
}
