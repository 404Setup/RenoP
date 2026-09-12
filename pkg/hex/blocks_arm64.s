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

#include "textflag.h"

// The Go wrapper supplies a multiple of sixteen source bytes and twice as
// many destination bytes. Every load and store stays within those slices.
TEXT ·encodeNEON(SB), NOSPLIT, $0-48
	MOVD dst_base+0(FP), R0
	MOVD src_base+24(FP), R1
	MOVD src_len+32(FP), R2
	MOVD $·hexAlphabet(SB), R3
	VLD1 (R3), [V16.B16]
	VMOVI $15, V17.B16
	VMOVI $252, V18.B16 // signed shift count -4

encode_loop:
	VLD1.P 16(R1), [V0.B16]
	VUSHL V18.B16, V0.B16, V1.B16
	VAND V17.B16, V0.B16, V2.B16
	VTBL V1.B16, [V16.B16], V3.B16
	VTBL V2.B16, [V16.B16], V4.B16
	VZIP1 V4.B16, V3.B16, V5.B16
	VZIP2 V4.B16, V3.B16, V6.B16
	VST1.P [V5.B16, V6.B16], 32(R0)
	SUB $16, R2
	CBNZ R2, encode_loop
	RET

// Decode thirty-two characters per iteration, checking both vectors before
// storing. The caller replays the first invalid block through encoding/hex.
TEXT ·decodeNEON(SB), NOSPLIT, $0-56
	MOVD dst_base+0(FP), R0
	MOVD src_base+24(FP), R1
	MOVD src_len+32(FP), R2
	MOVD R2, R3
	VMOVI $48, V16.B16 // '0'
	VMOVI $9, V17.B16
	VMOVI $97, V18.B16 // 'a'
	VMOVI $5, V19.B16
	VMOVI $32, V20.B16 // ASCII case bit
	VMOVI $10, V21.B16
	VMOVI $4, V22.B16

decode_loop:
	VLD1 (R1), [V0.B16, V1.B16]
	VSUB V16.B16, V0.B16, V2.B16
	VSUB V16.B16, V1.B16, V3.B16
	VORR V20.B16, V0.B16, V4.B16
	VORR V20.B16, V1.B16, V5.B16
	VSUB V18.B16, V4.B16, V4.B16
	VSUB V18.B16, V5.B16, V5.B16
	VCMHS V2.B16, V17.B16, V6.B16
	VCMHS V3.B16, V17.B16, V7.B16
	VCMHS V4.B16, V19.B16, V8.B16
	VCMHS V5.B16, V19.B16, V9.B16
	VORR V8.B16, V6.B16, V10.B16
	VORR V9.B16, V7.B16, V11.B16
	VAND V11.B16, V10.B16, V10.B16
	VUMINV V10.B16, V11
	VMOV V11.B[0], R4
	CMP $255, R4
	BNE decode_done
	VADD V21.B16, V4.B16, V4.B16
	VADD V21.B16, V5.B16, V5.B16
	VAND V6.B16, V2.B16, V2.B16
	VAND V7.B16, V3.B16, V3.B16
	VAND V8.B16, V4.B16, V4.B16
	VAND V9.B16, V5.B16, V5.B16
	VORR V4.B16, V2.B16, V0.B16
	VORR V5.B16, V3.B16, V1.B16
	VUZP1 V1.B16, V0.B16, V2.B16 // high nibbles
	VUZP2 V1.B16, V0.B16, V3.B16 // low nibbles
	VUSHL V22.B16, V2.B16, V2.B16
	VORR V3.B16, V2.B16, V2.B16
	VST1.P [V2.B16], 16(R0)
	ADD $32, R1
	SUB $32, R2
	CBNZ R2, decode_loop

decode_done:
	SUB R2, R3, R3
	MOVD R3, ret+48(FP)
	RET

DATA ·hexAlphabet+0(SB)/8, $0x3736353433323130
DATA ·hexAlphabet+8(SB)/8, $0x6665646362613938
GLOBL ·hexAlphabet(SB), RODATA|NOPTR, $16
