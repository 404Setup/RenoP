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
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/repositorycapacity"
	"renop/internal/service/index"
	"renop/internal/utils"
)

func reserveRepositoryCapacity(state *core.AppState, objects ...repositorycapacity.Object) (*repositorycapacity.Reservation, error) {
	if state == nil || state.Inner == nil || len(objects) == 0 {
		return nil, nil
	}
	cfg := state.Inner.Config.Load()
	if cfg == nil {
		return nil, nil
	}
	repository, _, err := repositoryArtifactPath(state, objects[0].Path)
	if err != nil {
		return nil, nil
	}
	repo := cfg.Maven.Repositories[repository]
	if repo == nil || repo.CapacityLimitBytes == 0 {
		return nil, nil
	}
	root := filepath.Join(cfg.StoragePath, repository)
	for i := range objects {
		objects[i].Path = filepath.Clean(objects[i].Path)
		if !utils.IsSubPath(root, objects[i].Path) || capacityTemporaryPath(root, objects[i].Path) || objects[i].Size < 0 {
			return nil, repositorycapacity.ErrInvalid
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return state.Inner.RepositoryCapacity.Reserve(ctx, repository, repo.CapacityLimitBytes, objects,
		func(ctx context.Context) (int64, error) { return repositoryStoredBytes(ctx, root, repo) },
		func(path string) (int64, error) {
			if repo.S3 != nil && repo.S3.Enabled {
				info, err := StatS3(utils.GetS3Key(path))
				if isS3NotFound(err) {
					return 0, nil
				}
				return info.Size, err
			}
			info, err := os.Stat(path)
			if errors.Is(err, fs.ErrNotExist) {
				return 0, nil
			}
			if err != nil {
				return 0, err
			}
			if !info.Mode().IsRegular() {
				return 0, repositorycapacity.ErrInvalid
			}
			return info.Size(), nil
		})
}

// repositoryStoredBytes measures installed artifacts, including hidden review objects.
// Temporary upload and GPG staging copies are checked when committed into the repository.
func repositoryStoredBytes(ctx context.Context, root string, repo *config.Repository) (int64, error) {
	var total int64
	add := func(size int64) error {
		if size < 0 || total > math.MaxInt64-size {
			return repositorycapacity.ErrInvalid
		}
		total += size
		return nil
	}
	if repo.S3 != nil && repo.S3.Enabled {
		err := WalkS3Files(ctx, repo.S3, root, func(path string, info index.FileInfo) error {
			if capacityTemporaryPath(root, path) {
				return nil
			}
			return add(info.Size)
		})
		return total, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root && errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if capacityTemporaryPath(root, path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return add(info.Size())
	})
	return total, err
}

func invalidateRepositoryCapacity(state *core.AppState, path string) {
	if state == nil || state.Inner == nil || state.Inner.RepositoryCapacity == nil {
		return
	}
	if repository, _, err := repositoryArtifactPath(state, path); err == nil {
		state.Inner.RepositoryCapacity.Invalidate(repository)
	}
}

func capacityTemporaryPath(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && index.IsTemporaryPath(relative)
}
