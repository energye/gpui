package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// Slider is Ant Design Slider (horizontal value Min..Max).
// https://ant.design/components/slider
type Slider struct {
	Root     *sliderHost
	Value    float64
	Min      float64
	Max      float64
	Step     float64
	Width    float64
	Face     text.Face
	Theme    *core.Theme
	OnChange func(v float64)
	dragging bool
}

// NewSlider creates a slider at value (default range 0..100).
func NewSlider(value float64) *Slider {
	s := &Slider{Value: value, Min: 0, Max: 100, Step: 1, Width: 200}
	s.rebuild()
	return s
}

// sliderHost paints track/thumb and maps pointer X → value.
type sliderHost struct {
	core.NodeBase
	*Slider
	width float64
}

// Node returns root.
func (s *Slider) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// SetValue sets value clamped to Min..Max (and stepped when Step>0).
func (s *Slider) SetValue(v float64) {
	min, max := s.Min, s.Max
	if max <= min {
		min, max = 0, 100
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	if s.Step > 0 {
		// snap to step from min
		steps := (v - min) / s.Step
		// round to nearest step
		if steps >= 0 {
			steps = float64(int(steps + 0.5))
		} else {
			steps = float64(int(steps - 0.5))
		}
		v = min + steps*s.Step
		if v < min {
			v = min
		}
		if v > max {
			v = max
		}
	}
	s.Value = v
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
	if s.OnChange != nil {
		s.OnChange(v)
	}
}

func (s *Slider) rebuild() {
	w := s.Width
	if w <= 0 {
		w = 200
	}
	if s.Max <= s.Min {
		s.Min, s.Max = 0, 100
	}
	if s.Step <= 0 {
		s.Step = 1
	}
	h := &sliderHost{Slider: s, width: w}
	h.Init(h)
	h.Hit = core.HitTarget
	s.Root = h
}

func (h *sliderHost) TypeID() string { return "kit.SliderHost" }

func (h *sliderHost) Layout(c core.Constraints) core.Size {
	w := h.width
	if w <= 0 {
		w = 200
	}
	out := c.Tighten(core.Size{Width: w, Height: 16})
	h.SetSize(out)
	return out
}

func (h *sliderHost) Paint(pc *core.PaintContext) {
	if pc == nil || h.Slider == nil {
		return
	}
	th := DefaultTheme()
	if h.Theme != nil {
		th = h.Theme
	}
	sz := h.Size()
	trackC := th.Color(core.TokenColorBorder)
	fillC := th.Color(core.TokenColorPrimary)
	if fillC.A < 0.5 {
		fillC = render.Hex("#1677FF")
	}
	pc.FillLocalRoundRect(0, sz.Height/2-2, sz.Width, 4, 2, trackC)
	min, max := h.Min, h.Max
	if max <= min {
		min, max = 0, 100
	}
	ratio := (h.Value - min) / (max - min)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	fw := sz.Width * ratio
	if fw > 0 {
		pc.FillLocalRoundRect(0, sz.Height/2-2, fw, 4, 2, fillC)
	}
	pc.FillLocalCircle(fw, sz.Height/2, 7, fillC)
}

func (h *sliderHost) HitTest(p core.Point) core.Node {
	if h.LocalBounds().Contains(p) {
		return h
	}
	return nil
}

func (h *sliderHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || ev == nil || h.Slider == nil {
		return
	}
	abs := core.AbsoluteBounds(h)
	lx := ev.X - abs.Min.X
	switch ev.Type {
	case core.PointerDown:
		h.dragging = true
		h.setFromX(lx)
		ev.Handled = true
	case core.PointerMove:
		if h.dragging {
			h.setFromX(lx)
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		h.dragging = false
		ev.Handled = true
	}
}

func (h *sliderHost) setFromX(lx float64) {
	w := h.Size().Width
	if w < 1 {
		w = h.width
	}
	min, max := h.Min, h.Max
	if max <= min {
		min, max = 0, 100
	}
	h.SetValue(min + lx/w*(max-min))
}
