// S1b-A1 amd64 chroma row kernel (SSE2, 8 pixels per call).
// Matches chromaInteriorScalar bit for bit:
//   v = (A*a+B*b+C*c+D*d+32)>>6, A=(8-fx)*(8-fy) etc.
// Each weight arrives packed twice (W|W<<16) so MOVD+PSHUFD broadcasts
// to 8 identical 16-bit lanes (same trick as video/color/convert_amd64.s).
// Pixel bytes widen via PUNPCKLBW (zero in X15); products stay 16-bit
// (max 64*255+32 = 16352 < 32767); shift is arithmetic (non-negative,
// same as logical); PACKUSWB with itself packs 8 words to 8 bytes.
// Requires only SSE2 (PMULLW/PADDW/PSRAW/PACKUSWB, baseline).
// Rounding/clip stay exact: v in 0..255, no saturation needed beyond
// the pack.

#include "textflag.h"

// func chromaRow8(dst, src0, src1 unsafe.Pointer, a32, b32, c32, d32 int)
TEXT ·chromaRow8(SB), NOSPLIT, $0-56
	MOVQ dst+0(FP), DI
	MOVQ src0+8(FP), SI
	MOVQ src1+16(FP), DX

	PXOR X15, X15

	MOVQ a32+24(FP), AX
	MOVD AX, X4
	PSHUFD $0, X4, X4
	MOVQ b32+32(FP), AX
	MOVD AX, X5
	PSHUFD $0, X5, X5
	MOVQ c32+40(FP), AX
	MOVD AX, X6
	PSHUFD $0, X6, X6
	MOVQ d32+48(FP), AX
	MOVD AX, X7
	PSHUFD $0, X7, X7

	MOVQ (SI), X0
	PUNPCKLBW X15, X0
	MOVQ 1(SI), X1
	PUNPCKLBW X15, X1
	MOVQ (DX), X2
	PUNPCKLBW X15, X2
	MOVQ 1(DX), X3
	PUNPCKLBW X15, X3

	PMULLW X4, X0
	PMULLW X5, X1
	PADDW X1, X0
	PMULLW X6, X2
	PMULLW X7, X3
	PADDW X3, X2
	PADDW X2, X0

	MOVL $0x00200020, AX
	MOVD AX, X8
	PSHUFD $0, X8, X8
	PADDW X8, X0
	PSRAW $6, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)
	RET

// S1b-A2 chroma 4-wide row kernel (SSE2, 4 pixels per call).
// Same math as chromaRow8, bit-identical to the interior scalar tail:
//   v = (A*a+B*b+C*c+D*d+32)>>6.
// Covers cw=4 blocks (bulk=0 today, all-scalar) and the 4-remainder of
// wider partitions. Loads are exact 5-byte (MOVL + MOVL+1 per row:
// src[0..4]), so no overread past the interior guarantee (ix0+cw+1<=w).
// MOVD to X zeroes the high lanes; PUNPCKLBW widens 4 bytes to 4 words.
// Store is exactly 4 bytes via AX (MOVD X->AX + MOVL AX->mem): a direct
// MOVD X0,(DI) assembles to MOVQ (8 bytes, cf. centerVec4 2026-09-22)
// and would overwrite the 4 neighbours right of a 4-wide block.
// Requires SSE2 only (same set as chromaRow8).

// func chromaRow4(dst, src0, src1 unsafe.Pointer, a32, b32, c32, d32 int)
TEXT ·chromaRow4(SB), NOSPLIT, $0-56
	MOVQ dst+0(FP), DI
	MOVQ src0+8(FP), SI
	MOVQ src1+16(FP), DX

	PXOR X15, X15

	MOVQ a32+24(FP), AX
	MOVD AX, X4
	PSHUFD $0, X4, X4
	MOVQ b32+32(FP), AX
	MOVD AX, X5
	PSHUFD $0, X5, X5
	MOVQ c32+40(FP), AX
	MOVD AX, X6
	PSHUFD $0, X6, X6
	MOVQ d32+48(FP), AX
	MOVD AX, X7
	PSHUFD $0, X7, X7

	MOVL (SI), AX
	MOVD AX, X0
	PUNPCKLBW X15, X0
	MOVL 1(SI), AX
	MOVD AX, X1
	PUNPCKLBW X15, X1
	MOVL (DX), AX
	MOVD AX, X2
	PUNPCKLBW X15, X2
	MOVL 1(DX), AX
	MOVD AX, X3
	PUNPCKLBW X15, X3

	PMULLW X4, X0
	PMULLW X5, X1
	PADDW X1, X0
	PMULLW X6, X2
	PMULLW X7, X3
	PADDW X3, X2
	PADDW X2, X0

	MOVL $0x00200020, AX
	MOVD AX, X8
	PSHUFD $0, X8, X8
	PADDW X8, X0
	PSRAW $6, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)
	RET

// S1-D chroma deblock whole-edge kernels (SSE2/SSSE3, 8 outputs/call).
// One call filters a full 8-line chroma edge (4 segments x 2 lines)
// with per-segment tc, mirroring the luma E16 shape: the Go caller
// passes tc8 (8 int16, one per line, negative = skip group) and the
// kernel blends branchlessly (skip = tc<=0 mask folded into GATE).
// Semantics equal four filterChromaEdge calls bit for bit: same taps
// (p1/p0/q0/q1 only — chroma never touches p2/q2 and never updates
// p1/q1), same gates, same delta, same rounding, same stores.
// Only weak edges arrive here (no bS==4, checked by the Go caller);
// intra (bS==4) and mixed edges stay on the per-segment scalar path.
// Requires SSSE3 only for the integer absolute used in gates? No:
// gates use PSUBW+PABSW — PABSW is SSSE3 (same set as the luma V/H
// weak kernels, which already require it on this path).

// func chromaDebVWeak8(p unsafe.Pointer, stride, alpha, beta int, tc8 unsafe.Pointer)
// p = &plane[cy*stride+cx]; 8 rows filtered, taps along x (contiguous:
// p1=[cx-2], p0=[cx-1], q0=[cx], q1=[cx+1] per row).
TEXT ·chromaDebVWeak8(SB), NOSPLIT, $0-40
	MOVQ p+0(FP), DI
	MOVQ stride+8(FP), SI
	MOVQ tc8+32(FP), AX
	MOVOU (AX), X10
	MOVQ alpha+16(FP), AX
	MOVQ beta+24(FP), BX
	// Full 8-lane broadcast (both kernels filter 8 lanes): MOVQ leaves
	// the high quadword untouched and PSHUFLW shuffles within 64-bit
	// halves, so lanes 4-7 would keep caller garbage (measured: lane 4
	// diverged while 0-3 matched). PSHUFD $0x44 copies the low two
	// dwords over the high two (SSE2, same set as the interp kernel).
	MOVQ AX, X8
	PSHUFLW $0, X8, X8
	PSHUFD $0x44, X8, X8
	MOVQ BX, X9
	PSHUFLW $0, X9, X9
	PSHUFD $0x44, X9, X9
	MOVQ DI, R8
	MOVQ DI, R9
	ADDQ SI, R9
	MOVQ R9, R10
	ADDQ SI, R10
	MOVQ R10, R11
	ADDQ SI, R11
	MOVQ R11, R12
	ADDQ SI, R12
	MOVQ R12, R13
	ADDQ SI, R13
	MOVQ R13, R14
	ADDQ SI, R14
	MOVQ R14, R15
	ADDQ SI, R15
	// Gather p1/p0/q0/q1 (positions cx-2..cx+1), 8 rows.
	PXOR X1, X1
	MOVBLZX -2(R8), AX; PINSRW $0, AX, X1
	MOVBLZX -2(R9), AX; PINSRW $1, AX, X1
	MOVBLZX -2(R10), AX; PINSRW $2, AX, X1
	MOVBLZX -2(R11), AX; PINSRW $3, AX, X1
	MOVBLZX -2(R12), AX; PINSRW $4, AX, X1
	MOVBLZX -2(R13), AX; PINSRW $5, AX, X1
	MOVBLZX -2(R14), AX; PINSRW $6, AX, X1
	MOVBLZX -2(R15), AX; PINSRW $7, AX, X1
	PXOR X2, X2
	MOVBLZX -1(R8), AX; PINSRW $0, AX, X2
	MOVBLZX -1(R9), AX; PINSRW $1, AX, X2
	MOVBLZX -1(R10), AX; PINSRW $2, AX, X2
	MOVBLZX -1(R11), AX; PINSRW $3, AX, X2
	MOVBLZX -1(R12), AX; PINSRW $4, AX, X2
	MOVBLZX -1(R13), AX; PINSRW $5, AX, X2
	MOVBLZX -1(R14), AX; PINSRW $6, AX, X2
	MOVBLZX -1(R15), AX; PINSRW $7, AX, X2
	PXOR X3, X3
	MOVBLZX 0(R8), AX; PINSRW $0, AX, X3
	MOVBLZX 0(R9), AX; PINSRW $1, AX, X3
	MOVBLZX 0(R10), AX; PINSRW $2, AX, X3
	MOVBLZX 0(R11), AX; PINSRW $3, AX, X3
	MOVBLZX 0(R12), AX; PINSRW $4, AX, X3
	MOVBLZX 0(R13), AX; PINSRW $5, AX, X3
	MOVBLZX 0(R14), AX; PINSRW $6, AX, X3
	MOVBLZX 0(R15), AX; PINSRW $7, AX, X3
	PXOR X4, X4
	MOVBLZX 1(R8), AX; PINSRW $0, AX, X4
	MOVBLZX 1(R9), AX; PINSRW $1, AX, X4
	MOVBLZX 1(R10), AX; PINSRW $2, AX, X4
	MOVBLZX 1(R11), AX; PINSRW $3, AX, X4
	MOVBLZX 1(R12), AX; PINSRW $4, AX, X4
	MOVBLZX 1(R13), AX; PINSRW $5, AX, X4
	MOVBLZX 1(R14), AX; PINSRW $6, AX, X4
	MOVBLZX 1(R15), AX; PINSRW $7, AX, X4
	// GATE = (|p0-q0|<a) & (|p1-p0|<b) & (|q1-q0|<b), kept under tc>0.
	MOVAPS X2, X11
	PSUBW X3, X11
	PABSW X11, X11
	MOVAPS X8, X12
	PCMPGTW X11, X12
	MOVAPS X1, X11
	PSUBW X2, X11
	PABSW X11, X11
	MOVAPS X9, X13
	PCMPGTW X11, X13
	MOVAPS X4, X11
	PSUBW X3, X11
	PABSW X11, X11
	MOVAPS X9, X14
	PCMPGTW X11, X14
	PAND X13, X12
	PAND X14, X12
	// Skip lanes with tc<=0 (X13/X14 dead: already folded into GATE).
	// tc==0 needs no work either (delta clips to [-0,0]=0, a no-op),
	// so one strict mask covers skips and zero-clips bit-exactly.
	// NOTE (bug signpost): PCMPGTW is dest=(dest>src); the first draft
	// wrote PCMPGTW X10, X11 = (0>tc), all-false, silencing the whole
	// kernel with zero output diff. Direction matters.
	MOVAPS X10, X13
	PXOR X11, X11
	PCMPGTW X11, X13
	PAND X13, X12
	// delta = clip3(((q0-p0)<<2)+(p1-q1)+4)>>3, -tc, tc).
	MOVAPS X3, X11
	PSUBW X2, X11
	PSLLW $2, X11
	MOVAPS X1, X9
	PSUBW X4, X9
	PADDW X9, X11
	PADDW ·chromaDebC4(SB), X11
	PSRAW $3, X11
	PXOR X9, X9
	PSUBW X10, X9
	PMAXSW X9, X11
	PMINSW X10, X11
	MOVAPS X11, X9
	// p0 = GATE ? p0+delta : p0 ; q0 = GATE ? q0-delta : q0.
	MOVAPS X2, X0
	PADDW X9, X0
	PXOR X2, X0
	PAND X12, X0
	PXOR X0, X2
	MOVAPS X3, X0
	PSUBW X9, X0
	PXOR X3, X0
	PAND X12, X0
	PXOR X0, X3
	// Store p0/q0 bytes (lanes to rows).
	PACKUSWB X2, X2
	MOVQ X2, AX
	MOVB AL, -1(R8)
	SHRQ $8, AX; MOVB AL, -1(R9)
	SHRQ $8, AX; MOVB AL, -1(R10)
	SHRQ $8, AX; MOVB AL, -1(R11)
	SHRQ $8, AX; MOVB AL, -1(R12)
	SHRQ $8, AX; MOVB AL, -1(R13)
	SHRQ $8, AX; MOVB AL, -1(R14)
	SHRQ $8, AX; MOVB AL, -1(R15)
	PACKUSWB X3, X3
	MOVQ X3, AX
	MOVB AL, 0(R8)
	SHRQ $8, AX; MOVB AL, 0(R9)
	SHRQ $8, AX; MOVB AL, 0(R10)
	SHRQ $8, AX; MOVB AL, 0(R11)
	SHRQ $8, AX; MOVB AL, 0(R12)
	SHRQ $8, AX; MOVB AL, 0(R13)
	SHRQ $8, AX; MOVB AL, 0(R14)
	SHRQ $8, AX; MOVB AL, 0(R15)
	RET

// func chromaDebHWeak8(p unsafe.Pointer, stride, alpha, beta int, tc8 unsafe.Pointer)
// p = &plane[cy*stride+cx]; 8 columns filtered, taps along y (rows
// cy-2..cy+1, one QWORD per row). tc8 holds per-column-pair tc.
TEXT ·chromaDebHWeak8(SB), NOSPLIT, $0-40
	MOVQ p+0(FP), DI
	MOVQ stride+8(FP), SI
	MOVQ tc8+32(FP), AX
	MOVOU (AX), X10
	MOVQ alpha+16(FP), AX
	MOVQ beta+24(FP), BX
	PXOR X15, X15
	// Full 8-lane broadcast, same fix as the V kernel above (PSHUFLW
	// alone leaves lanes 4-7 as caller garbage).
	MOVQ AX, X8
	PSHUFLW $0, X8, X8
	PSHUFD $0x44, X8, X8
	MOVQ BX, X9
	PSHUFLW $0, X9, X9
	PSHUFD $0x44, X9, X9
	MOVQ SI, AX
	SHLQ $1, AX
	MOVQ DI, R8
	SUBQ AX, R8
	MOVQ R8, R9
	ADDQ SI, R9
	MOVQ R9, R10
	ADDQ SI, R10
	MOVQ R10, R11
	ADDQ SI, R11
	// Gather p1/p0/q0/q1: one QWORD (8 columns) per tap row.
	PXOR X1, X1
	MOVQ (R8), AX; MOVQ AX, X1; PUNPCKLBW X15, X1
	PXOR X2, X2
	MOVQ (R9), AX; MOVQ AX, X2; PUNPCKLBW X15, X2
	PXOR X3, X3
	MOVQ (R10), AX; MOVQ AX, X3; PUNPCKLBW X15, X3
	PXOR X4, X4
	MOVQ (R11), AX; MOVQ AX, X4; PUNPCKLBW X15, X4
	// GATE = (|p0-q0|<a) & (|p1-p0|<b) & (|q1-q0|<b), kept under tc>0.
	MOVAPS X2, X11
	PSUBW X3, X11
	PABSW X11, X11
	MOVAPS X8, X12
	PCMPGTW X11, X12
	MOVAPS X1, X11
	PSUBW X2, X11
	PABSW X11, X11
	MOVAPS X9, X13
	PCMPGTW X11, X13
	MOVAPS X4, X11
	PSUBW X3, X11
	PABSW X11, X11
	MOVAPS X9, X14
	PCMPGTW X11, X14
	PAND X13, X12
	PAND X14, X12
	// Skip lanes with tc<=0 (X13/X14 dead: already folded into GATE).
	// tc==0 needs no work either (delta clips to [-0,0]=0, a no-op),
	// so one strict mask covers skips and zero-clips bit-exactly.
	// NOTE (bug signpost): PCMPGTW is dest=(dest>src); the first draft
	// wrote PCMPGTW X10, X11 = (0>tc), all-false, silencing the whole
	// kernel with zero output diff. Direction matters.
	MOVAPS X10, X13
	PXOR X11, X11
	PCMPGTW X11, X13
	PAND X13, X12
	// delta = clip3(((q0-p0)<<2)+(p1-q1)+4)>>3, -tc, tc).
	MOVAPS X3, X11
	PSUBW X2, X11
	PSLLW $2, X11
	MOVAPS X1, X9
	PSUBW X4, X9
	PADDW X9, X11
	PADDW ·chromaDebC4(SB), X11
	PSRAW $3, X11
	PXOR X9, X9
	PSUBW X10, X9
	PMAXSW X9, X11
	PMINSW X10, X11
	MOVAPS X11, X9
	// p0 = GATE ? p0+delta : p0 ; q0 = GATE ? q0-delta : q0.
	MOVAPS X2, X0
	PADDW X9, X0
	PXOR X2, X0
	PAND X12, X0
	PXOR X0, X2
	MOVAPS X3, X0
	PSUBW X9, X0
	PXOR X3, X0
	PAND X12, X0
	PXOR X0, X3
	// Store p0/q0 QWORDs to rows R9/R10.
	PACKUSWB X2, X2
	MOVQ X2, AX; MOVQ AX, (R9)
	PACKUSWB X3, X3
	MOVQ X3, AX; MOVQ AX, (R10)
	RET

GLOBL ·chromaDebC4(SB), RODATA|NOPTR, $16
DATA ·chromaDebC4+0(SB)/8, $0x0004000400040004
DATA ·chromaDebC4+8(SB)/8, $0x0004000400040004

// S1b-P chroma fused block kernels (SSE2, one call per block).
// The row kernels above cost a call + a Go wrapper row (index math,
// bounds checks) + four weight broadcasts per row: the wrapper flat
// (~110ms per 90f profile) matched the kernels' own flat. Fusing one
// block into one call kills the per-row transitions, the Go row loop,
// and the per-row broadcasts — same taps, same weights, same
// (v+32)>>6, same clip, bit-identical to chromaRow8/Row4 over the
// same rows (chroma_s1_test.go dispatch pins every frac/size against
// the scalar留守, so the fused path is pinned without a second test).
// Peer (ffmpeg, read-only, ideas only): one mc_op call per block with
// an internal row loop (h264chroma_template mc4/mc8 shape), our row
// loops below. Widths 8/16 (block8, groups in-asm) and exactly 4
// (block4); cw==2 and odd widths keep the row path. Loads are the
// same unaligned MOVQ/MOVL shape as the row kernels (interior gate
// guarantees ix0+cw+1<=w, iy0+ch+1<=h). Block4 stores exactly 4
// bytes via AX (a direct MOVD X,(DI) assembles to MOVQ, 8 bytes,
// cf. centerVec4 2026-09-22). Requires SSE2 only (same set as the
// row kernels).

// func chromaBlock8(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, a32, b32, c32, d32 int, w, h int)
// dst: byte rows (stride dstStride); src: row iy0 bytes at ix0
// (row iy0+1 = src+srcStride each row); w in {8,16}, h rows.
TEXT ·chromaBlock8(SB), NOSPLIT, $0-80
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ src+16(FP), SI
	MOVQ srcStride+24(FP), R10
	MOVQ w+64(FP), R8
	MOVQ h+72(FP), R9

	PXOR X15, X15

	MOVQ a32+32(FP), AX
	MOVD AX, X4
	PSHUFD $0, X4, X4
	MOVQ b32+40(FP), AX
	MOVD AX, X5
	PSHUFD $0, X5, X5
	MOVQ c32+48(FP), AX
	MOVD AX, X6
	PSHUFD $0, X6, X6
	MOVQ d32+56(FP), AX
	MOVD AX, X7
	PSHUFD $0, X7, X7

	MOVL $0x00200020, AX
	MOVD AX, X8
	PSHUFD $0, X8, X8

	MOVQ R8, R11
	SHRQ $3, R11

crow:
	MOVQ SI, R12
	ADDQ R10, R12
	MOVQ R11, CX
cgrp:
	MOVQ (SI), X0
	PUNPCKLBW X15, X0
	MOVQ 1(SI), X1
	PUNPCKLBW X15, X1
	MOVQ (R12), X2
	PUNPCKLBW X15, X2
	MOVQ 1(R12), X3
	PUNPCKLBW X15, X3

	PMULLW X4, X0
	PMULLW X5, X1
	PADDW X1, X0
	PMULLW X6, X2
	PMULLW X7, X3
	PADDW X3, X2
	PADDW X2, X0

	PADDW X8, X0
	PSRAW $6, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)

	ADDQ $8, SI
	ADDQ $8, R12
	ADDQ $8, DI
	SUBQ $1, CX
	JGT cgrp

	SUBQ R8, SI
	ADDQ R10, SI
	SUBQ R8, DI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT crow
	RET

// func chromaBlock4(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, a32, b32, c32, d32 int, h int)
// Same as block8 with the 4-wide body (exact 5-byte loads, exact
// 4-byte store via AX); w is fixed 4, only h rows vary.
TEXT ·chromaBlock4(SB), NOSPLIT, $0-72
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ src+16(FP), SI
	MOVQ srcStride+24(FP), R10
	MOVQ h+64(FP), R9

	PXOR X15, X15

	MOVQ a32+32(FP), AX
	MOVD AX, X4
	PSHUFD $0, X4, X4
	MOVQ b32+40(FP), AX
	MOVD AX, X5
	PSHUFD $0, X5, X5
	MOVQ c32+48(FP), AX
	MOVD AX, X6
	PSHUFD $0, X6, X6
	MOVQ d32+56(FP), AX
	MOVD AX, X7
	PSHUFD $0, X7, X7

	MOVL $0x00200020, AX
	MOVD AX, X8
	PSHUFD $0, X8, X8

c4row:
	MOVQ SI, R12
	ADDQ R10, R12

	MOVL (SI), AX
	MOVD AX, X0
	PUNPCKLBW X15, X0
	MOVL 1(SI), AX
	MOVD AX, X1
	PUNPCKLBW X15, X1
	MOVL (R12), AX
	MOVD AX, X2
	PUNPCKLBW X15, X2
	MOVL 1(R12), AX
	MOVD AX, X3
	PUNPCKLBW X15, X3

	PMULLW X4, X0
	PMULLW X5, X1
	PADDW X1, X0
	PMULLW X6, X2
	PMULLW X7, X3
	PADDW X3, X2
	PADDW X2, X0

	PADDW X8, X0
	PSRAW $6, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)

	ADDQ R10, SI
	ADDQ DX, DI
	SUBQ $1, R9
	JGT c4row
	RET
