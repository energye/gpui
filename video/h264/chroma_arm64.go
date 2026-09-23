//go:build arm64

package h264

// S1b-A1 arm64 chroma block path (interior scalar fast, NEON待续).
// Matches chromaInteriorScalar bit for bit; interior-only (caller
// guarantees all four taps inside, no per-tap clipping). Gives the
// hoisted-weights win on arm64 today; the NEON row kernel plugs into
// chromaBlock here without touching the dispatch or gates (same staging
// as S1 qpel/deblock: amd64 SIMD first, arm64 NEON follows).
func chromaBlock(out, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
	return chromaBlockInto(out, cw, plane, stride, ix0, iy0, cw, ch, fx, fy)
}

// chromaBlockInto is the strided entry (S1-W direct-write): same gate,
// output rows stride out.
func chromaBlockInto(out []byte, dstStride int, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
	if cw <= 0 || ch <= 0 || cw > 16 || ch > 16 {
		return false
	}
	chromaInteriorScalarStride(out, dstStride, plane, stride, ix0, iy0, cw, ch, fx, fy)
	return true
}

// chromaDebEdge16 stays on the per-segment scalar path on arm64 (the
// NEON 8-line twin of chromaDebVWeak8/chromaDebHWeak8 is open work;
// output is identical via filterChromaEdge, only the call count
// differs).
func chromaDebEdge16(p []uint8, stride, cx, cy int, vertical bool, bS, tc [4]int, alpha, beta int) bool {
	return false
}
