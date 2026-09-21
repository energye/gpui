//go:build (!amd64 && !arm64)

package h264

// bipredBlock on archs without a SIMD kernel: scalar, bit-identical.
func bipredBlock(dst, s1 []byte, n int, w int32) bool {
	if n <= 0 || n != len(s1) || n > len(dst) {
		return false
	}
	w1 := 64 - w
	for i := 0; i < n; i++ {
		dst[i] = uint8((w*int32(dst[i]) + w1*int32(s1[i]) + 32) >> 6)
	}
	return true
}
