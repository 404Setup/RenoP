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
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/service/auth"
	"renop/internal/service/index"
	"renop/internal/testutil"
	"renop/pkg/pb"
)

func setupGitRepoTestApp(t *testing.T) (*database.DB, *core.AppState, *fiber.App) {
	t.Helper()
	db, err := database.InitDB(config.DatabaseConfig{
		Driver:       "sqlite",
		Dsn:          filepath.Join(testutil.TempDir(t), "git-api.db"),
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	state := core.NewAppState()
	state.Inner.DB = db
	state.Inner.FileIndex = index.NewFileIndex()

	cfg := config.DefaultConfig()
	cfg.Maven.Repositories["github-mirror"] = &config.Repository{
		Name:       "github-mirror",
		Format:     config.RepositoryFormatGithub,
		Visibility: "PUBLIC",
		GitRepo:    config.DefaultGitRepoPolicy(),
	}
	cfg.Maven.Repositories["gitlab-mirror"] = &config.Repository{
		Name:       "gitlab-mirror",
		Format:     config.RepositoryFormatGitLab,
		Visibility: "PUBLIC",
		GitRepo:    config.DefaultGitRepoPolicy(),
	}
	state.Inner.Config.Store(cfg)

	// Users: alice (regular), mod (moderator), admin (manager)
	for _, username := range []string{"alice", "mod", "admin"} {
		perms := []string{"base"}
		if username == "mod" {
			perms = append(perms, "canmoderate:github-mirror", "canmoderate:gitlab-mirror")
		} else if username == "admin" {
			perms = append(perms, "manager")
		}
		require.NoError(t, db.SaveToken(&core.AccessToken{
			Name:        username,
			Permissions: perms,
		}))
		session := &core.Session{
			Username:  username,
			PublicID:  username,
			CreatedAt: time.Now().UnixMilli(),
		}
		session.LastActive.Store(session.CreatedAt)
		require.NoError(t, db.SaveSession(session, "session-"+username))
		require.NoError(t, state.SaveSession(session, "session-"+username))
	}
	state.Inner.TokensCount.Store(3)

	app := fiber.New()
	app.Use(auth.AuthMiddleware(state))
	setupGitRepoRoutes(app.Group("/api"), state)

	return db, state, app
}

func doGitRepoRequest(t *testing.T, app *fiber.App, method, urlPath, user string, payload any) (int, []byte) {
	t.Helper()
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.MarshalOf(payload)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, urlPath, bodyReader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.AddCookie(&http.Cookie{Name: "renop_session", Value: "session-" + user})
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

func TestGitRepoPolicyAPI(t *testing.T) {
	_, _, app := setupGitRepoTestApp(t)

	// GET policy
	code, body := doGitRepoRequest(t, app, "GET", "/api/git-repo/repositories/github-mirror/policy", "", nil)
	require.Equal(t, 200, code)
	var policy pb.GitRepoPolicyResponse
	require.NoError(t, proto.Unmarshal(body, &policy))
	assert.Equal(t, "lax", policy.Mode)
	assert.Equal(t, int64(300*1024*1024), policy.MaxVersionSizeBytes)
	assert.Equal(t, int64(25*1024*1024*1024), policy.MaxDailyTrafficBytes)
	assert.True(t, policy.RetainLatestWorkflowRuns)
	assert.True(t, policy.RetainLatestReleases)
	assert.Equal(t, int32(72), policy.QuotaRequestCooldownHours)

	// PUT policy: forbidden for regular user
	newSizeBytes := int64(600 * 1024 * 1024)
	newDailyBytes := int64(50 * 1024 * 1024 * 1024)
	retainWorkflows := false
	retainReleases := false
	cooldown := 48
	updateReq := updateGitRepoPolicyRequest{
		Mode:                      "strict",
		MaxVersionSizeBytes:       &newSizeBytes,
		MaxDailyTrafficBytes:      &newDailyBytes,
		RetainLatestWorkflowRuns:  &retainWorkflows,
		RetainLatestReleases:      &retainReleases,
		QuotaRequestCooldownHours: &cooldown,
	}
	code, _ = doGitRepoRequest(t, app, "PUT", "/api/git-repo/repositories/github-mirror/policy", "alice", updateReq)
	require.Equal(t, 403, code)

	// PUT policy: success for moderator
	code, body = doGitRepoRequest(t, app, "PUT", "/api/git-repo/repositories/github-mirror/policy", "mod", updateReq)
	require.Equal(t, 200, code)
	var updated pb.GitRepoPolicyResponse
	require.NoError(t, proto.Unmarshal(body, &updated))
	assert.Equal(t, "strict", updated.Mode)
	assert.Equal(t, newSizeBytes, updated.MaxVersionSizeBytes)
	assert.Equal(t, newDailyBytes, updated.MaxDailyTrafficBytes)
	assert.False(t, updated.RetainLatestWorkflowRuns)
	assert.False(t, updated.RetainLatestReleases)
	assert.Equal(t, int32(48), updated.QuotaRequestCooldownHours)

	// GET returns updated values
	code, body = doGitRepoRequest(t, app, "GET", "/api/git-repo/repositories/github-mirror/policy", "", nil)
	require.Equal(t, 200, code)
	require.NoError(t, proto.Unmarshal(body, &policy))
	assert.Equal(t, "strict", policy.Mode)
	assert.Equal(t, newSizeBytes, policy.MaxVersionSizeBytes)
}

func TestGitRepoRegisterAPI_GitHub(t *testing.T) {
	db, state, app := setupGitRepoTestApp(t)

	// Unauthenticated
	code, _ := doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "", map[string]any{
		"target_repo": "owner/repo",
	})
	require.Equal(t, 401, code)

	// Invalid target_repo format
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "invalid-no-slash",
	})
	require.Equal(t, 400, code)

	// GitHub OAuth disabled on server initially
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gh/project",
	})
	require.Equal(t, 400, code)

	// Enable GitHub OAuth
	cfg := state.Inner.Config.Load()
	cfg.Server.GitHubOAuth = config.GitHubOAuthConfig{
		Enabled:      true,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		CallbackURL:  "https://example.com/callback",
	}
	state.Inner.Config.Store(cfg)

	// Alice does not have GitHub identity stored yet -> 403 Forbidden
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gh/project",
	})
	require.Equal(t, 403, code)

	// Store GitHub identity for Alice with personal login "alice-gh" and org "sample-org"
	profile, err := db.GetUserProfile("alice")
	require.NoError(t, err)
	now := time.Now().UnixMilli()
	require.NoError(t, db.StoreGitHubIdentity(profile.UserID, 888, "alice-gh", []core.GitHubPrincipal{
		{Type: core.GitHubPrincipalUser, GitHubID: 888, Login: "alice-gh"},
		{Type: core.GitHubPrincipalOrganization, GitHubID: 999, Login: "sample-org"},
	}, now))

	// Register Alice's personal repo -> 201 Created
	code, body := doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gh/project",
	})
	require.Equal(t, 201, code, string(body))
	var task core.ReviewTask
	require.NoError(t, json.UnmarshalOf(body, &task))
	assert.Equal(t, core.ReviewKindGitRepoRegistration, task.Kind)
	assert.Equal(t, "alice-gh/project", task.ResourceKey)

	// Register Alice's org repo -> 201 Created
	code, body = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "sample-org/backend",
	})
	require.Equal(t, 201, code, string(body))

	// Attempting to register un-owned org repo -> 403 Forbidden
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "other-org/other-repo",
	})
	require.Equal(t, 403, code)

	// Duplicate registration task -> 409 Conflict
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gh/project",
	})
	require.Equal(t, 409, code)

	// If repository is already registered in config -> 409 Conflict
	cfg = state.Inner.Config.Load()
	cfg.Maven.Repositories["github-mirror"].GitRepo.RegisteredRepos = []string{"alice-gh/already-there"}
	state.Inner.Config.Store(cfg)
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gh/already-there",
	})
	require.Equal(t, 409, code)

	// Upstream repo does not exist -> 404 Not Found
	upstreamMux := http.NewServeMux()
	upstreamMux.HandleFunc("/repos/alice-gh/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	upstreamServer := httptest.NewServer(upstreamMux)
	defer upstreamServer.Close()

	cfg = state.Inner.Config.Load()
	cfg.Maven.Repositories["github-mirror"].Mirrors = []config.Mirror{{URL: upstreamServer.URL}}
	state.Inner.Config.Store(cfg)

	code, body = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gh/nonexistent",
	})
	require.Equal(t, 404, code, string(body))
}

func TestGitRepoRegisterAPI_GitLab(t *testing.T) {
	db, state, app := setupGitRepoTestApp(t)

	// Enable GitLab OAuth
	cfg := state.Inner.Config.Load()
	cfg.Server.OAuthProviders = []config.OAuthProviderConfig{
		{
			ID:           "gitlab",
			Type:         "gitlab",
			Name:         "GitLab",
			Enabled:      true,
			ClientID:     "gitlab-id",
			ClientSecret: "gitlab-secret",
			CallbackURL:  "https://example.com/api/auth/oauth/gitlab/callback",
		},
	}
	state.Inner.Config.Store(cfg)

	// Alice does not have GitLab identity -> 403 Forbidden
	code, _ := doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/gitlab-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gl/project",
	})
	require.Equal(t, 403, code)

	// Link GitLab identity with namespaces: "alice-gl" and "gitlab-group"
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	now := time.Now().UnixMilli()
	gitlabIdentity := core.OAuthIdentity{
		ProviderID: "gitlab",
		Subject:    "gitlab-sub-123",
		Authority:  strings.Repeat("a", 64),
		Login:      "alice-gl",
	}
	require.NoError(t, db.LinkOAuthIdentity("alice", "session-alice", mfa.Snapshot, gitlabIdentity, now))
	gitlabIdentity.Namespaces = []string{"alice-gl", "gitlab-group"}
	require.NoError(t, db.RefreshOAuthIdentity(mfa.UserID, gitlabIdentity, now+1))

	// Register Alice's GitLab repo -> 201 Created
	code, body := doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/gitlab-mirror/register", "alice", map[string]any{
		"target_repo": "alice-gl/project",
	})
	require.Equal(t, 201, code, string(body))

	// Register Alice's GitLab group repo -> 201 Created
	code, body = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/gitlab-mirror/register", "alice", map[string]any{
		"target_repo": "gitlab-group/service",
	})
	require.Equal(t, 201, code, string(body))

	// Unowned group -> 403 Forbidden
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/gitlab-mirror/register", "alice", map[string]any{
		"target_repo": "other-group/service",
	})
	require.Equal(t, 403, code)
}

func TestGitRepoUnregisterAPI(t *testing.T) {
	_, state, app := setupGitRepoTestApp(t)

	cfg := state.Inner.Config.Load()
	cfg.Maven.Repositories["github-mirror"].GitRepo.RegisteredRepos = []string{"owner/repo1", "owner/repo2"}
	state.Inner.Config.Store(cfg)

	// Regular user cannot unregister
	code, _ := doGitRepoRequest(t, app, "DELETE", "/api/git-repo/repositories/github-mirror/repos/owner/repo1", "alice", nil)
	require.Equal(t, 403, code)

	// Moderator can unregister
	code, _ = doGitRepoRequest(t, app, "DELETE", "/api/git-repo/repositories/github-mirror/repos/owner/repo1", "mod", nil)
	require.Equal(t, 200, code)

	// Verify repo was removed from config
	cfg = state.Inner.Config.Load()
	assert.Equal(t, []string{"owner/repo2"}, cfg.Maven.Repositories["github-mirror"].GitRepo.RegisteredRepos)

	// Deleting non-registered repo -> 404 Not Found
	code, _ = doGitRepoRequest(t, app, "DELETE", "/api/git-repo/repositories/github-mirror/repos/owner/repo1", "mod", nil)
	require.Equal(t, 404, code)
}

func TestGitRepoQuotaRequestAPI(t *testing.T) {
	db, state, app := setupGitRepoTestApp(t)

	// Test case 1: Super Team managed
	now := time.Now().UnixMilli()
	team := &core.SuperTeam{Prefix: "devteam", Name: "Dev Team", CreatedAt: now}
	require.NoError(t, db.CreateSuperTeam(team, "admin", 5, 10))
	// Add alice as L2 member (read/write, but not manage)
	require.NoError(t, db.ForceAddSuperTeamMembers("devteam", "admin", []string{"alice"}, core.SuperTeamRoleWrite, 5, 10, now+1))

	cfg := state.Inner.Config.Load()
	cfg.Maven.Repositories["github-mirror"].GitRepo.SuperTeamPrefix = "devteam"
	state.Inner.Config.Store(cfg)

	quotaPayload := gitRepoQuotaRequest{
		TargetRepo:               "owner/repo",
		MaxVersionSizeBytes:      500 * 1024 * 1024,
		MaxDailyTrafficBytes:     50 * 1024 * 1024 * 1024,
		RetainLatestWorkflowRuns: true,
		RetainLatestReleases:     true,
		Reason:                   "Need more bandwidth for releases",
	}

	// Alice has L2 role -> 403 Forbidden
	code, _ := doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/quota-request", "alice", quotaPayload)
	require.Equal(t, 403, code)

	// Upgrade Alice to L3 manager in Super Team
	require.NoError(t, db.SetSuperTeamMemberLevel("devteam", "admin", "alice", core.SuperTeamRoleManage, true))

	// Alice with L3 role can submit request -> 201 Created
	code, body := doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/quota-request", "alice", quotaPayload)
	require.Equal(t, 201, code, string(body))
	var task core.ReviewTask
	require.NoError(t, json.UnmarshalOf(body, &task))
	assert.Equal(t, core.ReviewKindGitRepoQuota, task.Kind)

	// Duplicate pending request -> 409 Conflict
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/github-mirror/quota-request", "alice", quotaPayload)
	require.Equal(t, 409, code)

	// Test case 2: Independently managed (no super team prefix)
	cfg = state.Inner.Config.Load()
	cfg.Maven.Repositories["gitlab-mirror"].GitRepo.SuperTeamPrefix = ""
	cfg.Maven.Repositories["gitlab-mirror"].GitRepo.Owner = "bob"
	state.Inner.Config.Store(cfg)

	// Alice is not owner -> 403 Forbidden
	code, _ = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/gitlab-mirror/quota-request", "alice", quotaPayload)
	require.Equal(t, 403, code)

	// Set Alice as owner -> 201 Created
	cfg = state.Inner.Config.Load()
	cfg.Maven.Repositories["gitlab-mirror"].GitRepo.Owner = "alice"
	state.Inner.Config.Store(cfg)

	code, body = doGitRepoRequest(t, app, "POST", "/api/git-repo/repositories/gitlab-mirror/quota-request", "alice", quotaPayload)
	require.Equal(t, 201, code, string(body))
}

func TestGitRepoLocksAPI(t *testing.T) {
	_, _, app := setupGitRepoTestApp(t)

	lockPayload := gitRepoLockRequest{
		TargetRepo: "owner/repo",
		Version:    "v1.0.0",
		Mode:       core.ResourceLockWrite,
		Reason:     "abuse",
	}

	// Regular user cannot lock -> 403 Forbidden
	code, _ := doGitRepoRequest(t, app, "PUT", "/api/git-repo/repositories/github-mirror/locks", "alice", lockPayload)
	require.Equal(t, 403, code)

	// Moderator can lock -> 200 OK
	code, _ = doGitRepoRequest(t, app, "PUT", "/api/git-repo/repositories/github-mirror/locks", "mod", lockPayload)
	require.Equal(t, 200, code)

	// Non-moderator cannot unlock -> 403 Forbidden
	code, _ = doGitRepoRequest(t, app, "DELETE", "/api/git-repo/repositories/github-mirror/locks?target_repo=owner/repo&version=v1.0.0", "alice", nil)
	require.Equal(t, 403, code)

	// Moderator can unlock -> 200 OK
	code, _ = doGitRepoRequest(t, app, "DELETE", "/api/git-repo/repositories/github-mirror/locks?target_repo=owner/repo&version=v1.0.0", "mod", nil)
	require.Equal(t, 200, code)
}
