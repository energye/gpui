package h264

import "os"

// S1 deblock fast path, step D1 (edge dispatch, scalar留守).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/h264_loopfilter.c C edge driver (per-edge strength then
//   filter, our DeblockPicture loop in filter.go),
//   libavcodec/x86/h264dsp_init.c:216-252 function-pointer table (h/v by
//   luma/chroma by normal/intra split, our deblockLumaArch per-arch
//   dispatch plus the weak/strong kernel select),
//   libavcodec/x86/h264_deblock.asm SIMD instance (our deblock_amd64.s
//   V/H x weak/strong 4-line kernels, direct picture load/store;
//   ffmpeg transposes one direction, we split V/H so each kernel reads
//   its own direction with no Go copy).
// Difference: ffmpeg pads reference edges once (emulated-edge buffers);
// our scalar path already indexes every non-skipped luma edge without
// bounds checks (picture-border MB edges skip in DeblockPicture, closed
// alpha/beta gates skip in the caller), so each kernel takes every
// handed luma edge with bS > 0, no interior/edge split.
// Ours: filter.go filterLumaEdge (留守) + this file (dispatch); amd64
// code lives in deblock_amd64.go with kernels in deblock_amd64.s; other
// archs use deblock_fallback.go. Output is bit-identical to the scalar
//留守 (deblock_s1_test.go pins all bS classes in both directions). Set
// GPUI_SCALAR_CONVERT=1 to force scalar (same switch as color convert
// and qpel).

// deblockScalarForced skips the vector kernel, read once from the
// environment so the hot path pays nothing.
var deblockScalarForced = func() bool {
	switch os.Getenv("GPUI_SCALAR_CONVERT") {
	case "1", "true", "TRUE":
		return true
	}
	return false
}()

// deblockLumaFast filters one 4-line luma edge through the arch kernel.
// It reports false (caller falls back to the scalar留守) for
// forced-scalar mode, zero strength, and archs without a kernel.
// Chroma edges stay scalar (2 lines are too narrow to win back the
// gather cost).
// Tried and withdrawn (2026-09-15, same machine): a lane-buffer SoA
// version (Go gathered 8 taps x 4 lines into int16 lanes, one shared
// kernel for both directions, Go scattered back) measured ~0.5-0.6x
// vs scalar per edge (synthetic 720p frame 17.2ms -> 27.4ms): per-byte
// Go loads/stores with bounds checks plus the extra call cost more
// than the 4-wide kernel saves on this gate-heavy filter. Rewritten
// direct (kernels load/store the picture themselves, V/H split).
func deblockLumaFast(p []uint8, stride, ex, ey int, vertical bool, bS, alpha, beta, tc int) bool {
	if deblockScalarForced || bS <= 0 {
		return false
	}
	return deblockLumaArch(p, stride, ex, ey, vertical, bS, alpha, beta, tc)
}
