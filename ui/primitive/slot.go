package primitive

import "github.com/energye/gpui/ui/core"

// Slot is a named child host for composition (prefix/suffix/tab-body/…).
//
// Flutter-style sizing:
//   - Empty slot → preferred size 0 (unless parent tight-forces an axis).
//   - With child → child's preferred size, optionally expanded when parent is tight.
//
// Never expand an empty slot to MaxWidth under loose constraints — that broke
// Input (prefix Slot ate the whole row and pushed placeholder off-screen).
type Slot struct {
	core.NodeBase
	Name string
	// ExpandFill when true expands to fill bounded max even under loose mins
	// (tab body host). Default false.
	//
	// Important: the child is re-laid out with tight min==max on each bounded
	// axis so descendants (e.g. kit.Flex justify) see real free space. Expanding
	// only the Slot shell left children at intrinsic width and broke space-between.
	ExpandFill bool
}

// NewSlot creates a named slot with an optional child.
func NewSlot(name string, child core.Node) *Slot {
	s := &Slot{Name: name}
	s.Init(s)
	s.Hit = core.HitDefer
	if child != nil {
		s.AddChild(child)
	}
	return s
}

// TypeID implements core.Node.
func (s *Slot) TypeID() string { return TypeSlot }

// SetChild replaces the single child.
func (s *Slot) SetChild(child core.Node) {
	s.ClearChildren()
	if child != nil {
		s.AddChild(child)
	}
}

// Child returns the first child or nil.
func (s *Slot) Child() core.Node {
	kids := s.Children()
	if len(kids) == 0 {
		return nil
	}
	return kids[0]
}

// Layout implements core.Node.
func (s *Slot) Layout(c core.Constraints) core.Size {
	kids := s.Children()
	if len(kids) == 0 {
		// Empty: size 0 unless parent tight-forces an axis (Flutter empty SizedBox).
		out := core.Size{}
		if c.MinWidth == c.MaxWidth && c.MaxWidth < core.Unbounded {
			out.Width = c.MaxWidth
		} else if c.MinWidth > 0 {
			out.Width = c.MinWidth
		}
		if c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded {
			out.Height = c.MaxHeight
		} else if c.MinHeight > 0 {
			out.Height = c.MinHeight
		}
		out = c.Tighten(out)
		s.SetSize(out)
		return out
	}

	// Child gets available max; tight axes stay tight.
	childC := core.Constraints{
		MaxWidth:  c.MaxWidth,
		MaxHeight: c.MaxHeight,
	}
	if c.MinWidth == c.MaxWidth && c.MaxWidth < core.Unbounded {
		childC.MinWidth, childC.MaxWidth = c.MinWidth, c.MaxWidth
	}
	if c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded {
		childC.MinHeight, childC.MaxHeight = c.MinHeight, c.MaxHeight
	}
	// ExpandFill: fill bounded axes and pass that tightness to the child so
	// product layout (Flex justify/align, wrap) uses the real content box.
	if s.ExpandFill {
		if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded {
			childC.MinWidth, childC.MaxWidth = c.MaxWidth, c.MaxWidth
		}
		// Height: only force when parent already tight (scroll viewport keeps
		// MaxHeight unbounded so content can grow and scroll).
		if c.MinHeight == c.MaxHeight && c.HasBoundedHeight() && c.MaxHeight < core.Unbounded {
			childC.MinHeight, childC.MaxHeight = c.MaxHeight, c.MaxHeight
		}
	}
	sz := kids[0].Layout(childC)
	kids[0].Base().SetOffset(core.Point{})

	out := c.Tighten(sz)
	// ExpandFill or tight parent → fill remaining space (tab panels).
	if s.ExpandFill || (c.MinWidth == c.MaxWidth && c.MaxWidth < core.Unbounded) {
		if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded && out.Width < c.MaxWidth {
			out.Width = c.MaxWidth
		}
	}
	if s.ExpandFill || (c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded) {
		if c.HasBoundedHeight() && c.MaxHeight < core.Unbounded && out.Height < c.MaxHeight {
			out.Height = c.MaxHeight
		}
	}
	s.SetSize(out)
	return out
}

// Paint implements core.Node.
func (s *Slot) Paint(pc *core.PaintContext) { s.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (s *Slot) HitTest(p core.Point) core.Node { return s.DefaultHitTest(p) }

// FindSlot walks n's children for a Slot with the given name.
func FindSlot(n core.Node, name string) *Slot {
	if n == nil {
		return nil
	}
	for _, c := range n.Children() {
		if s, ok := c.(*Slot); ok && s.Name == name {
			return s
		}
		if found := FindSlot(c, name); found != nil {
			return found
		}
	}
	return nil
}
