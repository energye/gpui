package scheduler

import (
	"sort"
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

// DefaultAnimTick is the fallback period when no display timing is known
// (~60 Hz nominal). It is only a last-resort floor: the scheduler prefers,
// in order, (1) the learned display period from vsync stamps, (2) the
// display refresh reported by the platform host (see
// platform.DisplayRefreshReporter), and only then this constant. Never
// hardcode a measured machine-specific period here — every display has its
// own real refresh (59.88Hz, 60Hz, 120Hz, ...).
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
	// lastStampAt / lastStampInterval track vsync arrival cadence (all
	// stamping paths go through stampVSyncLocked). FrameDue uses the interval
	// to tell a fast vsync source (stamps closer than animTick — e.g. a
	// 120Hz vblank, which must pace once per stamp) from a slow/batched one
	// (compositor notices arriving slower than the software interval, which
	// the software floor already covers — firing per stamp would over-render).
	lastStampAt       time.Time
	lastStampInterval time.Duration
	// displayPeriod is the learned display refresh interval (EMA of stamp
	// intervals). Wayland/X11 software-boundary pacing uses it instead of
	// the hardcoded DefaultAnimTick so the UI commit cadence matches the
	// actual display (e.g. 16.7ms at 60Hz) — free-running faster than the
	// display causes periodic skipped refreshes (judder, worse at higher
	// animation speeds where each skipped slot doubles the visible step).
	displayPeriod time.Duration
	stampCount    int
	// basePeriod is the platform-reported display refresh interval (see
	// SeedDisplayRefreshHz). Pacing baseline until learned displayPeriod
	// trims it. 0 = host reported nothing, fall back to animTick.
	basePeriod time.Duration
	// dispHist* learn the display period from compositor frame-presented
	// notices. Only steady signals move the estimate (see
	// learnCompositorPeriod); jittered batches are ignored.
	dispHist      [dispHistSize]time.Duration
	dispHistIdx   int
	dispHistCount int
}

// Compositor-period estimator tuning (E6-A).
const (
	// dispHistSize is the recent-interval window for the median.
	dispHistSize = 32
	// dispHistMin is the minimum samples before the estimate may move
	// (avoids startup poisoning from the first few notices).
	dispHistMin = 16
)

// boundaryPeriodLocked returns the software-boundary interval: learned
// display period first, then the platform-reported baseline, then
// DefaultAnimTick. Clamped to [baseline*0.8, 100ms]; the floor follows the
// known baseline. Caller holds s.mu.
func (s *FrameScheduler) boundaryPeriodLocked() time.Duration {
	ref := s.animTick
	if s.basePeriod > 0 {
		ref = s.basePeriod
	}
	p := s.displayPeriod
	if p <= 0 {
		p = ref
	}
	return clampPeriod(p, ref)
}

// clampPeriod keeps a pacing interval inside [ref*0.8, 100ms] (floor 4ms)
// so a bogus source cannot stall frames.
func clampPeriod(p, ref time.Duration) time.Duration {
	min := ref * 8 / 10
	if min < 4*time.Millisecond {
		min = 4 * time.Millisecond
	}
	if p < min {
		p = min
	}
	if p > 100*time.Millisecond {
		p = 100 * time.Millisecond
	}
	return p
}

// stampVSyncLocked records a vsync arrival with its interval. Caller holds
// vsyncMu. Compositor intervals also feed learnCompositorPeriod; the DRM
// path additionally feeds the fast hardware EMA in learnDisplayPeriod.
func (s *FrameScheduler) stampVSyncLocked(now time.Time) {
	if !s.lastStampAt.IsZero() {
		iv := now.Sub(s.lastStampAt)
		s.lastStampInterval = iv
		s.learnCompositorPeriod(iv)
	}
	s.lastStampAt = now
	s.lastVSync = now
}

// learnCompositorPeriod feeds one compositor-notice interval into the
// display-period estimate. It takes s.mu (caller holds vsyncMu; same order
// as FrameDue, so no inversion).
//
// Only steady signals move the estimate: window median in [8ms,25ms] with
// (p90-p10) within max(2ms, 20% of median); then displayPeriod eases toward
// it (α=1/16). Jittered batches are ignored, so the estimate can never be
// poisoned into a stall.
func (s *FrameScheduler) learnCompositorPeriod(iv time.Duration) {
	if iv < 8*time.Millisecond || iv > 100*time.Millisecond {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dispHist[s.dispHistIdx] = iv
	s.dispHistIdx = (s.dispHistIdx + 1) % dispHistSize
	if s.dispHistCount < dispHistSize {
		s.dispHistCount++
	}
	if s.dispHistCount < dispHistMin {
		return
	}
	var tmp []time.Duration
	if s.dispHistCount < dispHistSize {
		tmp = append([]time.Duration(nil), s.dispHist[:s.dispHistCount]...)
	} else {
		tmp = append([]time.Duration(nil), s.dispHist[:]...)
	}
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
	med := tmp[len(tmp)/2]
	p10 := tmp[len(tmp)/10]
	p90 := tmp[len(tmp)*9/10]
	tol := med / 5
	if tol < 2*time.Millisecond {
		tol = 2 * time.Millisecond
	}
	if med < 8*time.Millisecond || med > 25*time.Millisecond {
		// Above 25ms means batched delivery or a 30Hz display; following
		// it would halve the animation rate, so keep the old behavior.
		return
	}
	if p90-p10 > tol {
		return
	}
	if s.displayPeriod <= 0 {
		s.displayPeriod = med
	} else {
		s.displayPeriod += (med - s.displayPeriod) / 16 // EMA α=1/16
	}
}

// learnDisplayPeriod feeds a hardware vblank interval into the display-period
// estimate used by the software boundary (boundaryPeriodLocked). Only the
// DRM vblank listener calls this — compositor frame-done notices arrive at
// the compositor's mercy (batched/delayed) and would poison the estimate.
func (s *FrameScheduler) learnDisplayPeriod(iv time.Duration) {
	if iv <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if iv >= 8*time.Millisecond && iv <= 100*time.Millisecond {
		if s.displayPeriod <= 0 {
			s.displayPeriod = iv
		} else {
			s.displayPeriod += (iv - s.displayPeriod) / 8 // EMA α=1/8
		}
	}
}

// ensureVsyncListener starts the vsync listener goroutine once: it waits on
// WaitVSync and records the arrival timestamp (Flutter VsyncWaiter callback
// model — the frame loop never blocks on vsync). WaitVSync error is an
// unavailable-vsync signal: counted as a miss and retried at the software
// interval. A hang leaves this single goroutine blocked and the timestamp
// stale, which drops pacing to the software interval automatically.
//
// On-demand rendering (ENGINE_FRAME_PRESENT_STANDARD.md 块1/块2): the
// listener only stamps the pacing timestamp — it never schedules frames.
// Render demand comes exclusively from events/tickers (ScheduleFrame).
// A Host implementing FrameNotifier (compositor "frame presented" notice)
// replaces this client-side waiter entirely: single pacing source, and the
// notice only arrives after a commit, so idle costs nothing.
func (s *FrameScheduler) ensureVsyncListener(host platform.Host) {
	if s == nil {
		return
	}
	s.vsyncOnce.Do(func() {
		if platform.HostFrameNotifier(host) != nil {
			return // compositor-driven pacing; no client-side waiter
		}
		v := platform.HostVSync(host)
		if v == nil {
			return
		}
		go func() {
			var last time.Time
			for {
				if err := v.WaitVSync(); err == nil {
					now := time.Now()
					s.vsyncMu.Lock()
					if !last.IsZero() {
						s.learnDisplayPeriod(now.Sub(last))
					}
					s.stampVSyncLocked(now)
					s.vsyncMu.Unlock()
					last = now
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

// NoteFramePresented records a compositor "frame presented" notice as a
// fresh pacing timestamp (ENGINE_FRAME_PRESENT_STANDARD.md 块2). Like the
// DRM listener stamp it does NOT schedule a frame — on-demand rendering
// keeps demand exclusively with events/tickers; this only opens the
// FrameDue gate for the next demanded frame.
func (s *FrameScheduler) NoteFramePresented() {
	if s == nil {
		return
	}
	s.vsyncMu.Lock()
	s.stampVSyncLocked(time.Now())
	s.vsyncMu.Unlock()
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

// nextFrameBoundaryLocked is the wall time the next frame may render: the
// software interval (animTick) after the last rendered frame. Fast vsync
// sources (stamps closer than animTick) pace via the per-stamp path in
// FrameDue instead. Caller must hold s.mu.
func (s *FrameScheduler) nextFrameBoundaryLocked() time.Time {
	return s.lastFrameAt.Add(s.boundaryPeriodLocked())
}

// FrameDue is the non-blocking frame-pacing gate (Flutter frame callback
// semantics): a frame may render when a fresh vsync signal arrived since the
// last rendered frame, or when the software interval (animTick) has elapsed.
// It never blocks; the caller skips rendering otherwise. The frame timestamp
// advances only when the gate opens.
//
// The software floor is unconditional (not gated on the vsync signal being
// stale): compositor frame-done notices can arrive much slower than the
// display refresh (observed ~2×-delayed wl_surface.frame callbacks), and a
// once-per-stamp gate alone would then throttle animation below the software
// interval. A fresh stamp still opens the gate immediately (fast vsync
// sources — e.g. 120Hz DRM vblank — pace at their own rate).
func (s *FrameScheduler) FrameDue() bool {
	if s == nil {
		return false
	}
	now := time.Now()
	s.vsyncMu.Lock()
	defer s.vsyncMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastFrameAt.IsZero() {
		// First frame renders unconditionally (bootstraps the pacing clock).
		s.lastFrameAt = now
		return true
	}
	if !s.lastVSync.IsZero() && now.Sub(s.lastVSync) <= vsyncFreshWindow {
		// Fast vsync source: stamps arriving closer than the software
		// interval (e.g. a 120Hz vblank) pace once per stamp (Flutter
		// frame-callback semantics). Interval 0 is the first stamp — no
		// cadence known yet, treat it as a fresh boundary.
		if !s.lastStampAt.IsZero() && s.lastStampInterval <= s.animTick &&
			s.lastFrameAt.Before(s.lastVSync) {
			s.lastFrameAt = now
			return true
		}
	}
	// Software interval pacing (also the floor while a fresh-stamp render is
	// not due yet): the loop targets this boundary via WaitTimeout, so the
	// gate opens on the deadline even when the loop only re-checks on events.
	// Phase-lock: advance by exactly one period (not snap to now) when the
	// overshoot is less than a period. Snapping to now accumulates every
	// WaitEvents oversleep (~0.1ms) into a random walk with positive bias
	// (avg +0.1ms/frame → a missed vsync every ~2.5s at 0.9s/rev animation).
	// Major stalls (>= period) resync to now to avoid spiral.
	if boundary := s.nextFrameBoundaryLocked(); !now.Before(boundary) {
		if period := s.boundaryPeriodLocked(); period > 0 && now.Sub(boundary) < period {
			s.lastFrameAt = boundary
		} else {
			s.lastFrameAt = now
		}
		return true
	}
	return false
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

// FrameWanted reports whether the last Tick saw frame demand from any
// surviving ticker (see FrameWanter). Tickers that do not implement
// FrameWanter count as wanting a frame, preserving the legacy
// stay-registered-means-frames behavior.
func (s *FrameScheduler) FrameWanted() bool {
	if s == nil {
		return false
	}
	return s.tickers.FrameWanted()
}

// NextWake reports the minimum ticker deadline from the last Tick (see
// DeadlineWanter). False when no surviving ticker reported one.
func (s *FrameScheduler) NextWake() (time.Duration, bool) {
	if s == nil {
		return 0, false
	}
	return s.tickers.NextWake()
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

// SeedDisplayRefreshHz sets the pacing baseline from the display's real
// refresh rate. 20–240Hz only; 0/unknown keeps the nominal floor. The
// learned displayPeriod keeps trimming on top. Safe to re-call on
// output/mode changes.
func (s *FrameScheduler) SeedDisplayRefreshHz(hz float64) {
	if s == nil || hz < 20 || hz > 240 {
		return
	}
	s.mu.Lock()
	s.basePeriod = time.Duration(float64(time.Second) / hz)
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
// IDLE → -1 (infinite); persistent → time until the next frame boundary
// (≤animTick), so the event loop wakes exactly at the pacing deadline
// instead of quantizing it to event-arrival boundaries; transient with
// pending → 0 poll after wake.
func (s *FrameScheduler) WaitTimeout() time.Duration {
	if s == nil {
		return -1
	}
	s.RecomputeMode()
	// Quiet tickers (blink phase, HUD budgets) must not hold the pacing
	// cadence: with no frame demand and nothing pending, sleep until the
	// earliest ticker deadline (events always interrupt). Tickers without
	// a deadline sleep until an event; legacy wanters keep the 16ms path.
	if !s.Pending() && !s.tickers.MayWantFrames() {
		if d, ok := s.tickers.NextWake(); ok {
			if d < 0 {
				d = 0
			}
			return d
		}
		return -1
	}
	switch s.Mode() {
	case ModePersistent:
		s.vsyncMu.Lock()
		defer s.vsyncMu.Unlock()
		s.mu.Lock()
		defer s.mu.Unlock()
		d := s.animTick
		if p := s.boundaryPeriodLocked(); s.lastFrameAt.IsZero() || p < 4*time.Millisecond {
			// no learned period yet — keep the animTick ceiling
		} else if p < d {
			d = p
		}
		if !s.lastFrameAt.IsZero() {
			switch left := time.Until(s.nextFrameBoundaryLocked()); {
			case left > 0 && left < d:
				d = left
			case left <= 0:
				// Boundary already passed (a slow iteration overran it): do
				// not sleep the full interval again — poll and let FrameDue
				// open on the overdue deadline.
				d = 0
			}
		}
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

// Tick advances tickers with the display-period dt (boundaryPeriodLocked),
// not the wall gap: wall-gap dt turns one late wake into a doubled step.
// A missed slot keeps the same step and lands on the next boundary.
func (s *FrameScheduler) Tick() bool {
	if s == nil {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	dt := s.boundaryPeriodLocked().Seconds()
	s.lastTick = now
	s.mu.Unlock()
	s.tickers.TickAll(dt)
	return s.tickers.HasActive()
}
