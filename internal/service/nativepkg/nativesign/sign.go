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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"

	"github.com/klauspost/compress/gzip"

	"renop/internal/config"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func Detached(keys config.NativeSigningKeys, data []byte) ([]byte, error) {
	signer, err := load(keys)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&output, signer.pgp, bytes.NewReader(data), &packet.Config{DefaultHash: crypto.SHA256}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func Cleartext(keys config.NativeSigningKeys, data []byte) ([]byte, error) {
	signer, err := load(keys)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer, err := clearsign.Encode(&output, signer.pgp.PrivateKey, &packet.Config{DefaultHash: crypto.SHA256})
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// APK prepends a separate signature gzip member, as required by apk-tools.
func APK(keys config.NativeSigningKeys, index []byte) ([]byte, error) {
	signer, err := load(keys)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(index)
	signature, err := rsa.SignPKCS1v15(rand.Reader, signer.apk, crypto.SHA256, sum[:])
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: ".SIGN.RSA256.renop.rsa.pub", Mode: 0644, Size: int64(len(signature))}); err != nil {
		return nil, err
	}
	if _, err := archive.Write(signature); err != nil {
		return nil, err
	}
	// Flush entry padding without the tar end markers; the next gzip member
	// continues the logical index archive.
	if err := archive.Flush(); err != nil {
		return nil, err
	}
	if err := compressed.Close(); err != nil {
		return nil, err
	}
	output.Write(index)
	return output.Bytes(), nil
}
