/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package ttl

import (
	"fmt"
	"hash/maphash"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	syncv2 "sync/v2"
	"time"

	"renop/internal/cache"
)

const (
	numShards              = 32
	defaultMaxCacheEntries = 8192
	evictionSampleSize     = 8
)

// Entries are immutable after publication; readers never acquire a shard lock.
type cacheItem[K comparable, V any] struct {
	key       K
	value     cache.Value[V]
	expiredAt int64
	slot      int
}

type cacheShard[K comparable, V any] struct {
	mu       sync.Mutex
	slots    []*cacheItem[K, V]
	free     []int
	hand     int
	inFlight map[K]*cacheLoad[V]
	_        [128]byte
}

type cacheLoad[V any] struct {
	done       chan struct{}
	value      V
	err        error
	generation uint64
}

// CacheStats tracks lookup outcomes. Padding separates independently updated stripes.
type CacheStats struct {
	Hits   atomic.Int64
	Misses atomic.Int64
	_      [112]byte
}

// TTLCache provides lock-free reads, bounded sharded writes, lazy expiry, and miss coalescing.
// At capacity, eviction selects the earliest expiry from a bounded rotating sample.
type TTLCache[K comparable, V any] struct {
	items              syncv2.Map[K, *cacheItem[K, V]]
	shards             [numShards]*cacheShard[K, V]
	stats              [numShards]CacheStats
	defaultTTL         time.Duration
	maxEntriesPerShard int
	hashSeed           maphash.Seed
	generation         atomic.Uint64
	remote             *cache.Remote
	project            func(V) V
}

// UseRemote offloads values while preserving bounded local invalidation indexes.
// Call it before concurrent access; project must retain every field inspected by DeleteFunc.
func (c *TTLCache[K, V]) UseRemote(remote *cache.Remote, project func(V) V) {
	c.Clear()
	c.remote, c.project = remote, project
}

// IsRemote reports whether values are stored in the configured remote backend.
func (c *TTLCache[K, V]) IsRemote() bool { return c.remote != nil }

// NewTTLCache creates a cache with the default bounded capacity.
func NewTTLCache[K comparable, V any](defaultTTL time.Duration) *TTLCache[K, V] {
	return NewTTLCacheWithCapacity[K, V](defaultTTL, defaultMaxCacheEntries)
}

// NewTTLCacheWithCapacity creates a sharded cache with a bounded approximate total capacity.
func NewTTLCacheWithCapacity[K comparable, V any](defaultTTL time.Duration, maxEntries int) *TTLCache[K, V] {
	if maxEntries <= 0 {
		maxEntries = defaultMaxCacheEntries
	}
	c := &TTLCache[K, V]{defaultTTL: defaultTTL,
		maxEntriesPerShard: max((maxEntries+numShards-1)/numShards, 1), hashSeed: maphash.MakeSeed()}
	for i := range c.shards {
		c.shards[i] = &cacheShard[K, V]{}
	}
	return c
}

func (c *TTLCache[K, V]) getShard(key K) *cacheShard[K, V] {
	return c.shards[maphash.Comparable(c.hashSeed, key)%numShards]
}

// Stats returns cumulative hit and miss counts without putting all lookups on one atomic counter.
func (c *TTLCache[K, V]) Stats() (hits, misses int64) {
	for i := range c.stats {
		hits += c.stats[i].Hits.Load()
		misses += c.stats[i].Misses.Load()
	}
	return
}

// Len returns retained entries, including entries awaiting lazy expiry.
func (c *TTLCache[K, V]) Len() int {
	total := 0
	for _, shard := range c.shards {
		shard.mu.Lock()
		total += len(shard.slots) - len(shard.free)
		shard.mu.Unlock()
	}
	return total
}

// RangeIndex visits unexpired local projections without remote I/O. Like Get,
// it is a concurrent view, not a transactional snapshot. Callbacks must not
// change the cache; external values expose only the projection from UseRemote.
func (c *TTLCache[K, V]) RangeIndex(visit func(K, V) bool) {
	now := time.Now().UnixMilli()
	c.items.Range(func(key K, item *cacheItem[K, V]) bool {
		return item.expiredAt <= now || visit(key, item.value.Index)
	})
}

// Get returns one unexpired value and rejects results overlapping an invalidation.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	stats := &c.stats[rand.Uint64()%numShards]
	generation := c.generation.Load()
	item, ok := c.items.Load(key)
	if !ok {
		stats.Misses.Add(1)
		var zero V
		return zero, false
	}
	if time.Now().UnixMilli() >= item.expiredAt {
		shard := c.getShard(key)
		shard.mu.Lock()
		if current, _ := c.items.Load(key); current == item {
			c.removeLocked(shard, item)
		}
		shard.mu.Unlock()
		item.value.Blob.Delete()
		stats.Misses.Add(1)
		var zero V
		return zero, false
	}
	value, valid := item.value.Read()
	current := generation == c.generation.Load()
	if c.remote != nil {
		latest, _ := c.items.Load(key)
		current = latest == item && generation == c.generation.Load() && time.Now().UnixMilli() < item.expiredAt
	}
	if !valid || !current {
		stats.Misses.Add(1)
		var zero V
		return zero, false
	}
	stats.Hits.Add(1)
	return value, true
}

// Set stores a value using ttl or the cache default when ttl is non-positive.
func (c *TTLCache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.SetIfGeneration(key, value, ttl, c.Generation())
}

// Generation returns the current invalidation generation for a read-through load.
func (c *TTLCache[K, V]) Generation() uint64 { return c.generation.Load() }

// HTTP adapters may pass strings backed by reusable request buffers. Own the
// string before retaining it in either the entry map or an in-flight load.
func ownedKey[K comparable](key K) K {
	if value, ok := any(key).(string); ok {
		return any(strings.Clone(value)).(K)
	}
	return key
}

// SetIfGeneration stores a loaded value only when no invalidation occurred since generation was read.
func (c *TTLCache[K, V]) SetIfGeneration(key K, value V, ttl time.Duration, generation uint64) bool {
	// Non-reflexive keys (NaN, including NaN inside structs) cannot be retrieved or evicted by key.
	if key != key || c.Generation() != generation {
		return false
	}
	key = ownedKey(key)
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	now := time.Now()
	entry := &cacheItem[K, V]{key: key, value: cache.NewValue(c.remote, value, ttl, c.project), expiredAt: now.Add(ttl).UnixMilli()}
	shard := c.getShard(key)
	shard.mu.Lock()
	if c.generation.Load() != generation {
		shard.mu.Unlock()
		entry.value.Blob.Delete()
		return false
	}
	old := c.setLocked(shard, entry)
	shard.mu.Unlock()
	old.Delete()
	return true
}

// removeLocked updates the read index and bounded slot bookkeeping together.
func (c *TTLCache[K, V]) removeLocked(shard *cacheShard[K, V], entry *cacheItem[K, V]) {
	c.items.Delete(entry.key)
	shard.slots[entry.slot] = nil
	shard.free = append(shard.free, entry.slot)
}

// setLocked never performs remote I/O or scans the full cache on a capacity miss.
func (c *TTLCache[K, V]) setLocked(shard *cacheShard[K, V], entry *cacheItem[K, V]) *cache.Blob {
	var previous *cacheItem[K, V]
	if current, ok := c.items.Load(entry.key); ok {
		previous, entry.slot = current, current.slot
	} else if n := len(shard.free); n > 0 {
		entry.slot = shard.free[n-1]
		shard.free = shard.free[:n-1]
	} else if len(shard.slots) < c.maxEntriesPerShard {
		entry.slot = len(shard.slots)
		shard.slots = append(shard.slots, nil)
	} else {
		for range min(evictionSampleSize, len(shard.slots)) {
			candidate := shard.slots[shard.hand]
			shard.hand = (shard.hand + 1) % len(shard.slots)
			if previous == nil || candidate.expiredAt < previous.expiredAt {
				previous = candidate
			}
		}
		entry.slot = previous.slot
		c.items.Delete(previous.key)
	}
	shard.slots[entry.slot] = entry
	c.items.Store(entry.key, entry)
	if previous != nil {
		return previous.value.Blob
	}
	return nil
}

// GetOrLoad returns a cached value or coalesces concurrent misses for the same key.
// The loader controls positive and negative entry lifetimes by returning a TTL.
func (c *TTLCache[K, V]) GetOrLoad(key K, loader func() (V, time.Duration, error)) (V, error) {
	if c.remote == nil {
		if value, ok := c.Get(key); ok {
			return value, nil
		}
	}
	if key != key {
		value, _, err := loader()
		return value, err
	}
	key = ownedKey(key)
	shard := c.getShard(key)
	var pending *cacheLoad[V]
	var loadGeneration uint64
	for {
		shard.mu.Lock()
		if item, _ := c.items.Load(key); c.remote == nil && item != nil && time.Now().UnixMilli() < item.expiredAt {
			value := item.value.Index
			shard.mu.Unlock()
			return value, nil
		}
		loadGeneration = c.generation.Load()
		if current := shard.inFlight[key]; current != nil && current.generation == loadGeneration {
			shard.mu.Unlock()
			<-current.done
			return current.value, current.err
		}
		if len(shard.inFlight) >= c.maxEntriesPerShard && shard.inFlight[key] == nil {
			var done <-chan struct{}
			for _, current := range shard.inFlight {
				done = current.done
				break
			}
			shard.mu.Unlock()
			<-done
			continue
		}
		if shard.inFlight == nil {
			shard.inFlight = make(map[K]*cacheLoad[V])
		}
		pending = &cacheLoad[V]{done: make(chan struct{}), generation: loadGeneration}
		shard.inFlight[key] = pending
		shard.mu.Unlock()
		break
	}
	var value V
	var ttl time.Duration
	var loadErr error
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		if c.remote != nil {
			if cached, ok := c.Get(key); ok {
				value = cached
				return
			}
		}
		value, ttl, loadErr = loader()
		if loadErr == nil {
			c.SetIfGeneration(key, value, ttl, loadGeneration)
		}
	}()
	if panicValue != nil {
		loadErr = fmt.Errorf("cache loader panicked: %v", panicValue)
	}
	shard.mu.Lock()
	pending.value, pending.err = value, loadErr
	if shard.inFlight[key] == pending {
		delete(shard.inFlight, key)
	}
	close(pending.done)
	shard.mu.Unlock()
	if panicValue != nil {
		panic(panicValue)
	}
	return value, loadErr
}

// Delete removes one key and prevents an overlapping load from repopulating it.
func (c *TTLCache[K, V]) Delete(key K) {
	c.generation.Add(1)
	shard := c.getShard(key)
	shard.mu.Lock()
	item, _ := c.items.Load(key)
	if item != nil {
		c.removeLocked(shard, item)
	}
	shard.mu.Unlock()
	if item != nil {
		item.value.Blob.Delete()
	}
}

// Clear removes all retained values.
func (c *TTLCache[K, V]) Clear() {
	c.generation.Add(1)
	var blobs []*cache.Blob
	for _, shard := range c.shards {
		shard.mu.Lock()
		for _, item := range shard.slots {
			if item == nil {
				continue
			}
			c.items.Delete(item.key)
			if item.value.Blob != nil {
				blobs = append(blobs, item.value.Blob)
			}
		}
		shard.slots, shard.free, shard.hand = nil, nil, 0
		shard.mu.Unlock()
	}
	cache.DeleteBlobs(blobs)
}

// DeleteFunc removes values selected by predicate.
func (c *TTLCache[K, V]) DeleteFunc(predicate func(key K, value V) bool) {
	c.generation.Add(1)
	var blobs []*cache.Blob
	for _, shard := range c.shards {
		shard.mu.Lock()
		for _, item := range shard.slots {
			if item != nil && predicate(item.key, item.value.Index) {
				c.removeLocked(shard, item)
				if item.value.Blob != nil {
					blobs = append(blobs, item.value.Blob)
				}
			}
		}
		shard.mu.Unlock()
	}
	cache.DeleteBlobs(blobs)
}

// EvictExpired eagerly removes expired values; active readers never wait for the sweep.
func (c *TTLCache[K, V]) EvictExpired() {
	now := time.Now().UnixMilli()
	var blobs []*cache.Blob
	for _, shard := range c.shards {
		shard.mu.Lock()
		for _, item := range shard.slots {
			if item != nil && now >= item.expiredAt {
				c.removeLocked(shard, item)
				if item.value.Blob != nil {
					blobs = append(blobs, item.value.Blob)
				}
			}
		}
		shard.mu.Unlock()
	}
	cache.DeleteBlobs(blobs)
}
