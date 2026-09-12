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
	"path/filepath"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/auth"
	"renop/internal/service/conan"
	"renop/internal/service/index"

	"github.com/gofiber/fiber/v3"
)

func conanEngine() repositoryEngine {
	engine := managedNativeEngine()
	engine.prepare = func(c fiber.Ctx, state *core.AppState, repo *config.Repository, _ string, relative string) (string, bool, error) {
		if isRepositoryRead(c) && (relative == "v1/ping" || relative == "v2/ping") {
			c.Set("X-Conan-Server-Capabilities", "revisions")
			return relative, true, c.SendString("RenoP")
		}
		if isRepositoryRead(c) && relative == "v2/users/authenticate" {
			grant, err := auth.IssueProtocolGrant(c, repo.Name)
			if err != nil {
				return relative, true, conanChallenge(c)
			}
			c.Set(fiber.HeaderCacheControl, "no-store")
			return relative, true, c.SendString(grant)
		}
		return relative, false, nil
	}
	engine.deniedRead = func(c fiber.Ctx, user *config.User, _ *config.Repository, _ string) (bool, error) {
		if user == nil || user.Username == "guest" {
			return true, conanChallenge(c)
		}
		return true, c.SendStatus(fiber.StatusForbidden)
	}
	baseAuthorize := engine.authorizeWrite
	engine.authorizeWrite = func(c fiber.Ctx, state *core.AppState, user *config.User, repo *config.Repository, relative string) (bool, error) {
		if user == nil || user.Username == "guest" {
			return true, conanChallenge(c)
		}
		return baseAuthorize(c, state, user, repo, relative)
	}
	engine.handle = func(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, relative string, _ bool) (bool, error) {
		if relative == "v2/users/check_credentials" && isRepositoryRead(c) {
			if user := auth.GetUser(c); user == nil || user.Username == "guest" {
				return true, conanChallenge(c)
			}
			c.Set(fiber.HeaderCacheControl, "no-store")
			return true, c.SendString("ok")
		}
		store := conanStore{state, filepath.Join(storagePath, repo.Name)}
		if relative == "v2/conans/search" && isRepositoryRead(c) {
			result, err := conan.Search(store, c.Query("q"), !strings.EqualFold(c.Query("ignorecase"), "false"))
			return true, sendConanMetadata(c, map[string]any{"results": result}, err)
		}
		p, ok := conan.Parse(relative)
		if !ok {
			// The normal browser can still list the repository root.
			if (relative == "" || relative == "/") && isRepositoryRead(c) {
				return false, nil
			}
			return true, c.SendStatus(fiber.StatusNotFound)
		}
		if (p.Operation == "file" || p.Operation == "root") && c.Method() == fiber.MethodDelete {
			err := HandleDelete(c, state, repo, relative, filepath.Join(storagePath, repo.Name, filepath.FromSlash(relative)))
			if err == nil && c.Response().StatusCode() == fiber.StatusNoContent {
				return true, c.Status(fiber.StatusOK).SendString("")
			}
			return true, err
		}
		if p.Operation == "file" && (isRepositoryRead(c) || c.Method() == fiber.MethodPut) {
			return false, nil
		}
		if !isRepositoryRead(c) {
			return true, c.SendStatus(fiber.StatusMethodNotAllowed)
		}
		switch p.Operation {
		case "revisions", "latest":
			revisions, err := conan.Revisions(store, p)
			if err != nil {
				return true, sendConanMetadata(c, nil, err)
			}
			if p.Operation == "latest" {
				return true, sendConanMetadata(c, revisions[0], nil)
			}
			return true, sendConanMetadata(c, map[string]any{"reference": p.Reference(), "revisions": revisions}, nil)
		case "files":
			files, err := conan.FileList(store, p)
			return true, sendConanMetadata(c, map[string]any{"files": files}, err)
		case "search":
			packages, err := conan.Packages(store, p, strings.EqualFold(c.Query("list_only"), "true"))
			return true, sendConanMetadata(c, packages, err)
		}
		return true, c.SendStatus(fiber.StatusNotFound)
	}
	return engine
}

func conanChallenge(c fiber.Ctx) error {
	c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="Conan"`)
	return c.SendStatus(fiber.StatusUnauthorized)
}

func sendConanMetadata(c fiber.Ctx, value any, err error) error {
	c.Set(fiber.HeaderCacheControl, "private, no-cache")
	if errors.Is(err, conan.ErrNotFound) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return c.JSON(value)
}

type conanStore struct {
	state *core.AppState
	root  string
}

func (s conanStore) Files(prefix string) ([]conan.File, error) {
	root := filepath.Join(s.root, filepath.FromSlash(prefix))
	files := make([]conan.File, 0)
	visited := 0
	s.state.Inner.FileIndex.Walk(root, func(filename string, info index.FileInfo, directory bool) bool {
		visited++
		if visited > 100000 || len(files) >= 10000 {
			return false
		}
		if directory {
			return true
		}
		relative, err := filepath.Rel(s.root, filename)
		if err != nil {
			return false
		}
		files = append(files, conan.File{Path: filepath.ToSlash(relative), Size: info.Size, ModTime: info.ModTime})
		return true
	})
	if visited > 100000 || len(files) >= 10000 {
		return nil, conan.ErrUnavailable
	}
	return files, nil
}

func (s conanStore) ReadFile(relative string, limit int64) ([]byte, error) {
	filename := filepath.Join(s.root, filepath.FromSlash(relative))
	reader, size, exists, err := backendFor(filename).Open(filename)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, conan.ErrNotFound
	}
	defer reader.Close()
	if size < 0 || size > limit {
		return nil, conan.ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != size {
		return nil, conan.ErrUnavailable
	}
	return data, nil
}
