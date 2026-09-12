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
	"encoding/xml"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/service/auth"
	"renop/internal/service/gpg"
	"renop/internal/service/javadocs"
	"renop/internal/service/nativepkg"
	"renop/internal/service/repositorygate"
	"renop/internal/service/status"
	"renop/internal/utils"
)

func HandleDelete(c fiber.Ctx, state *core.AppState, repo *config.Repository, path string, localFilePath string) error {
	defer invalidateRepositoryCapacity(state, localFilePath)
	defer status.MarkStorageUpdated()
	if path == "" || path == "/" {
		return c.Status(fiber.StatusForbidden).SendString("Forbidden")
	}
	if repo == nil {
		return c.Status(fiber.StatusNotFound).SendString("Repository not found")
	}
	releaseMutation := repositorygate.AcquireMutation(repo.Name)
	defer releaseMutation()
	cfg := state.Inner.Config.Load()
	if cfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("Repository configuration is unavailable")
	}
	currentRepo := cfg.Maven.Repositories[repo.Name]
	if currentRepo == nil {
		return c.Status(fiber.StatusNotFound).SendString("Repository not found")
	}
	if currentRepo.NormalizedFormat() != repo.NormalizedFormat() {
		return c.Status(fiber.StatusConflict).SendString(ErrRepositoryFormatChanged.Error())
	}
	if currentRepo.Engine().ManagedNative {
		db, err := nativeDB(state)
		if err != nil {
			return NativeErrorResponse(c, err)
		}
		artifact, err := db.GetNativeArtifact(repo.Name, path)
		var releaseResource func()
		if err == nil {
			releaseResource = nativepkg.AcquireMutation(repo.Name, artifact.Name)
		} else {
			if !errors.Is(err, core.ErrNativeNotFound) {
				return NativeErrorResponse(c, err)
			}
			if name, _, ok := nativepkg.ConanIdentity(path); repo.Engine().Protocol == config.RepositoryFormatConan && ok {
				releaseResource = nativepkg.AcquireMutation(repo.Name, name)
			} else {
				releaseResource = nativepkg.AcquireAllMutations()
			}
		}
		defer releaseResource()
		if err := authorizeNativeDelete(state, auth.GetUser(c), currentRepo, path); err != nil {
			return NativeErrorResponse(c, err)
		}
	}
	if currentRepo.NormalizedFormat() == config.RepositoryFormatMaven && MavenMutationGuard != nil {
		if err := MavenMutationGuard(state, currentRepo, path); err != nil {
			return mavenMutationError(c, err)
		}
	}
	gpgReleaseStorageMutation.Lock()
	defer gpgReleaseStorageMutation.Unlock()

	isDir := false
	exists := false
	if IsS3Enabled(localFilePath) {
		isDir, _, exists, _ = state.Inner.FileIndex.GetPathState(localFilePath)
	} else {
		info, err := os.Stat(localFilePath)
		if err == nil {
			exists = true
			isDir = info.IsDir()
		}
	}

	if !exists {
		return c.Status(fiber.StatusNotFound).SendString("Not found")
	}
	isMaven := currentRepo.Engine().GPG
	if isMaven {
		if err := discardPendingGPGUploads(state, localFilePath, "Artifact or directory was deleted before publication"); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		}
	}

	if isDir {
		if IsS3Enabled(localFilePath) {
			s3Prefix := utils.GetS3Key(localFilePath)
			if !strings.HasSuffix(s3Prefix, "/") {
				s3Prefix += "/"
			}
			if err := DeletePrefixFromS3(s3Prefix); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}
		} else {
			if err := os.RemoveAll(localFilePath); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}
		}
		state.Inner.FileIndex.RemoveDir(localFilePath)
		if isMaven {
			if err := deleteGPGRecordsByLocalPrefix(state, c.Params("repo_name"), localFilePath); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}
		}
	} else {
		if artifactPath, isSignature := gpg.ArtifactForDetachedSignature(filepath.ToSlash(localFilePath)); isMaven && isSignature {
			cfg := state.Inner.Config.Load()
			var repo *config.Repository
			if cfg != nil {
				repo = cfg.Maven.Repositories[c.Params("repo_name")]
			}
			if repo != nil && repo.RequireGPGSignature && PathExistsForUpload(state, filepath.FromSlash(artifactPath)) {
				return c.Status(fiber.StatusConflict).SendString("Cannot delete a required GPG signature while its artifact exists")
			}
		}
		if isMaven && gpg.IsProtectedArtifact(filepath.ToSlash(localFilePath)) {
			if err := RemoveArtifactGPGSignature(state, localFilePath); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}
		}
		isJar := isMaven && filepath.Ext(localFilePath) == ".jar"
		dir := filepath.Dir(localFilePath)

		ext := filepath.Ext(localFilePath)
		fileStem := strings.TrimSuffix(filepath.Base(localFilePath), ext)

		if err := deleteIndexedFile(state, localFilePath); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		}

		if isJar {
			if strings.HasSuffix(localFilePath, "-javadoc.jar") {
				javadocs.CleanupJavadoc(localFilePath)
			}
			extensions := []string{
				".jar.md5", ".jar.sha1", ".jar.sha256", ".jar.sha512", ".jar.asc",
				".pom", ".pom.md5", ".pom.sha1", ".pom.sha256", ".pom.sha512", ".pom.asc",
				".module", ".module.md5", ".module.sha1", ".module.sha256", ".module.sha512", ".module.asc",
			}

			for _, ext := range extensions {
				toDelete := filepath.Join(dir, fileStem+ext)
				if state.Inner.FileIndex.HasFile(toDelete) {
					if err := deleteIndexedFile(state, toDelete); err != nil {
						return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
					}
				}
			}

			version := filepath.Base(dir)
			artifactDir := filepath.Dir(dir)
			metadataPath := filepath.Join(artifactDir, "maven-metadata.xml")

			if state.Inner.FileIndex.HasFile(metadataPath) {
				if err := state.Inner.FileIndex.UpdateMetadataCallback(func() error {
					var (
						src io.ReadCloser
						err error
					)
					if IsS3Enabled(metadataPath) {
						src, _, err = DownloadFromS3(utils.GetS3Key(metadataPath))
					} else {
						src, err = os.Open(metadataPath)
					}
					if err != nil {
						return err
					}
					var metadata config.Metadata
					decErr := xml.NewDecoder(src).Decode(&metadata)
					_ = src.Close()
					if decErr != nil {
						return decErr
					}
					var newVersions []string
					found := false
					if metadata.Versioning != nil && metadata.Versioning.Versions != nil {
						for _, v := range metadata.Versioning.Versions.Version {
							if v == version {
								found = true
							} else {
								newVersions = append(newVersions, v)
							}
						}
					}

					if !found {
						return nil
					}
					if len(newVersions) == 0 {
						if err := deleteIndexedFile(state, metadataPath); err != nil {
							return err
						}
						for _, ext := range []string{".md5", ".sha1", ".sha256", ".sha512"} {
							toDel := metadataPath + ext
							if err := deleteIndexedFile(state, toDel); err != nil {
								return err
							}
						}
						return nil
					}
					metadata.Versioning.Versions.Version = newVersions
					sortedVersions := make([]string, len(newVersions))
					copy(sortedVersions, newVersions)
					slices.SortFunc(sortedVersions, utils.CompareVersions)
					latest := sortedVersions[len(sortedVersions)-1]
					metadata.Versioning.Latest = &latest
					metadata.Versioning.Release = &latest
					lastUpdated := time.Now().UTC().Format("20060102150405")
					metadata.Versioning.LastUpdated = &lastUpdated

					updatedXML, wErr := xml.Marshal(metadata)
					if wErr != nil {
						return wErr
					}

					return writeMavenMetadata(state, metadataPath, updatedXML)
				}); err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
				}
			}
		}
	}

	if currentRepo.Engine().ManagedNative {
		db, err := nativeDB(state)
		if err != nil {
			return NativeErrorResponse(c, err)
		}
		if err := db.DeleteNativeArtifactsUnder(repo.Name, path); err != nil {
			return NativeErrorResponse(c, err)
		}
		state.Inner.FileIndex.UnblockTree(localFilePath)
	}
	username, op, authMethod, sessionID, ip := audit.ExtractAuthDetails(c, state)
	repoName := c.Params("repo_name")
	details := "Deleted artifact/directory: " + path
	if repoName != "" {
		details = "Repository: " + repoName + ", Path: " + path
	}
	audit.Log(state, &core.AuditLogEntry{
		Username:   username,
		Operator:   op,
		Action:     audit.ActionDelete,
		Details:    details,
		AuthMethod: authMethod,
		SessionID:  sessionID,
		IP:         ip,
	})

	return c.Status(fiber.StatusNoContent).SendString("")
}

func deleteFileHelper(path string) error {
	if IsS3Enabled(path) {
		s3Key := utils.GetS3Key(path)
		return DeleteFromS3(s3Key)
	}
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func deleteIndexedFile(state *core.AppState, path string) error {
	defer invalidateRepositoryCapacity(state, path)
	if err := deleteFileHelper(path); err != nil {
		return err
	}
	if state.Inner.FileIndex != nil {
		state.Inner.FileIndex.RemoveFile(path)
	}
	state.InvalidateFileCache(path)
	return deleteGPGRecordForLocalPath(state, path)
}

// RemoveRepositoryStorage deletes a repository's on-disk (and optional S3) data
// and purges the corresponding file-index / watcher entries.
// s3Cfg may be nil; when non-nil it is used even if the repo is already gone from config.
func RemoveRepositoryStorage(state *core.AppState, storagePath, repoName string, s3Cfg *config.S3Config) {
	if state == nil || state.IsDemo() || storagePath == "" || repoName == "" {
		return
	}
	if !utils.IsValidRepositoryName(repoName) {
		return
	}
	gpgReleaseStorageMutation.Lock()
	defer gpgReleaseStorageMutation.Unlock()

	repoDir := filepath.Join(storagePath, repoName)
	cleanStorage := filepath.Clean(storagePath)
	cleanRepo := filepath.Clean(repoDir)
	if cleanRepo == cleanStorage || !utils.IsSubPath(storagePath, repoDir) {
		return
	}
	_ = discardPendingGPGUploads(state, cleanRepo, "Repository was deleted before publication")

	if s3Cfg != nil && s3Cfg.Enabled {
		s3Prefix := utils.GetS3Key(repoDir)
		if !strings.HasSuffix(s3Prefix, "/") {
			s3Prefix += "/"
		}
		_ = DeletePrefixFromS3Config(s3Cfg, s3Prefix)
	}

	_ = os.RemoveAll(repoDir)

	pathNorm := filepath.ToSlash(cleanRepo)
	if state.Inner.FileIndex != nil {
		state.Inner.FileIndex.RemoveDir(pathNorm)
	}
	if db := state.GetDB(); db != nil {
		_ = db.DeleteGPGSignaturesByRepository(repoName)
	}

	state.Inner.IndexWatcherMutex.Lock()
	if state.Inner.IndexWatcher != nil {
		_ = state.Inner.IndexWatcher.Remove(pathNorm)
	}
	state.Inner.IndexWatcherMutex.Unlock()

	status.MarkStorageUpdated()
}
