package h264

import (
	"os"
	"sync/atomic"
)

// S1 qpel fast path, step I1 (block dispatch, scalar留守).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/h264qpel.c:59-101 function-pointer table (our qpelBlock
//   per-arch dispatch), libavcodec/h264qpel_template.c:317-460 H264_QPEL
//   macro family (horizontal pass into temp, then vertical/avg combine -
//   our qpel_amd64.go block skeleton), libavcodec/x86/h264_qpel_8bit.asm
//   (SIMD instance, our qpel_amd64.s row kernel).
// Difference: ffmpeg pads reference edges once (emulated-edge buffers);
// we instead take the fast path only for interior blocks and fall back
// to the scalar留守 for edge-touching blocks, so no padding bookkeeping.
// Ours: inter.go predictLumaBlockScalar (留守) + this file (dispatch);
// amd64 block code lives in qpel_amd64.go with its kernel in
// qpel_amd64.s; other archs use qpel_fallback.go. Output is bit-identical
// to the scalar留守 (qpel_s1_test.go pins all 16 fracs). Set
// GPUI_SCALAR_CONVERT=1 to force scalar (same switch as color convert).

// qpelScalarForced skips the vector kernel, read once from the
// environment so the hot path pays nothing.
var qpelScalarForced = func() bool {
	switch os.Getenv("GPUI_SCALAR_CONVERT") {
	case "1", "true", "TRUE":
		return true
	}
	return false
}()

// qpelFast runs one sub-pel partition through the arch block path.
// It reports false (caller falls back to the scalar留守) for integer
// motion (handled by the row-copy path, edge-clipped ints included),
// forced-scalar mode, edge-touching blocks, and archs without a kernel.
func qpelFast(plane []byte, w, h, px, py, bw, bh int, mx, my int16, out []byte) bool {
	if qpelScalarForced {
		return false
	}
	qx := px*4 + int(mx)
	qy := py*4 + int(my)
	// Floor divide by 4 for possibly negative vectors (same as留守).
	ix := qx >> 2
	iy := qy >> 2
	fx := qx - (ix << 2)
	fy := qy - (iy << 2)
	if fx == 0 && fy == 0 {
		return false
	}
	return qpelBlock(out, plane, w, h, ix, iy, bw, bh, fx, fy)
}

// qpelFastInto is the strided entry (S1-W direct-write): same gate as
// qpelFast, output rows land at dst[dy*dstStride:dy*dstStride+bw].
// False means "run the strided scalar留守".
func qpelFastInto(plane []byte, w, h, px, py, bw, bh int, mx, my int16, dst []byte, dstStride int) bool {
	if qpelScalarForced {
		return false
	}
	qx := px*4 + int(mx)
	qy := py*4 + int(my)
	ix := qx >> 2
	iy := qy >> 2
	fx := qx - (ix << 2)
	fy := qy - (iy << 2)
	if fx == 0 && fy == 0 {
		return false
	}
	return qpelBlockInto(dst, dstStride, plane, w, h, ix, iy, bw, bh, fx, fy)
}

// Luma prediction census (step C4: measure where sub-pel costs land).
// predictStatsOn is test-only: predictLumaBlock bumps one counter per
// block while on. Off (production + timing runs) the hot path pays a
// single predictable branch per block and nothing else. Counters are
// atomic so S2-parallel decodes stay race-clean.
//
// Layout note: the counters, snapshot and classifier live here in
// qpel_fast.go (untagged, every arch) — NOT in the amd64-only
// qpel_amd64.go — so the arm64 and fallback builds see them too.
// The interior size cap is mirrored per arch (qpelMaxW/H on amd64,
// qpelArmMaxW/H on arm64, scalar-only elsewhere); the classifier
// switches on GOARCH instead of importing an arch-tagged const.
var predictStatsOn bool

var predictCounters struct {
	total     atomic.Uint64
	intCopy   atomic.Uint64 // integer motion row-copy (no filter)
	qpelTaken atomic.Uint64 // sub-pel interior: arch block path
	qpelEdge  atomic.Uint64 // sub-pel edge-touching: scalar留守
	horOnly   atomic.Uint64 // sub-pel interior, fy==0 (one horizontal pass)
	verOnly   atomic.Uint64 // sub-pel interior, fx==0 (one vertical pass)
	general   atomic.Uint64 // sub-pel interior, both fracs (two passes + combine)
	center22  atomic.Uint64 // general, pure center (2,2): temp + cascade only
	halfCentr atomic.Uint64 // general, one frac == 2: half pass + temp + cascade
	corner111 atomic.Uint64 // general, fracs 1/3 x 1/3: two half passes + avg
}

type predictStats struct {
	total, intCopy, qpelTaken, qpelEdge uint64
	horOnly, verOnly, general           uint64
	center22, halfCentr, corner111      uint64
}

func predictSnapshot() predictStats {
	return predictStats{
		total:     predictCounters.total.Load(),
		intCopy:   predictCounters.intCopy.Load(),
		qpelTaken: predictCounters.qpelTaken.Load(),
		qpelEdge:  predictCounters.qpelEdge.Load(),
		horOnly:   predictCounters.horOnly.Load(),
		verOnly:   predictCounters.verOnly.Load(),
		general:   predictCounters.general.Load(),
		center22:  predictCounters.center22.Load(),
		halfCentr: predictCounters.halfCentr.Load(),
		corner111: predictCounters.corner111.Load(),
	}
}

func predictReset() {
	predictCounters.total.Store(0)
	predictCounters.intCopy.Store(0)
	predictCounters.qpelTaken.Store(0)
	predictCounters.qpelEdge.Store(0)
	predictCounters.horOnly.Store(0)
	predictCounters.verOnly.Store(0)
	predictCounters.general.Store(0)
	predictCounters.center22.Store(0)
	predictCounters.halfCentr.Store(0)
	predictCounters.corner111.Store(0)
}

// notePredictClass records one luma prediction block: integer row-copy
// vs sub-pel interior (arch path) vs sub-pel edge-touching (scalar).
// Mirrors the predictLumaBlock/qpelFast/qpelBlock decision chain
// without running it: integer motion counts as intCopy only when the
// source rect is fully inside the frame (same condition as the memcpy
// branch); sub-pel counts as taken only inside the interior margin
// (same ix/iy window as qpelBlock) and inside the block-size cap —
// oversized partitions fall back to scalar and count as edge here.
func notePredictClass(w, h, px, py, bw, bh int, mx, my int16) {
	predictCounters.total.Add(1)
	qx := px*4 + int(mx)
	qy := py*4 + int(my)
	ix := qx >> 2
	iy := qy >> 2
	fx := qx - (ix << 2)
	fy := qy - (iy << 2)
	if fx == 0 && fy == 0 {
		sx := px + int(mx>>2)
		sy := py + int(my>>2)
		if sx >= 0 && sy >= 0 && sx+bw <= w && sy+bh <= h {
			predictCounters.intCopy.Add(1)
			return
		}
		predictCounters.qpelEdge.Add(1)
		return
	}
	maxW, maxH := 16, 16
	if bw <= 0 || bh <= 0 || bw > maxW || bh > maxH ||
		ix < 2 || iy < 2 || ix+bw+4 > w || iy+bh+4 > h {
		predictCounters.qpelEdge.Add(1)
		return
	}
	predictCounters.qpelTaken.Add(1)
	// Frac-family split (which passes the block runs): horOnly needs
	// one horizontal pass, verOnly one vertical pass, general both
	// passes plus the combine. The general share splits three ways:
	// pure-center (2,2) runs temp + cascade only; half-center (one
	// frac == 2) runs one half pass + temp + cascade; corner (1/3 x
	// 1/3) runs two half passes + avg. Temp-users (center22 +
	// halfCentr) are the fused-kernel target: one widened 32-bit
	// vertical combine replaces their temp fill + cascade.
	switch {
	case fy == 0:
		predictCounters.horOnly.Add(1)
	case fx == 0:
		predictCounters.verOnly.Add(1)
	case fx == 2 && fy == 2:
		predictCounters.general.Add(1)
		predictCounters.center22.Add(1)
	case fx == 2 || fy == 2:
		predictCounters.general.Add(1)
		predictCounters.halfCentr.Add(1)
	default:
		predictCounters.general.Add(1)
		predictCounters.corner111.Add(1)
	}
}
