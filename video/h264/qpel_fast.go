package h264

import "os"

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
