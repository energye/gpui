package h264

import (
	"math/rand"
	"testing"
)

// itrans4x4Stimulus builds deterministic 4x4 coefficient blocks across
// the magnitudes the dequant feeds the core: small realistic levels,
// large levels, and adversarial full-int32 extremes (wrap-exactness:
// PADDL/PSUBL wrap mod 2^32 exactly like Go int32, PSRAL assembles to
// PSRAD — verified by objdump 2026-09-23 — and floors exactly like Go
// >>, so even extremes must match bit for bit).
func itrans4x4Stimulus(seed int64) [][16]int32 {
	r := rand.New(rand.NewSource(seed))
	var out [][16]int32
	// Zero block and DC-only blocks.
	out = append(out, [16]int32{})
	for _, dc := range []int32{1, -1, 32, -32, 1000000, -1000000, 1 << 30, -1 << 30} {
		var b [16]int32
		b[0] = dc
		out = append(out, b)
	}
	// Small realistic magnitudes (dequantized residuals).
	for n := 0; n < 8; n++ {
		var b [16]int32
		for i := range b {
			if r.Intn(4) == 0 {
				b[i] = int32(r.Intn(20001) - 10000)
			}
		}
		out = append(out, b)
	}
	// Large magnitudes (high QP, big levels).
	for n := 0; n < 8; n++ {
		var b [16]int32
		for i := range b {
			if r.Intn(2) == 0 {
				b[i] = int32(r.Intn(400001) - 200000)
			}
		}
		out = append(out, b)
	}
	// Adversarial full-range (wrap-exactness).
	for n := 0; n < 4; n++ {
		var b [16]int32
		for i := range b {
			b[i] = int32(r.Uint32())
		}
		out = append(out, b)
	}
	// Single-coefficient probes at every position (impulse response).
	for pos := 0; pos < 16; pos++ {
		var b [16]int32
		b[pos] = 1000
		out = append(out, b)
	}
	// Alternating-sign rows (worst case for the >>1 floors).
	var alt [16]int32
	for i := range alt {
		if i%2 == 0 {
			alt[i] = -77777
		} else {
			alt[i] = 77777
		}
	}
	out = append(out, alt)
	return out
}

// S1b-Y gate: the vector 4x4 core equals the scalar core bit for bit
// on every stimulus block (zero, DC-only, realistic, large,
// full-range adversarial, all 16 impulse positions, alternating
// signs). Forced-scalar and kernel legs are compared against each
// other so neither can drift.
func TestS1Itrans4x4MatchesScalar(t *testing.T) {
	old := itransScalarForced
	defer func() { itransScalarForced = old }()
	for _, c := range itrans4x4Stimulus(20260923) {
		want := itrans4x4CoreScalar(c)
		itransScalarForced = false
		got := itrans4x4Core(c)
		itransScalarForced = true
		gotForced := itrans4x4Core(c)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("lane %d kernel=%d scalar=%d (c=%v)", i, got[i], want[i], c)
			}
			if gotForced[i] != want[i] {
				t.Fatalf("lane %d forced=%d scalar=%d (c=%v)", i, gotForced[i], want[i], c)
			}
		}
	}
}

// S1b-Y hot path allocates zero (no heap temps: c/out ride the stack).
func TestS1Itrans4x4ZeroAlloc(t *testing.T) {
	var c [16]int32
	for i := range c {
		c[i] = int32(i*37 - 100)
	}
	old := itransScalarForced
	itransScalarForced = false
	defer func() { itransScalarForced = old }()
	if n := testing.AllocsPerRun(20, func() {
		itrans4x4Core(c)
	}); n != 0 {
		t.Fatalf("fast path allocs = %v want 0", n)
	}
}
