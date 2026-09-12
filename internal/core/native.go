/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package core

import (
	"errors"
	"strings"
)

const (
	NativePermissionRead = iota
	NativePermissionPublish
	NativePermissionVersion
	NativePermissionManage
	NativePermissionOwner
)

var (
	ErrNativeInvalid    = errors.New("invalid native resource")
	ErrNativeNotFound   = errors.New("native resource not found")
	ErrNativeExists     = errors.New("native resource already exists")
	ErrNativePermission = errors.New("native resource permission denied")
	ErrNativeLastOwner  = errors.New("native resource requires an owner")
	ErrNativeBusy       = errors.New("native resource has publications or pending reviews")
	ErrNativeSignature  = errors.New("native signature is missing or invalid")
)

// NativeResource is an explicitly reserved, unprefixed publishing identity.
// Names come from native package metadata (or a Conan recipe reference), not a
// Maven domain or a global-team namespace.
type NativeResource struct {
	ID              string          `json:"id"`
	Repository      string          `json:"repository"`
	Format          string          `json:"format"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	SigningKey      string          `json:"signing_key"`
	CreatedAt       int64           `json:"created_at"`
	PublishedAt     int64           `json:"published_at"`
	PermissionLevel int             `json:"permission_level"`
	Archived        bool            `json:"archived"`
	Deprecated      bool            `json:"deprecated"`
	Locks           []*ResourceLock `json:"locks,omitempty"`
}

type NativeMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Level    int    `json:"level"`
}

// NativeArtifact binds a concrete native path to its managed publishing identity.
type NativeArtifact struct {
	Repository string `json:"repository"`
	ResourceID string `json:"resource_id"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	CreatedAt  int64  `json:"created_at"`
	Published  bool   `json:"published"`
}

// NativePackageDB separates native resource persistence from protocol parsing.
type NativePackageDB interface {
	CreateNativeResource(repository, format, name, actor string, now int64) (*NativeResource, error)
	GetNativeResource(repository, name, actor string) (*NativeResource, error)
	ListNativeResources(repository, actor string, staff bool, limit, offset int) ([]*NativeResource, error)
	SearchNativeResources(repository, query, actor string, staff bool, limit int) ([]*NativeResource, int, error)
	ListNativeMembers(repository, name string) ([]*NativeMember, error)
	SetNativeMember(repository, name, actor, target string, level int, now int64) error
	UpdateNativeResource(repository, name, actor, description, signingKey string) error
	SetNativeResourceArchived(repository, name, actor string, archived bool) error
	DeleteNativeResource(repository, name, actor string) error
	DeleteNativeRepository(repository string) error
	CheckNativePermission(repository, name, actor string, level int) error
	HasNativePublishAccess(repository, actor string) (bool, error)
	NativePublicationState(repository, name, version string) (*ReviewTask, error)
	NativePathHasPendingReview(repository, prefix string) (bool, error)
	SaveNativeArtifact(artifact NativeArtifact, actor string, published bool) error
	GetNativeArtifact(repository, path string) (*NativeArtifact, error)
	ListNativeArtifacts(repository, name string, includePending bool, limit, offset int) ([]*NativeArtifact, error)
	NativeVersionArtifacts(repository, name, version string) ([]*NativeArtifact, error)
	PublishNativeArtifacts(repository, name, version, actor string, paths []string, now int64) error
	DeleteNativeArtifact(repository, path, actor string) error
	DeleteNativeArtifactsUnder(repository, prefix string) error
	DeleteNativeVersionArtifacts(repository, name, version string) ([]*NativeArtifact, error)
	ListHiddenNativeArtifacts() ([]*NativeArtifact, error)
}

func ValidNativeResourceName(name string) bool {
	if name == "" || len(name) > 255 || name == "." || name == ".." {
		return false
	}
	if strings.Contains(name, "/") {
		base, suffix, ok := strings.Cut(name, "@")
		parts := strings.Split(suffix, "/")
		return ok && !strings.Contains(base, "@") && len(parts) == 2 &&
			ValidNativeResourceName(base) && !strings.ContainsAny(parts[0]+parts[1], "@/") &&
			ValidNativeResourceName(parts[0]) && ValidNativeResourceName(parts[1])
	}
	first := name[0]
	if !isNativeAlphaNumeric(first) {
		return false
	}
	last := name[len(name)-1]
	if !isNativeAlphaNumeric(last) && last != '+' {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsRune(name, '@') {
		return false
	}
	for i := 0; i < len(name); i++ {
		char := name[i]
		if !(isNativeAlphaNumeric(char) || char == '_' || char == '.' || char == '+' || char == '-') {
			return false
		}
	}
	return true
}

func isNativeAlphaNumeric(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
}
