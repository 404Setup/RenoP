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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/docker"
	"renop/internal/service/index"
	"renop/internal/service/proxy"
)

func capacityTestState(t *testing.T, format string, limit int64) (*core.AppState, string, *config.Repository) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	repo := &config.Repository{Name: "limited", Format: format, Visibility: "PUBLIC", CapacityLimitBytes: limit, AllowRedeployment: true}
	cfg.Maven.Repositories[repo.Name] = repo
	InitS3(cfg)
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	return state, filepath.Join(cfg.StoragePath, repo.Name), repo
}

func TestRepositoryCapacityUsesStoredBytesAndReclaimsDeletes(t *testing.T) {
	state, root, _ := capacityTestState(t, "files", 10)
	require.NoError(t, os.MkdirAll(root, 0755))
	old := filepath.Join(root, "old.bin")
	require.NoError(t, os.WriteFile(old, []byte("12345678"), 0644))
	store := NewPackageStore()
	write := func(path, data string) error {
		stage, err := store.Stage(path)
		if err != nil {
			return err
		}
		defer stage.Discard()
		if _, err := io.WriteString(stage, data); err != nil {
			return err
		}
		return stage.Commit(state)
	}
	require.ErrorIs(t, write(filepath.Join(root, "new.bin"), "123"), core.ErrRepositoryCapacity)
	require.NoFileExists(t, filepath.Join(root, "new.bin"))
	require.NoError(t, write(old, "123456"))
	require.NoError(t, write(filepath.Join(root, "new.bin"), "1234"))
	require.ErrorIs(t, write(filepath.Join(root, "extra.bin"), "1"), core.ErrRepositoryCapacity)
	require.NoError(t, store.Delete(state, old))
	require.NoError(t, write(filepath.Join(root, "extra.bin"), "123456"))
	// A new process reconstructs usage from storage, including objects absent from the visible index.
	restarted := core.NewAppState()
	restarted.Inner.Config.Store(state.Inner.Config.Load())
	restarted.Inner.FileIndex = index.NewFileIndex()
	stage, err := store.Stage(filepath.Join(root, "restart.bin"))
	require.NoError(t, err)
	defer stage.Discard()
	_, err = io.WriteString(stage, "1")
	require.NoError(t, err)
	require.ErrorIs(t, stage.Commit(restarted), core.ErrRepositoryCapacity)
}

func TestRepositoryCapacityIncludesChecksumBatchAndActualUploadSize(t *testing.T) {
	state, root, repo := capacityTestState(t, "maven", 272)
	require.NoError(t, os.MkdirAll(root, 0755))
	path, temporary := filepath.Join(root, "artifact.jar"), filepath.Join(root, "artifact.jar.tmp.test")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0644))
	require.NoError(t, os.WriteFile(temporary, []byte("123456789"), 0644))
	digests, _, err := HashFile(temporary)
	require.NoError(t, err)
	require.ErrorIs(t, CommitUploadedFile(state, path, temporary, 1, time.Now().UnixNano(), true, true, digests), core.ErrRepositoryCapacity)
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "old", string(original))
	require.NoFileExists(t, path+".sha256")
	repo.CapacityLimitBytes = 273
	state.Inner.RepositoryCapacity.Invalidate(repo.Name)
	require.NoError(t, CommitUploadedFile(state, path, temporary, 1, time.Now().UnixNano(), true, true, digests))
	total, err := repositoryStoredBytes(context.Background(), root, repo)
	require.NoError(t, err)
	require.EqualValues(t, 273, total)
}

func TestRepositoryCapacityIncludesDockerBlobsAndManifests(t *testing.T) {
	state, _, repo := capacityTestState(t, "docker", 8)
	store := NewDockerStore(state.Inner.Config.Load().StoragePath)
	stage, err := store.StageBlob(repo.Name, "upload")
	require.NoError(t, err)
	defer stage.Discard()
	_, err = io.WriteString(stage, "12345678")
	require.NoError(t, err)
	require.NoError(t, stage.Close())
	digest, err := stage.Digest()
	require.NoError(t, err)
	_, err = store.CommitBlob(state, repo.Name, "upload", digest)
	require.NoError(t, err)
	manifest := []byte("{}")
	require.ErrorIs(t, store.PutManifest(state, repo.Name, "example", docker.CalculateDigest(manifest), manifest), core.ErrRepositoryCapacity)
	require.NoError(t, store.DeleteBlob(state, repo.Name, digest))
	require.NoError(t, store.PutManifest(state, repo.Name, "example", docker.CalculateDigest(manifest), manifest))
}

func TestRepositoryCapacityRejectsS3WriteBeforeUpload(t *testing.T) {
	state, root, repo := capacityTestState(t, "files", 9)
	var puts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			if strings.Trim(r.URL.Path, "/") == "artifacts" {
				w.WriteHeader(200)
			} else {
				w.WriteHeader(404)
			}
			return
		}
		if r.Method == "PUT" {
			puts.Add(1)
			w.WriteHeader(200)
			return
		}
		prefix := r.URL.Query().Get("prefix")
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>artifacts</Name><Prefix>%s</Prefix><IsTruncated>false</IsTruncated><Contents><Key>%sexisting.bin</Key><Size>8</Size><LastModified>2026-09-09T00:00:00Z</LastModified></Contents></ListBucketResult>`, prefix, prefix)
	}))
	defer server.Close()
	repo.S3 = &config.S3Config{Enabled: true, Endpoint: server.URL, Bucket: "artifacts", Region: "us-east-1", ForcePathStyle: true}
	InitS3(state.Inner.Config.Load())
	stage, err := NewPackageStore().Stage(filepath.Join(root, "new.bin"))
	require.NoError(t, err)
	defer stage.Discard()
	_, err = io.WriteString(stage, "12")
	require.NoError(t, err)
	require.ErrorIs(t, stage.Commit(state), core.ErrRepositoryCapacity)
	require.Zero(t, puts.Load())
}

func TestRepositoryCapacityMirrorStillServesCompleteUncachedResponse(t *testing.T) {
	state, root, repo := capacityTestState(t, "files", 3)
	path := filepath.Join(root, "mirror.bin")
	stream := proxy.CreateProxyStream(io.NopCloser(strings.NewReader("1234")), 4, path,
		nil, "mirror", nil, nil, nil, state.Inner.FileIndex, nil, 4, nil,
		func(size int64) (func(bool), error) { return proxy.ReserveMirrorCapacity(state, repo, path, size) })
	data, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Equal(t, "1234", string(data))
	require.NoError(t, stream.Close())
	require.NoFileExists(t, path)
	files, err := filepath.Glob(path + ".tmp.*")
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestRepositoryCapacityHTTPDenialUsesStableError(t *testing.T) {
	app, _, _, repo := setupSnapshotPutApp(t)
	repo.Format, repo.CapacityLimitBytes = "files", 3
	request := httptest.NewRequest(http.MethodPut, "/snapshots/artifact.bin", strings.NewReader("1234"))
	response, err := app.Test(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, fiber.StatusInsufficientStorage, response.StatusCode)
	require.Equal(t, "repository_capacity_exceeded", response.Header.Get("X-Renop-Error-Code"))
}
