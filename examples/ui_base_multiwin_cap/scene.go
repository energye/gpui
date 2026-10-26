// Scene for ui_base_multiwin_cap: one spinning pinwheel per window.
//
// All eight windows share the same deterministic content, so the CPU/window
// curve measures window count, not scene differences. The animation is a
// pure phase rotation: no per-frame allocation, no component state.
package main

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/rendering"
)

// winScene is the per-window animation state. Paint reads phase/active from
// atomics; the controller writes them from its own goroutine.
type winScene struct {
	idx    int
	active *atomic.Int32
	phase  atomic.Uint64
	frozen atomic.Bool
}

func newWinScene(idx int, active *atomic.Int32) *winScene {
	return &winScene{idx: idx, active: active}
}

func (s *winScene) advance(dt float64) {
	if s == nil || s.frozen.Load() || dt <= 0 {
		return
	}
	old := math.Float64frombits(s.phase.Load())
	s.phase.Store(math.Float64bits(old + dt*1.6))
}

// freeze pins phase 0 for a bitwise Golden snapshot.
func (s *winScene) freeze() {
	if s == nil {
		return
	}
	s.phase.Store(math.Float64bits(0))
	s.frozen.Store(true)
}

func (s *winScene) currentPhase() float64 {
	if s == nil {
		return 0
	}
	return math.Float64frombits(s.phase.Load())
}

var pinwheel = [4][3]float64{
	{0.95, 0.36, 0.30}, // coral
	{0.30, 0.75, 0.98}, // cyan
	{0.42, 0.90, 0.48}, // green
	{0.96, 0.82, 0.35}, // amber
}

// paint draws the window: dark ground + 4-blade pinwheel + floating metric
// line. The metric band (top 40px) and the W-index tag are excluded from the
// Golden mask only where marked in main; both are deterministic.
func (s *winScene) paint(pc *rendering.PaintContext, size rendering.Size, fps float64, presents int64) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc := pc.DC
	w, h := size.Width, size.Height
	if w <= 0 {
		w = winW
	}
	if h <= 0 {
		h = winH
	}
	dc.SetRGB(0.10, 0.12, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	phase := s.currentPhase()
	cx, cy := w/2, h/2
	for i := 0; i < 4; i++ {
		c := pinwheel[i]
		dc.SetRGBA(c[0], c[1], c[2], 1)
		dc.Push()
		dc.Translate(cx, cy)
		dc.Rotate(phase + float64(i)*math.Pi/2)
		dc.DrawRectangle(8, -6, 30, 12)
		_ = dc.Fill()
		dc.Pop()
	}
	dc.SetRGBA(0.92, 0.94, 0.98, 1)
	dc.DrawCircle(cx, cy, 9)
	_ = dc.Fill()

	if face := wrkit.FaceAt(13); face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(0.78, 0.88, 0.98, 1)
	state := "idle"
	active := int32(0)
	if s.active != nil {
		active = s.active.Load()
	}
	if s.idx < int(active) {
		state = "active"
	}
	dc.DrawString(fmt.Sprintf("MULTIWIN %d/%d %s n=%d fps=%.0f presents=%d",
		s.idx+1, maxWindows, state, active, fps, presents), 10, 22)
	// Per-window tag: makes every Golden file provably its own window.
	dc.SetRGBA(0.55, 0.62, 0.72, 1)
	dc.DrawString(fmt.Sprintf("W%d", s.idx+1), w-28, h-10)
}
