//go:build amd64

package h264

import (
	"testing"
	"unsafe"
)

// S1-R gate (amd64-only file: qpelRoundHalf8/roundHalfRow live in
// qpel_amd64.go, amd64-tagged; other archs run scalar combines pinned
// by TestS1QpelDispatchMatchesScalar): the round-half vector kernel
// equals the scalar roundHalf bit for bit on its contract band
// (s+16 in int16, s <= 32751): the live hor/ver sum band
// (-2550..10710), guard bands around it, and odd widths for the tail.
// Full-int16 sweep is NOT pinned on purpose: s >= 32752 wraps in PADDW
// where Go int math does not (measured kernel 0 vs scalar 255 at
// s=32752) — unreachable, since 6-tap sums of bytes never exceed 10710
// (see qpelHorSums note).
func TestS1RoundHalfMatchesScalar(t *testing.T) {
	// Contract band sweep in 8-lane chunks (bulk path every chunk):
	// -2600..10800 covers live sums plus guard on both sides.
	var sums [8]int16
	var kd, sd [8]uint8
	for base := -2608; base <= 10799; base += 8 {
		for i := 0; i < 8; i++ {
			sums[i] = int16(base + i)
		}
		qpelRoundHalf8(unsafe.Pointer(&kd[0]), unsafe.Pointer(&sums[0]))
		for i := 0; i < 8; i++ {
			sd[i] = roundHalf(sums[i])
			if kd[i] != sd[i] {
				t.Fatalf("s=%d lane %d kernel=%d scalar=%d", base+i, i, kd[i], sd[i])
			}
		}
	}
	// Clip wings: deep negatives pin to 0, highs pin to 255.
	for _, s := range []int16{-32768, -30000, -10000, -2600, 10710, 10800, 20000, 32751} {
		var ss [8]int16
		var kk [8]uint8
		for i := range ss {
			ss[i] = s
		}
		qpelRoundHalf8(unsafe.Pointer(&kk[0]), unsafe.Pointer(&ss[0]))
		want := roundHalf(s)
		for i := range kk {
			if kk[i] != want {
				t.Fatalf("s=%d lane %d kernel=%d scalar=%d", s, i, kk[i], want)
			}
		}
	}
	// Live band + odd widths (roundHalfRow bulk/tail mix).
	for _, n := range []int{1, 3, 7, 8, 9, 15, 16, 17} {
		var ss [17]int16
		var kd2, sd2 [17]uint8
		for i := 0; i < n; i++ {
			ss[i] = int16(-2600 + i*811)
		}
		roundHalfRow(kd2[:], ss[:], n)
		for i := 0; i < n; i++ {
			sd2[i] = roundHalf(ss[i])
			if kd2[i] != sd2[i] {
				t.Fatalf("n=%d lane %d kernel=%d scalar=%d", n, i, kd2[i], sd2[i])
			}
		}
	}
	// Round-half path allocates zero (kernel + tail, no heap).
	var s8 [8]int16
	var d8 [8]uint8
	if n := testing.AllocsPerRun(20, func() {
		roundHalfRow(d8[:], s8[:], 8)
		roundHalfRow(d8[:], s8[:], 5)
	}); n != 0 {
		t.Fatalf("round-half path allocs = %v want 0", n)
	}
}
