//go:build arm64 && !purego

package h264

// itrans8x8Arch stays on the scalar core on arm64 (the NEON twin of
// the column/transpose kernels is open work; output is identical via
// itrans8x8Core, only speed differs).
func itrans8x8Arch(c, t1, t2, out *[64]int32) {
	*out = itrans8x8Core(*c)
}

// itrans4x4Arch stays on the scalar core on arm64 (the NEON twin of
// the 4x4 block kernel is open work; output is identical via
// itrans4x4CoreScalar, only speed differs).
func itrans4x4Arch(c, out *[16]int32) {
	*out = itrans4x4CoreScalar(*c)
}
