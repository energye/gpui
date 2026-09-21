//go:build (!amd64 && !arm64)

package h264

// chromaBlock reports interior scalar fast on archs without a SIMD
// kernel (i386 and 32-bit arm stay on the interior fast path by design:
// bit-identical on interior, no per-tap clipping; see plan §12 S1).
func chromaBlock(out, plane []byte, stride, ix0, iy0, cw, ch, fx, fy int) bool {
	if cw <= 0 || ch <= 0 || cw > 16 || ch > 16 {
		return false
	}
	chromaInteriorScalar(out, plane, stride, ix0, iy0, cw, ch, fx, fy)
	return true
}
