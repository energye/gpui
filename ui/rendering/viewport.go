package rendering

import "github.com/energye/gpui/ui/painting"

// ScrollAware is implemented by content that must rebinding when the viewport scrolls
// (e.g. VirtualList). Called with content-space scroll offset and viewport height.
type ScrollAware interface {
	OnViewportScroll(offsetY, viewportH float64)
}

// RenderViewport clips painting to its bounds and applies ScrollOffset to its child.
//
// Scroll convention (Y-down):
//
//	ScrollOffset.Y increases → content moves up relative to the viewport
//	(user scrolls the list downward).
//
// SetScrollOffset / ScrollBy mark paint only (not layout), unless content's
// OnViewportScroll triggers a bind-window layout (VirtualList).
type RenderViewport struct {
	Base
	scrollX, scrollY float64
	// maxScrollY optional clamp; if < 0, derived from content height when known.
	maxScrollY float64
}

// NewRenderViewport creates a viewport with optional content child.
func NewRenderViewport(content RenderObject) *RenderViewport {
	v := &RenderViewport{maxScrollY: -1}
	v.Init(v)
	v.SetRelayoutBoundary(true)
	if content != nil {
		v.AddChild(content)
	}
	return v
}

// Content returns the first child or nil.
func (v *RenderViewport) Content() RenderObject {
	if v == nil || len(v.children) == 0 {
		return nil
	}
	return v.children[0]
}

// SetContent replaces the single content child.
func (v *RenderViewport) SetContent(c RenderObject) {
	if v == nil {
		return
	}
	for len(v.children) > 0 {
		v.RemoveChild(v.children[0])
	}
	if c != nil {
		v.AddChild(c)
	}
}

// ScrollOffset returns current scroll in logical pixels.
func (v *RenderViewport) ScrollOffset() Point {
	if v == nil {
		return Point{}
	}
	return Point{X: v.scrollX, Y: v.scrollY}
}

// SetScrollOffset sets scroll offset (paint-only path for viewport itself).
func (v *RenderViewport) SetScrollOffset(x, y float64) {
	if v == nil {
		return
	}
	if y < 0 {
		y = 0
	}
	if x < 0 {
		x = 0
	}
	if v.maxScrollY >= 0 && y > v.maxScrollY {
		y = v.maxScrollY
	}
	if x == v.scrollX && y == v.scrollY {
		return
	}
	v.scrollX, v.scrollY = x, y
	v.notifyContentScroll()
	v.MarkNeedsPaint()
}

// ScrollBy adds delta to scroll offset.
func (v *RenderViewport) ScrollBy(dx, dy float64) {
	if v == nil {
		return
	}
	v.SetScrollOffset(v.scrollX+dx, v.scrollY+dy)
}

// SetMaxScrollY clamps vertical scroll; <0 means no explicit clamp.
func (v *RenderViewport) SetMaxScrollY(max float64) {
	if v == nil {
		return
	}
	v.maxScrollY = max
	if max >= 0 && v.scrollY > max {
		v.SetScrollOffset(v.scrollX, max)
	}
}

func (v *RenderViewport) notifyContentScroll() {
	c := v.Content()
	if c == nil {
		return
	}
	if sa, ok := c.(ScrollAware); ok {
		sa.OnViewportScroll(v.scrollY, v.size.Height)
	}
}

// Layout implements RenderObject: viewport size from constraints; content laid out loosely.
func (v *RenderViewport) Layout(c Constraints) Size {
	if sz, ok := v.LayoutSkipIfClean(c); ok {
		return sz
	}
	out := c.Tighten(Size{Width: c.MaxWidth, Height: c.MaxHeight})
	if out.Width >= Unbounded/2 {
		out.Width = c.MinWidth
	}
	if out.Height >= Unbounded/2 {
		out.Height = c.MinHeight
	}
	v.setSize(out)
	if ch := v.Content(); ch != nil {
		// Content may be taller than viewport.
		inner := Constraints{
			MinWidth: out.Width, MaxWidth: out.Width,
			MinHeight: 0, MaxHeight: Unbounded,
		}
		csz := ch.Layout(inner)
		ch.SetOffset(Point{}) // paint applies scroll
		// Auto max scroll from content.
		maxY := csz.Height - out.Height
		if maxY < 0 {
			maxY = 0
		}
		v.maxScrollY = maxY
		if v.scrollY > maxY {
			v.scrollY = maxY
		}
		v.notifyContentScroll()
	}
	v.RememberConstraints(c)
	v.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
func (v *RenderViewport) Paint(pc *painting.Context) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !v.NeedsPaint() && !SubtreeNeedsPaint(v) {
		return
	}
	pc.NotePaintVisit()
	paintSelf := !pc.CompositeOnly || v.NeedsPaint()
	sz := v.size
	// Always clip when drawing content; scroll dirties the viewport so paintSelf is true.
	if ch := v.Content(); ch != nil && (paintSelf || ch.NeedsPaint() || SubtreeNeedsPaint(ch)) {
		pc.PushClipRect(0, 0, sz.Width, sz.Height)
		// Content origin: -scroll so positive scrollY moves content up.
		ox := pc.OriginX - v.scrollX
		oy := pc.OriginY - v.scrollY
		// When viewport is dirty from scroll, force content to repaint children:
		// temporarily clear CompositeOnly for content if paintSelf.
		childPC := pc.WithOrigin(ox, oy)
		if paintSelf {
			// Content must redraw at new offset even if its NeedsPaint was false.
			childPC.CompositeOnly = false
		}
		ch.Paint(childPC)
		pc.PopClip()
	}
	if paintSelf {
		v.clearPaintDirty()
	}
}

// HitTest implements RenderObject.
func (v *RenderViewport) HitTest(p Point) RenderObject {
	sz := v.size
	if p.X < 0 || p.Y < 0 || p.X >= sz.Width || p.Y >= sz.Height {
		return nil
	}
	if ch := v.Content(); ch != nil {
		// Transform into content space.
		cp := Point{X: p.X + v.scrollX, Y: p.Y + v.scrollY}
		if hit := ch.HitTest(cp); hit != nil {
			return hit
		}
	}
	return v
}
