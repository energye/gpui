// S1 arm64 row kernel (NEON, 8 pixels per iteration).
// Matches convertBandScalar bit for bit:
// R=(yMul*(Y-yOff)+rCr*e+128)>>8 etc, then clip8 via unsigned saturation.
// Peer (ffmpeg): libswscale/aarch64/yuv2rgb_neon.S (declare_rgb_funcs
// yuv420p->rgba entry, :1034) with table setup in
// libswscale/aarch64/swscale_unscaled.c:25 ff_yuv2rgb_init_aarch64;
// ours passes the taps straight in (tableFor) so all matrix/range
// variants share one kernel.
// Go asm note: the assembler has no MUL/SXTL/SSHR/UQXTN mnemonics, so
// those lanes are WORD instruction encodings, each verified byte-for-byte
// against clang-assembled references (clang -c + objdump -d on the arm64
// container; see the S1 arm64 notes). Everything else uses normal
// mnemonics. Signedness is load-bearing: Y-yOff, d, e all go negative
// (dark pixels, cold chroma), so widening is SXTL (signed, not VUXTL),
// the >>8 is SSHR (arithmetic, floors like Go >>, not logical VUSHR),
// and only the final narrow saturates to unsigned: SQXTUN (signed input
// -> unsigned range: negatives go to 0, big positives to max = clip8).
// UQXTN is wrong here: it reads inputs as unsigned, so negatives saturate
// to max (dark pixels came out white); the scalar留守 caught it.
// An earlier revision used VUXTL+VUSHR and failed exactly on negative
// lanes (dark Y / Cb,Cb<128); the scalar留守 caught it bit-exact.
// Layout (8 px = 4 chroma pairs, split low/high halves of 4 px):
//   d0 -> V5.H4 (d0d0d1d1) -> SXTL V9.S4; d1 -> V6.H4 -> SXTL V9.S4;
//   e0 -> V7.H4 -> SXTL V19.S4; e1 -> V8.H4 -> SXTL V19.S4.
//   Y0/Y1 -> V20/V21.S4 via SXTL; taps broadcast as S32 (V10-V14).
//   Low half computes R0/G0/B0 into V22/V24/V26.S4, high half into
//   V23/V25/V27; joined with SQXTUN2, then SQXTUN.8B narrows 16->8.
//   Stores: SQXTUN.8B leaves bytes in the LOW 8 (high half stale), so each
//   pixel pair is VEXT-shifted down and ZIP1-stored (VZIP.B8 also touches
//   only the low half; ZIP2 would read stale high bytes).
// n must be a positive multiple of 8; the Go wrapper handles the tail.

#include "textflag.h"

// func neonRowBulk(dst, y, cb, cr unsafe.Pointer, n int, yMul, yOff, rCr, gCb, gCr, bCb int)
TEXT ·neonRowBulk(SB), NOSPLIT, $0-88
	MOVD dst+0(FP), R0
	MOVD y+8(FP), R1
	MOVD cb+16(FP), R2
	MOVD cr+24(FP), R3
	MOVD n+32(FP), R4
	MOVW yMul+40(FP), R5
	MOVW yOff+48(FP), R6
	MOVW rCr+56(FP), R7
	MOVW gCb+64(FP), R8
	MOVW gCr+72(FP), R9
	MOVW bCb+80(FP), R10

	// Broadcast taps as S32 lanes (products stay 32-bit: |tap*d| fits).
	VDUP R5, V10.S4
	VDUP R7, V11.S4
	VDUP R8, V12.S4
	VDUP R9, V13.S4
	VDUP R10, V14.S4
	MOVD $128, R11
	VDUP R11, V15.S4
	VDUP R11, V16.H4
	VDUP R6, V17.H4
	// Alpha lane = 255.
	MOVD $255, R11
	VDUP R11, V18.B8

loop:
	// Y low half: Y[0..3] -> int32x4, yMul applied -> V20.
	VLD1 (R1), [V0.B8]
	VUXTL V0.B8, V5.H8
	VSUB V17.H8, V5.H8, V0.H8
	// SXTL.4S V20,V0 (signed: Y-yOff < 0 for dark pixels)
	WORD $0x0F10A414
	// MUL.4S V20,V20,V10
	WORD $0x4EAA9E94

	// Chroma low half: Cb[0..1]/Cr[0..1] -> d0d0d1d1 -> V5, eeee -> V7.
	// d pair -> V5.H4 = d0d0d1d1 (packed in R11: d0|d0<<16|d1<<32|d1<<48,
	// minus 128 folded by building (d-128) bytes: values stay 0..255 so
	// pack raw bytes then VSUB V16 once, no per-lane VMOV chain).
	// d0 in R11 (byte), d1 in R12 (byte); pack d0d0d1d1 H-lanes:
	// R11 = d0 | d0<<16 | d1<<32 | d1<<48 via shifts+adds.
	MOVBU (R2), R11
	MOVBU 1(R2), R12
	LSL $16, R11, R13
	ADD R13, R11, R11
	LSL $16, R12, R13
	ADD R13, R12, R13
	LSL $32, R13, R13
	ADD R13, R11, R11
	VMOV R11, V5.D[0]
	MOVBU (R3), R11
	MOVBU 1(R3), R12
	LSL $16, R11, R13
	ADD R13, R11, R11
	LSL $16, R12, R13
	ADD R13, R12, R13
	LSL $32, R13, R13
	ADD R13, R11, R11
	VMOV R11, V7.D[0]
	// V5/V7 already hold H lanes d0d0d1d1 / e0e0e1e1 (raw bytes);
	// subtract 128 in-lane, no widen needed.
	VSUB V16.H4, V5.H4, V5.H4
	VSUB V16.H4, V7.H4, V7.H4
	// SXTL.4S V9,V5 (D0) / V19,V7 (E0): d,e are signed
	WORD $0x0F10A4A9
	WORD $0x0F10A4F3

	// R0: re = rCr*e; R0 = Y0+re+128 -> V22.
	// MUL.4S V22,V19,V11
	WORD $0x4EAB9E76
	VADD V20.S4, V22.S4, V22.S4
	VADD V15.S4, V22.S4, V22.S4
	// SSHR.4S V22,#8 (arithmetic: negatives floor like Go >>)
	WORD $0x4F3806D6
	// G0: G0 = Y0-gd-ge+128 -> V24.
	// MUL.4S V24,V9,V12 (gd) ; MUL.4S V4,V19,V13 (ge to scratch: V28
	// holds R-low for the join below and must survive to it)
	WORD $0x4EAC9D38
	WORD $0x4EAD9E64
	VSUB V24.S4, V20.S4, V24.S4
	VSUB V4.S4, V24.S4, V24.S4
	VADD V15.S4, V24.S4, V24.S4
	// SSHR.4S V24,#8
	WORD $0x4F380718
	// B0: bd = bCb*d; B0 = Y0+bd+128 -> V26.
	// MUL.4S V3,V9,V14 (bd to scratch, same reason)
	WORD $0x4EAE9D23
	VADD V20.S4, V3.S4, V26.S4
	VADD V15.S4, V26.S4, V26.S4
	// SSHR.4S V26,#8
	WORD $0x4F38075A
	// Narrow low halves 32->16 into zip scratch regs directly:
	// SQXTUN.4H V28,V22 (R) / V29,V24 (G) / V30,V26 (B).
	WORD $0x2E612ADC
	WORD $0x2E612B1D
	WORD $0x2E612B5E

	// Y high half: Y[4..7] -> V21.
	MOVD 4(R1), R11
	VMOV R11, V0.D[0]
	VUXTL V0.B8, V5.H8
	VSUB V17.H8, V5.H8, V0.H8
	// SXTL.4S V21,V0 (signed)
	WORD $0x0F10A415
	// MUL.4S V21,V21,V10
	WORD $0x4EAA9EB5

	// Chroma high half: Cb[2..3]/Cr[2..3] -> V6/V8 doubled.
	MOVBU 2(R2), R11
	VMOV R11, V1.B[0]
	MOVBU 3(R2), R11
	VMOV R11, V1.B[1]
	MOVBU 2(R3), R11
	VMOV R11, V2.B[0]
	MOVBU 3(R3), R11
	VMOV R11, V2.B[1]
	VUXTL V1.B8, V3.H8
	VUXTL V2.B8, V4.H8
	VSUB V16.H8, V3.H8, V1.H8
	VSUB V16.H8, V4.H8, V2.H8
	// d2,d3 live in lanes 0,1 of V1; e2,e3 in lanes 0,1 of V2.
	VMOV V1.H[0], V6.H[0]
	VMOV V1.H[0], V6.H[1]
	VMOV V1.H[1], V6.H[2]
	VMOV V1.H[1], V6.H[3]
	VMOV V2.H[0], V8.H[0]
	VMOV V2.H[0], V8.H[1]
	VMOV V2.H[1], V8.H[2]
	VMOV V2.H[1], V8.H[3]
	// SXTL.4S V9,V6 (D1) / V19,V8 (E1): signed
	WORD $0x0F10A4C9
	WORD $0x0F10A513

	// R1/G1/B1 -> V23/V25/V27, joined onto low halves via UQXTN2.
	// MUL.4S V23,V19,V11 (re)
	WORD $0x4EAB9E77
	VADD V21.S4, V23.S4, V23.S4
	VADD V15.S4, V23.S4, V23.S4
	// SSHR.4S V23,#8
	WORD $0x4F3806F7
	// MUL.4S V25,V9,V12 (gd) ; MUL.4S V4,V19,V13 (ge to scratch)
	WORD $0x4EAC9D39
	WORD $0x4EAD9E64
	VSUB V25.S4, V21.S4, V25.S4
	VSUB V4.S4, V25.S4, V25.S4
	VADD V15.S4, V25.S4, V25.S4
	// SSHR.4S V25,#8
	WORD $0x4F380739
	// MUL.4S V3,V9,V14 (bd to scratch)
	WORD $0x4EAE9D23
	VADD V21.S4, V3.S4, V27.S4
	VADD V15.S4, V27.S4, V27.S4
	// SSHR.4S V27,#8
	WORD $0x4F38077B
	// Join: SQXTUN2.8H V28,V23 (R) / V29,V25 (G) / V30,V27 (B).
	WORD $0x6E612AFC
	WORD $0x6E612B3D
	WORD $0x6E612B7E
	// Narrow 16->8: SQXTUN.8B V28,V28 / V29,V29 / V30,V30 (size=00).
	WORD $0x2E212B9C
	WORD $0x2E212BBD
	WORD $0x2E212BDE

	// Interleave R/G/B/A bytes -> 8x RGBA pixels, four B8 stores.
	// The 8B narrows above leave r0..r7 in the LOW 8 bytes (high half is
	// stale), so ZIP2.B16 would read garbage: shift px4..7 down first.
	// Proven qemu semantics: VZIPk X,Y,D starts with the SECOND operand
	// (VZIP1 X,Y = [Y0,X0..]); .B8/.H4 forms touch only the low 64 bits,
	// which is exactly what the B8 stores below read.
	// Rebuild alpha here (not at setup): R11 is chroma scratch by now.
	MOVD $255, R13
	VDUP R13, V18.B8
	// Shift pixel bytes 4..7 down (V22/V24/V26 are dead: their S32 accs
	// were consumed by the narrows above).
	VEXT $4, V28.B16, V28.B16, V22.B16
	VEXT $4, V29.B16, V29.B16, V24.B16
	VEXT $4, V30.B16, V30.B16, V26.B16
	// px0,px1.
	VZIP1 V29.B8, V28.B8, V5.B8
	VZIP1 V18.B8, V30.B8, V6.B8
	VZIP1 V6.H4, V5.H4, V5.H4
	VST1 [V5.B8], (R0)
	// px2,px3 (shift bytes 2..7 down; ZIP2.B8 would take bytes 4..7).
	VEXT $2, V28.B16, V28.B16, V22.B16
	VEXT $2, V29.B16, V29.B16, V24.B16
	VEXT $2, V30.B16, V30.B16, V26.B16
	VZIP1 V24.B8, V22.B8, V5.B8
	VZIP1 V18.B8, V26.B8, V6.B8
	VZIP1 V6.H4, V5.H4, V5.H4
	ADD $8, R0
	VST1 [V5.B8], (R0)
	// px4,px5.
	VEXT $4, V28.B16, V28.B16, V22.B16
	VEXT $4, V29.B16, V29.B16, V24.B16
	VEXT $4, V30.B16, V30.B16, V26.B16
	VZIP1 V24.B8, V22.B8, V5.B8
	VZIP1 V18.B8, V26.B8, V6.B8
	VZIP1 V6.H4, V5.H4, V5.H4
	ADD $8, R0
	VST1 [V5.B8], (R0)
	// px6,px7.
	VEXT $6, V28.B16, V28.B16, V22.B16
	VEXT $6, V29.B16, V29.B16, V24.B16
	VEXT $6, V30.B16, V30.B16, V26.B16
	VZIP1 V24.B8, V22.B8, V5.B8
	VZIP1 V18.B8, V26.B8, V6.B8
	VZIP1 V6.H4, V5.H4, V5.H4
	ADD $8, R0
	VST1 [V5.B8], (R0)
	SUB $24, R0

	ADD $8, R1
	ADD $32, R0
	ADD $4, R2
	ADD $4, R3
	SUB $8, R4
	CBNZ R4, loop

	RET
