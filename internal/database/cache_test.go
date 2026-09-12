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
	"renop/internal/config"
	"renop/internal/core"
	"testing"
	"time"
)

func TestDBEvictExpiredCaches(t *testing.T) {
	db := &DB{
		tokenCache:       NewTTLCache[string, *core.AccessToken](time.Minute),
		tokenSecretCache: NewTTLCache[string, *core.AccessToken](time.Minute),
		sessionCache:     NewTTLCache[string, *core.Session](time.Minute),
		userIDCache:      NewTTLCache[string, string](time.Minute),
		profileCache:     NewTTLCache[string, core.UserProfile](time.Minute),
	}
	db.tokenCache.Set("token", &core.AccessToken{}, time.Nanosecond)
	db.tokenSecretCache.Set("secret", &core.AccessToken{}, time.Nanosecond)
	db.sessionCache.Set("session", &core.Session{}, time.Nanosecond)
	db.userIDCache.Set("user", "id", time.Nanosecond)
	db.profileCache.Set("profile", core.UserProfile{UserID: "id"}, time.Nanosecond)
	db.EvictExpiredCaches()
	if db.tokenCache.Len() != 0 || db.tokenSecretCache.Len() != 0 ||
		db.sessionCache.Len() != 0 || db.userIDCache.Len() != 0 ||
		db.profileCache.Len() != 0 {
		t.Fatal("database cache eviction left expired entries")
	}
}

func TestUserIdentityAndProfileSummaryCaches(t *testing.T) {
	db, err := InitDB(config.DatabaseConfig{
		Driver: "sqlite3", Dsn: "file:user-cache-test?mode=memory&cache=shared", MaxOpenConns: 2, MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("initialize cache test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close cache test database: %v", err)
		}
	})
	now := time.Now().UnixMilli()
	if err := db.SaveToken(&core.AccessToken{Name: "alice", CreatedAt: "2026-08-28T00:00:00Z"}); err != nil {
		t.Fatalf("save cache test account: %v", err)
	}
	db.userIDCache = NewTTLCacheWithCapacity[string, string](time.Minute, 64)
	firstID, err := db.userIDForUsername("alice")
	if err != nil {
		t.Fatalf("load uncached user ID: %v", err)
	}
	secondID, err := db.userIDForUsername("alice")
	if err != nil || secondID != firstID {
		t.Fatalf("load cached user ID: id=%q err=%v", secondID, err)
	}
	hits, misses := db.userIDCache.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("identity cache stats = %d hits, %d misses; want 1 and 1", hits, misses)
	}

	db.profileCache = NewTTLCacheWithCapacity[string, core.UserProfile](time.Minute, 64)
	for range 2 {
		profiles, err := db.GetUserProfiles([]string{"alice"})
		if err != nil || profiles["alice"] == nil {
			t.Fatalf("load profile summary: profile=%v err=%v", profiles["alice"], err)
		}
	}
	hits, misses = db.profileCache.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("profile cache stats = %d hits, %d misses; want 1 and 1", hits, misses)
	}

	if profiles, err := db.GetUserProfiles([]string{"bobby"}); err != nil || len(profiles) != 0 {
		t.Fatalf("prime missing profile cache: profiles=%v err=%v", profiles, err)
	}
	if err := db.CreateToken(&core.AccessToken{Name: "bobby", CreatedAt: "2026-08-28T00:00:00Z"}, "Bobby", now); err != nil {
		t.Fatalf("create account over negative cache: %v", err)
	}
	profiles, err := db.GetUserProfiles([]string{"bobby"})
	if err != nil || profiles["bobby"] == nil || profiles["bobby"].Nickname != "Bobby" {
		t.Fatalf("new account did not replace negative cache: profile=%v err=%v", profiles["bobby"], err)
	}

	account, err := db.GetTokenByName("alice")
	if err != nil {
		t.Fatalf("load account before rename: %v", err)
	}
	if _, err := db.UpdateUserProfile("alice", "alice_renamed", "Alice", account, now+1,
		core.AccountTokenChanges{}); err != nil {
		t.Fatalf("rename account with populated caches: %v", err)
	}
	profiles, err = db.GetUserProfiles([]string{"alice", "alice_renamed"})
	if err != nil || profiles["alice"] != nil || profiles["alice_renamed"] == nil {
		t.Fatalf("renamed profile cache is stale: profiles=%v err=%v", profiles, err)
	}
}
