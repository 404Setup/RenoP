//go:build arm64 && !purego

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

// Bound each non-preemptible assembly call. NEON is an arm64 baseline feature.
const neonChunk = 16 * 1024

func encodeBlocks(dst, src []byte) int {
	n := 0
	for len(src) >= 16 && len(dst) >= 32 {
		size := min(len(src)/16, len(dst)/32, neonChunk/16) * 16
		encodeNEON(dst[:size*2], src[:size])
		src, dst = src[size:], dst[size*2:]
		n += size
	}
	return n
}

func decodeBlocks(dst, src []byte) int {
	n := 0
	for len(src) >= 32 && len(dst) >= 16 {
		size := min(len(src)/32, len(dst)/16, neonChunk/32) * 32
		consumed := decodeNEON(dst[:size/2], src[:size])
		n += consumed
		if consumed != size {
			break
		}
		src, dst = src[consumed:], dst[consumed/2:]
	}
	return n
}

//go:noescape
func encodeNEON(dst, src []byte)

//go:noescape
func decodeNEON(dst, src []byte) int
