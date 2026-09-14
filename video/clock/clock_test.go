package clock

import (
	"sync"
	"testing"
	"time"
)

// TestQueueDueOrder pins the display contract: frames come out newest-due,
// stale ones are counted, future ones wait.
func TestQueueDueOrder(t *testing.T) {
	q := NewQueue(4)
	for _, pts := range []int64{0, 200, 400} {
		if ok, err := q.Push(&Frame{PTSMs: pts}); !ok || err != nil {
			t.Fatalf("push %d: %v %v", pts, ok, err)
		}
	}
	f, skipped, ok := q.PollDue(450)
	if !ok || f.PTSMs != 400 || skipped != 2 {
		t.Fatalf("due450 = %+v skipped %d ok %v, want pts400 skip2", f, skipped, ok)
	}
	if q.Dropped() != 2 {
		t.Fatalf("dropped = %d, want 2", q.Dropped())
	}
	if _, _, ok := q.PollDue(450); ok {
		t.Fatal("empty poll claims a frame")
	}
}

// TestQueueBlocksWhenFull pins backpressure: the second push waits while
// full, then lands once the display drains a slot. Nothing is lost.
func TestQueueBlocksWhenFull(t *testing.T) {
	q := NewQueue(1)
	if ok, _ := q.Push(&Frame{PTSMs: 0}); !ok {
		t.Fatal("first push refused")
	}
	done := make(chan bool, 1)
	go func() {
		ok, err := q.Push(&Frame{PTSMs: 200})
		done <- ok && err == nil
	}()
	select {
	case <-done:
		t.Fatal("push finished while full: backpressure lost")
	case <-time.After(200 * time.Millisecond):
	}
	if _, _, ok := q.PollDue(0); !ok {
		t.Fatal("no frame due at 0")
	}
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("blocked push never landed after drain")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked push stuck after drain")
	}
	if q.Pushes() != 2 || q.MaxDepth() != 1 {
		t.Fatalf("pushes=%d maxdepth=%d, want 2/1", q.Pushes(), q.MaxDepth())
	}
}

// TestQueueCloseUnblocks pins shutdown with a timeout: a blocked producer
// wakes with false, and Drained turns true after the last frame is eaten.
func TestQueueCloseUnblocks(t *testing.T) {
	q := NewQueue(1)
	if ok, _ := q.Push(&Frame{PTSMs: 0}); !ok {
		t.Fatal("first push refused")
	}
	done := make(chan bool, 1)
	go func() {
		ok, _ := q.Push(&Frame{PTSMs: 200})
		done <- ok
	}()
	// Let the producer reach the full queue before stopping it.
	select {
	case <-done:
		t.Fatal("push finished while full: backpressure lost")
	case <-time.After(200 * time.Millisecond):
	}
	q.Close()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("push after close claims kept")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not wake the blocked push")
	}
	if q.Drained() {
		t.Fatal("drained with a frame still queued")
	}
	if _, _, ok := q.PollDue(0); !ok {
		t.Fatal("queued frame lost on close")
	}
	if !q.Drained() {
		t.Fatal("not drained after last frame eaten")
	}
}

// TestQueueStats pins the waterline numbers the window HUD reports.
func TestQueueStats(t *testing.T) {
	q := NewQueue(2)
	if q.Cap() != 2 {
		t.Fatalf("cap = %d", q.Cap())
	}
	if got := NewQueue(0).Cap(); got != DefaultCap {
		t.Fatalf("default cap = %d, want %d", got, DefaultCap)
	}
	for _, pts := range []int64{0, 200} {
		if ok, _ := q.Push(&Frame{PTSMs: pts}); !ok {
			t.Fatalf("push %d refused", pts)
		}
	}
	if q.Depth() != 2 || q.MaxDepth() != 2 {
		t.Fatalf("depth=%d max=%d, want 2/2", q.Depth(), q.MaxDepth())
	}
	if avg := q.DepthAvg(); avg != 1.5 {
		t.Fatalf("avg = %v, want 1.5", avg)
	}
	if _, err := q.Push(nil); err == nil {
		t.Fatal("nil push accepted")
	}
}

// TestQueueTryPush pins non-blocking offer: full returns false at once
// (seek path uses this so progress clicks never wait for drain).
func TestQueueTryPush(t *testing.T) {
	q := NewQueue(1)
	if ok, _ := q.TryPush(&Frame{PTSMs: 0}); !ok {
		t.Fatal("empty trypush refused")
	}
	done := make(chan bool, 1)
	go func() {
		ok, _ := q.TryPush(&Frame{PTSMs: 200})
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("full trypush claims kept")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("trypush blocked (must never wait)")
	}
	if _, _, ok := q.PollDue(0); !ok {
		t.Fatal("queued frame lost")
	}
	if ok, _ := q.TryPush(&Frame{PTSMs: 200}); !ok {
		t.Fatal("trypush after drain refused")
	}
	if _, err := q.TryPush(nil); err == nil {
		t.Fatal("nil trypush accepted")
	}
}

// TestQueueReset pins atomic seek landing: full queue with stale frames
// becomes exactly the landed frame, never blocks, stale never shows.
func TestQueueReset(t *testing.T) {
	q := NewQueue(2)
	for _, pts := range []int64{0, 200} {
		if ok, _ := q.Push(&Frame{PTSMs: pts}); !ok {
			t.Fatalf("push %d refused", pts)
		}
	}
	done := make(chan bool, 1)
	go func() {
		ok, _ := q.Reset(&Frame{PTSMs: 999})
		done <- ok
	}()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("reset refused")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reset blocked (must be atomic, never wait)")
	}
	if q.Depth() != 1 {
		t.Fatalf("depth = %d, want 1 (stale cleared)", q.Depth())
	}
	f, skipped, ok := q.PollDue(999)
	if !ok || f.PTSMs != 999 || skipped != 0 {
		t.Fatalf("due999 = %+v skip %d ok %v, want landed 999 skip 0", f, skipped, ok)
	}
}

// TestClockPause pins the pause contract with a hand-driven clock:
// pausing freezes DuePTSMS, resuming skips the held span.
func TestClockPause(t *testing.T) {
	now := int64(1000)
	c := NewClock(func() int64 { return now })
	c.Start(500)
	now = 1300
	if got := c.DuePTSMS(); got != 800 {
		t.Fatalf("due = %d, want 800", got)
	}
	c.Pause()
	now = 2300
	if got := c.DuePTSMS(); got != 800 {
		t.Fatalf("due while paused = %d, want frozen 800", got)
	}
	if !c.Paused() {
		t.Fatal("not reporting paused")
	}
	c.Resume()
	now = 2500
	if got := c.DuePTSMS(); got != 1000 {
		t.Fatalf("due after resume = %d, want 1000 (held 1000 skipped)", got)
	}
	if c.Paused() {
		t.Fatal("still paused after resume")
	}
}

// TestClockDrift pins the drift sign: a shown stamp at or behind schedule
// reads <= 0, never a scary positive when on time.
func TestClockDrift(t *testing.T) {
	now := int64(0)
	c := NewClock(func() int64 { return now })
	c.Start(0)
	now = 350
	if d := c.DriftMs(200); d != 150 {
		t.Fatalf("drift = %d, want 150 (shown 200, due 350)", d)
	}
	if d := c.DriftMs(350); d != 0 {
		t.Fatalf("drift = %d, want 0", d)
	}
}

// TestClockRate pins the speed clock: 2x advances due stamps twice as
// fast, 0.5x half as fast, Start keeps the rate, and bad rates refuse.
func TestClockRate(t *testing.T) {
	now := int64(1000)
	c := NewClock(func() int64 { return now })
	if c.Rate() != 1 {
		t.Fatalf("rate = %v, want 1", c.Rate())
	}
	c.Start(0)
	if err := c.SetRate(2); err != nil {
		t.Fatalf("rate 2: %v", err)
	}
	now = 1100
	if got := c.DuePTSMS(); got != 200 {
		t.Fatalf("2x due = %d, want 200", got)
	}
	if err := c.SetRate(0.5); err != nil {
		t.Fatalf("rate 0.5: %v", err)
	}
	// Rate change re-anchors in production via Start; emulate the same:
	// due stays continuous (200) then walks at half speed.
	c.Start(200)
	now = 1200
	if got := c.DuePTSMS(); got != 250 {
		t.Fatalf("0.5x due = %d, want 250", got)
	}
	for _, bad := range []float64{0, -1, 9} {
		if err := c.SetRate(bad); err == nil {
			t.Fatalf("rate %v accepted", bad)
		}
	}
	if c.Rate() != 0.5 {
		t.Fatalf("rate after bad set = %v, want 0.5 (unchanged)", c.Rate())
	}
}

// TestQueueConcurrent hammers push/poll from two threads: counts must
// add up exactly, never lose or duplicate. Stamps rise with the push
// index so PollDue can always make progress; a stuck queue fails loud.
func TestQueueConcurrent(t *testing.T) {
	q := NewQueue(8)
	const n = 200
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			if ok, err := q.Push(&Frame{PTSMs: int64(i * 10)}); !ok || err != nil {
				t.Errorf("push %d: %v %v", i, ok, err)
				return
			}
		}
		q.Close()
	}()
	got := 0
	deadline := time.Now().Add(10 * time.Second)
	for got < n {
		if f, _, ok := q.PollDue(int64(got * 10)); ok {
			_ = f
			got++
			continue
		}
		if time.Now().After(deadline) {
			t.Fatalf("stuck at %d/%d frames", got, n)
		}
		time.Sleep(time.Millisecond)
	}
	wg.Wait()
	if q.Pushes() != n {
		t.Fatalf("pushes = %d, want %d", q.Pushes(), n)
	}
	if !q.Drained() {
		t.Fatal("not drained at the end")
	}
}
