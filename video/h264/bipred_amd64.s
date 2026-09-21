// S1b-A2 amd64 bipred row kernel (SSE2, 8 pixels per call).
// Matches bipredAvg bit for bit:
//   dst[i] = (w*dst[i]+(64-w)*s1[i]+32)>>6, w in 0..64.
// Weights arrive packed twice (W|W<<16) so MOVD+PSHUFD broadcasts to 8
// identical 16-bit lanes (same trick as chroma_amd64.s). Pixels widen
// via PUNPCKLBW; products stay 16-bit (max 64*255+32 = 16352 < 32767).
// Requires only SSE2 (baseline).

#include "textflag.h"

// func bipredRow8Arch(dst, src1 unsafe.Pointer, w32, w132 int)
// dst holds a in place, src1 holds b; result overwrites dst.
TEXT ·bipredRow8Arch(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src1+8(FP), SI

	MOVQ w32+16(FP), AX
	MOVD AX, X4
	PSHUFD $0, X4, X4
	MOVQ w132+24(FP), AX
	MOVD AX, X5
	PSHUFD $0, X5, X5

	PXOR X15, X15
	MOVQ (DI), X0
	PUNPCKLBW X15, X0
	MOVQ (SI), X1
	PUNPCKLBW X15, X1

	PMULLW X4, X0
	PMULLW X5, X1
	PADDW X1, X0

	MOVL $0x00200020, AX
	MOVD AX, X6
	PSHUFD $0, X6, X6
	PADDW X6, X0
	PSRAW $6, X0
	PACKUSWB X0, X0
	MOVQ X0, (DI)
	RET
