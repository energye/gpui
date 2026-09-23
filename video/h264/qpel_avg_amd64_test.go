//go:build amd64

package h264

import (
	"testing"
)

// S1-Q gate: the PAVGB avg row path equals the avg2 scalar loop bit
// for bit on every byte pair (the +1 rounding is where hand-rolled
// shifts usually drift: (0,0)->0, (0,1)->1, (255,255)->255,
// (255,0)->128). Covers widths 4/8/16 (live luma partitions) plus
// odd widths through the scalar tail, and the 4-byte store width
// (sentinel right neighbours — the centerVec4 MOVQ lesson, 2026-09-22).
func TestS1AvgRowMatchesScalar(t *testing.T) {
	pairs := [][2]uint8{{0, 0}, {0, 1}, {1, 0}, {255, 255}, {255, 0}, {0, 255}, {128, 127}, {127, 128}, {254, 255}, {1, 1}}
	for _, n := range []int{4, 8, 12, 16, 17} {
		a := make([]byte, n)
		b := make([]byte, n)
		for i := range a {
			a[i] = pairs[i%len(pairs)][0]
			b[i] = pairs[(i*3+1)%len(pairs)][1]
		}
		got := make([]byte, n)
		want := make([]byte, n)
		avgRowInto(got, a, b)
		for i := range want {
			want[i] = uint8(avg2(int(a[i]), int(b[i])))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("n=%d byte %d kernel=%d scalar=%d (a=%d b=%d)",
					n, i, got[i], want[i], a[i], b[i])
			}
		}
	}
	// Exhaustive all-pairs on 16 bytes (every rounding direction).
	a := make([]byte, 16)
	b := make([]byte, 16)
	for x := 0; x < 256; x++ {
		for i := range a {
			a[i] = uint8(x)
			b[i] = uint8((x*37 + i*11) & 0xFF)
		}
		got := make([]byte, 16)
		avgRowInto(got, a, b)
		for i := range got {
			if want := uint8(avg2(x, int(b[i]))); got[i] != want {
				t.Fatalf("x=%d byte %d kernel=%d scalar=%d", x, i, got[i], want)
			}
		}
	}
	// Store width: 4-wide into a sentinel buffer must not touch byte 4..7.
	sentinel := make([]byte, 8)
	for i := range sentinel {
		sentinel[i] = 0xA5
	}
	avgRowInto(sentinel[:4], []byte{10, 20, 30, 40}, []byte{11, 21, 31, 41})
	for i := 4; i < 8; i++ {
		if sentinel[i] != 0xA5 {
			t.Fatalf("byte %d overwritten: %#x (store wider than 4)", i, sentinel[i])
		}
	}
	// Zero-alloc on every live width.
	for _, n := range []int{4, 8, 16} {
		dst := make([]byte, n)
		as := make([]byte, n)
		bs := make([]byte, n)
		if m := testing.AllocsPerRun(100, func() {
			avgRowInto(dst, as, bs)
		}); m != 0 {
			t.Fatalf("n=%d avg allocs = %v want 0", n, m)
		}
	}
}
