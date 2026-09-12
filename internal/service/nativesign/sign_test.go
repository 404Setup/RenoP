/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package nativesign

import (
	"archive/tar"
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"renop/internal/config"
	"renop/internal/configstore"
	"renop/internal/core"
	"renop/internal/testutil"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
)

func TestPersistentIndexSignatures(t *testing.T) {
	state := core.NewAppState()
	state.Inner.Config.Store(config.DefaultConfig())
	settings := filepath.Join(testutil.TempDir(t), "settings.db")
	if err := EnsureKeys(state, settings); err != nil {
		t.Fatal(err)
	}
	keys := state.Inner.Config.Load().NativeSigningKeys
	reloaded, err := configstore.Load(settings, "")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NativeSigningKeys != keys {
		t.Fatal("signing keys did not survive reload")
	}
	state.Inner.Config.Store(reloaded)
	if err := EnsureKeys(state, settings); err != nil {
		t.Fatal(err)
	}
	if state.Inner.Config.Load().NativeSigningKeys != keys {
		t.Fatal("restart rotated signing keys")
	}
	encoded, err := json.Marshal(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("PRIVATE KEY")) || bytes.Contains(encoded, []byte("native_signing")) {
		t.Fatal("private signing keys exposed in JSON")
	}
	public, err := PublicKey(keys, false)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(public))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Origin: RenoP\nSHA256:\n 123 4 main/Packages\n")
	signature, err := Detached(keys, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openpgp.CheckArmoredDetachedSignature(ring, bytes.NewReader(data), bytes.NewReader(signature), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := openpgp.CheckArmoredDetachedSignature(ring, strings.NewReader("tampered"), bytes.NewReader(signature), nil); err == nil {
		t.Fatal("accepted tampered release")
	}
	clear, err := Cleartext(keys, data)
	if err != nil {
		t.Fatal(err)
	}
	block, rest := clearsign.Decode(clear)
	if block == nil || len(rest) != 0 {
		t.Fatal("invalid clearsigned document")
	}
	if _, err := openpgp.CheckDetachedSignature(ring, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil); err != nil {
		t.Fatal(err)
	}

	var index bytes.Buffer
	compressed := gzip.NewWriter(&index)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: "APKINDEX", Size: int64(len(data)), Mode: 0644}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	signed, err := APK(keys, index.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(signed)
	gz, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	gz.Multistream(false)
	tarReader := tar.NewReader(gz)
	header, err := tarReader.Next()
	if err != nil || header.Name != ".SIGN.RSA256.renop.rsa.pub" {
		t.Fatalf("signature member: %v, %v", header, err)
	}
	rawSignature, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, gz); err != nil {
		t.Fatal(err)
	}
	remaining, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(remaining, index.Bytes()) {
		t.Fatal("signature changed the compressed index")
	}
	signer, err := load(keys)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(remaining)
	if err := rsa.VerifyPKCS1v15(&signer.apk.PublicKey, crypto.SHA256, sum[:], rawSignature); err != nil {
		t.Fatal(err)
	}
	remaining[0] ^= 1
	sum = sha256.Sum256(remaining)
	if err := rsa.VerifyPKCS1v15(&signer.apk.PublicKey, crypto.SHA256, sum[:], rawSignature); err == nil {
		t.Fatal("accepted tampered APK index")
	}
}
