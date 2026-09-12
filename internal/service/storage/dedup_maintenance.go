/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"renop/internal/artifactstore"
	"renop/internal/core"
	"renop/internal/service/repositorygate"
)

// DeduplicateExisting runs as one cancellable scheduler callback. Directory
// reads, concurrency and batch work are bounded; every pause releases all gates.
func DeduplicateExisting(ctx context.Context, state *core.AppState) {
	if state.IsDemo() {
		return
	}
	cfg := state.Inner.Config.Load()
	if cfg == nil {
		return
	}
	processed := 0
	var bytes int64
	for name, repo := range cfg.Maven.Repositories {
		if len(repo.Mirrors) > 0 {
			continue
		}
		if repo.S3 != nil && repo.S3.Enabled {
			deduplicateS3Repository(ctx, state, name, cfg.StoragePath, repo)
			if ctx.Err() != nil {
				return
			}
			continue
		}
		root := filepath.Join(cfg.StoragePath, name)
		err := walkDedupDirectory(ctx, root, 0, func(filename string, info fs.DirEntry) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if strings.Contains(info.Name(), ".tmp.") || strings.HasSuffix(info.Name(), ".tmp") || info.Name() == "conanmanifest.txt" {
				return nil
			}
			relative, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}
			if repo.IsNativeMetadata(filepath.ToSlash(relative)) {
				return nil
			}
			stat, err := info.Info()
			if err != nil {
				return nil
			}
			if !stat.Mode().IsRegular() {
				return nil
			}
			unlock, available := repositorygate.TryAcquireMaintenance(name)
			if !available {
				return nil
			}
			current := state.Inner.Config.Load()
			currentRepo := current.Maven.Repositories[name]
			if current.StoragePath != cfg.StoragePath || currentRepo == nil ||
				currentRepo.ConfiguredFormat() != repo.ConfiguredFormat() ||
				currentRepo.S3 != nil && currentRepo.S3.Enabled || len(currentRepo.Mirrors) > 0 {
				unlock()
				return context.Canceled
			}
			_, err = artifactstore.DefaultDisk.ReuseExisting(ctx, filename)
			unlock()
			if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, context.Canceled) {
				log.Printf("Artifact deduplication skipped %q: %v", filename, err)
			}
			processed++
			bytes += stat.Size()
			if processed >= 32 || bytes >= 64<<20 {
				processed = 0
				bytes = 0
				timer := time.NewTimer(time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			return ctx.Err()
		})
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Printf("Artifact deduplication scan for %q: %v", name, err)
		}
	}
}

func walkDedupDirectory(ctx context.Context, root string, depth int, visit func(string, fs.DirEntry) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 128 {
		return nil
	}
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	for {
		entries, readErr := directory.ReadDir(128)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := filepath.Join(root, entry.Name())
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				if err := walkDedupDirectory(ctx, name, depth+1, visit); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			} else if err := visit(name, entry); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}
