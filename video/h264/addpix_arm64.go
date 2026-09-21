//go:build arm64

package h264

// S1b-A3 arm64 residual-add block path (scalar fast, NEON待续).
// Bit-identical with the scalar; same staging as S1 qpel/deblock.
func addResidArch(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) bool {
	if w != 4 && w != 8 {
		return false
	}
	addResidScalar(pic, stride, bx, by, pred, predStride, res, w, h)
	return true
}
