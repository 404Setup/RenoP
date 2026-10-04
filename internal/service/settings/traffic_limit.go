/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package settings

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/utils/protohttp"
	renopPb "renop/pkg/pb"
)

func getTrafficLimitSettings(c fiber.Ctx, state *core.AppState) error {
	if !isManager(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	cfg := state.Inner.Config.Load().TrafficLimit
	msg := &renopPb.TrafficLimitConfig{
		Unauthenticated: &renopPb.TrafficTierConfig{
			HourlyLimitMb:     cfg.Unauthenticated.HourlyLimitMB,
			DailyLimitMb:      cfg.Unauthenticated.DailyLimitMB,
			MonthlyLimitMb:    cfg.Unauthenticated.MonthlyLimitMB,
			HourlyEnabled:     cfg.Unauthenticated.HourlyEnabled,
			DailyEnabled:      cfg.Unauthenticated.DailyEnabled,
			MonthlyEnabled:    cfg.Unauthenticated.MonthlyEnabled,
			RecoveryAmountMb:  cfg.Unauthenticated.RecoveryAmountMB,
			RecoveryIntervalS: cfg.Unauthenticated.RecoveryIntervalS,
		},
		Authenticated: &renopPb.TrafficTierConfig{
			HourlyLimitMb:     cfg.Authenticated.HourlyLimitMB,
			DailyLimitMb:      cfg.Authenticated.DailyLimitMB,
			MonthlyLimitMb:    cfg.Authenticated.MonthlyLimitMB,
			HourlyEnabled:     cfg.Authenticated.HourlyEnabled,
			DailyEnabled:      cfg.Authenticated.DailyEnabled,
			MonthlyEnabled:    cfg.Authenticated.MonthlyEnabled,
			RecoveryAmountMb:  cfg.Authenticated.RecoveryAmountMB,
			RecoveryIntervalS: cfg.Authenticated.RecoveryIntervalS,
		},
		MultiThreadMultiplier: cfg.MultiThreadMultiplier,
	}

	if strings.Contains(c.Get(fiber.HeaderAccept), protohttp.ContentType) {
		return protohttp.Write(c, msg)
	}
	return c.JSON(cfg)
}

func putTrafficLimitSettings(c fiber.Ctx, state *core.AppState) error {
	if !isManager(c) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}

	var newConfig config.TrafficLimitConfig
	if strings.Contains(c.Get(fiber.HeaderContentType), protohttp.ContentType) {
		var protoReq renopPb.TrafficLimitConfig
		if err := protohttp.Read(c, &protoReq); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid traffic limit settings")
		}
		if protoReq.Unauthenticated != nil {
			newConfig.Unauthenticated = config.TrafficQuotaTier{
				HourlyLimitMB:     protoReq.Unauthenticated.HourlyLimitMb,
				DailyLimitMB:      protoReq.Unauthenticated.DailyLimitMb,
				MonthlyLimitMB:    protoReq.Unauthenticated.MonthlyLimitMb,
				HourlyEnabled:     protoReq.Unauthenticated.HourlyEnabled,
				DailyEnabled:      protoReq.Unauthenticated.DailyEnabled,
				MonthlyEnabled:    protoReq.Unauthenticated.MonthlyEnabled,
				RecoveryAmountMB:  protoReq.Unauthenticated.RecoveryAmountMb,
				RecoveryIntervalS: protoReq.Unauthenticated.RecoveryIntervalS,
			}
		}
		if protoReq.Authenticated != nil {
			newConfig.Authenticated = config.TrafficQuotaTier{
				HourlyLimitMB:     protoReq.Authenticated.HourlyLimitMb,
				DailyLimitMB:      protoReq.Authenticated.DailyLimitMb,
				MonthlyLimitMB:    protoReq.Authenticated.MonthlyLimitMb,
				HourlyEnabled:     protoReq.Authenticated.HourlyEnabled,
				DailyEnabled:      protoReq.Authenticated.DailyEnabled,
				MonthlyEnabled:    protoReq.Authenticated.MonthlyEnabled,
				RecoveryAmountMB:  protoReq.Authenticated.RecoveryAmountMb,
				RecoveryIntervalS: protoReq.Authenticated.RecoveryIntervalS,
			}
		}
		newConfig.MultiThreadMultiplier = protoReq.MultiThreadMultiplier
	} else {
		if err := c.Bind().Body(&newConfig); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid traffic limit settings")
		}
	}

	state.Inner.ConfigWriteLock.Lock()
	current := state.Inner.Config.Load()
	next := current.DeepCopy()
	next.TrafficLimit = newConfig
	err := persistConfigSnapshot(next)
	if err == nil {
		state.Inner.Config.Store(next)
	}
	state.Inner.ConfigWriteLock.Unlock()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save traffic limit settings")
	}

	username, operator, authMethod, sessionID, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{
		Username: username, Operator: operator, Action: audit.ActionSettingsUpdate,
		Details: "Updated global traffic limits", AuthMethod: authMethod, SessionID: sessionID, IP: ip,
		CreatedAt: time.Now().UnixMilli(),
	})

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(next.TrafficLimit)
}
