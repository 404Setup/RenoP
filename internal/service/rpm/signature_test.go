/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package rpm

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"renop/pkg/hex"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func TestRPMSignatureAuthenticatesHeaderAndPayload(t *testing.T) {
	key, err := openpgp.NewEntity("publisher", "", "test@example.invalid", &packet.Config{RSABits: 2048})
	if err != nil {
		t.Fatal(err)
	}
	var public bytes.Buffer
	writer, err := armor.Encode(&public, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := key.Serialize(writer); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("payload"))
	unsigned := testRPMFields(t, map[uint32]string{5092: hex.EncodeToString(digest[:])})
	for _, full := range []bool{false, true} {
		metadata, err := readHeader(bytes.NewReader(unsigned), int64(len(unsigned)), 112)
		if err != nil {
			t.Fatal(err)
		}
		end := metadata.end
		tag := uint32(268)
		if full {
			end = int64(len(unsigned))
			tag = 1002
		}
		var signature bytes.Buffer
		if err := openpgp.DetachSign(&signature, key, bytes.NewReader(unsigned[112:end]), &packet.Config{DefaultHash: crypto.SHA256}); err != nil {
			t.Fatal(err)
		}
		header := make([]byte, 32)
		copy(header, []byte{0x8e, 0xad, 0xe8, 1})
		binary.BigEndian.PutUint32(header[8:12], 1)
		binary.BigEndian.PutUint32(header[12:16], uint32(signature.Len()))
		binary.BigEndian.PutUint32(header[16:20], tag)
		binary.BigEndian.PutUint32(header[20:24], 7)
		binary.BigEndian.PutUint32(header[28:32], uint32(signature.Len()))
		body := append(bytes.Clone(unsigned[:96]), header...)
		body = append(body, signature.Bytes()...)
		for len(body)%8 != 0 {
			body = append(body, 0)
		}
		start := len(body)
		body = append(body, unsigned[112:]...)
		if err := VerifySignature(bytes.NewReader(body), int64(len(body)), public.String()); err != nil {
			t.Fatalf("full=%v: %v", full, err)
		}
		for _, offset := range []int{start + 20, len(body) - 1} {
			changed := bytes.Clone(body)
			changed[offset] ^= 1
			if err := VerifySignature(bytes.NewReader(changed), int64(len(changed)), public.String()); err == nil {
				t.Fatal("tampered package accepted")
			}
		}
		if err := VerifySignature(bytes.NewReader(body), int64(len(body)), ""); err == nil {
			t.Fatal("unknown signing key accepted")
		}
	}
	if err := VerifySignature(bytes.NewReader(unsigned), int64(len(unsigned)), public.String()); err == nil {
		t.Fatal("unsigned RPM accepted")
	}
}
