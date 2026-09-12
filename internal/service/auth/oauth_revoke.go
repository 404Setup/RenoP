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
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"renop/internal/config"
	"renop/internal/core"
)

var providerRevocationSlots = make(chan struct{}, 8)

// revokeProviderTokens transmits only server-held credentials to their original configured authority.
func revokeProviderTokens(ctx context.Context, cfg *config.Config, proof *oauthSessionProof) string {
	select {
	case providerRevocationSlots <- struct{}{}:
		defer func() { <-providerRevocationSlots }()
	default:
		return "failed"
	}
	if proof == nil || !currentOAuthProofConfiguration(cfg, proof) {
		return "unavailable"
	}
	client, err := oauthHTTPClient(cfg)
	if err != nil {
		return "failed"
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if proof.ProviderID == "github" {
		payload, _ := json.Marshal(map[string]string{"access_token": proof.Tokens.AccessToken})
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(proof.GitHubAPIURL, "/")+"/applications/"+url.PathEscape(cfg.Server.GitHubOAuth.ClientID)+"/token", bytes.NewReader(payload))
		if err != nil {
			return "failed"
		}
		req.SetBasicAuth(cfg.Server.GitHubOAuth.ClientID, cfg.Server.GitHubOAuth.ClientSecret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/vnd.github+json")
		return sendProviderRevocation(client, req)
	}
	p, _ := cfg.Server.OAuthProvider(proof.ProviderID)
	if p.Type == "stackexchange" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.stackexchange.com/2.3/access-tokens/"+url.PathEscape(proof.Tokens.AccessToken)+"/invalidate", nil)
		if err != nil {
			return "failed"
		}
		return sendProviderRevocation(client, req)
	}
	if p.RevocationURL == "" {
		return "unsupported"
	}
	// Revoking a refresh token need not revoke access tokens on every provider, so submit both when present.
	for _, item := range []struct{ value, hint string }{{proof.Tokens.RefreshToken, "refresh_token"}, {proof.Tokens.AccessToken, "access_token"}} {
		if item.value == "" {
			continue
		}
		form := url.Values{"token": {item.value}, "token_type_hint": {item.hint}}
		if p.Type != "google" && p.TokenAuth != "client_secret_basic" {
			form.Set("client_id", p.ClientID)
			if p.TokenAuth != "none" {
				form.Set("client_secret", p.ClientSecret)
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.RevocationURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "failed"
		}
		if p.Type != "google" && p.TokenAuth == "client_secret_basic" {
			req.SetBasicAuth(url.QueryEscape(p.ClientID), url.QueryEscape(p.ClientSecret))
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if status := sendProviderRevocation(client, req); status != "revoked" {
			return status
		}
	}
	return "revoked"
}

func sendProviderRevocation(client *http.Client, req *http.Request) string {
	req.Header.Set("User-Agent", "RenoP-OAuth/1")
	response, err := client.Do(req)
	if err != nil {
		return "failed"
	}
	defer response.Body.Close()
	if response.ContentLength > 65536 {
		return "failed"
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(body) > 65536 {
		return "failed"
	}
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusNoContent {
		return "revoked"
	}
	if response.StatusCode == http.StatusBadRequest && req.URL.Hostname() == "oauth2.googleapis.com" {
		var result struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &result) == nil && result.Error == "invalid_token" {
			return "revoked"
		}
	}
	return "failed"
}

func logoutOAuthProof(state *core.AppState, session *core.Session, token string) (*oauthSessionProof, string) {
	if session == nil {
		return nil, ""
	}
	method, _, _ := strings.Cut(session.LoginMethod, "+")
	provider := strings.TrimPrefix(method, "oauth:")
	if method != "github" && provider == method {
		return nil, ""
	}
	if state.GetDB() == nil {
		return nil, provider
	}
	grant, err := state.GetDB().GetSessionOAuthGrant(token)
	if err != nil || grant == nil {
		return nil, provider
	}
	proof, err := openSessionOAuthGrant(state, grant, session.PublicID)
	if err != nil || proof.ProviderID != provider {
		return nil, provider
	}
	return proof, provider
}
