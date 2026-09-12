//go:build arm64 && !goexperiment.simd && !purego

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

// Keep individual assembly calls bounded so large inputs can be preempted
// between calls. NEON is part of the arm64 baseline; no CPU probe is needed.
const neonChunk = 16 * 1024

func (enc *Encoding) encodeBlocks(dst, src []byte) int {
	n := 0
	for len(src) >= 16 && len(dst) >= 16 {
		window := src[:min(len(src), neonChunk)]
		// The assembly reads sixteen bytes and commits ten per iteration.
		blocks := min((len(window)-6)/10, len(dst)/16)
		if blocks == 0 {
			break
		}
		consumed := encodeNEON(dst, window[:blocks*10+6], &enc.encode)
		n += consumed
		src, dst = src[consumed:], dst[consumed/5*8:]
	}
	return n
}

func (enc *Encoding) decodeBlocks(dst, src []byte) int {
	if enc.alphabet == alphabetCustom {
		return 0
	}
	n := 0
	for len(src) >= 16 && len(dst) >= 10 {
		blocks := min(len(src)/16, len(dst)/10, neonChunk/16)
		consumed := decodeNEON(dst, src[:blocks*16], uint64(enc.alphabet))
		n += consumed
		if consumed < blocks*16 {
			break
		}
		src, dst = src[consumed:], dst[consumed/8*5:]
	}
	return n
}

//go:noescape
func encodeNEON(dst, src []byte, alphabet *[32]byte) int

//go:noescape
func decodeNEON(dst, src []byte, alphabet uint64) int
