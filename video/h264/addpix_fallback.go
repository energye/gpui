//go:build (!amd64 && !arm64)

package h264

// addResidArch on archs without a SIMD kernel: scalar, bit-identical.
func addResidArch(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) bool {
	if w != 4 && w != 8 {
		return false
	}
	addResidScalar(pic, stride, bx, by, pred, predStride, res, w, h)
	return true
}
