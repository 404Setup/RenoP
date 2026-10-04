/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"renop/internal/core"
	"renop/pkg/hex"
	"renop/pkg/id"
)

// CreateGitRepoRegistrationTask creates a pending review task for registering a git repository.
func (db *DB) CreateGitRepoRegistrationTask(repository, targetRepo, superTeamPrefix, actor, session string, createdAt int64) (*core.ReviewTask, error) {
	repository = strings.ToLower(SanitizeInputString(strings.TrimSpace(repository), 255))
	targetRepo = strings.TrimSpace(targetRepo)
	actor = strings.ToLower(SanitizeInputString(strings.TrimSpace(actor), maxTokenNameLen))
	if repository == "" || targetRepo == "" || actor == "" || createdAt <= 0 {
		return nil, core.ErrReviewInvalidRequest
	}

	reviewTaskMutationLock.Lock()
	defer reviewTaskMutationLock.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin git repo registration review: %w", err)
	}
	defer tx.Rollback()

	account, err := accountEmailSessionTx(tx, actor, session)
	if err != nil {
		return nil, core.ErrReviewPermissionDenied
	}

	digest := sha256.Sum256([]byte(core.ReviewKindGitRepoRegistration + "\x00" + repository + "\x00" + strings.ToLower(targetRepo)))
	activeKey := hex.EncodeToString(digest[:])

	var existing string
	if err := tx.QueryRow(`SELECT id FROM review_tasks WHERE active_key = ? AND status = ?`, activeKey, core.ReviewStatusPending).Scan(&existing); err == nil {
		return nil, core.ErrReviewTaskExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	task := &core.ReviewTask{
		ID:               id.New(),
		Kind:             core.ReviewKindGitRepoRegistration,
		ResourceType:     core.ReviewResourceGitRepo,
		Repository:       repository,
		ResourceKey:      targetRepo,
		ResourceName:     targetRepo,
		TargetTeamPrefix: superTeamPrefix,
		RequestedByID:    account.UserID,
		RequestedBy:      actor,
		Status:           core.ReviewStatusPending,
		CreatedAt:        createdAt,
		ActiveKey:        activeKey,
	}

	if _, err := tx.Exec(`INSERT INTO review_tasks
		(id, kind, resource_type, repository, resource_key, resource_name, source_team_prefix,
		target_team_prefix, review_team_prefix, requested_by_id, requested_by_name, status,
		decision_reason, decided_by_id, decided_by_name, created_at, decided_at, active_key)
		VALUES (?, ?, ?, ?, ?, ?, '', ?, '', ?, ?, ?, '', '', '', ?, 0, ?)`,
		task.ID, task.Kind, task.ResourceType, task.Repository, task.ResourceKey, task.ResourceName,
		task.TargetTeamPrefix, task.RequestedByID, task.RequestedBy, task.Status, task.CreatedAt, activeKey); err != nil {
		return nil, fmt.Errorf("insert git repo registration review: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit git repo registration review: %w", err)
	}
	return task, nil
}

// CreateGitRepoQuotaTask creates a pending review task for requesting git repo quota or retention modifications.
func (db *DB) CreateGitRepoQuotaTask(repository, targetRepo, superTeamPrefix, actor, session string, payload []byte, cooldownHours int, createdAt int64) (*core.ReviewTask, error) {
	repository = strings.ToLower(SanitizeInputString(strings.TrimSpace(repository), 255))
	targetRepo = strings.ToLower(SanitizeInputString(strings.TrimSpace(targetRepo), 255))
	actor = strings.ToLower(SanitizeInputString(strings.TrimSpace(actor), maxTokenNameLen))
	if repository == "" || targetRepo == "" || actor == "" || createdAt <= 0 || len(payload) == 0 {
		return nil, core.ErrReviewInvalidRequest
	}

	reviewTaskMutationLock.Lock()
	defer reviewTaskMutationLock.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin git repo quota review: %w", err)
	}
	defer tx.Rollback()

	account, err := accountEmailSessionTx(tx, actor, session)
	if err != nil {
		return nil, core.ErrReviewPermissionDenied
	}

	digest := sha256.Sum256([]byte(core.ReviewKindGitRepoQuota + "\x00" + repository + "\x00" + targetRepo))
	activeKey := hex.EncodeToString(digest[:])

	var existing string
	if err := tx.QueryRow(`SELECT id FROM review_tasks WHERE active_key = ? AND status = ?`, activeKey, core.ReviewStatusPending).Scan(&existing); err == nil {
		return nil, core.ErrReviewTaskExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if cooldownHours > 0 {
		var lastCreatedAt int64
		err := tx.QueryRow(`SELECT created_at FROM review_tasks WHERE kind = ? AND repository = ? AND resource_key = ? ORDER BY created_at DESC LIMIT 1`,
			core.ReviewKindGitRepoQuota, repository, targetRepo).Scan(&lastCreatedAt)
		if err == nil {
			cooldownMillis := int64(cooldownHours) * 3600 * 1000
			if createdAt-lastCreatedAt < cooldownMillis {
				return nil, core.ErrReviewTaskConflict
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	task := &core.ReviewTask{
		ID:               id.New(),
		Kind:             core.ReviewKindGitRepoQuota,
		ResourceType:     core.ReviewResourceGitRepo,
		Repository:       repository,
		ResourceKey:      targetRepo,
		ResourceName:     targetRepo,
		TargetTeamPrefix: superTeamPrefix,
		RequestedByID:    account.UserID,
		RequestedBy:      actor,
		Status:           core.ReviewStatusPending,
		CreatedAt:        createdAt,
		ActiveKey:        activeKey,
	}

	if _, err := tx.Exec(`INSERT INTO review_tasks
		(id, kind, resource_type, repository, resource_key, resource_name, source_team_prefix,
		target_team_prefix, review_team_prefix, requested_by_id, requested_by_name, status,
		decision_reason, decided_by_id, decided_by_name, created_at, decided_at, active_key)
		VALUES (?, ?, ?, ?, ?, ?, '', ?, '', ?, ?, ?, '', '', '', ?, 0, ?)`,
		task.ID, task.Kind, task.ResourceType, task.Repository, task.ResourceKey, task.ResourceName,
		task.TargetTeamPrefix, task.RequestedByID, task.RequestedBy, task.Status, task.CreatedAt, activeKey); err != nil {
		return nil, fmt.Errorf("insert git repo quota review: %w", err)
	}

	if err := savePublicationReviewPayloadTx(tx, task.ID, payload); err != nil {
		return nil, fmt.Errorf("save git repo quota review payload: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit git repo quota review: %w", err)
	}
	return task, nil
}

// GetLastGitRepoQuotaRequestTime returns the timestamp of the latest quota application for a repository.
func (db *DB) GetLastGitRepoQuotaRequestTime(repository, targetRepo string) (int64, error) {
	repository = strings.ToLower(SanitizeInputString(strings.TrimSpace(repository), 255))
	targetRepo = strings.ToLower(SanitizeInputString(strings.TrimSpace(targetRepo), 255))
	var lastCreatedAt int64
	err := db.QueryRow(`SELECT created_at FROM review_tasks WHERE kind = ? AND repository = ? AND resource_key = ? ORDER BY created_at DESC LIMIT 1`,
		core.ReviewKindGitRepoQuota, repository, targetRepo).Scan(&lastCreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query last git repo quota request: %w", err)
	}
	return lastCreatedAt, nil
}
