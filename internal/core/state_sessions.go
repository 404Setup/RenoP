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
	"strings"
	"sync/v2"
	"time"

	"renop/pkg/id"
)

func (state *AppState) GetSession(sessionToken string) *Session {
	if state == nil || state.Inner == nil || sessionToken == "" {
		return nil
	}
	if sess, ok := state.Inner.Sessions.Load(sessionToken); ok && sess != nil {
		return sess
	}
	db := state.GetDB()
	if db == nil {
		return nil
	}
	sess, err := db.GetSession(sessionToken)
	if err != nil || sess == nil {
		return nil
	}
	return sess
}

func (state *AppState) SaveSession(session *Session, sessionToken string) error {
	if state == nil || state.Inner == nil || session == nil || sessionToken == "" {
		return nil
	}
	if db := state.GetDB(); db != nil {
		if err := db.SaveSession(session, sessionToken); err != nil {
			return err
		}
	}
	state.Inner.Sessions.Store(sessionToken, session)
	return nil
}

// RevokeSession removes a browser session by its secret token and invalidates related auth cache.
func (state *AppState) RevokeSession(sessionToken string) (bool, error) {
	if state == nil || state.Inner == nil || sessionToken == "" {
		return false, nil
	}
	if db := state.GetDB(); db != nil {
		if err := db.DeleteSession(sessionToken); err != nil {
			return false, err
		}
	}
	state.DeleteAuthCache("Session " + sessionToken)
	state.Inner.Sessions.Delete(sessionToken)
	return true, nil
}

// sessionToDto maps an in-memory Session (+ map key) to a public SessionDto.
// Session secret (map key) is never included.
func sessionToDto(secretToken string, session *Session, currentSessionToken string) SessionDto {
	lastActive := session.LastActive.Load()
	return SessionDto{
		PublicID:   session.PublicID,
		Username:   session.Username,
		IP:         session.IP,
		UserAgent:  session.UserAgent,
		CreatedAt:  session.CreatedAt,
		LastActive: lastActive,
		ExpiresAt:  lastActive + SessionIdleTimeoutMillis,
		Current:    secretToken != "" && secretToken == currentSessionToken,
	}
}

type DeviceAccessRecord struct {
	mu          sync.Mutex
	PublicID    string
	Username    string
	UserAgent   string
	LoginMethod string
	IPs         []string
	CreatedAt   int64
	LastActive  int64
}

// RecordDeviceAccess tracks recent device accesses from Basic Auth, API Token, and browser sessions.
func (state *AppState) RecordDeviceAccess(username, loginMethod, userAgent, ip string, now int64) {
	if state == nil || state.Inner == nil || username == "" {
		return
	}
	username = strings.ToLower(strings.TrimSpace(username))
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		userAgent = "Unknown"
	}
	ip = strings.TrimSpace(ip)
	key := username + "\x00" + loginMethod + "\x00" + userAgent
	initialIPs := make([]string, 0, 10)
	if ip != "" {
		initialIPs = append(initialIPs, ip)
	}
	val, loaded := state.Inner.DeviceAccess.LoadOrStore(key, &DeviceAccessRecord{
		PublicID:    id.New(),
		Username:    username,
		UserAgent:   userAgent,
		LoginMethod: loginMethod,
		CreatedAt:   now,
		LastActive:  now,
		IPs:         initialIPs,
	})
	if loaded {
		val.mu.Lock()
		val.LastActive = now
		if ip != "" {
			newIPs := make([]string, 0, 10)
			newIPs = append(newIPs, ip)
			for _, existing := range val.IPs {
				if existing != ip && len(newIPs) < 10 {
					newIPs = append(newIPs, existing)
				}
			}
			val.IPs = newIPs
		}
		val.mu.Unlock()
	}
}

// RemoveDeviceAccess drops in-memory tracked device access for a given public ID.
func (state *AppState) RemoveDeviceAccess(username, publicID string) {
	if state == nil || state.Inner == nil {
		return
	}
	username = strings.ToLower(strings.TrimSpace(username))
	state.Inner.DeviceAccess.Range(func(k string, rec *DeviceAccessRecord) bool {
		if rec.Username == username && rec.PublicID == publicID {
			state.Inner.DeviceAccess.Delete(k)
		}
		return true
	})
}

// ListUserSessions returns browser sessions and active device accesses for username.
// Basic Auth and API Token requests from the same device are merged, with up to 10 recent IPs.
func (state *AppState) ListUserSessions(username, currentSessionToken string) []SessionDto {
	if state == nil || state.Inner == nil || username == "" {
		return []SessionDto{}
	}
	lowerUser := strings.ToLower(strings.TrimSpace(username))
	var sessions []SessionDto
	if db := state.GetDB(); db != nil {
		dbSessions, err := db.ListUserSessions(username, currentSessionToken)
		if err == nil && dbSessions != nil {
			sessions = append(sessions, dbSessions...)
		}
	} else {
		state.Inner.Sessions.Range(func(key string, value *Session) bool {
			if value != nil && strings.EqualFold(value.Username, lowerUser) {
				sessions = append(sessions, sessionToDto(key, value, currentSessionToken))
			}
			return true
		})
	}
	for i := range sessions {
		if len(sessions[i].RecentIPs) == 0 && sessions[i].IP != "" {
			sessions[i].RecentIPs = []string{sessions[i].IP}
		}
	}

	now := time.Now().UnixMilli()
	state.Inner.DeviceAccess.Range(func(k string, rec *DeviceAccessRecord) bool {
		if rec.Username == lowerUser && (rec.LoginMethod == "basic" || rec.LoginMethod == "token") {
			rec.mu.Lock()
			lastActive := rec.LastActive
			var primaryIP string
			if len(rec.IPs) > 0 {
				primaryIP = rec.IPs[0]
			}
			if now-lastActive <= SessionIdleTimeoutMillis {
				sessions = append(sessions, SessionDto{
					PublicID:    rec.PublicID,
					Username:    rec.Username,
					IP:          primaryIP,
					UserAgent:   rec.UserAgent,
					CreatedAt:   rec.CreatedAt,
					LastActive:  lastActive,
					ExpiresAt:   lastActive + SessionIdleTimeoutMillis,
					Current:     false,
					LoginMethod: rec.LoginMethod,
					RecentIPs:   append([]string(nil), rec.IPs...),
				})
			}
			rec.mu.Unlock()
		}
		return true
	})

	return sessions
}

// RevokeUserSessionByPublicID removes one session owned by username, identified by public_id.
// Returns (revoked, wasCurrent) where wasCurrent means the revoked secret matched currentSessionToken.
func (state *AppState) RevokeUserSessionByPublicID(username, publicID, currentSessionToken string) (revoked bool, wasCurrent bool, err error) {
	if state == nil || state.Inner == nil || username == "" || publicID == "" {
		return false, false, nil
	}
	defer state.RemoveDeviceAccess(username, publicID)
	if db := state.GetDB(); db != nil {
		token, revoked, wasCurrent, err := db.DeleteUserSessionByPublicID(username, publicID, currentSessionToken)
		if err != nil {
			return false, false, err
		}
		if revoked && token != "" {
			state.DeleteAuthCache("Session " + token)
			state.Inner.Sessions.Delete(token)
		}
		return revoked, wasCurrent, nil
	}
	var toRemove string
	state.Inner.Sessions.Range(func(key string, value *Session) bool {
		if value != nil && value.Username == username && value.PublicID == publicID {
			toRemove = key
			return false
		}
		return true
	})
	if toRemove == "" {
		return false, false, nil
	}
	wasCurrent = currentSessionToken != "" && toRemove == currentSessionToken
	revoked, err = state.RevokeSession(toRemove)
	return revoked, wasCurrent, err
}

// RevokeOtherUserSessions removes every session for username except the one matching keepSessionToken.
// If keepSessionToken is empty, all sessions for the user are removed.
func (state *AppState) RevokeOtherUserSessions(username, keepSessionToken string) (int, error) {
	if state == nil || state.Inner == nil || username == "" {
		return 0, nil
	}
	if db := state.GetDB(); db != nil {
		deletedTokens, err := db.DeleteOtherUserSessions(username, keepSessionToken)
		if err != nil {
			return 0, err
		}
		toRemove := append([]string(nil), deletedTokens...)
		seen := make(map[string]struct{}, len(deletedTokens))
		for _, token := range deletedTokens {
			seen[token] = struct{}{}
		}
		state.Inner.Sessions.Range(func(token string, session *Session) bool {
			if session != nil && strings.EqualFold(session.Username, username) && token != keepSessionToken {
				if _, exists := seen[token]; !exists {
					toRemove = append(toRemove, token)
				}
			}
			return true
		})
		for _, t := range toRemove {
			state.DeleteAuthCache("Session " + t)
			state.Inner.Sessions.Delete(t)
		}
		return len(deletedTokens), nil
	}
	var toRemove []string
	state.Inner.Sessions.Range(func(key string, value *Session) bool {
		if value != nil && value.Username == username && key != keepSessionToken {
			toRemove = append(toRemove, key)
		}
		return true
	})
	count := 0
	for _, token := range toRemove {
		if revoked, err := state.RevokeSession(token); err != nil {
			return count, err
		} else if revoked {
			count++
		}
	}
	return count, nil
}

// RevokeAllUserSessions removes every browser session for username.
func (state *AppState) RevokeAllUserSessions(username string) (int, error) {
	return state.RevokeOtherUserSessions(username, "")
}

// ForgetUserSessions clears in-memory session copies after the database revoked them transactionally.
func (state *AppState) ForgetUserSessions(username string) {
	if state == nil || state.Inner == nil || username == "" {
		return
	}
	state.Inner.Sessions.Range(func(token string, session *Session) bool {
		if session != nil && strings.EqualFold(session.Username, username) {
			state.DeleteAuthCache("Session " + token)
			state.Inner.Sessions.Delete(token)
		}
		return true
	})
}
