package painting

import "github.com/energye/gpui/render"

// GradientStop is a linear-gradient color stop (offset in [0,1], RGBA 0..1).
type GradientStop struct {
	Offset     float64
	R, G, B, A float64
}

// FillLinearGradient fills a local logical rect with a linear gradient.
// (x0,y0)→(x1,y1) are gradient endpoints in the same local space as (x,y,w,h).
// At least two stops are recommended; zero stops is a no-op.
func (c *Context) FillLinearGradient(x, y, w, h, x0, y0, x1, y1 float64, stops ...GradientStop) {
	if c == nil || c.DC == nil || w <= 0 || h <= 0 || len(stops) == 0 {
		return
	}
	ax := c.OriginX + x
	ay := c.OriginY + y
	grad := render.NewLinearGradientBrush(
		c.OriginX+x0, c.OriginY+y0,
		c.OriginX+x1, c.OriginY+y1,
	)
	for _, s := range stops {
		a := s.A
		if a == 0 && (s.R != 0 || s.G != 0 || s.B != 0) {
			a = 1
		}
		grad.AddColorStop(s.Offset, render.RGBA{R: s.R, G: s.G, B: s.B, A: a})
	}
	c.DC.SetFillBrush(grad)
	c.DC.DrawRectangle(ax, ay, w, h)
	_ = c.DC.Fill()
}

// FillLinearGradient2 is a two-stop convenience for FillLinearGradient.
func (c *Context) FillLinearGradient2(x, y, w, h, x0, y0, x1, y1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	c.FillLinearGradient(x, y, w, h, x0, y0, x1, y1,
		GradientStop{Offset: 0, R: r0, G: g0, B: b0, A: a0},
		GradientStop{Offset: 1, R: r1, G: g1, B: b1, A: a1},
	)
}
