/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package demo

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
)

const maxDemoSessions = 1024

// sessionStore keeps real browser authentication out of the immutable preset session table.
// Preset sessions are display data and can never authenticate a request.
type sessionStore struct {
	core.StateDB
	database         *database.DB
	repositoryWriter *database.DB
	state            *core.AppState
	mu               sync.Mutex
	sessions         map[string]*core.Session
}

func newSessionStore(db *database.DB, state *core.AppState) *sessionStore {
	return &sessionStore{StateDB: db, database: db, state: state, sessions: make(map[string]*core.Session)}
}

func (s *sessionStore) Close() error {
	err := s.database.Close()
	if s.repositoryWriter != nil {
		err = errors.Join(err, s.repositoryWriter.Close())
	}
	return err
}

// Only repository definitions receive a writable connection in configuration mode.
// All inherited data methods, including log writes, retain SQLite's read-only boundary.
func (s *sessionStore) SaveRepositorySettings(settings config.MavenSettings) error {
	if s.repositoryWriter == nil {
		return core.ErrDemoReadOnly
	}
	return s.repositoryWriter.SaveRepositorySettings(settings)
}

func (s *sessionStore) GetSession(token string) (*core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[token], nil
}

func (s *sessionStore) SaveSession(session *core.Session, token string) error {
	if session == nil || token == "" || session.OAuthGrant != nil {
		return core.ErrMFAInvalid
	}
	account, err := s.database.GetTokenByName(session.Username)
	if err != nil {
		return err
	}
	if account == nil || account.DeletedAt > 0 {
		return core.ErrAccountDeleted
	}
	now := time.Now().UnixMilli()
	if account.Ban.IsActive(now) {
		return core.ErrAccountBanned
	}
	if account.ExpiresAt != nil && *account.ExpiresAt <= now {
		return core.ErrMFAInvalid
	}
	mfa, err := s.database.GetMFAState(session.Username)
	if err != nil {
		return err
	}
	if session.AuthenticationSnapshot != "" && session.AuthenticationSnapshot != mfa.Snapshot {
		return core.ErrMFAInvalid
	}
	s.mu.Lock()
	var removed []string
	oldest, oldestAt := "", int64(0)
	for key, existing := range s.sessions {
		lastActive := existing.LastActive.Load()
		if lastActive+core.SessionIdleTimeoutMillis <= now {
			delete(s.sessions, key)
			removed = append(removed, key)
		} else if oldest == "" || lastActive < oldestAt {
			oldest, oldestAt = key, lastActive
		}
	}
	if len(s.sessions) >= maxDemoSessions && s.sessions[token] == nil {
		delete(s.sessions, oldest)
		removed = append(removed, oldest)
	}
	s.sessions[token] = session
	s.mu.Unlock()
	for _, key := range removed {
		s.state.Inner.Sessions.Delete(key)
		s.state.DeleteAuthCache("Session " + key)
	}
	return nil
}

func (s *sessionStore) GetSessionOAuthGrant(string) (*core.SessionOAuthGrant, error) { return nil, nil }

func (s *sessionStore) UpdateSessionLastActive(token string, at int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[token]; session != nil {
		session.LastActive.Store(at)
	}
	return nil
}

func (s *sessionStore) DeleteSession(token string) error {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
	return nil
}

func (s *sessionStore) DeleteSessionsByUsername(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, session := range s.sessions {
		if strings.EqualFold(session.Username, username) {
			delete(s.sessions, token)
		}
	}
	return nil
}

func (s *sessionStore) DeleteExpiredSessions(before int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, session := range s.sessions {
		if session.LastActive.Load() < before {
			delete(s.sessions, token)
		}
	}
	return nil
}

func (s *sessionStore) DeleteUserSessionByPublicID(username, publicID, current string) (string, bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, session := range s.sessions {
		if strings.EqualFold(session.Username, username) && session.PublicID == publicID {
			delete(s.sessions, token)
			return token, true, token == current, nil
		}
	}
	return "", false, false, nil
}

func (s *sessionStore) DeleteOtherUserSessions(username, keep string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []string
	for token, session := range s.sessions {
		if strings.EqualFold(session.Username, username) && token != keep {
			delete(s.sessions, token)
			removed = append(removed, token)
		}
	}
	return removed, nil
}

func (s *sessionStore) UpdateSessionsUsername(string, string) error { return core.ErrDemoReadOnly }

func (s *sessionStore) RevokeOAuthSessions(core.OAuthRevocation, int64) ([]string, error) {
	return nil, core.ErrDemoReadOnly
}

func (s *sessionStore) ListUserSessions(username, current string) ([]core.SessionDto, error) {
	result, err := s.database.ListUserSessions(username, "")
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, session := range s.sessions {
		if !strings.EqualFold(session.Username, username) {
			continue
		}
		active := session.LastActive.Load()
		result = append(result, core.SessionDto{PublicID: session.PublicID, Username: session.Username,
			IP: session.IP, UserAgent: session.UserAgent, CreatedAt: session.CreatedAt, LastActive: active,
			ExpiresAt: active + core.SessionIdleTimeoutMillis, Current: token == current, LoginMethod: session.LoginMethod})
	}
	slices.SortFunc(result, func(a, b core.SessionDto) int {
		if order := cmp.Compare(b.CreatedAt, a.CreatedAt); order != 0 {
			return order
		}
		return cmp.Compare(b.PublicID, a.PublicID)
	})
	return result, nil
}

func (s *sessionStore) ListActiveUserSessions(username string, before int64, beforeID string, limit int, now int64) ([]core.SessionDto, error) {
	all, err := s.ListUserSessions(username, "")
	if err != nil {
		return nil, err
	}
	result := make([]core.SessionDto, 0, min(max(limit, 1), 100))
	for _, session := range all {
		if session.ExpiresAt <= now || before > 0 && (session.CreatedAt > before || session.CreatedAt == before && session.PublicID >= beforeID) {
			continue
		}
		result = append(result, session)
		if len(result) >= min(max(limit, 1), 100) {
			break
		}
	}
	return result, nil
}
