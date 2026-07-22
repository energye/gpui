package primitive

import "github.com/energye/gpui/ui/core"

// Flex is a row/column layout container (C-Flex).
type Flex struct {
	core.NodeBase

	Axis       core.Axis
	MainAlign  core.MainAxisAlignment
	CrossAlign core.CrossAxisAlignment
	Gap        float64
	Padding    EdgeInsets
}

// NewFlex constructs a Flex along axis with optional children.
func NewFlex(axis core.Axis, children ...core.Node) *Flex {
	f := &Flex{Axis: axis, CrossAlign: core.CrossCenter}
	f.Init(f)
	f.Hit = core.HitDefer
	for _, c := range children {
		f.AddChild(c)
	}
	return f
}

// Row is NewFlex(AxisHorizontal, …).
func Row(children ...core.Node) *Flex { return NewFlex(core.AxisHorizontal, children...) }

// Column is NewFlex(AxisVertical, …).
func Column(children ...core.Node) *Flex { return NewFlex(core.AxisVertical, children...) }

// TypeID implements core.Node.
func (f *Flex) TypeID() string { return TypeFlex }

// Layout implements core.Node.
func (f *Flex) Layout(c core.Constraints) core.Size {
	if sz, ok := f.LayoutSkipIfClean(c); ok {
		return sz
	}
	inner := c.Deflate(f.Padding.Left, f.Padding.Top, f.Padding.Right, f.Padding.Bottom)
	// Temporarily layout children relative to content box; LayoutFlex writes offsets at 0,0.
	// We re-offset by padding after.
	sz := core.LayoutFlex(&f.NodeBase, inner, core.FlexLayoutParams{
		Axis:       f.Axis,
		MainAlign:  f.MainAlign,
		CrossAlign: f.CrossAlign,
		Gap:        f.Gap,
	})
	// Shift children by padding.
	if f.Padding.Left != 0 || f.Padding.Top != 0 {
		for _, child := range f.Children() {
			o := child.Base().Offset()
			child.Base().SetOffset(core.Point{X: o.X + f.Padding.Left, Y: o.Y + f.Padding.Top})
		}
	}
	out := core.Size{
		Width:  sz.Width + f.Padding.Left + f.Padding.Right,
		Height: sz.Height + f.Padding.Top + f.Padding.Bottom,
	}
	out = c.Tighten(out)
	f.SetSize(out)
	f.RememberConstraints(c)
	return out
}

// Paint implements core.Node.
func (f *Flex) Paint(pc *core.PaintContext) {
	f.DefaultPaintChildren(pc)
	if pc != nil {
		f.ClearPaintDirty()
	}
}

// HitTest implements core.Node.
func (f *Flex) HitTest(p core.Point) core.Node { return f.DefaultHitTest(p) }
