//go:build arm64 && !goexperiment.simd && !purego

#include "textflag.h"

// encodeNEON consumes ten bytes and produces sixteen symbols. All vector
// loads stay within src; the caller leaves the final six bytes to Go.
TEXT ·encodeNEON(SB), NOSPLIT, $0-64
	MOVD dst_base+0(FP), R0
	MOVD src_base+24(FP), R1
	MOVD src_len+32(FP), R2
	MOVD R2, R3
	MOVD alphabet+48(FP), R4
	VLD1 (R4), [V16.B16, V17.B16]
	MOVD $·encodeTables(SB), R4
	VLD1 (R4), [V18.B16, V19.B16, V20.B16, V21.B16]
	VMOVI $31, V22.B16

encode_loop:
	CMP $16, R2
	BLT encode_done
	VLD1 (R1), [V0.B16]
	VTBL V18.B16, [V0.B16], V1.B16
	VTBL V19.B16, [V0.B16], V2.B16
	VUSHL V20.B16, V1.B16, V1.B16
	VUSHL V21.B16, V2.B16, V2.B16
	VORR V2.B16, V1.B16, V1.B16
	VAND V22.B16, V1.B16, V1.B16
	VTBL V1.B16, [V16.B16, V17.B16], V2.B16
	VST1.P [V2.B16], 16(R0)
	ADD $10, R1
	SUB $10, R2
	B encode_loop

encode_done:
	SUB R2, R3, R3
	MOVD R3, ret+56(FP)
	RET

// decodeNEON validates all sixteen symbols before storing exactly ten bytes.
// It stops at the first special/invalid block for the Go decoder to inspect.
TEXT ·decodeNEON(SB), NOSPLIT, $0-64
	MOVD dst_base+0(FP), R0
	MOVD src_base+24(FP), R1
	MOVD src_len+32(FP), R2
	MOVD R2, R3
	MOVD alphabet+48(FP), R4
	VMOVI $65, V16.B16 // letter minimum
	CMP $2, R4 // alphabetHex
	BEQ decode_hex
	VMOVI $90, V17.B16
	VMOVI $50, V18.B16
	VMOVI $55, V19.B16
	VMOVI $65, V20.B16
	VMOVI $24, V21.B16
	B decode_tables
decode_hex:
	VMOVI $86, V17.B16
	VMOVI $48, V18.B16
	VMOVI $57, V19.B16
	VMOVI $55, V20.B16
	VMOVI $48, V21.B16
decode_tables:
	MOVD $·decodeTables(SB), R4
	VLD1.P 64(R4), [V22.B16, V23.B16, V24.B16, V25.B16]
	VLD1 (R4), [V26.B16, V27.B16]

decode_loop:
	CMP $16, R2
	BLT decode_done
	VLD1 (R1), [V0.B16]
	VCMHS V16.B16, V0.B16, V1.B16
	VCMHS V0.B16, V17.B16, V2.B16
	VAND V2.B16, V1.B16, V1.B16
	VCMHS V18.B16, V0.B16, V3.B16
	VCMHS V0.B16, V19.B16, V4.B16
	VAND V4.B16, V3.B16, V3.B16
	VORR V3.B16, V1.B16, V4.B16
	VUMINV V4.B16, V5
	VMOV V5.B[0], R5
	CMP $255, R5
	BNE decode_done
	VSUB V20.B16, V0.B16, V2.B16
	VSUB V21.B16, V0.B16, V4.B16
	VAND V1.B16, V2.B16, V2.B16
	VAND V3.B16, V4.B16, V4.B16
	VORR V4.B16, V2.B16, V0.B16
	VTBL V22.B16, [V0.B16], V1.B16
	VTBL V23.B16, [V0.B16], V2.B16
	VTBL V24.B16, [V0.B16], V3.B16
	VUSHL V25.B16, V1.B16, V1.B16
	VUSHL V26.B16, V2.B16, V2.B16
	VUSHL V27.B16, V3.B16, V3.B16
	VORR V2.B16, V1.B16, V1.B16
	VORR V3.B16, V1.B16, V1.B16
	VMOV V1.D[0], R5
	MOVD R5, (R0)
	VMOV V1.H[4], R5
	MOVH R5, 8(R0)
	ADD $10, R0
	ADD $16, R1
	SUB $16, R2
	B decode_loop

decode_done:
	SUB R2, R3, R3
	MOVD R3, ret+56(FP)
	RET

// Byte permutations followed by signed per-byte shift counts.
DATA ·encodeTables+0(SB)/8, $0x0403030201010000
DATA ·encodeTables+8(SB)/8, $0x0908080706060505
DATA ·encodeTables+16(SB)/8, $0xff04ff0302ff01ff
DATA ·encodeTables+24(SB)/8, $0xff09ff0807ff06ff
DATA ·encodeTables+32(SB)/8, $0x0003fe0104ff02fd
DATA ·encodeTables+40(SB)/8, $0x0003fe0104ff02fd
DATA ·encodeTables+48(SB)/8, $0x00fb00f9fc00fa00
DATA ·encodeTables+56(SB)/8, $0x00fb00f9fc00fa00
GLOBL ·encodeTables(SB), RODATA|NOPTR, $64

// Byte permutations followed by signed per-byte shift counts.
DATA ·decodeTables+0(SB)/8, $0x0b09080604030100
DATA ·decodeTables+8(SB)/8, $0xffffffffffff0e0c
DATA ·decodeTables+16(SB)/8, $0x0c0a090705040201
DATA ·decodeTables+24(SB)/8, $0xffffffffffff0f0d
DATA ·decodeTables+32(SB)/8, $0xff0bffff06ff03ff
DATA ·decodeTables+40(SB)/8, $0xffffffffffffff0e
DATA ·decodeTables+48(SB)/8, $0x0406030507040603
DATA ·decodeTables+56(SB)/8, $0x0000000000000507
DATA ·decodeTables+64(SB)/8, $0xff01fe0002ff01fe
DATA ·decodeTables+72(SB)/8, $0x0000000000000002
DATA ·decodeTables+80(SB)/8, $0x00fc0000fd00fc00
DATA ·decodeTables+88(SB)/8, $0x00000000000000fd
GLOBL ·decodeTables(SB), RODATA|NOPTR, $96
