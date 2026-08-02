// Package wrkit provides shared real-window helpers for ui_wr_* quality bars
// (LiveHUD, phase clock, run-duration helpers). HUD is example-local only —
// it must not introduce ui→gpu imports and must not be used to close engine holes.
package wrkit

import (
	"fmt"
	"os"
	"strconv"

	"github.com/energye/gpui/ui/rendering"
)

// Phase names for U17 phase scripts (Steady / Spike / Recover).
const (
	PhaseSteady  = "Steady"
	PhaseSpike   = "Spike"
	PhaseRecover = "Recover"
)

// PhaseClock maps elapsed seconds into Steady → Spike → Recover.
// Defaults fit a 5s close window; longer runs stretch proportionally only when
// total > SteadyEnd+SpikeEnd budget (callers pass absolute boundaries).
type PhaseClock struct {
	// SteadyEnd is exclusive end of Steady (seconds from start).
	SteadyEnd float64
	// SpikeEnd is exclusive end of Spike; after that → Recover.
	SpikeEnd float64
	elapsed  float64
	name     string
}

// NewPhaseClock builds a clock with absolute phase boundaries in seconds.
// Zero values fall back to 0–1.5 Steady, 1.5–3.5 Spike, then Recover.
func NewPhaseClock(steadyEnd, spikeEnd float64) *PhaseClock {
	if steadyEnd <= 0 {
		steadyEnd = 1.5
	}
	if spikeEnd <= steadyEnd {
		spikeEnd = steadyEnd + 2
	}
	return &PhaseClock{SteadyEnd: steadyEnd, SpikeEnd: spikeEnd, name: PhaseSteady}
}

// Advance adds dt and returns the current phase name.
func (p *PhaseClock) Advance(dt float64) string {
	if p == nil {
		return PhaseSteady
	}
	p.elapsed += dt
	switch {
	case p.elapsed < p.SteadyEnd:
		p.name = PhaseSteady
	case p.elapsed < p.SpikeEnd:
		p.name = PhaseSpike
	default:
		p.name = PhaseRecover
	}
	return p.name
}

// Name returns the last computed phase (or Steady if never advanced).
func (p *PhaseClock) Name() string {
	if p == nil || p.name == "" {
		return PhaseSteady
	}
	return p.name
}

// Elapsed returns seconds since start.
func (p *PhaseClock) Elapsed() float64 {
	if p == nil {
		return 0
	}
	return p.elapsed
}

// RunSeconds reads RUN_SECONDS or returns def. Invalid/empty → def.
func RunSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// RunSecondsOpt returns (secs, set): RUN_SECONDS set & valid → (n, true);
// unset/invalid → (0, false). When set=false the example runs indefinitely
// and only closes on user action (window X / close event).
func RunSecondsOpt() (int, bool) {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// RequireMinRun fails (stderr + exit 1) when secs < 5 (U16).
func RequireMinRun(secs int, abilityID string) {
	if secs < 5 {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close %s (U16)\n", abilityID)
		os.Exit(1)
	}
}

// HUDEnabled is true unless WR_HUD=0 (CI may still leave HUD on by default).
func HUDEnabled() bool {
	return os.Getenv("WR_HUD") != "0"
}

// Snap is one LiveHUD frame of metrics (filled by the example from Metrics/app).
type Snap struct {
	AbilityID   string
	Phase       string
	FPS         float64 // prefer fps_interval
	P95Ms       float64
	Policy      string
	PresentMode string
	PaintCount  int64
	Presents    int64
	// Core is ability-specific counters shown on line 2 (e.g. "skip=12 rr=3").
	Core string
	// GateOK is a human preview only; end-of-run JSON gates still decide PASS.
	GateOK bool
	// Extra optional third line.
	Extra string
}

// LiveHUD is a bottom-band, non-RepaintBoundary node that refreshes live metrics
// mid-run (U18). Place on the root AbsoluteBox at y = winH - Height.
//
// Cost discipline: Update throttles MarkNeedsPaint to ~10 Hz (or on phase/gate
// change). U18 allows "每帧或每 N 帧刷新" — full 60 Hz DrawString under FullPaint
// + dense static was measured to drag fps_interval under the 55 gate on loaded hosts.
// Damage from HUD is acceptable under full_paint; retained windows should keep HUD
// in a dedicated band and prefer content-only damage.
type LiveHUD struct {
	Box    *rendering.RenderBox
	Width  float64
	Height float64
	last   Snap
	// minIntervalSec is the minimum time between MarkNeedsPaint (default 0.1s).
	minIntervalSec float64
	sincePaint     float64
	haveSnap       bool
}

// NewLiveHUD builds a fixed-size HUD band. Call Update each tick (cheap); paint
// dirties at most ~10×/s unless phase/gate flips.
func NewLiveHUD(width, height float64) *LiveHUD {
	if height <= 0 {
		height = 72
	}
	h := &LiveHUD{Width: width, Height: height, minIntervalSec: 0.1}
	box := rendering.NewRenderBox()
	box.FixedWidth = width
	box.FixedHeight = height
	// Not a RepaintBoundary: when dirty, HUD repaints with fresh numbers.
	box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		h.paint(pc, size)
	}
	h.Box = box
	return h
}

// Update stores the snap. Dirties the HUD at most every minIntervalSec, or
// immediately when Phase / GateOK changes (human-visible transitions).
func (h *LiveHUD) Update(s Snap) {
	if h == nil || h.Box == nil {
		return
	}
	// dt is not passed; approximate with fixed step from caller via AdvanceTime,
	// or use wall-less counter: bump sincePaint by assuming ~16ms if not set.
	// Callers that care should call NoteTick(dt) then Update.
	force := !h.haveSnap || s.Phase != h.last.Phase || s.GateOK != h.last.GateOK
	h.last = s
	h.haveSnap = true
	if force || h.sincePaint >= h.minIntervalSec {
		h.Box.MarkNeedsPaint()
		h.sincePaint = 0
	}
}

// NoteTick advances the HUD refresh budget. Call once per frame before Update.
func (h *LiveHUD) NoteTick(dt float64) {
	if h == nil {
		return
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.25 {
		dt = 0.25 // clamp spikes so one hitch doesn't skip many refreshes
	}
	h.sincePaint += dt
}

func (h *LiveHUD) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	w, ht := size.Width, size.Height
	if w <= 0 {
		w = h.Width
	}
	if ht <= 0 {
		ht = h.Height
	}
	// Semi-transparent dark band.
	pc.DC.SetRGBA(0.05, 0.06, 0.09, 0.88)
	pc.DC.DrawRectangle(pc.OriginX, pc.OriginY, w, ht)
	_ = pc.DC.Fill()
	// Top hairline.
	pc.DC.SetRGBA(0.35, 0.55, 0.75, 0.9)
	pc.DC.SetLineWidth(1)
	pc.DC.DrawLine(pc.OriginX, pc.OriginY+0.5, pc.OriginX+w, pc.OriginY+0.5)
	_ = pc.DC.Stroke()

	s := h.last
	gate := "FAIL?"
	gr, gg, gb := 0.95, 0.35, 0.3
	if s.GateOK {
		gate = "OK?"
		gr, gg, gb = 0.25, 0.85, 0.45
	}
	// Two short lines only — FullPaint repaints HUD every frame; keep DrawString cheap.
	line1 := fmt.Sprintf("%s %s fps=%.0f p95=%.0f %s", s.AbilityID, s.Phase, s.FPS, s.P95Ms, gate)
	pol := s.Policy
	if pol == "" {
		pol = "-"
	}
	line2 := fmt.Sprintf("%s mode=%s paint=%d n=%d %s", pol, s.PresentMode, s.PaintCount, s.Presents, s.Core)

	drawHUDText(pc, 10, 22, line1, 0.92, 0.94, 0.98)
	drawHUDText(pc, 10, 48, line2, gr, gg, gb)
}

func drawHUDText(pc *rendering.PaintContext, x, y float64, msg string, r, g, b float64) {
	if pc == nil || pc.DC == nil || msg == "" {
		return
	}
	// DrawString no-ops without Font — must SetFont every paint (DC may reset).
	if face := FaceAt(13); face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(r, g, b, 1)
	// DrawString is baseline-oriented; y is local to HUD band.
	pc.DC.DrawString(msg, pc.OriginX+x, pc.OriginY+y)
}
