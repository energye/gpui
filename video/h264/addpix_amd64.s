// S1b-A3 amd64 residual-add row kernel (SSE2, 8 pixels per call).
// Matches addResidScalar bit for bit:
//   dst[i] = clip(pred[i]+res[i]), i=0..7.
// Pred bytes widen via PUNPCKLBW/PUNPCKHBW (low/high 4-packs); the two
// res dword quads narrow via one PACKSSLW (dwords->words, Go spelling
// of PACKSSDW: PACKSSLW X3,X2 packs X2-low+X3-low into X2-low and
// X2-high+X3-high into X2-high, so X2-low holds res[0..3] and X3-low
// holds res[4..7]); the final PACKUSWB saturates to bytes (== clipPixel:
// negatives to 0, >255 to 255).
// PACKSSLW pre-saturation is exact: pred is in 0..255, so saturating
// res to int16 before the add never changes the final clip (a true sum
// past either clip bound stays past it after saturation).
// Requires only SSE2 (baseline).

#include "textflag.h"

// func addResidRow8(dst, pred, res unsafe.Pointer)
// dst: *uint8[8] out; pred: *uint8[8]; res: *int32[8].
TEXT ·addResidRow8(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ pred+8(FP), SI
	MOVQ res+16(FP), DX

	PXOR X15, X15
	MOVQ (SI), X0
	MOVAPS X0, X1
	PUNPCKLBW X15, X0
	PUNPCKHBW X15, X1

	MOVOU (DX), X2
	MOVOU 16(DX), X3
	PACKSSLW X3, X2
	PADDW X2, X0
	// High half: res[4..7] arrive in X3 low after the pack; pred high
	// sits in X1. Reuse the pack result directly (no second pack).
	PADDW X3, X1
	PACKUSWB X1, X0
	MOVQ X0, (DI)
	RET

// S1b-R residual-add fused block kernels (SSE2, one call per block).
// The row kernels above cost a call + a Go wrapper row (slicing,
// bounds checks, base math: ~60ms flat + ~60ms slicing per 90f
// profile) per row: the wrapper matched the kernels' own flat.
// Fusing one block into one call kills the per-row transitions and
// the Go row loop — same formula, same PACKSSLW pre-saturation, same
// PACKUSWB clip, bit-identical to addResidRow8/Row4 over the same
// rows (addpix_s1_test.go dispatch pins every size/position against
// the scalar oracle, so the fused path is pinned without a new test).
// Peer (ffmpeg, read-only, ideas only): one add_pixels call per block
// with an internal row loop (h264addpx_template mc shape), our row
// loops below. Widths 4/8 (Go-gated); pic stride and pred stride ride
// as uintptr; res rows are contiguous (w int32 each). Block4 stores
// exactly 4 bytes via AX (a direct MOVD X,(DI) assembles to MOVQ,
// 8 bytes, cf. centerVec4 2026-09-22). Requires SSE2 only (same set
// as the row kernels).

// func addResidBlock8(dst unsafe.Pointer, dstStride uintptr, pred unsafe.Pointer, predStride uintptr, res unsafe.Pointer, h int)
// dst: pic rows (stride dstStride bytes) at (bx,by); pred: rows
// (stride predStride); res: 8 int32 per row, contiguous.
TEXT ·addResidBlock8(SB), NOSPLIT, $0-48
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ pred+16(FP), SI
	MOVQ predStride+24(FP), R10
	MOVQ res+32(FP), R11
	MOVQ h+40(FP), R9

	PXOR X15, X15

arow8:
	MOVQ (SI), X0
	MOVAPS X0, X1
	PUNPCKLBW X15, X0
	PUNPCKHBW X15, X1

	MOVOU (R11), X2
	MOVOU 16(R11), X3
	PACKSSLW X3, X2
	PADDW X2, X0
	PADDW X3, X1
	PACKUSWB X1, X0
	MOVQ X0, (DI)

	ADDQ R10, SI
	ADDQ DX, DI
	ADDQ $32, R11
	SUBQ $1, R9
	JGT arow8
	RET

// func addResidBlock4(dst unsafe.Pointer, dstStride uintptr, pred unsafe.Pointer, predStride uintptr, res unsafe.Pointer, h int)
// Same as block8 with the 4-wide body (exact 4B pred load, MOVOU 16B
// res = 4 int32, exact 4B store via AX).
TEXT ·addResidBlock4(SB), NOSPLIT, $0-48
	MOVQ dst+0(FP), DI
	MOVQ dstStride+8(FP), DX
	MOVQ pred+16(FP), SI
	MOVQ predStride+24(FP), R10
	MOVQ res+32(FP), R11
	MOVQ h+40(FP), R9

	PXOR X15, X15

arow4:
	MOVL (SI), AX
	MOVD AX, X0
	PUNPCKLBW X15, X0

	MOVOU (R11), X2
	PXOR X3, X3
	PACKSSLW X3, X2
	PADDW X2, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)

	ADDQ R10, SI
	ADDQ DX, DI
	ADDQ $16, R11
	SUBQ $1, R9
	JGT arow4
	RET

// S1b-A3 amd64 residual-add 4-wide row kernel (SSE2, 4 pixels/call).
//   dst[i] = clip(pred[i]+res[i]), i=0..3.
// Covers 4-wide rows (the bulk of luma 4x4 and chroma 4x4 adds: the old
// scalar+tmp+copy path cost ~half of addResidArch's 220ms cum on the
// 90f profile). Loads are exact (MOVL 4B pred, MOVOU 16B res = 4 int32,
// no overread). PACKSSLW pre-saturation is exact: pred is in 0..255, so
// saturating res to int16 before the add never changes the final clip
// (a true sum past either clip bound stays past it after saturation).
// Store is exactly 4 bytes via AX (MOVD X->AX + MOVL AX->mem): a direct
// MOVD X0,(DI) assembles to MOVQ (8 bytes, cf. centerVec4 2026-09-22)
// and would overwrite the 4 neighbours right of a 4-wide block.
// Requires SSE2 only (same set as addResidRow8).

// func addResidRow4(dst, pred, res unsafe.Pointer)
// dst: *uint8[4] out; pred: *uint8[4]; res: *int32[4].
TEXT ·addResidRow4(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ pred+8(FP), SI
	MOVQ res+16(FP), DX

	PXOR X15, X15
	MOVL (SI), AX
	MOVD AX, X0
	PUNPCKLBW X15, X0

	MOVOU (DX), X2
	PXOR X3, X3
	PACKSSLW X3, X2
	PADDW X2, X0
	PACKUSWB X0, X0
	MOVD X0, AX
	MOVL AX, (DI)
	RET
