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

import "slices"

/*
 * Encoder
 */

// Encode encodes src using the encoding enc,
// writing [Encoding.EncodedLen](len(src)) bytes to dst.
//
// The encoding pads the output to a multiple of 8 bytes,
// so Encode is not appropriate for use on individual blocks
// of a large data stream. Use [NewEncoder] instead.
func (enc *Encoding) Encode(dst, src []byte) {
	if len(src) == 0 {
		return
	}
	// enc is a pointer receiver, so the use of enc.encode within the hot
	// loop below means a nil check at every operation. Lift that nil check
	// outside of the loop to speed up the encoder.
	_ = enc.encode

	// Vector setup costs more than scalar encoding for short secrets/tokens.
	if len(src) >= 64 {
		if n := enc.encodeBlocks(dst, src); n > 0 {
			src = src[n:]
			dst = dst[n/5*8:]
		}
	}

	for len(src) >= 5 {
		// Combining two 32 bit loads allows the same code to be used
		// for 32 and 64 bit platforms.
		hi := uint32(src[0])<<24 | uint32(src[1])<<16 | uint32(src[2])<<8 | uint32(src[3])
		lo := hi<<8 | uint32(src[4])

		_ = dst[7] // Eliminate bounds checks below.
		dst[0] = enc.encode[(hi>>27)&0x1F]
		dst[1] = enc.encode[(hi>>22)&0x1F]
		dst[2] = enc.encode[(hi>>17)&0x1F]
		dst[3] = enc.encode[(hi>>12)&0x1F]
		dst[4] = enc.encode[(hi>>7)&0x1F]
		dst[5] = enc.encode[(hi>>2)&0x1F]
		dst[6] = enc.encode[(lo>>5)&0x1F]
		dst[7] = enc.encode[(lo)&0x1F]

		src = src[5:]
		dst = dst[8:]
	}

	// Add the remaining small block
	if len(src) == 0 {
		return
	}

	// Encode the remaining bytes in reverse order.
	val := uint32(0)
	switch len(src) {
	case 4:
		val |= uint32(src[3])
		dst[6] = enc.encode[val<<3&0x1F]
		dst[5] = enc.encode[val>>2&0x1F]
		fallthrough
	case 3:
		val |= uint32(src[2]) << 8
		dst[4] = enc.encode[val>>7&0x1F]
		fallthrough
	case 2:
		val |= uint32(src[1]) << 16
		dst[3] = enc.encode[val>>12&0x1F]
		dst[2] = enc.encode[val>>17&0x1F]
		fallthrough
	case 1:
		val |= uint32(src[0]) << 24
		dst[1] = enc.encode[val>>22&0x1F]
		dst[0] = enc.encode[val>>27&0x1F]
	}

	// Pad the final quantum
	if enc.padChar != NoPadding {
		nPad := (len(src) * 8 / 5) + 1
		for i := nPad; i < 8; i++ {
			dst[i] = byte(enc.padChar)
		}
	}
}

// AppendEncode appends the base32 encoded src to dst
// and returns the extended buffer.
func (enc *Encoding) AppendEncode(dst, src []byte) []byte {
	n := enc.EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	enc.Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}

// EncodeToString returns the base32 encoding of src.
func (enc *Encoding) EncodeToString(src []byte) string {
	buf := make([]byte, enc.EncodedLen(len(src)))
	enc.Encode(buf, src)
	return string(buf)
}
