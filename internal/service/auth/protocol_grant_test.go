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
	"encoding/binary"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"renop/internal/config"
	"renop/internal/core"

	"github.com/emmansun/base64"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestProtocolGrantRechecksRepositoryExpiryAndCredential(t *testing.T) {
	db := newTestAuthDB(t)
	state := core.NewAppState()
	state.Inner.DB = db
	cfg := config.DefaultConfig()
	for _, name := range []string{"first", "second"} {
		cfg.Maven.Repositories[name] = &config.Repository{Name: name, Format: "files", Visibility: "PUBLIC"}
	}
	state.Inner.Config.Store(cfg)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	require.NoError(t, err)
	account := &core.AccessToken{Name: "alice", EncryptedSecret: string(hash), Permissions: []string{"base"}}
	require.NoError(t, db.SaveToken(account))
	state.Inner.TokensCount.Store(1)
	app := fiber.New()
	app.Use(AuthMiddleware(state))
	app.Get("/first/authenticate", func(c fiber.Ctx) error {
		grant, err := IssueProtocolGrant(c, "first")
		if err != nil {
			return c.SendStatus(401)
		}
		return c.SendString(grant)
	})
	app.Get("/*", func(c fiber.Ctx) error { return c.SendString(GetUser(c).Username) })
	request := httptest.NewRequest("GET", "/first/authenticate", nil)
	request.SetBasicAuth("alice", "password")
	response, err := app.Test(request)
	require.NoError(t, err)
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	grant := string(data)
	require.True(t, strings.HasPrefix(grant, protocolGrantPrefix))
	check := func(path, token string, status int) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := app.Test(req)
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, status, res.StatusCode)
	}
	check("/first/file", grant, 200)
	check("/first/file", grant, 200)
	check("/second/file", grant, 401)
	check("/api/auth/profile", grant, 401)
	check("/first/file", grant+"x", 401)
	key, err := protocolGrantCipher()
	require.NoError(t, err)
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(grant, protocolGrantPrefix))
	require.NoError(t, err)
	payload, err := key.Open(nil, nil, sealed, []byte("first"))
	require.NoError(t, err)
	binary.BigEndian.PutUint64(payload, uint64(time.Now().Add(-time.Second).Unix()))
	expired := protocolGrantPrefix + base64.RawURLEncoding.EncodeToString(key.Seal(nil, nil, payload, []byte("first")))
	check("/first/file", expired, 401)
	newHash, err := bcrypt.GenerateFromPassword([]byte("changed"), bcrypt.MinCost)
	require.NoError(t, err)
	account.EncryptedSecret = string(newHash)
	require.NoError(t, db.SaveToken(account))
	state.InvalidateAccountAuthCache(false, "alice")
	check("/first/file", grant, 401)
}
