/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package core

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"renop/internal/cache"
)

// ErrFileCacheMiss is returned by FileByteCache.Get when the key is absent.
var ErrFileCacheMiss = errors.New("entry not found in file cache")

// fileCacheShardCount must be a power of two (mask hashing).
const fileCacheShardCount = 16

// FileByteCache is a size-bounded in-memory cache for small artifact metadata.
// It starts empty (no preallocation), shards keys for concurrent access, and
// publishes immutable entry buffers so reads stay allocation- and race-free.
type FileByteCache struct {
	maxBytes    int
	used        atomic.Int64
	maxMetadata int
	metadata    atomic.Int64
	shards      []fileCacheShard
	remote      *cache.Remote
}

const fileCacheEntryMetadata = 256

func (c *FileByteCache) overBudget() bool {
	return c.used.Load() > int64(c.maxBytes) || c.metadata.Load() > int64(c.maxMetadata)
}

type fileCacheEntry struct {
	data []byte
	blob *cache.Blob
	size int
}

type fileCacheShard struct {
	mu      sync.RWMutex
	entries map[string]fileCacheEntry

	// order is a FIFO of keys for eviction. May contain stale keys after Delete;
	// those are skipped and compacted once their overhead becomes meaningful.
	order []string
}

// NewFileByteCache creates a cache with a hard cap of maxBytes of stored payload.
// Key and entry metadata have a separate budget of max(maxBytes, 4096), so empty
// values cannot retain an unlimited number of keys.
// Values larger than maxBytes are not stored. maxBytes <= 0 disables the cache (0 bytes capacity, 0 shards allocated).
func NewFileByteCache(maxBytes int) *FileByteCache {
	if maxBytes <= 0 {
		return &FileByteCache{maxBytes: 0}
	}
	return &FileByteCache{
		maxBytes:    maxBytes,
		maxMetadata: max(maxBytes, 4096),
		shards:      make([]fileCacheShard, fileCacheShardCount),
	}
}

// UseRemote selects external value storage before concurrent cache access.
func (c *FileByteCache) UseRemote(remote *cache.Remote) {
	if c == nil || c.maxBytes <= 0 {
		return
	}
	c.remote = remote
}

func (c *FileByteCache) shard(key string) *fileCacheShard {
	return &c.shards[hashKey(key)&(fileCacheShardCount-1)]
}

// FNV-1a 32-bit — cheap, good enough for path keys.
func hashKey(key string) uint32 {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	h := uint32(offset)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= prime
	}
	return h
}

// Get returns a defensive copy of the cached value.
func (c *FileByteCache) Get(key string) ([]byte, error) {
	v, err := c.GetReadOnlyView(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

// GetReadOnlyView returns a read-only view of the cached value slice without allocation.
// Callers MUST NOT mutate the returned byte slice.
func (c *FileByteCache) GetReadOnlyView(key string) ([]byte, error) {
	if c == nil || c.maxBytes <= 0 || len(c.shards) == 0 {
		return nil, ErrFileCacheMiss
	}
	s := c.shard(key)
	s.mu.RLock()
	v, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrFileCacheMiss
	}
	if c.remote != nil {
		data, err := v.blob.Read()
		s.mu.RLock()
		current, exists := s.entries[key]
		s.mu.RUnlock()
		if err != nil || !exists || current.blob != v.blob {
			return nil, ErrFileCacheMiss
		}
		return data, nil
	}
	return v.data, nil
}

// Set stores an immutable copy of value under key, evicting oldest entries until
// under budget. Existing buffers are never mutated because readers may still
// hold a read-only view after the shard lock is released.
func (c *FileByteCache) Set(key string, value []byte) error {
	if c == nil || c.maxBytes <= 0 || len(c.shards) == 0 {
		return nil
	}
	if len(value) > c.maxBytes || len(key) > c.maxMetadata-fileCacheEntryMetadata {
		return c.Delete(key)
	}
	key = strings.Clone(key)
	entry := fileCacheEntry{size: len(value)}
	if c.remote != nil {
		var err error
		entry.blob, err = c.remote.Put(value, time.Hour)
		if err != nil {
			_ = c.Delete(key)
			return err
		}
	} else {
		entry.data = make([]byte, len(value))
		copy(entry.data, value)
	}

	s := c.shard(key)
	s.mu.Lock()
	if s.entries == nil {
		s.entries = make(map[string]fileCacheEntry)
	}

	delta := int64(entry.size)
	var previous *cache.Blob
	if old, ok := s.entries[key]; ok {
		delta -= int64(old.size)
		previous = old.blob
		s.entries[key] = entry
	} else {
		s.entries[key] = entry
		s.order = append(s.order, key)
		c.metadata.Add(int64(len(key) + fileCacheEntryMetadata))
	}
	if delta != 0 {
		c.used.Add(delta)
	}
	s.mu.Unlock()
	previous.Delete()

	if c.overBudget() {
		c.trimToMax(key)
	}
	return nil
}

// Delete removes key if present.
func (c *FileByteCache) Delete(key string) error {
	if c == nil || c.maxBytes <= 0 || len(c.shards) == 0 {
		return nil
	}
	s := c.shard(key)
	s.mu.Lock()
	var previous *cache.Blob
	if old, ok := s.entries[key]; ok {
		c.used.Add(-int64(old.size))
		c.metadata.Add(-int64(len(key) + fileCacheEntryMetadata))
		previous = old.blob
		delete(s.entries, key)
		s.compactOrderLocked()
	}
	s.mu.Unlock()
	previous.Delete()
	return nil
}

// Stats returns aggregate entry count and payload bytes (for tests/diagnostics).
func (c *FileByteCache) Stats() (entries, usedBytes int) {
	if c == nil || c.maxBytes <= 0 || len(c.shards) == 0 {
		return 0, 0
	}
	usedBytes = max(int(c.used.Load()), 0)
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.RLock()
		entries += len(s.entries)
		s.mu.RUnlock()
	}
	return entries, usedBytes
}

// trimToMax drops oldest entries until payload and metadata fit their budgets.
// protect is never removed while other entries remain (the entry just written).
// Shards are locked one at a time in index order to avoid deadlock.
func (c *FileByteCache) trimToMax(protect string) {
	if c == nil || c.maxBytes <= 0 || len(c.shards) == 0 {
		return
	}
	for c.overBudget() {
		progress := false
		for i := range c.shards {
			if !c.overBudget() {
				return
			}
			s := &c.shards[i]
			s.mu.Lock()
			evicted, blob := c.evictOneLocked(s, protect)
			if evicted {
				progress = true
			}
			s.compactOrderLocked()
			s.mu.Unlock()
			blob.Delete()
		}
		if !progress {
			return
		}
	}
}

// evictOneLocked removes one non-protect entry from s. Returns true if something was removed.
func (c *FileByteCache) evictOneLocked(s *fileCacheShard, protect string) (bool, *cache.Blob) {
	for len(s.order) > 0 {
		k := s.order[0]
		s.order[0] = ""
		s.order = s.order[1:]
		if k == protect {
			s.order = append(s.order, k)
			if len(s.entries) <= 1 {
				break
			}
			continue
		}
		if v, ok := s.entries[k]; ok {
			c.used.Add(-int64(v.size))
			c.metadata.Add(-int64(len(k) + fileCacheEntryMetadata))
			delete(s.entries, k)
			return true, v.blob
		}
	}
	for k, v := range s.entries {
		if k == protect {
			continue
		}
		c.used.Add(-int64(v.size))
		c.metadata.Add(-int64(len(k) + fileCacheEntryMetadata))
		delete(s.entries, k)
		return true, v.blob
	}
	return false, nil
}

func (s *fileCacheShard) compactOrderLocked() {
	if len(s.entries) == 0 {
		// Drop the map as well as its eviction order. A shard can briefly see
		// many unique keys during a rebuild; retaining its hash buckets after
		// the last entry is a significant source of idle heap retention.
		s.entries = nil
		clear(s.order)
		if cap(s.order) > 64 {
			s.order = nil
		} else {
			s.order = s.order[:0]
		}
		return
	}
	if len(s.order) <= 2*len(s.entries)+32 {
		return
	}
	n := 0
	for _, k := range s.order {
		if _, ok := s.entries[k]; ok {
			s.order[n] = k
			n++
		}
	}
	clear(s.order[n:])
	s.order = s.order[:n]
	if cap(s.order) > max(64, 4*n) {
		compacted := make([]string, n)
		copy(compacted, s.order)
		s.order = compacted
	}
}
