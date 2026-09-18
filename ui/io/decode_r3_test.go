package io_test

// R3-5 honest-counters: submitted/completed/cancelled must reconcile —
// every queued item lands in exactly one terminal bucket.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	uio "github.com/energye/gpui/ui/io"
)

// Bare Run() funcs finish outside deliver — they must count completed.
func TestStats_RunFnCountsCompleted(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	done := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		p.Run(func() { done <- struct{}{} })
	}
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("run fn did not execute")
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := p.Stats()
		if st.Completed >= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("completed=%d want >=4 (stats=%+v)", st.Completed, st)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if st := p.Stats(); st.Submitted != 4 {
		t.Fatalf("submitted=%d want 4", st.Submitted)
	}
}

// A nil listener still completed the work — count it, don't leak it.
func TestStats_NilDoneCountsCompleted(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	data, err := os.ReadFile(filepath.Join("testdata", "small.png"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	p.DecodeBytes(data, nil)
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := p.Stats()
		if st.Completed >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("completed=%d want >=1 (stats=%+v)", st.Completed, st)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A forward dropped by cancel (or close) must not score large_routed.
func TestStats_CancelledForwardNotRouted(t *testing.T) {
	p := uio.NewPool(1)
	before := p.Stats().LargeRouted
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled: submit drops before queueing
	data := append(append([]byte{}, 'T', '4', 'S', 'T', 'U', 'B', '0', '1'), make([]byte, 64)...)
	p.DecodeBytesWithContext(ctx, data, func(uio.Result) {})
	time.Sleep(100 * time.Millisecond)
	st := p.Stats()
	if st.LargeRouted != before {
		t.Fatalf("large_routed=%d want %d (cancelled work never saw the lane)", st.LargeRouted, before)
	}
	if st.Cancelled < 1 {
		t.Fatalf("cancelled=%d want >=1", st.Cancelled)
	}
	p.Close()
}
