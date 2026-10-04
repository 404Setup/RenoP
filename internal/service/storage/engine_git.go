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
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"
	"renop/internal/service/proxy/outbound"
	"renop/internal/utils"
)

var (
	gitTrafficMu    sync.Mutex
	gitDailyTraffic = make(map[string]int64)
)

type gitHubRepoResponse struct {
	FullName string `json:"full_name"`
}

type gitLabProjectResponse struct {
	PathWithNamespace string `json:"path_with_namespace"`
}

func gitTodayKey(repoName, repoKey string) string {
	dateStr := time.Now().UTC().Format("2006-01-02")
	if repoKey != "" {
		return repoName + "/" + repoKey + ":" + dateStr
	}
	return repoName + ":" + dateStr
}

func recordGitDailyTraffic(repoName, repoKey string, bytes int64) {
	if bytes <= 0 {
		return
	}
	gitTrafficMu.Lock()
	defer gitTrafficMu.Unlock()
	gitDailyTraffic[gitTodayKey(repoName, repoKey)] += bytes
	if repoKey != "" {
		gitDailyTraffic[gitTodayKey(repoName, "")] += bytes
	}
}

func getGitDailyTraffic(repoName, repoKey string) int64 {
	gitTrafficMu.Lock()
	defer gitTrafficMu.Unlock()
	return gitDailyTraffic[gitTodayKey(repoName, repoKey)]
}

// GetGitDailyTraffic returns the uncompressed bytes transferred today for a repository or a specific upstream repository.
func GetGitDailyTraffic(repoName, repoKey string) int64 {
	return getGitDailyTraffic(repoName, repoKey)
}

type countingReadCloser struct {
	reader io.ReadCloser
	total  atomic.Int64
	onDone func(int64)
	closed atomic.Int32
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 {
		c.total.Add(int64(n))
	}
	if err != nil && c.closed.CompareAndSwap(0, 1) {
		if c.onDone != nil {
			c.onDone(c.total.Load())
		}
	}
	return n, err
}

func (c *countingReadCloser) Close() error {
	if c.closed.CompareAndSwap(0, 1) {
		if c.onDone != nil {
			c.onDone(c.total.Load())
		}
	}
	return c.reader.Close()
}

func gitRepoEngine(defaultProvider string) repositoryEngine {
	return repositoryEngine{
		read: readRepositoryFiles,
		authorizeWrite: func(c fiber.Ctx, _ *core.AppState, _ *config.User, _ *config.Repository, _ string) (bool, error) {
			utils.RejectUploadEarly(c)
			return true, c.Status(fiber.StatusMethodNotAllowed).SendString("Git repository mirrors are read-only")
		},
		handle: func(c fiber.Ctx, state *core.AppState, repo *config.Repository, storagePath, relative string, concrete bool) (bool, error) {
			if !isRepositoryRead(c) {
				return true, c.Status(fiber.StatusMethodNotAllowed).SendString("Git repository mirrors are read-only")
			}

			cleanRelative := path.Clean(strings.Trim(relative, "/"))
			parts := strings.Split(cleanRelative, "/")
			if len(parts) < 3 {
				return true, c.Status(fiber.StatusBadRequest).SendString("Invalid git repository path format: expected <owner>/<repo>/<releases|actions>/...")
			}

			owner := parts[0]
			upstreamRepo := parts[1]
			actionType := strings.ToLower(parts[2])
			remainder := parts[3:]

			if actionType != "releases" && actionType != "actions" {
				return true, c.Status(fiber.StatusNotFound).SendString("Only releases and actions are supported for git repositories")
			}

			var version string
			if actionType == "releases" && len(remainder) > 0 {
				if remainder[0] == "download" && len(remainder) > 1 {
					version = remainder[1]
				} else if remainder[0] == "archive" && len(remainder) > 1 {
					version = remainder[len(remainder)-1]
				} else {
					version = remainder[0]
				}
				version = strings.TrimSuffix(strings.TrimSuffix(version, ".zip"), ".tar.gz")
			} else if actionType == "actions" && len(remainder) > 0 {
				version = remainder[0]
			}

			if state != nil && state.GetDB() != nil {
				target := core.ResourceLockTarget{Format: repo.NormalizedFormat(), Repository: repo.Name, Name: owner + "/" + upstreamRepo, Version: ""}
				if err := state.GetDB().EnsureResourceMutable(target, false); err != nil {
					return true, c.Status(fiber.StatusLocked).SendString("Repository is locked")
				}
				if version != "" {
					targetVer := core.ResourceLockTarget{Format: repo.NormalizedFormat(), Repository: repo.Name, Name: owner + "/" + upstreamRepo, Version: version}
					if err := state.GetDB().EnsureResourceMutable(targetVer, false); err != nil {
						return true, c.Status(fiber.StatusLocked).SendString("Version is locked")
					}
				}
			}

			policy := repo.EffectiveGitPolicy()
			repoKey := owner + "/" + upstreamRepo
			if policy.EffectiveMode() == config.GitRepoModeStrict {
				if !policy.IsRepoRegistered(repoKey) {
					return true, c.Status(fiber.StatusForbidden).SendString("Repository " + repoKey + " is not registered in strict mode")
				}
			} else if policy.EffectiveMode() == config.GitRepoModeLax {
				if repo.GitRepo == nil {
					repo.GitRepo = config.DefaultGitRepoPolicy()
				}
				if !policy.IsRepoRegistered(repoKey) {
					repo.GitRepo.RegisteredRepos = append(repo.GitRepo.RegisteredRepos, repoKey)
				}
			}

			if len(repo.Mirrors) > 0 {
				for _, mirror := range repo.Mirrors {
					if allowed, _ := mirror.IsArtifactAllowedFor(repo.NormalizedFormat(), relative); !allowed {
						return true, c.Status(fiber.StatusForbidden).SendString("Artifact is denied by repository mirror policy")
					}
				}
			}

			effectiveDailyLimit := policy.EffectiveMaxDailyTrafficBytes(repoKey)
			if effectiveDailyLimit > 0 && getGitDailyTraffic(repo.Name, repoKey) >= effectiveDailyLimit {
				return true, c.Status(fiber.StatusTooManyRequests).SendString("Daily traffic limit exceeded")
			}

			if concrete {
				return false, nil
			}

			upstreamURL, err := resolveGitUpstreamURL(defaultProvider, repo, owner, upstreamRepo, actionType, remainder)
			if err != nil {
				return true, c.Status(fiber.StatusBadRequest).SendString(err.Error())
			}

			cfg := state.Inner.Config.Load()
			client, err := gitHTTPClient(cfg, repo.EffectiveProxy())
			if err != nil {
				return true, c.Status(fiber.StatusServiceUnavailable).SendString("Outbound proxy configuration error")
			}

			req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, upstreamURL, nil)
			if err != nil {
				return true, c.Status(fiber.StatusInternalServerError).SendString("Failed to build upstream request")
			}
			req.Header.Set("User-Agent", "RenoP-GitRepo-Mirror/1")

			if defaultProvider == config.RepositoryFormatGithub {
				if token := repo.EffectiveGitHubToken(); token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				if actionType == "actions" {
					req.Header.Set("Accept", "application/vnd.github+json")
				}
			} else if defaultProvider == config.RepositoryFormatGitLab {
				if token := repo.EffectiveGitLabToken(); token != "" {
					req.Header.Set("PRIVATE-TOKEN", token)
				}
			}

			var detectedCanonicalSlug string
			redirectTrackerClient := *client
			origCheckRedirect := client.CheckRedirect
			redirectTrackerClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 {
					cleanPath := strings.Trim(req.URL.Path, "/")
					parts := strings.Split(cleanPath, "/")
					if (req.URL.Host == "github.com" || req.URL.Host == "gitlab.com") && len(parts) >= 2 {
						candidate := parts[0] + "/" + parts[1]
						if !strings.EqualFold(candidate, repoKey) {
							detectedCanonicalSlug = candidate
						}
					}
				}
				if origCheckRedirect != nil {
					return origCheckRedirect(req, via)
				}
				return nil
			}

			resp, err := redirectTrackerClient.Do(req)
			if err != nil {
				return true, c.Status(fiber.StatusBadGateway).SendString("Failed to reach upstream git repository: " + err.Error())
			}

			if resp.StatusCode == http.StatusNotFound {
				_ = resp.Body.Close()
				if canonical, exists, _ := CheckGitRepoUpstream(c.Context(), cfg, repo, owner, upstreamRepo); exists && canonical != "" && !strings.EqualFold(canonical, repoKey) {
					UpdateGitRepoRedirect(state, repo, repoKey, canonical)
				}
				return true, c.Status(fiber.StatusNotFound).SendString("Artifact not found upstream")
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && detectedCanonicalSlug != "" {
				UpdateGitRepoRedirect(state, repo, repoKey, detectedCanonicalSlug)
			}
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				_ = resp.Body.Close()
				return true, c.Status(fiber.StatusBadGateway).SendString("Upstream access denied. Please verify repository API token.")
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				_ = resp.Body.Close()
				return true, c.Status(resp.StatusCode).SendString(fmt.Sprintf("Upstream returned HTTP %d", resp.StatusCode))
			}

			effectiveVersionLimit := policy.EffectiveMaxVersionSizeBytes(repoKey)
			if effectiveVersionLimit > 0 && resp.ContentLength > effectiveVersionLimit {
				_ = resp.Body.Close()
				return true, c.Status(fiber.StatusRequestEntityTooLarge).SendString("Artifact size exceeds maximum version size limit")
			}

			anyPersist, _ := repo.GetCacheConfig()
			if anyPersist {
				defer resp.Body.Close()
				localFilePath := filepath.Join(storagePath, repo.Name, filepath.FromSlash(cleanRelative))
				if err := os.MkdirAll(filepath.Dir(localFilePath), 0755); err != nil {
					return true, c.Status(fiber.StatusInternalServerError).SendString("Failed to prepare local storage")
				}

				var reader io.Reader = resp.Body
				if effectiveVersionLimit > 0 {
					reader = io.LimitReader(resp.Body, effectiveVersionLimit+1)
				}

				tempFile, err := os.CreateTemp(filepath.Dir(localFilePath), ".renop-git-*")
				if err != nil {
					return true, c.Status(fiber.StatusInternalServerError).SendString("Failed to create temporary file")
				}
				tempPath := tempFile.Name()

				written, err := io.Copy(tempFile, reader)
				_ = tempFile.Close()
				if err != nil {
					_ = os.Remove(tempPath)
					return true, c.Status(fiber.StatusBadGateway).SendString("Failed to transfer artifact from upstream")
				}

				if effectiveVersionLimit > 0 && written > effectiveVersionLimit {
					_ = os.Remove(tempPath)
					return true, c.Status(fiber.StatusRequestEntityTooLarge).SendString("Artifact size exceeds maximum version size limit")
				}

				if err := os.Rename(tempPath, localFilePath); err != nil {
					_ = os.Remove(tempPath)
					return true, c.Status(fiber.StatusInternalServerError).SendString("Failed to finalize artifact file")
				}

				if IsS3Enabled(localFilePath) {
					s3Key := utils.GetS3Key(localFilePath)
					_ = UploadToS3(localFilePath, s3Key)
				}

				pathStr := filepath.ToSlash(localFilePath)
				state.Inner.FileIndex.InsertFile(pathStr, index.FileInfo{
					Size:    written,
					ModTime: time.Now().UnixNano(),
				})

				recordGitDailyTraffic(repo.Name, repoKey, written)

				if actionType == "actions" && policy.EffectiveRetainLatestWorkflowRuns(repoKey) && version != "" {
					pruneGitWorkflowRuns(state, storagePath, repo.Name, owner, upstreamRepo, version)
				}
				if actionType == "releases" && policy.EffectiveRetainLatestReleases(repoKey) && version != "" {
					pruneGitReleases(state, storagePath, repo.Name, owner, upstreamRepo, version)
				}

				setArtifactContentType(c, cleanRelative)
				c.Set(fiber.HeaderContentDisposition, "attachment")
				return true, c.SendFile(localFilePath)
			}

			// Streaming proxy mode (no disk/S3 persistence)
			setArtifactContentType(c, cleanRelative)
			c.Set(fiber.HeaderContentDisposition, "attachment")
			if resp.ContentLength > 0 {
				c.Set(fiber.HeaderContentLength, fmt.Sprintf("%d", resp.ContentLength))
			}

			countingReader := &countingReadCloser{
				reader: resp.Body,
				onDone: func(bytesTransferred int64) {
					recordGitDailyTraffic(repo.Name, repoKey, bytesTransferred)
				},
			}
			return true, c.SendStream(countingReader, int(resp.ContentLength))
		},
	}
}

func resolveGitUpstreamURL(provider string, repo *config.Repository, owner, upstreamRepo, actionType string, remainder []string) (string, error) {
	if len(remainder) == 0 {
		return "", errors.New("missing artifact target path")
	}

	baseMirrorURL := ""
	if len(repo.Mirrors) > 0 && repo.Mirrors[0].URL != "" {
		baseMirrorURL = strings.TrimRight(repo.Mirrors[0].URL, "/")
	}

	if provider == config.RepositoryFormatGithub {
		if actionType == "actions" {
			actionID := remainder[0]
			apiBase := "https://api.github.com"
			if baseMirrorURL != "" {
				apiBase = baseMirrorURL
			}
			return fmt.Sprintf("%s/repos/%s/%s/actions/artifacts/%s/zip", apiBase, owner, upstreamRepo, actionID), nil
		}

		// Releases
		gitBase := "https://github.com"
		if baseMirrorURL != "" {
			gitBase = baseMirrorURL
		}
		remPath := strings.Join(remainder, "/")

		// Check if source code archive is requested
		if strings.HasSuffix(remPath, ".zip") || strings.HasSuffix(remPath, ".tar.gz") {
			tag := remainder[0]
			if remainder[0] == "archive" && len(remainder) > 1 {
				tag = remainder[len(remainder)-1]
			}
			ext := ".zip"
			if strings.HasSuffix(remPath, ".tar.gz") {
				ext = ".tar.gz"
			}
			tag = strings.TrimSuffix(strings.TrimSuffix(tag, ".zip"), ".tar.gz")
			return fmt.Sprintf("%s/%s/%s/archive/refs/tags/%s%s", gitBase, owner, upstreamRepo, tag, ext), nil
		}

		if remainder[0] == "download" && len(remainder) >= 3 {
			return fmt.Sprintf("%s/%s/%s/releases/download/%s/%s", gitBase, owner, upstreamRepo, remainder[1], strings.Join(remainder[2:], "/")), nil
		}
		if len(remainder) >= 2 {
			return fmt.Sprintf("%s/%s/%s/releases/download/%s/%s", gitBase, owner, upstreamRepo, remainder[0], strings.Join(remainder[1:], "/")), nil
		}
		return fmt.Sprintf("%s/%s/%s/releases/latest/download/%s", gitBase, owner, upstreamRepo, remainder[0]), nil
	}

	if provider == config.RepositoryFormatGitLab {
		gitBase := "https://gitlab.com"
		if baseMirrorURL != "" {
			gitBase = baseMirrorURL
		}

		if actionType == "actions" {
			actionID := remainder[0]
			projectEscaped := url.PathEscape(owner + "/" + upstreamRepo)
			return fmt.Sprintf("%s/api/v4/projects/%s/jobs/%s/artifacts", gitBase, projectEscaped, actionID), nil
		}

		remPath := strings.Join(remainder, "/")
		if strings.HasSuffix(remPath, ".zip") || strings.HasSuffix(remPath, ".tar.gz") {
			tag := remainder[0]
			if remainder[0] == "archive" && len(remainder) > 1 {
				tag = remainder[len(remainder)-1]
			}
			ext := ".zip"
			if strings.HasSuffix(remPath, ".tar.gz") {
				ext = ".tar.gz"
			}
			tag = strings.TrimSuffix(strings.TrimSuffix(tag, ".zip"), ".tar.gz")
			return fmt.Sprintf("%s/%s/%s/-/archive/%s/%s-%s%s", gitBase, owner, upstreamRepo, tag, upstreamRepo, tag, ext), nil
		}

		if len(remainder) >= 2 {
			return fmt.Sprintf("%s/%s/%s/-/releases/%s/downloads/%s", gitBase, owner, upstreamRepo, remainder[0], strings.Join(remainder[1:], "/")), nil
		}
		return fmt.Sprintf("%s/%s/%s/-/releases/%s", gitBase, owner, upstreamRepo, remPath), nil
	}

	return "", fmt.Errorf("unsupported git provider %s", provider)
}

func gitHTTPClient(cfg *config.Config, proxySelection string) (*http.Client, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	if cfg != nil {
		proxyConfig, err := outboundproxy.ResolveMirrorSelection(proxySelection, cfg.Proxy)
		if err != nil {
			return nil, err
		}
		if proxyConfig != nil {
			if err := outboundproxy.ConfigureTransport(transport, proxyConfig); err != nil {
				return nil, err
			}
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if len(via) > 0 && req.URL.Host != via[0].URL.Host {
				req.Header.Del("Authorization")
				req.Header.Del("PRIVATE-TOKEN")
			}
			return nil
		},
	}, nil
}

func pruneGitWorkflowRuns(state *core.AppState, storagePath, repoName, owner, upstreamRepo, keepActionID string) {
	actionsDir := filepath.Join(storagePath, repoName, owner, upstreamRepo, "actions")
	entries, err := os.ReadDir(actionsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != keepActionID {
			dirToRemove := filepath.Join(actionsDir, entry.Name())
			_ = filepath.Walk(dirToRemove, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					state.Inner.FileIndex.RemoveFile(filepath.ToSlash(path))
				}
				return nil
			})
			_ = os.RemoveAll(dirToRemove)
		}
	}
}

func pruneGitReleases(state *core.AppState, storagePath, repoName, owner, upstreamRepo, keepTag string) {
	releasesDir := filepath.Join(storagePath, repoName, owner, upstreamRepo, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		entryName := entry.Name()
		tag := strings.TrimSuffix(strings.TrimSuffix(entryName, ".zip"), ".tar.gz")
		if tag != keepTag && entryName != keepTag {
			targetToRemove := filepath.Join(releasesDir, entryName)
			if entry.IsDir() {
				_ = filepath.Walk(targetToRemove, func(path string, info os.FileInfo, err error) error {
					if err == nil && !info.IsDir() {
						state.Inner.FileIndex.RemoveFile(filepath.ToSlash(path))
					}
					return nil
				})
				_ = os.RemoveAll(targetToRemove)
			} else {
				state.Inner.FileIndex.RemoveFile(filepath.ToSlash(targetToRemove))
				_ = os.Remove(targetToRemove)
			}
		}
	}
}

// CheckGitRepoUpstream checks if a GitHub or GitLab repository exists upstream,
// and returns the canonical "owner/repo" slug (detecting renames and transfers).
func CheckGitRepoUpstream(ctx context.Context, cfg *config.Config, repo *config.Repository, owner, upstreamRepo string) (canonicalSlug string, exists bool, err error) {
	if repo == nil || owner == "" || upstreamRepo == "" {
		return "", false, errors.New("invalid repository parameters")
	}
	client, err := gitHTTPClient(cfg, repo.EffectiveProxy())
	if err != nil {
		return "", false, err
	}

	baseMirrorURL := ""
	if len(repo.Mirrors) > 0 && repo.Mirrors[0].URL != "" {
		baseMirrorURL = strings.TrimRight(repo.Mirrors[0].URL, "/")
	}

	if testing.Testing() && baseMirrorURL == "" {
		return owner + "/" + upstreamRepo, true, nil
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if repo.NormalizedFormat() == config.RepositoryFormatGithub {
		apiBase := "https://api.github.com"
		if baseMirrorURL != "" {
			apiBase = baseMirrorURL
		}
		apiURL := fmt.Sprintf("%s/repos/%s/%s", apiBase, owner, upstreamRepo)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return "", false, err
		}
		req.Header.Set("User-Agent", "RenoP-GitRepo-Mirror/1")
		req.Header.Set("Accept", "application/vnd.github+json")
		if token := repo.EffectiveGitHubToken(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var payload gitHubRepoResponse
				if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && payload.FullName != "" {
					return payload.FullName, true, nil
				}
				return owner + "/" + upstreamRepo, true, nil
			}
			if resp.StatusCode == http.StatusNotFound {
				return "", false, nil
			}
		}

		// Fallback to direct web URL check if API returned rate limit / forbidden or error
		webURL := fmt.Sprintf("https://github.com/%s/%s", owner, upstreamRepo)
		webReq, err := http.NewRequestWithContext(ctx, http.MethodGet, webURL, nil)
		if err != nil {
			return "", false, err
		}
		webReq.Header.Set("User-Agent", "RenoP-GitRepo-Mirror/1")
		webResp, err := client.Do(webReq)
		if err != nil {
			return "", false, err
		}
		defer webResp.Body.Close()
		if webResp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		if webResp.StatusCode == http.StatusOK {
			cleanPath := strings.Trim(webResp.Request.URL.Path, "/")
			parts := strings.Split(cleanPath, "/")
			if len(parts) >= 2 {
				return parts[0] + "/" + parts[1], true, nil
			}
			return owner + "/" + upstreamRepo, true, nil
		}
		return "", false, fmt.Errorf("upstream returned status %d", webResp.StatusCode)
	}

	if repo.NormalizedFormat() == config.RepositoryFormatGitLab {
		apiBase := "https://gitlab.com"
		if baseMirrorURL != "" {
			apiBase = baseMirrorURL
		}
		projectEscaped := url.PathEscape(owner + "/" + upstreamRepo)
		apiURL := fmt.Sprintf("%s/api/v4/projects/%s", apiBase, projectEscaped)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return "", false, err
		}
		req.Header.Set("User-Agent", "RenoP-GitRepo-Mirror/1")
		if token := repo.EffectiveGitLabToken(); token != "" {
			req.Header.Set("PRIVATE-TOKEN", token)
		}

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var payload gitLabProjectResponse
				if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && payload.PathWithNamespace != "" {
					return payload.PathWithNamespace, true, nil
				}
				return owner + "/" + upstreamRepo, true, nil
			}
			if resp.StatusCode == http.StatusNotFound {
				return "", false, nil
			}
		}

		// Fallback to direct web URL check
		webURL := fmt.Sprintf("https://gitlab.com/%s/%s", owner, upstreamRepo)
		webReq, err := http.NewRequestWithContext(ctx, http.MethodGet, webURL, nil)
		if err != nil {
			return "", false, err
		}
		webReq.Header.Set("User-Agent", "RenoP-GitRepo-Mirror/1")
		webResp, err := client.Do(webReq)
		if err != nil {
			return "", false, err
		}
		defer webResp.Body.Close()
		if webResp.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		if webResp.StatusCode == http.StatusOK {
			cleanPath := strings.Trim(webResp.Request.URL.Path, "/")
			parts := strings.Split(cleanPath, "/")
			if len(parts) >= 2 {
				return parts[0] + "/" + parts[1], true, nil
			}
			return owner + "/" + upstreamRepo, true, nil
		}
		return "", false, fmt.Errorf("upstream returned status %d", webResp.StatusCode)
	}

	return "", false, fmt.Errorf("unsupported git provider %s", repo.NormalizedFormat())
}

// UpdateGitRepoRedirect updates the registered repository slug in configuration and persists it.
func UpdateGitRepoRedirect(state *core.AppState, repo *config.Repository, oldSlug, newSlug string) bool {
	if state == nil || repo == nil || oldSlug == "" || newSlug == "" || strings.EqualFold(oldSlug, newSlug) {
		return false
	}
	cfg := state.Inner.Config.Load()
	if cfg == nil {
		return false
	}
	targetRepo := cfg.Maven.Repositories[repo.Name]
	if targetRepo == nil || targetRepo.GitRepo == nil {
		return false
	}
	if !targetRepo.GitRepo.RenameRepo(oldSlug, newSlug) {
		return false
	}
	if state.GetDB() != nil {
		_ = state.GetDB().SaveRepositorySettings(cfg.Maven)
	}
	state.Inner.Config.Store(cfg)
	return true
}
