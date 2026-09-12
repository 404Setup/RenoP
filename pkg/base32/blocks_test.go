//go:build !purego && (arm64 || (amd64 && goexperiment.simd))

/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package base32

import (
	"bytes"
	stdbase32 "encoding/base32"
	"testing"
)

// Public API tests also run on scalar-only builds. This verifies that an
// accelerated build actually processes blocks and respects its write bounds.
func TestAcceleratedBlocks(t *testing.T) {
	for _, alphabet := range testAlphabets {
		enc, ref := NewEncoding(alphabet), stdbase32.NewEncoding(alphabet)
		data := bytes.Repeat([]byte("0123456789"), 7)
		encoded := bytes.Repeat([]byte{0xa5}, enc.EncodedLen(len(data))+16)
		n := enc.encodeBlocks(encoded[:len(encoded)-16], data)
		if n == 0 {
			t.Skip("CPU does not support the SIMD backend")
		}
		if n%5 != 0 || n > len(data) || string(encoded[:n/5*8]) != ref.EncodeToString(data[:n]) {
			t.Fatalf("invalid encoded block prefix: %d", n)
		}
		if !bytes.Equal(encoded[n/5*8:], bytes.Repeat([]byte{0xa5}, len(encoded)-n/5*8)) {
			t.Fatal("encoder wrote beyond the returned prefix")
		}
		if enc.alphabet == alphabetCustom {
			continue
		}
		valid := []byte(ref.EncodeToString(data))
		for i := 0; i <= len(valid); i++ {
			src := bytes.Clone(valid)
			if i < len(src) {
				src[i] = '!'
			}
			dst := bytes.Repeat([]byte{0xa5}, len(data)+16)
			n := enc.decodeBlocks(dst[:len(data)], src)
			if n != i/16*16 {
				t.Fatalf("invalid block at %d: consumed %d, want %d", i, n, i/16*16)
			}
			if !bytes.Equal(dst[:n/8*5], data[:n/8*5]) || !bytes.Equal(dst[n/8*5:], bytes.Repeat([]byte{0xa5}, len(dst)-n/8*5)) {
				t.Fatalf("decoder write bounds at input offset %d", i)
			}
		}
	}
}
