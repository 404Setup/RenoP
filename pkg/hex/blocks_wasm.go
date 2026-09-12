//go:build wasm && goexperiment.simd && !purego

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

// WebAssembly SIMD is selected at build time; runtimes must support SIMD128.
func encodeBlocks(dst, src []byte) int {
	if len(src) < 16 || len(dst) < 32 {
		return 0
	}
	table := archsimd.LoadUint8x16([]byte("0123456789abcdef")).BitsToInt8()
	mask := archsimd.BroadcastUint8x16(15)
	hiLo := archsimd.LoadInt8x16([]int8{0, -1, 1, -1, 2, -1, 3, -1, 4, -1, 5, -1, 6, -1, 7, -1})
	loLo := archsimd.LoadInt8x16([]int8{-1, 0, -1, 1, -1, 2, -1, 3, -1, 4, -1, 5, -1, 6, -1, 7})
	hiHi := archsimd.LoadInt8x16([]int8{8, -1, 9, -1, 10, -1, 11, -1, 12, -1, 13, -1, 14, -1, 15, -1})
	loHi := archsimd.LoadInt8x16([]int8{-1, 8, -1, 9, -1, 10, -1, 11, -1, 12, -1, 13, -1, 14, -1, 15})
	n := 0
	for len(src) >= 16 && len(dst) >= 32 {
		x := archsimd.LoadUint8x16(src)
		hi := table.LookupOrZero(x.ShiftAllRight(4).BitsToInt8())
		lo := table.LookupOrZero(x.And(mask).BitsToInt8())
		hi.LookupOrZero(hiLo).Or(lo.LookupOrZero(loLo)).ToBits().Store(dst)
		hi.LookupOrZero(hiHi).Or(lo.LookupOrZero(loHi)).ToBits().Store(dst[16:])
		src, dst = src[16:], dst[32:]
		n += 16
	}
	return n
}

func decodeBlocks(dst, src []byte) int {
	if len(src) < 16 || len(dst) < 8 {
		return 0
	}
	zero, nine := archsimd.BroadcastUint8x16('0'), archsimd.BroadcastUint8x16(9)
	letter, five := archsimd.BroadcastUint8x16('a'), archsimd.BroadcastUint8x16(5)
	caseBit, ten := archsimd.BroadcastUint8x16(0x20), archsimd.BroadcastUint8x16(10)
	mask := archsimd.BroadcastUint16x8(0xf0)
	pack := archsimd.LoadInt8x16([]int8{0, 2, 4, 6, 8, 10, 12, 14, -1, -1, -1, -1, -1, -1, -1, -1})
	n := 0
	for len(src) >= 16 && len(dst) >= 8 {
		x := archsimd.LoadUint8x16(src)
		digits, letters := x.Sub(zero), x.Or(caseBit).Sub(letter)
		isDigit, isLetter := digits.LessEqual(nine), letters.LessEqual(five)
		valid := isDigit.Or(isLetter).ToInt8x16().ToBits().ReshapeToUint64s()
		if valid.GetElem(0) != ^uint64(0) || valid.GetElem(1) != ^uint64(0) {
			break
		}
		values := digits.IfElse(isDigit, letters.Add(ten)).ReshapeToUint16s()
		packed := values.ShiftAllLeft(4).And(mask).Or(values.ShiftAllRight(8))
		packed.ReshapeToUint8s().BitsToInt8().LookupOrZero(pack).ToBits().StorePart(dst[:8])
		src, dst = src[16:], dst[8:]
		n += 16
	}
	return n
}
