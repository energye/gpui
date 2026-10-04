package scheduler

import (
	"testing"
	"time"
)

// TestPhaseStagger_SpreadsWindows: phase slots shift boundaries inside one
// period (Flutter per-VsyncWaiter construction-time phase): slot 0 is the
// legacy grid, slots 1..3 spread by golden-ratio fractions. Same lastFrameAt
// must yield distinct boundaries.
func TestPhaseStagger_SpreadsWindows(t *testing.T) {
	mk := func(seq int) *FrameScheduler {
		s := New()
		s.SetPhaseSeq(seq)
		base := time.Now().Add(-time.Second).Truncate(time.Millisecond)
		s.mu.Lock()
		s.lastFrameAt = base
		s.mu.Unlock()
		return s
	}
	mk0, mk1, mk2 := mk(0), mk(1), mk(2)
	now := time.Now()
	b := func(s *FrameScheduler) time.Time {
		s.mu.Lock()
		defer s.mu.Unlock()
		bd, _ := s.nextFrameBoundaryLocked(now)
		return bd
	}
	b0, b1, b2 := b(mk0), b(mk1), b(mk2)
	// Golden-ratio fractions are distinct by slot, not monotonic:
	// f(1)=0.618, f(2)=0.236, f(3)=0.854. Assert spread + containment.
	if b0.Equal(b1) || b0.Equal(b2) || b1.Equal(b2) {
		t.Fatalf("boundaries not spread: %v %v %v", b0, b1, b2)
	}
	period := mk0.boundaryPeriodLocked()
	lo, hi := b0, b0
	for _, x := range []time.Time{b1, b2} {
		if x.Before(lo) {
			lo = x
		}
		if x.After(hi) {
			hi = x
		}
	}
	if d := hi.Sub(lo); d <= 0 || d >= period {
		t.Fatalf("spread %v outside one period %v", d, period)
	}
}

// TestPhaseStagger_SlotZeroLegacy: default schedulers (no SetPhaseSeq) keep
// the legacy grid — single-window behavior unchanged.
func TestPhaseStagger_SlotZeroLegacy(t *testing.T) {
	a, b := New(), New()
	now := time.Now()
	for _, s := range []*FrameScheduler{a, b} {
		s.mu.Lock()
		s.lastFrameAt = now.Add(-time.Second).Truncate(time.Millisecond)
		s.mu.Unlock()
	}
	ba, _ := func() (time.Time, time.Duration) {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.nextFrameBoundaryLocked(now)
	}()
	bb, _ := func() (time.Time, time.Duration) {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.nextFrameBoundaryLocked(now)
	}()
	if !ba.Equal(bb) {
		t.Fatalf("slot-0 boundaries differ: %v vs %v", ba, bb)
	}
}
