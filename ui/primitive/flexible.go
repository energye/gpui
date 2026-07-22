package primitive

import "github.com/energye/gpui/ui/core"

// Flexible is Flutter Expanded/Flexible with Align.topLeft by default:
//
//	parent Flex allocates free MAIN-axis space → tight Min==Max on that axis
//	cross-axis stays loose unless CrossStretch (also tight) or FillChild
//	child offset is always (0,0) top-left
//
// CRITICAL: do NOT size to loose MaxWidth/MaxHeight. A Spacer in a Row used to
// take MaxHeight from the Column (hundreds of px), inflate the Row, then
// CrossCenter pushed sibling Buttons mid-box (Modal footer bug).
// Flutter Expanded only expands the main axis the parent made tight.
type Flexible struct {
	core.NodeBase

	Grow   float64
	Shrink float64
	// FillChild: pass tight constraints so the child fills the allocation
	// (Input editor fills control height). Default false = top-left Align.
	FillChild bool
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

// Spacer is a Flexible with grow=1 and no child.
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
	if sz, ok := f.LayoutSkipIfClean(c); ok {
		return sz
	}

	// Only TIGHT axes come from flex allocation / CrossStretch.
	// Loose Max must not expand the node (Spacer in Row, Expanded cross-axis).
	tightW := c.MinWidth == c.MaxWidth && c.MaxWidth < core.Unbounded
	tightH := c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded

	selfW, selfH := c.MinWidth, c.MinHeight
	if tightW {
		selfW = c.MaxWidth
	}
	if tightH {
		selfH = c.MaxHeight
	}

	kids := f.Children()
	if len(kids) == 0 {
		// Spacer / empty: size is only what parent made tight (+ mins).
		out := c.Tighten(core.Size{Width: selfW, Height: selfH})
		f.SetSize(out)
		f.RememberConstraints(c)
		return out
	}

	// Child constraints: max = allocation; tight when parent axis is tight or FillChild.
	childC := core.Constraints{
		MaxWidth:  c.MaxWidth,
		MaxHeight: c.MaxHeight,
	}
	// Child: max = allocation on tight axes; min=0 unless FillChild (Align.topLeft).
	// Never use loose Max as expand size.
	if tightW {
		w := c.MaxWidth
		if w < 0 {
			w = 0
		}
		childC.MaxWidth = w
		if f.FillChild {
			childC.MinWidth = w
		}
	}
	if tightH {
		h := c.MaxHeight
		if h < 0 {
			h = 0
		}
		childC.MaxHeight = h
		if f.FillChild {
			childC.MinHeight = h
		}
	}
	// Cap max to parent max when not tight
	if !tightW && childC.MaxWidth > c.MaxWidth {
		childC.MaxWidth = c.MaxWidth
	}
	if !tightH && childC.MaxHeight > c.MaxHeight {
		childC.MaxHeight = c.MaxHeight
	}

	csz := kids[0].Layout(childC)
	kids[0].Base().SetOffset(core.Point{}) // top-left: paint Origin == hit offset

	// Non-tight axes adopt child size (Flutter Flexible cross-axis behavior).
	// NEVER adopt loose Max — that inflated Spacer in Modal footer / Button mid-box.
	if !tightW {
		selfW = csz.Width
		if selfW < c.MinWidth {
			selfW = c.MinWidth
		}
	}
	if !tightH {
		selfH = csz.Height
		if selfH < c.MinHeight {
			selfH = c.MinHeight
		}
	}

	out := c.Tighten(core.Size{Width: selfW, Height: selfH})
	f.SetSize(out)
	f.RememberConstraints(c)
	return out
}

// Paint implements core.Node.
func (f *Flexible) Paint(pc *core.PaintContext) { f.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (f *Flexible) HitTest(p core.Point) core.Node { return f.DefaultHitTest(p) }
