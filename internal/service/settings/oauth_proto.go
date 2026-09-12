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
	"renop/internal/config"
	"renop/pkg/pb"
)

func oauthProviderMessage(value oauthProviderSettings) *pb.OAuthProviderSettings {
	return &pb.OAuthProviderSettings{
		Id:                         value.ID,
		Type:                       value.Type,
		Name:                       value.Name,
		Enabled:                    value.Enabled,
		ClientId:                   value.ClientID,
		ClientSecret:               value.ClientSecret,
		CallbackUrl:                value.CallbackURL,
		Tenant:                     value.Tenant,
		BaseUrl:                    value.BaseURL,
		Site:                       value.Site,
		ApiKey:                     value.APIKey,
		AuthorizeUrl:               value.AuthorizeURL,
		TokenUrl:                   value.TokenURL,
		UserinfoUrl:                value.UserInfoURL,
		Issuer:                     value.Issuer,
		JwksUrl:                    value.JWKSURL,
		Scopes:                     value.Scopes,
		TokenAuth:                  value.TokenAuth,
		DisablePkce:                value.DisablePKCE,
		RevocationUrl:              value.RevocationURL,
		RevocationSecret:           value.RevocationSecret,
		ClientSecretConfigured:     value.ClientSecretConfigured,
		ApiKeyConfigured:           value.APIKeyConfigured,
		ClearClientSecret:          value.ClearClientSecret,
		ClearApiKey:                value.ClearAPIKey,
		RevocationSecretConfigured: value.RevocationSecretConfigured,
		ClearRevocationSecret:      value.ClearRevocationSecret,
		Claims:                     &pb.OAuthClaims{Subject: value.Claims.Subject, Username: value.Claims.Username, Name: value.Claims.Name, Email: value.Claims.Email, EmailVerified: value.Claims.EmailVerified, Avatar: value.Claims.Avatar},
	}
}
func oauthProviderRequest(value *pb.OAuthProviderSettings) oauthProviderSettings {
	return oauthProviderSettings{
		ID:                         value.GetId(),
		Type:                       value.GetType(),
		Name:                       value.GetName(),
		Enabled:                    value.GetEnabled(),
		ClientID:                   value.GetClientId(),
		ClientSecret:               value.GetClientSecret(),
		CallbackURL:                value.GetCallbackUrl(),
		Tenant:                     value.GetTenant(),
		BaseURL:                    value.GetBaseUrl(),
		Site:                       value.GetSite(),
		APIKey:                     value.GetApiKey(),
		AuthorizeURL:               value.GetAuthorizeUrl(),
		TokenURL:                   value.GetTokenUrl(),
		UserInfoURL:                value.GetUserinfoUrl(),
		Issuer:                     value.GetIssuer(),
		JWKSURL:                    value.GetJwksUrl(),
		Scopes:                     value.GetScopes(),
		TokenAuth:                  value.GetTokenAuth(),
		DisablePKCE:                value.GetDisablePkce(),
		RevocationURL:              value.GetRevocationUrl(),
		RevocationSecret:           value.GetRevocationSecret(),
		Claims:                     config.OAuthClaims{Subject: value.GetClaims().GetSubject(), Username: value.GetClaims().GetUsername(), Name: value.GetClaims().GetName(), Email: value.GetClaims().GetEmail(), EmailVerified: value.GetClaims().GetEmailVerified(), Avatar: value.GetClaims().GetAvatar()},
		ClientSecretConfigured:     value.GetClientSecretConfigured(),
		APIKeyConfigured:           value.GetApiKeyConfigured(),
		ClearClientSecret:          value.GetClearClientSecret(),
		ClearAPIKey:                value.GetClearApiKey(),
		RevocationSecretConfigured: value.GetRevocationSecretConfigured(),
		ClearRevocationSecret:      value.GetClearRevocationSecret(),
	}
}
