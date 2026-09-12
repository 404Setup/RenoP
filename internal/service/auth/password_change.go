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
	"crypto/rand"
	"crypto/sha256"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/pkg/hex"
	"renop/pkg/pb"
)

const passwordPasskeyPurpose = "password-passkey"

func passwordChangeAccount(c fiber.Ctx, state *core.AppState) (*config.User, string, *core.MFAState, error) {
	user, err := requireAccountSession(c)
	if err != nil {
		return nil, "", nil, err
	}
	id := c.Locals("current_session_id").(string)
	if c.Cookies(sessionCookieName) != id {
		return nil, "", nil, fiber.ErrForbidden
	}
	session, err := state.GetDB().GetSession(id)
	if err != nil {
		return nil, "", nil, err
	}
	if session == nil || !strings.EqualFold(session.Username, user.Username) {
		return nil, "", nil, core.ErrMFAInvalid
	}
	mfa, err := state.GetDB().GetMFAState(user.Username)
	return user, id, mfa, err
}

func passwordSessionHash(session string) string {
	digest := sha256.Sum256([]byte(session))
	return hex.EncodeToString(digest[:])
}

func beginPasswordPasskey(c fiber.Ctx, state *core.AppState) error {
	var request struct{}
	if err := readMFARequest(c, &request); err != nil {
		return err
	}
	user, session, mfa, err := passwordChangeAccount(c, state)
	if err != nil {
		return mfaError(c, err)
	}
	if !mfa.Passkey {
		return mfaError(c, core.ErrMFAInvalid)
	}
	w, err := getWebAuthnEngine(c, state)
	if err != nil {
		return mfaError(c, err)
	}
	options, data, err := w.BeginLogin(buildFidoUser(user.Username, state), webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return mfaError(c, err)
	}
	encoded, err := json.Marshal(data)
	if err != nil || len(encoded) > 64<<10 {
		return mfaError(c, core.ErrMFAInvalid)
	}
	now := time.Now()
	id := rand.Text()
	expires := now.Add(mfaTTL).UnixMilli()
	if !state.Inner.ExternalAuthStates.Put(id, core.TransientAuthState{Provider: passwordPasskeyPurpose,
		UserID: mfa.UserID, Intent: passwordPasskeyPurpose, SessionHash: passwordSessionHash(session),
		Snapshot: mfa.Snapshot, Verifier: string(encoded), ExpiresAt: expires}, now.UnixMilli()) {
		return mfaError(c, core.ErrMFAInvalid)
	}
	setPrivateResponseHeaders(c)
	return c.JSON(fiber.Map{"challenge_id": id, "options": options, "expires_at": expires})
}

func passwordChangeProof(c fiber.Ctx, state *core.AppState, username, session string, mfa *core.MFAState, request *pb.UpdatePasswordRequest) (core.PasswordChange, error) {
	change := core.PasswordChange{Username: username, Session: session, Snapshot: mfa.Snapshot, Factor: request.Factor, TOTPStep: -1}
	switch request.Factor {
	case "":
		if mfa.Enabled() {
			return change, core.ErrMFARequired
		}
	case "totp":
		if mfa.Secret == "" {
			return change, core.ErrMFAInvalid
		}
		secret, err := decryptMFASecret(state, mfa)
		if err != nil {
			return change, err
		}
		change.TOTPStep = core.VerifyTOTP(secret, request.TotpCode, time.Now())
	case "email":
		cfg := state.Inner.Config.Load()
		if !cfg.Mail.Enabled || len(request.EmailCode) != 8 || strings.IndexFunc(request.EmailCode, func(c rune) bool { return c < '0' || c > '9' }) >= 0 {
			return change, core.ErrEmailCodeInvalid
		}
		security, err := state.GetDB().GetAccountSecurity(username)
		if err != nil {
			return change, err
		}
		if security.Email == "" {
			return change, core.ErrEmailCodeInvalid
		}
		change.Email = security.Email
		change.EmailCodeHash = emailPasswordResetHash(cfg.Mail.EncryptionKey, security.Email, request.EmailCode)
	case "passkey":
		if !mfa.Passkey || len(request.ChallengeId) > 128 {
			return change, core.ErrMFAInvalid
		}
		proof, ok := state.Inner.ExternalAuthStates.Consume(request.ChallengeId, passwordPasskeyPurpose, time.Now().UnixMilli())
		if !ok || proof.Intent != passwordPasskeyPurpose || proof.UserID != mfa.UserID || proof.Snapshot != mfa.Snapshot || proof.SessionHash != passwordSessionHash(session) {
			return change, core.ErrMFAInvalid
		}
		var data webauthn.SessionData
		if err := json.Unmarshal([]byte(proof.Verifier), &data); err != nil {
			return change, core.ErrMFAInvalid
		}
		credential, err := validateSecondFactorPasskey(c, state, username, data, request.PasskeyCredential)
		if err != nil {
			return change, err
		}
		change.CredentialID = credential.ID
	default:
		return change, core.ErrMFAInvalid
	}
	return change, nil
}

func validateSecondFactorPasskey(c fiber.Ctx, state *core.AppState, username string, data webauthn.SessionData, payload []byte) (*webauthn.Credential, error) {
	w, err := getWebAuthnEngine(c, state)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(payload))
	if err != nil {
		return nil, core.ErrMFAInvalid
	}
	request.Header.Set("Content-Type", "application/json")
	parsed, err := protocol.ParseCredentialRequestResponse(request)
	if err != nil {
		return nil, core.ErrMFAInvalid
	}
	credential, err := w.ValidateLogin(buildFidoAssertionUser(username, state, parsed), data, parsed)
	if err != nil || credential == nil || credential.Authenticator.CloneWarning {
		return nil, core.ErrMFAInvalid
	}
	if err := state.UpdateFidoDeviceState(credential.ID, credential.Authenticator.SignCount, credential.Flags.BackupState, credential.Flags.BackupEligible); err != nil {
		return nil, err
	}
	return credential, nil
}
