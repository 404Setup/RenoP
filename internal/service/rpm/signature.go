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
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"io"
	"strings"

	"renop/pkg/hex"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/emmansun/base64"
)

var ErrSignature = errors.New("RPM package requires a valid trusted signature")

// VerifySignature verifies native RPM signatures, including the payload trust
// chain for header-only signatures. A signature packet's presence is not proof.
// Tag ranges follow rpm.org's Signatures and Digests specification.
func VerifySignature(reader io.ReaderAt, size int64, publicKey string) error {
	if size <= 112 || size > 8<<30 || len(publicKey) == 0 || len(publicKey) > 64<<10 {
		return ErrSignature
	}
	ring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(publicKey))
	if err != nil || len(ring) == 0 {
		return ErrSignature
	}
	signatures, err := readHeader(reader, size, 96)
	if err != nil {
		return ErrSignature
	}
	start := (signatures.end + 7) &^ 7
	metadata, err := readHeader(reader, size, start)
	if err != nil || metadata.end >= size {
		return ErrSignature
	}
	for _, tag := range []uint32{1002, 1005} {
		if signature := signatures.binary(tag); len(signature) > 0 {
			if _, err := openpgp.CheckDetachedSignature(ring, io.NewSectionReader(reader, start, size-start), bytes.NewReader(signature), nil); err == nil {
				return nil
			}
		}
	}
	var candidates [][]byte
	for _, tag := range []uint32{267, 268} {
		if signature := signatures.binary(tag); len(signature) > 0 {
			candidates = append(candidates, signature)
		}
	}
	// RPM 6 can carry several base64-encoded OpenPGP signatures.
	if encoded, err := signatures.strings(278); err == nil && len(encoded) <= 16 {
		for _, text := range encoded {
			if len(text) <= 64<<10 {
				if signature, err := base64.StdEncoding.DecodeString(text); err == nil {
					candidates = append(candidates, signature)
				}
			}
		}
	}
	verified := false
	for _, signature := range candidates {
		if _, err := openpgp.CheckDetachedSignature(ring, io.NewSectionReader(reader, start, metadata.end-start), bytes.NewReader(signature), nil); err == nil {
			verified = true
			break
		}
	}
	if !verified {
		return ErrSignature
	}
	// Modern RPM header signatures authenticate the compressed payload digest.
	// Require a supported digest; otherwise unsigned payload bytes could change.
	for _, tag := range []uint32{5092, 5121} {
		digests, err := metadata.strings(tag)
		if err != nil {
			return ErrSignature
		}
		if len(digests) == 0 {
			continue
		}
		if len(digests) != 1 {
			return ErrSignature
		}
		var sum []byte
		if tag == 5092 {
			hash := sha256.New()
			if _, err := io.Copy(hash, io.NewSectionReader(reader, metadata.end, size-metadata.end)); err != nil {
				return err
			}
			sum = hash.Sum(nil)
		} else {
			hash := sha512.New()
			if _, err := io.Copy(hash, io.NewSectionReader(reader, metadata.end, size-metadata.end)); err != nil {
				return err
			}
			sum = hash.Sum(nil)
		}
		if !strings.EqualFold(hex.EncodeToString(sum), digests[0]) {
			return ErrSignature
		}
		return nil
	}
	return ErrSignature
}

func (h *header) binary(tag uint32) []byte {
	entry, ok := h.entries[tag]
	if !ok || entry.kind != 7 || entry.count == 0 || entry.count > 64<<10 || uint64(entry.offset)+uint64(entry.count) > uint64(len(h.data)) {
		return nil
	}
	return h.data[entry.offset : entry.offset+entry.count]
}
