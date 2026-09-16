// S1 arm64 qpel row kernel (NEON, 8 unrounded 6-tap sums per iteration).
// Matches horSumsRow's scalar tail bit for bit:
//   sums[i] = p[-2]+p[+3] - 5*(p[-1]+p[+2]) + 20*(p[0]+p[+1])
// (int16-exact: fits s16, same sum as halfH pre-round).
// Per-output form with pair-sums a=S0+S5, b=S1+S4, c=S2+S3:
//   sum = a - 5b + 20c
// (a and b have DIFFERENT coefficients, so no a+b grouping: an early
// draft used 21c-5(a+b) and the ramp probe read exactly 11x on every
// lane, since 21-10=11. Caught by the qemu probe before any gate ran.)
// Built add-by-add with no doubling in place and no value reused after
// death, so a miscount shows as one wrong line, not a global shift:
//   c2=c+c, c4=c2+c2, c8=c4+c4, c16=c8+c8,
//   V5 = c16+c4 (=20c); V5 += a; b2=b+b, b4=b2+b2, 5b=b4+b;
//   sum = V5-5b.
// Every vector op is a plain named mnemonic VLD1/VUXTL/VADD/VSUB/VST1
// (same set as video/color/convert_arm64.s); no WORD encodings, no
// cross-lane moves. Lanes never overflow (all non-negative until the
// final VSUB, max 20c+a = 10710 < 65535; VSUB wraps mod 2^16 to exactly
// the scalar int16 sum, range -2550..10710). Rounding/clip/avg stay in
// Go, so this kernel cannot drift the output bits.
// Structure: ONE pass computes all 8 outputs. Each B8 load holds 8
// pixels; VUXTL widens those 8 bytes to 8 halfwords, so lane j holds
// src[j] in V0, src[j+1] in V1, ... src[j+5] in V5 — exactly output j's
// six taps for j=0..7. The old two-group draft assumed one B8 load fed
// lanes 0..3 only and recomputed 4..7 with a second load group; that
// premise is wrong (the widen consumes all 8 bytes into 8 lanes) and
// the redundant group B is where the high-half failures came from.
// One VST1 H8 stores all 8 sums (16 bytes). n must be a positive
// multiple of 8; the Go wrapper feeds full-8 windows only.
// Draft lessons 2026-09-16 (all caught by qemu probes before any gate
// ran): (1) VSUB X,Y,Z is Z=Y-X (minuend is the MIDDLE operand:
// VSUB V1,V5,V5 means V5=V5-V1 = (20c+a)-5b); (2) VST1 width is the
// store width: H4 stores 4 sums (8 bytes, one half), H8 stores 8 sums
// (16 bytes, the whole iteration) — S4 would store 4 words (16 bytes
// with a different lane view) and overflowed the sums window, which
// showed as wild-slice panics in the Go wrapper; (3) Go asm has no
// VLD1/VST1 offset form (step the pointer) and no H->S cross-size lane
// move (this kernel never widens past H8); (4) never group a+b: their
// coefficients differ (+1 vs -5).

#include "textflag.h"

// func neonQpelHorSums(sums, src unsafe.Pointer, n int)
// sums: *int16, n outputs (n%8==0, n>0); reads src[0..n+4] bytes.
TEXT ·neonQpelHorSums(SB), NOSPLIT, $0-24
	MOVD sums+0(FP), R0
	MOVD src+8(FP), R1
	MOVD n+16(FP), R2

top:
	VLD1 (R1), [V0.B8]
	ADD $1, R1
	VLD1 (R1), [V1.B8]
	ADD $1, R1
	VLD1 (R1), [V2.B8]
	ADD $1, R1
	VLD1 (R1), [V3.B8]
	ADD $1, R1
	VLD1 (R1), [V4.B8]
	ADD $1, R1
	VLD1 (R1), [V5.B8]
	// Unwind the five +1 steps (R1 = base+5 -> base).
	SUB $5, R1
	VUXTL V0.B8, V0.H8
	VUXTL V1.B8, V1.H8
	VUXTL V2.B8, V2.H8
	VUXTL V3.B8, V3.H8
	VUXTL V4.B8, V4.H8
	VUXTL V5.B8, V5.H8
	// Pairs, all 8 lanes live: a=S0+S5 (V6), b=S1+S4 (V7),
	// c=S2+S3 (V2).
	VADD V0.H8, V5.H8, V6.H8
	VADD V1.H8, V4.H8, V7.H8
	VADD V2.H8, V3.H8, V2.H8
	// c2 (V4), c4 (V0), c8 (V1), c16 (V3). V0/V1/V3/V4 are dead
	// taps/pairs by now (a=V6, b=V7, c=V2 stay alive).
	VADD V2.H8, V2.H8, V4.H8
	VADD V4.H8, V4.H8, V0.H8
	VADD V0.H8, V0.H8, V1.H8
	VADD V1.H8, V1.H8, V3.H8
	// V5 = c16+c4 (=20c); V5 += a.
	VADD V3.H8, V0.H8, V5.H8
	VADD V5.H8, V6.H8, V5.H8
	// 5b: b2 (V0, c4 dead), b4 (V1, c8 dead), 5b=b4+b (V1).
	VADD V7.H8, V7.H8, V0.H8
	VADD V0.H8, V0.H8, V1.H8
	VADD V1.H8, V7.H8, V1.H8
	// sum = (20c+a)-5b. Order load-bearing: V5=V5-V1 (minuend is
	// the middle operand).
	VSUB V1.H8, V5.H8, V5.H8
	// All 8 lanes hold outputs 0..7; one H8 store writes 8 int16.
	VST1 [V5.H8], (R0)

	// Next 8: consumed src[0..12] (13 bytes for 8 outputs + taps),
	// so cursor += 8; sums += 8 int16 = 16 bytes.
	ADD $8, R1
	ADD $16, R0
	SUB $8, R2
	CBNZ R2, top
	RET
