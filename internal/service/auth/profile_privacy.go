/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package auth

import (
	"errors"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"

	"github.com/gofiber/fiber/v3"
)

func canReadUserProfile(viewer *config.User, profile *core.UserProfile) bool {
	return profile != nil && (!profile.Private || viewer != nil &&
		(strings.EqualFold(viewer.Username, profile.Username) || viewer.CanViewPrivateProfiles()))
}

func updateProfilePrivacy(c fiber.Ctx, state *core.AppState) error {
	setPrivateResponseHeaders(c)
	user, err := requireAccountSession(c)
	if err != nil {
		return accountSessionError(c, err)
	}
	session, _ := c.Locals("current_session_id").(string)
	if c.Cookies(sessionCookieName) != session {
		return c.SendStatus(fiber.StatusForbidden)
	}
	var request struct {
		UserID  string `json:"user_id"`
		Private *bool  `json:"private"`
	}
	if err := readMFARequest(c, &request); err != nil {
		return err
	}
	if request.Private == nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	err = state.GetDB().SetUserProfilePrivacy(user.Username, session, request.UserID, *request.Private)
	if errors.Is(err, core.ErrUserProfileNotFound) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	if errors.Is(err, core.ErrEmailCodeInvalid) || errors.Is(err, core.ErrAccountDeleted) {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	if err != nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	username, operator, method, publicSession, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{Username: username, Operator: operator, AuthMethod: method,
		SessionID: publicSession, IP: ip, Action: audit.ActionProfileUpdate, Details: "Updated profile privacy"})
	return ownUserProfile(c, state)
}
