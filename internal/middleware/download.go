/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
)

// repositoryRead includes metadata, HEAD, Range, missing files, and mirror requests.
// Admission runs before authentication/database work, including for authenticated clients.
func repositoryRead(method, requestPath string, cfg *config.Config) bool {
	if method != fiber.MethodGet && method != fiber.MethodHead {
		return false
	}
	requestPath = strings.TrimPrefix(requestPath, "/")
	root, rest, _ := strings.Cut(requestPath, "/")
	if root == "v2" {
		return rest != "" && rest != "token" && rest != "auth"
	}
	if strings.HasPrefix(requestPath, "api/tickets/") && strings.Contains(requestPath, "/files/") {
		return true
	}
	_, exists := cfg.Maven.Repositories[strings.ToLower(root)]
	return exists
}

func downloadRateResponse(c fiber.Ctx, delay time.Duration) error {
	c.Set(fiber.HeaderRetryAfter, strconv.FormatInt(int64(max(1, (delay+time.Second-1)/time.Second)), 10))
	c.Set(fiber.HeaderCacheControl, "no-store")
	if strings.HasPrefix(c.Path(), "/v2/") {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"errors": []fiber.Map{{
			"code": "TOOMANYREQUESTS", "message": "too many requests",
		}}})
	}
	return c.SendStatus(fiber.StatusTooManyRequests)
}
