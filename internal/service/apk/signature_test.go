/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package apk

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"renop/pkg/hex"
)

func TestAPKSignatureAuthenticatesControlAndPayload(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	public := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded}))
	payload := testSegment(t, "usr/share/example", "payload")
	digest := sha256.Sum256(payload)
	control := testSegment(t, ".PKGINFO", "pkgname = example\npkgver = 1.0-r0\narch = x86_64\ndatahash = "+hex.EncodeToString(digest[:])+"\n")
	for _, algorithm := range []struct {
		name string
		hash crypto.Hash
	}{{"RSA", crypto.SHA1}, {"RSA256", crypto.SHA256}, {"RSA512", crypto.SHA512}} {
		t.Run(algorithm.name, func(t *testing.T) {
			hash := algorithm.hash.New()
			hash.Write(control)
			signature, err := rsa.SignPKCS1v15(rand.Reader, key, algorithm.hash, hash.Sum(nil))
			if err != nil {
				t.Fatal(err)
			}
			prefix := testSegment(t, ".SIGN."+algorithm.name+".publisher.rsa.pub", string(signature))
			body := append(append(bytes.Clone(prefix), control...), payload...)
			if err := VerifySignature(bytes.NewReader(body), int64(len(body)), public); err != nil {
				t.Fatal(err)
			}
			for _, offset := range []int{len(prefix) + len(control)/2, len(body) - 1} {
				changed := bytes.Clone(body)
				changed[offset] ^= 1
				if err := VerifySignature(bytes.NewReader(changed), int64(len(changed)), public); err == nil {
					t.Fatal("tampered package accepted")
				}
			}
			unsigned := append(bytes.Clone(control), payload...)
			if err := VerifySignature(bytes.NewReader(unsigned), int64(len(unsigned)), public); err == nil {
				t.Fatal("unsigned package accepted")
			}
			if err := VerifySignature(bytes.NewReader(body), int64(len(body)), ""); err == nil {
				t.Fatal("untrusted key accepted")
			}
		})
	}
}
