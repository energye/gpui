//go:build !amd64 && !arm64

package h264

// qpelBlock reports unavailable off amd64/arm64 (scalar留守 runs).
// amd64 has its kernel (qpel_amd64.go/s), arm64 has its NEON kernel
// (qpel_arm64.go/s, wired via qpelFast below); i386 and 32-bit arm
// stay on the scalar path by design (see plan §12 S1).
func qpelBlock(dst, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	return false
}

// copyBlockInto copies a w*h block (scalar留守 off amd64/arm64;
// amd64 runs the copyBlock kernel): same bytes as the per-row copy
// builtin, bit for bit.
func copyBlockInto(dst []byte, dstStride int, src []byte, srcStride int, w, h int) {
	for dy := 0; dy < h; dy++ {
		copy(dst[dy*dstStride:dy*dstStride+w], src[dy*srcStride:dy*srcStride+w])
	}
}

// qpelBlockInto reports unavailable off amd64/arm64 (the strided
// scalar留守 in inter.go runs instead).
func qpelBlockInto(dst []byte, dstStride int, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	return false
}
