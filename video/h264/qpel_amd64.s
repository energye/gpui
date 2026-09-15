// S1 amd64 qpel row kernel (SSSE3, 8 pixels per iteration).
// Unrounded 6-tap horizontal sums with taps [1 -5 20 20 -5 1] (same sum
// as halfH pre-round, int16-exact: range -2550..10710). Each tap pair
// runs as one PMADDUBSW (unsigned pixels, signed taps); even/odd outputs
// merge with PUNPCKLWL (Go asm spelling of PUNPCKLWD: unpack low words,
// see convert_amd64.s NOTE). Requires SSSE3 (PMADDUBSW, implied by the
// baseline, see convert_fast.go). Rounding/clip/avg stay in Go
// (qpel_amd64.go), so this kernel cannot drift the output bits.

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
