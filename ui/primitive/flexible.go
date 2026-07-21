package primitive

import "github.com/energye/gpui/ui/core"

// Flexible wraps a child with a flex grow factor for Flex parents.
type Flexible struct {
	core.NodeBase

	Grow   float64
	Shrink float64
}

// NewFlexible wraps child with the given grow factor (default shrink=1).
func NewFlexible(grow float64, child core.Node) *Flexible {
	if grow < 0 {
		grow = 0
	}
	f := &Flexible{Grow: grow, Shrink: 1}
	f.Init(f)
	f.Hit = core.HitDefer
	if child != nil {
		f.AddChild(child)
	}
	return f
}

// Spacer is a Flexible with grow=1 and no child (expands to fill free space).
func Spacer() *Flexible {
	return NewFlexible(1, nil)
}

// TypeID implements core.Node.
func (f *Flexible) TypeID() string { return TypeFlexible }

// FlexGrow implements core.FlexFactorNode.
func (f *Flexible) FlexGrow() float64 { return f.Grow }

// FlexShrink implements core.FlexFactorNode.
func (f *Flexible) FlexShrink() float64 { return f.Shrink }

// Layout implements core.Node.
func (f *Flexible) Layout(c core.Constraints) core.Size {
	kids := f.Children()
	if len(kids) == 0 {
		// Empty flexible / spacer: take minimum of constraints (0 unless tight).
		out := c.Tighten(core.Size{})
		// When given a tight main size from Flex, honor it.
		if c.MinWidth > 0 || c.MinHeight > 0 {
			out = core.Size{Width: c.MinWidth, Height: c.MinHeight}
			if c.MaxWidth < core.Unbounded && c.MaxWidth > out.Width {
				out.Width = c.MaxWidth
			}
			if c.MaxHeight < core.Unbounded && c.MaxHeight > out.Height {
				out.Height = c.MaxHeight
			}
			// Prefer tight axes.
			if c.MinWidth == c.MaxWidth {
				out.Width = c.MinWidth
			}
			if c.MinHeight == c.MaxHeight {
				out.Height = c.MinHeight
			}
		}
		f.SetSize(out)
		return out
	}
	child := kids[0]
	sz := child.Layout(c.Expand())
	// If parent assigned a tight size, expand child to fill.
	if c.IsTight() {
		sz = child.Layout(c)
	} else {
		// Expand to min constraints when larger.
		if c.MinWidth > sz.Width {
			sz.Width = c.MinWidth
		}
		if c.MinHeight > sz.Height {
			sz.Height = c.MinHeight
		}
		if c.MinWidth == c.MaxWidth {
			sz.Width = c.MinWidth
		}
		if c.MinHeight == c.MaxHeight {
			sz.Height = c.MinHeight
		}
	}
	child.Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	// When Flex assigns exact main size via Min==Max, use it.
	if c.MinWidth == c.MaxWidth {
		out.Width = c.MinWidth
	}
	if c.MinHeight == c.MaxHeight {
		out.Height = c.MinHeight
	}
	f.SetSize(out)
	return out
}

// Paint implements core.Node.
func (f *Flexible) Paint(pc *core.PaintContext) { f.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (f *Flexible) HitTest(p core.Point) core.Node { return f.DefaultHitTest(p) }
