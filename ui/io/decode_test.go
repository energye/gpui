package io_test

import (
	"sync"
	"testing"
	"time"

	"github.com/energye/gpui/ui/io"
)

func TestPool_RunAsync(t *testing.T) {
	p := io.NewPool(2)
	var wg sync.WaitGroup
	wg.Add(1)
	started := make(chan struct{})
	p.Run(func() {
		close(started)
		time.Sleep(30 * time.Millisecond)
		wg.Done()
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	// Caller not blocked for full sleep if we don't wait — we already returned from Run.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not finish")
	}
}

func TestDecodeFile_Missing(t *testing.T) {
	done := make(chan io.Result, 1)
	io.DecodeFile("/nonexistent/path/nope.png", func(r io.Result) {
		done <- r
	})
	select {
	case r := <-done:
		if r.Err == nil {
			t.Fatal("expected error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}
