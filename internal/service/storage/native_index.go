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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"renop/pkg/hex"
	"slices"
	"sort"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"
	"renop/internal/service/nativesign"

	"github.com/gofiber/fiber/v3"
)

// Native parsing and rendering have bounded independent budgets. Cached records
// are addressed by immutable configuration, file index identity and file version;
// an old in-flight fill can never overwrite a newer file version's cache entry.
var nativeIndexAdmission = make(chan struct{}, 4)

var errNativeIndexNotFound = errors.New("native index not found")

func serveNativePublicKey(c fiber.Ctx, state *core.AppState, apk bool) (bool, error) {
	data, err := nativesign.PublicKey(state.Inner.Config.Load().NativeSigningKeys, apk)
	if err != nil {
		return true, c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return true, sendNativeIndex(c, data, "text/plain; charset=utf-8")
}

type nativeArtifact struct {
	Path     string
	Relative string
	Info     index.FileInfo
}

type nativeIndexSpec struct {
	root        string
	accept      func(string) bool
	parse       func(io.ReaderAt, int64, string) ([]byte, error)
	render      func([][]byte) ([]byte, error)
	contentType string
}

func serveGeneratedNativeIndex(c fiber.Ctx, state *core.AppState, repo *config.Repository, spec nativeIndexSpec) (bool, error) {
	select {
	case nativeIndexAdmission <- struct{}{}:
		defer func() { <-nativeIndexAdmission }()
	default:
		return true, c.SendStatus(fiber.StatusServiceUnavailable)
	}
	files, err := nativeArtifacts(state, spec.root, spec.accept)
	if err != nil {
		return true, c.Status(fiber.StatusServiceUnavailable).SendString("Repository index is unavailable")
	}
	if len(files) == 0 && len(repo.Mirrors) > 0 {
		return false, nil
	}
	records := make([][]byte, 0, len(files))
	totalBytes := 0
	for _, file := range files {
		key := nativeRecordKey(state, repo, file)
		data, err := state.Inner.NativeIndexCache.GetReadOnlyView(key)
		if err != nil {
			data, err = readNativeArtifact(file, func(reader io.ReaderAt, size int64) ([]byte, error) { return spec.parse(reader, size, file.Relative) })
			if err == nil {
				_ = state.Inner.NativeIndexCache.Set(key, data)
			}
		}
		if err != nil {
			return true, c.Status(fiber.StatusServiceUnavailable).SendString("Repository package metadata is unavailable")
		}
		totalBytes += len(data)
		if totalBytes > 32<<20 {
			return true, c.Status(fiber.StatusServiceUnavailable).SendString("Repository index is too large")
		}
		records = append(records, data)
	}
	current, err := nativeArtifacts(state, spec.root, spec.accept)
	if err != nil || !slices.Equal(current, files) {
		return true, c.Status(fiber.StatusServiceUnavailable).SendString("Repository changed during indexing; retry the request")
	}
	data, err := spec.render(records)
	if errors.Is(err, errNativeIndexNotFound) {
		return true, c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		return true, c.Status(fiber.StatusServiceUnavailable).SendString("Repository index is unavailable")
	}
	return true, sendNativeIndex(c, data, spec.contentType)
}

func nativeArtifacts(state *core.AppState, root string, accept func(string) bool) ([]nativeArtifact, error) {
	files := make([]nativeArtifact, 0)
	visited := 0
	exceeded := false
	state.Inner.FileIndex.Walk(root, func(filename string, info index.FileInfo, directory bool) bool {
		visited++
		if visited > 100000 {
			exceeded = true
			return false
		}
		if directory {
			return true
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
		relative = filepath.ToSlash(relative)
		if accept(relative) {
			if len(files) >= 10000 {
				exceeded = true
				return false
			}
			files = append(files, nativeArtifact{filename, relative, info})
		}
		return true
	})
	if exceeded {
		return nil, errors.New("native repository index exceeds its work limit")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Relative < files[j].Relative })
	return files, nil
}

func nativeRecordKey(state *core.AppState, repo *config.Repository, artifact nativeArtifact) string {
	return fmt.Sprintf("%p:%p:%s:%s:%d:%d:%d", state.Inner.Config.Load(), state.Inner.FileIndex,
		repo.ConfiguredFormat(), artifact.Path, artifact.Info.Size, artifact.Info.ModTime, artifact.Info.Revision)
}

func readNativeArtifact(artifact nativeArtifact, parse func(io.ReaderAt, int64) ([]byte, error)) ([]byte, error) {
	reader, size, exists, err := backendFor(artifact.Path).Open(artifact.Path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("native artifact no longer exists")
	}
	defer reader.Close()
	if size != artifact.Info.Size {
		return nil, errors.New("native artifact changed during indexing")
	}
	ranged, ok := reader.(io.ReaderAt)
	if !ok {
		return nil, errors.New("native artifact requires a ranged reader")
	}
	return parse(ranged, size)
}

func sendNativeIndex(c fiber.Ctx, content []byte, contentType string) error {
	sum := sha256.Sum256(content)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	c.Set(fiber.HeaderCacheControl, "private, no-cache")
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderETag, etag)
	if CheckNotModified(c, &etag, nil) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	c.Set(fiber.HeaderContentLength, fmt.Sprint(len(content)))
	if c.Method() == fiber.MethodHead {
		return nil
	}
	return c.Send(content)
}
