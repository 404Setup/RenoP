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
	"renop/internal/config"
	"renop/internal/core"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// repositoryEngine composes protocol-owned operations around the common path,
// storage, cache and download pipeline. Package mutation handlers retain their
// own live package/team checks; direct file writes use authorizeWrite instead.
type repositoryEngine struct {
	prepare        func(fiber.Ctx, *core.AppState, *config.Repository, string, string) (string, bool, error)
	read           func(*core.AppState, *config.User, *config.Repository, string, bool) (bool, error)
	deniedRead     func(fiber.Ctx, *config.User, *config.Repository, string) (bool, error)
	authorizeWrite func(fiber.Ctx, *core.AppState, *config.User, *config.Repository, string) (bool, error)
	handle         func(fiber.Ctx, *core.AppState, *config.Repository, string, string, bool) (bool, error)
}

var repositoryEngines = map[string]repositoryEngine{
	config.RepositoryFormatMaven:       mavenEngine(),
	config.RepositoryFormatFiles:       filesEngine(),
	config.RepositoryFormatCargo:       cargoEngine(),
	config.RepositoryFormatNPM:         npmEngine(),
	config.RepositoryFormatDocker:      dockerEngine(),
	config.RepositoryFormatConda:       condaEngine(),
	config.RepositoryFormatConan:       conanEngine(),
	config.RepositoryFormatCondaNative: condaEngine(),
	config.RepositoryFormatAPK:         apkEngine(),
	config.RepositoryFormatAPT:         aptEngine(),
	config.RepositoryFormatRPM:         rpmEngine(),
	config.RepositoryFormatYUM:         rpmEngine(),
}

func filesEngine() repositoryEngine {
	return repositoryEngine{
		read: readRepositoryFiles,
		authorizeWrite: func(c fiber.Ctx, _ *core.AppState, user *config.User, repo *config.Repository, _ string) (bool, error) {
			if user == nil || !user.CheckUpdatePermission(repo.Name) {
				return true, c.Status(fiber.StatusForbidden).SendString("Forbidden")
			}
			return false, nil
		},
	}
}

func isRepositoryRead(c fiber.Ctx) bool {
	return c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead
}

func readRepositoryFiles(_ *core.AppState, user *config.User, repo *config.Repository, path string, root bool) (bool, error) {
	if repo == nil {
		return false, nil
	}
	return strings.EqualFold(repo.Visibility, "PUBLIC") || (user != nil && user.CheckReadPermission(repo.Name, path, repo.Visibility, root)), nil
}
