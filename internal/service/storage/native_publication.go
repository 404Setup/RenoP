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
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/conan"
	"renop/internal/service/index"
	"renop/internal/service/nativepkg"
	"renop/internal/service/nativesign"
	"renop/internal/service/publicationquota"
	"renop/internal/service/ticketnotify"
	"renop/pkg/hex"

	"github.com/gofiber/fiber/v3"
)

func nativeDB(state *core.AppState) (core.NativePackageDB, error) {
	if state == nil || state.GetDB() == nil {
		return nil, core.ErrDatabaseUnavailable
	}
	db, ok := state.GetDB().(core.NativePackageDB)
	if !ok {
		return nil, core.ErrDatabaseUnavailable
	}
	return db, nil
}

func NativeErrorResponse(c fiber.Ctx, err error) error {
	status, code := fiber.StatusInternalServerError, "native_failed"
	switch {
	case errors.Is(err, core.ErrNativeInvalid):
		status, code = 400, "native_invalid"
	case errors.Is(err, core.ErrNativeSignature):
		status, code = 400, "native_signature_required"
	case errors.Is(err, core.ErrNativePermission):
		status, code = 403, "native_permission"
	case errors.Is(err, core.ErrDemoReadOnly):
		status, code = 403, "demo_read_only"
	case errors.Is(err, core.ErrNativeNotFound):
		status, code = 404, "native_not_found"
	case errors.Is(err, core.ErrNativeExists):
		status, code = 409, "native_exists"
	case errors.Is(err, core.ErrNativeLastOwner):
		status, code = 409, "native_last_owner"
	case errors.Is(err, core.ErrNativeBusy):
		status, code = 409, "native_busy"
	case errors.Is(err, core.ErrPackageDeprecated):
		status, code = 409, "package_deprecated"
	case errors.Is(err, core.ErrResourceLocked):
		status, code = 423, "resource_locked"
	case errors.Is(err, core.ErrResourceLockInvalid):
		status, code = 400, "resource_lock_invalid"
	case errors.Is(err, core.ErrResourceLockPermission):
		status, code = 403, "resource_lock_permission"
	case errors.Is(err, core.ErrDatabaseUnavailable):
		status, code = 503, "service_unavailable"
	case errors.Is(err, core.ErrRepositoryCapacity), errors.Is(err, core.ErrPublicationFileLimit), errors.Is(err, core.ErrPublicationByteLimit), errors.Is(err, core.ErrPublicationCountLimit):
		status, _ = GPGUploadErrorResponse(err)
		code = publicationquota.ErrorCode(err)
	case errors.Is(err, core.ErrReviewPermissionDenied), errors.Is(err, core.ErrReviewPublicationSealed), errors.Is(err, core.ErrReviewInvalidRequest), errors.Is(err, core.ErrReviewFileLimit):
		status, _ = PublicationReviewErrorResponse(err)
		code = "native_review_failed"
	}
	c.Set("X-Renop-Error-Code", code)
	return c.Status(status).SendString(code)
}

// AuthorizeNativeUpload is shared by direct and chunked upload admission. The
// exact resource and live L1/L2 authority are checked again after archive parsing.
func AuthorizeNativeUpload(state *core.AppState, user *config.User, repo *config.Repository, relative string) error {
	if user == nil || user.Username == "" || user.Username == "guest" {
		return core.ErrNativePermission
	}
	if !nativepkg.ValidUploadPath(repo.Engine().Protocol, relative) {
		return core.ErrNativeInvalid
	}
	db, err := nativeDB(state)
	if err != nil {
		return err
	}
	if user.IsManager() || user.CheckUpdatePermission(repo.Name) {
		return nil
	}
	allowed, err := db.HasNativePublishAccess(repo.Name, user.Username)
	if err != nil {
		return err
	}
	if !allowed {
		return core.ErrNativePermission
	}
	return nil
}

func managedNativeEngine() repositoryEngine {
	engine := filesEngine()
	engine.authorizeWrite = func(c fiber.Ctx, state *core.AppState, user *config.User, repo *config.Repository, relative string) (bool, error) {
		if c.Method() == fiber.MethodDelete {
			if err := authorizeNativeDelete(state, user, repo, relative); err != nil {
				return true, NativeErrorResponse(c, err)
			}
			return false, nil
		}
		if err := AuthorizeNativeUpload(state, user, repo, relative); err != nil {
			return true, NativeErrorResponse(c, err)
		}
		return false, nil
	}
	engine.read = func(state *core.AppState, user *config.User, repo *config.Repository, relative string, root bool) (bool, error) {
		allowed, err := readRepositoryFiles(state, user, repo, relative, root)
		if err != nil || !allowed {
			return allowed, err
		}
		// The durable binding is authoritative even if a watcher or cache has
		// observed bytes before a review or publication transaction completed.
		if db, err := nativeDB(state); err == nil {
			artifact, err := db.GetNativeArtifact(repo.Name, relative)
			if errors.Is(err, core.ErrNativeNotFound) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return artifact.Published, nil
		}
		return true, nil
	}
	return engine
}

// CanReplaceUpload allows a retry of the same hidden native publication without
// permitting replacement of a published immutable artifact.
func CanReplaceUpload(state *core.AppState, user *config.User, repo *config.Repository, localPath string) (bool, error) {
	if CanReplaceArtifact(repo, localPath) {
		return true, nil
	}
	if !repo.Engine().ManagedNative || user == nil {
		return false, nil
	}
	db, err := nativeDB(state)
	if err != nil {
		return false, err
	}
	relative, err := publicationQuotaRelativePath(state, repo, localPath)
	if err != nil {
		return false, err
	}
	artifact, err := db.GetNativeArtifact(repo.Name, relative)
	if errors.Is(err, core.ErrNativeNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if artifact.Published {
		if repo.Engine().Protocol != config.RepositoryFormatConan {
			return false, nil
		}
		if err := db.CheckNativePermission(repo.Name, artifact.Name, user.Username, core.NativePermissionPublish); err != nil {
			return false, err
		}
		return true, nil // Only byte-identical Conan retries are accepted below.
	}
	if err := db.CheckNativePermission(repo.Name, artifact.Name, user.Username, core.NativePermissionPublish); err != nil {
		return false, err
	}
	review, err := db.NativePublicationState(repo.Name, artifact.Name, artifact.Version)
	if err != nil {
		return false, err
	}
	return review == nil || review.Status == core.ReviewStatusPending && strings.EqualFold(review.RequestedBy, user.Username), nil
}

func authorizeNativeDelete(state *core.AppState, user *config.User, repo *config.Repository, relative string) error {
	if user == nil || user.Username == "" || user.Username == "guest" {
		return core.ErrNativePermission
	}
	db, err := nativeDB(state)
	if err != nil {
		return err
	}
	artifact, err := db.GetNativeArtifact(repo.Name, relative)
	if errors.Is(err, core.ErrNativeNotFound) {
		if user.IsManager() {
			if pending, err := db.NativePathHasPendingReview(repo.Name, relative); err != nil {
				return err
			} else if pending {
				return core.ErrNativeBusy
			}
			return nil
		}
		if repo.Engine().Protocol == config.RepositoryFormatConan {
			parsed, valid := conan.Parse(relative)
			name, _, named := nativepkg.ConanIdentity(relative)
			if valid && named && parsed.Operation == "root" {
				if err := db.CheckNativePermission(repo.Name, name, user.Username, core.NativePermissionVersion); err != nil {
					return err
				}
				reviews, err := state.GetDB().ListPublicationReviews(repo.Name, core.ReviewResourceNativePackage, name)
				if err != nil {
					return err
				}
				for _, review := range reviews {
					if review.Status == core.ReviewStatusPending {
						return core.ErrNativeBusy
					}
				}
				return nil
			}
		}
	}
	if err != nil {
		return err
	}
	if pending, err := state.GetDB().IsPublicationReviewPathPending(repo.Name, relative); err != nil {
		return err
	} else if pending {
		return core.ErrNativeBusy
	}
	return db.CheckNativePermission(repo.Name, artifact.Name, user.Username, core.NativePermissionVersion)
}

func processNativePublication(ctx context.Context, state *core.AppState, repo *config.Repository, upload *PreparedUpload) (GPGUploadResult, error) {
	if upload == nil || upload.TempPath == "" || state == nil || state.Inner == nil {
		return GPGUploadResult{}, core.ErrNativeInvalid
	}
	if state.IsDemo() {
		return GPGUploadResult{}, core.ErrDemoReadOnly
	}
	if err := ctx.Err(); err != nil {
		return GPGUploadResult{}, err
	}
	cfg := state.Inner.Config.Load()
	if cfg == nil {
		return GPGUploadResult{}, core.ErrDatabaseUnavailable
	}
	current := cfg.Maven.Repositories[repo.Name]
	if current == nil {
		return GPGUploadResult{}, ErrGPGRepositoryMissing
	}
	if current.NormalizedFormat() != repo.NormalizedFormat() {
		return GPGUploadResult{}, ErrRepositoryFormatChanged
	}
	repo = current
	db, err := nativeDB(state)
	if err != nil {
		return GPGUploadResult{}, err
	}
	relative, err := publicationQuotaRelativePath(state, repo, upload.LocalFilePath)
	if err != nil {
		return GPGUploadResult{}, err
	}
	file, err := os.Open(upload.TempPath)
	if err != nil {
		return GPGUploadResult{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return GPGUploadResult{}, err
	}
	artifact, record, err := nativepkg.Inspect(repo.Engine().Protocol, relative, file, info.Size())
	if err != nil {
		file.Close()
		return GPGUploadResult{}, err
	}
	releaseResource := nativepkg.AcquireMutation(repo.Name, artifact.Name)
	defer releaseResource()
	resource, err := db.GetNativeResource(repo.Name, artifact.Name, upload.Username)
	if err != nil {
		file.Close()
		return GPGUploadResult{}, err
	}
	if resource.Format != repo.Engine().Protocol {
		file.Close()
		return GPGUploadResult{}, core.ErrNativeInvalid
	}
	if resource.Archived || resource.Deprecated {
		file.Close()
		return GPGUploadResult{}, core.ErrPackageDeprecated
	}
	target := core.ResourceLockTarget{Format: repo.Engine().Protocol, Repository: repo.Name, Name: artifact.Name, Version: artifact.Version}
	if err := state.GetDB().EnsureResourceMutable(target, false); err != nil {
		file.Close()
		return GPGUploadResult{}, err
	}
	level := core.NativePermissionPublish
	previous, previousErr := db.GetNativeArtifact(repo.Name, relative)
	if previousErr != nil && !errors.Is(previousErr, core.ErrNativeNotFound) {
		file.Close()
		return GPGUploadResult{}, previousErr
	}
	upload.Existed = PathExistsForUpload(state, upload.LocalFilePath)
	if upload.Existed && !CanReplaceArtifact(repo, upload.LocalFilePath) && (previous == nil || previous.Published) && repo.Engine().Protocol != config.RepositoryFormatConan {
		file.Close()
		return GPGUploadResult{}, ErrRedeploymentDenied
	}
	if upload.Existed && (previous == nil || previous.Published) && repo.Engine().Protocol != config.RepositoryFormatConan {
		level = core.NativePermissionVersion
	}
	if err := db.CheckNativePermission(repo.Name, artifact.Name, upload.Username, level); err != nil {
		file.Close()
		return GPGUploadResult{}, err
	}
	if previous != nil && previous.Published && repo.Engine().Protocol == config.RepositoryFormatConan {
		same, err := sameNativeContent(upload.LocalFilePath, file, info.Size())
		file.Close()
		if err != nil {
			return GPGUploadResult{}, err
		}
		if !same {
			return GPGUploadResult{}, ErrRedeploymentDenied
		}
		if err := os.Remove(upload.TempPath); err != nil {
			return GPGUploadResult{}, err
		}
		upload.TempPath = ""
		return GPGUploadResult{}, nil
	}
	if err := verifyNativePublicationSignature(state, repo.Engine().Protocol, file, info.Size(), resource.SigningKey, upload.Username); err != nil {
		file.Close()
		return GPGUploadResult{}, err
	}
	file.Close()
	existingReview, err := db.NativePublicationState(repo.Name, artifact.Name, artifact.Version)
	if err != nil {
		return GPGUploadResult{}, err
	}
	if existingReview != nil {
		if existingReview.Status == core.ReviewStatusApproved {
			return GPGUploadResult{}, core.ErrReviewPublicationSealed
		}
		if !strings.EqualFold(existingReview.RequestedBy, upload.Username) {
			return GPGUploadResult{}, core.ErrReviewPermissionDenied
		}
	}
	if repo.Engine().Protocol == config.RepositoryFormatAPT || repo.Engine().Protocol == config.RepositoryFormatAPK || repo.Engine().Protocol == config.RepositoryFormatRPM {
		if _, err := nativesign.PublicKey(cfg.NativeSigningKeys, repo.Engine().Protocol == config.RepositoryFormatAPK); err != nil {
			return GPGUploadResult{}, core.ErrNativeSignature
		}
	}
	policy := repo.PublicationReviewPolicy()
	needsReview := policy == config.PublicationReviewEveryVersion || policy == config.PublicationReviewNewPackages && resource.PublishedAt == 0
	if upload.Existed && needsReview && (previous == nil || previous.Published) {
		return GPGUploadResult{}, core.ErrReviewPublicationSealed
	}
	if previous != nil && previous.Name != artifact.Name {
		return GPGUploadResult{}, core.ErrNativePermission
	}
	if previous == nil && upload.Existed {
		return GPGUploadResult{}, core.ErrNativeBusy
	}
	artifact.Repository, artifact.ResourceID, artifact.CreatedAt = repo.Name, resource.ID, time.Now().UnixMilli()
	upload.FileSize, upload.ModTime = info.Size(), info.ModTime().UnixNano()
	quota, err := reserveUploadedFileQuota(state, repo, upload)
	if err != nil {
		return GPGUploadResult{}, err
	}
	defer quota.Release()
	if err := quota.Commit(); err != nil {
		return GPGUploadResult{}, err
	}
	if err := db.SaveNativeArtifact(*artifact, upload.Username, false); err != nil {
		return GPGUploadResult{}, err
	}
	state.Inner.FileIndex.BlockFile(upload.LocalFilePath)
	state.InvalidateFileCache(upload.LocalFilePath)
	if err := commitUploadedFile(state, upload.LocalFilePath, upload.TempPath, upload.FileSize, upload.ModTime, upload.Existed, false, nil, record); err != nil {
		var rollbackErr error
		if previous != nil {
			rollbackErr = db.SaveNativeArtifact(*previous, upload.Username, previous.Published)
		} else {
			rollbackErr = db.DeleteNativeArtifact(repo.Name, relative, "")
		}
		if rollbackErr == nil && (previous == nil || previous.Published) {
			state.Inner.FileIndex.UnblockFile(upload.LocalFilePath)
			reindexPathIfPresent(state, upload.LocalFilePath)
		}
		return GPGUploadResult{}, errors.Join(err, rollbackErr)
	}
	artifacts := []*core.NativeArtifact{artifact}
	if repo.Engine().Protocol == config.RepositoryFormatConan {
		artifacts, err = db.NativeVersionArtifacts(repo.Name, artifact.Name, artifact.Version)
		if err != nil {
			return GPGUploadResult{}, err
		}
		complete, err := verifyConanFiles(state, repo.Name, resource.SigningKey, upload.Username, artifacts)
		if err != nil {
			return GPGUploadResult{}, err
		}
		if !complete {
			return GPGUploadResult{Pending: true, NativePending: true}, nil
		}
	}
	files := make([]*core.ReviewFile, 0, len(artifacts))
	paths := make([]string, 0, len(artifacts))
	for _, item := range artifacts {
		files = append(files, &core.ReviewFile{Path: item.Path, Size: item.Size, AddedAt: item.CreatedAt, Critical: true})
		paths = append(paths, item.Path)
	}
	result, err := state.GetDB().CreateOrUpdatePublicationReview(core.PublicationReviewRequest{
		ResourceType: core.ReviewResourceNativePackage, Repository: repo.Name, ResourceKey: artifact.Name, ResourceName: artifact.Name,
		Version: artifact.Version, RequestedBy: upload.Username, Policy: policy, PackageExists: resource.PublishedAt > 0, CreatedAt: artifact.CreatedAt,
		Files: files,
	})
	if err != nil {
		return GPGUploadResult{}, err
	}
	if result.Pending {
		ticketnotify.DeliverPending(state, result)
		return GPGUploadResult{Pending: true, ReviewPending: true, ReviewID: result.TaskID}, nil
	}
	if err := db.PublishNativeArtifacts(repo.Name, artifact.Name, artifact.Version, upload.Username, paths, artifact.CreatedAt); err != nil {
		return GPGUploadResult{}, err
	}
	for _, item := range artifacts {
		local := filepath.Join(cfg.StoragePath, repo.Name, filepath.FromSlash(item.Path))
		state.Inner.FileIndex.UnblockFile(local)
		reindexPathIfPresent(state, local)
	}
	if record != nil {
		if info, ok := state.Inner.FileIndex.GetFileInfo(upload.LocalFilePath); ok {
			_ = state.Inner.NativeIndexCache.Set(nativeRecordKey(state, repo, nativeArtifact{Path: upload.LocalFilePath, Info: info}), record)
		}
	}
	return GPGUploadResult{}, nil
}

func verifyConanFiles(state *core.AppState, repository, publicKey, username string, artifacts []*core.NativeArtifact) (bool, error) {
	cfg := state.Inner.Config.Load()
	open := func(relative string) (io.ReadCloser, int64, error) {
		local := filepath.Join(cfg.StoragePath, repository, filepath.FromSlash(relative))
		reader, size, found, err := backendFor(local).Open(local)
		if err != nil {
			return nil, 0, err
		}
		if !found {
			return nil, 0, core.ErrNativeNotFound
		}
		return reader, size, nil
	}
	readPayload := func(relative string, limit int64) ([]byte, error) {
		reader, size, err := open(relative)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		if size > limit {
			return nil, core.ErrNativeInvalid
		}
		data, err := io.ReadAll(io.LimitReader(reader, limit+1))
		if int64(len(data)) > limit {
			return nil, core.ErrNativeInvalid
		}
		return data, err
	}
	hashPayload := func(relative string) (string, error) {
		reader, size, err := open(relative)
		if err != nil {
			return "", err
		}
		defer reader.Close()
		hash := sha256.New()
		n, err := io.Copy(hash, io.LimitReader(reader, nativepkg.MaxArtifactBytes+1))
		if err != nil {
			return "", err
		}
		if n != size || n > nativepkg.MaxArtifactBytes {
			return "", core.ErrNativeInvalid
		}
		return hex.EncodeToString(hash.Sum(nil)), nil
	}
	if publicKey != "" {
		complete, err := nativepkg.VerifyConanPublication(publicKey, artifacts, readPayload, hashPayload)
		if err == nil {
			return complete, nil
		}
	}
	if state != nil && state.GetDB() != nil {
		keys, err := state.GetDB().ListUserGPGKeys(username)
		if err == nil {
			for _, k := range keys {
				complete, err := nativepkg.VerifyConanPublication(string(k.PublicKey), artifacts, readPayload, hashPayload)
				if err == nil {
					return complete, nil
				}
			}
		}
	}
	if publicKey != "" {
		return false, core.ErrNativeSignature
	}
	return nativepkg.VerifyConanPublication("", artifacts, readPayload, hashPayload)
}

func verifyNativePublicationSignature(state *core.AppState, protocol string, reader io.ReaderAt, size int64, resourceKey, username string) error {
	if resourceKey != "" {
		if err := nativepkg.VerifySignature(protocol, reader, size, resourceKey); err == nil {
			return nil
		}
	}
	if state != nil && state.GetDB() != nil && (protocol == config.RepositoryFormatRPM || protocol == config.RepositoryFormatConan) {
		keys, err := state.GetDB().ListUserGPGKeys(username)
		if err == nil {
			for _, k := range keys {
				if err := nativepkg.VerifySignature(protocol, reader, size, string(k.PublicKey)); err == nil {
					return nil
				}
			}
		}
	}
	if resourceKey == "" && protocol != config.RepositoryFormatRPM && protocol != config.RepositoryFormatConan && protocol != config.RepositoryFormatAPK {
		return nil
	}
	return core.ErrNativeSignature
}

func sameNativeContent(local string, staged io.ReaderAt, size int64) (bool, error) {
	reader, storedSize, exists, err := backendFor(local).Open(local)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	defer reader.Close()
	if storedSize != size {
		return false, nil
	}
	first, second := sha256.New(), sha256.New()
	if _, err := io.Copy(first, reader); err != nil {
		return false, err
	}
	if _, err := io.Copy(second, io.NewSectionReader(staged, 0, size)); err != nil {
		return false, err
	}
	return bytes.Equal(first.Sum(nil), second.Sum(nil)), nil
}

// ValidateNativeReview rechecks current trust keys and bytes before approval.
func ValidateNativeReview(state *core.AppState, task *core.ReviewTask, files []*core.ReviewFile) error {
	db, err := nativeDB(state)
	if err != nil {
		return err
	}
	resource, err := db.GetNativeResource(task.Repository, task.ResourceKey, task.RequestedBy)
	if err != nil {
		return err
	}
	cfg := state.Inner.Config.Load()
	repo := cfg.Maven.Repositories[task.Repository]
	if repo == nil || !repo.Engine().ManagedNative || repo.Engine().Protocol != resource.Format {
		return core.ErrReviewResourceConflict
	}
	if resource.Archived || resource.Deprecated {
		return core.ErrPackageDeprecated
	}
	target := core.ResourceLockTarget{Format: resource.Format, Repository: task.Repository, Name: task.ResourceKey, Version: task.ResourceVersion}
	if err := state.GetDB().EnsureResourceMutable(target, false); err != nil {
		return err
	}
	for _, file := range files {
		local, err := reviewFileLocalPath(state, file)
		if err != nil {
			return err
		}
		if _, err := readNativeArtifact(nativeArtifact{Path: local, Info: index.FileInfo{Size: file.Size}}, func(reader io.ReaderAt, size int64) ([]byte, error) {
			artifact, _, err := nativepkg.Inspect(resource.Format, file.Path, reader, size)
			if err != nil {
				return nil, err
			}
			if artifact.Name != task.ResourceKey || artifact.Version != task.ResourceVersion {
				return nil, core.ErrReviewResourceConflict
			}
			return nil, verifyNativePublicationSignature(state, resource.Format, reader, size, resource.SigningKey, task.RequestedBy)
		}); err != nil {
			return err
		}
	}
	if resource.Format == config.RepositoryFormatConan {
		artifacts, err := db.NativeVersionArtifacts(task.Repository, task.ResourceKey, task.ResourceVersion)
		if err != nil {
			return err
		}
		complete, err := verifyConanFiles(state, task.Repository, resource.SigningKey, task.RequestedBy, artifacts)
		if err != nil {
			return err
		}
		if !complete {
			return core.ErrNativeSignature
		}
	}
	return nil
}

func restoreNativePublications(state *core.AppState) error {
	db, err := nativeDB(state)
	if err != nil {
		return err
	}
	artifacts, err := db.ListHiddenNativeArtifacts()
	if err != nil {
		return err
	}
	cfg := state.Inner.Config.Load()
	for _, artifact := range artifacts {
		if cfg.Maven.Repositories[artifact.Repository] == nil {
			continue
		}
		local := filepath.Join(cfg.StoragePath, artifact.Repository, filepath.FromSlash(artifact.Path))
		state.Inner.FileIndex.BlockFile(local)
	}
	return nil
}
