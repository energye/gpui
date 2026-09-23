package h264

import "os"

// S1-T 8x8 inverse transform dispatch. Ours: transform.go
// ITransform8x8Scaled (scalar留守); amd64 kernels live in
// itrans_amd64.go/itrans_amd64.s; other archs use the scalar core
// (itrans_arm64.go/itrans_fallback.go). Output is bit-identical to
// itrans8x8Core (itrans_s1_test.go pins it). Set GPUI_SCALAR_CONVERT=1
// to force scalar (same switch as deblock/qpel/convert).

// itransScalarForced skips the vector kernel, read once from the
// environment so the hot path pays nothing.
var itransScalarForced = func() bool {
	switch os.Getenv("GPUI_SCALAR_CONVERT") {
	case "1", "true", "TRUE":
		return true
	}
	return false
}()

// itrans8x8Fast runs the 8x8 core through the arch entry. It reports
// false (caller falls back to itrans8x8Core) only for forced-scalar
// mode: on amd64 the entry runs the kernels, elsewhere it runs the
// scalar core itself (same output, no double work). All stack buffers
// (no heap).
func itrans8x8Fast(c [64]int32) ([64]int32, bool) {
	if itransScalarForced {
		return [64]int32{}, false
	}
	var t1, t2, out [64]int32
	itrans8x8Arch(&c, &t1, &t2, &out)
	return out, true
}
