package scheduler

import (
	"testing"
	"time"
)

// Steady 60Hz compositor notices must move the software boundary to the
// display period (~16.68ms), ending the 16ms-vs-16.68ms phase drift (E6-A).
func TestCompositorPeriodLearnsSteady(t *testing.T) {
	s := New()
	for i := 0; i < 40; i++ {
		s.learnCompositorPeriod(16680 * time.Microsecond)
	}
	s.mu.Lock()
	p := s.displayPeriod
	boundary := s.boundaryPeriodLocked()
	s.mu.Unlock()
	if p < 16*time.Millisecond || p > 18*time.Millisecond {
		t.Fatalf("displayPeriod=%v want ~16.68ms", p)
	}
	if boundary < 16*time.Millisecond || boundary > 18*time.Millisecond {
		t.Fatalf("boundary=%v want ~16.68ms", boundary)
	}
}

// Batched delivery (pairs of notices stamped together: ~0ms + ~33ms
// intervals) must NOT move the estimate — following it would halve the
// animation rate. The ~0ms samples die at the range gate; the ~33ms median
// dies at the 25ms cap / jitter gate.
func TestCompositorPeriodRejectsBatched(t *testing.T) {
	s := New()
	for i := 0; i < 64; i++ {
		if i%2 == 0 {
			s.learnCompositorPeriod(500 * time.Microsecond)
		} else {
			s.learnCompositorPeriod(33 * time.Millisecond)
		}
	}
	s.mu.Lock()
	p := s.displayPeriod
	s.mu.Unlock()
	if p != 0 {
		t.Fatalf("displayPeriod=%v want unset (batched input rejected)", p)
	}
}

// A learned estimate must survive later jitter: one delayed notice batch
// must not yank the boundary.
func TestCompositorPeriodHoldsThroughJitter(t *testing.T) {
	s := New()
	for i := 0; i < 40; i++ {
		s.learnCompositorPeriod(16680 * time.Microsecond)
	}
	s.mu.Lock()
	base := s.displayPeriod
	s.mu.Unlock()
	if base <= 0 {
		t.Fatal("steady input did not learn")
	}
	for i := 0; i < 10; i++ {
		s.learnCompositorPeriod(60 * time.Millisecond)
	}
	s.mu.Lock()
	p := s.displayPeriod
	s.mu.Unlock()
	d := p - base
	if d < 0 {
		d = -d
	}
	if d > 2*time.Millisecond {
		t.Fatalf("displayPeriod moved %v -> %v on jitter", base, p)
	}
}
