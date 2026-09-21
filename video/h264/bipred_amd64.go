//go:build amd64

package h264

import "unsafe"

//go:noescape
func bipredRow8Arch(dst, src1 unsafe.Pointer, w32, w132 int)

//go:noescape
func bipredAvg16Arch(dst, src1 unsafe.Pointer)

func bipredRow8(dst, src1 unsafe.Pointer, w32, w132 int) {
	bipredRow8Arch(dst, src1, w32, w132)
}

// S1b-K amd64 equal-weight lane: w==32 runs 16 bytes per Go call
// through PAVGB (one instruction, same formula); shorter tails keep
// the 8-wide kernel + scalar below, bit-identical.
func bipredAvg16(dst, src1 unsafe.Pointer) {
	bipredAvg16Arch(dst, src1)
}

// S1b-A2 amd64 bipred block path (SSE2 row kernel + scalar tail).

// bipredBlock is the amd64 entry: average n bytes (n>0). True means done.
func bipredBlock(dst, s1 []byte, n int, w int32) bool {
	if n <= 0 || n != len(s1) || n > len(dst) {
		return false
	}
	if w == 32 {
		bulk := n &^ 15
		for off := 0; off < bulk; off += 16 {
			bipredAvg16(
				unsafe.Pointer(&dst[off]),
				unsafe.Pointer(&s1[off]),
			)
		}
		// Tail under 16 bytes: existing 8-wide kernel + scalar.
		for off := bulk; off < n; {
			if n-off >= 8 {
				bipredRow8(
					unsafe.Pointer(&dst[off]),
					unsafe.Pointer(&s1[off]),
					32|(32<<16), 32|(32<<16),
				)
				off += 8
				continue
			}
			dst[off] = uint8((int32(dst[off]) + int32(s1[off]) + 1) >> 1)
			off++
		}
		return true
	}
	w1 := 64 - w
	w32 := int(w) | (int(w) << 16)
	w132 := int(w1) | (int(w1) << 16)
	bulk := n &^ 7
	for off := 0; off < bulk; off += 8 {
		bipredRow8(
			unsafe.Pointer(&dst[off]),
			unsafe.Pointer(&s1[off]),
			w32, w132,
		)
	}
	// Tail with the scalar, same formula.
	for i := bulk; i < n; i++ {
		dst[i] = uint8((w*int32(dst[i]) + w1*int32(s1[i]) + 32) >> 6)
	}
	return true
}
