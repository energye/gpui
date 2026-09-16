package hostsink

import (
	"math"
	"testing"
)

// TestFloatToS16 pins the float-to-wire conversion: bounds clip,
// NaN goes silent, 1.0/-1.0 hit the rails. Pure math, no speaker.
func TestFloatToS16(t *testing.T) {
	src := []float32{0, 1, -1, 0.5, 2, -2}
	dst := make([]int16, len(src))
	FloatToS16(dst, src)
	want := []int16{0, 32767, -32768, 16384, 32767, -32768}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("dst[%d] = %d, want %d", i, dst[i], want[i])
		}
	}
	nan := []float32{float32(math.NaN())}
	FloatToS16(dst[:1], nan)
	if dst[0] != 0 {
		t.Fatalf("nan = %d, want 0", dst[0])
	}
	got := S16Bytes(nil, []int16{1, -1})
	if len(got) != 4 || got[0] != 1 || got[1] != 0 || got[2] != 0xFF || got[3] != 0xFF {
		t.Fatalf("s16bytes = % x, want little-endian", got)
	}
}

// TestProbeConsistent pins probe honesty shape: available implies a
// named backend; unavailable implies a readable reason. No speaker
// is opened either way.
func TestProbeConsistent(t *testing.T) {
	backend, available, reason := ProbeHostAudio()
	if available && backend == "" {
		t.Fatal("available with empty backend")
	}
	if !available && reason == "" {
		t.Fatal("unavailable with empty reason")
	}
	if _, err := NewHostSink(0, 2); err == nil {
		t.Fatal("bad spec rate=0 accepted")
	}
}
