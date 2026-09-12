//go:build arm64 && goexperiment.simd && !purego

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

import "simd/archsimd"

// NEON byte lookups and signed per-byte shifts extract sixteen symbols
// from ten source bytes. Six readable trailing bytes make full loads safe.
func (enc *Encoding) encodeBlocks(dst, src []byte) int {
	if len(src) < 16 || len(dst) < 16 {
		return 0
	}
	lo, hi := archsimd.LoadUint8x16(enc.encode[:16]), archsimd.LoadUint8x16(enc.encode[16:])
	left := archsimd.LoadUint8x16([]byte{0, 0, 1, 1, 2, 3, 3, 4, 5, 5, 6, 6, 7, 8, 8, 9})
	right := archsimd.LoadUint8x16([]byte{255, 1, 255, 2, 3, 255, 4, 255, 255, 6, 255, 7, 8, 255, 9, 255})
	leftShift := archsimd.LoadInt8x16([]int8{-3, 2, -1, 4, 1, -2, 3, 0, -3, 2, -1, 4, 1, -2, 3, 0})
	rightShift := archsimd.LoadInt8x16([]int8{0, -6, 0, -4, -7, 0, -5, 0, 0, -6, 0, -4, -7, 0, -5, 0})
	mask, half := archsimd.BroadcastUint8x16(31), archsimd.BroadcastUint8x16(16)
	n := 0
	for len(src) >= 16 && len(dst) >= 16 {
		x := archsimd.LoadUint8x16(src)
		indices := x.LookupOrZero(left).Shift(leftShift).Or(x.LookupOrZero(right).Shift(rightShift)).And(mask)
		lo.LookupOrZero(indices).Or(hi.LookupOrZero(indices.Sub(half))).Store(dst)
		src, dst = src[10:], dst[16:]
		n += 10
	}
	return n
}

func (enc *Encoding) decodeBlocks(dst, src []byte) int {
	if enc.alphabet == alphabetCustom || len(src) < 16 || len(dst) < 10 {
		return 0
	}
	letterMin, letterMax, digitMin, digitMax, letterOffset, digitOffset := byte('A'), byte('Z'), byte('2'), byte('7'), byte('A'), byte('2'-26)
	if enc.alphabet == alphabetHex {
		letterMax, digitMin, digitMax, letterOffset, digitOffset = 'V', '0', '9', 'A'-10, '0'
	}
	lmin, lmax := archsimd.BroadcastUint8x16(letterMin), archsimd.BroadcastUint8x16(letterMax)
	dmin, dmax := archsimd.BroadcastUint8x16(digitMin), archsimd.BroadcastUint8x16(digitMax)
	loff, doff := archsimd.BroadcastUint8x16(letterOffset), archsimd.BroadcastUint8x16(digitOffset)
	a := archsimd.LoadUint8x16([]byte{0, 1, 3, 4, 6, 8, 9, 11, 12, 14, 255, 255, 255, 255, 255, 255})
	b := archsimd.LoadUint8x16([]byte{1, 2, 4, 5, 7, 9, 10, 12, 13, 15, 255, 255, 255, 255, 255, 255})
	c := archsimd.LoadUint8x16([]byte{255, 3, 255, 6, 255, 255, 11, 255, 14, 255, 255, 255, 255, 255, 255, 255})
	ashift := archsimd.LoadInt8x16([]int8{3, 6, 4, 7, 5, 3, 6, 4, 7, 5, 0, 0, 0, 0, 0, 0})
	bshift := archsimd.LoadInt8x16([]int8{-2, 1, -1, 2, 0, -2, 1, -1, 2, 0, 0, 0, 0, 0, 0, 0})
	cshift := archsimd.LoadInt8x16([]int8{0, -4, 0, -3, 0, 0, -4, 0, -3, 0, 0, 0, 0, 0, 0, 0})
	n := 0
	for len(src) >= 16 && len(dst) >= 10 {
		x := archsimd.LoadUint8x16(src)
		letters := x.GreaterEqual(lmin).And(x.LessEqual(lmax))
		digits := x.GreaterEqual(dmin).And(x.LessEqual(dmax))
		valid := letters.Or(digits).ToInt8x16().ToBits().ReshapeToUint64s()
		if valid.GetElem(0)&valid.GetElem(1) != ^uint64(0) {
			break
		}
		values := x.Sub(loff).IfElse(letters, x.Sub(doff))
		packed := values.LookupOrZero(a).Shift(ashift).Or(values.LookupOrZero(b).Shift(bshift)).Or(values.LookupOrZero(c).Shift(cshift))
		packed.StorePart(dst[:10])
		src, dst = src[16:], dst[10:]
		n += 16
	}
	return n
}
