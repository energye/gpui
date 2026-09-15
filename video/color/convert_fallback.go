//go:build !amd64 && !arm64

package color

// convertBandSIMD reports unavailable off amd64/arm64 (scalar留守 runs).
// i386 and 32-bit arm stay on the scalar path by design (see plan §12 S1).
func convertBandSIMD(dst, y, cb, cr []byte, w int, t coeffs, ys, ye int) bool {
	return false
}
