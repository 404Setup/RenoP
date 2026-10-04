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

type StatusSnapshot struct {
	Timestamp   int64  `json:"timestamp"`
	UsedMemory  uint64 `json:"used_memory"`
	VssMemory   uint64 `json:"vss_memory"`
	UsedThreads uint64 `json:"used_threads"`
	OpenFiles   uint64 `json:"open_files"`
}

type TableStat struct {
	Name        string `json:"name"`
	RowCount    int64  `json:"row_count"`
	SizeBytes   int64  `json:"size_bytes"`
	Reclaimable bool   `json:"reclaimable"`
}

type DatabaseStats struct {
	Driver           string      `json:"driver"`
	TotalSizeBytes   uint64      `json:"total_size_bytes"`
	Tables           []TableStat `json:"tables"`
	ReclaimableRows  int64       `json:"reclaimable_rows"`
	ReclaimableBytes uint64      `json:"reclaimable_bytes"`
}

type DatabaseCleanupResult struct {
	CleanedRows    int64  `json:"cleaned_rows"`
	ReclaimedBytes uint64 `json:"reclaimed_bytes"`
}

type StorageItem struct {
	Category  string `json:"category"`
	Name      string `json:"name"`
	SizeBytes uint64 `json:"size_bytes"`
	Count     int    `json:"count,omitempty"`
}

type StorageStats struct {
	RenopUsedBytes        uint64        `json:"renop_used_bytes"`
	DiskUsedBytes         uint64        `json:"disk_used_bytes"`
	DiskTotalBytes        uint64        `json:"disk_total_bytes"`
	DiskFreeBytes         uint64        `json:"disk_free_bytes"`
	TempFilesCount        int           `json:"temp_files_count"`
	TempFilesBytes        uint64        `json:"temp_files_bytes"`
	UpdaterResidualsCount int           `json:"updater_residuals_count"`
	UpdaterResidualsBytes uint64        `json:"updater_residuals_bytes"`
	Items                 []StorageItem `json:"items"`
}

type StorageCleanupResult struct {
	CleanedFiles   int    `json:"cleaned_files"`
	ReclaimedBytes uint64 `json:"reclaimed_bytes"`
}

type CacheStats struct {
	TotalBytes       uint64 `json:"total_bytes"`
	EntriesCount     int    `json:"entries_count"`
	Mode             string `json:"mode"`
	FileCacheBytes   uint64 `json:"file_cache_bytes"`
	NativeIndexBytes uint64 `json:"native_index_bytes"`
	DBCacheEntries   int    `json:"db_cache_entries"`
	DBCacheBytes     uint64 `json:"db_cache_bytes"`
	RemoteCacheBytes uint64 `json:"remote_cache_bytes,omitempty"`
}

type CacheCleanupResult struct {
	ReclaimedBytes uint64 `json:"reclaimed_bytes"`
	Success        bool   `json:"success"`
}
