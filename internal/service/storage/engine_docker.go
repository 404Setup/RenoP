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

func dockerEngine() repositoryEngine {
	return repositoryEngine{
		read: readRepositoryFiles,
		handle: func(c fiber.Ctx, state *core.AppState, _ *config.Repository, _, _ string, concrete bool) (bool, error) {
			if !concrete {
				if handled, err := TryHTMLFallback(state, c); handled {
					return true, err
				}
			}
			if isRepositoryRead(c) {
				return true, c.Status(fiber.StatusOK).SendString("Docker repository must be accessed via Docker client or /v2/ API")
			}
			return true, c.Status(fiber.StatusMethodNotAllowed).SendString("Docker repositories must be modified through the Docker registry API")
		},
	}
}
