package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Divider is a geometric separator line (horizontal or vertical).
// Solid paints a filled hairline; when Dash is non-empty, paints a stroked
// dashed/dotted line (antd Divider variant).
type Divider struct {
	core.NodeBase

	// Vertical when true draws a vertical bar; default horizontal.
	Vertical bool
	// Thickness of the line (default 1).
	Thickness float64
	// Length along the main axis; 0 → expand to constraints.
	Length float64
	// Color of the line.
	Color render.RGBA
	// ColorToken optional theme key (e.g. colorSplit).
	ColorToken string
	// Margin around the line.
	Margin EdgeInsets
	// Dash when non-empty draws a dashed/dotted stroke instead of a solid fill.
	// Example: dashed ≈ {4, 4}, dotted ≈ {1, 3}.
	Dash []float64
}

// NewDivider creates a horizontal divider.
func NewDivider() *Divider {
	d := &Divider{
		Thickness: 1,
		Color:     render.RGBA{R: 0.85, G: 0.85, B: 0.85, A: 1},
	}
	d.Init(d)
	d.Hit = core.HitTransparent
	return d
}

// NewVerticalDivider creates a vertical divider.
func NewVerticalDivider() *Divider {
	d := NewDivider()
	d.Vertical = true
	return d
}

// TypeID implements core.Node.
func (d *Divider) TypeID() string { return TypeDivider }

// Layout implements core.Node.
func (d *Divider) Layout(c core.Constraints) core.Size {
	th := d.Thickness
	if th <= 0 {
		th = 1
	}
	var w, h float64
	if d.Vertical {
		w = th + d.Margin.Left + d.Margin.Right
		h = d.Length
		if h <= 0 {
			if c.HasBoundedHeight() {
				h = c.MaxHeight
			} else {
				h = 24
			}
		}
		h += d.Margin.Top + d.Margin.Bottom
	} else {
		h = th + d.Margin.Top + d.Margin.Bottom
		w = d.Length
		if w <= 0 {
			if c.HasBoundedWidth() {
				w = c.MaxWidth
			} else {
				w = 24
			}
		}
		w += d.Margin.Left + d.Margin.Right
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	d.SetSize(out)
	return out
}

// Paint implements core.Node.
func (d *Divider) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	col := d.Color
	if pc.Theme != nil {
		if d.ColorToken != "" {
			if c := pc.Theme.Color(d.ColorToken); c.A > 0 {
				col = c
			}
		} else if col.A == 0 {
			col = pc.Theme.Color(core.TokenColorBorder)
		}
	}
	if col.A <= 0 {
		return
	}
	th := d.Thickness
	if th <= 0 {
		th = 1
	}
	sz := d.Size()
	x := d.Margin.Left
	y := d.Margin.Top
	if d.Vertical {
		h := sz.Height - d.Margin.Top - d.Margin.Bottom
		if h <= 0 {
			return
		}
		if len(d.Dash) > 0 {
			d.strokeDashed(pc, x+th/2, y, x+th/2, y+h, th, col)
			return
		}
		pc.FillLocalRect(x, y, th, h, col)
		return
	}
	w := sz.Width - d.Margin.Left - d.Margin.Right
	if w <= 0 {
		return
	}
	if len(d.Dash) > 0 {
		d.strokeDashed(pc, x, y+th/2, x+w, y+th/2, th, col)
		return
	}
	pc.FillLocalRect(x, y, w, th, col)
}

func (d *Divider) strokeDashed(pc *core.PaintContext, x0, y0, x1, y1, th float64, col render.RGBA) {
	if pc.DC == nil {
		// Fallback: solid segment when no draw context (headless layout-only).
		if d.Vertical {
			pc.FillLocalRect(x0-th/2, y0, th, y1-y0, col)
		} else {
			pc.FillLocalRect(x0, y0-th/2, x1-x0, th, col)
		}
		return
	}
	pc.DC.SetDash(d.Dash...)
	pc.StrokeLocalLine(x0, y0, x1, y1, th, col)
	pc.DC.SetDash()
}

// HitTest implements core.Node.
func (d *Divider) HitTest(p core.Point) core.Node { return d.DefaultHitTest(p) }
