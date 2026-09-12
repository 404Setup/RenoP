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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/service/auth"
	"renop/internal/service/captcha"
	"renop/internal/service/nativepkg"
	"renop/internal/service/repositorygate"
	"renop/internal/service/storage"
	"renop/internal/utils"

	"github.com/gofiber/fiber/v3"
)

func setupNativeRoutes(router fiber.Router, state *core.AppState) {
	base := "/native/repositories/:repo_name/resources"
	router.Get(base, func(c fiber.Ctx) error { return getNativeResources(c, state) })
	router.Get(base+"/key", func(c fiber.Ctx) error {
		repo, db, user, err := nativeAPIContext(c, state, false)
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		resource, err := db.GetNativeResource(repo.Name, c.Query("name"), user.Username)
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		if resource.PublishedAt == 0 && resource.PermissionLevel < 0 && !user.CheckModeratePermission(repo.Name) {
			return storage.NativeErrorResponse(c, core.ErrNativeNotFound)
		}
		signingKey := resource.SigningKey
		if signingKey == "" {
			members, err := db.ListNativeMembers(repo.Name, resource.Name)
			if err == nil {
				for _, m := range members {
					if m.Level >= core.NativePermissionOwner {
						userKeys, err := state.GetDB().ListUserGPGKeys(m.Username)
						if err == nil && len(userKeys) > 0 {
							var b strings.Builder
							for _, k := range userKeys {
								b.WriteString(string(k.PublicKey))
								b.WriteString("\n")
							}
							signingKey = b.String()
							break
						}
					}
				}
			}
		}
		if signingKey == "" {
			return storage.NativeErrorResponse(c, core.ErrNativeNotFound)
		}
		extension := ".asc"
		if resource.Format == config.RepositoryFormatAPK {
			extension = ".rsa.pub"
		}
		c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+strings.ReplaceAll(resource.Name, "/", "-")+extension+`"`)
		c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
		return c.SendString(signingKey)
	})
	router.Get(base+"/users", func(c fiber.Ctx) error {
		repo, db, user, err := nativeAPIContext(c, state, false)
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		if err := db.CheckNativePermission(repo.Name, c.Query("name"), user.Username, core.NativePermissionManage); err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		users, err := state.GetDB().SearchTokenNames(strings.TrimSpace(c.Query("q")), 8, time.Now().UnixMilli(), user.CanViewPrivateProfiles())
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		return c.JSON(fiber.Map{"users": users})
	})
	router.Put(base+"/deprecate", func(c fiber.Ctx) error {
		release := repositorygate.AcquireMutation(c.Params("repo_name"))
		defer release()
		return deprecateNativeResource(c, state)
	})
	router.Put(base+"/locks", func(c fiber.Ctx) error {
		release := repositorygate.AcquireMutation(c.Params("repo_name"))
		defer release()
		return setNativeResourceLock(c, state)
	})
	router.Delete(base+"/locks", func(c fiber.Ctx) error {
		release := repositorygate.AcquireMutation(c.Params("repo_name"))
		defer release()
		return setNativeResourceLock(c, state)
	})
	for _, method := range []string{fiber.MethodPost, fiber.MethodPut, fiber.MethodDelete} {
		router.Add([]string{method}, base, func(c fiber.Ctx) error {
			release := repositorygate.AcquireMutation(c.Params("repo_name"))
			defer release()
			return mutateNativeResource(c, state)
		})
	}
	router.Put(base+"/members", func(c fiber.Ctx) error {
		release := repositorygate.AcquireMutation(c.Params("repo_name"))
		defer release()
		return mutateNativeMember(c, state)
	})
}

func nativeAPIContext(c fiber.Ctx, state *core.AppState, mutation bool) (*config.Repository, core.NativePackageDB, *config.User, error) {
	if state == nil || state.Inner == nil || state.GetDB() == nil {
		return nil, nil, nil, core.ErrDatabaseUnavailable
	}
	repository := c.Params("repo_name")
	cfg := state.Inner.Config.Load()
	if cfg == nil || !utils.IsValidRepositoryName(repository) {
		return nil, nil, nil, core.ErrNativeNotFound
	}
	repo := cfg.Maven.Repositories[repository]
	if repo == nil || !repo.Engine().ManagedNative {
		return nil, nil, nil, core.ErrNativeNotFound
	}
	db, ok := state.GetDB().(core.NativePackageDB)
	if !ok {
		return nil, nil, nil, core.ErrDatabaseUnavailable
	}
	user := auth.GetUser(c)
	if user == nil {
		user = &config.User{Username: "guest"}
	}
	if mutation {
		if state.IsDemo() {
			return nil, nil, nil, core.ErrDemoReadOnly
		}
		if auth.CurrentCredentialKind(c) != "session" || auth.CurrentSessionToken(c) == "" || c.Cookies("renop_session") != auth.CurrentSessionToken(c) {
			return nil, nil, nil, core.ErrNativePermission
		}
	} else if !strings.EqualFold(repo.Visibility, "PUBLIC") && !user.CheckReadPermission(repo.Name, c.Query("name"), repo.Visibility, c.Query("name") == "") {
		return nil, nil, nil, core.ErrNativePermission
	}
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	return repo, db, user, nil
}

func nativePage(c fiber.Ctx) (int, int, error) {
	limit, offset := 50, 0
	var err error
	if value := c.Query("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil {
			return 0, 0, core.ErrNativeInvalid
		}
	}
	if value := c.Query("offset"); value != "" {
		offset, err = strconv.Atoi(value)
		if err != nil {
			return 0, 0, core.ErrNativeInvalid
		}
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return 0, 0, core.ErrNativeInvalid
	}
	return limit, offset, nil
}

func getNativeResources(c fiber.Ctx, state *core.AppState) error {
	repo, db, user, err := nativeAPIContext(c, state, false)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	limit, offset, err := nativePage(c)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	staff := user.IsManager() || user.CheckModeratePermission(repo.Name)
	name := c.Query("name")
	if name == "" {
		resources, err := db.ListNativeResources(repo.Name, user.Username, staff, limit, offset)
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		for _, resource := range resources {
			resource.SigningKey = ""
			if user.IsManager() {
				resource.PermissionLevel = core.NativePermissionOwner
			}
		}
		return c.JSON(fiber.Map{"repository": repo.Name, "resources": resources, "can_create": user.CheckUpdatePermission(repo.Name) && len(repo.Mirrors) == 0, "limit": limit, "offset": offset})
	}
	resource, err := db.GetNativeResource(repo.Name, name, user.Username)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	if resource.PublishedAt == 0 && resource.PermissionLevel < 0 && !staff {
		return storage.NativeErrorResponse(c, core.ErrNativeNotFound)
	}
	if user.IsManager() {
		resource.PermissionLevel = core.NativePermissionOwner
	}
	includePending := staff || resource.PermissionLevel >= core.NativePermissionRead
	artifacts, err := db.ListNativeArtifacts(repo.Name, name, includePending, limit, offset)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	var members []*core.NativeMember
	if includePending {
		members, err = db.ListNativeMembers(repo.Name, name)
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
	}
	versions := parseNativeVersions(resource.Format, artifacts)
	return c.JSON(fiber.Map{"resource": resource, "versions": versions, "artifacts": artifacts, "members": members, "limit": limit, "offset": offset})
}

type NativeVersionSummary struct {
	Version   string                 `json:"version"`
	Published bool                   `json:"published"`
	CreatedAt int64                  `json:"created_at"`
	Artifacts []*core.NativeArtifact `json:"artifacts"`
}

func parseNativeVersions(format string, artifacts []*core.NativeArtifact) []*NativeVersionSummary {
	var list []*NativeVersionSummary
	byVersion := make(map[string]*NativeVersionSummary)
	for _, art := range artifacts {
		clean := art.Version
		if idx := strings.IndexByte(clean, '#'); idx >= 0 {
			clean = clean[:idx]
		} else if idx := strings.IndexByte(clean, '/'); idx >= 0 {
			clean = clean[:idx]
		}
		if clean == "" {
			clean = art.Version
		}
		summary, exists := byVersion[clean]
		if !exists {
			summary = &NativeVersionSummary{
				Version:   clean,
				Published: art.Published,
				CreatedAt: art.CreatedAt,
				Artifacts: make([]*core.NativeArtifact, 0, 1),
			}
			byVersion[clean] = summary
			list = append(list, summary)
		}
		if art.Published {
			summary.Published = true
		}
		if art.CreatedAt > summary.CreatedAt {
			summary.CreatedAt = art.CreatedAt
		}
		summary.Artifacts = append(summary.Artifacts, art)
	}
	return list
}

func logNativeMutation(c fiber.Ctx, state *core.AppState, details string) {
	username, operator, method, session, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{Username: username, Operator: operator, AuthMethod: method, SessionID: session, IP: ip, Action: audit.ActionNativeResourceUpdate, Details: details})
}

func mutateNativeResource(c fiber.Ctx, state *core.AppState) error {
	repo, db, user, err := nativeAPIContext(c, state, true)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	var request struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		SigningKey  string `json:"signing_key"`
		Archived    *bool  `json:"archived"`
	}
	if err := utils.ReadJSONLimited(c, &request, 72<<10); err != nil {
		return storage.NativeErrorResponse(c, core.ErrNativeInvalid)
	}
	if !core.ValidNativeResourceName(request.Name) {
		return storage.NativeErrorResponse(c, core.ErrNativeInvalid)
	}
	releaseResource := nativepkg.AcquireMutation(repo.Name, request.Name)
	defer releaseResource()
	if repo.Engine().Protocol != config.RepositoryFormatConan && strings.ContainsAny(request.Name, "/@") {
		return storage.NativeErrorResponse(c, core.ErrNativeInvalid)
	}
	switch c.Method() {
	case fiber.MethodPost:
		if !user.CheckUpdatePermission(repo.Name) {
			return storage.NativeErrorResponse(c, core.ErrNativePermission)
		}
		if len(repo.Mirrors) > 0 {
			return storage.NativeErrorResponse(c, core.ErrNativeBusy)
		}
		if err := captcha.Require(c, state, config.CaptchaPackageCreate); err != nil {
			return err
		}
		resource, err := db.CreateNativeResource(repo.Name, repo.Engine().Protocol, request.Name, user.Username, time.Now().UnixMilli())
		if err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		logNativeMutation(c, state, "Reserved "+repo.Name+"/"+request.Name)
		return c.Status(fiber.StatusCreated).JSON(resource)
	case fiber.MethodPut:
		target := core.ResourceLockTarget{Format: repo.Engine().Protocol, Repository: repo.Name, Name: request.Name}
		if err := state.GetDB().EnsureResourceMutable(target, true); err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		if request.Archived != nil {
			if err := db.SetNativeResourceArchived(repo.Name, request.Name, user.Username, *request.Archived); err != nil {
				return storage.NativeErrorResponse(c, err)
			}
		}
		if request.Description != "" || request.SigningKey != "" || request.Archived == nil {
			if err := nativepkg.ValidateSigningKey(repo.Engine().Protocol, request.SigningKey); err != nil {
				return storage.NativeErrorResponse(c, err)
			}
			err = db.UpdateNativeResource(repo.Name, request.Name, user.Username, request.Description, request.SigningKey)
		}
	case fiber.MethodDelete:
		cfg := state.Inner.Config.Load()
		if request.Version != "" {
			target := core.ResourceLockTarget{Format: repo.Engine().Protocol, Repository: repo.Name, Name: request.Name, Version: request.Version}
			if err := state.GetDB().EnsureResourceMutable(target, false); err != nil {
				return storage.NativeErrorResponse(c, err)
			}
			if err := db.CheckNativePermission(repo.Name, request.Name, user.Username, core.NativePermissionVersion); err != nil {
				return storage.NativeErrorResponse(c, err)
			}
			deleted, delErr := db.DeleteNativeVersionArtifacts(repo.Name, request.Name, request.Version)
			if delErr != nil {
				return storage.NativeErrorResponse(c, delErr)
			}
			if cfg != nil {
				for _, art := range deleted {
					local := filepath.Join(cfg.StoragePath, repo.Name, filepath.FromSlash(art.Path))
					_ = os.Remove(local)
					state.Inner.FileIndex.UnblockFile(local)
					state.InvalidateFileCache(local)
				}
			}
			logNativeMutation(c, state, "Deleted version "+repo.Name+"/"+request.Name+"@"+request.Version)
			return c.JSON(fiber.Map{"ok": true})
		}
		target := core.ResourceLockTarget{Format: repo.Engine().Protocol, Repository: repo.Name, Name: request.Name}
		if err := state.GetDB().EnsureResourceMutable(target, true); err != nil {
			return storage.NativeErrorResponse(c, err)
		}
		artifacts, _ := db.ListNativeArtifacts(repo.Name, request.Name, true, 1000, 0)
		err = db.DeleteNativeResource(repo.Name, request.Name, user.Username)
		if err == nil && cfg != nil {
			for _, art := range artifacts {
				local := filepath.Join(cfg.StoragePath, repo.Name, filepath.FromSlash(art.Path))
				_ = os.Remove(local)
				state.Inner.FileIndex.UnblockFile(local)
				state.InvalidateFileCache(local)
			}
		}
	default:
		err = core.ErrNativeInvalid
	}
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	logNativeMutation(c, state, c.Method()+" "+repo.Name+"/"+request.Name)
	return c.JSON(fiber.Map{"ok": true})
}

func deprecateNativeResource(c fiber.Ctx, state *core.AppState) error {
	repo, db, user, err := nativeAPIContext(c, state, true)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	name := c.Query("name")
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		if utils.ReadJSONLimited(c, &body, 4096) == nil {
			name = body.Name
		}
	}
	if !core.ValidNativeResourceName(name) {
		return storage.NativeErrorResponse(c, core.ErrNativeInvalid)
	}
	release := nativepkg.AcquireMutation(repo.Name, name)
	defer release()
	target := core.ResourceLockTarget{Format: repo.Engine().Protocol, Repository: repo.Name, Name: name}
	if err := state.GetDB().EnsureResourceMutable(target, true); err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	if !user.IsManager() && !user.CheckUpdatePermission(repo.Name) {
		if err := db.CheckNativePermission(repo.Name, name, user.Username, core.NativePermissionManage); err != nil {
			return storage.NativeErrorResponse(c, core.ErrNativePermission)
		}
	}
	if err := state.GetDB().DeprecatePackage(repo.Engine().Protocol, repo.Name, name, time.Now().UnixMilli()); err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	username, operator, method, sess, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{
		Username:   username,
		Operator:   operator,
		AuthMethod: method,
		SessionID:  sess,
		IP:         ip,
		Action:     audit.ActionPackageDeprecate,
		Details:    "Format: " + repo.Engine().Protocol + ", repository: " + repo.Name + ", package: " + name,
	})
	return c.JSON(fiber.Map{"ok": true})
}

func setNativeResourceLock(c fiber.Ctx, state *core.AppState) error {
	repo, _, user, err := nativeAPIContext(c, state, false)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	session := auth.CurrentSessionToken(c)
	if user == nil || !user.CheckModeratePermission(repo.Name) || auth.CurrentCredentialKind(c) != "session" || session == "" || c.Cookies("renop_session") != session {
		return storage.NativeErrorResponse(c, core.ErrResourceLockPermission)
	}
	var request struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Mode       string `json:"mode"`
		Reason     string `json:"reason"`
		ReasonText string `json:"reason_text"`
	}
	if err := utils.ReadJSONLimited(c, &request, 4096); err != nil {
		return storage.NativeErrorResponse(c, core.ErrResourceLockInvalid)
	}
	if request.Name == "" {
		request.Name = c.Query("name")
	}
	if !core.ValidNativeResourceName(request.Name) {
		return storage.NativeErrorResponse(c, core.ErrNativeInvalid)
	}
	target := core.ResourceLockTarget{
		Format:     repo.Engine().Protocol,
		Repository: repo.Name,
		Name:       request.Name,
		Version:    request.Version,
	}
	action := audit.ActionResourceLock
	if c.Method() == fiber.MethodDelete {
		action = audit.ActionResourceUnlock
		err = state.GetDB().DeleteResourceLock(target, core.ResourceLockManual, user.Username, session)
	} else {
		err = state.GetDB().SetResourceLock(&core.ResourceLock{
			ResourceLockTarget: target,
			Source:             core.ResourceLockManual,
			Mode:               request.Mode,
			Reason:             request.Reason,
			ReasonText:         request.ReasonText,
			LockedAt:           time.Now().UnixMilli(),
		}, user.Username, session)
	}
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	username, operator, method, sess, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{
		Username:   username,
		Operator:   operator,
		AuthMethod: method,
		SessionID:  sess,
		IP:         ip,
		Action:     action,
		Details:    "Repository: " + repo.Name + ", format: " + repo.Engine().Protocol + ", name: " + request.Name + ", version: " + request.Version + ", reason: " + request.Reason,
	})
	return c.JSON(fiber.Map{"ok": true})
}

func mutateNativeMember(c fiber.Ctx, state *core.AppState) error {
	repo, db, user, err := nativeAPIContext(c, state, true)
	if err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	var request struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Level    *int   `json:"level"`
	}
	if err := utils.ReadJSONLimited(c, &request, 2048); err != nil || request.Level == nil {
		return storage.NativeErrorResponse(c, core.ErrNativeInvalid)
	}
	releaseResource := nativepkg.AcquireMutation(repo.Name, request.Name)
	defer releaseResource()
	target := core.ResourceLockTarget{Format: repo.Engine().Protocol, Repository: repo.Name, Name: request.Name}
	if err := state.GetDB().EnsureResourceMutable(target, true); err != nil {
		return storage.NativeErrorResponse(c, err)
	}
	if strings.EqualFold(user.Username, request.Username) && *request.Level >= 0 {
		return storage.NativeErrorResponse(c, core.ErrNativePermission)
	}
	if err := db.SetNativeMember(repo.Name, request.Name, user.Username, request.Username, *request.Level, time.Now().UnixMilli()); err != nil {
		if errors.Is(err, core.ErrAccountDeleted) {
			err = core.ErrNativePermission
		}
		return storage.NativeErrorResponse(c, err)
	}
	logNativeMutation(c, state, "Membership updated for "+repo.Name+"/"+request.Name)
	return c.JSON(fiber.Map{"ok": true})
}
