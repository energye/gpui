package raster_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/energye/gpui/ui/raster"
)

// T2 G4 build-before-place: a full pipeline must refuse the reserve probe so
// the UI thread skips the expensive build instead of wasting it.
func TestLoop_TryReserve_BuildBeforePlace(t *testing.T) {
	loop := raster.NewLoop(1, nil)
	loop.Start()
	defer loop.Stop()
	if !loop.TryReserve() {
		t.Fatal("empty pipeline must reserve")
	}
	if loop.Full() {
		t.Fatal("empty pipeline must not report full")
	}
	block := make(chan struct{})
	// Occupy the worker.
	if !loop.TrySubmit(raster.FrameJob{Run: func() error {
		<-block
		return nil
	}}) {
		t.Fatal("first submit must be accepted")
	}
	time.Sleep(50 * time.Millisecond) // worker picked it up
	// Fill the single-slot buffer while the worker is blocked.
	if !loop.TrySubmit(raster.FrameJob{Run: func() error { return nil }}) {
		close(block)
		t.Fatal("buffer slot must be accepted while worker blocked")
	}
	if !loop.Full() {
		close(block)
		t.Fatal("filled pipeline must report full")
	}
	if loop.TryReserve() {
		close(block)
		t.Fatal("full pipeline must refuse reserve (build-before-place)")
	}
	// SubmitLatest while full parks in pending (latest-wins, never blocks).
	if loop.SubmitLatest(raster.FrameJob{Run: func() error { return nil }}) {
		close(block)
		t.Fatal("SubmitLatest on a full pipeline must report pending, not queued")
	}
	if !loop.HasPending() {
		close(block)
		t.Fatal("pending slot must hold the latest job")
	}
	close(block)
	// Drain: reserve must open again.
	deadline := time.Now().Add(3 * time.Second)
	for loop.Full() || !loop.TryReserve() {
		if time.Now().After(deadline) {
			t.Fatal("pipeline did not drain")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// T2 G5 thread assertions: UI helpers panic on the wrong thread, pass on the
// right one. Panics inside the job are recovered and reported via channel so
// a failure never kills the loop goroutine.
func TestLoop_ThreadAssertions(t *testing.T) {
	loop := raster.NewLoop(2, nil)
	// Off-raster (test goroutine = UI side): raster assert panics; UI assert
	// is convention-only no-op (cannot enforce across concurrent goroutines).
	loop.AssertUIThread()
	mustPanic(t, "AssertRasterThread off raster", func() { loop.AssertRasterThread() })
	if loop.OnRasterThread() {
		t.Fatal("must not report raster thread on UI")
	}
	var nilLoop *raster.Loop
	nilLoop.AssertUIThread() // nil-safe
	nilLoop.AssertRasterThread()
	if nilLoop.OnRasterThread() || nilLoop.TryReserve() || nilLoop.Full() || nilLoop.HasPending() {
		t.Fatal("nil loop helpers must be safe no-ops")
	}

	loop.Start()
	defer loop.Stop()
	done := make(chan error, 1)
	if !loop.TrySubmit(raster.FrameJob{Run: func() error {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("unexpected panic on raster: %v", r)
			}
		}()
		loop.AssertRasterThread()
		if !loop.OnRasterThread() {
			done <- fmt.Errorf("must report raster thread inside job")
			return nil
		}
		done <- nil
		return nil
	}}) {
		t.Fatal("submit")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("raster job did not run")
	}
}

func mustPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s must panic", what)
		}
	}()
	fn()
}
