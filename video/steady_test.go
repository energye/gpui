package video

import (
	"runtime"
	"testing"
)

// TestPollSteadyNoAlloc pins the VR7 Poll fast path: after the head is
// queued, polls with nothing due cost no heap (no timer per call).
func TestPollSteadyNoAlloc(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_m_bframes.mp4", Options{NowMs: h.at, Loop: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	// Drain the head so readyCh is closed and the clock is past it.
	h.now += 10000
	for i := 0; i < 10; i++ {
		p.Poll()
		h.now += 200
	}
	// Steady: nothing due (clock frozen), Poll must not allocate.
	n := testing.AllocsPerRun(50, func() {
		p.Poll()
	})
	if n != 0 {
		t.Fatalf("steady Poll allocs = %v, want 0", n)
	}
}

// TestLoopSteadyBytes pins the VR7 byte budget on a real 1080p loop:
// five wall seconds of loop play cost under 2KB per shown frame (cold
// open excluded; wall pacing only, no correctness assertion here).
func TestLoopSteadyBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("wall-clock steady probe skipped in -short")
	}
	p, err := OpenFile("testdata/vr2_1080p.mp4", Options{Loop: true})
	if err != nil {
		t.Skipf("1080p clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	defer p.Close()
	// Warm up past open + first pass so the measurement is steady only.
	WallSleep(1500)
	runtime.GC()
	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)
	deadline := WallDeadline(5000)
	var shown int64
	for {
		f, _ := p.Poll()
		if f != nil {
			shown++
		}
		if WallPast(deadline) {
			break
		}
		WallSleep(5)
	}
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	if shown < 5 {
		t.Fatalf("shown = %d, want >= 5 in 5s loop", shown)
	}
	per := int64(m1.TotalAlloc-m0.TotalAlloc) / shown
	t.Logf("1080p steady: shown=%d per_frame_B=%d", shown, per)
	if per > 2048 {
		t.Fatalf("per_frame_B = %d, want <= 2048", per)
	}
}
