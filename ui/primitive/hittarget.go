package primitive

import "github.com/energye/gpui/ui/core"

// HitTarget expands (or shrinks) the hit area around a child without affecting layout size.
// Expansion is in logical pixels outward from the child's laid-out bounds (C-Hit).
type HitTarget struct {
	core.NodeBase

	// Expand grows the hit rect on all sides (visual size stays child's size).
	Expand float64
	// MinWidth/MinHeight force a minimum hit size (centered on child).
	MinWidth, MinHeight float64
}

// NewHitTarget wraps a child with an expanded hit region.
func NewHitTarget(child core.Node) *HitTarget {
	h := &HitTarget{Expand: 4}
	h.Init(h)
	h.Hit = core.HitTarget
	if child != nil {
		h.AddChild(child)
	}
	return h
}

// TypeID implements core.Node.
func (h *HitTarget) TypeID() string { return TypeHitTarget }

// Layout implements core.Node — layout size equals child (not expanded).
func (h *HitTarget) Layout(c core.Constraints) core.Size {
	kids := h.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		h.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	h.SetSize(out)
	return out
}

// Paint implements core.Node.
func (h *HitTarget) Paint(pc *core.PaintContext) { h.DefaultPaintChildren(pc) }

// HitTest implements core.Node using expanded bounds.
func (h *HitTarget) HitTest(p core.Point) core.Node {
	sz := h.Size()
	// Expanded rect in local coords (negative origin allowed).
	ex := h.Expand
	minW, minH := h.MinWidth, h.MinHeight
	hitW, hitH := sz.Width+ex*2, sz.Height+ex*2
	if minW > hitW {
		hitW = minW
	}
	if minH > hitH {
		hitH = minH
	}
	ox := (sz.Width - hitW) / 2
	oy := (sz.Height - hitH) / 2
	r := core.NewRect(ox, oy, hitW, hitH)
	if !r.Contains(p) {
		return nil
	}
	// Prefer self as interactive shell; children paint only.
	return h
}
