package primitive

import "github.com/energye/gpui/ui/core"

// Stack overlays children with alignment or absolute offsets (C-Stack).
type Stack struct {
	core.NodeBase
	// Fit expands to constraints when true (default); otherwise sizes to children.
	Fit bool
}

// positioned is an internal Stack child with alignment/offset.
type positioned struct {
	core.NodeBase
	align  core.Alignment
	offset core.Point
	useOff bool
}

func (p *positioned) TypeID() string { return "primitive.Positioned" }
func (p *positioned) Layout(c core.Constraints) core.Size {
	kids := p.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		p.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	p.SetSize(sz)
	return sz
}
func (p *positioned) Paint(pc *core.PaintContext)     { p.DefaultPaintChildren(pc) }
func (p *positioned) HitTest(pt core.Point) core.Node { return p.DefaultHitTest(pt) }
func (p *positioned) StackAlignment() core.Alignment  { return p.align }
func (p *positioned) StackOffset() (core.Point, bool) { return p.offset, p.useOff }

// SetStackOffset updates absolute offset used by LayoutStack (ink bar animation).
// Also writes NodeBase.Offset so DefaultPaintChildren sees the new position without
// a full tree layout pass (paint-only animation). Previously only StackOffset()
// was updated while paint still used the last LayoutStack offset — Tabs ink left
// a ghost of the previous selection until the next layout.
func (p *positioned) SetStackOffset(x, y float64) {
	p.offset = core.Point{X: x, Y: y}
	p.useOff = true
	p.SetOffset(p.offset)
	p.MarkNeedsPaint()
}

// NewStack constructs a Stack.
func NewStack(children ...core.Node) *Stack {
	s := &Stack{Fit: true}
	s.Init(s)
	s.Hit = core.HitDefer
	for _, c := range children {
		s.AddChild(c)
	}
	return s
}

// Positioned wraps child with stack alignment.
func Positioned(align core.Alignment, child core.Node) core.Node {
	p := &positioned{align: align}
	p.Init(p)
	p.Hit = core.HitDefer
	if child != nil {
		p.AddChild(child)
	}
	return p
}

// PositionedAt wraps child with an absolute offset in the stack.
func PositionedAt(x, y float64, child core.Node) core.Node {
	p := &positioned{offset: core.Point{X: x, Y: y}, useOff: true}
	p.Init(p)
	p.Hit = core.HitDefer
	if child != nil {
		p.AddChild(child)
	}
	return p
}

// TypeID implements core.Node.
func (s *Stack) TypeID() string { return TypeStack }

// Layout implements core.Node.
func (s *Stack) Layout(c core.Constraints) core.Size {
	if s.Fit && c.IsTight() {
		// Force children into tight size via stack algorithm with tight parent.
		return core.LayoutStack(&s.NodeBase, c)
	}
	return core.LayoutStack(&s.NodeBase, c)
}

// Paint implements core.Node.
func (s *Stack) Paint(pc *core.PaintContext) { s.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (s *Stack) HitTest(p core.Point) core.Node { return s.DefaultHitTest(p) }
