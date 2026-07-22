package primitive

import "github.com/energye/gpui/ui/core"

// PainterNode invokes a custom paint callback (C-Paint) then paints children.
type PainterNode struct {
	core.NodeBase

	// PaintFn draws chrome in local coordinates (Origin already applied on pc).
	PaintFn func(pc *core.PaintContext, size core.Size)
	// Width/Height preferred size when > 0.
	Width, Height float64
	Padding       EdgeInsets
}

// NewPainterNode creates a node with a custom paint callback.
func NewPainterNode(fn func(pc *core.PaintContext, size core.Size), children ...core.Node) *PainterNode {
	p := &PainterNode{PaintFn: fn}
	p.Init(p)
	p.Hit = core.HitDefer
	for _, c := range children {
		p.AddChild(c)
	}
	return p
}

// TypeID implements core.Node.
func (p *PainterNode) TypeID() string { return TypePainterNode }

// Layout implements core.Node.
func (p *PainterNode) Layout(c core.Constraints) core.Size {
	return layoutPaddedChild(&p.NodeBase, c, p.Padding, p.Width, p.Height)
}

// Paint implements core.Node.
func (p *PainterNode) Paint(pc *core.PaintContext) {
	if p.PaintFn != nil && pc != nil {
		p.PaintFn(pc, p.Size())
	}
	p.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (p *PainterNode) HitTest(pt core.Point) core.Node { return p.DefaultHitTest(pt) }
