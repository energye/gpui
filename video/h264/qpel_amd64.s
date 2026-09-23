// S1 amd64 qpel row kernel (SSSE3, 8 pixels per iteration).
// Unrounded 6-tap horizontal sums with taps [1 -5 20 20 -5 1] (same sum
// as halfH pre-round, int16-exact: range -2550..10710). Each tap pair
// runs as one PMADDUBSW (unsigned pixels, signed taps); even/odd outputs
// merge with PUNPCKLWL (Go asm spelling of PUNPCKLWD: unpack low words,
// see convert_amd64.s NOTE). Requires SSSE3 (PMADDUBSW, implied by the
// baseline, see convert_fast.go). Rounding/clip/avg stay in Go
// (qpel_amd64.go), so this kernel cannot drift the output bits.
//
// S1-Q NOTE (2026-09-22, measured): fusing the center vertical 6-tap
// into a row kernel is NOT bit-safe in 16 lanes — horizontal sums
// (±10710/row) sum six-high to ~±200k, PMULLW wraps mod 2^16 (true
// 57600 observed as 8). The open route is a widened 32-bit vertical
// combine (PMOVSXWD+PMADDWD); until then the center cascade stays in
// Go (centerRows8) and this file holds the two exact row kernels.

#include "textflag.h"

// func qpelHorSums(sums, src unsafe.Pointer, n int)
// sums: *int16, n outputs (n%8==0, n>0); reads src[0..n+4] bytes.
TEXT ·qpelHorSums(SB), NOSPLIT, $0-24
	MOVQ sums+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	CMPQ CX, $0
	JLE  done

loop:
	MOVOU (SI), X0
	MOVOU 1(SI), X1
	MOVAPS X0, X2
	PMADDUBSW ·qpelC1(SB), X2
	MOVAPS X1, X3
	PMADDUBSW ·qpelC1(SB), X3
	PUNPCKLWL X3, X2

	MOVOU 2(SI), X0
	MOVOU 3(SI), X1
	MOVAPS X0, X4
	PMADDUBSW ·qpelC2(SB), X4
	MOVAPS X1, X5
	PMADDUBSW ·qpelC2(SB), X5
	PUNPCKLWL X5, X4

	MOVOU 4(SI), X0
	MOVOU 5(SI), X1
	MOVAPS X0, X6
	PMADDUBSW ·qpelC3(SB), X6
	MOVAPS X1, X7
	PMADDUBSW ·qpelC3(SB), X7
	PUNPCKLWL X7, X6

	PADDW X4, X2
	PADDW X6, X2
	MOVOU X2, (DI)

	ADDQ $8, SI
	ADDQ $16, DI
	SUBQ $8, CX
	JGT  loop

done:
	RET

GLOBL ·qpelC1(SB), RODATA|NOPTR, $16
DATA ·qpelC1+0(SB)/8, $0xFB01FB01FB01FB01
DATA ·qpelC1+8(SB)/8, $0xFB01FB01FB01FB01
GLOBL ·qpelC2(SB), RODATA|NOPTR, $16
DATA ·qpelC2+0(SB)/8, $0x1414141414141414
DATA ·qpelC2+8(SB)/8, $0x1414141414141414
GLOBL ·qpelC3(SB), RODATA|NOPTR, $16
DATA ·qpelC3+0(SB)/8, $0x01FB01FB01FB01FB
DATA ·qpelC3+8(SB)/8, $0x01FB01FB01FB01FB

// S1-A vertical sums kernel (SSE2, 8 outputs per call).
// Unrounded 6-tap vertical sums with taps [1 -5 20 20 -5 1] (same sum
// as halfV pre-round, int16-exact: range -2550..10710). Unlike the
// horizontal kernel (sliding window inside one row), each tap comes
// from its own row pointer (rows ay-2..ay+3, 8 contiguous columns
// each), so the math is element-wise: widen bytes, pair, scale, add.
// Rounding/clip/avg stay in Go (roundHalf, same as horSumsRow), so
// this kernel cannot drift the output bits. Requires only SSE2
// (PUNPCKLBW/PMULLW/PADDW/PSUBW, baseline, same set as chromaRow8).

// func qpelVerSums(sums, r0, r1, r2, r3, r4, r5 unsafe.Pointer)
// sums: *int16, 8 outputs; rN: 8 bytes each (one row's 8 columns).
TEXT ·qpelVerSums(SB), NOSPLIT, $0-56
	MOVQ sums+0(FP), DI
	MOVQ r0+8(FP), SI
	MOVQ r1+16(FP), DX
	MOVQ r2+24(FP), CX
	MOVQ r3+32(FP), R8
	MOVQ r4+40(FP), R9
	MOVQ r5+48(FP), R10

	PXOR X15, X15

	MOVQ (SI), X0
	PUNPCKLBW X15, X0
	MOVQ (DX), X1
	PUNPCKLBW X15, X1
	MOVQ (CX), X2
	PUNPCKLBW X15, X2
	MOVQ (R8), X3
	PUNPCKLBW X15, X3
	MOVQ (R9), X4
	PUNPCKLBW X15, X4
	MOVQ (R10), X5
	PUNPCKLBW X15, X5

	// Pairs: a=r0+r5 (X0), b=r1+r4 (X1), c=r2+r3 (X2).
	PADDW X5, X0
	PADDW X4, X1
	PADDW X3, X2

	// sum = a - 5b + 20c.
	PMULLW ·qpelK20(SB), X2
	PMULLW ·qpelK5(SB), X1
	PADDW X2, X0
	PSUBW X1, X0
	MOVOU X0, (DI)
	RET

GLOBL ·qpelK20(SB), RODATA|NOPTR, $16
DATA ·qpelK20+0(SB)/8, $0x0014001400140014
DATA ·qpelK20+8(SB)/8, $0x0014001400140014
GLOBL ·qpelK5(SB), RODATA|NOPTR, $16
DATA ·qpelK5+0(SB)/8, $0x0005000500050005
DATA ·qpelK5+8(SB)/8, $0x0005000500050005

// S1-R round-half kernel (SSE2, 8 outputs per call).
// Fused (s+16)>>5 + clip to u8, bit-identical to roundHalf (Go) while
// s+16 stays in int16 (s <= 32751): PADDW wraps mod 2^16 where Go int
// math does not (measured: s=32752 gives kernel 0 vs scalar 255), so
// the caller must guarantee the live band. All callers do: hor/ver
// 6-tap sums of bytes stay within -2550..10710 (see qpelHorSums
// note), +16 never wraps. PSRAW floors exactly like Go >>;
// PACKUSWB saturates exactly like clipU8. Requires SSE2 only
// (baseline, same set as verSums).

// func qpelRoundHalf8(dst, sums unsafe.Pointer)
// dst: 8 output bytes; sums: 8 int16 unrounded sums.
TEXT ·qpelRoundHalf8(SB), NOSPLIT, $0-16
	MOVQ dst+0(FP), DI
	MOVQ sums+8(FP), SI
	MOVOU (SI), X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)
	RET

GLOBL ·qpelK16(SB), RODATA|NOPTR, $16
DATA ·qpelK16+0(SB)/8, $0x0010001000100010
DATA ·qpelK16+8(SB)/8, $0x0010001000100010

// S1-F fused block kernels (SSE2/SSSE3, one call per block).
// horSums+round and verSums+round ran as two kernel calls per row
// (plus Go wrapper loops with per-row slicing): the wrappers cost
// ~360ms flat per 90f profile vs ~300ms for the SIMD kernels. Fusing
// sums->round->store into one call per block kills the transitions,
// the Go row loops, and the int16 temp traffic — same taps, same
// (s+16)>>5, same clip, bit-identical to the split row paths
// (TestS1FusedBlockMatchesSplit pins every fusion). Peer (ffmpeg,
// read-only, ideas only): one mc_op call per block with an internal
// row loop (x86/h264_qpel_8bit.asm), our row loops below. Live widths
// only (8/16, Go-gated); odd widths keep the split path. History:
// briefly withdrawn 2026-09-22 on noisy-machine A/B, re-applied the
// same session when profile comparison showed qpelGeneralInto cum
// 710ms->530ms with the wrappers nearly gone from the profile.
// Requires SSSE3 (hor taps) / SSE2 (ver, round), same sets as the row
// kernels they fuse.

// func horRoundBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)
// dst: byte rows (stride dstStride); src: row bytes at ax-2.
TEXT ·horRoundBlock(SB), NOSPLIT, $0-48
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ src+16(FP), SI
	MOVQ srcStride+24(FP), R10
	MOVQ w+32(FP), R8
	MOVQ h+40(FP), R9

hrow:
	MOVQ R8, CX
hgrp:
	MOVOU (SI), X0
	MOVOU 1(SI), X1
	MOVAPS X0, X2
	PMADDUBSW ·qpelC1(SB), X2
	MOVAPS X1, X3
	PMADDUBSW ·qpelC1(SB), X3
	PUNPCKLWL X3, X2

	MOVOU 2(SI), X0
	MOVOU 3(SI), X1
	MOVAPS X0, X4
	PMADDUBSW ·qpelC2(SB), X4
	MOVAPS X1, X5
	PMADDUBSW ·qpelC2(SB), X5
	PUNPCKLWL X5, X4

	MOVOU 4(SI), X0
	MOVOU 5(SI), X1
	MOVAPS X0, X6
	PMADDUBSW ·qpelC3(SB), X6
	MOVAPS X1, X7
	PMADDUBSW ·qpelC3(SB), X7
	PUNPCKLWL X7, X6

	PADDW X4, X2
	PADDW X6, X2
	PADDW ·qpelK16(SB), X2
	PSRAW $5, X2
	PACKUSWB X2, X2
	// 4-wide store (same rule as ablk4/avgRow4: a direct MOVQ X,(DI)
	// writes 8 bytes and paints the 4 neighbours right of a 4-wide
	// row; rows 1+ then read back zero through the hh window —
	// dbg11g 2026-09-23). MOVD X->AX (not MOVQ X,AX: MOVQ moves the
	// full 8 bytes and faults — objdump showed MOVQ X2,AX assembling
	// to a GPR->XMM move that left garbage, dbg11i 2026-09-23).
	MOVD X2, AX
	CMPQ R8, $4
	JEQ hstore4
	MOVQ AX, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	SUBQ $8, CX
	JGT hgrp

	SUBQ R8, SI
	ADDQ R10, SI
	SUBQ R8, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT hrow
	RET

hstore4:
	MOVL AX, (DI)

	ADDQ $4, SI
	ADDQ $4, DI
	SUBQ $8, CX
	JGT hgrp

	SUBQ R8, SI
	ADDQ R10, SI
	SUBQ R8, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT hrow
	RET

// func horSumsBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)
// dst: int16 rows (stride dstStride BYTES); src: row bytes at ax-2.
// Unrounded 6-tap horizontal sums for the jt temp (needJT paths):
// same taps as horSumsRow/qpelHorSums, stored raw (no round, no clip),
// bit-identical to bh+5x horSumsRow. One call kills the per-row Go
// loop + per-row kernel transitions for the temp window (temp-users
// are ~25% of taken blocks on 1080p). w in {8,16} (Go-gated); requires
// SSSE3, same as qpelHorSums.
TEXT ·horSumsBlock(SB), NOSPLIT, $0-48
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ src+16(FP), SI
	MOVQ srcStride+24(FP), R10
	MOVQ w+32(FP), R8
	MOVQ h+40(FP), R9

srow:
	MOVQ R8, CX
sgrp:
	MOVOU (SI), X0
	MOVOU 1(SI), X1
	MOVAPS X0, X2
	PMADDUBSW ·qpelC1(SB), X2
	MOVAPS X1, X3
	PMADDUBSW ·qpelC1(SB), X3
	PUNPCKLWL X3, X2

	MOVOU 2(SI), X0
	MOVOU 3(SI), X1
	MOVAPS X0, X4
	PMADDUBSW ·qpelC2(SB), X4
	MOVAPS X1, X5
	PMADDUBSW ·qpelC2(SB), X5
	PUNPCKLWL X5, X4

	MOVOU 4(SI), X0
	MOVOU 5(SI), X1
	MOVAPS X0, X6
	PMADDUBSW ·qpelC3(SB), X6
	MOVAPS X1, X7
	PMADDUBSW ·qpelC3(SB), X7
	PUNPCKLWL X7, X6

	PADDW X4, X2
	PADDW X6, X2
	// 4-wide int16 store (exact 8B = 4 sums via MOVQ; the old MOVOU
	// wrote 16 bytes and painted the next row's first 4 sums — same
	// 4-wide store rule, int16 side, 2026-09-23).
	CMPQ R8, $4
	JEQ sstore4
	MOVOU X2, (DI)

	ADDQ $8, SI
	ADDQ $16, DI
	SUBQ $8, CX
	JGT sgrp

	SUBQ R8, SI
	ADDQ R10, SI
	MOVQ R8, AX
	ADDQ AX, AX
	SUBQ AX, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT srow
	RET

sstore4:
	MOVQ X2, (DI)

	ADDQ $8, SI
	ADDQ $16, DI
	SUBQ $8, CX
	JGT sgrp

	SUBQ R8, SI
	ADDQ R10, SI
	MOVQ R8, AX
	ADDQ AX, AX
	SUBQ AX, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT srow
	RET

// func verRoundBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)
// dst: byte rows (stride dstStride); src: row ay-2 bytes at ax.
TEXT ·verRoundBlock(SB), NOSPLIT, $0-48
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), R13
	MOVQ src+16(FP), AX
	MOVQ srcStride+24(FP), R12
	MOVQ w+32(FP), R8
	MOVQ h+40(FP), R11
	// Six row pointers (r0..r5 = src+0..5 stride).
	MOVQ AX, SI
	MOVQ AX, DX
	ADDQ R12, DX
	MOVQ DX, CX
	ADDQ R12, CX
	MOVQ CX, R8
	ADDQ R12, R8
	MOVQ R8, R9
	ADDQ R12, R9
	MOVQ R9, R10
	ADDQ R12, R10
	CMPQ w+32(FP), $8
	JEQ v8rows
	CMPQ w+32(FP), $4
	JEQ v4rows

v16row:
	PXOR X15, X15
	MOVQ (SI), X0
	PUNPCKLBW X15, X0
	MOVQ (DX), X1
	PUNPCKLBW X15, X1
	MOVQ (CX), X2
	PUNPCKLBW X15, X2
	MOVQ (R8), X3
	PUNPCKLBW X15, X3
	MOVQ (R9), X4
	PUNPCKLBW X15, X4
	MOVQ (R10), X5
	PUNPCKLBW X15, X5
	PADDW X5, X0
	PADDW X4, X1
	PADDW X3, X2
	PMULLW ·qpelK20(SB), X2
	PMULLW ·qpelK5(SB), X1
	PADDW X2, X0
	PSUBW X1, X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)
	PXOR X15, X15
	MOVQ 8(SI), X0
	PUNPCKLBW X15, X0
	MOVQ 8(DX), X1
	PUNPCKLBW X15, X1
	MOVQ 8(CX), X2
	PUNPCKLBW X15, X2
	MOVQ 8(R8), X3
	PUNPCKLBW X15, X3
	MOVQ 8(R9), X4
	PUNPCKLBW X15, X4
	MOVQ 8(R10), X5
	PUNPCKLBW X15, X5
	PADDW X5, X0
	PADDW X4, X1
	PADDW X3, X2
	PMULLW ·qpelK20(SB), X2
	PMULLW ·qpelK5(SB), X1
	PADDW X2, X0
	PSUBW X1, X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVQ X0, 8(DI)
	ADDQ R12, SI
	ADDQ R12, DX
	ADDQ R12, CX
	ADDQ R12, R8
	ADDQ R12, R9
	ADDQ R12, R10
	ADDQ R13, DI
	SUBQ $1, R11
	JGT v16row
	RET

v8rows:
v8row:
	PXOR X15, X15
	MOVQ (SI), X0
	PUNPCKLBW X15, X0
	MOVQ (DX), X1
	PUNPCKLBW X15, X1
	MOVQ (CX), X2
	PUNPCKLBW X15, X2
	MOVQ (R8), X3
	PUNPCKLBW X15, X3
	MOVQ (R9), X4
	PUNPCKLBW X15, X4
	MOVQ (R10), X5
	PUNPCKLBW X15, X5
	PADDW X5, X0
	PADDW X4, X1
	PADDW X3, X2
	PMULLW ·qpelK20(SB), X2
	PMULLW ·qpelK5(SB), X1
	PADDW X2, X0
	PSUBW X1, X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)
	ADDQ R12, SI
	ADDQ R12, DX
	ADDQ R12, CX
	ADDQ R12, R8
	ADDQ R12, R9
	ADDQ R12, R10
	ADDQ R13, DI
	SUBQ $1, R11
	JGT v8row
	RET

v4rows:
	// 4-wide vertical (same taps as v8row; exact 4B loads via MOVL,
	// exact 4B store via AX — a direct MOVQ X,(DI) would paint the 4
	// neighbours right of a 4-wide block, same bug as horRoundBlock
	// hstore4 above, 2026-09-23).
v4row:
	PXOR X15, X15
	MOVL (SI), AX
	MOVD AX, X0
	PUNPCKLBW X15, X0
	MOVL (DX), AX
	MOVD AX, X1
	PUNPCKLBW X15, X1
	MOVL (CX), AX
	MOVD AX, X2
	PUNPCKLBW X15, X2
	MOVL (R8), AX
	MOVD AX, X3
	PUNPCKLBW X15, X3
	MOVL (R9), AX
	MOVD AX, X4
	PUNPCKLBW X15, X4
	MOVL (R10), AX
	MOVD AX, X5
	PUNPCKLBW X15, X5
	PADDW X5, X0
	PADDW X4, X1
	PADDW X3, X2
	PMULLW ·qpelK20(SB), X2
	PMULLW ·qpelK5(SB), X1
	PADDW X2, X0
	PSUBW X1, X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)
	ADDQ R12, SI
	ADDQ R12, DX
	ADDQ R12, CX
	ADDQ R12, R8
	ADDQ R12, R9
	ADDQ R12, R10
	ADDQ R13, DI
	SUBQ $1, R11
	JGT v4row
	RET

// func roundHalfBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)
// dst: byte rows (stride dstStride); src: int16 rows (stride bytes).
// Rounds the jt temp into hh (needHH+needJT path), bit-identical to
// roundHalfRow over the same rows.
TEXT ·roundHalfBlock(SB), NOSPLIT, $0-48
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ src+16(FP), SI
	MOVQ srcStride+24(FP), R10
	MOVQ w+32(FP), R8
	MOVQ h+40(FP), R9

rrow:
	MOVQ R8, CX
	CMPQ R8, $4
	JEQ rgrp4
rgrp:
	MOVOU (SI), X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)

	ADDQ $16, SI
	ADDQ $8, DI
	SUBQ $8, CX
	JGT rgrp

	MOVQ R8, AX
	ADDQ AX, AX
	SUBQ AX, SI
	ADDQ R10, SI
	SUBQ R8, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT rrow
	RET

	// 4-wide round (exact 8B int16 load = 4 sums, exact 4B store via
	// AX — same 4-wide store rule, 2026-09-23).
rgrp4:
	MOVQ (SI), X0
	PADDW ·qpelK16(SB), X0
	PSRAW $5, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)

	ADDQ $8, SI
	ADDQ $4, DI
	SUBQ $4, CX
	JGT rgrp4

	MOVQ R8, AX
	ADDQ AX, AX
	SUBQ AX, SI
	ADDQ R10, SI
	SUBQ R8, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT rrow
	RET
// Fused vertical 6-tap over unrounded horizontal sums with the single
// +512>>10 rounding and clip to u8, bit-identical to centerAt (Go):
// s = r0+r5 - 5*(r1+r4) + 20*(r2+r3), out = clipU8((s+512)>>10).
// The old 16-lane attempt (2026-09-22) wrapped in PMULLW (sums reach
// ~±200k: true 57600 observed as 8); this kernel widens to 32-bit
// first so nothing wraps: int16 rows -> sign-extended int32
// (PUNPCKLWD+PSRAW, SSE2), then 32-bit adds/subs/shifts (PADDL/PSUBL/
// PSLLD/PSRAL, all SSE2, lane-local like Go int arithmetic).
// 5*b = 4b+b and 20*c = 16c+4c via shifts+adds (exact, no overflow:
// |s| < 600k << 2^31). PSRAL floors exactly like Go >>; PACKSSLW is
// Go's spelling of PACKSSDW (signed dword->word, a no-op in range)
// and PACKUSWB saturates exactly like clipU8.
// Only the packed 4 bytes store, so the untouched high lanes (caller
// garbage after MOVQ loads) never reach memory. Requires SSE2 only
// (baseline, same set as verSums).

// func centerVec4(dst, r0, r1, r2, r3, r4, r5 unsafe.Pointer)
// dst: 4 output bytes; rN: 4 int16 each (one jt row's 4 columns).
TEXT ·centerVec4(SB), NOSPLIT, $0-56
	MOVQ dst+0(FP), DI
	MOVQ r0+8(FP), SI
	MOVQ r1+16(FP), DX
	MOVQ r2+24(FP), CX
	MOVQ r3+32(FP), R8
	MOVQ r4+40(FP), R9
	MOVQ r5+48(FP), R10
	// Widen rows to int32 (sign-extend): Xk = 4 int32 of row k.
	MOVQ (SI), X0
	MOVAPS X0, X8
	PSRAW $15, X8
	PUNPCKLWL X8, X0
	MOVQ (DX), X1
	MOVAPS X1, X8
	PSRAW $15, X8
	PUNPCKLWL X8, X1
	MOVQ (CX), X2
	MOVAPS X2, X8
	PSRAW $15, X8
	PUNPCKLWL X8, X2
	MOVQ (R8), X3
	MOVAPS X3, X8
	PSRAW $15, X8
	PUNPCKLWL X8, X3
	MOVQ (R9), X4
	MOVAPS X4, X8
	PSRAW $15, X8
	PUNPCKLWL X8, X4
	MOVQ (R10), X5
	MOVAPS X5, X8
	PSRAW $15, X8
	PUNPCKLWL X8, X5
	// Pairs: a=r0+r5 (X0), b=r1+r4 (X1), c=r2+r3 (X2).
	PADDL X5, X0
	PADDL X4, X1
	PADDL X3, X2
	// 5b = 4b+b (X1 dead after: X9 = 5b).
	MOVAPS X1, X9
	PSLLL $2, X9
	PADDL X1, X9
	// 20c = 16c+4c (X2 shared: X10 = 20c, X2 kept? X2 dead after).
	MOVAPS X2, X10
	PSLLL $4, X10
	MOVAPS X2, X11
	PSLLL $2, X11
	PADDL X11, X10
	// s = a - 5b + 20c.
	PSUBL X9, X0
	PADDL X10, X0
	// Single center rounding + clip (pure SIMD, no branches):
	// PSRAL floors like Go >>; PACKSSLW (Go spelling of PACKSSDW:
	// signed dword->word) is a no-op in range (|out|<600<<32767);
	// PACKUSWB saturates exactly like clipU8. Store exactly 4 bytes
	// via AX (MOVD X->AX + MOVL AX->mem, same pattern as deblock):
	// a direct MOVD X0,(DI) assembles to MOVQ (8 bytes, objdump
	// 2026-09-22: 66 0F D6) and overwrites the 4 neighbours right
	// of the block, which broke prefix parity by 3052 bytes.
	PADDL ·centerK512(SB), X0
	PSRAL $10, X0
	PACKSSLW X0, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)
	RET

// S1b-U center-avg fused kernel (SSE2, 4 outputs per call).
// The four half-center pairs (2,1 / 1,2 / 3,2 / 2,3) averaged the
// center cascade with a half-pel plane: centerRow4 into a 4-byte
// scratch, then avgRowInto over the scratch (two calls + a Go temp
// per 4 pixels, ~110ms flat per 90f profile). Fusing center+avg
// into one call kills the second transition and the scratch —
// same taps, same (s+512)>>10 center round, same (a+b+1)>>1 avg,
// bit-identical to centerRow4+avgRowInto (TestS1CenterVec pins the
// center half, qpel_avg_amd64_test.go pins the avg half, so the
// fused path is pinned without a new test). Peer (ffmpeg, read-only,
// ideas only): h264qpel_template H264_QPEL center+avg in one mc_op
// (our single call below). avg plane arrives as 4 bytes in memory
// (a Unsafe.Pointer to the first avg byte); its high lanes load
// zero via MOVD (no caller garbage: the old MOVQ-load shape is
// banned here — PAVGB would average garbage into bytes 4..7 and
// the 4-byte store would keep only the low half, silently wrong
// nowhere but slow nowhere... precisely: keep it exact). Store is
// exactly 4 bytes via AX (same rule as centerVec4, 2026-09-22).
// Requires SSE2 only (same set as centerVec4).

// func centerAvg4(dst, r0, r1, r2, r3, r4, r5, avg unsafe.Pointer)
// dst: 4 output bytes; rN: 4 int16 each (one jt row's 4 columns);
// avg: 4 bytes to average with the center outputs.
TEXT ·centerAvg4(SB), NOSPLIT, $0-64
	MOVQ dst+0(FP), DI
	MOVQ r0+8(FP), SI
	MOVQ r1+16(FP), DX
	MOVQ r2+24(FP), CX
	MOVQ r3+32(FP), R8
	MOVQ r4+40(FP), R9
	MOVQ r5+48(FP), R10
	MOVQ avg+56(FP), R11
	// Widen rows to int32 (sign-extend): Xk = 4 int32 of row k.
	MOVQ (SI), X0
	MOVAPS X0, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X0
	MOVQ (DX), X1
	MOVAPS X1, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X1
	MOVQ (CX), X2
	MOVAPS X2, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X2
	MOVQ (R8), X3
	MOVAPS X3, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X3
	MOVQ (R9), X4
	MOVAPS X4, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X4
	MOVQ (R10), X5
	MOVAPS X5, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X5
	// Pairs: a=r0+r5 (X0), b=r1+r4 (X1), c=r2+r3 (X2).
	PADDL X5, X0
	PADDL X4, X1
	PADDL X3, X2
	// 5b = 4b+b (X1 dead after: X12 = 5b).
	MOVAPS X1, X12
	PSLLL $2, X12
	PADDL X1, X12
	// 20c = 16c+4c (X2 dead after: X9 = 20c... X9/X10 free here:
	// centerVec4 used X9-X11 the same way above).
	MOVAPS X2, X9
	PSLLL $4, X9
	MOVAPS X2, X10
	PSLLL $2, X10
	PADDL X10, X9
	// s = a - 5b + 20c.
	PSUBL X12, X0
	PADDL X9, X0
	PADDL ·centerK512(SB), X0
	PSRAL $10, X0
	PACKSSLW X0, X0
	PACKUSWB X0, X0
	// avg 4 bytes (MOVD zeroes high lanes) then PAVGB.
	MOVL (R11), AX
	MOVD AX, X1
	PAVGB X1, X0
	MOVD X0, AX
	MOVL AX, (DI)
	RET

// S1b-V center-avg block kernel (SSE2, one call per block).
// The four half-center pairs ran centerRowAvg4 per 4 pixels: a call
// plus 6 unsafe.Add index mults per group (~140ms flat per 90f
// profile for wrappers + kernels). One call per block kills the
// per-group transitions and the mults — row pointers derive from one
// jt base via displacements (r_k = base+k*32, the jt row stride is
// fixed 32 bytes by the [21][16]int16 window type), so a single jt
// pointer stays live. Same taps, same (s+512)>>10 center round, same
// (a+b+1)>>1 avg — bit-identical to centerRowAvg4 over the same rows
// (TestS1CenterAvgBlockMatchesRow pins every live width). Peer
// (ffmpeg, read-only, ideas only): h264qpel_template center+avg in
// one mc_op with an internal loop, our row/group loops below.
// w in {4,8,16} (Go-gated via fusedAvgW); avg rows ride avgStride.
// Loads are the same exact shapes as centerAvg4 (8B jt lanes, 4B avg
// via MOVL+MOVD so high lanes stay zero for PAVGB); store is exactly
// 4 bytes via AX (same rule as centerVec4, 2026-09-22). Requires SSE2
// only (same set as centerAvg4).

// func centerAvgBlock(dst unsafe.Pointer, dstStride uintptr, jt unsafe.Pointer, avg unsafe.Pointer, avgStride uintptr, w, h int)
// dst: byte rows (stride dstStride); jt: int16 window base;
// avg: byte rows (stride avgStride); w in {4,8,16}, h rows.
TEXT ·centerAvgBlock(SB), NOSPLIT, $0-56
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ jt+16(FP), SI
	MOVQ avg+24(FP), R11
	MOVQ avgStride+32(FP), R12
	MOVQ w+40(FP), R8
	MOVQ h+48(FP), R9

cbrow:
	MOVQ R8, CX
cbgrp:
	// Widen 6 jt rows x4 int16 -> int32 (displacements off SI).
	MOVQ (SI), X0
	MOVAPS X0, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X0
	MOVQ 32(SI), X1
	MOVAPS X1, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X1
	MOVQ 64(SI), X2
	MOVAPS X2, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X2
	MOVQ 96(SI), X3
	MOVAPS X3, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X3
	MOVQ 128(SI), X4
	MOVAPS X4, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X4
	MOVQ 160(SI), X5
	MOVAPS X5, X12
	PSRAW $15, X12
	PUNPCKLWL X12, X5
	// Pairs: a=r0+r5 (X0), b=r1+r4 (X1), c=r2+r3 (X2).
	PADDL X5, X0
	PADDL X4, X1
	PADDL X3, X2
	// 5b = 4b+b (X1 dead after: X12 = 5b).
	MOVAPS X1, X12
	PSLLL $2, X12
	PADDL X1, X12
	// 20c = 16c+4c (X2 dead after: X9/X10, same as centerAvg4).
	MOVAPS X2, X9
	PSLLL $4, X9
	MOVAPS X2, X10
	PSLLL $2, X10
	PADDL X10, X9
	// s = a - 5b + 20c, round, clip, avg, store.
	PSUBL X12, X0
	PADDL X9, X0
	PADDL ·centerK512(SB), X0
	PSRAL $10, X0
	PACKSSLW X0, X0
	PACKUSWB X0, X0
	MOVL (R11), AX
	MOVD AX, X1
	PAVGB X1, X0
	MOVD X0, AX
	MOVL AX, (DI)

	ADDQ $8, SI
	ADDQ $4, R11
	ADDQ $4, DI
	SUBQ $4, CX
	JGT cbgrp

	// Row end: SI ran w*2 ahead via groups; rewind to this row start
	// then step one jt row (+32). R11/DI ran w ahead; step their
	// strides. (w in {4,8,16}: CX hits exactly 0, no partial group.)
	MOVQ R8, AX
	ADDQ AX, AX
	SUBQ AX, SI
	ADDQ $32, SI
	SUBQ R8, R11
	ADDQ R12, R11
	SUBQ R8, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT cbrow
	RET

GLOBL ·centerK512(SB), RODATA|NOPTR, $16
DATA ·centerK512+0(SB)/4, $512
DATA ·centerK512+4(SB)/4, $512
DATA ·centerK512+8(SB)/4, $512
DATA ·centerK512+12(SB)/4, $512

// S1-Q avg row kernels (SSE2 PAVGB, out-of-place).
// dst[i] = (a[i]+b[i]+1)>>1 — exactly what PAVGB computes in one
// instruction, bit-identical to the avg2 scalar loop. Peer (ffmpeg,
// read-only, ideas only): libavcodec/x86/h264_qpel_8bit.asm avg
// instances (pavgb over the two half-pel inputs). Unaligned-safe
// (MOVOU); the 8-wide store is an exact MOVQ and the 4-wide store
// goes through AX (a direct MOVD X,(DI) assembles to MOVQ, 8 bytes,
// cf. centerVec4 2026-09-22 — it would overwrite the 4 neighbours
// right of a 4-wide block). Requires SSE2 only (PAVGB is SSE1).

// func avgRow16(dst, a, b unsafe.Pointer) — 16 bytes.
TEXT ·avgRow16(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX
	MOVOU (SI), X0
	MOVOU (DX), X1
	PAVGB X1, X0
	MOVOU X0, (DI)
	RET

// func avgRow8(dst, a, b unsafe.Pointer) — 8 bytes, exact.
TEXT ·avgRow8(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX
	MOVQ (SI), X0
	MOVQ (DX), X1
	PAVGB X1, X0
	MOVQ X0, (DI)
	RET

// func avgRow4(dst, a, b unsafe.Pointer) — 4 bytes, exact.
TEXT ·avgRow4(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ a+8(FP), SI
	MOVQ b+16(FP), DX
	MOVL (SI), AX
	MOVD AX, X0
	MOVL (DX), AX
	MOVD AX, X1
	PAVGB X1, X0
	MOVD X0, AX
	MOVL AX, (DI)
	RET

// S1b-T corner fused block kernels (SSE2 PAVGB, one call per block).
// The four corner pairs (1,1 / 3,1 / 1,3 / 3,3) average two half-pel
// planes (hh x hv) that the fused passes already materialized: the
// old path ran avgRowInto per row (a call + Go row loop + slices per
// row, ~120ms flat per 90f profile). Fusing one block into one call
// kills the per-row transitions and the Go row loops — same PAVGB,
// same (a+b+1)>>1, bit-identical to avgRowInto over the same rows
// (qpel_avg_amd64_test.go pins avgRowInto against avg2, so the fused
// path is pinned without a new test). Peer (ffmpeg, read-only, ideas
// only): h264qpel.c:59-101 put/avg_pixels_tab (per-frac function
// pointers, our corner switch below) + x86/h264_qpel_8bit.asm avg
// instances (pavgb over the two half-pel inputs, our row bodies).
// Widths 4/8/16 (Go-gated via fusedW at the callers); hh/hv rows are
// dense (qpelMaxW / qpelMaxW+1 strides); dst strides out. Stride
// operands ride as uintptr. Requires SSE2 only (same set as avgRow).

// func avgBlock(dst unsafe.Pointer, dstStride uintptr, a unsafe.Pointer, aStride uintptr, b unsafe.Pointer, bStride uintptr, w, h int)
// dst: byte rows (stride dstStride); a/b: byte rows (strides
// aStride/bStride); w in {4,8,16}, h rows. w==4 stores exactly 4
// bytes via AX (a direct MOVD X,(DI) assembles to MOVQ, 8 bytes,
// cf. centerVec4 2026-09-22).
TEXT ·avgBlock(SB), NOSPLIT, $0-64
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ a+16(FP), SI
	MOVQ aStride+24(FP), R10
	MOVQ b+32(FP), R11
	MOVQ bStride+40(FP), R12
	MOVQ w+48(FP), R8
	MOVQ h+56(FP), R9
	CMPQ R8, $4
	JEQ ablk4
	CMPQ R8, $16
	JEQ ablk16

ablk8:
	MOVQ (SI), X0
	MOVQ (R11), X1
	PAVGB X1, X0
	MOVQ X0, (DI)
	ADDQ R10, SI
	ADDQ R12, R11
	ADDQ DX, DI
	SUBQ $1, R9
	JGT ablk8
	RET

ablk4:
	// 4-byte lanes via XMM loads (MOVD X,(m) assembles to MOVQ: the
	// avgRow4 shape; MOVL+MOVD left garbage in X0/X1 high lanes and
	// PAVGB averaged garbage into bytes 4..7, breaking byte 4+ on
	// 4-wide blocks — gate caught 4x4 (1,1) byte 4, 2026-09-23).
	MOVQ (SI), X0
	MOVQ (R11), X1
	PAVGB X1, X0
	MOVQ X0, AX
	MOVL AX, (DI)
	ADDQ R10, SI
	ADDQ R12, R11
	ADDQ DX, DI
	SUBQ $1, R9
	JGT ablk4
	RET

ablk16:
	MOVOU (SI), X0
	MOVOU (R11), X1
	PAVGB X1, X0
	MOVOU X0, (DI)
	ADDQ R10, SI
	ADDQ R12, R11
	ADDQ DX, DI
	SUBQ $1, R9
	JGT ablk16
	RET
