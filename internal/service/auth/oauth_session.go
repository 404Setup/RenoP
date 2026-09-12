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
	"errors"
	"strings"

	"github.com/emmansun/base64"
	"github.com/goccy/go-json"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/utils/secretcipher"
)

// oauthSessionProof is server-owned material carried through MFA, then encrypted for one browser session.
type oauthSessionProof struct {
	ProviderID   string      `json:"provider"`
	UserID       string      `json:"user_id"`
	Authority    string      `json:"authority"`
	Subject      string      `json:"subject"`
	SessionID    string      `json:"sid"`
	AuthorizedAt int64       `json:"authorized_at"`
	Issuer       string      `json:"issuer"`
	ConfigHash   string      `json:"config_hash"`
	GitHubAPIURL string      `json:"github_api_url,omitempty"`
	Tokens       oauthTokens `json:"tokens"`
}

func githubRevocationAuthority(clientID, endpoint string) string {
	return registrationHash("github\x00" + clientID + "\x00" + strings.TrimRight(endpoint, "/"))
}

func githubSessionConfigurationHash(value config.GitHubOAuthConfig) string {
	data, _ := json.Marshal(value)
	return registrationHash(string(data))
}

func currentOAuthProofConfiguration(cfg *config.Config, proof *oauthSessionProof) bool {
	if proof.ProviderID == "github" {
		return cfg.Server.GitHubOAuth.Configured() && githubSessionConfigurationHash(cfg.Server.GitHubOAuth) == proof.ConfigHash &&
			githubRevocationAuthority(cfg.Server.GitHubOAuth.ClientID, proof.GitHubAPIURL) == proof.Authority
	}
	p, ok := cfg.Server.OAuthProvider(proof.ProviderID)
	return ok && oauthConfigurationHash(p) == proof.ConfigHash && p.Authority(proof.Issuer) == proof.Authority
}

func sealSessionOAuthGrant(state *core.AppState, proof *oauthSessionProof, publicID string) (*core.SessionOAuthGrant, error) {
	if proof == nil || proof.UserID == "" || !currentOAuthProofConfiguration(state.Inner.Config.Load(), proof) {
		return nil, core.ErrMFAInvalid
	}
	value := *proof
	value.Tokens.IDToken, value.Tokens.Error = "", nil
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	cipher, err := secretcipher.New(state.Inner.Config.Load().MFAEncryptionKey)
	if err != nil {
		return nil, err
	}
	sealed := cipher.Seal(nil, nil, payload, []byte("oauth-session:"+publicID))
	return &core.SessionOAuthGrant{ProviderID: proof.ProviderID, Authority: proof.Authority, Subject: proof.Subject,
		SessionID: proof.SessionID, AuthorizedAt: proof.AuthorizedAt, EncryptedTokens: base64.RawStdEncoding.EncodeToString(sealed)}, nil
}

func openSessionOAuthGrant(state *core.AppState, grant *core.SessionOAuthGrant, publicID string) (*oauthSessionProof, error) {
	invalid := errors.New("provider session authorization is unavailable")
	if grant == nil || len(grant.EncryptedTokens) > 60000 {
		return nil, invalid
	}
	sealed, err := base64.RawStdEncoding.DecodeString(grant.EncryptedTokens)
	if err != nil {
		return nil, invalid
	}
	cipher, err := secretcipher.New(state.Inner.Config.Load().MFAEncryptionKey)
	if err != nil {
		return nil, invalid
	}
	payload, err := cipher.Open(nil, nil, sealed, []byte("oauth-session:"+publicID))
	if err != nil {
		return nil, invalid
	}
	var proof oauthSessionProof
	if json.Unmarshal(payload, &proof) != nil || proof.ProviderID != grant.ProviderID || proof.Authority != grant.Authority ||
		proof.Subject != grant.Subject || proof.SessionID != grant.SessionID || proof.AuthorizedAt != grant.AuthorizedAt {
		return nil, invalid
	}
	return &proof, nil
}
