package primitive

import "github.com/energye/gpui/ui/core"

// Clip clips painting and hit-testing of children to its bounds (C-Paint / C-Hit).
type Clip struct {
	core.NodeBase
	// Width/Height when > 0 force clip size; else sizes to child.
	Width, Height float64
}

// NewClip wraps children in a clip region.
func NewClip(children ...core.Node) *Clip {
	c := &Clip{}
	c.Init(c)
	c.Hit = core.HitDefer
	c.ClipHit = true
	for _, ch := range children {
		c.AddChild(ch)
	}
	return c
}

// TypeID implements core.Node.
func (c *Clip) TypeID() string { return TypeClip }

// Layout implements core.Node.
func (c *Clip) Layout(cons core.Constraints) core.Size {
	childC := cons.Expand()
	if c.Width > 0 {
		childC = childC.WithMaxWidth(c.Width)
	}
	if c.Height > 0 {
		childC = childC.WithMaxHeight(c.Height)
	}
	content := core.Size{}
	kids := c.Children()
	if len(kids) > 0 {
		content = kids[0].Layout(childC)
		kids[0].Base().SetOffset(core.Point{})
	}
	w, h := content.Width, content.Height
	if c.Width > 0 {
		w = c.Width
	}
	if c.Height > 0 {
		h = c.Height
	}
	out := cons.Tighten(core.Size{Width: w, Height: h})
	c.SetSize(out)
	return out
}

// Paint implements core.Node.
func (c *Clip) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	sz := c.Size()
	pc.PushClipLocal(0, 0, sz.Width, sz.Height)
	c.DefaultPaintChildren(pc)
	pc.Pop()
}

// HitTest implements core.Node.
func (c *Clip) HitTest(p core.Point) core.Node {
	return c.DefaultHitTest(p)
}
