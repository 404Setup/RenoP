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
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"mime"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/utils"
	"renop/internal/utils/ratelimit"
	"renop/pkg/hex"
	"renop/pkg/pb"
)

func isProviderRevocationPath(path string) bool {
	rest, ok := strings.CutPrefix(path, "/api/auth/oauth/")
	provider, action, found := strings.Cut(rest, "/")
	return ok && found && provider != "" && len(provider) <= 32 && (action == "revoked" || action == "backchannel-logout")
}

func setupOAuthRevocationRoutes(router fiber.Router, state *core.AppState) {
	gate := ratelimit.New(rate.Limit(2), 20, 4096)
	slots := make(chan struct{}, 8)
	admit := func(c fiber.Ctx) error {
		setPrivateResponseHeaders(c)
		if gate.Allow(ratelimit.NetworkKey(utils.ExtractIP(c, &state.Inner.Config.Load().Server))) > 0 {
			c.Set(fiber.HeaderRetryAfter, "1")
			return fiber.ErrTooManyRequests
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			return fiber.ErrServiceUnavailable
		}
		return c.Next()
	}
	router.Post("/oauth/:provider/revoke", func(c fiber.Ctx) error {
		token := CurrentSessionToken(c)
		session := state.GetSession(token)
		if session == nil || token == "" || c.Cookies(sessionCookieName) != token {
			return fiber.ErrUnauthorized
		}
		method, _, _ := strings.Cut(session.LoginMethod, "+")
		provider := strings.TrimPrefix(method, "oauth:")
		if provider != c.Params("provider") || method != "github" && !strings.HasPrefix(method, "oauth:") {
			return fiber.ErrConflict
		}
		return PostAuthLogout(c, state)
	})
	router.Post("/oauth/:provider/revoked", admit, func(c fiber.Ctx) error { return receiveOAuthRevocation(c, state) })
	router.Post("/oauth/:provider/backchannel-logout", admit, func(c fiber.Ctx) error { return receiveOIDCLogout(c, state) })
}

// revocationConfiguration includes disabled clients so providers can still invalidate existing sessions.
func revocationConfiguration(state *core.AppState, id string) (config.OAuthProviderConfig, bool) {
	if id == "github" {
		github := state.Inner.Config.Load().Server.GitHubOAuth
		return config.OAuthProviderConfig{ID: "github", Type: "github", ClientID: github.ClientID, RevocationSecret: github.RevocationSecret}, github.ClientID != ""
	}
	for _, p := range state.Inner.Config.Load().Server.OAuthProviders {
		if p.ID == id && p.ClientID != "" && p.Validate() == nil {
			return p.Resolved(), true
		}
	}
	return config.OAuthProviderConfig{}, false
}

func revocationAuthority(p config.OAuthProviderConfig, issuer string) (string, error) {
	if p.Type == "github" {
		if issuer != "" {
			return "", core.ErrMFAInvalid
		}
		return githubRevocationAuthority(p.ClientID, defaultGitHubOAuthProvider.APIURL), nil
	}
	if p.Issuer == "" {
		if issuer != "" {
			return "", core.ErrMFAInvalid
		}
		return p.Authority(""), nil
	}
	tenant := ""
	if p.Type == "microsoft" {
		u, err := url.Parse(issuer)
		if err != nil {
			return "", core.ErrMFAInvalid
		}
		tenant = strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	}
	if !oauthIssuerAllowed(p, issuer, tenant) {
		return "", core.ErrMFAInvalid
	}
	if p.Type == "google" {
		issuer = p.Issuer
	}
	return p.Authority(issuer), nil
}

func receiveOAuthRevocation(c fiber.Ctx, state *core.AppState) error {
	p, ok := revocationConfiguration(state, c.Params("provider"))
	if !ok || len(p.RevocationSecret) < 32 {
		return fiber.ErrNotFound
	}
	body, err := utils.ReadRequestBodyLimited(c, 16384)
	if err != nil {
		return err
	}
	mediaType, _, typeErr := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if typeErr != nil || mediaType != "application/x-protobuf" {
		return fiber.ErrUnsupportedMediaType
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(c.Get("X-Renop-Signature-256"), "sha256="))
	mac := hmac.New(sha256.New, []byte(p.RevocationSecret))
	mac.Write(body)
	if err != nil || !hmac.Equal(mac.Sum(nil), signature) {
		return fiber.ErrUnauthorized
	}
	var request pb.ProviderRevocationRequest
	if proto.Unmarshal(body, &request) != nil {
		return fiber.ErrBadRequest
	}
	authority, err := revocationAuthority(p, request.Issuer)
	if err != nil {
		return fiber.ErrBadRequest
	}
	return applyProviderRevocation(c, state, p, core.OAuthRevocation{ProviderID: p.ID, Authority: authority,
		EventID: request.EventId, Subject: request.Subject, SessionID: request.SessionId, IssuedAt: request.IssuedAt})
}

func receiveOIDCLogout(c fiber.Ctx, state *core.AppState) error {
	p, ok := revocationConfiguration(state, c.Params("provider"))
	if !ok || p.Issuer == "" || p.JWKSURL == "" {
		return fiber.ErrNotFound
	}
	body, err := utils.ReadRequestBodyLimited(c, 32768)
	if err != nil {
		return err
	}
	mediaType, _, typeErr := mime.ParseMediaType(c.Get(fiber.HeaderContentType))
	if typeErr != nil || mediaType != "application/x-www-form-urlencoded" {
		return fiber.ErrUnsupportedMediaType
	}
	form, err := url.ParseQuery(string(body))
	if err != nil || len(form["logout_token"]) != 1 {
		return fiber.ErrBadRequest
	}
	raw := form.Get("logout_token")
	if raw == "" || len(raw) > 16384 {
		return fiber.ErrBadRequest
	}
	client, err := oauthHTTPClient(state.Inner.Config.Load())
	if err != nil {
		return fiber.ErrServiceUnavailable
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(c.Context(), 8*time.Second)
	defer cancel()
	claims, err := parseOAuthJWT(ctx, client, p, raw)
	if err != nil {
		return fiber.ErrBadRequest
	}
	if _, exists := claims["nonce"]; exists {
		return fiber.ErrBadRequest
	}
	issued, err := claims.GetIssuedAt()
	if err != nil || issued == nil {
		return fiber.ErrBadRequest
	}
	subject, err := claims.GetSubject()
	if err != nil {
		return fiber.ErrBadRequest
	}
	sid, hasSID := claims["sid"].(string)
	if _, exists := claims["sid"]; exists && !hasSID {
		return fiber.ErrBadRequest
	}
	jti, ok := claims["jti"].(string)
	if !ok {
		return fiber.ErrBadRequest
	}
	events, ok := claims["events"].(map[string]any)
	if !ok {
		return fiber.ErrBadRequest
	}
	if _, ok := events["http://schemas.openid.net/event/backchannel-logout"].(map[string]any); !ok {
		return fiber.ErrBadRequest
	}
	issuer, _ := claims.GetIssuer()
	authority, err := revocationAuthority(p, issuer)
	if err != nil {
		return fiber.ErrBadRequest
	}
	return applyProviderRevocation(c, state, p, core.OAuthRevocation{ProviderID: p.ID, Authority: authority,
		EventID: jti, Subject: subject, SessionID: sid, IssuedAt: issued.Time.UnixMilli() + 999})
}

func applyProviderRevocation(c fiber.Ctx, state *core.AppState, p config.OAuthProviderConfig, event core.OAuthRevocation) error {
	state.Inner.ConfigWriteLock.Lock()
	defer state.Inner.ConfigWriteLock.Unlock()
	current, ok := revocationConfiguration(state, p.ID)
	if !ok || oauthConfigurationHash(current) != oauthConfigurationHash(p) {
		return fiber.ErrBadRequest
	}
	db := state.GetDB()
	if db == nil {
		return fiber.ErrServiceUnavailable
	}
	tokens, err := db.RevokeOAuthSessions(event, time.Now().UnixMilli())
	if errors.Is(err, core.ErrMFAInvalid) {
		return fiber.ErrBadRequest
	}
	if err != nil {
		return fiber.ErrServiceUnavailable
	}
	for _, token := range tokens {
		state.DeleteAuthCache("Session " + token)
		state.Inner.Sessions.Delete(token)
	}
	return c.SendStatus(fiber.StatusOK)
}
