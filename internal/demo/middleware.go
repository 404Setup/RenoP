/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package demo

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"
)

func denied(c fiber.Ctx, code string) error {
	c.Set("X-Renop-Error-Code", code)
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": code})
}

// Middleware denies mutations before request staging, while retaining the normal authorization checks.
func Middleware(state *core.AppState) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !state.IsDemo() {
			return c.Next()
		}
		decoded, err := url.PathUnescape(c.Path())
		if err != nil {
			return fiber.ErrBadRequest
		}
		clean := strings.ToLower(strings.TrimSuffix(path.Clean(decoded), "/"))
		method := c.Method()
		if method == fiber.MethodOptions {
			return c.Next()
		}
		if method == fiber.MethodPost && (clean == "/api/auth/login" || clean == "/api/auth/logout") {
			return c.Next()
		}
		if method == fiber.MethodGet || method == fiber.MethodHead {
			if strings.HasPrefix(clean, "/api/auth/oauth/") && clean != "/api/auth/oauth/providers" ||
				strings.HasPrefix(clean, "/api/auth/github/") && clean != "/api/auth/github/status" ||
				strings.HasPrefix(clean, "/api/updater/") && clean != "/api/updater/status" ||
				strings.HasPrefix(clean, "/api/debug/") || strings.HasPrefix(clean, "/api/status/debug") {
				return denied(c, "demo_read_only")
			}
			if strings.HasPrefix(clean, "/api/tickets/") && strings.Contains(clean, "/files/") || strings.HasPrefix(clean, "/v2/") || strings.HasPrefix(clean, "/javadoc/") || strings.HasPrefix(clean, "/cargodoc/") {
				return denied(c, "demo_file_unavailable")
			}
			cfg := state.Inner.Config.Load()
			parts := strings.SplitN(strings.TrimPrefix(decoded, "/"), "/", 2)
			if repo := cfg.Maven.Repositories[strings.ToLower(parts[0])]; repo != nil {
				local := filepath.Join(cfg.StoragePath, filepath.FromSlash(strings.TrimPrefix(decoded, "/")))
				relative := ""
				if len(parts) == 2 {
					relative = strings.Trim(parts[1], "/")
				}
				if repo.NormalizedFormat() == config.RepositoryFormatCargo && cargoMetadataPath(relative) {
					return c.Next()
				}
				if state.Inner.FileIndex != nil && !state.Inner.FileIndex.HasFile(local) {
					if state.Inner.FileIndex.HasDir(local) {
						return c.Next()
					}
					format := repo.NormalizedFormat()
					packagePage := format == config.RepositoryFormatDocker ||
						format != config.RepositoryFormatFiles && (relative == "packages" || strings.HasPrefix(relative, "packages/"))
					if packagePage && strings.Contains(strings.ToLower(c.Get(fiber.HeaderAccept)), fiber.MIMETextHTML) {
						return c.Next()
					}
				}
				return denied(c, "demo_file_unavailable")
			}
			return c.Next()
		}
		if state.Inner.Config.Load().Runtime.DemoTemp {
			if method == fiber.MethodPut && strings.HasPrefix(clean, "/api/settings/") {
				return c.Next()
			}
			if method == fiber.MethodDelete {
				for _, prefix := range []string{"/api/settings/repositories/", "/api/settings/maven/repositories/"} {
					if name, ok := strings.CutPrefix(clean, prefix); ok && name != "" && !strings.Contains(name, "/") {
						return c.Next()
					}
				}
			}
		}
		return denied(c, "demo_read_only")
	}
}

// Cargo's browser shares its registry metadata routes. Only these reads bypass
// the artifact boundary; sparse index files, downloads and archive previews do not.
func cargoMetadataPath(relative string) bool {
	if relative == "config.json" || relative == "api/v1/crates" || relative == "api/v1/me/crates" {
		return true
	}
	parts := strings.Split(relative, "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "crates" || parts[3] == "" || parts[3] == "new" {
		return false
	}
	return len(parts) == 4 || len(parts) == 5 && (parts[4] == "owners" || parts[4] == "users") ||
		len(parts) == 6 && parts[5] == "docs"
}

// Info exposes only public demonstration credentials and process mode.
func Info(c fiber.Ctx, state *core.AppState) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if !state.IsDemo() {
		return protohttp.Write(c, &pb.DemoInfo{})
	}
	return protohttp.Write(c, &pb.DemoInfo{Enabled: true, Temporary: state.Inner.Config.Load().Runtime.DemoTemp,
		Username: "admin", Password: "12345678"})
}
