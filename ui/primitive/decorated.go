package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Decorated paints background, border, and optional rounded corners (C-Theme/C-Skin).
// Children are laid out inside padding. Prefer token keys when Theme is present.
//
// When the final height exceeds content+padding (MinHeight/Height force), the
// single content child is vertically centered — matches Ant control chrome
// (Button / Input / Select label alignment).
type Decorated struct {
	core.NodeBase

	Padding     EdgeInsets
	Radius      float64
	BorderWidth float64
	Background  render.RGBA
	BorderColor render.RGBA
	// BorderDash when non-empty draws a dashed border (Ant Button dashed).
	// Units are logical px, e.g. {3, 2}. Cleared after stroke.
	BorderDash []float64
	// Token keys (optional · override solid colors when Theme resolves them).
	BackgroundToken string
	BorderToken     string
	// Min size hints.
	MinWidth, MinHeight float64
	// Width/Height when > 0 force preferred size.
	Width, Height float64
	// CenterContent when true (default) vertically centers a single child when
	// the box is taller than content+padding. Set false for top-aligned multi-line.
	// Zero-value is true for Ant form-control defaults; set CenterContent=false
	// explicitly only when needed (use centerContentSet to track).
	CenterContent    bool
	centerContentSet bool
}

// SetCenterContent enables/disables vertical content centering.
func (d *Decorated) SetCenterContent(v bool) {
	d.CenterContent = v
	d.centerContentSet = true
}

func (d *Decorated) centerContentEnabled() bool {
	if d == nil {
		return true
	}
	if d.centerContentSet {
		return d.CenterContent
	}
	return true // Ant default: center labels in control chrome
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
	} else if len(kids) > 1 {
		for _, child := range kids {
			sz := child.Layout(inner.Expand())
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
	// Vertical center content when chrome is taller (Button/Input Ant alignment).
	offY := d.Padding.Top
	if d.centerContentEnabled() && len(kids) >= 1 {
		availH := out.Height - d.Padding.Top - d.Padding.Bottom
		if availH > content.Height {
			offY = d.Padding.Top + (availH-content.Height)/2
		}
	}
	for _, child := range kids {
		child.Base().SetOffset(core.Point{X: d.Padding.Left, Y: offY})
	}
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
		if len(d.BorderDash) > 0 && pc.DC != nil {
			pc.DC.SetDash(d.BorderDash...)
			pc.StrokeLocalRoundRect(0, 0, sz.Width, sz.Height, radius, borderW, bd)
			pc.DC.SetDash()
		} else {
			pc.StrokeLocalRoundRect(0, 0, sz.Width, sz.Height, radius, borderW, bd)
		}
	}
}

// HitTest implements core.Node.
func (d *Decorated) HitTest(p core.Point) core.Node { return d.DefaultHitTest(p) }
