/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/testutil"
)

func TestProfileProviderLinksRequireBindingAuthorityAndConsent(t *testing.T) {
	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "links.db"), MaxOpenConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	state, cfg := core.NewAppState(), config.DefaultConfig()
	state.Inner.DB = db
	provider := config.OAuthProviderConfig{ID: "work", Type: "gitlab", Name: "Work GitLab", ClientID: "client", BaseURL: "https://git.example/gitlab"}.Resolved()
	cfg.Server.OAuthProviders = []config.OAuthProviderConfig{provider}
	state.Inner.Config.Store(cfg)
	now := time.Now().UnixMilli()
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "alice", EncryptedSecret: "configured-password"}))
	session := &core.Session{PublicID: "links", Username: "alice", CreatedAt: now}
	session.LastActive.Store(now)
	require.NoError(t, db.SaveSession(session, "links-session"))
	profile, err := db.GetUserProfile("alice")
	require.NoError(t, err)
	require.NoError(t, db.StoreGitHubIdentity(profile.UserID, 42, "alice-code", []core.GitHubPrincipal{
		{Type: core.GitHubPrincipalUser, GitHubID: 42, Login: "alice-code", AuthorizedAt: now},
	}, now))
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	identity := core.OAuthIdentity{ProviderID: provider.ID, Subject: "12", Authority: provider.Authority(provider.Issuer), Login: "alice.dev"}
	require.NoError(t, db.LinkOAuthIdentity("alice", "links-session", mfa.Snapshot, identity, now))
	links, err := userProfileLinks(state, profile, false)
	require.NoError(t, err)
	require.Empty(t, links.GitHub)
	require.Empty(t, links.Providers)
	require.Nil(t, links.Visibility)
	_, err = db.UpdateUserProfileLinks("alice", core.UserProfileLinks{GitHub: "https://github.com/impostor"}, now)
	require.Error(t, err)
	profile, err = db.UpdateUserProfileLinks("alice", core.UserProfileLinks{Visibility: &core.ProfileLinkVisibility{GitHub: true, GitLab: true}}, now)
	require.NoError(t, err)
	links, err = userProfileLinks(state, profile, false)
	require.NoError(t, err)
	require.Equal(t, "https://github.com/alice-code", links.GitHub)
	require.Equal(t, []core.ProviderProfileLink{{Name: "Work GitLab", URL: "https://git.example/gitlab/alice.dev"}}, links.Providers)
	require.Nil(t, links.Visibility)
	own, err := userProfileLinks(state, profile, true)
	require.NoError(t, err)
	require.True(t, own.Visibility.GitHub)
	require.True(t, own.Visibility.GitLab)

	reconfigured := cfg.DeepCopy()
	reconfigured.Server.OAuthProviders[0].ClientID = "replacement-client"
	state.Inner.Config.Store(reconfigured)
	links, err = userProfileLinks(state, profile, false)
	require.NoError(t, err)
	require.Empty(t, links.Providers, "an old binding must not become a link to a replacement authority")
	state.Inner.Config.Store(cfg)
	require.NoError(t, db.DeleteOAuthIdentity("alice", "links-session", provider.ID, now+1))
	links, err = userProfileLinks(state, profile, false)
	require.NoError(t, err)
	require.Empty(t, links.Providers, "unlinking must take effect without refreshing a profile cache")
	profile, err = db.UpdateUserProfileLinks("alice", core.UserProfileLinks{}, now+2)
	require.NoError(t, err)
	links, err = userProfileLinks(state, profile, false)
	require.NoError(t, err)
	require.Empty(t, links.GitHub)
}

func TestProviderProfileURLRejectsInjectedPaths(t *testing.T) {
	for _, login := range []string{"", ".", "..", "a/b", "a?tab=1", "a#section", "a\\b", "%2e%2e", "a\n"} {
		require.Empty(t, providerProfileURL("https://git.example", login), login)
	}
	require.Empty(t, providerProfileURL("javascript:alert(1)", "alice"))
	require.Equal(t, "https://git.example/subpath/alice-dev", providerProfileURL("https://git.example/subpath", "alice-dev"))
}
