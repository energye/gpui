package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Decorated is a Flutter-style "box decoration" node:
//   - Decides its own size from constraints + Width/Height/Min*
//   - Passes capped constraints to children (children keep intrinsic size)
//   - Only StretchChild=true forces children to fill the inner box
//
// This matches Flutter RenderDecoratedBox / ConstrainedBox semantics:
// the decoration box is sized by the parent/own constraints; the child
// is laid out with those constraints but is not forcibly expanded unless
// explicitly requested (Align/Expand equivalent = StretchChild).
type Decorated struct {
	core.NodeBase

	Padding     EdgeInsets
	Radius      float64
	BorderWidth float64
	Background  render.RGBA
	BorderColor render.RGBA
	// BorderDash when non-empty draws a dashed border (e.g. Button dashed).
	BorderDash []float64
	// Token keys (optional).
	BackgroundToken string
	BorderToken     string
	// Min size hints (soft floor after preferred size).
	MinWidth, MinHeight float64
	// Width/Height when > 0 force preferred outer size (then tightened by parent c).
	Width, Height float64
	// StretchChild: child receives tight constraints equal to the inner box.
	// Use for tab hit hosts. Leave false for Switch track (thumb must stay 18×18).
	StretchChild bool
	// ExpandWidth: when parent MaxWidth is finite, outer width becomes MaxWidth
	// (Ant block Button / full-width bars). Works under loose min (CrossStretch
	// already tightens; ExpandWidth also covers non-stretch parents with a max).
	ExpandWidth bool
	// CenterContent: vertically center a single child when outer box is taller
	// (Ant control label alignment). Default false = top-left (Flutter Align.topLeft).
	// Opt in via SetCenterContent(true) for Button/Input/Select chrome only.
	CenterContent bool
}

// SetCenterContent enables/disables vertical content centering.
func (d *Decorated) SetCenterContent(v bool) {
	d.CenterContent = v
}

func (d *Decorated) centerContentEnabled() bool {
	if d == nil {
		return false
	}
	return d.CenterContent
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

// Layout implements core.Node (Flutter constraint model).
func (d *Decorated) Layout(c core.Constraints) core.Size {
	if sz, ok := d.LayoutSkipIfClean(c); ok {
		return sz
	}

	// 1) Preferred outer size from content, then force Width/Height/Min*.
	// Child max is capped by the *resulting* inner box so a 160-wide rail
	// never hands MaxWidth=800 to its bar (tab merge bug).
	padL, padT, padR, padB := d.Padding.Left, d.Padding.Top, d.Padding.Right, d.Padding.Bottom

	// Inner constraints from parent, then cap by forced outer size if set.
	inner := c.Deflate(padL, padT, padR, padB)
	if d.Width > 0 {
		iw := d.Width - padL - padR
		if iw < 0 {
			iw = 0
		}
		// Cap max; do not raise min unless StretchChild.
		if inner.MaxWidth > iw {
			inner.MaxWidth = iw
		}
		if inner.MinWidth > inner.MaxWidth {
			inner.MinWidth = inner.MaxWidth
		}
	}
	if d.Height > 0 {
		ih := d.Height - padT - padB
		if ih < 0 {
			ih = 0
		}
		if inner.MaxHeight > ih {
			inner.MaxHeight = ih
		}
		if inner.MinHeight > inner.MaxHeight {
			inner.MinHeight = inner.MaxHeight
		}
	}

	// Child constraints: loose (zero min) unless StretchChild.
	childC := core.Constraints{
		MaxWidth:  inner.MaxWidth,
		MaxHeight: inner.MaxHeight,
	}
	if d.StretchChild {
		// Fill inner box (tab host).
		if d.Width > 0 || (inner.MinWidth == inner.MaxWidth && inner.MaxWidth < core.Unbounded) {
			w := inner.MaxWidth
			if d.Width > 0 {
				w = d.Width - padL - padR
				if w < 0 {
					w = 0
				}
			}
			childC.MinWidth, childC.MaxWidth = w, w
		}
		if d.Height > 0 || (inner.MinHeight == inner.MaxHeight && inner.MaxHeight < core.Unbounded) {
			h := inner.MaxHeight
			if d.Height > 0 {
				h = d.Height - padT - padB
				if h < 0 {
					h = 0
				}
			}
			childC.MinHeight, childC.MaxHeight = h, h
		}
	}

	content := core.Size{}
	kids := d.Children()
	if len(kids) == 1 {
		content = kids[0].Layout(childC)
	} else if len(kids) > 1 {
		for _, child := range kids {
			sz := child.Layout(childC)
			content = core.MaxSize(content, sz)
		}
	}

	// Preferred outer size.
	w := content.Width + padL + padR
	h := content.Height + padT + padB
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
	if d.ExpandWidth && c.HasBoundedWidth() {
		w = c.MaxWidth
	}
	// Honor parent constraints.
	out := c.Tighten(core.Size{Width: w, Height: h})

	// Position children top-left (hit == paint == Flutter Align.topLeft).
	// CenterContent is opt-in and only when THIS Decorated requested a taller
	// chrome via Height/MinHeight — never because a parent tight Min forced out
	// taller (Tabs body / Flexible would otherwise center nested chrome mid-box).
	offY := padT
	explicitChromeH := d.Height > 0 || d.MinHeight > content.Height+padT+padB
	if !d.StretchChild && d.centerContentEnabled() && explicitChromeH && len(kids) >= 1 {
		availH := out.Height - padT - padB
		if availH > content.Height {
			offY = padT + (availH-content.Height)/2
		}
	}
	for _, child := range kids {
		child.Base().SetOffset(core.Point{X: padL, Y: offY})
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

// PaintDecorated is the single chrome path for Decorated.
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
