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
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/emmansun/base64"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/mail"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"
)

const passwordTestTOTP = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"

func passwordChangeTestApp(t *testing.T, factors bool) (*fiber.App, *core.AppState, *database.DB, *ecdsa.PrivateKey) {
	t.Helper()
	db := newTestAuthDB(t)
	state := core.NewAppState()
	state.Inner.DB = db
	cfg := config.DefaultConfig()
	cfg.MFAEncryptionKey = base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	require.NoError(t, cfg.Mail.EnsureKey())
	state.Inner.Config.Store(cfg)
	state.Inner.TokensCount.Store(1)
	password, err := bcrypt.GenerateFromPassword([]byte("original-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "alice", EncryptedSecret: string(password), Permissions: []string{"base"}}))
	now := time.Now().UnixMilli()
	_, err = db.UpdateAccountEmail("alice", "primary@example.com", now)
	require.NoError(t, err)
	session := &core.Session{Username: "alice", PublicID: "current", CreatedAt: now, LoginMethod: "password"}
	session.LastActive.Store(now)
	require.NoError(t, state.SaveSession(session, "current"))
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	// COSE EC2 / ES256 / P-256 public key with fixed-width coordinates.
	cose := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	cose = append(cose, key.X.FillBytes(make([]byte, 32))...)
	cose = append(cose, 0x22, 0x58, 0x20)
	cose = append(cose, key.Y.FillBytes(make([]byte, 32))...)
	require.NoError(t, db.SaveFidoDevice(&core.FidoDevice{Username: "alice", ID: "passkey", Name: "Security key",
		CredentialID: []byte("password-key"), PublicKey: cose, CreatedAt: now, UserPresent: true, UserVerified: true}))
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	if factors {
		secret, err := encryptMFASecret(state, mfa.UserID, passwordTestTOTP)
		require.NoError(t, err)
		require.NoError(t, db.UpdateMFA("alice", mfa.Snapshot, secret, true, -1, "current"))
	}
	mfa, err = db.GetMFAState("alice")
	require.NoError(t, err)
	other := &core.Session{Username: "alice", PublicID: "other", CreatedAt: now,
		AuthenticationSnapshot: mfa.Snapshot, LoginMethod: "password+totp"}
	other.LastActive.Store(now)
	require.NoError(t, state.SaveSession(other, "other"))
	app := fiber.New()
	app.Use(AuthMiddleware(state))
	SetupAuthRoutes(app.Group("/api"), state, nil)
	return app, state, db, key
}

func putTestPassword(t *testing.T, app *fiber.App, session string, payload *pb.UpdatePasswordRequest) *http.Response {
	t.Helper()
	body, err := proto.Marshal(payload)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPut, "http://localhost/api/auth/profile/password", bytes.NewReader(body))
	request.Header.Set("Content-Type", protohttp.ContentType)
	if session != "" {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session})
	}
	response, err := app.Test(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestPasswordChangeRequiresCurrentTOTPAndKeepsOnlyCurrentSession(t *testing.T) {
	app, state, db, _ := passwordChangeTestApp(t, true)
	request := &pb.UpdatePasswordRequest{NewPassword: "replacement-password"}
	response := putTestPassword(t, app, "current", request)
	require.Equal(t, 409, response.StatusCode)
	require.Equal(t, "MFA_REQUIRED", response.Header.Get("X-Renop-Error-Code"))
	request.Factor = "totp"
	response = putTestPassword(t, app, "current", request)
	require.Equal(t, 400, response.StatusCode)
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	require.Equal(t, 1, mfa.Failures)
	request.TotpCode = core.TOTPCode(passwordTestTOTP, time.Now().Unix()/30)
	response = putTestPassword(t, app, "current", request)
	require.Equal(t, 200, response.StatusCode)
	account, err := db.GetTokenByName("alice")
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(account.EncryptedSecret), []byte(request.NewPassword)))
	require.NotNil(t, state.GetSession("current"))
	require.Nil(t, state.GetSession("other"))
	request.NewPassword = "must-not-apply"
	require.Equal(t, 400, putTestPassword(t, app, "current", request).StatusCode)
	account, err = db.GetTokenByName("alice")
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(account.EncryptedSecret), []byte("replacement-password")))
}

func TestPasswordChangeEmailProofAndSessionSnapshot(t *testing.T) {
	app, state, db, _ := passwordChangeTestApp(t, true)
	cfg := state.Inner.Config.Load().DeepCopy()
	cfg.Mail.Enabled = true
	state.Inner.Config.Store(cfg)
	now := time.Now().UnixMilli()
	job := &mail.Job{ID: "password-email", AccountID: "sender", Scene: "password_reset", TicketHash: "ticket",
		CreatedAt: now, ExpiresAt: now + 600000, Message: mail.Message{ID: "password-email", To: "primary@example.com", Subject: "Code", Text: "12345678"}}
	_, err := db.QueueEmailPasswordReset(job, emailPasswordResetHash(cfg.Mail.EncryptionKey, job.Message.To, "12345678"),
		cfg.Mail.EncryptionKey, "192.0.2.1", cfg.Mail.ManualRate)
	require.NoError(t, err)
	request := &pb.UpdatePasswordRequest{NewPassword: "email-password", Factor: "email", EmailCode: "87654321"}
	require.Equal(t, 400, putTestPassword(t, app, "current", request).StatusCode)
	request.EmailCode = "12345678"
	require.Equal(t, 200, putTestPassword(t, app, "current", request).StatusCode)
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	require.True(t, mfa.Enabled(), "email password changes retain the second-factor policy")
	require.Equal(t, 400, putTestPassword(t, app, "current", request).StatusCode)
	change := core.PasswordChange{Username: "alice", Session: "current", Snapshot: mfa.Snapshot,
		PasswordHash: "stale-change", Factor: "totp", TOTPStep: time.Now().Unix() / 30, Now: time.Now().UnixMilli()}
	_, err = db.UpdateAccountEmail("alice", "new-primary@example.com", time.Now().UnixMilli())
	require.NoError(t, err)
	require.ErrorIs(t, db.ChangeAccountPassword(change), core.ErrMFAInvalid)
}

func TestPasswordChangePasskeyIsBoundToBrowserAndRequiresVerification(t *testing.T) {
	app, _, db, key := passwordChangeTestApp(t, true)
	begin := func() (string, string) {
		request := httptest.NewRequest(http.MethodPost, "http://localhost/api/auth/profile/password/passkey/begin", bytes.NewBufferString("{}"))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "current"})
		response, err := app.Test(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, 200, response.StatusCode)
		var result struct {
			ID      string `json:"challenge_id"`
			Options struct {
				PublicKey struct {
					Challenge string `json:"challenge"`
				} `json:"publicKey"`
			} `json:"options"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		return result.ID, result.Options.PublicKey.Challenge
	}
	assertion := func(challenge, origin string, flags byte) []byte {
		client, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": challenge, "origin": origin, "crossOrigin": false})
		require.NoError(t, err)
		rpHash := sha256.Sum256([]byte("localhost"))
		authenticator := append(rpHash[:], flags)
		authenticator = binary.BigEndian.AppendUint32(authenticator, 1)
		clientHash := sha256.Sum256(client)
		digest := sha256.Sum256(append(bytes.Clone(authenticator), clientHash[:]...))
		signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
		require.NoError(t, err)
		encoded := base64.RawURLEncoding.EncodeToString
		payload, err := json.Marshal(map[string]any{"id": encoded([]byte("password-key")), "rawId": encoded([]byte("password-key")), "type": "public-key",
			"response": map[string]any{"authenticatorData": encoded(authenticator), "clientDataJSON": encoded(client), "signature": encoded(signature), "userHandle": encoded([]byte("alice"))}})
		require.NoError(t, err)
		return payload
	}
	for _, scenario := range []struct {
		session, origin string
		flags           byte
	}{
		{"other", "http://localhost", 5}, {"current", "https://untrusted.example", 5}, {"current", "http://localhost", 1},
	} {
		id, challenge := begin()
		request := &pb.UpdatePasswordRequest{NewPassword: "must-not-apply", Factor: "passkey", ChallengeId: id,
			PasskeyCredential: assertion(challenge, scenario.origin, scenario.flags)}
		require.Equal(t, 400, putTestPassword(t, app, scenario.session, request).StatusCode)
	}
	id, challenge := begin()
	request := &pb.UpdatePasswordRequest{NewPassword: "passkey-password", Factor: "passkey", ChallengeId: id,
		PasskeyCredential: assertion(challenge, "http://localhost", 5)}
	require.Equal(t, 200, putTestPassword(t, app, "current", request).StatusCode)
	require.Equal(t, 400, putTestPassword(t, app, "current", request).StatusCode)
	account, err := db.GetTokenByName("alice")
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(account.EncryptedSecret), []byte("passkey-password")))
}

func TestPasswordChangeWithoutMFAStillRequiresItsBrowser(t *testing.T) {
	app, _, _, _ := passwordChangeTestApp(t, false)
	request := &pb.UpdatePasswordRequest{NewPassword: "replacement-password"}
	require.NotEqual(t, 200, putTestPassword(t, app, "", request).StatusCode)
	require.Equal(t, 200, putTestPassword(t, app, "current", request).StatusCode)
}
