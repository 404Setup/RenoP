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
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"renop/internal/core"
	"renop/internal/repositorycapacity"
	"renop/internal/service/index"
	"renop/internal/service/packagestore"
	"renop/internal/service/status"
)

// packageStore adapts package protocols to the existing atomic storage pipeline.
type packageStore struct{}

type packageStagedFile struct {
	targetPath string
	tempPath   string
	file       *os.File
	closed     bool
	committed  bool
}

var _ packagestore.Store = packageStore{}
var _ packagestore.StagedFile = (*packageStagedFile)(nil)

// NewPackageStore returns the shared Disk/S3 package persistence adapter.
func NewPackageStore() packagestore.Store {
	return packageStore{}
}

func (packageStore) Open(path string) (io.ReadCloser, bool, error) {
	reader, _, found, err := backendFor(path).Open(path)
	return reader, found, err
}

func (packageStore) Exists(path string) (bool, error) {
	_, found, err := backendFor(path).Stat(path)
	return found, err
}

func (packageStore) Stage(path string) (packagestore.StagedFile, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	tempPath := path + ".tmp.package." + uuid.NewString()
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &packageStagedFile{targetPath: path, tempPath: tempPath, file: file}, nil
}

func (staged *packageStagedFile) Write(data []byte) (int, error) {
	if staged == nil || staged.file == nil || staged.closed {
		return 0, fs.ErrClosed
	}
	return staged.file.Write(data)
}

func (staged *packageStagedFile) Close() error {
	if staged == nil || staged.file == nil || staged.closed {
		return nil
	}
	staged.closed = true
	return staged.file.Close()
}

func (staged *packageStagedFile) Open() (io.ReadCloser, error) {
	if staged == nil || staged.tempPath == "" || staged.committed {
		return nil, fs.ErrNotExist
	}
	if err := staged.Close(); err != nil {
		return nil, err
	}
	return os.Open(staged.tempPath)
}

func (staged *packageStagedFile) Size() (int64, error) {
	if staged == nil || staged.tempPath == "" {
		return 0, fs.ErrNotExist
	}
	if err := staged.Close(); err != nil {
		return 0, err
	}
	info, err := os.Stat(staged.tempPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (staged *packageStagedFile) Commit(state *core.AppState) error {
	if state.IsDemo() {
		return core.ErrDemoReadOnly
	}
	if staged == nil || staged.tempPath == "" || staged.targetPath == "" || staged.committed {
		return errors.New("package staged file is unavailable")
	}
	if state == nil || state.Inner == nil || state.Inner.FileIndex == nil {
		return errors.New("storage index is unavailable")
	}
	if err := staged.Close(); err != nil {
		return err
	}
	info, err := os.Stat(staged.tempPath)
	if err != nil {
		return err
	}
	capacity, err := reserveRepositoryCapacity(state, repositorycapacity.Object{Path: staged.targetPath, Size: info.Size()})
	if err != nil {
		return err
	}
	defer capacity.Release()
	if err := backendFor(staged.targetPath).Commit(staged.tempPath, staged.targetPath); err != nil {
		return err
	}
	staged.committed = true
	state.Inner.FileIndex.EnsureParentDirs(staged.targetPath)
	state.Inner.FileIndex.InsertFile(staged.targetPath, index.FileInfo{Size: info.Size(), ModTime: info.ModTime().UnixNano()})
	state.InvalidateFileCache(staged.targetPath)
	status.MarkStorageUpdated()
	capacity.Commit()
	return nil
}

func (staged *packageStagedFile) Discard() error {
	if staged == nil {
		return nil
	}
	closeErr := staged.Close()
	if staged.committed || staged.tempPath == "" {
		return closeErr
	}
	removeErr := os.Remove(staged.tempPath)
	if errors.Is(removeErr, fs.ErrNotExist) {
		removeErr = nil
	}
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func (packageStore) Delete(state *core.AppState, path string) error {
	defer invalidateRepositoryCapacity(state, path)
	if state == nil || state.Inner == nil || state.Inner.FileIndex == nil {
		return errors.New("storage index is unavailable")
	}
	if err := backendFor(path).Delete(path); err != nil {
		return err
	}
	state.Inner.FileIndex.RemoveFile(path)
	state.InvalidateFileCache(path)
	status.MarkStorageUpdated()
	return nil
}
