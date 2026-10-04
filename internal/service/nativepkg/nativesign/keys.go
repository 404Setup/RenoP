/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package nativesign owns persistent keys for generated native repository indexes.
package nativesign

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"renop/internal/config"
	"renop/internal/configstore"
	"renop/internal/core"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type signer struct {
	keys                 config.NativeSigningKeys
	pgp                  *openpgp.Entity
	apk                  *rsa.PrivateKey
	pgpPublic, apkPublic []byte
}

var current atomic.Pointer[signer]
var loadMu sync.Mutex

func EnsureKeys(state *core.AppState, settingsPath string) error {
	if state.IsDemo() {
		return nil
	}
	state.Inner.ConfigWriteLock.Lock()
	defer state.Inner.ConfigWriteLock.Unlock()
	cfg := state.Inner.Config.Load()
	keys := cfg.NativeSigningKeys
	if keys.OpenPGP != "" && keys.APK != "" {
		_, err := load(keys)
		return err
	}
	if keys.OpenPGP == "" {
		entity, err := openpgp.NewEntity("RenoP Repository", "Repository index signing", "", &packet.Config{RSABits: 2048, DefaultHash: crypto.SHA256})
		if err != nil {
			return err
		}
		var output bytes.Buffer
		writer, err := armor.Encode(&output, openpgp.PrivateKeyType, nil)
		if err != nil {
			return err
		}
		if err := entity.SerializePrivateWithoutSigning(writer, nil); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		keys.OpenPGP = output.String()
	}
	if keys.APK == "" {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}
		encoded, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return err
		}
		keys.APK = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}))
	}
	if _, err := load(keys); err != nil {
		return err
	}
	next := cfg.DeepCopy()
	next.NativeSigningKeys = keys
	if err := configstore.Save(settingsPath, next); err != nil {
		return err
	}
	state.Inner.Config.Store(next)
	return nil
}

func load(keys config.NativeSigningKeys) (*signer, error) {
	if cached := current.Load(); cached != nil && cached.keys == keys {
		return cached, nil
	}
	loadMu.Lock()
	defer loadMu.Unlock()
	if cached := current.Load(); cached != nil && cached.keys == keys {
		return cached, nil
	}
	if keys.OpenPGP == "" || keys.APK == "" || len(keys.OpenPGP) > 64<<10 || len(keys.APK) > 16<<10 {
		return nil, errors.New("native repository signing keys are unavailable")
	}
	entities, err := openpgp.ReadArmoredKeyRing(strings.NewReader(keys.OpenPGP))
	if err != nil || len(entities) != 1 || entities[0].PrivateKey == nil || entities[0].PrivateKey.Encrypted {
		return nil, errors.New("invalid native OpenPGP signing key")
	}
	block, rest := pem.Decode([]byte(keys.APK))
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("invalid native APK signing key")
	}
	private, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid native APK signing key")
	}
	key, ok := private.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < 2048 || key.N.BitLen() > 4096 {
		return nil, errors.New("invalid native APK signing key size")
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	key.Precompute()
	var public bytes.Buffer
	writer, err := armor.Encode(&public, openpgp.PublicKeyType, nil)
	if err != nil {
		return nil, err
	}
	if err := entities[0].Serialize(writer); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	pkix, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	result := &signer{keys, entities[0], key, public.Bytes(), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})}
	current.Store(result)
	return result, nil
}

func PublicKey(keys config.NativeSigningKeys, apk bool) ([]byte, error) {
	signer, err := load(keys)
	if err != nil {
		return nil, err
	}
	if apk {
		return bytes.Clone(signer.apkPublic), nil
	}
	return bytes.Clone(signer.pgpPublic), nil
}
