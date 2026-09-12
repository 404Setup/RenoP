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
	"strings"

	"renop/internal/service/legal"

	"github.com/gofiber/fiber/v3"
)

const apiNoCacheControl = "no-store, no-cache, must-revalidate, private, max-age=0"

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func setAPINoCacheHeaders(c fiber.Ctx) {
	c.Set(fiber.HeaderCacheControl, apiNoCacheControl)
	c.Set(fiber.HeaderPragma, "no-cache")
	c.Set(fiber.HeaderExpires, "0")
}

// APINoCacheMiddleware prevents clients and intermediaries from retaining API
// responses, except public legal resources with explicit ETag revalidation.
// Early failures receive no-store headers before downstream middleware runs.
func APINoCacheMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !isAPIPath(c.Path()) {
			return c.Next()
		}

		setAPINoCacheHeaders(c)
		retainLegalPolicy := false
		defer func() {
			if !retainLegalPolicy {
				setAPINoCacheHeaders(c)
			}
		}()
		err := c.Next()
		status := c.Response().StatusCode()
		retainLegalPolicy = err == nil && (c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead) &&
			legal.IsPublicPath(c.Path()) && (status == fiber.StatusOK || status == fiber.StatusNotModified) &&
			len(c.Response().Header.Peek(fiber.HeaderETag)) > 0 && string(c.Response().Header.Peek(fiber.HeaderCacheControl)) == "public, no-cache"
		return err
	}
}
