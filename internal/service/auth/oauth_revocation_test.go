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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emmansun/base64"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/utils/protohttp"
	"renop/pkg/hex"
	"renop/pkg/pb"
)

func revocationTestState(t *testing.T, endpoint string) (*core.AppState, config.OAuthProviderConfig) {
	t.Helper()
	state := core.NewAppState()
	state.Inner.DB = newTestAuthDB(t)
	cfg := config.DefaultConfig()
	cfg.MFAEncryptionKey = base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	p := config.OAuthProviderConfig{ID: "demo", Type: "custom", Name: "Demo", Enabled: true, ClientID: "client", ClientSecret: "client-secret",
		CallbackURL: "https://renop.example/api/auth/oauth/demo/callback", AuthorizeURL: endpoint + "/authorize", TokenURL: endpoint + "/token", UserInfoURL: endpoint + "/userinfo",
		RevocationURL: endpoint + "/revoke", RevocationSecret: strings.Repeat("signed-callback-", 3), Claims: config.OAuthClaims{Subject: "sub"}}
	require.NoError(t, p.Validate())
	p = p.Resolved()
	cfg.Server.OAuthProviders = []config.OAuthProviderConfig{p}
	state.Inner.Config.Store(cfg)
	for _, user := range []string{"alice", "bob"} {
		require.NoError(t, state.GetDB().SaveToken(&core.AccessToken{Name: user, Permissions: []string{"base"}}))
	}
	return state, p
}

func saveRevocationSession(t *testing.T, state *core.AppState, p config.OAuthProviderConfig, user, sid string, authorizedAt int64) string {
	t.Helper()
	profile, err := state.GetDB().GetUserProfile(user)
	require.NoError(t, err)
	publicID := uuid.NewString()
	proof := &oauthSessionProof{ProviderID: p.ID, UserID: profile.UserID, Authority: p.Authority(p.Issuer), Issuer: p.Issuer,
		Subject: user + "-subject", SessionID: sid, AuthorizedAt: authorizedAt, ConfigHash: oauthConfigurationHash(p),
		Tokens: oauthTokens{AccessToken: "access-" + sid, RefreshToken: "refresh-" + sid}}
	grant, err := sealSessionOAuthGrant(state, proof, publicID)
	require.NoError(t, err)
	require.NotContains(t, grant.EncryptedTokens, "access-")
	session := &core.Session{Username: user, PublicID: publicID, LoginMethod: "oauth:" + p.ID, CreatedAt: time.Now().UnixMilli(), OAuthGrant: grant}
	session.LastActive.Store(time.Now().UnixMilli())
	secret := uuid.NewString()
	require.NoError(t, state.SaveSession(session, secret))
	return secret
}

func TestProviderLogoutKeepsLocalRevocationOnUpstreamFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			var state *core.AppState
			var secret string
			var tokens []string
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Nil(t, state.GetSession(secret), "local session must be removed before contacting the provider")
				require.Equal(t, "/revoke", r.URL.Path)
				require.Equal(t, http.MethodPost, r.Method)
				require.NoError(t, r.ParseForm())
				require.Equal(t, "client", r.Form.Get("client_id"))
				require.Equal(t, "client-secret", r.Form.Get("client_secret"))
				tokens = append(tokens, r.Form.Get("token"))
				if fail {
					w.WriteHeader(http.StatusServiceUnavailable)
				} else {
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer provider.Close()
			var p config.OAuthProviderConfig
			state, p = revocationTestState(t, provider.URL)
			secret = saveRevocationSession(t, state, p, "alice", "target", time.Now().UnixMilli())
			app := fiber.New()
			app.Use(AuthMiddleware(state))
			setupOAuthRoutes(app.Group("/api/auth"), state)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/demo/revoke", nil)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: secret})
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var result pb.LogoutResponse
			require.NoError(t, proto.Unmarshal(body, &result))
			require.True(t, result.LocalRevoked)
			require.Equal(t, "demo", result.Provider)
			if fail {
				require.Equal(t, "failed", result.ProviderStatus)
				require.Len(t, tokens, 1)
			} else {
				require.Equal(t, "revoked", result.ProviderStatus)
				require.Equal(t, []string{"refresh-target", "access-target"}, tokens)
			}
			require.NotContains(t, string(body), "access-target")
			grant, err := state.GetDB().GetSessionOAuthGrant(secret)
			require.NoError(t, err)
			require.Nil(t, grant)
			require.Empty(t, resp.Cookies()[0].Value)
		})
	}
}

func TestSignedProviderRevocationScopesSessionsAndRejectsReplay(t *testing.T) {
	state, p := revocationTestState(t, "https://provider.example")
	now := time.Now().UnixMilli()
	target := saveRevocationSession(t, state, p, "alice", "target", now-1000)
	sibling := saveRevocationSession(t, state, p, "alice", "sibling", now-1000)
	other := saveRevocationSession(t, state, p, "bob", "target", now-1000)
	app := fiber.New()
	app.Use(AuthMiddleware(state))
	setupOAuthRoutes(app.Group("/api/auth"), state)
	event := &pb.ProviderRevocationRequest{EventId: uuid.NewString(), Subject: "alice-subject", SessionId: "target", IssuedAt: now}
	send := func(payload *pb.ProviderRevocationRequest, key string) int {
		t.Helper()
		body, err := proto.Marshal(payload)
		require.NoError(t, err)
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write(body)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/demo/revoked", bytes.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, protohttp.ContentType)
		req.Header.Set("X-Renop-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}
	require.Equal(t, http.StatusUnauthorized, send(event, "wrong-secret"))
	require.NotNil(t, state.GetSession(target))
	require.Equal(t, http.StatusOK, send(event, p.RevocationSecret))
	require.Nil(t, state.GetSession(target))
	require.NotNil(t, state.GetSession(sibling))
	require.NotNil(t, state.GetSession(other))
	fresh := saveRevocationSession(t, state, p, "alice", "target", now+1)
	event.IssuedAt = now + 1000
	require.Equal(t, http.StatusOK, send(event, p.RevocationSecret))
	require.NotNil(t, state.GetSession(fresh))
	event.EventId, event.IssuedAt = uuid.NewString(), now-int64(11*time.Minute/time.Millisecond)
	require.Equal(t, http.StatusBadRequest, send(event, p.RevocationSecret))
	// Durable event history blocks a delayed proof even after process-local state is gone.
	grant, err := state.GetDB().GetSessionOAuthGrant(fresh)
	require.NoError(t, err)
	grant.AuthorizedAt = now - 1000
	stale := &core.Session{Username: "alice", PublicID: uuid.NewString(), CreatedAt: now, OAuthGrant: grant}
	stale.LastActive.Store(now)
	require.ErrorIs(t, state.GetDB().SaveSession(stale, "delayed-proof"), core.ErrMFAInvalid)
}

func TestOIDCBackchannelLogoutValidatesClaimsAndScope(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/keys", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kid": "test", "kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig",
			"x": base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))), "y": base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32)))}}})
	}))
	defer provider.Close()
	state, p := revocationTestState(t, provider.URL)
	p.Issuer, p.JWKSURL, p.Scopes = provider.URL, provider.URL+"/keys", "openid"
	require.NoError(t, p.Validate())
	cfg := state.Inner.Config.Load().DeepCopy()
	cfg.Server.OAuthProviders = []config.OAuthProviderConfig{p}
	state.Inner.Config.Store(cfg)
	now := time.Now().Unix()
	target := saveRevocationSession(t, state, p, "alice", "target", (now-1)*1000)
	sibling := saveRevocationSession(t, state, p, "alice", "sibling", (now-1)*1000)
	app := fiber.New()
	app.Use(AuthMiddleware(state))
	setupOAuthRoutes(app.Group("/api/auth"), state)
	send := func(change func(jwt.MapClaims)) int {
		claims := jwt.MapClaims{"iss": provider.URL, "aud": "client", "iat": now, "exp": now + 60, "jti": uuid.NewString(),
			"sub": "alice-subject", "sid": "target", "events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}}
		change(claims)
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		token.Header["kid"] = "test"
		raw, err := token.SignedString(key)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/demo/backchannel-logout", strings.NewReader(url.Values{"logout_token": {raw}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}
	for name, change := range map[string]func(jwt.MapClaims){
		"audience":         func(c jwt.MapClaims) { c["aud"] = "another-client" },
		"issuer":           func(c jwt.MapClaims) { c["iss"] = "https://another-provider.example" },
		"expired":          func(c jwt.MapClaims) { c["exp"] = now - 60 },
		"missing-expiry":   func(c jwt.MapClaims) { delete(c, "exp") },
		"missing-issued":   func(c jwt.MapClaims) { delete(c, "iat") },
		"stale":            func(c jwt.MapClaims) { c["iat"] = now - 660 },
		"future":           func(c jwt.MapClaims) { c["iat"] = now + 60 },
		"nonce":            func(c jwt.MapClaims) { c["nonce"] = "not-a-logout" },
		"event":            func(c jwt.MapClaims) { c["events"] = map[string]any{} },
		"empty-scope":      func(c jwt.MapClaims) { delete(c, "sub"); delete(c, "sid") },
		"missing-event-id": func(c jwt.MapClaims) { delete(c, "jti") },
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, http.StatusBadRequest, send(change))
			require.NotNil(t, state.GetSession(target))
		})
	}
	require.Equal(t, http.StatusOK, send(func(c jwt.MapClaims) {}))
	require.Nil(t, state.GetSession(target))
	require.NotNil(t, state.GetSession(sibling))
	require.Equal(t, http.StatusOK, send(func(c jwt.MapClaims) { delete(c, "sid") }))
	require.Nil(t, state.GetSession(sibling))
}
