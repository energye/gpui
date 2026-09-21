//go:build arm64

package h264

// S1b-A1 arm64 chroma block path (interior scalar fast, NEON待续).
// Matches chromaInteriorScalar bit for bit; interior-only (caller
// guarantees all four taps inside, no per-tap clipping). Gives the
// hoisted-weights win on arm64 today; the NEON row kernel plugs into
// chromaBlock here without touching the dispatch or gates (same staging
// as S1 qpel/deblock: amd64 SIMD first, arm64 NEON follows).
func chromaBlock(out, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
	if cw <= 0 || ch <= 0 || cw > 16 || ch > 16 {
		return false
	}
	chromaInteriorScalar(out, plane, stride, ix0, iy0, cw, ch, fx, fy)
	return true
}
