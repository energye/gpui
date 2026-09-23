// S1-T 8x8 inverse transform kernels (SSE2, 4 columns/rows per batch).
// Two kernels composed by itrans8x8Arch (itrans_amd64.go): colPass1
// (columns of c -> t1 TRANSPOSED MIX rows) and rowPass2 (MIX rows ->
// out in the spec transposed final layout with >>6). Structure mirrors
// ffmpeg's IDCT8_ADD_SSE (1D pass, transpose, 1D pass) with the
// transpose FOLDED into the pass-1 stores (zero extra pass); the math
// is our exact idct8_1d lane by lane, so output is bit-identical to
// itrans8x8Core: every op is lane-local 32-bit (PADDL/PSUBL wrap mod
// 2^32 exactly like Go int32/uint32, PSRAL floors exactly like Go >>).
// Requires SSE2 only (baseline, same set as the chroma row kernel).
//
// Batch layout: pass 1 batches are COLUMNS (DX=0/16 bytes: cols 0-3,
// 4-7); pass 2 batches are ROWS (DX=0/128 bytes: rows 0-3, 4-7).
// NOTE (bug signposts, all measured): (1) the first fused draft stored
// pass-2 lanes without PSRLDQ extracts (same lane 4x); (2) its loop
// tail had no ADDQ/CMPQ so batch 1 reran batch 0; (3) the >>6 folded
// BEFORE the final combines halves the rounding — shift O-regs only;
// (4) pass-2 loads must GATHER rows (stride-32) with SCATTER stores,
// not column assembly; (5) the transpose is STRUCTURAL (reference
// keeps pass-1 output transposed): pass-1 stores transpose, pass-2
// stores transpose back. Review any new store group against the
// MOVD+PSRLDQ extract pattern and the O-reg map below.
#include "textflag.h"

TEXT ·itransColPass1(SB), NOSPLIT, $0-16
	MOVQ c+0(FP), SI
	MOVQ t1+8(FP), DI
	MOVQ $0, DX
p1batch:
	MOVQ SI, R10
	ADDQ DX, R10
	// Row loads: Xk = 4 lanes of row k at columns DX/4..DX/4+3.
	MOVL 0(R10), AX
	MOVD AX, X0
	MOVL 4(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X0
	MOVL 8(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X0
	MOVL 12(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X0
	MOVL 32(R10), AX
	MOVD AX, X1
	MOVL 36(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X1
	MOVL 40(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X1
	MOVL 44(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X1
	MOVL 64(R10), AX
	MOVD AX, X2
	MOVL 68(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X2
	MOVL 72(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X2
	MOVL 76(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X2
	MOVL 96(R10), AX
	MOVD AX, X3
	MOVL 100(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X3
	MOVL 104(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X3
	MOVL 108(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X3
	MOVL 128(R10), AX
	MOVD AX, X4
	MOVL 132(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X4
	MOVL 136(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X4
	MOVL 140(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X4
	MOVL 160(R10), AX
	MOVD AX, X5
	MOVL 164(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X5
	MOVL 168(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X5
	MOVL 172(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X5
	MOVL 192(R10), AX
	MOVD AX, X6
	MOVL 196(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X6
	MOVL 200(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X6
	MOVL 204(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X6
	MOVL 224(R10), AX
	MOVD AX, X7
	MOVL 228(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X7
	MOVL 232(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X7
	MOVL 236(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X7
	// blk[0] += 32 on batch 0 only (element (0,0) = lane 0 of X0).
	CMPQ DX, $0
	JNE p1butterfly
	PADDL ·itransK32(SB), X0
p1butterfly:
	// Butterfly, exact idct8_1d order per lane.
	// Even taps: A0=W0+W4 (X8), A2=W0-W4 (X9),
	// A4=(W2>>1)-W6 (X10), A6=(W6>>1)+W2 (X11).
	MOVAPS X0, X8
	PADDL X4, X8
	MOVAPS X0, X9
	PSUBL X4, X9
	MOVAPS X2, X10
	PSRAL $1, X10
	PSUBL X6, X10
	MOVAPS X6, X11
	PSRAL $1, X11
	PADDL X2, X11
	// B0=A0+A6 (X12), B2=A2+A4 (X13), B4=A2-A4 (X14), B6=A0-A6 (X15).
	MOVAPS X8, X12
	PADDL X11, X12
	MOVAPS X9, X13
	PADDL X10, X13
	MOVAPS X9, X14
	PSUBL X10, X14
	MOVAPS X8, X15
	PSUBL X11, X15
	// Odd taps (X0 = zero; W0/W2/W4/W6 dead now).
	// A1=-W3+W5-W7-(W7>>1) (X2), A3=W1+W7-W3-(W3>>1) (X4).
	PXOR X0, X0
	MOVAPS X0, X2
	PSUBL X3, X2
	PADDL X5, X2
	PSUBL X7, X2
	MOVAPS X7, X4
	PSRAL $1, X4
	PSUBL X4, X2
	MOVAPS X1, X4
	PADDL X7, X4
	PSUBL X3, X4
	MOVAPS X3, X6
	PSRAL $1, X6
	PSUBL X6, X4
	// A7=W3+W5+W1+(W1>>1) (X6; W1 still live in X1).
	MOVAPS X3, X6
	PADDL X5, X6
	PADDL X1, X6
	MOVAPS X1, X0
	PSRAL $1, X0
	PADDL X0, X6
	PXOR X0, X0
	// A5=-W1+W7+W5+(W5>>1) (X9; A2 dead, X8 free for the shift copy).
	MOVAPS X5, X8
	PSRAL $1, X8
	MOVAPS X0, X9
	PSUBL X1, X9
	PADDL X7, X9
	PADDL X5, X9
	PADDL X8, X9
	// B1=(A7>>2)+A1 (X10), B3=A3+(A5>>2) (X11),
	// B5=(A3>>2)-A5 (X8), B7=A7-(A1>>2) (X6 in place).
	MOVAPS X6, X10
	PSRAL $2, X10
	PADDL X2, X10
	MOVAPS X9, X11
	PSRAL $2, X11
	PADDL X4, X11
	MOVAPS X4, X8
	PSRAL $2, X8
	PSUBL X9, X8
	MOVAPS X2, X9
	PSRAL $2, X9
	PSUBL X9, X6
	// Outputs (X2/X4 free: A1/A3 dead).
	// O0=B0+B7 (X0 scratch), O7=B0-B7 (X12 in place),
	// O1=B2+B5 (X2), O6=B2-B5 (X13 in place),
	// O2=B4+B3 (X4), O5=B4-B3 (X14 in place),
	// O4=B6-B1 (X9), O3=B6+B1 (X15 in place).
	MOVAPS X12, X0
	PADDL X6, X0
	PSUBL X6, X12
	MOVAPS X13, X2
	PADDL X8, X2
	PSUBL X8, X13
	MOVAPS X14, X4
	PADDL X11, X4
	PSUBL X11, X14
	MOVAPS X15, X9
	PSUBL X10, X9
	PADDL X10, X15
	// Transposed stores: Oi[j'] (col j+j' tap i) -> t1[i*8+j+j'],
	// byte i*32+(j+j')*4. Base DI+DX (DX=j*4): row i at +i*32, plus j*4.
	// Oi regs: O0=X0, O1=X2, O2=X4, O3=X15, O4=X9, O5=X14, O6=X13, O7=X12.
	MOVQ DI, R10
	ADDQ DX, R10
	MOVD X0, AX
	MOVL AX, 0(R10)
	PSRLDQ $4, X0
	MOVD X0, AX
	MOVL AX, 4(R10)
	PSRLDQ $4, X0
	MOVD X0, AX
	MOVL AX, 8(R10)
	PSRLDQ $4, X0
	MOVD X0, AX
	MOVL AX, 12(R10)
	MOVD X2, AX
	MOVL AX, 32(R10)
	PSRLDQ $4, X2
	MOVD X2, AX
	MOVL AX, 36(R10)
	PSRLDQ $4, X2
	MOVD X2, AX
	MOVL AX, 40(R10)
	PSRLDQ $4, X2
	MOVD X2, AX
	MOVL AX, 44(R10)
	MOVD X4, AX
	MOVL AX, 64(R10)
	PSRLDQ $4, X4
	MOVD X4, AX
	MOVL AX, 68(R10)
	PSRLDQ $4, X4
	MOVD X4, AX
	MOVL AX, 72(R10)
	PSRLDQ $4, X4
	MOVD X4, AX
	MOVL AX, 76(R10)
	MOVD X15, AX
	MOVL AX, 96(R10)
	PSRLDQ $4, X15
	MOVD X15, AX
	MOVL AX, 100(R10)
	PSRLDQ $4, X15
	MOVD X15, AX
	MOVL AX, 104(R10)
	PSRLDQ $4, X15
	MOVD X15, AX
	MOVL AX, 108(R10)
	MOVD X9, AX
	MOVL AX, 128(R10)
	PSRLDQ $4, X9
	MOVD X9, AX
	MOVL AX, 132(R10)
	PSRLDQ $4, X9
	MOVD X9, AX
	MOVL AX, 136(R10)
	PSRLDQ $4, X9
	MOVD X9, AX
	MOVL AX, 140(R10)
	MOVD X14, AX
	MOVL AX, 160(R10)
	PSRLDQ $4, X14
	MOVD X14, AX
	MOVL AX, 164(R10)
	PSRLDQ $4, X14
	MOVD X14, AX
	MOVL AX, 168(R10)
	PSRLDQ $4, X14
	MOVD X14, AX
	MOVL AX, 172(R10)
	MOVD X13, AX
	MOVL AX, 192(R10)
	PSRLDQ $4, X13
	MOVD X13, AX
	MOVL AX, 196(R10)
	PSRLDQ $4, X13
	MOVD X13, AX
	MOVL AX, 200(R10)
	PSRLDQ $4, X13
	MOVD X13, AX
	MOVL AX, 204(R10)
	MOVD X12, AX
	MOVL AX, 224(R10)
	PSRLDQ $4, X12
	MOVD X12, AX
	MOVL AX, 228(R10)
	PSRLDQ $4, X12
	MOVD X12, AX
	MOVL AX, 232(R10)
	PSRLDQ $4, X12
	MOVD X12, AX
	MOVL AX, 236(R10)
ADDQ $16, DX
	CMPQ DX, $32
	JLT p1batch
	RET

TEXT ·itransRowPass2(SB), NOSPLIT, $0-16
	MOVQ t1+0(FP), SI
	MOVQ out+8(FP), DI
	MOVQ $0, DX
p2batch:
	MOVQ SI, R10
	ADDQ DX, R10
	MOVL 0(R10), AX
	MOVD AX, X0
	MOVL 32(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X0
	MOVL 64(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X0
	MOVL 96(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X0
	MOVL 4(R10), AX
	MOVD AX, X1
	MOVL 36(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X1
	MOVL 68(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X1
	MOVL 100(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X1
	MOVL 8(R10), AX
	MOVD AX, X2
	MOVL 40(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X2
	MOVL 72(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X2
	MOVL 104(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X2
	MOVL 12(R10), AX
	MOVD AX, X3
	MOVL 44(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X3
	MOVL 76(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X3
	MOVL 108(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X3
	MOVL 16(R10), AX
	MOVD AX, X4
	MOVL 48(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X4
	MOVL 80(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X4
	MOVL 112(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X4
	MOVL 20(R10), AX
	MOVD AX, X5
	MOVL 52(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X5
	MOVL 84(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X5
	MOVL 116(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X5
	MOVL 24(R10), AX
	MOVD AX, X6
	MOVL 56(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X6
	MOVL 88(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X6
	MOVL 120(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X6
	MOVL 28(R10), AX
	MOVD AX, X7
	MOVL 60(R10), AX
	MOVD AX, X8
	PSLLDQ $4, X8
	POR X8, X7
	MOVL 92(R10), AX
	MOVD AX, X8
	PSLLDQ $8, X8
	POR X8, X7
	MOVL 124(R10), AX
	MOVD AX, X8
	PSLLDQ $12, X8
	POR X8, X7
// Butterfly (same order as pass 1).
	MOVAPS X0, X8
	PADDL X4, X8
	MOVAPS X0, X9
	PSUBL X4, X9
	MOVAPS X2, X10
	PSRAL $1, X10
	PSUBL X6, X10
	MOVAPS X6, X11
	PSRAL $1, X11
	PADDL X2, X11
	MOVAPS X8, X12
	PADDL X11, X12
	MOVAPS X9, X13
	PADDL X10, X13
	MOVAPS X9, X14
	PSUBL X10, X14
	MOVAPS X8, X15
	PSUBL X11, X15
	PXOR X0, X0
	MOVAPS X0, X2
	PSUBL X3, X2
	PADDL X5, X2
	PSUBL X7, X2
	MOVAPS X7, X4
	PSRAL $1, X4
	PSUBL X4, X2
	MOVAPS X1, X4
	PADDL X7, X4
	PSUBL X3, X4
	MOVAPS X3, X6
	PSRAL $1, X6
	PSUBL X6, X4
	MOVAPS X3, X6
	PADDL X5, X6
	PADDL X1, X6
	MOVAPS X1, X0
	PSRAL $1, X0
	PADDL X0, X6
	PXOR X0, X0
	MOVAPS X5, X8
	PSRAL $1, X8
	MOVAPS X0, X9
	PSUBL X1, X9
	PADDL X7, X9
	PADDL X5, X9
	PADDL X8, X9
	MOVAPS X6, X10
	PSRAL $2, X10
	PADDL X2, X10
	MOVAPS X9, X11
	PSRAL $2, X11
	PADDL X4, X11
	MOVAPS X4, X8
	PSRAL $2, X8
	PSUBL X9, X8
	MOVAPS X2, X9
	PSRAL $2, X9
	PSUBL X9, X6
	MOVAPS X12, X0
	PADDL X6, X0
	PSUBL X6, X12
	MOVAPS X13, X2
	PADDL X8, X2
	PSUBL X8, X13
	MOVAPS X14, X4
	PADDL X11, X4
	PSUBL X11, X14
	MOVAPS X15, X9
	PSUBL X10, X9
	PADDL X10, X15
	// >>6 fold on the eight final outputs (spec final normalization;
	// exactly the r[k]>>6 in itrans8x8Core — shifting B-stage regs
	// before combining halves the rounding, so shift O-regs only).
	PSRAL $6, X0
	PSRAL $6, X2
	PSRAL $6, X4
	PSRAL $6, X15
	PSRAL $6, X9
	PSRAL $6, X14
	PSRAL $6, X13
	PSRAL $6, X12
	// Transposed stores: want ROW i = tap i down the input rows = Oi
	// lanes across the batch. Oi lane j' -> out[i*8+j+j'], byte
	// i*32+(j+j')*4. Store base R11 = DI+j*4 (DX/8); row i at +i*32.
	// Oi regs: O0=X0, O1=X2, O2=X4, O3=X15, O4=X9, O5=X14, O6=X13, O7=X12.
	MOVQ DI, R11
	MOVQ DX, R10
	SHRQ $3, R10
	ADDQ R10, R11
	MOVD X0, AX
	MOVL AX, 0(R11)
	PSRLDQ $4, X0
	MOVD X0, AX
	MOVL AX, 4(R11)
	PSRLDQ $4, X0
	MOVD X0, AX
	MOVL AX, 8(R11)
	PSRLDQ $4, X0
	MOVD X0, AX
	MOVL AX, 12(R11)
	MOVD X2, AX
	MOVL AX, 32(R11)
	PSRLDQ $4, X2
	MOVD X2, AX
	MOVL AX, 36(R11)
	PSRLDQ $4, X2
	MOVD X2, AX
	MOVL AX, 40(R11)
	PSRLDQ $4, X2
	MOVD X2, AX
	MOVL AX, 44(R11)
	MOVD X4, AX
	MOVL AX, 64(R11)
	PSRLDQ $4, X4
	MOVD X4, AX
	MOVL AX, 68(R11)
	PSRLDQ $4, X4
	MOVD X4, AX
	MOVL AX, 72(R11)
	PSRLDQ $4, X4
	MOVD X4, AX
	MOVL AX, 76(R11)
	MOVD X15, AX
	MOVL AX, 96(R11)
	PSRLDQ $4, X15
	MOVD X15, AX
	MOVL AX, 100(R11)
	PSRLDQ $4, X15
	MOVD X15, AX
	MOVL AX, 104(R11)
	PSRLDQ $4, X15
	MOVD X15, AX
	MOVL AX, 108(R11)
	MOVD X9, AX
	MOVL AX, 128(R11)
	PSRLDQ $4, X9
	MOVD X9, AX
	MOVL AX, 132(R11)
	PSRLDQ $4, X9
	MOVD X9, AX
	MOVL AX, 136(R11)
	PSRLDQ $4, X9
	MOVD X9, AX
	MOVL AX, 140(R11)
	MOVD X14, AX
	MOVL AX, 160(R11)
	PSRLDQ $4, X14
	MOVD X14, AX
	MOVL AX, 164(R11)
	PSRLDQ $4, X14
	MOVD X14, AX
	MOVL AX, 168(R11)
	PSRLDQ $4, X14
	MOVD X14, AX
	MOVL AX, 172(R11)
	MOVD X13, AX
	MOVL AX, 192(R11)
	PSRLDQ $4, X13
	MOVD X13, AX
	MOVL AX, 196(R11)
	PSRLDQ $4, X13
	MOVD X13, AX
	MOVL AX, 200(R11)
	PSRLDQ $4, X13
	MOVD X13, AX
	MOVL AX, 204(R11)
	MOVD X12, AX
	MOVL AX, 224(R11)
	PSRLDQ $4, X12
	MOVD X12, AX
	MOVL AX, 228(R11)
	PSRLDQ $4, X12
	MOVD X12, AX
	MOVL AX, 232(R11)
	PSRLDQ $4, X12
	MOVD X12, AX
	MOVL AX, 236(R11)
	ADDQ $128, DX
	CMPQ DX, $256
JLT p2batch
	RET

GLOBL ·itransK32(SB), RODATA|NOPTR, $16
DATA ·itransK32+0(SB)/4, $32
DATA ·itransK32+4(SB)/4, $0
DATA ·itransK32+8(SB)/4, $0
DATA ·itransK32+12(SB)/4, $0

// S1b-Y 4x4 inverse transform kernel (SSE2, one call per block).
// Single call runs the whole itrans4x4Core: column gather, horizontal
// butterfly, transpose, vertical butterfly, (f+32)>>6, raster stores.
// No memory temp (everything rides XMM); the caller owns nothing but
// c/out. Structure mirrors the 8x8 two-pass shape with the transpose
// FOLDED between passes (zero extra pass); the math is the exact
// 4x4 lane equations (s0=c0+c2, s1=c0-c2, s2=(c1>>1)-c3,
// s3=c1+(c3>>1), e0=s0+s3, e1=s1+s2, e2=s1-s2, e3=s0-s3), so output is
// bit-identical to itrans4x4CoreScalar: every op is lane-local 32-bit
// (PADDL/PSUBL wrap mod 2^32 exactly like Go int32, PSRAL assembles
// to PSRAD — verified by objdump 2026-09-23 — and floors exactly
// like Go >>). Peer (ffmpeg, read-only, ideas only, no code copied):
// libavcodec/h264_idct.c ff_h264_idct_add (1D pass, transpose, 1D
// pass), our gather/transpose/rows below. Requires SSE2 only
// (baseline, same set as the 8x8 kernels).
//
// Register map: X0..X3 = columns (lane j = row j), X4..X7 = S/E
// temps, then T0..T3, then R0..R3 rows, then F rows. All within
// X0..X7. Transpose: T0=UNPCKL(E0,E2), T1=UNPCKL(E1,E3),
// T2=UNPCKH(E0,E2), T3=UNPCKH(E1,E3); R0=UNPCKL(T0,T1),
// R1=UNPCKH(T0,T1), R2=UNPCKL(T2,T3), R3=UNPCKH(T2,T3).

GLOBL ·itrans4x4K32(SB), RODATA|NOPTR, $16
DATA ·itrans4x4K32+0(SB)/4, $32
DATA ·itrans4x4K32+4(SB)/4, $32
DATA ·itrans4x4K32+8(SB)/4, $32
DATA ·itrans4x4K32+12(SB)/4, $32

// func itrans4x4Block(c, out unsafe.Pointer)
// c: 16 int32 raster (4 contiguous rows of 4); out: 16 int32 raster.
TEXT ·itrans4x4Block(SB), NOSPLIT, $0-16
	MOVQ c+0(FP), SI
	MOVQ out+8(FP), DI
	// Column gather: Xk = column k, lane j = row j (stride 16).
	MOVL 0(SI), AX
	MOVD AX, X0
	MOVL 16(SI), AX
	MOVD AX, X4
	PSLLDQ $4, X4
	POR X4, X0
	MOVL 32(SI), AX
	MOVD AX, X4
	PSLLDQ $8, X4
	POR X4, X0
	MOVL 48(SI), AX
	MOVD AX, X4
	PSLLDQ $12, X4
	POR X4, X0
	MOVL 4(SI), AX
	MOVD AX, X1
	MOVL 20(SI), AX
	MOVD AX, X4
	PSLLDQ $4, X4
	POR X4, X1
	MOVL 36(SI), AX
	MOVD AX, X4
	PSLLDQ $8, X4
	POR X4, X1
	MOVL 52(SI), AX
	MOVD AX, X4
	PSLLDQ $12, X4
	POR X4, X1
	MOVL 8(SI), AX
	MOVD AX, X2
	MOVL 24(SI), AX
	MOVD AX, X4
	PSLLDQ $4, X4
	POR X4, X2
	MOVL 40(SI), AX
	MOVD AX, X4
	PSLLDQ $8, X4
	POR X4, X2
	MOVL 56(SI), AX
	MOVD AX, X4
	PSLLDQ $12, X4
	POR X4, X2
	MOVL 12(SI), AX
	MOVD AX, X3
	MOVL 28(SI), AX
	MOVD AX, X4
	PSLLDQ $4, X4
	POR X4, X3
	MOVL 44(SI), AX
	MOVD AX, X4
	PSLLDQ $8, X4
	POR X4, X3
	MOVL 60(SI), AX
	MOVD AX, X4
	PSLLDQ $12, X4
	POR X4, X3
	// Horizontal butterfly (lane-parallel, no shuffles).
	MOVAPS X0, X4
	PADDL X2, X4
	MOVAPS X0, X5
	PSUBL X2, X5
	MOVAPS X1, X6
	PSRAL $1, X6
	PSUBL X3, X6
	MOVAPS X3, X7
	PSRAL $1, X7
	PADDL X1, X7
	MOVAPS X4, X0
	PADDL X7, X0
	MOVAPS X5, X1
	PADDL X6, X1
	MOVAPS X5, X2
	PSUBL X6, X2
	MOVAPS X4, X3
	PSUBL X7, X3
	// Transpose E columns (X0..X3) into e rows (X0..X3).
	MOVAPS X0, X4
	UNPCKLPS X2, X4
	MOVAPS X1, X5
	UNPCKLPS X3, X5
	MOVAPS X0, X6
	UNPCKHPS X2, X6
	MOVAPS X1, X7
	UNPCKHPS X3, X7
	MOVAPS X4, X0
	UNPCKLPS X5, X0
	MOVAPS X4, X1
	UNPCKHPS X5, X1
	MOVAPS X6, X2
	UNPCKLPS X7, X2
	MOVAPS X6, X3
	UNPCKHPS X7, X3
	// Vertical butterfly (rows, lane-parallel).
	MOVAPS X0, X4
	PADDL X2, X4
	MOVAPS X0, X5
	PSUBL X2, X5
	MOVAPS X1, X6
	PSRAL $1, X6
	PSUBL X3, X6
	MOVAPS X3, X7
	PSRAL $1, X7
	PADDL X1, X7
	MOVAPS X4, X0
	PADDL X7, X0
	MOVAPS X5, X1
	PADDL X6, X1
	MOVAPS X5, X2
	PSUBL X6, X2
	MOVAPS X4, X3
	PSUBL X7, X3
	// (f+32)>>6, then raster stores.
	PADDL ·itrans4x4K32(SB), X0
	PADDL ·itrans4x4K32(SB), X1
	PADDL ·itrans4x4K32(SB), X2
	PADDL ·itrans4x4K32(SB), X3
	PSRAL $6, X0
	PSRAL $6, X1
	PSRAL $6, X2
	PSRAL $6, X3
	MOVOU X0, 0(DI)
	MOVOU X1, 16(DI)
	MOVOU X2, 32(DI)
	MOVOU X3, 48(DI)
	RET
