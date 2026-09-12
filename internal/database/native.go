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
	"slices"
	"strings"
	"sync"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/utils"
	"renop/pkg/hex"
)

var nativeResourceMutationLock sync.Mutex

func (db *DB) NativePathHasPendingReview(repository, prefix string) (bool, error) {
	var found int
	err := db.QueryRow(`SELECT 1 FROM review_task_files f JOIN review_tasks r ON r.id = f.task_id
		WHERE r.repository = ? AND r.kind = ? AND r.resource_type = ? AND r.status = ?
		AND (f.path = ? OR f.path LIKE ? ESCAPE '!') LIMIT 1`, repository, core.ReviewKindPublication,
		core.ReviewResourceNativePackage, core.ReviewStatusPending, prefix, escapeLikePrefix(strings.TrimSuffix(prefix, "/")+"/")+"%").Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return found != 0, err
}

func (db *DB) NativePublicationState(repository, name, version string) (*core.ReviewTask, error) {
	task, err := scanReviewTask(db.QueryRow(`SELECT `+reviewTaskSelectColumns+` FROM review_tasks r`+reviewTaskProfileJoins+`
		WHERE r.kind = ? AND r.resource_type = ? AND r.repository = ? AND r.resource_key = ?
		AND (r.status = ? OR r.status = ?) ORDER BY r.created_at DESC, r.id DESC LIMIT 1`,
		core.ReviewKindPublication, core.ReviewResourceNativePackage, repository, publicationReviewKey(name, version), core.ReviewStatusPending, core.ReviewStatusApproved))
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, core.ErrReviewTaskNotFound) {
		return nil, nil
	}
	return task, err
}

func (db *DB) HasNativePublishAccess(repository, actor string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM native_members m JOIN native_resources r ON r.id = m.resource_id
		JOIN user_profiles p ON p.user_id = m.user_id WHERE r.repository = ? AND p.username = ? AND m.permission_level >= ?`, repository, strings.ToLower(actor), core.NativePermissionPublish).Scan(&count)
	return count > 0, err
}

func nativeID(repository, key string) string {
	digest := sha256.Sum256([]byte(repository + "\x00" + key))
	return hex.EncodeToString(digest[:])
}

func nativeKey(repository, name string) (string, error) {
	if !utils.IsValidRepositoryName(repository) || !core.ValidNativeResourceName(name) {
		return "", core.ErrNativeInvalid
	}
	return nativeID(repository, name), nil
}

func nativeActorTx(tx *Tx, actor string) (string, *config.User, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	if actor == "" || actor == "guest" {
		return "", nil, core.ErrNativePermission
	}
	if err := lockAccountByUsernameTx(tx, actor); err != nil {
		return "", nil, err
	}
	token, err := tokenByNameTx(tx, actor)
	if err != nil {
		return "", nil, err
	}
	if token == nil || token.DeletedAt != 0 || token.Ban.IsActive(time.Now().UnixMilli()) ||
		token.ExpiresAt != nil && *token.ExpiresAt <= time.Now().UnixMilli() {
		return "", nil, core.ErrNativePermission
	}
	id, err := userIDForUsernameTx(tx, actor)
	if err != nil {
		return "", nil, core.ErrNativePermission
	}
	roles := append([]string(nil), token.Permissions...)
	for _, role := range roles {
		if role == "m" || role == "access-token:manager" {
			roles = append(roles, "manager")
			break
		}
	}
	return id, &config.User{Username: actor, Roles: roles}, nil
}

func nativePermissionTx(tx *Tx, repository, name, actor string, required int) (string, int, error) {
	id, err := nativeKey(repository, name)
	if err != nil {
		return "", -1, err
	}
	userID, user, err := nativeActorTx(tx, actor)
	if err != nil {
		return "", -1, err
	}
	if _, err := tx.Exec(`UPDATE native_resources SET created_at = created_at WHERE id = ?`, id); err != nil {
		return "", -1, err
	}
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM native_resources WHERE id = ?`, id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return "", -1, core.ErrNativeNotFound
	} else if err != nil {
		return "", -1, err
	}
	if user.IsManager() {
		return userID, core.NativePermissionOwner, nil
	}
	var level int
	err = tx.QueryRow(`SELECT permission_level FROM native_members WHERE resource_id = ? AND user_id = ?`, id, userID).Scan(&level)
	if errors.Is(err, sql.ErrNoRows) || err == nil && level < required {
		return "", -1, core.ErrNativePermission
	}
	return userID, level, err
}

func (db *DB) CreateNativeResource(repository, format, name, actor string, now int64) (*core.NativeResource, error) {
	id, err := nativeKey(repository, name)
	engine, valid := config.LookupRepositoryEngine(format)
	if err != nil || !valid || !engine.ManagedNative || now <= 0 {
		return nil, core.ErrNativeInvalid
	}
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	userID, user, err := nativeActorTx(tx, actor)
	if err != nil {
		return nil, err
	}
	if !user.CheckUpdatePermission(repository) {
		return nil, core.ErrNativePermission
	}
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM native_resources WHERE id = ?`, id).Scan(&count); err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, core.ErrNativeExists
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM native_resources WHERE repository = ?`, repository).Scan(&count); err != nil {
		return nil, err
	}
	if count >= 10000 {
		return nil, core.ErrNativeBusy
	}
	if _, err := tx.Exec(`INSERT INTO native_resources (id, repository, format, name, description, signing_key, created_at, published_at, archived) VALUES (?, ?, ?, ?, '', '', ?, 0, 0)`, id, repository, engine.Protocol, name, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO native_members (resource_id, user_id, permission_level, added_at) VALUES (?, ?, ?, ?)`, id, userID, core.NativePermissionOwner, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &core.NativeResource{ID: id, Repository: repository, Format: engine.Protocol, Name: name, CreatedAt: now, PermissionLevel: core.NativePermissionOwner}, nil
}

const nativeResourceColumns = `r.id, r.repository, r.format, r.name, r.description, r.signing_key, r.created_at, r.published_at, COALESCE(m.permission_level, -1), r.archived`

func scanNativeResource(row row) (*core.NativeResource, error) {
	resource := &core.NativeResource{}
	var archived int
	err := row.Scan(&resource.ID, &resource.Repository, &resource.Format, &resource.Name, &resource.Description, &resource.SigningKey, &resource.CreatedAt, &resource.PublishedAt, &resource.PermissionLevel, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNativeNotFound
	}
	resource.Archived = archived != 0
	return resource, err
}

func (db *DB) GetNativeResource(repository, name, actor string) (*core.NativeResource, error) {
	id, err := nativeKey(repository, name)
	if err != nil {
		return nil, err
	}
	userID, err := db.nativeViewerID(actor)
	if err != nil {
		return nil, err
	}
	resource, err := scanNativeResource(db.QueryRow(`SELECT `+nativeResourceColumns+` FROM native_resources r
		LEFT JOIN native_members m ON m.resource_id = r.id AND m.user_id = ?
		WHERE r.id = ?`, userID, id))
	if err != nil {
		return nil, err
	}
	deprecated, _ := db.IsPackageDeprecated(resource.Format, resource.Repository, resource.Name)
	resource.Deprecated = deprecated
	locks, _ := db.GetResourceLocks(core.ResourceLockTarget{Format: resource.Format, Repository: resource.Repository, Name: resource.Name}, false)
	resource.Locks = locks
	return resource, nil
}

func (db *DB) nativeViewerID(actor string) (string, error) {
	if actor == "" || actor == "guest" {
		return "", nil
	}
	id, err := db.userIDForUsername(strings.ToLower(actor))
	if errors.Is(err, core.ErrUserProfileNotFound) {
		return "", nil
	}
	return id, err
}

func (db *DB) ListNativeResources(repository, actor string, staff bool, limit, offset int) ([]*core.NativeResource, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return nil, core.ErrNativeInvalid
	}
	userID, err := db.nativeViewerID(actor)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT `+nativeResourceColumns+` FROM native_resources r
		LEFT JOIN native_members m ON m.resource_id = r.id AND m.user_id = ?
		WHERE r.repository = ? AND (r.published_at > 0 OR m.user_id IS NOT NULL OR ? = 1)
		ORDER BY r.name, r.id LIMIT ? OFFSET ?`, userID, repository, boolInt(staff), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := make([]*core.NativeResource, 0)
	for rows.Next() {
		resource, err := scanNativeResource(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for _, resource := range resources {
		deprecated, _ := db.IsPackageDeprecated(resource.Format, resource.Repository, resource.Name)
		resource.Deprecated = deprecated
		locks, _ := db.GetResourceLocks(core.ResourceLockTarget{Format: resource.Format, Repository: resource.Repository, Name: resource.Name}, false)
		resource.Locks = locks
	}
	return resources, nil
}

func (db *DB) SearchNativeResources(repository, query, actor string, staff bool, limit int) ([]*core.NativeResource, int, error) {
	if db == nil || db.SQLDB == nil {
		return nil, 0, core.ErrDatabaseUnavailable
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	query = strings.ToLower(SanitizeInputString(strings.TrimSpace(query), 128))
	query = strings.NewReplacer("%", "", "_", "").Replace(query)
	pattern := "%" + query + "%"

	userID, err := db.nativeViewerID(actor)
	if err != nil {
		return nil, 0, err
	}

	where := `r.repository = ? AND (r.name LIKE ? OR LOWER(r.description) LIKE ?) AND (r.published_at > 0 OR m.user_id IS NOT NULL OR ? = 1)`
	args := []any{repository, pattern, pattern, boolInt(staff)}

	var total int
	countQuery := `SELECT COUNT(DISTINCT r.id) FROM native_resources r
		LEFT JOIN native_members m ON m.resource_id = r.id AND m.user_id = ?
		WHERE ` + where
	if err := db.QueryRow(countQuery, append([]any{userID}, args...)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count native search results: %w", err)
	}

	selectQuery := `SELECT ` + nativeResourceColumns + ` FROM native_resources r
		LEFT JOIN native_members m ON m.resource_id = r.id AND m.user_id = ?
		WHERE ` + where + `
		ORDER BY CASE WHEN LOWER(r.name) = ? THEN 0 ELSE 1 END, r.name, r.id LIMIT ?`
	rows, err := db.Query(selectQuery, append([]any{userID}, append(args, query, limit)...)...)
	if err != nil {
		return nil, 0, fmt.Errorf("search native resources: %w", err)
	}
	defer rows.Close()

	resources := make([]*core.NativeResource, 0, limit)
	for rows.Next() {
		resource, err := scanNativeResource(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan native search result: %w", err)
		}
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate native search results: %w", err)
	}
	rows.Close()

	for _, resource := range resources {
		deprecated, _ := db.IsPackageDeprecated(resource.Format, resource.Repository, resource.Name)
		resource.Deprecated = deprecated
		locks, _ := db.GetResourceLocks(core.ResourceLockTarget{Format: resource.Format, Repository: resource.Repository, Name: resource.Name}, false)
		resource.Locks = locks
	}
	return resources, total, nil
}

func (db *DB) ListNativeMembers(repository, name string) ([]*core.NativeMember, error) {
	id, err := nativeKey(repository, name)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT m.user_id, p.username, m.permission_level FROM native_members m JOIN user_profiles p ON p.user_id = m.user_id WHERE m.resource_id = ? ORDER BY m.permission_level DESC, p.username LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]*core.NativeMember, 0)
	for rows.Next() {
		member := &core.NativeMember{}
		if err := rows.Scan(&member.UserID, &member.Username, &member.Level); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

// SetNativeMember adds or changes a resource grant. A level of -1 removes it.
// Direct grants are explicit owner/administrator actions and never confer a
// repository role. The last L4 membership is retained transactionally.
func (db *DB) SetNativeMember(repository, name, actor, target string, level int, now int64) error {
	if level < -1 || level > core.NativePermissionOwner || now <= 0 {
		return core.ErrNativeInvalid
	}
	if strings.EqualFold(actor, target) && level >= 0 {
		return core.ErrNativePermission
	}
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	accounts := []string{strings.ToLower(strings.TrimSpace(actor)), strings.ToLower(strings.TrimSpace(target))}
	slices.Sort(accounts)
	for _, username := range slices.Compact(accounts) {
		if err := lockAccountByUsernameTx(tx, username); err != nil && !(level == -1 && username != strings.ToLower(actor) && errors.Is(err, core.ErrAccountDeleted)) {
			return err
		}
	}
	_, actorLevel, err := nativePermissionTx(tx, repository, name, actor, core.NativePermissionManage)
	if err != nil {
		return err
	}
	targetID, err := userIDForUsernameTx(tx, strings.ToLower(strings.TrimSpace(target)))
	if err != nil {
		return err
	}
	if level >= 0 {
		if _, _, err := nativeActorTx(tx, target); err != nil {
			return err
		}
	}
	id, _ := nativeKey(repository, name)
	old := -1
	err = tx.QueryRow(`SELECT permission_level FROM native_members WHERE resource_id = ? AND user_id = ?`, id, targetID).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if (level == core.NativePermissionOwner || old == core.NativePermissionOwner) && actorLevel < core.NativePermissionOwner {
		return core.ErrNativePermission
	}
	if old == core.NativePermissionOwner && level != old {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM native_members WHERE resource_id = ? AND permission_level = ? AND user_id <> ?`, id, core.NativePermissionOwner, targetID).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return core.ErrNativeLastOwner
		}
	}
	if old == -1 && level >= 0 {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM native_members WHERE resource_id = ?`, id).Scan(&count); err != nil {
			return err
		}
		if count >= 100 {
			return core.ErrNativeBusy
		}
		_, err = tx.Exec(`INSERT INTO native_members (resource_id,user_id,permission_level,added_at) VALUES (?,?,?,?)`, id, targetID, level, now)
	} else if level == -1 {
		_, err = tx.Exec(`DELETE FROM native_members WHERE resource_id = ? AND user_id = ?`, id, targetID)
	} else {
		_, err = tx.Exec(`UPDATE native_members SET permission_level = ? WHERE resource_id = ? AND user_id = ?`, level, id, targetID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) CheckNativePermission(repository, name, actor string, level int) error {
	if level < 0 || level > core.NativePermissionOwner {
		return core.ErrNativeInvalid
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, _, err = nativePermissionTx(tx, repository, name, actor, level)
	return err
}

func (db *DB) UpdateNativeResource(repository, name, actor, description, signingKey string) error {
	if len(description) > 4096 || strings.ContainsRune(description, 0) || len(signingKey) > 64<<10 {
		return core.ErrNativeInvalid
	}
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := nativePermissionTx(tx, repository, name, actor, core.NativePermissionManage); err != nil {
		return err
	}
	id, _ := nativeKey(repository, name)
	if _, err := tx.Exec(`UPDATE native_resources SET description = ?, signing_key = ? WHERE id = ?`, description, signingKey, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) SetNativeResourceArchived(repository, name, actor string, archived bool) error {
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := nativePermissionTx(tx, repository, name, actor, core.NativePermissionManage); err != nil {
		return err
	}
	id, _ := nativeKey(repository, name)
	if _, err := tx.Exec(`UPDATE native_resources SET archived = ? WHERE id = ?`, boolInt(archived), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteNativeResource(repository, name, actor string) error {
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, user, err := nativeActorTx(tx, actor)
	if err != nil {
		return err
	}
	if _, _, err := nativePermissionTx(tx, repository, name, actor, core.NativePermissionOwner); err != nil {
		return err
	}
	id, _ := nativeKey(repository, name)
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM native_artifacts WHERE resource_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count != 0 && !user.IsManager() {
		return core.ErrNativeBusy
	}
	pending, err := hasPendingPackageReviewTx(tx, core.ReviewResourceNativePackage, repository, name)
	if err != nil {
		return err
	}
	if pending {
		return core.ErrNativeBusy
	}
	var format string
	_ = tx.QueryRow(`SELECT format FROM native_resources WHERE id = ?`, id).Scan(&format)
	if _, err := tx.Exec(`DELETE FROM native_artifacts WHERE resource_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM native_members WHERE resource_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM native_resources WHERE id = ?`, id); err != nil {
		return err
	}
	if format != "" {
		_, _ = tx.Exec(`DELETE FROM resource_locks WHERE format = ? AND repository = ? AND name = ?`, format, repository, name)
	}
	return tx.Commit()
}

func (db *DB) DeleteNativeRepository(repository string) error {
	if !utils.IsValidRepositoryName(repository) {
		return core.ErrNativeInvalid
	}
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id FROM native_resources WHERE repository = ? LIMIT 10001`, repository)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(ids) > 10000 {
		return core.ErrNativeBusy
	}
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM native_members WHERE resource_id = ?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM native_artifacts WHERE repository = ?`, repository); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM native_resources WHERE repository = ?`, repository); err != nil {
		return err
	}
	return tx.Commit()
}

func validNativeArtifact(artifact core.NativeArtifact) bool {
	clean, valid := utils.SanitizePath(artifact.Path)
	return valid && clean == artifact.Path && len(clean) > 0 && len(clean) <= 1024 && len(artifact.Version) > 0 && len(artifact.Version) <= 255 &&
		!strings.ContainsAny(artifact.Version, "\x00\r\n") && artifact.Size >= 0 && artifact.CreatedAt > 0
}

func (db *DB) SaveNativeArtifact(artifact core.NativeArtifact, actor string, published bool) error {
	if !validNativeArtifact(artifact) {
		return core.ErrNativeInvalid
	}
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := nativePermissionTx(tx, artifact.Repository, artifact.Name, actor, core.NativePermissionPublish); err != nil {
		return err
	}
	resourceID, _ := nativeKey(artifact.Repository, artifact.Name)
	id := nativeID(artifact.Repository, artifact.Path)
	var oldResource string
	err = tx.QueryRow(`SELECT resource_id FROM native_artifacts WHERE id = ?`, id).Scan(&oldResource)
	if err == nil {
		if oldResource != resourceID {
			return core.ErrNativePermission
		}
		_, err = tx.Exec(`UPDATE native_artifacts SET version = ?,size = ?,created_at = ?,published = ? WHERE id = ?`, artifact.Version, artifact.Size, artifact.CreatedAt, boolInt(published), id)
	} else if errors.Is(err, sql.ErrNoRows) {
		if !published {
			var count int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM native_artifacts WHERE resource_id = ? AND published = 0`, resourceID).Scan(&count); err != nil {
				return err
			}
			if count >= 256 {
				return core.ErrNativeBusy
			}
			if err := tx.QueryRow(`SELECT COUNT(*) FROM native_artifacts WHERE published = 0`).Scan(&count); err != nil {
				return err
			}
			if count >= 16384 {
				return core.ErrNativeBusy
			}
		}
		_, err = tx.Exec(`INSERT INTO native_artifacts (id,resource_id,repository,path,version,size,created_at,published) VALUES (?,?,?,?,?,?,?,?)`, id, resourceID, artifact.Repository, artifact.Path, artifact.Version, artifact.Size, artifact.CreatedAt, boolInt(published))
	}
	if err != nil {
		return err
	}
	if published {
		if _, err := tx.Exec(`UPDATE native_resources SET published_at = ? WHERE id = ? AND published_at = 0`, artifact.CreatedAt, resourceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const nativeArtifactColumns = `a.repository,a.resource_id,r.name,a.version,a.path,a.size,a.created_at,a.published`

func scanNativeArtifact(row row) (*core.NativeArtifact, error) {
	artifact := &core.NativeArtifact{}
	var published int
	err := row.Scan(&artifact.Repository, &artifact.ResourceID, &artifact.Name, &artifact.Version, &artifact.Path, &artifact.Size, &artifact.CreatedAt, &published)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.ErrNativeNotFound
	}
	artifact.Published = published != 0
	return artifact, err
}

func (db *DB) GetNativeArtifact(repository, path string) (*core.NativeArtifact, error) {
	return scanNativeArtifact(db.QueryRow(`SELECT `+nativeArtifactColumns+` FROM native_artifacts a JOIN native_resources r ON r.id = a.resource_id WHERE a.id = ?`, nativeID(repository, path)))
}

func (db *DB) ListNativeArtifacts(repository, name string, includePending bool, limit, offset int) ([]*core.NativeArtifact, error) {
	id, err := nativeKey(repository, name)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return nil, core.ErrNativeInvalid
	}
	rows, err := db.Query(`SELECT `+nativeArtifactColumns+` FROM native_artifacts a JOIN native_resources r ON r.id = a.resource_id
		WHERE a.resource_id = ? AND (a.published = 1 OR ? = 1) ORDER BY a.created_at DESC,a.id LIMIT ? OFFSET ?`, id, boolInt(includePending), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]*core.NativeArtifact, 0)
	for rows.Next() {
		artifact, err := scanNativeArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (db *DB) NativeVersionArtifacts(repository, name, version string) ([]*core.NativeArtifact, error) {
	id, err := nativeKey(repository, name)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT `+nativeArtifactColumns+` FROM native_artifacts a JOIN native_resources r ON r.id = a.resource_id
		WHERE a.resource_id = ? AND a.version = ? ORDER BY a.path LIMIT 257`, id, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]*core.NativeArtifact, 0)
	for rows.Next() {
		artifact, err := scanNativeArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if len(artifacts) > 256 {
		return nil, core.ErrNativeBusy
	}
	return artifacts, rows.Err()
}

func (db *DB) PublishNativeArtifacts(repository, name, version, actor string, paths []string, now int64) error {
	if len(paths) == 0 || len(paths) > 256 {
		return core.ErrNativeInvalid
	}
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err := nativePermissionTx(tx, repository, name, actor, core.NativePermissionPublish); err != nil {
		return err
	}
	id, _ := nativeKey(repository, name)
	for _, path := range paths {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM native_artifacts WHERE id = ? AND resource_id = ? AND version = ?`, nativeID(repository, path), id, version).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return core.ErrNativeInvalid
		}
		if _, err := tx.Exec(`UPDATE native_artifacts SET published = 1 WHERE id = ? AND resource_id = ?`, nativeID(repository, path), id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE native_resources SET published_at = ? WHERE id = ? AND published_at = 0`, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) DeleteNativeArtifact(repository, path, actor string) error {
	nativeResourceMutationLock.Lock()
	defer nativeResourceMutationLock.Unlock()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	artifact, err := scanNativeArtifact(tx.QueryRow(`SELECT `+nativeArtifactColumns+` FROM native_artifacts a JOIN native_resources r ON r.id = a.resource_id WHERE a.id = ?`, nativeID(repository, path)))
	if errors.Is(err, core.ErrNativeNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if actor != "" {
		if _, _, err := nativePermissionTx(tx, repository, artifact.Name, actor, core.NativePermissionVersion); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM native_artifacts WHERE id = ?`, nativeID(repository, path)); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteNativeArtifactsUnder follows an already authorized physical deletion.
// Callers hold the repository mutation gate and check every resource affected.
func (db *DB) DeleteNativeArtifactsUnder(repository, prefix string) error {
	if !utils.IsValidRepositoryName(repository) || prefix == "" || prefix == "/" {
		return core.ErrNativeInvalid
	}
	_, err := db.Exec(`DELETE FROM native_artifacts WHERE repository = ? AND (path = ? OR path LIKE ? ESCAPE '!')`, repository, prefix, escapeLikePrefix(strings.TrimSuffix(prefix, "/")+"/")+"%")
	return err
}

// DeleteNativeVersionArtifacts removes all artifacts whose clean version equals the given
// version string, where the clean version is the part before the first '/' or '#'.
// Permission must be checked by the caller. Returns the deleted artifacts for physical cleanup.
func (db *DB) DeleteNativeVersionArtifacts(repository, name, version string) ([]*core.NativeArtifact, error) {
	id, err := nativeKey(repository, name)
	if err != nil || version == "" || len(version) > 255 {
		return nil, core.ErrNativeInvalid
	}
	// Gather matching artifacts first
	rows, err := db.Query(`SELECT `+nativeArtifactColumns+` FROM native_artifacts a JOIN native_resources r ON r.id = a.resource_id
		WHERE a.resource_id = ? AND (
			a.version = ?
			OR a.version LIKE ? ESCAPE '!'
			OR a.version LIKE ? ESCAPE '!'
		)`, id, version,
		escapeLikePrefix(version)+"/"+"%",
		escapeLikePrefix(version)+"#"+"%",
	)
	if err != nil {
		return nil, err
	}
	var deleted []*core.NativeArtifact
	var paths []string
	for rows.Next() {
		art, err := scanNativeArtifact(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		deleted = append(deleted, art)
		paths = append(paths, art.Path)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	// Delete by artifact id (safe even if table changes between select and delete)
	for _, path := range paths {
		artID := nativeID(repository, path)
		if _, err := db.Exec(`DELETE FROM native_artifacts WHERE id = ?`, artID); err != nil {
			return nil, err
		}
	}
	return deleted, nil
}

func (db *DB) ListHiddenNativeArtifacts() ([]*core.NativeArtifact, error) {
	rows, err := db.Query(`SELECT ` + nativeArtifactColumns + ` FROM native_artifacts a JOIN native_resources r ON r.id = a.resource_id WHERE a.published = 0 ORDER BY a.id LIMIT 65537`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]*core.NativeArtifact, 0)
	for rows.Next() {
		if len(artifacts) == 65536 {
			return nil, fmt.Errorf("too many unpublished native artifacts")
		}
		artifact, err := scanNativeArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

// Metadata visibility and the review decision commit in the same transaction.
func publishNativeReviewTx(tx *Tx, task *core.ReviewTask, now int64) error {
	id, err := nativeKey(task.Repository, task.ResourceKey)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE native_resources SET published_at = published_at WHERE id = ?`, id); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT path FROM review_task_files WHERE task_id = ?`, task.ID)
	if err != nil {
		return err
	}
	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return err
		}
		paths = append(paths, path)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return core.ErrReviewInvalidRequest
	}
	for _, path := range paths {
		var resourceID, version string
		if err := tx.QueryRow(`SELECT resource_id,version FROM native_artifacts WHERE id = ?`, nativeID(task.Repository, path)).Scan(&resourceID, &version); err != nil {
			return core.ErrReviewResourceConflict
		}
		if resourceID != id || version != task.ResourceVersion {
			return core.ErrReviewResourceConflict
		}
		if _, err := tx.Exec(`UPDATE native_artifacts SET published = 1 WHERE id = ? AND resource_id = ?`, nativeID(task.Repository, path), id); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`UPDATE native_resources SET published_at = ? WHERE id = ? AND published_at = 0`, now, id)
	return err
}
