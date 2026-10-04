/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package core defines shared application state and domain models.
package core

import (
	"crypto/rand"
	"errors"
	"sync/atomic"
	atomic2 "sync/atomic/v2"
	"sync/v2"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/llxisdsh/pb"

	"renop/internal/cache"
	"renop/internal/config"
	"renop/internal/repositorycapacity"
	"renop/internal/service/index"
)

type AuthCacheEntry struct {
	User           *config.User
	CredentialKind string
	AuthScheme     string
	APITokenID     string
	Scopes         []string
	Targets        map[string][]string
	ExpiredAt      int64
	Invalid        bool
}

var ErrDatabaseUnavailable = errors.New("database unavailable")

type CachedFile struct {
	Bytes        []byte
	Etag         *string
	LastModified *string
	ContentType  string
}

type AppStateInner struct {
	Config                      *atomic2.Value[*config.Config]
	ConfigWriteLock             sync.Mutex
	TokensCount                 atomic.Uint64
	TokenWriteLock              sync.Mutex
	StatusSnapshots             atomic.Pointer[[]StatusSnapshot]
	ActiveRequests              atomic.Uint64
	FailuresCount               atomic.Uint64
	AuthCache                   cache.IndexedMap[string, AuthCacheEntry]
	AuthCacheEntries            atomic.Uint64
	AuthCacheGeneration         atomic.Uint64
	AuthCacheWriteLock          sync.Mutex
	Sessions                    pb.MapOf[string, *Session]
	AuditLogChan                chan *AuditLogEntry
	MailWake                    chan struct{}
	DB                          StateDB
	DockerSecret                []byte
	DownloadStatisticsCounter   DownloadStatisticsCounter
	DownloadStatisticsCounterMu sync.Mutex
	ExternalAuthStates          *TransientAuthStateStore
	CaptchaApprovals            *TransientAuthStateStore
	DeviceAccess                sync.Map[string, *DeviceAccessRecord]

	RepositoryCapacity     *repositorycapacity.Manager
	FileIndex              *index.FileIndex
	IndexWatcher           *fsnotify.Watcher
	IndexWatcherMutex      sync.Mutex
	StartTime              int64
	FileCache              *FileByteCache
	HTMLCache              *FileByteCache
	NativeIndexCache       *FileByteCache
	MetadataCache          cache.IndexedMap[string, *config.Metadata]
	RemoteCache            *cache.Remote
	MetadataCacheEntries   atomic.Uint64
	MetadataCacheWriteLock sync.Mutex
	InFlightDownloads      *InFlightManager
	GPGKeyFetches          *InFlightManager
	GPGUserKeyUpdates      *InFlightManager
	GPGReleaseWake         chan struct{}
	GPGReleaseWorkerActive atomic.Bool
	AnomalyFailures        *AnomalyFailureStore
	ProxyClientSemaphore   chan struct{}
	TrafficManager         any
}

type AppState struct {
	Inner *AppStateInner
}

func NewAppState() *AppState {
	return &AppState{
		Inner: &AppStateInner{
			Config:               &atomic2.Value[*config.Config]{},
			RepositoryCapacity:   repositorycapacity.New(),
			NativeIndexCache:     NewFileByteCache(32 << 20),
			ProxyClientSemaphore: make(chan struct{}, 256),
			StartTime:            time.Now().UnixMilli(),
			InFlightDownloads:    NewInFlightManager(),
			GPGKeyFetches:        NewInFlightManager(),
			GPGUserKeyUpdates:    NewInFlightManager(),
			GPGReleaseWake:       make(chan struct{}, 1),
			AnomalyFailures:      NewAnomalyFailureStore(),
			AuditLogChan:         make(chan *AuditLogEntry, 500),
			MailWake:             make(chan struct{}, 1),
			ExternalAuthStates:   NewTransientAuthStateStore(),
			CaptchaApprovals:     NewTransientAuthStateStore(),
		},
	}
}

func (state *AppState) GetDB() StateDB {
	if state == nil || state.Inner == nil {
		return nil
	}
	return state.Inner.DB
}

func (state *AppState) GetDockerSecret() []byte {
	if state == nil || state.Inner == nil {
		return []byte("renop-docker-token-secret-fallback")
	}
	if len(state.Inner.DockerSecret) == 0 {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			state.Inner.DockerSecret = []byte("renop-docker-token-secret-fallback")
		} else {
			state.Inner.DockerSecret = secret
		}
	}
	return state.Inner.DockerSecret
}
