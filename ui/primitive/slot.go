package primitive

import "github.com/energye/gpui/ui/core"

// Slot is a named child host for composition (prefix/suffix/child/…).
type Slot struct {
	core.NodeBase
	Name string
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
		out := c.Tighten(core.Size{})
		s.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
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
