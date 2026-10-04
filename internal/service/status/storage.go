/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package status

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"

	"renop/internal/core"
	"renop/internal/service/updater"
)

func isUploadPartialName(name string) bool {
	base := filepath.Base(name)
	if base == "" {
		return false
	}
	if strings.HasSuffix(base, ".tmp") || strings.Contains(base, ".tmp.") {
		return true
	}
	return strings.Contains(base, ".chunk.")
}

var updaterTempPrefixes = []string{
	"renop-download-",
	"renop-inner-",
	"renop-new-",
}

var uploadTempPrefixes = []string{
	"renop-chunk-upload-",
	"renop-upload-",
}

// GetStorageStats inspects disk, repository, database, updater, and temporary file occupancies.
func GetStorageStats(state *core.AppState) (*core.StorageStats, error) {
	storagePath := "storage"
	if state != nil {
		if cfgVal := state.Inner.Config.Load(); cfgVal != nil && cfgVal.StoragePath != "" {
			storagePath = cfgVal.StoragePath
		}
	}

	targetDir := "."
	if absPath, err := filepath.Abs(storagePath); err == nil {
		targetDir = absPath
	}

	var diskTotal, diskUsed, diskFree uint64
	if usage, err := disk.Usage(targetDir); err == nil && usage.Total > 0 {
		diskTotal = usage.Total
		diskUsed = usage.Used
		diskFree = usage.Free
	} else if usage, err := disk.Usage("."); err == nil && usage.Total > 0 {
		diskTotal = usage.Total
		diskUsed = usage.Used
		diskFree = usage.Free
	}

	var renopUsed uint64
	if state != nil && state.Inner != nil && state.Inner.FileIndex != nil {
		renopUsed = state.Inner.FileIndex.TotalFileBytes()
	} else {
		renopUsed = walkStorageBytes(storagePath)
	}

	// 1. Scan updater residuals
	updaterCount, updaterBytes := scanUpdaterResiduals()

	// 2. Scan temporary files (chunks and orphan partials)
	tempCount, tempBytes := scanTempFiles(storagePath)

	// 3. Database stats
	var dbBytes uint64
	if state != nil {
		if db := state.GetDB(); db != nil {
			if dbStats, err := db.GetDatabaseStats(); err == nil && dbStats != nil {
				dbBytes = dbStats.TotalSizeBytes
			}
		}
	}

	items := []core.StorageItem{
		{
			Category:  "repositories",
			Name:      "RenoP Package Repositories",
			SizeBytes: renopUsed,
		},
		{
			Category:  "database",
			Name:      "Database Persistence",
			SizeBytes: dbBytes,
		},
		{
			Category:  "updater",
			Name:      "Updater Residuals & Binaries",
			SizeBytes: updaterBytes,
			Count:     updaterCount,
		},
		{
			Category:  "temp",
			Name:      "Temporary Uploads & Chunks",
			SizeBytes: tempBytes,
			Count:     tempCount,
		},
	}

	accounted := renopUsed + dbBytes + updaterBytes + tempBytes
	otherSystem := uint64(0)
	if diskUsed > accounted {
		otherSystem = diskUsed - accounted
	}
	items = append(items, core.StorageItem{
		Category:  "system",
		Name:      "System & Other Files",
		SizeBytes: otherSystem,
	})

	return &core.StorageStats{
		RenopUsedBytes:        renopUsed,
		DiskUsedBytes:         diskUsed,
		DiskTotalBytes:        diskTotal,
		DiskFreeBytes:         diskFree,
		TempFilesCount:        tempCount,
		TempFilesBytes:        tempBytes,
		UpdaterResidualsCount: updaterCount,
		UpdaterResidualsBytes: updaterBytes,
		Items:                 items,
	}, nil
}

// CleanupStorage removes updater residuals, stale OS temp files, and orphan upload chunks.
func CleanupStorage(state *core.AppState) (*core.StorageCleanupResult, error) {
	storagePath := "storage"
	if state != nil {
		if cfgVal := state.Inner.Config.Load(); cfgVal != nil && cfgVal.StoragePath != "" {
			storagePath = cfgVal.StoragePath
		}
	}

	var cleanedFiles int
	var reclaimedBytes uint64

	// Helper to remove and count
	removeFile := func(path string) {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			size := uint64(fi.Size())
			if os.Remove(path) == nil {
				cleanedFiles++
				reclaimedBytes += size
			}
		}
	}

	// 1. Remove old executable backups (.old and .bak) and .renop-new-* in executable dir
	if exePath, err := os.Executable(); err == nil {
		if eval, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = eval
		}
		removeFile(exePath + ".old")
		removeFile(exePath + ".bak")

		exeDir := filepath.Dir(exePath)
		if entries, err := os.ReadDir(exeDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasPrefix(e.Name(), ".renop-new-") {
					removeFile(filepath.Join(exeDir, e.Name()))
				}
			}
		}
	}

	// 2. Clean updater and upload temps in OS temp dir
	osTemp := os.TempDir()
	if entries, err := os.ReadDir(osTemp); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			match := false
			for _, p := range updaterTempPrefixes {
				if strings.HasPrefix(name, p) {
					match = true
					break
				}
			}
			if !match {
				for _, p := range uploadTempPrefixes {
					if strings.HasPrefix(name, p) {
						match = true
						break
					}
				}
			}
			if match {
				removeFile(filepath.Join(osTemp, name))
			}
		}
	}

	// 3. Clean orphan upload partials in storagePath
	if absStorage, err := filepath.Abs(storagePath); err == nil {
		_ = filepath.WalkDir(absStorage, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if isUploadPartialName(d.Name()) {
				removeFile(path)
			}
			return nil
		})
	}

	// Trigger updater routine
	updater.CleanOldExecutables()

	// Update cached disk stats
	if state != nil {
		UpdateDiskStats(state)
	}

	return &core.StorageCleanupResult{
		CleanedFiles:   cleanedFiles,
		ReclaimedBytes: reclaimedBytes,
	}, nil
}

func scanUpdaterResiduals() (int, uint64) {
	var count int
	var totalBytes uint64

	checkFile := func(path string) {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			count++
			totalBytes += uint64(fi.Size())
		}
	}

	if exePath, err := os.Executable(); err == nil {
		if eval, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = eval
		}
		checkFile(exePath + ".old")
		checkFile(exePath + ".bak")

		exeDir := filepath.Dir(exePath)
		if entries, err := os.ReadDir(exeDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasPrefix(e.Name(), ".renop-new-") {
					if info, err := e.Info(); err == nil {
						count++
						totalBytes += uint64(info.Size())
					}
				}
			}
		}
	}

	osTemp := os.TempDir()
	if entries, err := os.ReadDir(osTemp); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			for _, p := range updaterTempPrefixes {
				if strings.HasPrefix(name, p) {
					if info, err := e.Info(); err == nil {
						count++
						totalBytes += uint64(info.Size())
					}
					break
				}
			}
		}
	}

	return count, totalBytes
}

func scanTempFiles(storagePath string) (int, uint64) {
	var count int
	var totalBytes uint64

	osTemp := os.TempDir()
	if entries, err := os.ReadDir(osTemp); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			for _, p := range uploadTempPrefixes {
				if strings.HasPrefix(name, p) {
					if info, err := e.Info(); err == nil {
						count++
						totalBytes += uint64(info.Size())
					}
					break
				}
			}
		}
	}

	if absStorage, err := filepath.Abs(storagePath); err == nil {
		_ = filepath.WalkDir(absStorage, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil || d.IsDir() {
				return nil
			}
			if isUploadPartialName(d.Name()) {
				if info, err := d.Info(); err == nil {
					count++
					totalBytes += uint64(info.Size())
				}
			}
			return nil
		})
	}

	return count, totalBytes
}
