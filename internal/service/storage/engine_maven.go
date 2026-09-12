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

	"github.com/gofiber/fiber/v3"
)

func mavenEngine() repositoryEngine {
	return repositoryEngine{
		read: func(state *core.AppState, user *config.User, repo *config.Repository, path string, root bool) (bool, error) {
			if MavenReadAuthorizer != nil {
				return MavenReadAuthorizer(state, user, repo, path, root)
			}
			return readRepositoryFiles(state, user, repo, path, root)
		},
		authorizeWrite: func(c fiber.Ctx, state *core.AppState, user *config.User, repo *config.Repository, path string) (bool, error) {
			if MavenMutationAuthorizer == nil {
				return true, c.Status(fiber.StatusServiceUnavailable).SendString("Maven domain authorization is unavailable")
			}
			level := core.MavenPermissionPublish
			if c.Method() == fiber.MethodDelete {
				level = core.MavenPermissionVersion
			}
			if err := MavenMutationAuthorizer(state, user, repo, path, level); err != nil {
				return true, mavenMutationError(c, err)
			}
			return false, nil
		},
	}
}
