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
