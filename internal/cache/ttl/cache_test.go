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
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"renop/internal/testutil"
)

func TestTTLCacheOwnsRequestBufferKeys(t *testing.T) {
	for _, load := range []bool{false, true} {
		c := NewTTLCache[string, int](time.Minute)
		buffer := []byte("session-original")
		borrowed := unsafe.String(unsafe.SliceData(buffer), len(buffer))
		if load {
			_, err := c.GetOrLoad(borrowed, func() (int, time.Duration, error) { return 7, time.Minute, nil })
			if err != nil {
				t.Fatal(err)
			}
		} else {
			c.Set(borrowed, 7, time.Minute)
		}
		copy(buffer, "session-replaced")
		c.RangeIndex(func(key string, _ int) bool {
			if key != "session-original" {
				t.Fatalf("retained borrowed key: %q", key)
			}
			return true
		})
		if value, ok := c.Get("session-original"); !ok || value != 7 {
			t.Fatal("request buffer reuse changed a cached session lookup")
		}
		c.Delete("session-original")
		if c.Len() != 0 {
			t.Fatal("original key could no longer be invalidated")
		}
	}
}

func TestTTLCacheCoalescesConcurrentLoads(t *testing.T) {
	cache := NewTTLCacheWithCapacity[string, int](time.Minute, 64)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := func() (int, time.Duration, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return 42, time.Minute, nil
	}
	results := make(chan int, 32)
	errors := make(chan error, 32)
	var workers sync.WaitGroup
	workers.Add(32)
	for range 32 {
		go func() {
			defer workers.Done()
			value, err := cache.GetOrLoad("shared", loader)
			results <- value
			errors <- err
		}()
	}
	<-started
	close(release)
	workers.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("coalesced load failed: %v", err)
		}
	}
	for value := range results {
		if value != 42 {
			t.Fatalf("coalesced value = %d, want 42", value)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("loader calls = %d, want 1", calls.Load())
	}
}

func TestTTLCacheCapacityAndInvalidationDuringLoad(t *testing.T) {
	cache := NewTTLCacheWithCapacity[string, int](time.Minute, 64)
	cache.UseRemote(testutil.RemoteCache(t), nil)
	for index := range 1024 {
		cache.Set(fmt.Sprintf("key-%d", index), index, time.Minute)
	}
	if entries := cache.Len(); entries > 64 {
		t.Fatalf("bounded cache retained %d entries, want at most 64", entries)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	loaded := make(chan int, 1)
	go func() {
		value, _ := cache.GetOrLoad("invalidated", func() (int, time.Duration, error) {
			close(started)
			<-release
			return 7, time.Minute, nil
		})
		loaded <- value
	}()
	<-started
	cache.Delete("invalidated")
	close(release)
	if value := <-loaded; value != 7 {
		t.Fatalf("in-flight value = %d, want 7", value)
	}
	if _, ok := cache.Get("invalidated"); ok {
		t.Fatal("an invalidated in-flight load repopulated the cache")
	}
	staleGeneration := cache.Generation()
	cache.Delete("generation-guard")
	if cache.SetIfGeneration("generation-guard", 8, time.Minute, staleGeneration) {
		t.Fatal("stale direct load bypassed the cache generation guard")
	}
	freshGeneration := cache.Generation()
	if !cache.SetIfGeneration("generation-guard", 9, time.Minute, freshGeneration) {
		t.Fatal("current direct load was not cached")
	}
}

func TestTTLCacheLoaderPanicDoesNotPoisonKey(t *testing.T) {
	cache := NewTTLCacheWithCapacity[string, int](time.Minute, 64)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("cache loader panic was not propagated")
			}
		}()
		_, _ = cache.GetOrLoad("panic", func() (int, time.Duration, error) {
			panic("loader failure")
		})
	}()
	value, err := cache.GetOrLoad("panic", func() (int, time.Duration, error) {
		return 9, time.Minute, nil
	})
	if err != nil || value != 9 {
		t.Fatalf("cache remained blocked after loader panic: value=%d err=%v", value, err)
	}
}

func TestTTLCacheNewRequestDoesNotJoinInvalidatedLoad(t *testing.T) {
	for _, invalidate := range []string{"delete", "clear", "predicate"} {
		t.Run(invalidate, func(t *testing.T) {
			cache := NewTTLCache[string, int](time.Minute)
			cache.UseRemote(testutil.RemoteCache(t), nil)
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(finished)
				_, _ = cache.GetOrLoad("account", func() (int, time.Duration, error) {
					close(started)
					<-release
					return 1, time.Minute, nil
				})
			}()
			<-started
			defer func() { close(release); <-finished }()
			switch invalidate {
			case "delete":
				cache.Delete("account")
			case "clear":
				cache.Clear()
			case "predicate":
				cache.DeleteFunc(func(string, int) bool { return true })
			}
			fresh := make(chan int, 1)
			go func() {
				value, _ := cache.GetOrLoad("account", func() (int, time.Duration, error) {
					return 2, time.Minute, nil
				})
				fresh <- value
			}()
			select {
			case value := <-fresh:
				if value != 2 {
					t.Fatalf("fresh load returned stale value %d", value)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("fresh request joined a load started before invalidation")
			}
		})
	}
}

func TestTTLCacheRemoteEncodingDoesNotHoldShardLock(t *testing.T) {
	remote := testutil.RemoteCache(t)
	if remote == nil {
		t.Skip("RENOP_TEST_CACHE_ADDRESS is not configured")
	}
	cache := NewTTLCache[string, int](time.Minute)
	cache.UseRemote(remote, func(value int) int {
		shard := cache.getShard("account")
		if !shard.mu.TryLock() {
			t.Error("remote value encoding holds the local shard lock")
		} else {
			shard.mu.Unlock()
		}
		return value
	})
	cache.Set("account", 1, time.Minute)
	if value, err := cache.GetOrLoad("account", func() (int, time.Duration, error) {
		t.Error("remote cache hit reached the database loader")
		return 0, 0, nil
	}); err != nil || value != 1 {
		t.Fatalf("remote cache hit = %d, %v", value, err)
	}
}

func expireCacheKey[K comparable, V any](cache *TTLCache[K, V], key K) {
	shard := cache.getShard(key)
	shard.mu.Lock()
	current, _ := cache.items.Load(key)
	item := *current
	item.expiredAt = 1
	shard.slots[item.slot] = &item
	cache.items.Store(key, &item)
	shard.mu.Unlock()
}

func cacheContains[K comparable, V any](cache *TTLCache[K, V], key K) bool {
	_, ok := cache.items.Load(key)
	return ok
}

func TestTTLCacheEvictExpired(t *testing.T) {
	cache := NewTTLCache[string, int](time.Minute)
	cache.Set("expired", 1, time.Minute)
	cache.Set("live", 2, time.Minute)
	expireCacheKey(cache, "expired")
	cache.EvictExpired()
	if cacheContains(cache, "expired") {
		t.Fatal("expired key remained after eviction")
	}
	if !cacheContains(cache, "live") {
		t.Fatal("live key was removed during eviction")
	}
}

func TestTTLCacheHitsDoNotWaitForWriters(t *testing.T) {
	c := NewTTLCache[string, int](time.Minute)
	c.Set("hot", 42, time.Minute)
	shard := c.getShard("hot")
	shard.mu.Lock()
	defer shard.mu.Unlock()
	done := make(chan bool, 1)
	go func() { value, ok := c.Get("hot"); done <- ok && value == 42 }()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("cached read failed while a writer held the shard")
		}
	case <-time.After(time.Second):
		t.Fatal("a cache hit waited for the writer lock")
	}
}

func TestTTLCacheConcurrentChurnKeepsIndexesBounded(t *testing.T) {
	c := NewTTLCacheWithCapacity[int, int](time.Minute, 64)
	var workers sync.WaitGroup
	for worker := range 16 {
		workers.Go(func() {
			for i := range 1000 {
				key := (worker*37 + i) % 256
				c.Set(key, key, time.Minute)
				if value, ok := c.Get(key); ok && value != key {
					t.Errorf("wrong cached value %d for %d", value, key)
				}
				if i%3 == 0 {
					c.Delete(key)
				}
				if i%67 == 0 {
					c.EvictExpired()
				}
				if i%151 == 0 {
					c.DeleteFunc(func(key, value int) bool { return key%7 == 0 })
				}
				if i%283 == 0 {
					c.Clear()
				}
			}
		})
	}
	workers.Wait()
	count := 0
	c.items.Range(func(key int, entry *cacheItem[int, int]) bool {
		count++
		shard := c.getShard(key)
		if shard.slots[entry.slot] != entry {
			t.Error("read index and writer slot disagree")
		}
		return true
	})
	if count != c.Len() || count > 64 {
		t.Fatalf("retained entries = %d, slots = %d", count, c.Len())
	}
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear retained cache slots")
	}
	c.items.Range(func(int, *cacheItem[int, int]) bool { t.Error("clear retained a readable entry"); return false })
}

func TestTTLCacheNonReflexiveKeysDoNotRetainUnreachableEntries(t *testing.T) {
	c := NewTTLCache[float64, int](time.Minute)
	for range 100 {
		if c.SetIfGeneration(math.NaN(), 1, time.Minute, c.Generation()) {
			t.Fatal("NaN was cached")
		}
	}
	value, err := c.GetOrLoad(math.NaN(), func() (int, time.Duration, error) { return 2, time.Minute, nil })
	if err != nil || value != 2 || c.Len() != 0 {
		t.Fatalf("uncacheable load = %d, %v, entries = %d", value, err, c.Len())
	}
}

// BenchmarkTTLCacheParallelHit measures the hot read path shared by authentication and identity lookups.
func BenchmarkTTLCacheParallelHit(b *testing.B) {
	cache := NewTTLCacheWithCapacity[string, int](time.Minute, 4096)
	cache.Set("hot", 42, time.Minute)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			value, ok := cache.Get("hot")
			if !ok || value != 42 {
				b.Fatalf("cache hit failed: value=%d ok=%v", value, ok)
			}
		}
	})
}

func BenchmarkTTLCacheParallelDistributedHit(b *testing.B) {
	cache := NewTTLCacheWithCapacity[string, int](time.Minute, 4096)
	keys := make([]string, 256)
	for i := range keys {
		keys[i] = fmt.Sprintf("account-%d", i)
		cache.Set(keys[i], i, time.Minute)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i = (i + 1) & 255
			if value, ok := cache.Get(keys[i]); !ok || value != i {
				b.Fatal("cache hit failed")
			}
		}
	})
}

func BenchmarkTTLCacheParallelChurn(b *testing.B) {
	cache := NewTTLCacheWithCapacity[uint64, uint64](time.Minute, 4096)
	for i := range uint64(4096) {
		cache.Set(i, i, time.Minute)
	}
	var workers atomic.Uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := workers.Add(1) << 40
		for pb.Next() {
			key++
			cache.Set(key, key, time.Minute)
		}
	})
}
