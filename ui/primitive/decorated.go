package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Decorated paints background, border, and optional rounded corners (C-Theme/C-Skin).
// Children are laid out inside padding. Prefer token keys when Theme is present.
type Decorated struct {
	core.NodeBase

	Padding     EdgeInsets
	Radius      float64
	BorderWidth float64
	Background  render.RGBA
	BorderColor render.RGBA
	// Token keys (optional · override solid colors when Theme resolves them).
	BackgroundToken string
	BorderToken     string
	// Min size hints.
	MinWidth, MinHeight float64
	// Width/Height when > 0 force preferred size.
	Width, Height float64
}

// NewDecorated wraps children with decoration.
func NewDecorated(children ...core.Node) *Decorated {
	d := &Decorated{Radius: 6, BorderWidth: 0}
	d.Init(d)
	d.Hit = core.HitDefer
	for _, c := range children {
		d.AddChild(c)
	}
	return d
}

// TypeID implements core.Node.
func (d *Decorated) TypeID() string { return TypeDecorated }

// Layout implements core.Node.
func (d *Decorated) Layout(c core.Constraints) core.Size {
	if sz, ok := d.LayoutSkipIfClean(c); ok {
		return sz
	}
	inner := c.Deflate(d.Padding.Left, d.Padding.Top, d.Padding.Right, d.Padding.Bottom)
	content := core.Size{}
	kids := d.Children()
	if len(kids) == 1 {
		content = kids[0].Layout(inner.Expand())
		kids[0].Base().SetOffset(core.Point{X: d.Padding.Left, Y: d.Padding.Top})
	} else if len(kids) > 1 {
		for _, child := range kids {
			sz := child.Layout(inner.Expand())
			child.Base().SetOffset(core.Point{X: d.Padding.Left, Y: d.Padding.Top})
			content = core.MaxSize(content, sz)
		}
	}
	w := content.Width + d.Padding.Left + d.Padding.Right
	h := content.Height + d.Padding.Top + d.Padding.Bottom
	if d.Width > 0 {
		w = d.Width
	}
	if d.Height > 0 {
		h = d.Height
	}
	if w < d.MinWidth {
		w = d.MinWidth
	}
	if h < d.MinHeight {
		h = d.MinHeight
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	d.SetSize(out)
	d.RememberConstraints(c)
	return out
}

// Paint implements core.Node.
func (d *Decorated) Paint(pc *core.PaintContext) {
	paintChrome := pc == nil || !pc.CompositeOnly || d.NeedsPaint() || pc.ForceFullPaint
	if paintChrome && pc != nil {
		var p core.Painter
		if pc.Theme != nil {
			p = pc.Theme.Painter(TypeDecorated)
		}
		if p != nil {
			p(pc, d)
		} else {
			PaintDecorated(pc, d)
		}
	}
	d.DefaultPaintChildren(pc)
	if pc != nil {
		d.ClearPaintDirty()
	}
}

// PaintDecorated is the single chrome path for Decorated: resolve optional
// color tokens, then fill → stroke (border-box via PaintContext helpers).
// Used by the node default path and skin/default so fill/stroke order and
// radius/border handling never diverge.
func PaintDecorated(pc *core.PaintContext, d *Decorated) {
	if pc == nil || d == nil {
		return
	}
	sz := d.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	bg := d.Background
	bd := d.BorderColor
	if pc.Theme != nil {
		if d.BackgroundToken != "" {
			if c := pc.Theme.Color(d.BackgroundToken); c.A > 0 {
				bg = c
			}
		}
		if d.BorderToken != "" {
			if c := pc.Theme.Color(d.BorderToken); c.A > 0 {
				bd = c
			}
		}
	}
	radius := d.Radius
	borderW := d.BorderWidth
	if bg.A > 0 {
		pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, radius, bg)
	}
	if borderW > 0 && bd.A > 0 {
		pc.StrokeLocalRoundRect(0, 0, sz.Width, sz.Height, radius, borderW, bd)
	}
}

// HitTest implements core.Node.
func (d *Decorated) HitTest(p core.Point) core.Node { return d.DefaultHitTest(p) }
