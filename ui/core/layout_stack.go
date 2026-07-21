package core

// Alignment places a child within a Stack.
type Alignment int

const (
	AlignTopLeft Alignment = iota
	AlignTopCenter
	AlignTopRight
	AlignCenterLeft
	AlignCenter
	AlignCenterRight
	AlignBottomLeft
	AlignBottomCenter
	AlignBottomRight
)

// StackChild is an optional positioned child for Stack layout.
type StackChild interface {
	Node
	// StackAlignment returns alignment when not using absolute offset.
	StackAlignment() Alignment
	// StackOffset returns an optional absolute top-left offset.
	// If ok is true, offset is used instead of alignment.
	StackOffset() (offset Point, ok bool)
}

// LayoutStack sizes parent to max child (or tight constraints) and positions
// children by alignment or absolute offset.
func LayoutStack(parent *NodeBase, c Constraints) Size {
	kids := parent.children
	maxW, maxH := 0.0, 0.0

	// Measure each child with loose constraints.
	type measured struct {
		node Node
		base *NodeBase
		sz   Size
	}
	ms := make([]measured, len(kids))
	for i, child := range kids {
		childC := Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
		sz := child.Layout(childC)
		ms[i] = measured{child, child.Base(), sz}
		if sz.Width > maxW {
			maxW = sz.Width
		}
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}

	out := c.Tighten(Size{Width: maxW, Height: maxH})
	// If tight constraints, prefer them.
	if c.IsTight() {
		out = Size{Width: c.MaxWidth, Height: c.MaxHeight}
	}
	parent.SetSize(out)

	for _, m := range ms {
		var off Point
		if sc, ok := m.node.(StackChild); ok {
			if o, use := sc.StackOffset(); use {
				off = o
			} else {
				off = alignOffset(sc.StackAlignment(), out, m.sz)
			}
		} else {
			off = alignOffset(AlignTopLeft, out, m.sz)
		}
		m.base.SetOffset(off)
	}
	return out
}

func alignOffset(a Alignment, parent, child Size) Point {
	px, py := 0.0, 0.0
	switch a {
	case AlignTopCenter, AlignCenter, AlignBottomCenter:
		px = (parent.Width - child.Width) / 2
	case AlignTopRight, AlignCenterRight, AlignBottomRight:
		px = parent.Width - child.Width
	}
	switch a {
	case AlignCenterLeft, AlignCenter, AlignCenterRight:
		py = (parent.Height - child.Height) / 2
	case AlignBottomLeft, AlignBottomCenter, AlignBottomRight:
		py = parent.Height - child.Height
	}
	return Point{X: px, Y: py}
}
