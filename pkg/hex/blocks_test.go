/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package hex

import (
	"bytes"
	stdhex "encoding/hex"
	"testing"
)

func checkAcceleratedBlocks(t *testing.T) {
	t.Helper()
	src := make([]byte, 65536)
	for i := range src {
		src[i] = byte(i)
	}
	dst := make([]byte, len(src)*2)
	if n := encodeBlocks(dst, src); n != len(src) || string(dst) != stdhex.EncodeToString(src) {
		t.Fatalf("encode backend consumed %d of %d bytes", n, len(src))
	}
	decoded := make([]byte, len(src))
	if n := decodeBlocks(decoded, dst); n != len(dst) || !bytes.Equal(decoded, src) {
		t.Fatalf("decode backend consumed %d of %d characters", n, len(dst))
	}
	for _, pos := range []int{0, 15, 16, 31, 32, 16383, 16384, len(dst) - 1} {
		original := dst[pos]
		dst[pos] = '!'
		clear(decoded)
		n := decodeBlocks(decoded, dst)
		if n > pos || n%2 != 0 || !bytes.Equal(decoded[:n/2], src[:n/2]) || !bytes.Equal(decoded[n/2:], make([]byte, len(decoded)-n/2)) {
			t.Fatalf("invalid block at %d: backend consumed %d or wrote unvalidated bytes", pos, n)
		}
		dst[pos] = original
	}
}
