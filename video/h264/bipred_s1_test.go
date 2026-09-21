package h264

// S1b-A2 gate: bipred average equals the scalar loop bit for bit.
// Oracle is the pre-S1b formula (w*a+(64-w)*b+32)>>6 with per-byte
// math (no new vectors here, VR2 exact pins pixels).

import (
	"testing"
)

// bipredScalarOracle runs the pre-S1b scalar verbatim.
func bipredScalarOracle(a, b []byte, w int32) []byte {
	out := make([]byte, len(a))
	w1 := 64 - w
	for i := range a {
		out[i] = uint8((w*int32(a[i]) + w1*int32(b[i]) + 32) >> 6)
	}
	return out
}

func TestS1BipredMatchesScalar(t *testing.T) {
	weights := []int32{0, 1, 16, 31, 32, 33, 48, 63, 64}
	sizes := []int{1, 7, 8, 9, 15, 16, 17, 64, 256}
	for _, w := range weights {
		for _, n := range sizes {
			a := make([]byte, n)
			b := make([]byte, n)
			for i := range a {
				a[i] = uint8((i*7 + int(w)*3) & 0xFF)
				b[i] = uint8((i*13 + 41) & 0xFF)
			}
			got := append([]byte(nil), a...)
			bipredAvg(got, b, w)
			want := bipredScalarOracle(a, b, w)
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("w=%d n=%d byte %d got=%d want=%d", w, n, i, got[i], want[i])
				}
			}
		}
	}
}

func TestS1BipredZeroAlloc(t *testing.T) {
	a := make([]byte, 256)
	b := make([]byte, 256)
	for i := range a {
		a[i] = uint8(i)
		b[i] = uint8(255 - i)
	}
	n := testing.AllocsPerRun(100, func() {
		bipredAvg(a, b, 32)
	})
	if n != 0 {
		t.Fatalf("bipred 256 allocs = %v, want 0", n)
	}
}
