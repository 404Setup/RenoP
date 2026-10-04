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
	"context"
	"runtime"
)

func (state *AppState) GetCacheStats() *CacheStats {
	if state == nil || state.Inner == nil {
		return &CacheStats{}
	}
	cfg := state.Inner.Config.Load()
	mode := "memory"
	if cfg != nil && cfg.Cache.Mode != "" {
		mode = cfg.Cache.Mode
	}

	var fileBytes, indexBytes uint64
	var fileEntries, indexEntries int
	if state.Inner.FileCache != nil {
		e, b := state.Inner.FileCache.Stats()
		fileEntries += e
		fileBytes += uint64(b)
	}
	if state.Inner.HTMLCache != nil {
		e, b := state.Inner.HTMLCache.Stats()
		fileEntries += e
		fileBytes += uint64(b)
	}
	if state.Inner.NativeIndexCache != nil {
		e, b := state.Inner.NativeIndexCache.Stats()
		indexEntries, indexBytes = e, uint64(b)
	}

	authEntries := int(state.Inner.AuthCacheEntries.Load())
	metaEntries := int(state.Inner.MetadataCacheEntries.Load())
	var dbEntries int
	var dbBytes uint64
	if db := state.GetDB(); db != nil {
		if provider, ok := db.(interface {
			GetCacheStats() (int, uint64)
		}); ok {
			dbEntries, dbBytes = provider.GetCacheStats()
		}
	}

	var remoteBytes uint64
	if state.Inner.RemoteCache != nil {
		remoteBytes, _ = state.Inner.RemoteCache.MemoryBytes(context.Background())
	}
	totalEntries := fileEntries + indexEntries + authEntries + metaEntries + dbEntries
	totalBytes := fileBytes + indexBytes + uint64(authEntries*256) + uint64(metaEntries*512) + dbBytes + remoteBytes

	return &CacheStats{
		TotalBytes:       totalBytes,
		EntriesCount:     totalEntries,
		Mode:             mode,
		FileCacheBytes:   fileBytes,
		NativeIndexBytes: indexBytes,
		DBCacheEntries:   authEntries + metaEntries + dbEntries,
		DBCacheBytes:     dbBytes,
		RemoteCacheBytes: remoteBytes,
	}
}

func (state *AppState) PurgeAllCaches() *CacheCleanupResult {
	if state == nil || state.Inner == nil {
		return &CacheCleanupResult{Success: true}
	}
	before := state.GetCacheStats()

	if state.Inner.FileCache != nil {
		state.Inner.FileCache.Clear()
	}
	if state.Inner.HTMLCache != nil {
		state.Inner.HTMLCache.Clear()
	}
	if state.Inner.NativeIndexCache != nil {
		state.Inner.NativeIndexCache.Clear()
	}
	state.ClearAuthCache()
	if state.Inner.RemoteCache != nil {
		_ = state.Inner.RemoteCache.Flush(context.Background())
	}
	if db := state.GetDB(); db != nil {
		if cdb, ok := db.(interface{ InvalidateCaches() }); ok {
			cdb.InvalidateCaches()
		}
	}
	runtime.GC()

	reclaimed := before.TotalBytes
	return &CacheCleanupResult{
		ReclaimedBytes: reclaimed,
		Success:        true,
	}
}
