// S1 amd64 row kernel (SSE4.1, 4 pixels per iteration).
// Matches convertBandScalar bit for bit:
// R=(yMul*(Y-yOff)+rCr*e+128)>>8 etc, then clip via unsigned saturation.
// Requires SSE4.1 (PMOVZXBD, PMULLD, PACKUSDW); see convert_fast.go.
// Note: Go asm spells dword lanes with L (PSUBL/PADDL/PSRAL/PCMPEQL).

#include "textflag.h"

// func simdRowBulk(dst, y, cb, cr unsafe.Pointer, n int, yMul, yOff, rCr, gCb, gCr, bCb int)
TEXT ·simdRowBulk(SB), NOSPLIT, $0-88
	MOVQ dst+0(FP), DI
	MOVQ y+8(FP), SI
	MOVQ cb+16(FP), DX
	MOVQ cr+24(FP), CX
	MOVQ n+32(FP), R8
	CMPQ R8, $0
	JLE  done

	MOVQ yMul+40(FP), AX
	MOVD AX, X9
	PSHUFD $0, X9, X9
	MOVQ yOff+48(FP), AX
	MOVD AX, X10
	PSHUFD $0, X10, X10
	MOVQ rCr+56(FP), AX
	MOVD AX, X11
	PSHUFD $0, X11, X11
	MOVQ gCb+64(FP), AX
	MOVD AX, X12
	PSHUFD $0, X12, X12
	MOVQ gCr+72(FP), AX
	MOVD AX, X13
	PSHUFD $0, X13, X13
	MOVQ bCb+80(FP), AX
	MOVD AX, X14
	PSHUFD $0, X14, X14
	MOVL $128, AX
	MOVD AX, X15
	PSHUFD $0, X15, X15

loop:
	MOVL (SI), AX
	MOVD AX, X0
	PMOVZXBD X0, X0
	PSUBL X10, X0
	PMULLD X9, X0

	XORL AX, AX
	MOVW (DX), AX
	MOVD AX, X1
	PMOVZXBD X1, X1
	PSUBL X15, X1
	PSHUFD $0x50, X1, X1

	XORL AX, AX
	MOVW (CX), AX
	MOVD AX, X2
	PMOVZXBD X2, X2
	PSUBL X15, X2
	PSHUFD $0x50, X2, X2

	MOVAPS X1, X3
	PMULLD X12, X3
	MOVAPS X2, X4
	PMULLD X13, X4
	MOVAPS X1, X5
	PMULLD X14, X5
	PMULLD X11, X2

	MOVAPS X0, X6
	PADDL  X2, X6
	PADDL  X15, X6
	PSRAL  $8, X6

	MOVAPS X0, X7
	PSUBL  X3, X7
	PSUBL  X4, X7
	PADDL  X15, X7
	PSRAL  $8, X7

	MOVAPS X0, X8
	PADDL  X5, X8
	PADDL  X15, X8
	PSRAL  $8, X8

	PXOR     X0, X0
	PACKUSDW X0, X6
	PACKUSWB  X0, X6
	PACKUSDW X0, X7
	PACKUSWB  X0, X7
	PACKUSDW X0, X8
	PACKUSWB  X0, X8

	PUNPCKLBW X7, X6
	PCMPEQL   X7, X7
	PUNPCKLBW X7, X8
	PUNPCKLWL X8, X6
	MOVOU X6, (DI)

	ADDQ $4, SI
	ADDQ $16, DI
	ADDQ $2, DX
	ADDQ $2, CX
	SUBQ $4, R8
	JGT  loop

done:
	RET
