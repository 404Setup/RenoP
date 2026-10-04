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
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/service/auth"
	"renop/internal/service/storage"
	"renop/internal/utils"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"
)

type updateGitRepoPolicyRequest struct {
	Mode                      string `json:"mode"`
	MaxVersionSizeBytes       *int64 `json:"max_version_size_bytes"`
	MaxDailyTrafficBytes      *int64 `json:"max_daily_traffic_bytes"`
	RetainLatestWorkflowRuns  *bool  `json:"retain_latest_workflow_runs"`
	RetainLatestReleases      *bool  `json:"retain_latest_releases"`
	QuotaRequestCooldownHours *int   `json:"quota_request_cooldown_hours"`
	SuperTeamPrefix           string `json:"super_team_prefix"`
	Owner                     string `json:"owner"`
}

type gitRepoRegistrationRequest struct {
	TargetRepo string `json:"target_repo"`
}

type gitRepoQuotaRequest struct {
	TargetRepo               string `json:"target_repo"`
	MaxVersionSizeBytes      int64  `json:"max_version_size_bytes"`
	MaxDailyTrafficBytes     int64  `json:"max_daily_traffic_bytes"`
	RetainLatestWorkflowRuns bool   `json:"retain_latest_workflow_runs"`
	RetainLatestReleases     bool   `json:"retain_latest_releases"`
	Reason                   string `json:"reason"`
}

type gitRepoLockRequest struct {
	TargetRepo string `json:"target_repo"`
	Version    string `json:"version"`
	Mode       string `json:"mode"`
	Reason     string `json:"reason"`
	ReasonText string `json:"reason_text"`
}

func setupGitRepoRoutes(router fiber.Router, state *core.AppState) {
	router.Get("/git-repo/repositories/:repo_name/policy", func(c fiber.Ctx) error {
		return getGitRepoPolicyAPI(c, state)
	})
	router.Put("/git-repo/repositories/:repo_name/policy", func(c fiber.Ctx) error {
		return putGitRepoPolicyAPI(c, state)
	})
	router.Post("/git-repo/repositories/:repo_name/register", func(c fiber.Ctx) error {
		return registerGitRepoAPI(c, state)
	})
	router.Delete("/git-repo/repositories/:repo_name/repos/*", func(c fiber.Ctx) error {
		return unregisterGitRepoAPI(c, state)
	})
	router.Post("/git-repo/repositories/:repo_name/quota-request", func(c fiber.Ctx) error {
		return quotaRequestGitRepoAPI(c, state)
	})
	router.Put("/git-repo/repositories/:repo_name/locks", func(c fiber.Ctx) error {
		return setGitRepoLockAPI(c, state)
	})
	router.Delete("/git-repo/repositories/:repo_name/locks", func(c fiber.Ctx) error {
		return deleteGitRepoLockAPI(c, state)
	})
}

func getGitRepoPolicyAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	cfg := state.Inner.Config.Load()
	if cfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("Configuration unavailable")
	}
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || (repo.NormalizedFormat() != config.RepositoryFormatGithub && repo.NormalizedFormat() != config.RepositoryFormatGitLab) {
		return c.Status(fiber.StatusNotFound).SendString("Git repository was not found")
	}

	user := auth.GetUser(c)
	if !strings.EqualFold(repo.Visibility, "PUBLIC") && (user == nil || !user.CheckReadPermission(repoName, "", repo.Visibility, true)) {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}

	policy := repo.EffectiveGitPolicy()
	targetRepo := strings.TrimSpace(c.Query("target_repo"))
	if c.Query("sync") == "true" {
		if targetRepo != "" {
			parts := strings.Split(targetRepo, "/")
			if len(parts) == 2 {
				if canonical, exists, _ := storage.CheckGitRepoUpstream(c.Context(), cfg, repo, parts[0], parts[1]); exists && canonical != "" && !strings.EqualFold(canonical, targetRepo) {
					storage.UpdateGitRepoRedirect(state, repo, targetRepo, canonical)
					targetRepo = canonical
					policy = repo.EffectiveGitPolicy()
				}
			}
		} else {
			for _, reg := range policy.RegisteredRepos {
				parts := strings.Split(reg, "/")
				if len(parts) == 2 {
					if canonical, exists, _ := storage.CheckGitRepoUpstream(c.Context(), cfg, repo, parts[0], parts[1]); exists && canonical != "" && !strings.EqualFold(canonical, reg) {
						storage.UpdateGitRepoRedirect(state, repo, reg, canonical)
					}
				}
			}
			policy = repo.EffectiveGitPolicy()
		}
	}
	isLocked := false
	var maxVer int64
	var maxDaily int64
	var retainRuns bool
	var retainReleases bool
	var dailyTraffic int64

	if targetRepo != "" {
		maxVer = policy.EffectiveMaxVersionSizeBytes(targetRepo)
		maxDaily = policy.EffectiveMaxDailyTrafficBytes(targetRepo)
		retainRuns = policy.EffectiveRetainLatestWorkflowRuns(targetRepo)
		retainReleases = policy.EffectiveRetainLatestReleases(targetRepo)
		dailyTraffic = storage.GetGitDailyTraffic(repoName, targetRepo)
		if state.GetDB() != nil {
			target := core.ResourceLockTarget{Format: repo.NormalizedFormat(), Repository: repo.Name, Name: targetRepo, Version: ""}
			if err := state.GetDB().EnsureResourceMutable(target, false); err != nil {
				isLocked = true
			}
		}
	} else {
		maxVer = policy.MaxVersionSizeBytes
		maxDaily = policy.MaxDailyTrafficBytes
		retainRuns = policy.ShouldRetainLatestWorkflowRuns()
		retainReleases = policy.ShouldRetainLatestReleases()
		dailyTraffic = storage.GetGitDailyTraffic(repoName, "")
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	resp := &pb.GitRepoPolicyResponse{
		Mode:                      policy.EffectiveMode(),
		MaxVersionSizeBytes:       maxVer,
		MaxDailyTrafficBytes:      maxDaily,
		RetainLatestWorkflowRuns:  retainRuns,
		RetainLatestReleases:      retainReleases,
		QuotaRequestCooldownHours: int32(policy.QuotaRequestCooldownHours),
		SuperTeamPrefix:           policy.SuperTeamPrefix,
		Owner:                     policy.Owner,
		RegisteredRepos:           policy.RegisteredRepos,
		DailyTrafficTodayBytes:    dailyTraffic,
		TargetRepo:                targetRepo,
		IsLocked:                  isLocked,
	}
	return protohttp.Write(c, resp)
}

func putGitRepoPolicyAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	user := auth.GetUser(c)
	if user == nil || !user.CheckModeratePermission(repoName) {
		return c.Status(fiber.StatusForbidden).SendString("Only administrators and regional moderators may adjust repository quotas and policy")
	}

	var req updateGitRepoPolicyRequest
	if err := utils.ReadJSONLimited(c, &req, 16<<10); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid policy payload")
	}

	state.Inner.ConfigWriteLock.Lock()
	defer state.Inner.ConfigWriteLock.Unlock()

	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || (repo.NormalizedFormat() != config.RepositoryFormatGithub && repo.NormalizedFormat() != config.RepositoryFormatGitLab) {
		return c.Status(fiber.StatusNotFound).SendString("Git repository was not found")
	}
	if repo.GitRepo == nil {
		repo.GitRepo = config.DefaultGitRepoPolicy()
	}

	if req.Mode != "" {
		if req.Mode == config.GitRepoModeStrict || req.Mode == config.GitRepoModeLax {
			repo.GitRepo.Mode = req.Mode
		}
	}
	if req.MaxVersionSizeBytes != nil {
		repo.GitRepo.MaxVersionSizeBytes = *req.MaxVersionSizeBytes
	}
	if req.MaxDailyTrafficBytes != nil {
		repo.GitRepo.MaxDailyTrafficBytes = *req.MaxDailyTrafficBytes
	}
	if req.RetainLatestWorkflowRuns != nil {
		repo.GitRepo.RetainLatestWorkflowRuns = req.RetainLatestWorkflowRuns
	}
	if req.RetainLatestReleases != nil {
		repo.GitRepo.RetainLatestReleases = req.RetainLatestReleases
	}
	if req.QuotaRequestCooldownHours != nil {
		repo.GitRepo.QuotaRequestCooldownHours = *req.QuotaRequestCooldownHours
	}
	if req.SuperTeamPrefix != "" {
		return c.Status(fiber.StatusBadRequest).SendString("Global teams are not allowed for Git repositories")
	}
	repo.GitRepo.SuperTeamPrefix = ""
	if req.Owner != "" {
		repo.GitRepo.Owner = strings.TrimSpace(req.Owner)
	}

	if err := state.GetDB().SaveRepositorySettings(cfg.Maven); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save repository settings")
	}
	state.Inner.Config.Store(cfg)

	policy := repo.EffectiveGitPolicy()
	return protohttp.Write(c, &pb.GitRepoPolicyResponse{
		Mode:                      policy.EffectiveMode(),
		MaxVersionSizeBytes:       policy.MaxVersionSizeBytes,
		MaxDailyTrafficBytes:      policy.MaxDailyTrafficBytes,
		RetainLatestWorkflowRuns:  policy.ShouldRetainLatestWorkflowRuns(),
		RetainLatestReleases:      policy.ShouldRetainLatestReleases(),
		QuotaRequestCooldownHours: int32(policy.QuotaRequestCooldownHours),
		SuperTeamPrefix:           policy.SuperTeamPrefix,
		Owner:                     policy.Owner,
		RegisteredRepos:           policy.RegisteredRepos,
		DailyTrafficTodayBytes:    storage.GetGitDailyTraffic(repoName, ""),
	})
}

func registerGitRepoAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	user := auth.GetUser(c)
	session := auth.CurrentSessionToken(c)
	if user == nil || user.Username == "" || user.Username == "guest" {
		c.Set("X-Renop-Error-Code", "git_repo_signin_required")
		return c.Status(fiber.StatusUnauthorized).SendString("Sign in is required to register repositories")
	}

	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || (repo.NormalizedFormat() != config.RepositoryFormatGithub && repo.NormalizedFormat() != config.RepositoryFormatGitLab) {
		return c.Status(fiber.StatusNotFound).SendString("Git repository was not found")
	}

	var req gitRepoRegistrationRequest
	if err := utils.ReadJSONLimited(c, &req, 4096); err != nil {
		c.Set("X-Renop-Error-Code", "git_repo_invalid_payload")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid registration payload")
	}

	targetRepo := strings.TrimSpace(req.TargetRepo)
	parts := strings.Split(targetRepo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		c.Set("X-Renop-Error-Code", "git_repo_invalid_format")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid repository format: must be <owner>/<repository>")
	}
	owner := parts[0]
	upstreamRepo := parts[1]

	// Verify OAuth and ownership
	if repo.NormalizedFormat() == config.RepositoryFormatGithub {
		if !cfg.Server.IsGitHubOAuthEnabled() {
			c.Set("X-Renop-Error-Code", "git_repo_oauth_required")
			return c.Status(fiber.StatusBadRequest).SendString("GitHub OAuth must be enabled on the server to register GitHub repositories")
		}
		principal, err := state.GetDB().GetRecentGitHubPrincipal(user.Username, owner, 0)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to verify GitHub identity")
		}
		if principal == nil {
			c.Set("X-Renop-Error-Code", "git_repo_ownership_required")
			return c.Status(fiber.StatusForbidden).SendString("You must own this repository or have administrative permissions in the organization")
		}
	} else if repo.NormalizedFormat() == config.RepositoryFormatGitLab {
		if !cfg.Server.IsGitLabOAuthEnabled() {
			c.Set("X-Renop-Error-Code", "git_repo_oauth_required")
			return c.Status(fiber.StatusBadRequest).SendString("GitLab OAuth must be enabled on the server to register GitLab repositories")
		}
		identities, err := state.GetDB().GetOAuthIdentities(user.Username)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to verify GitLab identity")
		}
		authorized := false
		for _, identity := range identities {
			if slices.Contains(identity.Namespaces, strings.ToLower(owner)) {
				authorized = true
				break
			}
		}
		if !authorized {
			c.Set("X-Renop-Error-Code", "git_repo_ownership_required")
			return c.Status(fiber.StatusForbidden).SendString("You must own this repository or be an owner of the GitLab group")
		}
	}

	// Validate upstream existence and auto-detect redirects
	canonical, exists, checkErr := storage.CheckGitRepoUpstream(c.Context(), cfg, repo, owner, upstreamRepo)
	if checkErr == nil {
		if !exists {
			c.Set("X-Renop-Error-Code", "git_repo_not_found")
			return c.Status(fiber.StatusNotFound).SendString("Upstream git repository was not found")
		}
		if canonical != "" && !strings.EqualFold(canonical, targetRepo) {
			targetRepo = canonical
			parts = strings.Split(targetRepo, "/")
			owner = parts[0]
			upstreamRepo = parts[1]
		}
	}

	policy := repo.EffectiveGitPolicy()
	if policy.IsRepoRegistered(targetRepo) {
		c.Set("X-Renop-Error-Code", "git_repo_already_registered")
		return c.Status(fiber.StatusConflict).SendString("Repository is already registered")
	}

	task, err := state.GetDB().CreateGitRepoRegistrationTask(repo.Name, targetRepo, policy.SuperTeamPrefix, user.Username, session, time.Now().UnixMilli())
	if errors.Is(err, core.ErrReviewTaskExists) {
		c.Set("X-Renop-Error-Code", "git_repo_request_pending")
		return c.Status(fiber.StatusConflict).SendString("A registration request is already pending for this repository")
	}
	if err != nil {
		c.Set("X-Renop-Error-Code", "git_repo_register_failed")
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to submit registration application: " + err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

func unregisterGitRepoAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	user := auth.GetUser(c)
	if user == nil || !user.CheckModeratePermission(repoName) {
		return c.Status(fiber.StatusForbidden).SendString("Only administrators and regional moderators may remove registered repositories")
	}

	targetRepo := strings.Trim(c.Params("*"), "/")
	if targetRepo == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Target repository is required")
	}

	state.Inner.ConfigWriteLock.Lock()
	defer state.Inner.ConfigWriteLock.Unlock()

	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || repo.GitRepo == nil {
		return c.Status(fiber.StatusNotFound).SendString("Repository was not found")
	}

	remaining := make([]string, 0, len(repo.GitRepo.RegisteredRepos))
	removed := false
	for _, registered := range repo.GitRepo.RegisteredRepos {
		if strings.EqualFold(registered, targetRepo) {
			removed = true
		} else {
			remaining = append(remaining, registered)
		}
	}
	if !removed {
		return c.Status(fiber.StatusNotFound).SendString("Target repository was not registered")
	}

	repo.GitRepo.RegisteredRepos = remaining
	if err := state.GetDB().SaveRepositorySettings(cfg.Maven); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to update repository settings")
	}
	state.Inner.Config.Store(cfg)

	return c.JSON(fiber.Map{"ok": true, "removed": targetRepo})
}

func quotaRequestGitRepoAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	user := auth.GetUser(c)
	session := auth.CurrentSessionToken(c)
	if user == nil || user.Username == "" || user.Username == "guest" {
		c.Set("X-Renop-Error-Code", "git_repo_signin_required")
		return c.Status(fiber.StatusUnauthorized).SendString("Sign in is required to request quota modifications")
	}

	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || (repo.NormalizedFormat() != config.RepositoryFormatGithub && repo.NormalizedFormat() != config.RepositoryFormatGitLab) {
		return c.Status(fiber.StatusNotFound).SendString("Git repository was not found")
	}

	policy := repo.EffectiveGitPolicy()
	if policy.SuperTeamPrefix != "" {
		role, _ := state.GetDB().GetSuperTeamRole(policy.SuperTeamPrefix, user.Username)
		if role < core.SuperTeamRoleManage {
			c.Set("X-Renop-Error-Code", "git_repo_permission_denied")
			return c.Status(fiber.StatusForbidden).SendString("You must be at least a manager (L3) in the managing Super Team to apply for quota modifications")
		}
	} else {
		isManager := strings.EqualFold(policy.Owner, user.Username) || user.CheckUpdatePermission(repo.Name) || user.IsManager()
		if !isManager {
			c.Set("X-Renop-Error-Code", "git_repo_permission_denied")
			return c.Status(fiber.StatusForbidden).SendString("You must be the repository manager to apply for quota modifications")
		}
	}

	var req gitRepoQuotaRequest
	if err := utils.ReadJSONLimited(c, &req, 4096); err != nil {
		c.Set("X-Renop-Error-Code", "git_repo_invalid_payload")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid quota request payload")
	}

	targetRepo := strings.TrimSpace(req.TargetRepo)
	if targetRepo == "" {
		c.Set("X-Renop-Error-Code", "git_repo_target_required")
		return c.Status(fiber.StatusBadRequest).SendString("Target repository is required")
	}
	parts := strings.Split(targetRepo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		c.Set("X-Renop-Error-Code", "git_repo_invalid_format")
		return c.Status(fiber.StatusBadRequest).SendString("Invalid repository format: must be <owner>/<repository>")
	}

	if !policy.IsRepoRegistered(targetRepo) {
		if policy.EffectiveMode() == config.GitRepoModeStrict {
			c.Set("X-Renop-Error-Code", "git_repo_not_registered")
			return c.Status(fiber.StatusBadRequest).SendString("Target repository is not registered")
		}
		state.Inner.ConfigWriteLock.Lock()
		if repo.GitRepo == nil {
			repo.GitRepo = config.DefaultGitRepoPolicy()
		}
		repo.GitRepo.RegisteredRepos = append(repo.GitRepo.RegisteredRepos, targetRepo)
		_ = state.GetDB().SaveRepositorySettings(cfg.Maven)
		state.Inner.ConfigWriteLock.Unlock()
	}

	payloadBytes, err := json.MarshalOf(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to serialize quota request payload")
	}

	task, err := state.GetDB().CreateGitRepoQuotaTask(repo.Name, targetRepo, policy.SuperTeamPrefix, user.Username, session, payloadBytes, policy.QuotaRequestCooldownHours, time.Now().UnixMilli())
	if errors.Is(err, core.ErrReviewTaskExists) {
		c.Set("X-Renop-Error-Code", "git_repo_quota_pending")
		return c.Status(fiber.StatusConflict).SendString("A quota modification request is already pending for this repository")
	}
	if errors.Is(err, core.ErrReviewTaskConflict) {
		c.Set("X-Renop-Error-Code", "git_repo_quota_cooldown")
		return c.Status(fiber.StatusTooManyRequests).SendString("Quota request cooldown is active. Please wait before submitting another request.")
	}
	if err != nil {
		c.Set("X-Renop-Error-Code", "git_repo_quota_failed")
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to submit quota request: " + err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

func setGitRepoLockAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	user := auth.GetUser(c)
	session := auth.CurrentSessionToken(c)
	if user == nil || !user.CheckModeratePermission(repoName) || auth.CurrentCredentialKind(c) != "session" || session == "" || c.Cookies("renop_session") != session {
		return c.Status(fiber.StatusForbidden).SendString("A moderator browser session is required")
	}

	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || (repo.NormalizedFormat() != config.RepositoryFormatGithub && repo.NormalizedFormat() != config.RepositoryFormatGitLab) {
		return c.Status(fiber.StatusNotFound).SendString("Git repository was not found")
	}

	var req gitRepoLockRequest
	if err := utils.ReadJSONLimited(c, &req, 4096); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid lock payload")
	}

	targetRepo := strings.TrimSpace(req.TargetRepo)
	if targetRepo == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Target repository is required")
	}

	target := core.ResourceLockTarget{
		Format:     repo.NormalizedFormat(),
		Repository: repo.Name,
		Name:       targetRepo,
		Version:    strings.TrimSpace(req.Version),
	}

	lock := &core.ResourceLock{
		ResourceLockTarget: target,
		Source:             core.ResourceLockManual,
		Mode:               req.Mode,
		Reason:             req.Reason,
		ReasonText:         req.ReasonText,
		LockedAt:           time.Now().UnixMilli(),
	}

	if err := state.GetDB().SetResourceLock(lock, user.Username, session); err != nil {
		if errors.Is(err, core.ErrResourceLockPermission) {
			return c.Status(fiber.StatusForbidden).SendString("Moderator permission is required")
		}
		if errors.Is(err, core.ErrResourceLockInvalid) {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid resource lock parameters")
		}
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to set resource lock")
	}

	audit.Log(state, &core.AuditLogEntry{
		Username: user.Username,
		Operator: user.Username,
		Action:   audit.ActionResourceLock,
		Details:  "Repository: " + repoName + ", target: " + targetRepo + ", version: " + req.Version + ", reason: " + req.Reason,
		IP:       c.IP(),
	})

	return c.JSON(fiber.Map{"ok": true})
}

func deleteGitRepoLockAPI(c fiber.Ctx, state *core.AppState) error {
	repoName := c.Params("repo_name")
	user := auth.GetUser(c)
	session := auth.CurrentSessionToken(c)
	if user == nil || !user.CheckModeratePermission(repoName) || auth.CurrentCredentialKind(c) != "session" || session == "" || c.Cookies("renop_session") != session {
		return c.Status(fiber.StatusForbidden).SendString("A moderator browser session is required")
	}

	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[repoName]
	if repo == nil || (repo.NormalizedFormat() != config.RepositoryFormatGithub && repo.NormalizedFormat() != config.RepositoryFormatGitLab) {
		return c.Status(fiber.StatusNotFound).SendString("Git repository was not found")
	}

	targetRepo := strings.TrimSpace(c.Query("target_repo"))
	if targetRepo == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Target repository query parameter is required")
	}
	version := strings.TrimSpace(c.Query("version"))

	target := core.ResourceLockTarget{
		Format:     repo.NormalizedFormat(),
		Repository: repo.Name,
		Name:       targetRepo,
		Version:    version,
	}

	if err := state.GetDB().DeleteResourceLock(target, core.ResourceLockManual, user.Username, session); err != nil {
		if errors.Is(err, core.ErrResourceLockPermission) {
			return c.Status(fiber.StatusForbidden).SendString("Moderator permission is required")
		}
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to delete resource lock")
	}

	audit.Log(state, &core.AuditLogEntry{
		Username: user.Username,
		Operator: user.Username,
		Action:   audit.ActionResourceUnlock,
		Details:  "Repository: " + repoName + ", target: " + targetRepo + ", version: " + version,
		IP:       c.IP(),
	})

	return c.JSON(fiber.Map{"ok": true})
}
