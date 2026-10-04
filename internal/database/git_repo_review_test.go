/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database_test

import (
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"renop/internal/core"
)

func TestGitRepoRegistrationTask(t *testing.T) {
	db := newMavenDB(t)
	session := ticketSession(t, db, "alice")
	now := time.Now().UnixMilli()

	// Invalid input
	_, err := db.CreateGitRepoRegistrationTask("", "owner/repo", "", "alice", session, now)
	require.ErrorIs(t, err, core.ErrReviewInvalidRequest)
	_, err = db.CreateGitRepoRegistrationTask("github-mirror", "", "", "alice", session, now)
	require.ErrorIs(t, err, core.ErrReviewInvalidRequest)
	_, err = db.CreateGitRepoRegistrationTask("github-mirror", "owner/repo", "", "alice", "invalid-session", now)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)

	// Valid task creation
	task, err := db.CreateGitRepoRegistrationTask("github-mirror", "owner/repo", "platform", "alice", session, now)
	require.NoError(t, err)
	require.NotEmpty(t, task.ID)
	assert.Equal(t, core.ReviewKindGitRepoRegistration, task.Kind)
	assert.Equal(t, core.ReviewResourceGitRepo, task.ResourceType)
	assert.Equal(t, "github-mirror", task.Repository)
	assert.Equal(t, "owner/repo", task.ResourceKey)
	assert.Equal(t, "owner/repo", task.ResourceName)
	assert.Equal(t, "platform", task.TargetTeamPrefix)
	assert.Equal(t, "alice", task.RequestedBy)
	assert.Equal(t, core.ReviewStatusPending, task.Status)

	// Duplicate task should be rejected
	_, err = db.CreateGitRepoRegistrationTask("github-mirror", "owner/repo", "platform", "alice", session, now+10)
	require.ErrorIs(t, err, core.ErrReviewTaskExists)
}

func TestGitRepoQuotaTaskAndCooldown(t *testing.T) {
	db := newMavenDB(t)
	session := ticketSession(t, db, "alice")
	now := time.Now().UnixMilli()

	payload, err := json.MarshalOf(map[string]any{
		"max_version_size_bytes":      int64(500 * 1024 * 1024),
		"max_daily_traffic_bytes":     int64(50 * 1024 * 1024 * 1024),
		"retain_latest_workflow_runs": false,
		"retain_latest_releases":      true,
	})
	require.NoError(t, err)

	// Invalid input
	_, err = db.CreateGitRepoQuotaTask("", "org/repo", "", "alice", session, payload, 72, now)
	require.ErrorIs(t, core.ErrReviewInvalidRequest, err)
	_, err = db.CreateGitRepoQuotaTask("github-mirror", "", "", "alice", session, payload, 72, now)
	require.ErrorIs(t, core.ErrReviewInvalidRequest, err)
	_, err = db.CreateGitRepoQuotaTask("github-mirror", "org/repo", "", "alice", session, nil, 72, now)
	require.ErrorIs(t, core.ErrReviewInvalidRequest, err)
	_, err = db.CreateGitRepoQuotaTask("github-mirror", "org/repo", "", "alice", "bad-session", payload, 72, now)
	require.ErrorIs(t, core.ErrReviewPermissionDenied, err)

	// Check initially no request time
	lastTime, err := db.GetLastGitRepoQuotaRequestTime("github-mirror", "org/repo")
	require.NoError(t, err)
	assert.Equal(t, int64(0), lastTime)

	// Valid task creation
	task, err := db.CreateGitRepoQuotaTask("github-mirror", "org/repo", "platform", "alice", session, payload, 72, now)
	require.NoError(t, err)
	require.NotEmpty(t, task.ID)
	assert.Equal(t, core.ReviewKindGitRepoQuota, task.Kind)
	assert.Equal(t, core.ReviewResourceGitRepo, task.ResourceType)
	assert.Equal(t, "github-mirror", task.Repository)
	assert.Equal(t, "org/repo", task.ResourceKey)
	assert.Equal(t, "platform", task.TargetTeamPrefix)
	assert.Equal(t, "alice", task.RequestedBy)
	assert.Equal(t, core.ReviewStatusPending, task.Status)

	// Stored payload verification
	savedPayload, err := db.GetReviewTaskPayload(task.ID)
	require.NoError(t, err)
	assert.JSONEq(t, string(payload), string(savedPayload))

	// GetLastGitRepoQuotaRequestTime should reflect now
	lastTime, err = db.GetLastGitRepoQuotaRequestTime("github-mirror", "org/repo")
	require.NoError(t, err)
	assert.Equal(t, now, lastTime)

	// Active duplicate task while pending should be rejected with ErrReviewTaskExists
	_, err = db.CreateGitRepoQuotaTask("github-mirror", "org/repo", "platform", "alice", session, payload, 72, now+100)
	require.ErrorIs(t, core.ErrReviewTaskExists, err)

	// Claim and decide on task (approve it)
	adminSession := ticketSession(t, db, "admin")
	_, err = db.TransitionTicket(task.ID, "admin", adminSession, core.TicketAction{Action: "claim"}, now+150)
	require.NoError(t, err)
	_, err = db.DecideReviewTask(task.ID, "admin", core.ReviewStatusApproved, "Approved higher quota", now+200)
	require.NoError(t, err)

	// Payload is cleaned up on decision
	_, err = db.GetReviewTaskPayload(task.ID)
	require.ErrorIs(t, core.ErrReviewTaskNotFound, err)

	// After decision, activeKey is no longer pending, but cooldown is still in effect (within 72 hours)
	_, err = db.CreateGitRepoQuotaTask("github-mirror", "org/repo", "platform", "alice", session, payload, 72, now+(10*time.Hour).Milliseconds())
	require.ErrorIs(t, core.ErrReviewTaskConflict, err)

	// After 72 hours cooldown, task creation succeeds
	afterCooldown := now + (73 * time.Hour).Milliseconds()
	task2, err := db.CreateGitRepoQuotaTask("github-mirror", "org/repo", "platform", "alice", session, payload, 72, afterCooldown)
	require.NoError(t, err)
	require.NotEmpty(t, task2.ID)
}
