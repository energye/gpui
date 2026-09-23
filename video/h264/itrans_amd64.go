//go:build amd64 && !purego

package h264

import "unsafe"

// S1-T amd64 8x8 inverse transform path. Kernels live in
// itrans_amd64.s; orchestration (two temps on the stack, zero-alloc)
// is here. Output is bit-identical to itrans8x8Core (the gate test
// pins the full int32 range, all-QP dequant magnitudes, zero and
// DC-only blocks).

//go:noescape
func itransColPass1(c, t1 unsafe.Pointer)

//go:noescape
func itransRowPass2(t1, out unsafe.Pointer)

//go:noescape
func itrans4x4Block(c, out unsafe.Pointer)

// itrans8x8Arch runs the whole 8x8 core through two kernels: column
// pass over c into t1 (transposed MIX rows), then the row pass over
// t1 into out (spec transposed final layout). t2 stays in the
// signature as scratch the caller owns (kept so the zero-alloc test
// pins the full path).
func itrans8x8Arch(c, t1, t2, out *[64]int32) {
	_ = t2
	itransColPass1(unsafe.Pointer(&c[0]), unsafe.Pointer(&t1[0]))
	itransRowPass2(unsafe.Pointer(&t1[0]), unsafe.Pointer(&out[0]))
}

// itrans4x4Arch runs the whole 4x4 core in one kernel call (gather,
// both butterflies, transpose folded between passes, round, stores).
// No memory temp: everything rides XMM. amd64 takes the kernel;
// arm64/fallback run the scalar core (same output, only speed
// differs) — see itrans_arm64.go / itrans_fallback.go.
func itrans4x4Arch(c, out *[16]int32) {
	itrans4x4Block(unsafe.Pointer(&c[0]), unsafe.Pointer(&out[0]))
}
