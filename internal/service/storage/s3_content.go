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
	"log"
	"path/filepath"
	"strings"
	"time"

	"renop/internal/artifactstore"
	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"
	"renop/internal/service/repositorygate"
	"renop/internal/utils"
)

func s3ContentStore(cfg *config.S3Config) (artifactstore.S3, error) {
	client, err := GetS3Client(cfg)
	if err != nil {
		return artifactstore.S3{}, err
	}
	current := currentConfig.Load()
	if current == nil || client == nil {
		return artifactstore.S3{}, errors.New("S3 configuration is unavailable")
	}
	root := current.StoragePath
	prefix, err := s3ObjectKey(cfg, utils.GetS3Key(filepath.Join(root, ".renop-content-v1")))
	store := artifactstore.S3{Client: client, Bucket: cfg.Bucket, Prefix: strings.TrimSuffix(prefix, "/")}
	if metadata := newS3MetadataIndex(store); metadata != nil {
		store.Index = metadata
	}
	return store, err
}

func collectRemovedS3Namespace(ctx context.Context, removed *config.S3Config) error {
	store, err := s3ContentStore(removed)
	if err != nil {
		return err
	}
	for _, repo := range currentConfig.Load().Maven.Repositories {
		if repo.S3 == nil || !repo.S3.Enabled {
			continue
		}
		other, err := s3ContentStore(repo.S3)
		if err != nil {
			return err
		}
		if other.Client.EndpointURL().String() == store.Client.EndpointURL().String() && other.Bucket == store.Bucket && other.Prefix == store.Prefix {
			return nil
		}
	}
	// No future scheduler tick can discover removed credentials. Finish cleanup
	// while the deletion operation still owns the old configuration.
	after := ""
	for {
		next, err := store.CollectUnreferenced(ctx, after, 128)
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		after = next
	}
}

func deduplicateS3Repository(ctx context.Context, state *core.AppState, name, storagePath string, repo *config.Repository) {
	store, err := s3ContentStore(repo.S3)
	if err != nil {
		return
	}
	metadata, ok := store.Index.(*s3MetadataIndex)
	if !ok {
		return
	}
	prefix, err := s3ObjectKey(repo.S3, utils.GetS3Key(filepath.Join(storagePath, name)))
	if err != nil {
		return
	}
	prefix = strings.TrimSuffix(prefix, "/") + "/"
	metadata.files.RangeContent(metadata.scope, func(key string, record index.ContentInfo) bool {
		if ctx.Err() != nil {
			return false
		}
		if !strings.HasPrefix(key, prefix) || !record.Verified || record.Reference || record.Digest == "" || record.Size < artifactstore.S3DedupMinimumSize {
			return true
		}
		available, ok := repositorygate.TryAcquireMaintenance(name)
		if !ok {
			return true
		}
		available()
		release := repositorygate.AcquireMutation(name)
		current := state.Inner.Config.Load()
		currentRepo := current.Maven.Repositories[name]
		if current.StoragePath != storagePath || currentRepo == nil || currentRepo.S3 == nil || *currentRepo.S3 != *repo.S3 || len(currentRepo.Mirrors) > 0 {
			release()
			return false
		}
		transfer, stop := context.WithTimeout(ctx, s3TransferTimeout)
		err = store.DeduplicateKnown(transfer, key, artifactstore.S3Record(record))
		stop()
		release()
		if err != nil && ctx.Err() == nil {
			log.Printf("S3 artifact deduplication skipped %q: %v", key, err)
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		return true
	})
}

// NewS3ContentCollectionTask keeps bounded per-namespace cursors. Separate ticks
// advance the scan without repeatedly stopping at the first live content objects.
func NewS3ContentCollectionTask(state *core.AppState) func(context.Context) {
	cursors := map[string]string{}
	stagingAt := map[string]time.Time{}
	return func(ctx context.Context) {
		if state.IsDemo() {
			return
		}
		cfg := state.Inner.Config.Load()
		if cfg == nil {
			return
		}
		seen := map[string]bool{}
		for _, repo := range cfg.Maven.Repositories {
			if repo.S3 == nil || !repo.S3.Enabled {
				continue
			}
			store, err := s3ContentStore(repo.S3)
			if err != nil {
				continue
			}
			if store.Index == nil {
				continue
			}
			key := store.Client.EndpointURL().String() + "\x00" + store.Bucket + "\x00" + store.Prefix
			if seen[key] {
				continue
			}
			seen[key] = true
			bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
			next, err := store.Collect(bounded, cursors[key], 16)
			if err == nil && !time.Now().Before(stagingAt[key]) {
				err = store.CleanupStaging(bounded)
				stagingAt[key] = time.Now().Add(24 * time.Hour)
			}
			cancel()
			if err == nil {
				cursors[key] = next
			}
			if ctx.Err() != nil {
				return
			}
		}
		for key := range cursors {
			if !seen[key] {
				delete(cursors, key)
				delete(stagingAt, key)
			}
		}
	}
}
