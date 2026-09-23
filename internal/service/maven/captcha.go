/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package maven

import (
	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/captcha"

	"github.com/gofiber/fiber/v3"
)

func requireNewPackageCaptcha(c fiber.Ctx, state *core.AppState, repo *config.Repository, path string) error {
	coordinate, valid := ParseArtifactPath(path)
	if !valid {
		return nil
	}
	exists, err := state.GetDB().MavenArtifactExists(repo.Name, coordinate.GroupID, coordinate.ArtifactID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return captcha.Require(c, state, config.CaptchaPackageCreate)
}
