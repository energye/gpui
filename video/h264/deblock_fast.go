package h264

import (
	"os"
	"sync/atomic"
)

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

// edge16Kind classifies one 16-line luma edge for the whole-edge
// dispatch. Single source for the taken matrix: the amd64 dispatch
// (deblock_amd64.go) switches on this, and the filter.go call site
// recounts with it when deblockStatsOn. Keep both callers on this
// helper; a second inline copy will drift.
type edge16Kind int

const (
	edge16Zero   edge16Kind = iota // all four bS == 0: nothing would filter
	edge16Gated                    // alpha/beta closed: every line gates out
	edge16Weak                     // all bS < 4 with some filtering: one weak call
	edge16Strong                   // all bS == 4: one strong call
	edge16Mixed                    // weak/strong blend: per-segment fallback
)

func classifyEdge16(bS [4]int, alpha, beta int) edge16Kind {
	any := false
	allStrong := true
	allWeakOrSkip := true
	for _, v := range bS {
		if v != 4 {
			allStrong = false
		}
		if v >= 4 {
			allWeakOrSkip = false
		}
		if v > 0 {
			any = true
		}
	}
	if !any {
		return edge16Zero
	}
	if alpha == 0 || beta == 0 {
		return edge16Gated
	}
	if allStrong {
		return edge16Strong
	}
	if allWeakOrSkip {
		return edge16Weak
	}
	return edge16Mixed
}

// Whole-edge hit-rate census (step C1: measure before cutting further).
// deblockStatsOn is test-only: the filter.go call site bumps one
// counter per edge while on. Off (production + timing runs) the hot
// path pays a single predictable branch per edge and nothing else.
// Counters are atomic so S2-parallel decodes stay race-clean.
var deblockStatsOn bool

var edge16Counters struct {
	total         atomic.Uint64
	zero          atomic.Uint64
	gated         atomic.Uint64
	weakTaken     atomic.Uint64
	strongTaken   atomic.Uint64
	mixedFallback atomic.Uint64
	otherFallback atomic.Uint64
}

// edge16Note records one whole-edge dispatch: its class plus whether
// the single call ran (taken) or the per-segment path ran (fallback).
// On amd64 unforced, weak/strong always take and mixed always falls
// back, so otherFallback > 0 flags forced-scalar or non-amd64 runs.
func edge16Note(kind edge16Kind, taken bool) {
	edge16Counters.total.Add(1)
	switch kind {
	case edge16Zero:
		edge16Counters.zero.Add(1)
	case edge16Gated:
		edge16Counters.gated.Add(1)
	case edge16Weak:
		if taken {
			edge16Counters.weakTaken.Add(1)
		} else {
			edge16Counters.otherFallback.Add(1)
		}
	case edge16Strong:
		if taken {
			edge16Counters.strongTaken.Add(1)
		} else {
			edge16Counters.otherFallback.Add(1)
		}
	default:
		edge16Counters.mixedFallback.Add(1)
	}
}

type edge16Stats struct {
	total, zero, gated, weakTaken, strongTaken, mixedFallback, otherFallback uint64
}

func edge16Snapshot() edge16Stats {
	return edge16Stats{
		total:         edge16Counters.total.Load(),
		zero:          edge16Counters.zero.Load(),
		gated:         edge16Counters.gated.Load(),
		weakTaken:     edge16Counters.weakTaken.Load(),
		strongTaken:   edge16Counters.strongTaken.Load(),
		mixedFallback: edge16Counters.mixedFallback.Load(),
		otherFallback: edge16Counters.otherFallback.Load(),
	}
}

func edge16Reset() {
	edge16Counters.total.Store(0)
	edge16Counters.zero.Store(0)
	edge16Counters.gated.Store(0)
	edge16Counters.weakTaken.Store(0)
	edge16Counters.strongTaken.Store(0)
	edge16Counters.mixedFallback.Store(0)
	edge16Counters.otherFallback.Store(0)
}
