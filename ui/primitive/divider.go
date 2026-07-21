package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Divider is a geometric separator line (horizontal or vertical).
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
	// ColorToken optional theme key.
	ColorToken string
	// Margin around the line.
	Margin EdgeInsets
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
	th := d.Thickness
	if th <= 0 {
		th = 1
	}
	sz := d.Size()
	if d.Vertical {
		x := d.Margin.Left
		y := d.Margin.Top
		h := sz.Height - d.Margin.Top - d.Margin.Bottom
		pc.FillLocalRect(x, y, th, h, col)
	} else {
		x := d.Margin.Left
		y := d.Margin.Top
		w := sz.Width - d.Margin.Left - d.Margin.Right
		pc.FillLocalRect(x, y, w, th, col)
	}
}

// HitTest implements core.Node.
func (d *Divider) HitTest(p core.Point) core.Node { return d.DefaultHitTest(p) }
