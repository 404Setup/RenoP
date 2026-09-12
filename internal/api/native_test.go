/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/service/auth"
	"renop/internal/service/index"
	"renop/internal/testutil"
)

func TestNativeResourceAPIRequiresOwnershipAndBrowserSession(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "native-api.db"), MaxOpenConns: 1, MaxIdleConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	state := core.NewAppState()
	state.Inner.DB = db
	state.Inner.FileIndex = index.NewFileIndex()
	cfg := config.DefaultConfig()
	cfg.Maven.Repositories["native"] = &config.Repository{Name: "native", Format: config.RepositoryFormatConda, Visibility: "PUBLIC"}
	state.Inner.Config.Store(cfg)
	for _, name := range []string{"owner", "other", "member"} {
		require.NoError(t, db.SaveToken(&core.AccessToken{Name: name, Permissions: []string{"base", "canupdate:native"}}))
		session := &core.Session{Username: name, PublicID: name, CreatedAt: time.Now().UnixMilli()}
		session.LastActive.Store(session.CreatedAt)
		require.NoError(t, db.SaveSession(session, "session-"+name))
		require.NoError(t, state.SaveSession(session, "session-"+name))
	}
	state.Inner.TokensCount.Store(3)
	app := fiber.New()
	app.Use(auth.AuthMiddleware(state))
	setupNativeRoutes(app.Group("/api"), state)
	request := func(method, suffix, user string, payload any) (int, []byte) {
		t.Helper()
		data, err := json.Marshal(payload)
		require.NoError(t, err)
		req := httptest.NewRequest(method, "/api/native/repositories/native/resources"+suffix, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if user != "" {
			req.AddCookie(&http.Cookie{Name: "renop_session", Value: "session-" + user})
		}
		response, err := app.Test(req)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return response.StatusCode, body
	}
	code, _ := request("POST", "", "", map[string]any{"name": "example"})
	require.Equal(t, 403, code)
	code, _ = request("POST", "", "owner", map[string]any{"name": "example"})
	require.Equal(t, 201, code)
	code, body := request("GET", "", "", nil)
	require.Equal(t, 200, code)
	require.NotContains(t, string(body), "example")
	code, _ = request("GET", "?name=example", "other", nil)
	require.Equal(t, 404, code)
	code, _ = request("PUT", "", "other", map[string]any{"name": "example", "description": "takeover"})
	require.Equal(t, 403, code)
	code, _ = request("PUT", "/members", "owner", map[string]any{"name": "example", "username": "member", "level": 1})
	require.Equal(t, 200, code)
	code, body = request("GET", "?name=example", "member", nil)
	require.Equal(t, 200, code)
	require.Contains(t, string(body), `"permission_level":1`)
	code, _ = request("PUT", "/members", "member", map[string]any{"name": "example", "username": "member", "level": 4})
	require.Equal(t, 403, code)
	// Test archive
	code, _ = request("PUT", "", "owner", map[string]any{"name": "example", "archived": true})
	require.Equal(t, 200, code)
	code, body = request("GET", "?name=example", "owner", nil)
	require.Equal(t, 200, code)
	require.Contains(t, string(body), `"archived":true`)

	// Test locks
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "moderator", Permissions: []string{"base", "canmoderate:native"}}))
	modSession := &core.Session{Username: "moderator", PublicID: "moderator", CreatedAt: time.Now().UnixMilli()}
	modSession.LastActive.Store(modSession.CreatedAt)
	require.NoError(t, db.SaveSession(modSession, "session-moderator"))
	require.NoError(t, state.SaveSession(modSession, "session-moderator"))

	code, _ = request("PUT", "/locks", "owner", map[string]any{"name": "example", "mode": "write", "reason": "abuse"})
	require.Equal(t, 403, code) // only moderator can lock
	code, body = request("PUT", "/locks", "moderator", map[string]any{"name": "example", "mode": "write", "reason": "abuse"})
	require.Equal(t, 200, code, string(body))
	code, body = request("GET", "?name=example", "owner", nil)
	require.Equal(t, 200, code)
	require.Contains(t, string(body), `"locks"`)
	code, _ = request("DELETE", "/locks", "moderator", map[string]any{"name": "example"})
	require.Equal(t, 200, code)

	// Test deprecate
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "outsider", Permissions: []string{"base"}}))
	outSession := &core.Session{Username: "outsider", PublicID: "outsider", CreatedAt: time.Now().UnixMilli()}
	outSession.LastActive.Store(outSession.CreatedAt)
	require.NoError(t, db.SaveSession(outSession, "session-outsider"))
	require.NoError(t, state.SaveSession(outSession, "session-outsider"))

	code, _ = request("PUT", "/deprecate", "outsider", map[string]any{"name": "example"})
	require.Equal(t, 403, code)
	code, _ = request("PUT", "/deprecate", "owner", map[string]any{"name": "example"})
	require.Equal(t, 200, code)
	code, body = request("GET", "?name=example", "owner", nil)
	require.Equal(t, 200, code)
	require.Contains(t, string(body), `"deprecated":true`)

	// Test admin delete of package with artifact
	res, err := db.GetNativeResource("native", "example", "owner")
	require.NoError(t, err)
	require.NoError(t, db.SaveNativeArtifact(core.NativeArtifact{
		Repository: "native", ResourceID: res.ID, Name: "example", Version: "1.0.0", Path: "example-1.0.0.tar.bz2", Size: 10, Published: true, CreatedAt: time.Now().UnixMilli(),
	}, "owner", true))
	code, body = request("GET", "?name=example", "owner", nil)
	require.Equal(t, 200, code)
	require.Contains(t, string(body), `"versions"`)
	// Test delete specific version
	require.NoError(t, db.SaveNativeArtifact(core.NativeArtifact{
		Repository: "native", ResourceID: res.ID, Name: "example", Version: "2.0.0", Path: "example-2.0.0.tar.bz2", Size: 20, Published: true, CreatedAt: time.Now().UnixMilli(),
	}, "owner", true))
	code, body = request("GET", "?name=example", "owner", nil)
	require.Equal(t, 200, code)
	require.Contains(t, string(body), `"2.0.0"`)
	// outsider cannot delete version
	code, _ = request("DELETE", "", "outsider", map[string]any{"name": "example", "version": "2.0.0"})
	require.Equal(t, 403, code)
	// owner can delete specific version
	code, _ = request("DELETE", "", "owner", map[string]any{"name": "example", "version": "2.0.0"})
	require.Equal(t, 200, code)
	code, body = request("GET", "?name=example", "owner", nil)
	require.Equal(t, 200, code)
	require.NotContains(t, string(body), `"2.0.0"`)
	require.Contains(t, string(body), `"1.0.0"`)

	code, _ = request("DELETE", "", "owner", map[string]any{"name": "example"})
	require.Equal(t, 409, code) // non-manager cannot delete package with artifacts
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "admin", Permissions: []string{"base", "manager"}}))
	adminSession := &core.Session{Username: "admin", PublicID: "admin", CreatedAt: time.Now().UnixMilli()}
	adminSession.LastActive.Store(adminSession.CreatedAt)
	require.NoError(t, db.SaveSession(adminSession, "session-admin"))
	require.NoError(t, state.SaveSession(adminSession, "session-admin"))
	code, _ = request("DELETE", "", "admin", map[string]any{"name": "example"})
	require.Equal(t, 200, code) // manager can directly delete package with artifacts!
}
