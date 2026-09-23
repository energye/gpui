//go:build !amd64 && !arm64

package h264

// chromaBlock reports interior scalar fast on archs without a SIMD
// kernel (i386 and 32-bit arm stay on the interior fast path by design:
// bit-identical on interior, no per-tap clipping; see plan §12 S1).
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

// chromaDebEdge16 stays scalar off amd64/arm64 (scalar留守 runs).
func chromaDebEdge16(p []uint8, stride, cx, cy int, vertical bool, bS, tc [4]int, alpha, beta int) bool {
	return false
}
