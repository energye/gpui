//go:build !amd64

package h264

// qpelBlock reports unavailable off amd64 (scalar留守 runs).
// arm64 NEON随后 (same split as color convert); i386 and 32-bit arm stay
// on the scalar path by design (see plan §12 S1).
func qpelBlock(dst, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	return false
}
