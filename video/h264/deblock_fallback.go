//go:build !amd64

package h264

// deblockLumaArch reports unavailable off amd64 (scalar留守 runs).
// arm64 NEON随后 (same split as color convert and qpel); i386 and
// 32-bit arm stay on the scalar path by design (see plan §12 S1).
func deblockLumaArch(p []uint8, stride, ex, ey int, vertical bool, bS, alpha, beta, tc int) bool {
	return false
}
