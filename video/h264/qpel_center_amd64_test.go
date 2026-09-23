//go:build amd64

package h264

import (
	"math/rand"
	"testing"
)

// S1-C gate (amd64-only file: centerVec4/centerRow4 live in
// qpel_amd64.go, amd64-tagged; other archs run scalar centerAt pinned
// by TestS1QpelDispatchMatchesScalar): the 4-wide center kernel equals
// 4x scalar centerAt bit for bit on the FULL int16 input range —
// 32-bit math never wraps (|s| < 900k << 2^31 for any int16 taps),
// unlike the 16-lane roundHalf kernel next door (see its contract
// note). Covers live hor-sum band, extremes, impulses, and every
// dy/dx position of a max-size window.
func TestS1CenterVecMatchesScalar(t *testing.T) {
	// Live band: hor sums of bytes (±10710) plus guard.
	r := rand.New(rand.NewSource(20260922))
	var jt [qpelMaxH + 5][qpelMaxW]int16
	for trial := 0; trial < 40; trial++ {
		for y := range jt {
			for x := range jt[y] {
				switch trial % 4 {
				case 0:
					jt[y][x] = int16(r.Intn(21421) - 10710)
				case 1:
					jt[y][x] = int16(int32(r.Uint32()))
				case 2:
					if r.Intn(2) == 0 {
						jt[y][x] = -32768
					} else {
						jt[y][x] = 32767
					}
				default:
					jt[y][x] = 0
				}
			}
		}
		for dy := 0; dy+5 < len(jt); dy++ {
			for dx := 0; dx+4 <= qpelMaxW; dx += 4 {
				var got [4]uint8
				centerRow4(got[:], &jt, dy, dx)
				for i := 0; i < 4; i++ {
					want := uint8(centerAt(&jt, dy, dx+i))
					if got[i] != want {
						t.Fatalf("trial=%d dy=%d dx=%d lane=%d kernel=%d scalar=%d",
							trial, dy, dx, i, got[i], want)
					}
				}
			}
		}
	}
	// Impulse at every tap position (6 rows x 16 cols) with killer
	// magnitudes (max int16: exercises sign extension per lane).
	for rr := 0; rr < 6; rr++ {
		for cc := 0; cc < qpelMaxW; cc++ {
			var jt2 [qpelMaxH + 5][qpelMaxW]int16
			jt2[rr][cc] = 32767
			jt2[(rr+3)%(qpelMaxH+5)][(cc+7)%qpelMaxW] = -32768
			for dy := 0; dy <= 2; dy++ {
				var got [4]uint8
				dx := (cc / 4) * 4
				if dx+4 > qpelMaxW {
					dx = qpelMaxW - 4
				}
				centerRow4(got[:], &jt2, dy, dx)
				for i := 0; i < 4; i++ {
					want := uint8(centerAt(&jt2, dy, dx+i))
					if got[i] != want {
						t.Fatalf("impulse r=%d c=%d dy=%d dx=%d lane=%d kernel=%d scalar=%d",
							rr, cc, dy, dx, i, got[i], want)
					}
				}
			}
		}
	}
	// Zero-alloc (plain arithmetic + one asm call, no heap).
	var jt3 [qpelMaxH + 5][qpelMaxW]int16
	var o3 [4]uint8
	if n := testing.AllocsPerRun(20, func() {
		centerRow4(o3[:], &jt3, 3, 4)
	}); n != 0 {
		t.Fatalf("centerRow4 allocs = %v want 0", n)
	}
}
