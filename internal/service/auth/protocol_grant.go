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
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"time"

	"renop/internal/core"
	"renop/internal/utils/secretcipher"

	"github.com/emmansun/base64"
	"github.com/gofiber/fiber/v3"
)

const protocolGrantPrefix = "renop-protocol-"

var protocolGrantCipher = sync.OnceValues(func() (cipher.AEAD, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	return secretcipher.New(base64.RawStdEncoding.EncodeToString(key[:]))
})

// IssueProtocolGrant exchanges already verified Basic credentials for an opaque,
// short-lived repository-bound bearer. Every use rechecks the original credential
// through the normal live account/token path. The embedded credential is encrypted
// with a process-private key and never returned as plaintext or persisted server-side.
func IssueProtocolGrant(c fiber.Ctx, repository string) (string, error) {
	header := c.Get(fiber.HeaderAuthorization)
	if c.Locals(authSchemeLocal) != "basic" || GetUser(c) == nil || GetUser(c).Username == "guest" || !strings.HasPrefix(header, "Basic ") || len(header) > 8192 {
		return "", errors.New("verified basic credentials required")
	}
	key, err := protocolGrantCipher()
	if err != nil {
		return "", err
	}
	payload := make([]byte, 8+len(header))
	binary.BigEndian.PutUint64(payload, uint64(time.Now().Add(15*time.Minute).Unix()))
	copy(payload[8:], header)
	sealed := key.Seal(nil, nil, payload, []byte(repository))
	return protocolGrantPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func authenticateProtocolGrant(state *core.AppState, grant string, c fiber.Ctx) (*authResult, error) {
	if len(grant) > 12000 || !isRepositoryRequest(c, state) {
		return nil, nil
	}
	repository, _, _ := strings.Cut(strings.TrimPrefix(c.Path(), "/"), "/")
	key, err := protocolGrantCipher()
	if err != nil {
		return nil, err
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(grant, protocolGrantPrefix))
	if err != nil {
		return nil, nil
	}
	payload, err := key.Open(nil, nil, sealed, []byte(repository))
	if err != nil || len(payload) < 8 || binary.BigEndian.Uint64(payload) <= uint64(time.Now().Unix()) {
		return nil, nil
	}
	return handleBasicAuth(state, string(payload[8:]), c)
}
