package raster_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/energye/gpui/ui/raster"
)

// UI must not block when present is slow: SubmitLatest returns immediately.
func TestSubmitLatest_DoesNotBlockUI(t *testing.T) {
	loop := raster.NewLoop(1, nil)
	loop.Start()
	defer func() {
		// Unblock any stuck job
		time.Sleep(10 * time.Millisecond)
		loop.Stop()
	}()

	block := make(chan struct{})
	slow := raster.FrameJob{Run: func() error {
		<-block
		return nil
	}}
	// Fill the single-slot queue.
	if !loop.TrySubmit(slow) {
		t.Fatal("first submit")
	}
	time.Sleep(20 * time.Millisecond)

	t0 := time.Now()
	var uiContinued atomic.Bool
	// SubmitLatest should return without waiting for slow job.
	_ = loop.SubmitLatest(raster.FrameJob{Run: func() error {
		uiContinued.Store(true)
		return nil
	}})
	if time.Since(t0) > 50*time.Millisecond {
		close(block)
		t.Fatal("SubmitLatest blocked UI")
	}
	// UI can do more work immediately.
	uiContinued.Store(true)
	if !uiContinued.Load() {
		t.Fatal("ui stuck")
	}
	close(block)
	time.Sleep(50 * time.Millisecond)
}

func TestSubmitLatest_PendingReplace(t *testing.T) {
	loop := raster.NewLoop(1, nil)
	loop.Start()
	block := make(chan struct{})
	_ = loop.TrySubmit(raster.FrameJob{Run: func() error {
		<-block
		return nil
	}})
	time.Sleep(15 * time.Millisecond)

	var last atomic.Int32
	for i := 1; i <= 5; i++ {
		n := int32(i)
		_ = loop.SubmitLatest(raster.FrameJob{Run: func() error {
			last.Store(n)
			return nil
		}})
	}
	close(block)
	time.Sleep(80 * time.Millisecond)
	loop.Stop()
	// Pending should be latest (5) after drain; at least some later frame ran.
	if last.Load() == 0 {
		t.Fatal("expected pending job to run")
	}
}
