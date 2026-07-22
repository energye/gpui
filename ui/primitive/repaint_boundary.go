package primitive

import (
	"github.com/energye/gpui/ui/core"
)

// RepaintBoundary isolates paint dirty bubbling (Flutter RepaintBoundary).
//
// Children that MarkNeedsPaint only dirty this boundary (and schedule a tree
// frame); ancestors stay paint-clean so composite-only frames can skip them.
//
// Paint always draws children into the current PaintContext (vector path).
// Hosts that want GPU layer caching can wrap additional logic; this primitive
// provides the dirty isolation contract required by Flutter-aligned demand paint.
type RepaintBoundary struct {
	core.NodeBase
}

// NewRepaintBoundary wraps a child (or empty) as a paint isolation root.
func NewRepaintBoundary(children ...core.Node) *RepaintBoundary {
	b := &RepaintBoundary{}
	b.Init(b)
	b.Hit = core.HitDefer
	b.SetRepaintBoundary(true)
	for _, c := range children {
		b.AddChild(c)
	}
	return b
}

// TypeID implements core.Node.
func (b *RepaintBoundary) TypeID() string { return TypeRepaintBoundary }

// Layout implements core.Node — pass-through to single child or max of children.
func (b *RepaintBoundary) Layout(c core.Constraints) core.Size {
	if sz, ok := b.LayoutSkipIfClean(c); ok {
		return sz
	}
	kids := b.Children()
	content := core.Size{}
	if len(kids) == 1 {
		content = kids[0].Layout(c.Expand())
		kids[0].Base().SetOffset(core.Point{})
	} else if len(kids) > 1 {
		for _, child := range kids {
			sz := child.Layout(c.Expand())
			child.Base().SetOffset(core.Point{})
			content = core.MaxSize(content, sz)
		}
	}
	out := c.Tighten(content)
	b.SetSize(out)
	b.RememberConstraints(c)
	return out
}

// Paint implements core.Node.
// Always paints children when this boundary is dirty or the frame is a full paint.
// On composite-only clean frames, still paints children once so the first cache
// exists; subsequent clean composite frames re-paint children only if dirty
// (vector path has no retained texture — host may upgrade later).
func (b *RepaintBoundary) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	// When composite-only and clean: skip re-recording (pixels retained on surface).
	if pc.CompositeOnly && !b.NeedsPaint() && !pc.ForceFullPaint {
		b.ClearPaintDirty()
		return
	}
	// Force children to paint when the boundary itself is dirty.
	childPC := pc
	if b.NeedsPaint() || pc.ForceFullPaint {
		childPC = pc.WithForceFullPaint()
		// Restore CompositeOnly false for children under force.
		childPC.CompositeOnly = false
	}
	for _, c := range b.Children() {
		cb := c.Base()
		c.Paint(childPC.WithOrigin(pc.Origin.Add(cb.Offset())))
	}
	b.ClearPaintDirty()
}

// HitTest implements core.Node.
func (b *RepaintBoundary) HitTest(p core.Point) core.Node { return b.DefaultHitTest(p) }
