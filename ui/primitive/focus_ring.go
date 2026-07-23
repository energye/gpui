package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// PaintFocusRing draws a keyboard focus ring just outside a control box.
//
// Ant Design-style: tight outline (≈1.5–2px), following control radius — not a
// large floating frame. Callers pass the control's corner radius.
func PaintFocusRing(pc *core.PaintContext, w, h, radius, outset, lineWidth float64) {
	if pc == nil || w <= 0 || h <= 0 {
		return
	}
	if outset <= 0 {
		outset = 1.5
	}
	if lineWidth <= 0 {
		lineWidth = 2.5
	}
	if radius < 0 {
		radius = 0
	}
	// Outer path radius ≈ control radius + outset so the ring hugs chrome.
	ringR := radius + outset
	col := render.RGBA{R: 0.09, G: 0.47, B: 1.0, A: 0.4}
	if pc.Theme != nil {
		if c := pc.Theme.Color(core.TokenColorPrimary); c.A > 0 {
			col = c
			col.A = 0.65
		}
		if c := pc.Theme.Color(core.TokenColorControlOutline); c.A > 0 {
			// Soft Ant-like outline (primary-tinted).
			col = render.RGBA{R: c.R, G: c.G, B: c.B, A: 0.7}
		}
	}
	pc.StrokeLocalRoundRect(-outset, -outset, w+2*outset, h+2*outset, ringR, lineWidth, col)
}
