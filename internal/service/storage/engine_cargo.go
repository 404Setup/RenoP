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
	"renop/internal/service/cargo"
	"renop/internal/service/proxy"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var cargoHandler = cargo.Handler{Store: packageStore{}, UpstreamIndexExists: proxy.UpstreamArtifactExists}

func cargoEngine() repositoryEngine {
	return repositoryEngine{
		read: cargo.CanReadRepository,
		deniedRead: func(c fiber.Ctx, user *config.User, repo *config.Repository, path string) (bool, error) {
			if path == "config.json" && strings.EqualFold(repo.Visibility, "PRIVATE") && (user == nil || user.Username == "guest") {
				return true, cargo.SendAuthChallenge(c)
			}
			return false, nil
		},
		handle: func(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, path string, _ bool) (bool, error) {
			if handled, err := cargoHandler.Handle(c, state, repo, storagePath, path); handled {
				return true, err
			}
			if !isRepositoryRead(c) {
				return true, c.Status(fiber.StatusMethodNotAllowed).SendString("Cargo repositories must be modified through the Cargo registry API")
			}
			return false, nil
		},
	}
}
