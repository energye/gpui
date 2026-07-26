package raster_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/raster"
	"github.com/energye/gpui/ui/scheduler"
)

func TestLoop_Backpressure(t *testing.T) {
	const depth = 2
	m := scheduler.New().Metrics()
	loop := raster.NewLoop(depth, m)
	loop.Start()

	block := make(chan struct{})
	job := raster.FrameJob{Run: func() error {
		<-block
		return nil
	}}

	accepted := 0
	for i := 0; i < 8; i++ {
		if loop.TrySubmit(job) {
			accepted++
		} else {
			break
		}
	}
	// Buffer holds `depth` jobs; worker may hold 1 more → accepted ∈ [depth, depth+1].
	if accepted < depth || accepted > depth+1 {
		close(block)
		loop.Stop()
		t.Fatalf("accepted=%d want %d or %d", accepted, depth, depth+1)
	}
	if loop.TrySubmit(job) {
		close(block)
		loop.Stop()
		t.Fatal("expected backpressure after pipeline full")
	}
	close(block)
	time.Sleep(30 * time.Millisecond)
	loop.Stop()
}

func TestLoop_DoneChannel(t *testing.T) {
	loop := raster.NewLoop(2, nil)
	loop.Start()
	defer loop.Stop()

	done := make(chan error, 1)
	if !loop.TrySubmit(raster.FrameJob{
		Run:  func() error { return nil },
		Done: done,
	}) {
		t.Fatal("submit")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
