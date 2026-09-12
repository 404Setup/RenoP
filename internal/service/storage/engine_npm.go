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
	"renop/internal/service/npm"

	"github.com/gofiber/fiber/v3"
)

func npmEngine() repositoryEngine {
	handler := npm.Handler{Store: packageStore{}}
	return repositoryEngine{
		read: npm.CanReadRepository,
		prepare: func(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, path string) (string, bool, error) {
			if handled, err := handler.Handle(c, state, repo, storagePath, path); handled {
				return path, true, err
			}
			if !isRepositoryRead(c) {
				return path, true, c.Status(fiber.StatusMethodNotAllowed).SendString("npm repositories must be modified through npm registry endpoints")
			}
			normalized, valid := npm.NormalizeRegistryPath(path)
			if !valid {
				return path, true, c.Status(fiber.StatusBadRequest).SendString("Bad Request")
			}
			return normalized, false, nil
		},
	}
}
