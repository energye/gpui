package scheduler

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/ui/platform"
)

// Mode is the demand-driven frame mode (Flutter-like).
type Mode int

const (
	// ModeIdle blocks on events only (no anim tick).
	ModeIdle Mode = iota
	// ModeTransient draws until no pending frame / tickers.
	ModeTransient
	// ModePersistent keeps vsync/fallback ticks while tickers active.
	ModePersistent
)

// DefaultAnimTick is the fallback period when no VSyncWaiter (~60 Hz).
const DefaultAnimTick = 16 * time.Millisecond

// vsyncFreshWindow is how recent a vsync signal must be to count as
// driving frame pacing. Signals arrive every vsync (~16.7ms at 60Hz); a
// window of two ticks absorbs jitter while a stale timestamp (vsync
// stopped) falls out and pacing drops to the software interval.
const vsyncFreshWindow = 2 * DefaultAnimTick

// FrameScheduler decides wait timeouts and tracks schedule demand.
type FrameScheduler struct {
	mu sync.Mutex

	mode     Mode
	pending  atomic.Bool
	tickers  TickerRegistry
	metrics  MetricsStore
	animTick time.Duration
	lastTick time.Time
	// lastFrameAt is when the last frame was actually rendered (FrameDue
	// software pacing). Guarded by mu.
	lastFrameAt time.Time

	// Vsync callback model (Flutter VsyncWaiter semantics): a listener
	// goroutine waits on WaitVSync and stamps lastVSync on success — the
	// frame loop never blocks on vsync itself. If the vblank source hangs,
	// the listener goroutine stays blocked once and lastVSync goes stale,
	// which automatically falls pacing back to the software interval (no
	// timeout/latch bookkeeping needed).
	vsyncOnce sync.Once
	vsyncMu   sync.Mutex
	lastVSync time.Time
}

// ensureVsyncListener starts the vsync listener goroutine once: it waits on
// WaitVSync and records the arrival timestamp (Flutter VsyncWaiter callback
// model — the frame loop never blocks on vsync). WaitVSync error is an
// unavailable-vsync signal: counted as a miss and retried at the software
// interval. A hang leaves this single goroutine blocked and the timestamp
// stale, which drops pacing to the software interval automatically.
func (s *FrameScheduler) ensureVsyncListener(host platform.Host) {
	if s == nil {
		return
	}
	s.vsyncOnce.Do(func() {
		v := platform.HostVSync(host)
		if v == nil {
			return
		}
		go func() {
			for {
				if err := v.WaitVSync(); err == nil {
					s.vsyncMu.Lock()
					s.lastVSync = time.Now()
					s.vsyncMu.Unlock()
					s.ScheduleFrame()
					continue
				}
				s.metrics.NoteMissedVSync()
				s.mu.Lock()
				d := s.animTick
				s.mu.Unlock()
				time.Sleep(d)
			}
		}()
	})
}

// vsyncFresh reports whether a vsync signal arrived recently enough to drive
// frame pacing.
func (s *FrameScheduler) vsyncFresh() bool {
	if s == nil {
		return false
	}
	s.vsyncMu.Lock()
	last := s.lastVSync
	s.vsyncMu.Unlock()
	return !last.IsZero() && time.Since(last) <= vsyncFreshWindow
}

// FrameDue is the non-blocking frame-pacing gate (Flutter frame callback
// semantics): a frame may render when a fresh vsync signal arrived, or when
// the software interval (animTick) has elapsed since the last rendered
// frame. It never blocks; the caller skips rendering otherwise. The frame
// timestamp advances only when the gate opens.
func (s *FrameScheduler) FrameDue() bool {
	if s == nil {
		return false
	}
	now := time.Now()
	s.vsyncMu.Lock()
	lastV := s.lastVSync
	s.vsyncMu.Unlock()
	if !lastV.IsZero() && now.Sub(lastV) <= vsyncFreshWindow {
		s.mu.Lock()
		s.lastFrameAt = now
		s.mu.Unlock()
		return true
	}
	s.mu.Lock()
	due := s.lastFrameAt.IsZero() || now.Sub(s.lastFrameAt) >= s.animTick
	if due {
		s.lastFrameAt = now
	}
	s.mu.Unlock()
	return due
}

// New creates a FrameScheduler with default anim tick.
func New() *FrameScheduler {
	return &FrameScheduler{
		mode:     ModeIdle,
		animTick: DefaultAnimTick,
	}
}

// Metrics returns the metrics store.
func (s *FrameScheduler) Metrics() *MetricsStore {
	if s == nil {
		return nil
	}
	return &s.metrics
}

// Tickers returns the ticker registry.
func (s *FrameScheduler) Tickers() *TickerRegistry {
	if s == nil {
		return nil
	}
	return &s.tickers
}

// SetAnimTick overrides fallback frame period.
func (s *FrameScheduler) SetAnimTick(d time.Duration) {
	if s == nil || d <= 0 {
		return
	}
	s.mu.Lock()
	s.animTick = d
	s.mu.Unlock()
}

// ScheduleFrame requests a frame (wakes IDLE).
func (s *FrameScheduler) ScheduleFrame() {
	if s == nil {
		return
	}
	s.pending.Store(true)
}

// ClearPending clears the frame-needed flag after a successful frame path.
func (s *FrameScheduler) ClearPending() {
	if s == nil {
		return
	}
	s.pending.Store(false)
}

// Pending reports whether a frame was requested.
func (s *FrameScheduler) Pending() bool {
	return s != nil && s.pending.Load()
}

// SetMode sets the demand mode.
func (s *FrameScheduler) SetMode(m Mode) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.mode = m
	s.mu.Unlock()
}

// Mode returns the current mode.
func (s *FrameScheduler) Mode() Mode {
	if s == nil {
		return ModeIdle
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode
}

// RecomputeMode updates mode from tickers + pending (call after events/ticks).
func (s *FrameScheduler) RecomputeMode() {
	if s == nil {
		return
	}
	if s.tickers.HasActive() {
		s.SetMode(ModePersistent)
		return
	}
	if s.Pending() {
		s.SetMode(ModeTransient)
		return
	}
	s.SetMode(ModeIdle)
}

// WaitTimeout returns how long the host should WaitEvents.
// IDLE → -1 (infinite); animating → animTick; transient with pending → 0 poll after wake.
func (s *FrameScheduler) WaitTimeout() time.Duration {
	if s == nil {
		return -1
	}
	s.RecomputeMode()
	switch s.Mode() {
	case ModePersistent:
		s.mu.Lock()
		d := s.animTick
		s.mu.Unlock()
		return d
	case ModeTransient:
		if s.Pending() {
			return 0
		}
		return -1
	default:
		return -1
	}
}

// WaitFramePace runs the non-blocking pacing bookkeeping before a frame.
// Call after WaitEvents when ModePersistent / need steady cadence.
//
// Flutter VsyncWaiter model: vsync waits happen on the listener goroutine
// (ensureVsyncListener); the frame loop itself never blocks on vsync. This
// call starts the listener (once) and reports vsync honesty for metrics —
// it does NOT sleep and does NOT wait. Actual frame gating is FrameDue
// (fresh vsync signal or software interval elapsed), so a hung vblank cannot
// freeze the loop: pacing silently falls back to the software interval.
//
// When a fresh vsync signal exists, metrics vsync_source is "true"; when the
// waiter is missing or the timestamp went stale, it is "fallback"
// (missed_vsync is counted by the listener on WaitVSync error).
func (s *FrameScheduler) WaitFramePace(host platform.Host) {
	if s == nil {
		return
	}
	// The listener owns the (potentially blocking) WaitVSync.
	s.ensureVsyncListener(host)
	if s.Mode() != ModePersistent && !s.Pending() {
		return
	}
	if s.vsyncFresh() {
		s.metrics.SetVSyncSource("true")
	} else {
		s.metrics.SetVSyncSource("fallback")
	}
	// Non-blocking: frame pacing is gated by FrameDue at the render point.
}

// Tick advances tickers with clamped dt; returns whether any remain.
func (s *FrameScheduler) Tick() bool {
	if s == nil {
		return false
	}
	now := time.Now()
	dt := 1.0 / 60.0
	s.mu.Lock()
	if !s.lastTick.IsZero() {
		dt = now.Sub(s.lastTick).Seconds()
	}
	s.lastTick = now
	s.mu.Unlock()
	if dt > 0.066 {
		dt = 0.066
	}
	if dt < 0 {
		dt = 0
	}
	s.tickers.TickAll(dt)
	return s.tickers.HasActive()
}
