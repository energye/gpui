package scheduler_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

// stayTicker keeps the scheduler in ModePersistent (like an always-on
// animation ticker in a real app; RecomputeMode drops to IDLE otherwise).
type stayTicker struct{}

func (t *stayTicker) Tick(dt float64) bool { return true }

// TestFrameDue_SlowStampsDoNotThrottleBelowSoftwareInterval guards the
// degraded-vsync case that drove the pacing fix: compositor frame-done
// notices can arrive far slower than the display refresh (observed
// ~2×-delayed wl_surface.frame callbacks, stamps ≈ renders/2). A
// once-per-stamp gate alone throttles animation below the software floor;
// the floor must stay unconditional. Stamps at 50ms (interval > animTick)
// with a 16ms floor → a throttled gate opens ~16× in 400ms, the floor opens
// ~23×. Assert ≥20 AND that gates never bunch below the floor cadence.
func TestFrameDue_SlowStampsDoNotThrottleBelowSoftwareInterval(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(16 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)

	// Seed the stamp cadence before measuring: the first stamp has no
	// interval yet (opens the gate as a fresh boundary); the second stamps
	// it slow (interval > animTick) so the goroutine below cannot open the
	// gate per stamp.
	s.NoteFramePresented()
	time.Sleep(20 * time.Millisecond)
	s.NoteFramePresented()

	stop := make(chan struct{})
	defer close(stop)
	go func() { // slow compositor notices
		tk := time.NewTicker(50 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				s.NoteFramePresented()
			}
		}
	}()

	open := 0
	var last time.Time
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		if s.FrameDue() {
			if open > 0 {
				if el := time.Since(last); el < 12*time.Millisecond {
					t.Fatalf("gate opened %v after previous — floor cadence violated (double-fire?)", el)
				}
				time.Sleep(time.Millisecond)
			}
			open++
			last = time.Now()
		}
		time.Sleep(2 * time.Millisecond)
	}
	if open < 20 {
		t.Fatalf("slow stamps throttled animation: %d gates/400ms, want ≥20 (software floor ~60Hz)", open)
	}
}

// TestFrameDue_FastStampsPaceOncePerStamp: a fast vsync source (interval ≤
// animTick, e.g. a 120Hz vblank) must pace once per stamp — no double-fire
// per stamp, and the gate re-opens only on the next stamp.
func TestFrameDue_FastStampsPaceOncePerStamp(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(16 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)

	// Fast stamps: 5ms apart.
	s.NoteFramePresented()
	time.Sleep(5 * time.Millisecond)
	s.NoteFramePresented() // interval 5ms ≤ 16ms → fast source

	if !s.FrameDue() {
		t.Fatal("first gate after a fast stamp must open")
	}
	if s.FrameDue() {
		t.Fatal("gate must not open twice for one stamp")
	}
	time.Sleep(5 * time.Millisecond)
	s.NoteFramePresented()
	if !s.FrameDue() {
		t.Fatal("gate must re-open on the next stamp")
	}
}

// TestFrameDue_StaleStampsFallBackToSoftwareFloor: the floor must open even
// while the last stamp is still inside the fresh window — the software floor
// is unconditional (a slow notice source cannot hold it closed).
func TestFrameDue_StaleStampsFallBackToSoftwareFloor(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(12 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)

	s.NoteFramePresented() // seed interval 0 → slow classification
	if !s.FrameDue() {
		t.Fatal("first gate must open")
	}
	if s.FrameDue() {
		t.Fatal("immediate second gate must be closed (floor not elapsed)")
	}
	time.Sleep(20 * time.Millisecond)
	if !s.FrameDue() {
		t.Fatal("software floor must open the gate even while the stamp is fresh")
	}
}

// TestWaitTimeout_TargetsNextFrameBoundary: in persistent mode the event
// loop must wake exactly at the next floor deadline (not the full animTick),
// so the pacing deadline is not quantized to event-arrival boundaries.
func TestWaitTimeout_TargetsNextFrameBoundary(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(40 * time.Millisecond)
	s.Tickers().Add(&stayTicker{}) // keep ModePersistent through RecomputeMode
	s.SetMode(scheduler.ModePersistent)

	if !s.FrameDue() {
		t.Fatal("first gate must open")
	}
	time.Sleep(10 * time.Millisecond)
	got := s.WaitTimeout()
	if got <= 0 || got > 40*time.Millisecond {
		t.Fatalf("WaitTimeout=%v want (0, 40ms] (remaining to floor boundary)", got)
	}
	if got < 20*time.Millisecond || got > 35*time.Millisecond {
		t.Fatalf("WaitTimeout=%v want ≈30ms remaining after 10ms of a 40ms floor", got)
	}
}

// TestWaitTimeout_OverdueReturnsZero: when a slow iteration overruns the
// floor boundary, the loop must not sleep the full interval again — poll
// immediately and let FrameDue open on the overdue deadline.
func TestWaitTimeout_OverdueReturnsZero(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(40 * time.Millisecond)
	s.Tickers().Add(&stayTicker{}) // keep ModePersistent through RecomputeMode
	s.SetMode(scheduler.ModePersistent)

	if !s.FrameDue() {
		t.Fatal("first gate must open")
	}
	time.Sleep(50 * time.Millisecond) // > animTick → boundary already passed
	if got := s.WaitTimeout(); got != 0 {
		t.Fatalf("WaitTimeout=%v want 0 when the floor boundary already passed", got)
	}
}
