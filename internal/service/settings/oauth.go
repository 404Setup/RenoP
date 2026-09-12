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
	"errors"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"

	"github.com/gofiber/fiber/v3"
)

type oauthProviderSettings struct {
	config.OAuthProviderConfig
	ClientSecretConfigured     bool `json:"client_secret_configured"`
	APIKeyConfigured           bool `json:"api_key_configured"`
	ClearClientSecret          bool `json:"clear_client_secret,omitempty"`
	ClearAPIKey                bool `json:"clear_api_key,omitempty"`
	RevocationSecretConfigured bool `json:"revocation_secret_configured"`
	ClearRevocationSecret      bool `json:"clear_revocation_secret,omitempty"`
}

func oauthSettings(server config.ServerConfig) *pb.OAuthSettings {
	github := server.GitHubOAuth
	var values []oauthProviderSettings
	if github.Configured() || strings.TrimSpace(github.ClientID) != "" || github.Enabled {
		values = append(values, oauthProviderSettings{
			ID: "github", Type: "github", Name: "GitHub", Enabled: github.Enabled,
			ClientID: github.ClientID, CallbackURL: github.CallbackURL, ClientSecretConfigured: github.ClientSecret != "", RevocationSecretConfigured: github.RevocationSecret != "",
		})
	}
	for _, p := range server.OAuthProviders {
		value := oauthProviderSettings{OAuthProviderConfig: p.Resolved(), ClientSecretConfigured: p.ClientSecret != "", APIKeyConfigured: p.APIKey != ""}
		value.RevocationSecretConfigured = p.RevocationSecret != ""
		value.ClientSecret, value.APIKey, value.RevocationSecret = "", "", ""
		values = append(values, value)
	}
	var presets []config.OAuthProviderConfig
	for _, item := range [][2]string{{"github", "GitHub"}, {"microsoft", "Microsoft"}, {"google", "Google"}, {"gitlab", "GitLab"},
		{"cloudflare", "Cloudflare"}, {"stackexchange", "Stack Exchange"}, {"custom", "OAuth 2.0"}} {
		presets = append(presets, (config.OAuthProviderConfig{ID: item[0], Type: item[0], Name: item[1]}).Resolved())
	}
	response := &pb.OAuthSettings{}
	for _, value := range values {
		response.Providers = append(response.Providers, oauthProviderMessage(value))
	}
	for _, value := range presets {
		response.Presets = append(response.Presets, oauthProviderMessage(oauthProviderSettings{OAuthProviderConfig: value}))
	}
	return response
}

func getOAuthSettings(c fiber.Ctx, state *core.AppState) error {
	if !isManager(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return protohttp.Write(c, oauthSettings(state.Inner.Config.Load().Server))
}

func normalizeOAuthSettings(current []config.OAuthProviderConfig, request []oauthProviderSettings) ([]config.OAuthProviderConfig, error) {
	invalid := errors.New("OAuth settings are invalid")
	if len(request) > 12 {
		return nil, invalid
	}
	next := make([]config.OAuthProviderConfig, 0, len(request))
	seen := map[string]bool{}
	seenTypes := map[string]bool{}
	seenCustomEndpoints := map[string]bool{}
	seenCoreEndpoints := map[string]bool{}
	for _, value := range request {
		p := value.OAuthProviderConfig
		p.ID, p.Type, p.Name = strings.TrimSpace(p.ID), strings.TrimSpace(p.Type), strings.TrimSpace(p.Name)
		p.ClientID = strings.TrimSpace(p.ClientID)
		p.ClientSecret = strings.TrimSpace(p.ClientSecret)
		p.APIKey = strings.TrimSpace(p.APIKey)
		p.CallbackURL = strings.TrimSpace(p.CallbackURL)
		p.BaseURL = strings.TrimSpace(p.BaseURL)
		p.TokenAuth = strings.TrimSpace(p.TokenAuth)
		if seen[p.ID] {
			return nil, invalid
		}
		seen[p.ID] = true
		if p.Type != "custom" {
			if seenTypes[p.Type] {
				return nil, invalid
			}
			seenTypes[p.Type] = true
		} else {
			resolved := p.Resolved()
			auth := strings.ToLower(strings.TrimRight(strings.TrimSpace(resolved.AuthorizeURL), "/"))
			token := strings.ToLower(strings.TrimRight(strings.TrimSpace(resolved.TokenURL), "/"))
			userinfo := strings.ToLower(strings.TrimRight(strings.TrimSpace(resolved.UserInfoURL), "/"))
			endpointKey := auth + "|" + token + "|" + userinfo
			if seenCustomEndpoints[endpointKey] {
				return nil, invalid
			}
			seenCustomEndpoints[endpointKey] = true
			if auth != "" && token != "" {
				coreKey := auth + "|" + token
				if seenCoreEndpoints[coreKey] {
					return nil, invalid
				}
				seenCoreEndpoints[coreKey] = true
			}
		}
		for _, old := range current {
			if old.ID != p.ID || old.Type != p.Type || strings.TrimSpace(old.ClientID) != p.ClientID || old.Resolved().TokenURL != p.Resolved().TokenURL {
				continue
			}
			if p.ClientSecret == "" && !value.ClearClientSecret {
				p.ClientSecret = strings.TrimSpace(old.ClientSecret)
			}
			if p.APIKey == "" && !value.ClearAPIKey {
				p.APIKey = strings.TrimSpace(old.APIKey)
			}
			if p.RevocationSecret == "" && !value.ClearRevocationSecret {
				p.RevocationSecret = old.RevocationSecret
			}
		}
		if value.ClearClientSecret {
			p.ClientSecret = ""
		}
		if value.ClearAPIKey {
			p.APIKey = ""
		}
		if value.ClearRevocationSecret {
			p.RevocationSecret = ""
		}
		if err := p.Validate(); err != nil {
			return nil, invalid
		}
		next = append(next, p)
	}
	return next, nil
}

func putOAuthSettings(c fiber.Ctx, state *core.AppState) error {
	if !isManager(c) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	var payload pb.OAuthSettings
	if err := protohttp.ReadLimit(c, &payload, 128<<10); err != nil {
		if errors.Is(err, fiber.ErrRequestEntityTooLarge) || errors.Is(err, fiber.ErrUnsupportedMediaType) {
			return err
		}
		return cacheSettingsError(c, 400, "oauth_settings_invalid")
	}
	if !payload.ReplaceProviders || len(payload.Providers) > 13 {
		return cacheSettingsError(c, 400, "oauth_settings_invalid")
	}
	request := struct{ Providers []oauthProviderSettings }{}
	for _, value := range payload.Providers {
		request.Providers = append(request.Providers, oauthProviderRequest(value))
	}
	state.Inner.ConfigWriteLock.Lock()
	defer state.Inner.ConfigWriteLock.Unlock()
	current := state.Inner.Config.Load()
	github := current.Server.GitHubOAuth
	providersRequest := make([]oauthProviderSettings, 0, len(request.Providers))
	seenGitHub := false
	for _, value := range request.Providers {
		if value.Type != "github" {
			providersRequest = append(providersRequest, value)
			continue
		}
		if seenGitHub || value.ID != "github" {
			return cacheSettingsError(c, 400, "oauth_settings_invalid")
		}
		seenGitHub = true
		var err error
		github, err = normalizeGitHubOAuthSettings(github, githubOAuthSettingsRequest{
			ClientID: value.ClientID, ClientSecret: value.ClientSecret, CallbackURL: value.CallbackURL,
			Enabled: value.Enabled, ClearClientSecret: value.ClearClientSecret,
			RevocationSecret: value.RevocationSecret, ClearRevocationSecret: value.ClearRevocationSecret,
		})
		if err != nil {
			return cacheSettingsError(c, 400, "oauth_settings_invalid")
		}
	}
	providers, err := normalizeOAuthSettings(current.Server.OAuthProviders, providersRequest)
	if err != nil {
		return cacheSettingsError(c, 400, "oauth_settings_invalid")
	}
	next := current.DeepCopy()
	next.Server.OAuthProviders = providers
	next.Server.GitHubOAuth = github
	if persistConfigSnapshot(next) != nil {
		return cacheSettingsError(c, 500, "oauth_settings_save_failed")
	}
	state.Inner.Config.Store(next)
	username, operator, method, sessionID, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{Username: username, Operator: operator, AuthMethod: method,
		SessionID: sessionID, IP: ip, Action: audit.ActionSettingsUpdate, Details: "Updated OAuth providers"})
	c.Set(fiber.HeaderCacheControl, "no-store")
	return protohttp.Write(c, oauthSettings(next.Server))
}
