/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package storage

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/service/index"
)

func newGitRouteTestApp(t *testing.T, format string, policy *config.GitRepoPolicy, upstreamURL string, persist bool) (*fiber.App, *core.AppState, string) {
	t.Helper()
	config.ClearRepoCacheConfigs()
	t.Cleanup(config.ClearRepoCacheConfigs)
	storageDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageDir

	repo := &config.Repository{
		Name:       "git-mirror",
		Format:     format,
		Visibility: "PUBLIC",
		Mirrors: []config.Mirror{
			{URL: upstreamURL, Persist: persist},
		},
		GitRepo: policy,
	}
	cfg.Maven.Repositories["git-mirror"] = repo

	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(storageDir, "git.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	state := core.NewAppState()
	state.Inner.DB = db
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	state.Inner.FileCache = core.NewFileByteCache(1 << 20)

	app := fiber.New(fiber.Config{UnescapePath: false})
	app.All("/:repo_name/*", func(c fiber.Ctx) error { return HandleRepository(c, state) })

	return app, state, storageDir
}

func TestGitRepoEngine_PathRouting(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock content"))
	}))
	defer upstream.Close()

	policy := config.DefaultGitRepoPolicy()
	policy.RegisteredRepos = []string{"404Setup/RenoP"}
	app, _, _ := newGitRouteTestApp(t, config.RepositoryFormatGithub, policy, upstream.URL, false)

	// Too short path
	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/404Setup", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	// Unsupported action type
	req = httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/404Setup/RenoP/issues/1", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	// Valid releases path
	req = httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/404Setup/RenoP/releases/v1.0.0.zip", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "mock content", string(body))
}

func TestGitRepoEngine_StrictVsLaxMode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()

	// Strict mode: unregistered repo must be rejected with 403
	strictPolicy := config.DefaultGitRepoPolicy()
	strictPolicy.Mode = config.GitRepoModeStrict
	strictPolicy.RegisteredRepos = []string{"allowed/repo"}
	strictApp, _, _ := newGitRouteTestApp(t, config.RepositoryFormatGithub, strictPolicy, upstream.URL, false)

	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/unregistered/repo/releases/v1.0.0.zip", nil)
	resp, err := strictApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	// Allowed repo in strict mode
	req = httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/allowed/repo/releases/v1.0.0.zip", nil)
	resp, err = strictApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Lax mode: any repo allowed, auto-registered
	laxPolicy := config.DefaultGitRepoPolicy()
	laxPolicy.Mode = config.GitRepoModeLax
	laxPolicy.RegisteredRepos = []string{}
	laxApp, _, _ := newGitRouteTestApp(t, config.RepositoryFormatGithub, laxPolicy, upstream.URL, false)

	req = httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/any/repo/releases/v1.0.0.zip", nil)
	resp, err = laxApp.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.True(t, laxPolicy.IsRepoRegistered("any/repo"))
}

func TestGitRepoEngine_ResourceLocking(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()

	policy := config.DefaultGitRepoPolicy()
	policy.RegisteredRepos = []string{"owner/locked-repo"}
	app, state, _ := newGitRouteTestApp(t, config.RepositoryFormatGithub, policy, upstream.URL, false)

	// Set repository lock
	err := state.GetDB().SetResourceLock(&core.ResourceLock{
		Format: "github", Repository: "git-mirror", Name: "owner/locked-repo",
		Source:   core.ResourceLockSystem,
		Mode:     core.ResourceLockRead,
		Reason:   "prohibited",
		LockedAt: time.Now().UnixMilli(),
	}, "", "")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/owner/locked-repo/releases/v1.0.0.zip", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusLocked, resp.StatusCode)
}

func TestGitRepoEngine_DailyTrafficLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("12345"))
	}))
	defer upstream.Close()

	policy := config.DefaultGitRepoPolicy()
	policy.MaxDailyTrafficBytes = 10
	policy.RegisteredRepos = []string{"owner/repo"}
	app, _, _ := newGitRouteTestApp(t, config.RepositoryFormatGithub, policy, upstream.URL, false)

	// Exceed daily traffic manually
	recordGitDailyTraffic("git-mirror", "owner/repo", 10)

	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/owner/repo/releases/v1.0.0.zip", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
}

func TestGitRepoEngine_MaxVersionSize(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "200")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, 200))
	}))
	defer upstream.Close()

	policy := config.DefaultGitRepoPolicy()
	policy.MaxVersionSizeBytes = 100 // 100 bytes limit
	policy.RegisteredRepos = []string{"owner/repo"}
	app, _, _ := newGitRouteTestApp(t, config.RepositoryFormatGithub, policy, upstream.URL, false)

	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/owner/repo/releases/v1.0.0.zip", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestGitRepoEngine_CachingAndRetentionPruning(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("artifact data"))
	}))
	defer upstream.Close()

	policy := config.DefaultGitRepoPolicy()
	policy.RegisteredRepos = []string{"owner/repo"}
	app, _, storageDir := newGitRouteTestApp(t, config.RepositoryFormatGithub, policy, upstream.URL, true)

	// First request: downloads and caches
	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/owner/repo/actions/101/artifact.zip", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, 1, upstreamCalls)

	// Verify file exists on disk
	expectedPath := filepath.Join(storageDir, "git-mirror", "owner", "repo", "actions", "101", "artifact.zip")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err)

	// Second request for the same file: served from cache
	req = httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/owner/repo/actions/101/artifact.zip", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, 1, upstreamCalls) // Upstream was not called again!

	// New action run: should prune older run 101 because RetainLatestWorkflowRuns is true
	req = httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/owner/repo/actions/102/artifact.zip", nil)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, 2, upstreamCalls)

	// Verify run 101 was pruned from disk
	_, err = os.Stat(expectedPath)
	require.True(t, os.IsNotExist(err), "older workflow run 101 must be pruned")

	// Verify run 102 exists
	newExpectedPath := filepath.Join(storageDir, "git-mirror", "owner", "repo", "actions", "102", "artifact.zip")
	_, err = os.Stat(newExpectedPath)
	require.NoError(t, err)
}

func TestGitRepoEngine_GitLabArtifacts(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "gitlab response for %s", r.URL.Path)
	}))
	defer upstream.Close()

	policy := config.DefaultGitRepoPolicy()
	policy.RegisteredRepos = []string{"gitlab-org/gitlab"}
	app, _, _ := newGitRouteTestApp(t, config.RepositoryFormatGitLab, policy, upstream.URL, false)

	req := httptest.NewRequest(http.MethodGet, "http://repo.example/git-mirror/gitlab-org/gitlab/actions/555/build.zip", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "gitlab response")
}

func TestCheckGitRepoUpstream(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/old-org/old-repo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"full_name": "new-org/new-repo"}`))
	})
	mux.HandleFunc("/repos/missing/repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v4/projects/gitlab-old%2Frepo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path_with_namespace": "gitlab-new/repo"}`))
	})
	mux.HandleFunc("/api/v4/projects/missing%2Frepo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := config.DefaultConfig()
	ghRepo := &config.Repository{
		Format:  config.RepositoryFormatGithub,
		Mirrors: []config.Mirror{{URL: server.URL}},
	}

	canonical, exists, err := CheckGitRepoUpstream(t.Context(), cfg, ghRepo, "old-org", "old-repo")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "new-org/new-repo", canonical)

	canonical, exists, err = CheckGitRepoUpstream(t.Context(), cfg, ghRepo, "missing", "repo")
	require.NoError(t, err)
	require.False(t, exists)
	require.Empty(t, canonical)

	glRepo := &config.Repository{
		Format:  config.RepositoryFormatGitLab,
		Mirrors: []config.Mirror{{URL: server.URL}},
	}

	canonical, exists, err = CheckGitRepoUpstream(t.Context(), cfg, glRepo, "gitlab-old", "repo")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "gitlab-new/repo", canonical)

	canonical, exists, err = CheckGitRepoUpstream(t.Context(), cfg, glRepo, "missing", "repo")
	require.NoError(t, err)
	require.False(t, exists)
	require.Empty(t, canonical)
}
