package scheduler

import (
	"testing"
	"time"
)

// The platform-reported refresh seeds the pacing baseline from the first
// frame (no 16-sample learn lag, no hardcoded machine-specific period).
func TestSeedDisplayRefreshHz_Baseline(t *testing.T) {
	s := New()
	s.mu.Lock()
	before := s.boundaryPeriodLocked()
	s.mu.Unlock()
	if before != DefaultAnimTick {
		t.Fatalf("unseeded boundary=%v want DefaultAnimTick %v", before, DefaultAnimTick)
	}
	s.SeedDisplayRefreshHz(59.88)
	s.mu.Lock()
	got := s.boundaryPeriodLocked()
	s.mu.Unlock()
	wantSec := float64(time.Second) / 59.88
	want := time.Duration(wantSec)
	if d := got - want; d < -time.Microsecond || d > time.Microsecond {
		t.Fatalf("seeded boundary=%v want %v (59.88Hz)", got, want)
	}
}

// Garbage/unknown reports must not move the baseline.
func TestSeedDisplayRefreshHz_RejectsGarbage(t *testing.T) {
	for _, hz := range []float64{0, -60, 5, 19.9, 240.1, 1000} {
		s := New()
		s.SeedDisplayRefreshHz(hz)
		s.mu.Lock()
		got := s.boundaryPeriodLocked()
		s.mu.Unlock()
		if got != DefaultAnimTick {
			t.Fatalf("hz=%v: boundary=%v want DefaultAnimTick %v", hz, got, DefaultAnimTick)
		}
	}
}

// Matching measured stamps agree with the seeded baseline (reported rate
// wins when they diverge — see TestGears_ReportedWinsOverLearnedDrift).
func TestSeedDisplayRefreshHz_LearnedMatchesSeed(t *testing.T) {
	s := New()
	s.SeedDisplayRefreshHz(120)
	s.learnDisplayPeriod(8333 * time.Microsecond) // 120Hz DRM stamp
	s.mu.Lock()
	got := s.boundaryPeriodLocked()
	s.mu.Unlock()
	if got < 8*time.Millisecond || got > 9*time.Millisecond {
		t.Fatalf("learned boundary=%v want ~8.33ms on a 120Hz seed", got)
	}
}
