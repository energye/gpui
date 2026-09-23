//go:build !amd64 && !arm64

package h264

// deblockLumaArch reports unavailable off amd64/arm64 (scalar留守 runs).
// amd64 has its kernels (deblock_amd64.go/s), arm64 has its NEON
// kernels (deblock_arm64.go/s: V/H x weak/strong four kernels); i386 and
// 32-bit arm stay on the scalar path by design (see plan §12 S1).
func deblockLumaArch(p []uint8, stride, ex, ey int, vertical bool, bS, alpha, beta, tc int) bool {
	return false
}

// deblockLumaEdge16 stays scalar off amd64/arm64 (scalar留守 runs).
func deblockLumaEdge16(p []uint8, stride, ex, ey int, vertical bool, bS, tc [4]int, alpha, beta int) bool {
	return false
}
