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
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/testutil"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestPrivateProfilesAndSuggestionsRequireStaffAuthority(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "privacy.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	state := core.NewAppState()
	state.Inner.DB = db
	state.Inner.Config.Store(config.DefaultConfig())
	now := time.Now().UnixMilli()
	for name, roles := range map[string][]string{
		"alice": {"base"}, "alina": {"base"}, "bobby": {"base"}, "admin": {"admin"},
		"globalmod": {"canmoderate:*"}, "scopedmod": {"canmoderate:private"},
	} {
		require.NoError(t, db.SaveToken(&core.AccessToken{Name: name, Permissions: roles, EncryptedSecret: "configured-password"}))
		session := &core.Session{PublicID: name, Username: name, CreatedAt: now, LoginMethod: "password"}
		session.LastActive.Store(now)
		require.NoError(t, state.SaveSession(session, name+"-session"))
	}
	profile, err := db.GetUserProfile("alice")
	require.NoError(t, err)
	avatar, err := normalizeAvatar(avatarPNG(t, 256, 256), "image/png", int64(config.DefaultAvatarMaxSizeBytes))
	require.NoError(t, err)
	avatar.UpdatedAt = now
	require.NoError(t, db.PutUserAvatar("alice", avatar))
	app := fiber.New()
	app.Use(AuthMiddleware(state))
	SetupAuthRoutes(app.Group("/api"), state, nil)
	request := func(method, path, name, body string, status int) string {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if name != "" {
			r.Header.Set("Cookie", sessionCookieName+"="+name+"-session")
		}
		response, err := app.Test(r)
		require.NoError(t, err)
		data, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, status, response.StatusCode, string(data))
		return string(data)
	}
	const privacyPath = "/api/auth/profile/privacy"
	payload := fmt.Sprintf(`{"user_id":%q,"private":true}`, profile.UserID)
	request("GET", "/api/users/profiles?names=alice", "", "", 200) // Prime the batch cache before changing visibility.
	request("PUT", privacyPath, "", payload, 401)
	request("PUT", privacyPath, "bobby", payload, 403)
	request("PUT", privacyPath, "alice", `{}`, 400)
	require.Contains(t, request("PUT", privacyPath, "alice", payload, 200), `"private":true`)
	for _, endpoint := range []string{"profile", "avatar?v=" + avatar.SHA256, "memberships?format=cargo", "super-teams"} {
		path := "/api/users/alice/" + endpoint
		for _, name := range []string{"", "bobby"} {
			request("GET", path, name, "", 404)
		}
		for _, name := range []string{"alice", "admin", "globalmod", "scopedmod"} {
			request("GET", path, name, "", 200)
		}
	}
	require.NotContains(t, request("GET", "/api/users/profiles?names=alice,alina", "bobby", "", 200), `"username":"alice"`)
	require.Contains(t, request("GET", "/api/users/profiles?names=alice,alina", "globalmod", "", 200), `"username":"alice"`)
	require.Contains(t, request("GET", "/api/users/profiles?names=alice,alina", "scopedmod", "", 200), `"username":"alice"`)
	names, err := db.SearchTokenNames("ali", 1, now, false)
	require.NoError(t, err)
	require.Equal(t, []string{"alina"}, names, "privacy filtering must happen before LIMIT")
	names, err = db.SearchTokenNames("ali", 1, now, true)
	require.NoError(t, err)
	require.Equal(t, []string{"alice"}, names)
	require.NoError(t, db.CreateSuperTeam(&core.SuperTeam{Prefix: "privacy-team", Name: "Privacy", CreatedAt: now}, "bobby", 5, 5))
	invitation := &core.SuperTeamInvitation{ID: "00000000-0000-4000-8000-000000000302", TeamPrefix: "privacy-team",
		Inviter: "bobby", Recipient: "alice", Level: core.SuperTeamRoleRead, CreatedAt: now, ExpiresAt: now + 600000}
	message := &core.UserMessage{ID: invitation.ID, Recipient: "alice", Sender: "bobby", Kind: "super_team_invite", Severity: "info",
		Title: "Invitation", Body: "Invitation", ActionKind: "super_team_invite", ActionStatus: core.MessageActionPending,
		CreatedAt: now, ExpiresAt: invitation.ExpiresAt}
	require.NoError(t, db.CreateSuperTeamInvitations([]*core.SuperTeamInvitation{invitation}, []*core.UserMessage{message}), "private users remain explicitly invitable")
	public := fmt.Sprintf(`{"user_id":%q,"private":false}`, profile.UserID)
	request("PUT", privacyPath, "alice", public, 200)
	request("GET", "/api/users/alice/profile", "", "", 200)
	require.Contains(t, request("GET", "/api/users/profiles?names=alice", "", "", 200), `"username":"alice"`)
	_, err = state.RevokeSession("alice-session")
	require.NoError(t, err)
	request("PUT", privacyPath, "alice", payload, 401)
}
