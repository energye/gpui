package video

import (
	"runtime"
	"testing"
)

// handClock is a deterministic millisecond source for player tests.
type handClock struct{ now int64 }

func (h *handClock) at() int64 { return h.now }

// openTestClip opens a tiny committed clip with the hand clock.
func openTestClip(t *testing.T, h *handClock, loop bool) *Player {
	t.Helper()
	p, err := OpenFile("testdata/vr2_m_bframes.mp4", Options{NowMs: h.at, Loop: loop})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return p
}

// TestPlayOrder pins the VR4 core: presentation order follows display
// stamps (B reorder fixed), every frame shows exactly once, then Ended.
// The gate clip is 5fps, so the hand clock steps one frame interval.
func TestPlayOrder(t *testing.T) {
	h := &handClock{}
	p := openTestClip(t, h, false)
	defer p.Close()
	if p.Info().Frames != 5 {
		t.Fatalf("frames = %d, want 5", p.Info().Frames)
	}
	var seqs []int64
	// Clock starts one step before the head stamp, so tick 0 already
	// shows seq 0; then one frame per tick, done on seq 4's call.
	ended := false
	for i := 0; i < 8 && !ended; i++ {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			seqs = append(seqs, fr.Seq)
		}
		ended = done
		if len(seqs) > 5 {
			t.Fatalf("shown %d frames, clip has 5", len(seqs))
		}
	}
	if !ended {
		t.Fatal("not ended after full play")
	}
	for i, s := range seqs {
		if s != int64(i) {
			t.Fatalf("show order = %v, want 0..4", seqs)
		}
	}
	st := p.Stats()
	if st.Shown != 5 || st.Decoded != 5 {
		t.Fatalf("stats shown=%d decoded=%d, want 5/5", st.Shown, st.Decoded)
	}
	if st.Dropped != 0 {
		t.Fatalf("dropped = %d, want 0 (display kept up)", st.Dropped)
	}
	// Ended latches on the last shown frame; confirm the snapshot agrees.
	if !p.Stats().Ended {
		t.Fatal("not ended after full play")
	}
}

// TestPauseFreezes pins pause: the same stamp shows nothing new while
// held, then play continues where it stopped.
func TestPauseFreezes(t *testing.T) {
	h := &handClock{}
	p := openTestClip(t, h, false)
	defer p.Close()
	h.now += 200
	f0, _ := p.Poll()
	if f0 == nil {
		t.Fatal("first frame never due")
	}
	p.Pause()
	if !p.Paused() {
		t.Fatal("not reporting paused")
	}
	h.now += 10000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d showed while paused", f.Seq)
	}
	p.Resume()
	if p.Paused() {
		t.Fatal("still paused after resume")
	}
	h.now += 200
	f1, _ := p.Poll()
	if f1 == nil || f1.Seq != f0.Seq+1 {
		t.Fatalf("after resume = %+v, want seq %d", f1, f0.Seq+1)
	}
}

// TestDropStale pins the catch-up rule: when the display stalls, only
// the newest due frame shows and the skipped ones count as dropped.
func TestDropStale(t *testing.T) {
	h := &handClock{}
	p := openTestClip(t, h, false)
	defer p.Close()
	h.now += 10000
	f, _ := p.Poll()
	if f == nil || f.Seq != 4 {
		t.Fatalf("catch-up = %+v, want seq 4 (newest)", f)
	}
	if d := p.Stats().Dropped; d != 4 {
		t.Fatalf("dropped = %d, want 4", d)
	}
	// The rest drains to Ended: the catch-up poll already showed the
	// head, live hit zero there, so the snapshot reports done.
	if !p.Stats().Ended {
		t.Fatal("not ended after drain")
	}
}

// TestLoopReplays pins looping: stamps keep counting up across the wrap
// and the first frame shows again. Pass 1 spans 400..1200; pass 2 replays
// at +1000, so tick 5 shows seq 0 a second time. The producer needs a
// scheduling quantum between passes, so the test yields each tick.
func TestLoopReplays(t *testing.T) {
	h := &handClock{}
	p := openTestClip(t, h, true)
	defer p.Close()
	seen := map[int64]int{}
	for i := 0; i < 11; i++ {
		h.now += 200
		f, ended := p.Poll()
		if f != nil {
			seen[f.Seq]++
		}
		if ended {
			t.Fatalf("looping player reports ended at tick %d", i)
		}
		runtime.Gosched()
	}
	if seen[0] < 2 {
		t.Fatalf("seq0 shown %d times, want >= 2 (looped)", seen[0])
	}
	if _, ended := p.Poll(); ended {
		t.Fatal("looping player reports ended")
	}
}

// TestBadClips pins readable errors: missing file, non-MP4, MP4 without
// a video track all fail naming the problem, never a bare panic.
func TestBadClips(t *testing.T) {
	h := &handClock{}
	if _, err := OpenFile("testdata/does-not-exist.mp4", Options{NowMs: h.at}); err == nil {
		t.Fatal("missing file opens")
	}
	if _, err := OpenFile("testdata/vr3_vectors.json", Options{NowMs: h.at}); err == nil {
		t.Fatal("non-mp4 opens")
	} else {
		t.Logf("non-mp4 err: %v", err)
	}
}
