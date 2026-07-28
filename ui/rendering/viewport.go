package rendering

import "math"

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
//
// Optional Physics (FScroll-PHYSICS) clamps and drives Fling/TickPhysics ballistic
// motion. Nil Physics keeps the historical hard clamp with no fling.
type RenderViewport struct {
	Base
	scrollX, scrollY float64
	// maxScrollY optional clamp; if < 0, derived from content height when known.
	maxScrollY float64
	// Physics applies boundary + ballistic fling (nil = hard clamp only).
	Physics ScrollPhysics
	// ballistic is the active fling simulation (vertical); nil when idle.
	ballistic BallisticSimulation
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
// Applies Physics.AdjustPosition when set; otherwise hard-clamps to [0, maxScrollY].
// User-driven SetScrollOffset cancels an active ballistic fling.
func (v *RenderViewport) SetScrollOffset(x, y float64) {
	v.setScrollOffset(x, y, true)
}

func (v *RenderViewport) setScrollOffset(x, y float64, cancelBallistic bool) {
	if v == nil {
		return
	}
	if cancelBallistic {
		v.ballistic = nil
	}
	minY, maxY := 0.0, v.maxScrollY
	if maxY < 0 {
		// Unset max: still prevent negative; allow any positive until layout.
		if v.Physics != nil {
			y = v.Physics.AdjustPosition(y, 0, math.Inf(1))
		} else if y < 0 {
			y = 0
		}
	} else if v.Physics != nil {
		y = v.Physics.AdjustPosition(y, minY, maxY)
	} else {
		if y < 0 {
			y = 0
		}
		if y > maxY {
			y = maxY
		}
	}
	if x < 0 {
		x = 0
	}
	// X: hard clamp only (MVP vertical nesting).
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

// SetPhysics installs scroll physics (may be nil to restore hard clamp only).
func (v *RenderViewport) SetPhysics(p ScrollPhysics) {
	if v == nil {
		return
	}
	v.Physics = p
	v.ballistic = nil
	// Re-clamp current offset under new physics.
	v.setScrollOffset(v.scrollX, v.scrollY, true)
}

// Fling starts a vertical ballistic simulation from velocityY (px/s in scroll space).
// No-op without Physics or when CreateBallistic returns nil. Cancels prior fling.
func (v *RenderViewport) Fling(velocityY float64) {
	if v == nil || v.Physics == nil {
		return
	}
	maxY := v.maxScrollY
	if maxY < 0 {
		maxY = math.Inf(1)
	}
	v.ballistic = v.Physics.CreateBallistic(velocityY, v.scrollY, 0, maxY)
}

// TickPhysics advances an active fling by dt seconds.
// Returns true if a ballistic simulation is still running (caller should schedule frames).
func (v *RenderViewport) TickPhysics(dt float64) bool {
	if v == nil || v.ballistic == nil {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	pos, done := v.ballistic.Step(dt)
	v.setScrollOffset(v.scrollX, pos, false)
	if done {
		v.ballistic = nil
		return false
	}
	return true
}

// HasBallistic reports whether a fling is in progress.
func (v *RenderViewport) HasBallistic() bool {
	return v != nil && v.ballistic != nil
}

// ScrollToIndex scrolls so item index sits at the top of the viewport when content
// is a *VirtualList (fixed or variable extent). Uses VirtualList.ScrollOffsetForIndex
// (prefix-sum for variable rows — not index×fixedExtent). Returns false if content
// is not a VirtualList. Clamping uses the viewport max scroll after layout.
func (v *RenderViewport) ScrollToIndex(index int) bool {
	if v == nil {
		return false
	}
	list, ok := v.Content().(*VirtualList)
	if !ok || list == nil {
		return false
	}
	y := list.ScrollOffsetForIndex(index)
	v.SetScrollOffset(v.scrollX, y)
	return true
}

// SetMaxScrollY clamps vertical scroll; <0 means no explicit clamp (until Layout sets one).
func (v *RenderViewport) SetMaxScrollY(max float64) {
	if v == nil {
		return
	}
	v.maxScrollY = max
	if max >= 0 && v.scrollY > max {
		v.SetScrollOffset(v.scrollX, max)
	}
}

// MaxScrollY returns the vertical clamp. <0 means unset / unlimited for clamping purposes
// until Layout derives content-based max.
func (v *RenderViewport) MaxScrollY() float64 {
	if v == nil {
		return -1
	}
	return v.maxScrollY
}

// MaxScrollX is reserved; MVP vertical nesting only (always 0 clamp unless extended).
func (v *RenderViewport) MaxScrollX() float64 {
	return 0
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
func (v *RenderViewport) Paint(pc *PaintContext) {
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
		pushClipRect(pc, 0, 0, sz.Width, sz.Height)
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
		popClip(pc)
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
