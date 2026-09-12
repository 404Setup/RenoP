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
	"bytes"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"renop/internal/api"
	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/service/auth"
	"renop/internal/service/maven"
	"renop/internal/service/message"
	"renop/internal/service/npm"
	"renop/internal/service/publicationquota"
	"renop/internal/service/settings"
	"renop/internal/service/status"
	"renop/internal/service/storage"
	"renop/internal/service/superteam"
	"renop/internal/service/ticket"
	"renop/internal/service/token"
	"renop/internal/testutil"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"
)

func demoTestPaths(t *testing.T) string {
	t.Helper()
	dir := testutil.TempDir(t)
	t.Setenv("RENOP_DEMO_DATABASE", filepath.Join(dir, "renop-demo.db"))
	t.Setenv("RENOP_DEMO_SETTINGS_DB", filepath.Join(dir, "renop-demo-settings.db"))
	return dir
}

func demoTestApp(t *testing.T, state *core.AppState) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(Middleware(state))
	app.Get("/api/demo", func(c fiber.Ctx) error { return Info(c, state) })
	app.Use(auth.AuthMiddleware(state))
	apiRoutes := app.Group("/api")
	auth.SetupAuthRoutes(apiRoutes, state, nil)
	settings.SetupSettingsRoutes(apiRoutes.Group("/settings"), state)
	token.SetupTokenRoutes(apiRoutes, state, nil)
	audit.SetupAuditRoutes(apiRoutes.Group("/auth"), state)
	api.SetupAPIRoutes(apiRoutes, state)
	status.SetupRoutes(apiRoutes, state)
	maven.SetupRoutes(apiRoutes, state)
	npm.SetupRoutes(apiRoutes, state, storage.NewPackageStore())
	superteam.SetupRoutes(apiRoutes, state)
	ticket.SetupRoutes(apiRoutes, state)
	message.SetupRoutes(apiRoutes, state)
	publicationquota.SetupRoutes(apiRoutes, state)
	previousFallback := storage.HTMLFallback
	storage.HTMLFallback = func(c fiber.Ctx, _ *core.AppState) error { return c.SendString("demo application shell") }
	t.Cleanup(func() { storage.HTMLFallback = previousFallback })
	storage.SetupRoutes(app, state)
	return app
}

func demoRequest(t *testing.T, app *fiber.App, method, path string, message proto.Message, cookie *http.Cookie) *http.Response {
	t.Helper()
	var body []byte
	if message != nil {
		var err error
		body, err = proto.Marshal(message)
		require.NoError(t, err)
	}
	request := httptest.NewRequest(method, "http://localhost"+path, bytes.NewReader(body))
	request.Header.Set("Content-Type", protohttp.ContentType)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response, err := app.Test(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func demoLogin(t *testing.T, app *fiber.App, state *core.AppState) *http.Cookie {
	t.Helper()
	body, err := proto.Marshal(&pb.LoginRequest{Name: "admin", Secret: "12345678"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", protohttp.ContentType)
	request.Header.Set("X-Renop-Legal-Revision", state.Inner.Config.Load().Legal.Revision())
	response, err := app.Test(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	for _, cookie := range response.Cookies() {
		if cookie.Name == "renop_session" {
			return cookie
		}
	}
	t.Fatal("login did not issue a browser session")
	return nil
}

func TestDemoPresetsAreCompleteAndReadOnly(t *testing.T) {
	dir := demoTestPaths(t)
	state, err := Open(Options{Enabled: true})
	require.NoError(t, err)
	defer state.GetDB().(io.Closer).Close()
	db := state.GetDB().(*sessionStore).database
	for _, table := range []string{"tokens", "super_teams", "maven_artifacts", "cargo_packages", "npm_packages", "docker_images",
		"review_tasks", "ticket_messages", "user_messages", "audit_logs", "download_statistics", "mail_jobs", "demo_files"} {
		var count int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count), table)
		require.Positive(t, count, table)
	}
	before, err := os.ReadFile(databasePath())
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE tokens SET description = 'must not persist' WHERE name = 'admin'`)
	require.Error(t, err)
	app := demoTestApp(t, state)
	cookie := demoLogin(t, app, state)
	root := filepath.Join(state.Inner.Config.Load().StoragePath, "downloads")
	require.True(t, state.Inner.FileIndex.HasDir(root), "preset repository is indexed: %s", root)
	require.NotNil(t, api.CreateFileDetails(state, root, true))
	require.Equal(t, 200, demoRequest(t, app, "GET", "/api/auth/me", nil, cookie).StatusCode)
	for _, path := range []string{
		"/api/settings/domains", "/api/settings/domain/frontend", "/api/settings/domain/server", "/api/settings/repositories",
		"/api/status/instance", "/api/status/snapshots", "/api/auth/profile/security", "/api/auth/profile/fido",
		"/api/tokens", "/api/super-teams", "/api/super-teams/platform", "/api/super-teams/platform/resources?format=maven",
		"/api/tickets", "/api/messages", "/api/auth/logs", "/api/settings/mail/jobs",
		"/api/repositories/details", "/api/repositories/details/downloads", "/api/maven/repositories/releases/packages",
		"/api/maven/repositories/releases/package?group=com.example.demo&artifact=demo-client",
		"/api/docker/repositories/docker/images", "/api/docker/repositories/docker/images/platform/api",
		"/cargo/config.json", "/cargo/api/v1/crates", "/cargo/api/v1/me/crates", "/cargo/api/v1/crates/renop_demo",
		"/cargo/api/v1/crates/renop_demo/owners", "/cargo/api/v1/crates/renop_demo/1.0.0/docs",
	} {
		response := demoRequest(t, app, "GET", path, nil, cookie)
		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		require.Equal(t, 200, response.StatusCode, "%s: %s", path, body)
	}
	for _, action := range []struct{ method, path string }{
		{"PUT", "/api/settings/domain/server"}, {"POST", "/api/auth/profile/recovery-codes"},
		{"DELETE", "/api/auth/logs"}, {"PUT", "/downloads/file.bin"}, {"GET", "/api/auth/oauth/github/start"},
	} {
		response := demoRequest(t, app, action.method, action.path, nil, cookie)
		require.Equal(t, 403, response.StatusCode, action.path)
		require.Equal(t, "demo_read_only", response.Header.Get("X-Renop-Error-Code"))
	}
	for _, path := range []string{"/cargo/packages/renop_demo", "/npm/packages/@platform/ui", "/releases/packages/com.example.demo/demo-client", "/docker/platform/api"} {
		request := httptest.NewRequest("GET", "http://localhost"+path, nil)
		request.Header.Set("Accept", "text/html")
		request.AddCookie(cookie)
		response, err := app.Test(request)
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode, path)
		require.NoError(t, response.Body.Close())
	}
	for _, path := range []string{"/cargo/api/v1/crates/renop_demo/1.0.0/download", "/downloads/releases/demo-tool-1.2.0.zip", "/cargo/re/no/renop_demo"} {
		response := demoRequest(t, app, "GET", path, nil, cookie)
		require.Equal(t, 403, response.StatusCode, path)
		require.Equal(t, "demo_file_unavailable", response.Header.Get("X-Renop-Error-Code"))
	}
	audit.Log(state, &core.AuditLogEntry{Username: "admin", Action: "LOGIN", Details: "must not be saved"})
	require.Empty(t, state.Inner.AuditLogChan)
	_, err = state.GetDB().GetSession("preset-session-admin")
	require.NoError(t, err)
	preset, err := state.GetDB().GetSession("preset-session-admin")
	require.NoError(t, err)
	require.Nil(t, preset, "display sessions cannot authenticate")
	require.Equal(t, 204, demoRequest(t, app, "POST", "/api/auth/logout", nil, cookie).StatusCode)
	after, err := os.ReadFile(databasePath())
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(before), sha256.Sum256(after), "requests must not modify the preset database")
	_, err = os.Stat(state.Inner.Config.Load().StoragePath)
	require.True(t, os.IsNotExist(err))
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, 2, "demo creates only its two databases")
}

func TestDemoTempPersistsOnlySettingsAndRepositoryDefinitions(t *testing.T) {
	demoTestPaths(t)
	state, err := Open(Options{Enabled: true, Temporary: true})
	require.NoError(t, err)
	_, err = state.GetDB().(*sessionStore).database.Exec(`UPDATE audit_logs SET details = 'must not persist' WHERE username = 'admin'`)
	require.Error(t, err, "configuration mode must retain the database write boundary for logs")
	app := demoTestApp(t, state)
	cookie := demoLogin(t, app, state)
	cfg := state.Inner.Config.Load()
	server := pb.FromServerConfig(cfg.Server, cfg.Database, cfg.AuditLog)
	server.Port = 43123
	require.Equal(t, 200, demoRequest(t, app, "PUT", "/api/settings/domain/server", server, cookie).StatusCode)
	repo := &pb.Repository{Name: "showcase", Format: "files", Visibility: "PUBLIC"}
	require.Equal(t, 200, demoRequest(t, app, "PUT", "/api/settings/repositories/showcase", repo, cookie).StatusCode)
	storage := pb.FromStorageConfig(state.Inner.Config.Load())
	storage.StoragePath = filepath.Join(filepath.Dir(databasePath()), "presentation-only-storage")
	require.Equal(t, 200, demoRequest(t, app, "PUT", "/api/settings/domain/storage", storage, cookie).StatusCode)
	require.Equal(t, 403, demoRequest(t, app, "DELETE", "/api/auth/logs", nil, cookie).StatusCode)
	require.Equal(t, 403, demoRequest(t, app, "PUT", "/api/auth/profile/password", &pb.UpdatePasswordRequest{NewPassword: "not-allowed"}, cookie).StatusCode)
	require.NoError(t, state.GetDB().(io.Closer).Close())
	restarted, err := Open(Options{Enabled: true})
	require.NoError(t, err)
	defer restarted.GetDB().(io.Closer).Close()
	require.EqualValues(t, 43123, restarted.Inner.Config.Load().Server.Port)
	require.NotNil(t, restarted.Inner.Config.Load().Maven.Repositories["showcase"])
	require.Equal(t, storage.StoragePath, pb.FromStorageConfig(restarted.Inner.Config.Load()).StoragePath)
	_, err = os.Stat(storage.StoragePath)
	require.True(t, os.IsNotExist(err))
}

func TestDemoFlagDependencyAndNormalDataIsolation(t *testing.T) {
	_, err := ParseOptions([]string{"--demo-temp"})
	require.ErrorContains(t, err, "requires --demo")
	options, err := ParseOptions([]string{"--demo-temp", "--demo"})
	require.NoError(t, err)
	require.True(t, options.Temporary)
	demoTestPaths(t)
	require.NoError(t, os.WriteFile(databasePath(), []byte("existing unrelated data"), 0600))
	cfg := config.DefaultConfig()
	err = prepareDatabase(databasePath(), cfg)
	require.Error(t, err)
	data, err := os.ReadFile(databasePath())
	require.NoError(t, err)
	require.Equal(t, "existing unrelated data", string(data))
}
