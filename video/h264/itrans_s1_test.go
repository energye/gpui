package h264

import (
	"math/rand"
	"runtime"
	"testing"
)

// itransStimulus builds deterministic 8x8 coefficient blocks across
// the magnitudes the dequant feeds the core: small realistic levels,
// large levels, and adversarial full-int32 extremes (wrap-exactness:
// PADDD/PSUBD wrap mod 2^32 exactly like Go int32/uint32, PSRAD
// floors exactly like Go >>, so even extremes must match bit for bit).
func itransStimulus(seed int64) [][64]int32 {
	r := rand.New(rand.NewSource(seed))
	var out [][64]int32
	// Zero block and DC-only blocks.
	out = append(out, [64]int32{})
	for _, dc := range []int32{1, -1, 32, -32, 1000000, -1000000, 1 << 30, -1 << 30} {
		var b [64]int32
		b[0] = dc
		out = append(out, b)
	}
	// Small realistic magnitudes (dequantized residuals).
	for n := 0; n < 8; n++ {
		var b [64]int32
		for i := range b {
			if r.Intn(4) == 0 {
				b[i] = int32(r.Intn(20001) - 10000)
			}
		}
		out = append(out, b)
	}
	// Large magnitudes (high QP, big levels).
	for n := 0; n < 8; n++ {
		var b [64]int32
		for i := range b {
			if r.Intn(2) == 0 {
				b[i] = int32(r.Intn(400001) - 200000)
			}
		}
		out = append(out, b)
	}
	// Adversarial full-range (wrap-exactness).
	for n := 0; n < 4; n++ {
		var b [64]int32
		for i := range b {
			b[i] = int32(r.Uint32())
		}
		out = append(out, b)
	}
	// Single-coefficient probes at every position (impulse response).
	for pos := 0; pos < 64; pos++ {
		var b [64]int32
		b[pos] = 1000
		out = append(out, b)
	}
	return out
}

// S1-T gate: the vector 8x8 core equals the scalar core bit for bit
// on every stimulus block (zero, DC-only, realistic, large,
// full-range adversarial, all 64 impulse positions).
func TestS1Itrans8x8MatchesScalar(t *testing.T) {
	old := itransScalarForced
	itransScalarForced = false
	defer func() { itransScalarForced = old }()
	for _, c := range itransStimulus(20260922) {
		want := itrans8x8Core(c)
		got, ok := itrans8x8Fast(c)
		if runtime.GOARCH == "amd64" && !ok {
			t.Fatalf("amd64 unforced fast path not taken")
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("lane %d fast=%d scalar=%d (c=%v)", i, got[i], want[i], c)
			}
		}
	}
}

// S1-T taken: unforced reports true (kernels on amd64, scalar core
// via the arch stub elsewhere); forced-scalar reports false and the
// caller runs the scalar留守 with identical output.
func TestS1Itrans8x8Taken(t *testing.T) {
	var c [64]int32
	c[0], c[63] = 100, -50
	old := itransScalarForced
	defer func() { itransScalarForced = old }()
	itransScalarForced = false
	if _, ok := itrans8x8Fast(c); !ok {
		t.Fatalf("unforced fast path not taken")
	}
	itransScalarForced = true
	if _, ok := itrans8x8Fast(c); ok {
		t.Fatalf("forced scalar fast path taken, want fallback")
	}
}

// S1-T hot path allocates zero (two stack temps only).
func TestS1Itrans8x8ZeroAlloc(t *testing.T) {
	var c [64]int32
	for i := range c {
		c[i] = int32(i*37 - 100)
	}
	if n := testing.AllocsPerRun(20, func() {
		itrans8x8Fast(c)
	}); n != 0 {
		t.Fatalf("fast path allocs = %v want 0", n)
	}
}
