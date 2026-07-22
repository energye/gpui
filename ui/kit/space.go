package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Space is Ant Design Space: Flex gap between children.
// https://ant.design/components/space
type Space struct {
	Root *primitive.Flex
	// Direction: horizontal (default) or vertical.
	Vertical bool
	// Size gap in logical px (0 → theme TokenMarginSM / 8).
	Size  float64
	Theme *core.Theme
}

// NewSpace lays out children with uniform gap.
func NewSpace(children ...core.Node) *Space {
	s := &Space{}
	if len(children) > 0 {
		s.Root = primitive.Row(children...)
	} else {
		s.Root = primitive.Row()
	}
	s.Root.Gap = 8
	s.Root.CrossAlign = core.CrossCenter
	return s
}

// Node returns the flex root.
func (s *Space) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		s.Root = primitive.Row()
	}
	s.apply()
	return s.Root
}

// SetVertical switches to column layout.
func (s *Space) SetVertical(v bool) {
	s.Vertical = v
	s.apply()
}

// SetSize sets gap.
func (s *Space) SetSize(px float64) {
	s.Size = px
	s.apply()
}

// Add appends a child.
func (s *Space) Add(n core.Node) {
	if s.Root == nil {
		s.Root = primitive.Row()
	}
	if n != nil {
		s.Root.AddChild(n)
	}
}

func (s *Space) apply() {
	if s == nil || s.Root == nil {
		return
	}
	gap := s.Size
	if gap <= 0 {
		th := s.Theme
		if th == nil {
			th = DefaultTheme()
		}
		gap = th.SizeOr(core.TokenMarginSM, 8)
	}
	s.Root.Gap = gap
	if s.Vertical {
		s.Root.Axis = core.AxisVertical
		s.Root.CrossAlign = core.CrossStart
	} else {
		s.Root.Axis = core.AxisHorizontal
		s.Root.CrossAlign = core.CrossCenter
	}
}
