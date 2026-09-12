//go:build amd64 && goexperiment.simd && !purego

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

// encodeBlocks encodes ten bytes per iteration using two sets of eight
// 16-bit windows. Multiplication by powers of two extracts each 5-bit symbol.
// Loads require sixteen readable bytes; the scalar codec handles the tail.
func (enc *Encoding) encodeBlocks(dst, src []byte) int {
	if !archsimd.X86.AVX2() || len(src) < 16 || len(dst) < 16 {
		return 0
	}
	lo := archsimd.LoadUint8x16(enc.encode[:16])
	hi := archsimd.LoadUint8x16(enc.encode[16:])
	first := archsimd.LoadInt8x16([]int8{-1, 0, 1, 0, -1, 1, 2, 1, 3, 2, -1, 3, 4, 3, -1, 4})
	second := archsimd.LoadInt8x16([]int8{-1, 5, 6, 5, -1, 6, 7, 6, 8, 7, -1, 8, 9, 8, -1, 9})
	factor := archsimd.LoadUint16x8([]uint16{32, 1024, 128, 4096, 512, 64, 2048, 256})
	mask := archsimd.BroadcastUint16x8(31)
	pack := archsimd.LoadInt8x16([]int8{0, 2, 4, 6, 8, 10, 12, 14, -1, -1, -1, -1, -1, -1, -1, -1})
	limit := archsimd.BroadcastUint8x16(16)
	n := 0
	for len(src) >= 16 && len(dst) >= 16 {
		x := archsimd.LoadUint8x16(src)
		a := x.PermuteOrZero(first).ReshapeToUint16s().MulHigh(factor).And(mask).ReshapeToUint8s().PermuteOrZero(pack)
		b := x.PermuteOrZero(second).ReshapeToUint16s().MulHigh(factor).And(mask).ReshapeToUint8s().PermuteOrZero(pack)
		indices := a.ReshapeToUint64s().InterleaveLo(b.ReshapeToUint64s()).ReshapeToUint8s()
		low := lo.PermuteOrZero(indices.BitsToInt8())
		high := hi.PermuteOrZero(indices.BitsToInt8())
		low.IfElse(indices.Less(limit), high).Store(dst)
		src, dst = src[10:], dst[16:]
		n += 10
	}
	return n
}

// decodeBlocks stops before the first invalid block without writing it. The
// scalar decoder then preserves padding, partial output and error offsets.
func (enc *Encoding) decodeBlocks(dst, src []byte) int {
	if !archsimd.X86.AVX2() || enc.alphabet == alphabetCustom || len(src) < 16 || len(dst) < 10 {
		return 0
	}
	letterMin, letterMax, digitMin, digitMax, letterOffset, digitOffset := byte('A'), byte('Z'), byte('2'), byte('7'), byte('A'), byte('2'-26)
	if enc.alphabet == alphabetHex {
		letterMax, digitMin, digitMax, letterOffset, digitOffset = 'V', '0', '9', 'A'-10, '0'
	}
	lmin, lmax := archsimd.BroadcastUint8x16(letterMin), archsimd.BroadcastUint8x16(letterMax)
	dmin, dmax := archsimd.BroadcastUint8x16(digitMin), archsimd.BroadcastUint8x16(digitMax)
	loff, doff := archsimd.BroadcastUint8x16(letterOffset), archsimd.BroadcastUint8x16(digitOffset)
	pairs := archsimd.LoadInt8x16([]int8{32, 1, 32, 1, 32, 1, 32, 1, 32, 1, 32, 1, 32, 1, 32, 1})
	quads := archsimd.LoadInt16x8([]int16{1024, 1, 1024, 1, 1024, 1, 1024, 1})
	mask := archsimd.BroadcastUint64x2(0xfffff00000)
	pack := archsimd.LoadInt8x16([]int8{4, 3, 2, 1, 0, 12, 11, 10, 9, 8, -1, -1, -1, -1, -1, -1})
	n := 0
	for len(src) >= 16 && len(dst) >= 10 {
		x := archsimd.LoadUint8x16(src)
		letters := x.GreaterEqual(lmin).And(x.LessEqual(lmax))
		digits := x.GreaterEqual(dmin).And(x.LessEqual(dmax))
		if letters.Or(digits).ToBits() != 0xffff {
			break
		}
		values := x.Sub(loff).IfElse(letters, x.Sub(doff))
		words := values.DotProductPairsSaturated(pairs).DotProductPairs(quads).ToBits().ReshapeToUint64s()
		packed := words.ShiftAllLeft(20).And(mask).Or(words.ShiftAllRight(32)).ReshapeToUint8s().PermuteOrZero(pack)
		packed.StorePart(dst[:10])
		src, dst = src[16:], dst[10:]
		n += 16
	}
	return n
}
