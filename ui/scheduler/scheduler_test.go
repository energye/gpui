package scheduler_test

import (
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"
)

// paceHost embeds StubHost and implements platform.VSyncWaiter for pace tests.
// Fields are atomic: WaitVSync runs on the listener goroutine while the test
// reads/updates them (race-detector clean).
type paceHost struct {
	*platform.StubHost
	waitErr atomic.Value // error (nil → success)
	waits   atomic.Int32
}

func (h *paceHost) WaitVSync() error {
	h.waits.Add(1)
	if e := h.waitErr.Load(); e != nil {
		return e.(error)
	}
	return nil
}

func newPaceHost(waitErr error) *paceHost {
	h := &paceHost{StubHost: platform.NewStubHost(100, 100)}
	if waitErr != nil {
		h.waitErr.Store(waitErr) // atomic.Value cannot Store nil
	}
	return h
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

// demandTicker is a controllable ticker with optional per-tick frame demand.
type demandTicker struct {
	alive bool
	want  bool
}

func (t *demandTicker) Tick(dt float64) bool { return t.alive }
func (t *demandTicker) WantsFrame() bool     { return t.want }

// FrameWanted defaults to legacy behavior: a registered ticker that does
// not opt out requests a frame; an empty registry wants nothing.
func TestFrameWanted_LegacyDefault(t *testing.T) {
	s := scheduler.New()
	if s.FrameWanted() {
		t.Fatal("empty registry must not want frames")
	}
	legacy := &onceTicker{}
	s.Tickers().Add(legacy)
	s.Tickers().TickAll(1.0 / 60)
	if !s.FrameWanted() {
		t.Fatal("legacy ticker (no FrameWanter) must keep per-tick frames")
	}
	if !s.Tickers().HasActive() {
		t.Fatal("legacy ticker must stay registered after first tick")
	}
	s.Tickers().TickAll(1.0 / 60)
	if s.Tickers().HasActive() {
		t.Fatal("legacy ticker must unregister after returning false")
	}
	if s.FrameWanted() {
		t.Fatal("unregistered tickers must not hold frame demand")
	}
}

// An opt-out ticker stays registered for dt without requesting frames.
func TestFrameWanted_OptOut(t *testing.T) {
	s := scheduler.New()
	quiet := &demandTicker{alive: true, want: false}
	s.Tickers().Add(quiet)
	s.Tickers().TickAll(1.0 / 60)
	if s.FrameWanted() {
		t.Fatal("opt-out ticker must not request frames")
	}
	if !s.Tickers().HasActive() {
		t.Fatal("opt-out ticker must stay registered while Tick returns true")
	}
	// Mixed: one legacy ticker keeps demand true.
	s.Tickers().Add(&onceTicker{})
	s.Tickers().TickAll(1.0 / 60)
	if !s.FrameWanted() {
		t.Fatal("any legacy ticker must keep frame demand true")
	}
	// Flipping the opt-out back to wanting resumes demand alone.
	loud := &demandTicker{alive: true, want: true}
	r := scheduler.New()
	r.Tickers().Add(loud)
	r.Tickers().TickAll(1.0 / 60)
	if !r.FrameWanted() {
		t.Fatal("WantsFrame()=true must request frames")
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

// TestFramePace_VsyncSignalDrivesFrames: a working vsync waiter feeds the
// listener; the frame gate opens on fresh signals and metrics report "true".
func TestFramePace_VsyncSignalDrivesFrames(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(200 * time.Millisecond) // software interval long — fallback would be obvious
	s.SetMode(scheduler.ModePersistent)
	h := newPaceHost(nil)

	s.WaitFramePace(h) // starts the listener; first call honestly reports fallback (no signal yet)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		s.WaitFramePace(h)
		if s.Metrics().Snapshot().VSyncSource == "true" && s.FrameDue() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	m := s.Metrics().Snapshot()
	if m.VSyncSource != "true" {
		t.Fatalf("vsync_source=%q want true after signal", m.VSyncSource)
	}
	// One frame per stamp: the gate consumed by the loop break re-opens on
	// the NEXT fresh signal (a stamp may land between two calls).
	deadline2 := time.Now().Add(500 * time.Millisecond)
	reopened := false
	for time.Now().Before(deadline2) {
		if s.FrameDue() {
			reopened = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !reopened {
		t.Fatal("frame gate must reopen on the next fresh vsync signal")
	}
	if h.waits.Load() < 3 {
		t.Fatalf("listener should keep consuming vsyncs, waits=%d", h.waits.Load())
	}
}

// TestFramePace_VsyncErrorCountsMiss: WaitVSync error is an unavailable-vsync
// signal — the listener counts a miss and pacing falls back (no blocking).
func TestFramePace_VsyncErrorCountsMiss(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(15 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := newPaceHost(errors.New("no vblank"))

	t0 := time.Now()
	s.WaitFramePace(h)
	if time.Since(t0) > 500*time.Millisecond {
		t.Fatalf("WaitFramePace blocked on vsync error: %v", time.Since(t0))
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m := s.Metrics().Snapshot(); m.MissedVSync > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	m := s.Metrics().Snapshot()
	if m.MissedVSync == 0 {
		t.Fatalf("listener should count vsync error as a miss, stats=%+v", m)
	}
	if m.VSyncSource != "fallback" {
		t.Fatalf("vsync_source=%q want fallback", m.VSyncSource)
	}
}

// TestFramePace_VsyncErrorRetryThrottled: WaitVSync error is retried, but
// paced at the software interval — a busy-loop would blow past the bound.
func TestFramePace_VsyncErrorRetryThrottled(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(20 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := newPaceHost(errors.New("no vblank"))

	s.WaitFramePace(h)
	time.Sleep(300 * time.Millisecond)
	// 300ms / (20ms retry sleep) ≈ 15 retries; an unthrottled loop would be far more.
	if h.waits.Load() > 30 {
		t.Fatalf("vsync error retry must be throttled, waits=%d in 300ms", h.waits.Load())
	}
	if h.waits.Load() < 3 {
		t.Fatalf("listener should keep retrying on error, waits=%d", h.waits.Load())
	}
}

// TestFrameDue_VsyncStopsThenSoftwareCadence: the real failure sequence —
// vblank was driving frames, then it stops; the fresh-signal window expires
// and pacing must fall back to the software interval without blocking.
func TestFrameDue_VsyncStopsThenSoftwareCadence(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(15 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := newPaceHost(nil)

	s.WaitFramePace(h)
	// Wait for a vsync-driven gate: two consecutive FrameDue within ~5ms
	// (the fresh-signal window; the 15ms software interval cannot do this).
	gotSignal := false
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if s.FrameDue() && s.FrameDue() {
			gotSignal = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !gotSignal {
		t.Fatal("fresh vsync signal should open the gate")
	}

	// Vblank stops: the listener starts failing (simulated stop).
	h.waitErr.Store(errors.New("vblank stopped"))
	time.Sleep(80 * time.Millisecond) // > fresh window (33ms) + software interval
	if !s.FrameDue() {
		t.Fatal("software cadence must take over after the vsync signal expires")
	}
}

// TestFrameDue_SoftwareInterval: without any vsync waiter the gate opens on
// the software interval and never blocks.
func TestFrameDue_SoftwareInterval(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(12 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := platform.NewStubHost(64, 64) // no VSyncWaiter
	s.WaitFramePace(h)                // sets metrics fallback; no listener

	if !s.FrameDue() {
		t.Fatal("first gate must be open (no prior frame)")
	}
	if s.FrameDue() {
		t.Fatal("immediate second gate must be closed (software interval)")
	}
	time.Sleep(20 * time.Millisecond)
	if !s.FrameDue() {
		t.Fatal("software interval should open the gate")
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

// hangHost implements a VSyncWaiter whose WaitVSync never returns — the
// headless/software-Vulkan failure mode the listener design absorbs.
type hangHost struct {
	*platform.StubHost
	waits int32
}

func (h *hangHost) WaitVSync() error {
	atomic.AddInt32(&h.waits, 1)
	select {} // simulate a hung DRM vblank wait
}

// TestFramePace_HungVsync_DoesNotBlock is the regression test for
// "渲染运行一会自动停止": a WaitVSync that never returns must NOT freeze the
// frame loop. The listener goroutine absorbs the hang (once); WaitFramePace
// returns immediately and FrameDue falls back to the software interval.
func TestFramePace_HungVsync_DoesNotBlock(t *testing.T) {	s := scheduler.New()
	s.SetAnimTick(15 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := &hangHost{StubHost: platform.NewStubHost(100, 100)}

	t0 := time.Now()
	s.WaitFramePace(h)
	if time.Since(t0) > 500*time.Millisecond {
		t.Fatalf("WaitFramePace blocked on hung vsync: %v", time.Since(t0))
	}
	// The listener goroutine absorbs the hang asynchronously.
	deadline := time.Now().Add(500 * time.Millisecond)
	for atomic.LoadInt32(&h.waits) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if atomic.LoadInt32(&h.waits) != 1 {
		t.Fatalf("listener waits=%d want 1 (hang absorbed once)", atomic.LoadInt32(&h.waits))
	}
	if s.Metrics().Snapshot().VSyncSource != "fallback" {
		t.Fatalf("vsync_source=%q want fallback (no signal)", s.Metrics().Snapshot().VSyncSource)
	}
	// Software cadence with no vsync signal: gate opens on the interval.
	if !s.FrameDue() {
		t.Fatal("first gate must be open (no prior frame)")
	}
	if s.FrameDue() {
		t.Fatal("immediate second gate must be closed")
	}
	time.Sleep(30 * time.Millisecond) // > animTick
	if !s.FrameDue() {
		t.Fatal("software interval should open the gate")
	}
}

// frameNotifierHost implements platform.FrameNotifier (compositor-driven
// pacing, ENGINE_FRAME_PRESENT_STANDARD.md 块2) and deliberately NOT
// VSyncWaiter — the replacement for the client-side DRM waiter.
type frameNotifierHost struct {
	*platform.StubHost
	requests atomic.Int32
}

func (h *frameNotifierHost) RequestFrameNotify() {
	h.requests.Add(1)
}

// TestFramePace_FrameNotifierSkipsDRMListener: a Host implementing
// FrameNotifier replaces the client-side waiter entirely (single pacing
// source) — no DRM listener goroutine starts, no missed-vsync accounting,
// pacing stays "fallback" until a NoteFramePresented stamps a signal.
func TestFramePace_FrameNotifierSkipsDRMListener(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(200 * time.Millisecond)
	s.SetMode(scheduler.ModePersistent)
	h := &frameNotifierHost{StubHost: platform.NewStubHost(100, 100)}

	s.WaitFramePace(h)
	time.Sleep(100 * time.Millisecond) // would let a DRM listener spin
	s.WaitFramePace(h)
	m := s.Metrics().Snapshot()
	if m.MissedVSync != 0 {
		t.Fatalf("missed_vsync=%d want 0 (no DRM listener)", m.MissedVSync)
	}
	if m.VSyncSource != "fallback" {
		t.Fatalf("vsync_source=%q want fallback until a notice arrives", m.VSyncSource)
	}
}

// TestFramePace_ListenerStampsButDoesNotSchedule: the DRM waiter (块1) only
// stamps pacing — it must NOT schedule frames, so idle stays truly idle
// (on-demand rendering: demand comes from events/tickers only).
func TestFramePace_ListenerStampsButDoesNotSchedule(t *testing.T) {
	s := scheduler.New()
	s.SetAnimTick(200 * time.Millisecond) // software interval long — only a signal opens the gate
	s.SetMode(scheduler.ModePersistent)
	h := newPaceHost(nil)

	s.WaitFramePace(h) // starts the DRM listener
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		s.WaitFramePace(h)
		if s.Metrics().Snapshot().VSyncSource == "true" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s.Metrics().Snapshot().VSyncSource != "true" {
		t.Fatal("working waiter must report vsync_source=true")
	}
	if s.Pending() {
		t.Fatal("listener must not schedule frames (on-demand rendering 块1)")
	}
	if !s.FrameDue() {
		t.Fatal("fresh stamp must open the frame gate")
	}
}

// TestNoteFramePresented_StampsPacing: a compositor "frame shown" notice
// (EventFramePresented → NoteFramePresented) opens the frame gate for the
// next demanded frame but never creates render demand by itself (块1+块2).
func TestNoteFramePresented_StampsPacing(t *testing.T) {
	s := scheduler.New()
	s.SetMode(scheduler.ModePersistent)
	if !s.FrameDue() {
		t.Fatal("first gate is open (no prior frame)")
	}
	if s.FrameDue() {
		t.Fatal("second gate must be closed (software interval not elapsed)")
	}
	s.NoteFramePresented()
	if !s.FrameDue() {
		t.Fatal("gate must open on a fresh frame-presented notice")
	}
	if s.Pending() {
		t.Fatal("notice must not create render demand (块1)")
	}
}
