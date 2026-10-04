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
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/testutil"
)

func setupTrafficSettingsTestApp(t *testing.T) (*fiber.App, *core.AppState) {
	t.Helper()
	t.Setenv("RENOP_CONFIG", filepath.Join(testutil.TempDir(t), "config.yaml"))
	dbPath := filepath.Join(testutil.TempDir(t), "settings-traffic-test.db")
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: "sqlite", Dsn: dbPath, MaxOpenConns: 1, MaxIdleConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	state := core.NewAppState()
	state.Inner.DB = db
	cfg := config.DefaultConfig()
	state.Inner.Config.Store(cfg)

	app := fiber.New(fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
	})

	app.Use(func(c fiber.Ctx) error {
		testUser := c.Get("X-Test-User")
		if testUser == "admin" {
			c.Locals("user", &config.User{Username: "admin", Roles: []string{"base", "manager"}})
		} else if testUser != "" {
			c.Locals("user", &config.User{Username: testUser, Roles: []string{"base"}})
		}
		return c.Next()
	})

	settingsGroup := app.Group("/api/settings")
	SetupSettingsRoutes(settingsGroup, state)

	return app, state
}

func TestTrafficLimitSettingsRoutes(t *testing.T) {
	app, state := setupTrafficSettingsTestApp(t)

	// 1. Non-admin GET /api/settings/traffic-limit -> 403 Forbidden
	reqUser := httptest.NewRequest(http.MethodGet, "/api/settings/traffic-limit", nil)
	reqUser.Header.Set("X-Test-User", "bob")
	respUser, err := app.Test(reqUser)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respUser.StatusCode)

	// 2. Admin GET /api/settings/traffic-limit -> 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/settings/traffic-limit", nil)
	reqAdmin.Header.Set("X-Test-User", "admin")
	respAdmin, err := app.Test(reqAdmin)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respAdmin.StatusCode)

	var currentCfg config.TrafficLimitConfig
	err = json.NewDecoder(respAdmin.Body).Decode(&currentCfg)
	require.NoError(t, err)
	assert.Equal(t, 1024.0, currentCfg.Unauthenticated.HourlyLimitMB)
	assert.Equal(t, 5120.0, currentCfg.Authenticated.HourlyLimitMB)
	assert.Equal(t, 1.25, currentCfg.MultiThreadMultiplier)

	// 3. Admin PUT /api/settings/traffic-limit -> 200 OK
	currentCfg.Unauthenticated.HourlyLimitMB = 2048.0
	currentCfg.MultiThreadMultiplier = 1.5
	body, _ := json.MarshalOf(currentCfg)

	putReq := httptest.NewRequest(http.MethodPut, "/api/settings/traffic-limit", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("X-Test-User", "admin")
	putResp, err := app.Test(putReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, putResp.StatusCode)

	// Verify state updated
	updatedCfg := state.Inner.Config.Load().TrafficLimit
	assert.Equal(t, 2048.0, updatedCfg.Unauthenticated.HourlyLimitMB)
	assert.Equal(t, 1.5, updatedCfg.MultiThreadMultiplier)
}
