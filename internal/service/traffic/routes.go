/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package traffic

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/auth"
	"renop/internal/utils"
	"renop/internal/utils/protohttp"
	renopPb "renop/pkg/pb"
)

func writeTrafficStatus(c fiber.Ctx, status *renopPb.TrafficQuotaStatus) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if strings.Contains(c.Get(fiber.HeaderAccept), protohttp.ContentType) {
		return protohttp.Write(c, status)
	}
	return c.JSON(status)
}

func isManagerUser(user *config.User) bool {
	if user == nil {
		return false
	}
	return user.IsManager()
}

func getOwnOrIPQuota(c fiber.Ctx, state *core.AppState) error {
	tm := GetManager(state)
	subject, _ := tm.ExtractSubject(c)
	status := tm.GetStatus(subject)
	return writeTrafficStatus(c, status)
}

func getUserQuota(c fiber.Ctx, state *core.AppState) error {
	targetUsername := strings.ToLower(strings.TrimSpace(c.Params("username")))
	if targetUsername == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid username")
	}

	caller := auth.GetUser(c)
	isSelf := caller != nil && strings.EqualFold(caller.Username, targetUsername)
	if !isSelf && !isManagerUser(caller) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	tm := GetManager(state)
	status := tm.GetStatus("user:" + targetUsername)
	return writeTrafficStatus(c, status)
}

func putUserQuota(c fiber.Ctx, state *core.AppState) error {
	caller := auth.GetUser(c)
	if !isManagerUser(caller) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	targetUsername := strings.ToLower(strings.TrimSpace(c.Params("username")))
	if targetUsername == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid username")
	}

	var req renopPb.TrafficQuotaUserOverrideRequest
	if strings.Contains(c.Get(fiber.HeaderContentType), protohttp.ContentType) {
		if err := protohttp.Read(c, &req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid request payload")
		}
	} else {
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid request payload")
		}
	}

	tm := GetManager(state)
	if err := tm.SetUserOverride(targetUsername, &req); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save user quota override")
	}

	status := tm.GetStatus("user:" + targetUsername)
	return writeTrafficStatus(c, status)
}

func getIPQuota(c fiber.Ctx, state *core.AppState) error {
	caller := auth.GetUser(c)
	if !isManagerUser(caller) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	targetIP := strings.TrimSpace(c.Params("ip"))
	if targetIP == "" {
		cfg := state.Inner.Config.Load()
		targetIP = utils.ExtractIP(c, &cfg.Server)
	}

	tm := GetManager(state)
	status := tm.GetStatus("ip:" + targetIP)
	return writeTrafficStatus(c, status)
}

// SetupRoutes registers /api/traffic-quota endpoints.
func SetupRoutes(router fiber.Router, state *core.AppState) {
	group := router.Group("/traffic-quota")
	// Public / no auth required: returns IP quota if guest, or user quota if authenticated
	group.Get("", func(c fiber.Ctx) error { return getOwnOrIPQuota(c, state) })
	group.Get("/users/:username", func(c fiber.Ctx) error { return getUserQuota(c, state) })
	group.Put("/users/:username", func(c fiber.Ctx) error { return putUserQuota(c, state) })
	group.Get("/ips/:ip", func(c fiber.Ctx) error { return getIPQuota(c, state) })
}
