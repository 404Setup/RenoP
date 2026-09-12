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
	"net/url"
	"strings"

	"renop/internal/config"
	"renop/internal/core"
)

// userProfileLinks derives URLs on each profile read so unlinking and provider reconfiguration take effect immediately.
func userProfileLinks(state *core.AppState, profile *core.UserProfile, own bool) (core.UserProfileLinks, error) {
	links := profile.Links
	links.GitHub, links.Providers = "", nil
	visibility := links.Visibility
	if !own {
		links.Visibility = nil
	}
	if visibility == nil {
		return links, nil
	}
	if visibility.GitHub {
		identity, err := state.GetDB().GetGitHubIdentity(profile.Username)
		if err != nil {
			return links, err
		}
		if identity != nil {
			links.GitHub = providerProfileURL("https://github.com", identity.GitHubLogin)
		}
	}
	if !visibility.GitLab {
		return links, nil
	}
	providers := make(map[string]config.OAuthProviderConfig)
	for _, candidate := range state.Inner.Config.Load().Server.OAuthProviders {
		if candidate.Type == "gitlab" {
			providers[candidate.ID] = candidate.Resolved()
		}
	}
	if len(providers) == 0 {
		return links, nil
	}
	identities, err := state.GetDB().GetOAuthIdentities(profile.Username)
	if err != nil {
		return links, err
	}
	for _, identity := range identities {
		provider, ok := providers[identity.ProviderID]
		if !ok || identity.Authority != provider.Authority(provider.Issuer) {
			continue
		}
		if href := providerProfileURL(provider.BaseURL, identity.Login); href != "" {
			name := provider.Name
			if name == "" {
				name = "GitLab"
			}
			links.Providers = append(links.Providers, core.ProviderProfileLink{Name: name, URL: href})
		}
	}
	return links, nil
}

func providerProfileURL(base, login string) string {
	if !config.ValidOAuthURL(base) || login == "" || len(login) > 255 || login == "." || login == ".." {
		return ""
	}
	for _, ch := range login {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("_-.", ch)) {
			return ""
		}
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.RawQuery != "" {
		return ""
	}
	return parsed.JoinPath(login).String()
}
