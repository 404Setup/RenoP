/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package ticket

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/time/rate"

	"renop/internal/core"
	"renop/internal/utils"
	"renop/internal/utils/ratelimit"
)

// ticketAdmission bounds repeated reads and rejected writes before they reach the database.
// Durable creation/comment limits additionally follow the immutable account across restarts.
func ticketAdmission(state *core.AppState) fiber.Handler {
	reads := ratelimit.New(rate.Limit(5), 60, 10000)
	writes := ratelimit.New(rate.Limit(1), 12, 10000)
	network := ratelimit.New(rate.Limit(20), 120, 10000)
	return func(c fiber.Ctx) error {
		username, _, err := currentUser(c)
		if err != nil {
			return reviewError(c, err)
		}
		ip := utils.ExtractIP(c, &state.Inner.Config.Load().Server)
		delay := network.Allow(ratelimit.NetworkKey(ip))
		if delay == 0 {
			gate := writes
			if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead {
				gate = reads
			}
			delay = gate.Allow(username)
		}
		if delay > 0 {
			c.Set(fiber.HeaderRetryAfter, strconv.FormatInt(int64(max(1, (delay+time.Second-1)/time.Second)), 10))
			c.Set(fiber.HeaderCacheControl, "no-store")
			return reviewError(c, core.ErrReviewFileLimit)
		}
		return c.Next()
	}
}
