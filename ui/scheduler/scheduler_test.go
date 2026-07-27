package scheduler_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"
)

// paceHost embeds StubHost and implements platform.VSyncWaiter for pace tests.
type paceHost struct {
	*platform.StubHost
	waitErr error
	waits   int
}

func (h *paceHost) WaitVSync() error {
	h.waits++
	return h.waitErr
}

type onceTicker struct{ n int }

func (t *onceTicker) Tick(dt float64) bool {
	t.n++
	return t.n < 2
}

func TestMetrics_JSON(t *testing.T) {
	s := scheduler.New()
	s.Metrics().NotePresent()
	s.Metrics().SetPipeline(1, 2)
	b, err := s.Metrics().JSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["present_count"].(float64) != 1 {
		t.Fatalf("metrics=%s", b)
	}
}

func TestSchedule_PendingAndMode(t *testing.T) {
	s := scheduler.New()
	if s.WaitTimeout() != -1 {
		t.Fatal("idle should wait forever")
	}
	s.ScheduleFrame()
	s.RecomputeMode()
	if s.Mode() != scheduler.ModeTransient {
		t.Fatalf("mode=%v", s.Mode())
	}
	if s.WaitTimeout() != 0 {
		t.Fatal("transient pending should poll")
	}
	s.ClearPending()
	s.RecomputeMode()
	if s.Mode() != scheduler.ModeIdle {
		t.Fatalf("mode=%v", s.Mode())
	}
}

func TestTickers(t *testing.T) {
	s := scheduler.New()
	tk := &onceTicker{}
	s.Tickers().Add(tk)
	s.RecomputeMode()
	if s.Mode() != scheduler.ModePersistent {
		t.Fatal(s.Mode())
	}
	if s.WaitTimeout() != scheduler.DefaultAnimTick {
		t.Fatalf("timeout=%v", s.WaitTimeout())
	}
	s.Tick()
	if !s.Tickers().HasActive() {
		t.Fatal("expected still active after first tick")
	}
	s.Tick()
	if s.Tickers().HasActive() {
		t.Fatal("expected removed after second tick")
	}
}

func TestMetrics_Interval(t *testing.T) {
	s := scheduler.New()
	s.Metrics().NoteFrameInterval(time.Now())
	time.Sleep(5 * time.Millisecond)
	s.Metrics().NoteFrameInterval(time.Now())
	m := s.Metrics().Snapshot()
	if m.FrameCount != 2 || m.LastFrameIntervalMs <= 0 {
		t.Fatalf("%+v", m)
	}
	if m.MaxFrameIntervalMs < m.LastFrameIntervalMs {
		t.Fatalf("max interval %.2f < last %.2f", m.MaxFrameIntervalMs, m.LastFrameIntervalMs)
	}
	if m.AvgFrameIntervalMs <= 0 {
		t.Fatalf("avg interval expected > 0: %+v", m)
	}
}

func TestMetrics_HitchCount(t *testing.T) {
	s := scheduler.New()
	t0 := time.Now()
	s.Metrics().NoteFrameInterval(t0)
	// Simulate a clear hitch (> 33.4ms).
	s.Metrics().NoteFrameInterval(t0.Add(50 * time.Millisecond))
	m := s.Metrics().Snapshot()
	if m.HitchCount != 1 {
		t.Fatalf("hitch_count want 1 got %d (%+v)", m.HitchCount, m)
	}
	if m.MaxFrameIntervalMs < 49 {
		t.Fatalf("max_frame_interval_ms want ~50 got %.2f", m.MaxFrameIntervalMs)
	}
}

func TestWaitFramePace_TrueVSyncPreferred(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(200 * time.Millisecond) // would be obvious if sleep path taken
	s.SetMode(scheduler.ModePersistent)
	h := &paceHost{StubHost: platform.NewStubHost(100, 100), waitErr: nil}

	t0 := time.Now()
	s.WaitFramePace(h)
	elapsed := time.Since(t0)
	if h.waits != 1 {
		t.Fatalf("WaitVSync calls=%d want 1", h.waits)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("true WaitVSync path slept too long: %v (fallback animTick?)", elapsed)
	}
	m := s.Metrics().Snapshot()
	if m.VSyncSource != "true" {
		t.Fatalf("vsync_source=%q want true", m.VSyncSource)
	}
	if m.MissedVSync != 0 {
		t.Fatalf("missed_vsync=%d want 0", m.MissedVSync)
	}
}

func TestWaitFramePace_ErrorFallsBackAndMisses(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(15 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := &paceHost{StubHost: platform.NewStubHost(100, 100), waitErr: errors.New("no vblank")}

	t0 := time.Now()
	s.WaitFramePace(h)
	elapsed := time.Since(t0)
	if h.waits != 1 {
		t.Fatalf("WaitVSync calls=%d", h.waits)
	}
	if elapsed < 10*time.Millisecond {
		t.Fatalf("fallback should sleep animTick, elapsed=%v", elapsed)
	}
	m := s.Metrics().Snapshot()
	if m.VSyncSource != "fallback" {
		t.Fatalf("vsync_source=%q want fallback", m.VSyncSource)
	}
	if m.MissedVSync != 1 {
		t.Fatalf("missed_vsync=%d want 1", m.MissedVSync)
	}
}

func TestWaitFramePace_NoWaiter_Fallback(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(12 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := platform.NewStubHost(64, 64) // no VSyncWaiter

	t0 := time.Now()
	s.WaitFramePace(h)
	if time.Since(t0) < 8*time.Millisecond {
		t.Fatalf("expected software sleep")
	}
	m := s.Metrics().Snapshot()
	if m.VSyncSource != "fallback" {
		t.Fatalf("vsync_source=%q want fallback", m.VSyncSource)
	}
	if m.MissedVSync != 0 {
		t.Fatalf("no waiter is not a miss, got missed=%d", m.MissedVSync)
	}
}

func TestHostVSync_TypeAssert(t *testing.T) {
	h := &paceHost{StubHost: platform.NewStubHost(1, 1)}
	if platform.HostVSync(h) == nil {
		t.Fatal("HostVSync should see VSyncWaiter")
	}
	if platform.HostVSync(platform.NewStubHost(1, 1)) != nil {
		t.Fatal("plain StubHost must not claim WaitVSync")
	}
}
