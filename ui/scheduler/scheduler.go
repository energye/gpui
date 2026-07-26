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

// FrameScheduler decides wait timeouts and tracks schedule demand.
type FrameScheduler struct {
	mu sync.Mutex

	mode     Mode
	pending  atomic.Bool
	tickers  TickerRegistry
	metrics  MetricsStore
	animTick time.Duration
	lastTick time.Time
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

// WaitFramePace blocks for vsync or fallback tick when animating.
// Call after WaitEvents when ModePersistent / need steady cadence.
func (s *FrameScheduler) WaitFramePace(host platform.Host) {
	if s == nil {
		return
	}
	if s.Mode() != ModePersistent && !s.Pending() {
		return
	}
	if v := platform.HostVSync(host); v != nil {
		if err := v.WaitVSync(); err == nil {
			return
		}
		s.metrics.NoteMissedVSync()
	}
	s.mu.Lock()
	d := s.animTick
	s.mu.Unlock()
	time.Sleep(d)
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
