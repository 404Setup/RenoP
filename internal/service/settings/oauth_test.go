/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package settings

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"renop/internal/config"

	"renop/internal/utils/protohttp"
	"renop/pkg/pb"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestOAuthSettingsKeepCredentialsPrivate(t *testing.T) {
	cfg := config.DefaultConfig()
	app, state := setupSettingsTestApp(t, cfg)
	p := config.OAuthProviderConfig{ID: "stack", Type: "stackexchange", Name: "Stack Exchange", ClientID: "client", ClientSecret: "private-secret",
		APIKey: "private-key", RevocationSecret: strings.Repeat("callback-secret-", 3), Enabled: true, CallbackURL: "https://renop.example/api/auth/oauth/stack/callback"}
	put := func(providers []oauthProviderSettings) *http.Response {
		data, err := proto.Marshal(oauthTestPayload(providers))
		require.NoError(t, err)
		request := httptest.NewRequest("PUT", "/oauth-providers", bytes.NewReader(data))
		request.Header.Set("Content-Type", protohttp.ContentType)
		response, err := app.Test(request)
		require.NoError(t, err)
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	response := put([]oauthProviderSettings{{OAuthProviderConfig: p}})
	require.Equal(t, 200, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "private-secret")
	require.NotContains(t, string(body), "private-key")
	require.NotContains(t, string(body), p.RevocationSecret)
	var saved pb.OAuthSettings
	require.NoError(t, proto.Unmarshal(body, &saved))
	var stackProvider *pb.OAuthProviderSettings
	for _, prov := range saved.Providers {
		if prov.Id == "stack" {
			stackProvider = prov
			break
		}
	}
	require.NotNil(t, stackProvider)
	require.True(t, stackProvider.ApiKeyConfigured)
	require.Len(t, state.Inner.Config.Load().Server.OAuthProviders, 1)
	for _, invalid := range []string{"", "\x1a\x00"} {
		request := httptest.NewRequest("PUT", "/oauth-providers", strings.NewReader(invalid))
		request.Header.Set("Content-Type", protohttp.ContentType)
		rejected, err := app.Test(request)
		require.NoError(t, err)
		require.NoError(t, rejected.Body.Close())
		require.Equal(t, 400, rejected.StatusCode, "a missing provider list must not clear saved clients")
		require.Len(t, state.Inner.Config.Load().Server.OAuthProviders, 1)
	}
	savedSecret := p.RevocationSecret
	p.ClientSecret, p.APIKey, p.RevocationSecret = "", "", ""
	response = put([]oauthProviderSettings{{OAuthProviderConfig: p}})
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, "private-secret", state.Inner.Config.Load().Server.OAuthProviders[0].ClientSecret)
	require.Equal(t, "private-key", state.Inner.Config.Load().Server.OAuthProviders[0].APIKey)
	require.Equal(t, savedSecret, state.Inner.Config.Load().Server.OAuthProviders[0].RevocationSecret)
	response = put([]oauthProviderSettings{{OAuthProviderConfig: p}, {OAuthProviderConfig: p}})
	require.Equal(t, 400, response.StatusCode)
	p.Type = "custom"
	p.AuthorizeURL, p.TokenURL, p.UserInfoURL, p.Claims.Subject = "https://other.example/auth", "https://other.example/token", "https://other.example/me", "id"
	response = put([]oauthProviderSettings{{OAuthProviderConfig: p}})
	require.Equal(t, 400, response.StatusCode, "changing provider must not send saved credentials to another service")
	p.Type, p.Enabled = "stackexchange", false
	response = put([]oauthProviderSettings{{OAuthProviderConfig: p, ClearClientSecret: true, ClearAPIKey: true}})
	require.Equal(t, 200, response.StatusCode)
	require.Empty(t, state.Inner.Config.Load().Server.OAuthProviders[0].ClientSecret)
	require.Empty(t, state.Inner.Config.Load().Server.OAuthProviders[0].APIKey)
	response = put([]oauthProviderSettings{})
	require.Equal(t, 200, response.StatusCode)
	require.Empty(t, state.Inner.Config.Load().Server.OAuthProviders)
}

func TestUnifiedOAuthSettingsPreserveLegacyGitHub(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.GitHubOAuth = config.GitHubOAuthConfig{Enabled: true, ClientID: "legacy-client", ClientSecret: "legacy-secret", CallbackURL: "https://renop.example/api/auth/github/callback"}
	app, state := setupSettingsTestApp(t, cfg)
	response, err := app.Test(httptest.NewRequest("GET", "/oauth-providers", nil))
	require.NoError(t, err)
	var payload pb.OAuthSettings
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(body, &payload))
	view := struct{ Providers []oauthProviderSettings }{}
	for _, value := range payload.Providers {
		view.Providers = append(view.Providers, oauthProviderRequest(value))
	}
	require.NoError(t, response.Body.Close())
	require.Len(t, view.Providers, 1)
	github := view.Providers[0]
	require.Equal(t, "github", github.ID)
	require.True(t, github.ClientSecretConfigured)
	require.Empty(t, github.ClientSecret)
	put := func(providers []oauthProviderSettings, status int) {
		t.Helper()
		data, err := proto.Marshal(oauthTestPayload(providers))
		require.NoError(t, err)
		request := httptest.NewRequest("PUT", "/oauth-providers", bytes.NewReader(data))
		request.Header.Set("Content-Type", protohttp.ContentType)
		response, err := app.Test(request)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, status, response.StatusCode)
	}
	put(view.Providers, 200)
	require.Equal(t, cfg.Server.GitHubOAuth, state.Inner.Config.Load().Server.GitHubOAuth)
	require.Empty(t, state.Inner.Config.Load().Server.OAuthProviders)
	put([]oauthProviderSettings{}, 200)
	require.True(t, state.Inner.Config.Load().Server.GitHubOAuth.Configured(), "legacy clients omitting GitHub must preserve it")
	put([]oauthProviderSettings{github, github}, 400)
	github.ClientID = "other-client"
	put([]oauthProviderSettings{github}, 400)
	require.Equal(t, "legacy-secret", state.Inner.Config.Load().Server.GitHubOAuth.ClientSecret)
	github.Enabled, github.ClearClientSecret = false, true
	put([]oauthProviderSettings{github}, 200)
	require.Empty(t, state.Inner.Config.Load().Server.GitHubOAuth.ClientSecret)
	require.False(t, state.Inner.Config.Load().Server.GitHubOAuth.Enabled)
}

func oauthTestPayload(values []oauthProviderSettings) *pb.OAuthSettings {
	request := &pb.OAuthSettings{ReplaceProviders: true}
	for _, value := range values {
		request.Providers = append(request.Providers, oauthProviderMessage(value))
	}
	return request
}

func TestOAuthSettingsValidationAndLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	app, state := setupSettingsTestApp(t, cfg)
	put := func(providers []oauthProviderSettings) int {
		data, err := proto.Marshal(oauthTestPayload(providers))
		require.NoError(t, err)
		request := httptest.NewRequest("PUT", "/oauth-providers", bytes.NewReader(data))
		request.Header.Set("Content-Type", protohttp.ContentType)
		response, err := app.Test(request)
		require.NoError(t, err)
		defer response.Body.Close()
		return response.StatusCode
	}

	// 1. Check GET returns github in presets
	getResp, err := app.Test(httptest.NewRequest("GET", "/oauth-providers", nil))
	require.NoError(t, err)
	defer getResp.Body.Close()
	body, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	var getPayload pb.OAuthSettings
	require.NoError(t, proto.Unmarshal(body, &getPayload))
	hasGitHubPreset := false
	for _, preset := range getPayload.Presets {
		if preset.Type == "github" {
			hasGitHubPreset = true
			break
		}
	}
	require.True(t, hasGitHubPreset, "presets should include github")

	// 2. Reject duplicate non-custom provider types (e.g., two google providers with different IDs)
	google1 := oauthProviderSettings{
		ID: "google1", Type: "google", Name: "Google 1"}
	google2 := oauthProviderSettings{
		ID: "google2", Type: "google", Name: "Google 2"}
	require.Equal(t, 400, put([]oauthProviderSettings{google1, google2}), "duplicate google providers must be rejected")

	// 3. Reject duplicate endpoints for custom OAuth 2.0
	custom1 := oauthProviderSettings{
		ID: "custom1", Type: "custom", Name: "Custom 1",
		AuthorizeURL: "https://auth.example.com/oauth/authorize",
		TokenURL:     "https://auth.example.com/oauth/token",
		UserInfoURL:  "https://auth.example.com/oauth/userinfo"}
	custom2 := oauthProviderSettings{
		ID: "custom2", Type: "custom", Name: "Custom 2",
		AuthorizeURL: "https://auth.example.com/oauth/authorize/", // trailing slash difference
		TokenURL:     "https://auth.example.com/oauth/token",
		UserInfoURL:  "https://auth.example.com/oauth/userinfo"}
	require.Equal(t, 400, put([]oauthProviderSettings{custom1, custom2}), "duplicate custom endpoints must be rejected")

	// Also reject two custom providers with same AuthorizeURL and TokenURL
	custom3 := oauthProviderSettings{
		ID: "custom3", Type: "custom", Name: "Custom 3",
		AuthorizeURL: "https://auth.example.com/oauth/authorize",
		TokenURL:     "https://auth.example.com/oauth/token",
		UserInfoURL:  "https://auth.example.com/oauth/other-userinfo"}
	require.Equal(t, 400, put([]oauthProviderSettings{custom1, custom3}), "duplicate custom authorize and token endpoints must be rejected")

	// 4. Accept multiple custom OAuth 2.0 providers with distinct endpoints
	customDistinct := oauthProviderSettings{
		ID: "custom2", Type: "custom", Name: "Custom 2",
		AuthorizeURL: "https://auth2.example.com/oauth/authorize",
		TokenURL:     "https://auth2.example.com/oauth/token",
		UserInfoURL:  "https://auth2.example.com/oauth/userinfo"}
	require.Equal(t, 200, put([]oauthProviderSettings{custom1, customDistinct}), "distinct custom endpoints must be accepted")
	require.Len(t, state.Inner.Config.Load().Server.OAuthProviders, 2)

	// 5. Enforce 12 provider limit (12 providers accepted, 13 rejected)
	twelveProviders := make([]oauthProviderSettings, 12)
	for i := range 12 {
		twelveProviders[i] = oauthProviderSettings{
			ID:           "c" + string(rune('a'+i)),
			Type:         "custom",
			Name:         "Custom " + string(rune('a'+i)),
			AuthorizeURL: "https://auth" + string(rune('a'+i)) + ".example.com/oauth/authorize",
			TokenURL:     "https://auth" + string(rune('a'+i)) + ".example.com/oauth/token",
			UserInfoURL:  "https://auth" + string(rune('a'+i)) + ".example.com/oauth/userinfo"}
	}
	require.Equal(t, 200, put(twelveProviders), "12 providers must be accepted")
	require.Len(t, state.Inner.Config.Load().Server.OAuthProviders, 12)

	thirteenProviders := append(twelveProviders, oauthProviderSettings{
		ID:           "c_extra",
		Type:         "custom",
		Name:         "Custom Extra",
		AuthorizeURL: "https://extra.example.com/oauth/authorize",
		TokenURL:     "https://extra.example.com/oauth/token",
		UserInfoURL:  "https://extra.example.com/oauth/userinfo"})
	require.Equal(t, 400, put(thirteenProviders), "more than 12 providers must be rejected")
}
