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
	"io"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"
	"renop/internal/testutil/tickettest"
)

func TestNativePublicationOwnershipReviewAndRestart(t *testing.T) {
	state, db, repo, root := setupGPGUploadState(t)
	repo.Format = config.RepositoryFormatCondaNative
	repo.RequireGPGSignature = false
	repo.PublicationReview = config.PublicationReviewEveryVersion
	for _, name := range []string{"alice", "other"} {
		require.NoError(t, db.SaveToken(&core.AccessToken{Name: name, Permissions: []string{"base", "canupdate:releases"}}))
	}
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "reviewer", Permissions: []string{"base", "canmoderate:releases"}}))
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "manager", Permissions: []string{"manager"}}))
	_, err := db.CreateNativeResource(repo.Name, repo.Format, "example", "alice", time.Now().UnixMilli())
	require.NoError(t, err)
	relative := "noarch/example-1.0-0.conda"
	target := filepath.Join(root, repo.Name, filepath.FromSlash(relative))
	archive := condaStoragePackage(t)
	_, err = ProcessUploadedFile(context.Background(), state, repo, preparedTestUpload(t, target, "other", archive))
	require.ErrorIs(t, err, core.ErrNativePermission)
	result, err := ProcessUploadedFile(context.Background(), state, repo, preparedTestUpload(t, target, "alice", archive))
	require.NoError(t, err)
	require.True(t, result.ReviewPending)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if c.Method() == "DELETE" {
			c.Locals("user", &config.User{Username: "manager", Roles: []string{"manager"}})
		}
		return c.Next()
	})
	SetupRoutes(app, state)
	get := func(path string) (int, []byte) {
		t.Helper()
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return response.StatusCode, body
	}
	code, _ := get("/releases/" + relative)
	require.Equal(t, 404, code)
	code, body := get("/releases/noarch/repodata.json")
	require.Equal(t, 200, code)
	require.NotContains(t, string(body), "example-1.0-0.conda")
	deleted, err := app.Test(httptest.NewRequest("DELETE", "/releases/noarch", nil))
	require.NoError(t, err)
	deleted.Body.Close()
	require.Equal(t, 409, deleted.StatusCode)
	state.Inner.FileIndex = index.NewFileIndex()
	require.NoError(t, RestorePublicationReviewState(state))
	require.NoError(t, index.BuildIndexSync(root, state.Inner.FileIndex))
	require.True(t, state.Inner.FileIndex.IsBlocked(target))
	require.False(t, state.Inner.FileIndex.HasFile(target))
	files, err := db.ListReviewTaskFiles(result.ReviewID)
	require.NoError(t, err)
	task, err := db.GetReviewTask(result.ReviewID)
	require.NoError(t, err)
	require.NoError(t, ValidateNativeReview(state, task, files))
	tickettest.ClaimTicket(t, db, task, "reviewer")
	_, err = db.DecideReviewTask(task.ID, "reviewer", core.ReviewStatusApproved, "", time.Now().UnixMilli()+core.PublicationReviewSettleMillis+1)
	require.NoError(t, err)
	require.NoError(t, UnblockPublicationReviewFiles(state, files))
	code, body = get("/releases/" + relative)
	require.Equal(t, 200, code)
	require.True(t, bytes.Equal(body, archive))
	code, body = get("/releases/noarch/repodata.json")
	require.Equal(t, 200, code)
	require.Contains(t, string(body), "example-1.0-0.conda")
	require.NoError(t, DeletePublicationReviewFiles(state, files))
	_, err = db.GetNativeArtifact(repo.Name, relative)
	require.ErrorIs(t, err, core.ErrNativeNotFound)
}

func TestNativePreparationRejectsNilAndUnownedIndex(t *testing.T) {
	state, _, repo, _ := setupGPGUploadState(t)
	repo.Format = config.RepositoryFormatConda
	_, err := ProcessUploadedFile(context.Background(), state, repo, nil)
	require.Error(t, err)
	_, err = ProcessUploadedFile(context.Background(), nil, repo, &PreparedUpload{})
	require.Error(t, err)
	require.ErrorIs(t, AuthorizeNativeUpload(state, &config.User{Username: "alice", Roles: []string{"manager"}}, repo, "noarch/repodata.json"), core.ErrNativeInvalid)
}
