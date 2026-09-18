package raster_test

import (
	"errors"
	"testing"
	"time"

	"github.com/energye/gpui/ui/raster"
)

// R0-7: the raster loop must survive Stop → Start. Before the fix the quit
// channel was closed by Stop and never recreated, so the restarted goroutine
// exited immediately and every later job hung forever.
func TestLoop_RestartRunsJobs(t *testing.T) {
	loop := raster.NewLoop(2, nil)
	loop.Start()
	if !loop.TrySubmit(raster.FrameJob{Run: func() error { return nil }}) {
		loop.Stop()
		t.Fatal("first-generation submit must be accepted")
	}
	loop.Stop()

	loop.Start()
	done := make(chan error, 1)
	if !loop.TrySubmit(raster.FrameJob{
		Run:  func() error { return nil },
		Done: done,
	}) {
		loop.Stop()
		t.Fatal("post-restart submit must be accepted")
	}
	select {
	case err := <-done:
		if err != nil {
			loop.Stop()
			t.Fatalf("post-restart job: %v", err)
		}
	case <-time.After(3 * time.Second):
		loop.Stop()
		t.Fatal("post-restart job never ran (quit not recreated?)")
	}
	loop.Stop()
}

func TestLoop_RestartDoubleStopStartSafe(t *testing.T) {
	loop := raster.NewLoop(1, nil)
	loop.Start()
	loop.Start() // second start is a no-op
	loop.Stop()
	loop.Stop() // second stop is a no-op
	loop.Start()
	done := make(chan error, 1)
	want := errors.New("r0-7-marker")
	if !loop.TrySubmit(raster.FrameJob{
		Run:  func() error { return want },
		Done: done,
	}) {
		loop.Stop()
		t.Fatal("submit after double stop/start must be accepted")
	}
	select {
	case err := <-done:
		if !errors.Is(err, want) {
			loop.Stop()
			t.Fatalf("got %v want marker error", err)
		}
	case <-time.After(3 * time.Second):
		loop.Stop()
		t.Fatal("job never ran after double stop/start")
	}
	loop.Stop()
}
