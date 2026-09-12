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

package hex

import "simd/archsimd"

// Keep the lookup table in static storage. A string-to-slice conversion can
// introduce legacy SSE copies and costly transitions between SIMD calls.
var encodeAlphabet = [32]byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f',
}

var decodePairs = [32]int8{
	16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1,
	16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1, 16, 1,
}

var decodePack = [32]int8{
	0, 2, 4, 6, 8, 10, 12, 14, -1, -1, -1, -1, -1, -1, -1, -1,
	0, 2, 4, 6, 8, 10, 12, 14, -1, -1, -1, -1, -1, -1, -1, -1,
}

// encodeBlocks consumes complete vectors and returns the source bytes consumed.
func encodeBlocks(dst, src []byte) int {
	if !archsimd.X86.AVX2() || len(src) < 16 || len(dst) < 32 {
		return 0
	}
	table := archsimd.LoadUint8x32(encodeAlphabet[:])
	mask := archsimd.BroadcastUint8x32(15)
	n := 0
	for len(src) >= 32 && len(dst) >= 64 {
		x := archsimd.LoadUint8x32(src)
		hi := x.ReshapeToUint16s().ShiftAllRight(4).ReshapeToUint8s().And(mask)
		lo := x.And(mask)
		hi = table.PermuteOrZeroGrouped(hi.BitsToInt8())
		lo = table.PermuteOrZeroGrouped(lo.BitsToInt8())
		a, b := hi.InterleaveLoGrouped(lo), hi.InterleaveHiGrouped(lo)
		// Interleaves operate within each 128-bit lane; restore byte order.
		a.ConcatPermute128Scalars(0, 2, b).Store(dst)
		a.ConcatPermute128Scalars(1, 3, b).Store(dst[32:])
		src, dst = src[32:], dst[64:]
		n += 32
	}
	if len(src) >= 16 && len(dst) >= 32 {
		x := archsimd.LoadUint8x16(src)
		hi := x.ReshapeToUint16s().ShiftAllRight(4).ReshapeToUint8s().And(mask.GetLo())
		lo := x.And(mask.GetLo())
		hi = table.GetLo().PermuteOrZero(hi.BitsToInt8())
		lo = table.GetLo().PermuteOrZero(lo.BitsToInt8())
		hi.InterleaveLo(lo).Store(dst)
		hi.InterleaveHi(lo).Store(dst[16:])
		n += 16
	}
	// The caller and the standard-library tail may use legacy SSE instructions.
	archsimd.ClearAVXUpperBits()
	return n
}

// decodeBlocks validates a whole vector before writing exactly sixteen bytes.
// Invalid blocks and tails are left to the standard decoder.
func decodeBlocks(dst, src []byte) int {
	if !archsimd.X86.AVX2() || len(src) < 32 || len(dst) < 16 {
		return 0
	}
	zero, nine := archsimd.BroadcastUint8x32('0'), archsimd.BroadcastUint8x32(9)
	letter, five := archsimd.BroadcastUint8x32('a'), archsimd.BroadcastUint8x32(5)
	caseBit, ten := archsimd.BroadcastUint8x32(0x20), archsimd.BroadcastUint8x32(10)
	pairs := archsimd.LoadInt8x32(decodePairs[:])
	pack := archsimd.LoadInt8x32(decodePack[:])
	n := 0
	for len(src) >= 32 && len(dst) >= 16 {
		x := archsimd.LoadUint8x32(src)
		digits := x.Sub(zero)
		letters := x.Or(caseBit).Sub(letter)
		// Unsigned max/equality avoids the sign conversions of AVX2 comparisons.
		isDigit, isLetter := digits.Max(nine).Equal(nine), letters.Max(five).Equal(five)
		if isDigit.Or(isLetter).ToBits() != 0xffffffff {
			break
		}
		values := digits.IfElse(isDigit, letters.Add(ten))
		// Narrow by shuffling: SaturateToUint8 requires AVX-512, not AVX2.
		packed := values.DotProductPairsSaturated(pairs).ToBits().ReshapeToUint8s().PermuteOrZeroGrouped(pack)
		packed.GetLo().ReshapeToUint64s().InterleaveLo(packed.GetHi().ReshapeToUint64s()).ReshapeToUint8s().Store(dst)
		src, dst = src[32:], dst[16:]
		n += 32
	}
	archsimd.ClearAVXUpperBits()
	return n
}
