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
