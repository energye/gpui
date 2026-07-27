package rendering

// ItemBuilder creates a row RenderObject for a logical index.
type ItemBuilder func(index int) RenderObject

// VirtualList is a fixed-extent vertical list that only mounts children in the
// visible window (+ cache). It is ScrollAware for use under RenderViewport.
//
// Content height = ItemCount * ItemExtent (logical px).
type VirtualList struct {
	Base
	ItemCount   int
	ItemExtent  float64
	Builder     ItemBuilder
	CacheExtent float64 // extra logical px above/below viewport

	// Scroll window (updated via OnViewportScroll).
	scrollY   float64
	viewportH float64

	first int // inclusive
	last  int // exclusive

	// mounted maps index → child
	mounted map[int]RenderObject

	// BindCount is the number of currently mounted children (test metric).
	BindCount int
}

// NewVirtualList creates a virtual list. extent must be > 0.
func NewVirtualList(count int, extent float64, builder ItemBuilder) *VirtualList {
	if extent <= 0 {
		extent = 40
	}
	if count < 0 {
		count = 0
	}
	v := &VirtualList{
		ItemCount:   count,
		ItemExtent:  extent,
		Builder:     builder,
		CacheExtent: extent * 2,
		mounted:     make(map[int]RenderObject),
		last:        0,
	}
	v.Init(v)
	return v
}

// OnViewportScroll implements ScrollAware.
func (v *VirtualList) OnViewportScroll(offsetY, viewportH float64) {
	if v == nil {
		return
	}
	if offsetY < 0 {
		offsetY = 0
	}
	if viewportH < 0 {
		viewportH = 0
	}
	changed := offsetY != v.scrollY || viewportH != v.viewportH
	v.scrollY, v.viewportH = offsetY, viewportH
	if !changed && len(v.mounted) > 0 {
		return
	}
	if v.rebindWindow() {
		v.MarkNeedsLayout()
	} else {
		v.MarkNeedsPaint()
	}
}

// rebindWindow updates mounted children. Returns true if set of indices changed.
func (v *VirtualList) rebindWindow() bool {
	if v.ItemExtent <= 0 || v.ItemCount == 0 {
		return v.clearMounted()
	}
	vh := v.viewportH
	if vh <= 0 {
		// Do NOT use v.size.Height — that is content height (count×extent).
		// Until OnViewportScroll, bind a provisional page (~12 rows).
		vh = v.ItemExtent * 12
		if vh < 100 {
			vh = 400
		}
	}
	cache := v.CacheExtent
	if cache < 0 {
		cache = 0
	}
	startY := v.scrollY - cache
	if startY < 0 {
		startY = 0
	}
	endY := v.scrollY + vh + cache
	first := int(startY / v.ItemExtent)
	last := int(endY/v.ItemExtent) + 1
	if first < 0 {
		first = 0
	}
	if last > v.ItemCount {
		last = v.ItemCount
	}
	if first > last {
		first = last
	}
	if first == v.first && last == v.last && len(v.mounted) == last-first {
		v.BindCount = len(v.mounted)
		return false
	}
	// Unmount outside
	for idx, ch := range v.mounted {
		if idx < first || idx >= last {
			v.RemoveChild(ch)
			delete(v.mounted, idx)
		}
	}
	// Mount missing
	if v.Builder != nil {
		for idx := first; idx < last; idx++ {
			if _, ok := v.mounted[idx]; ok {
				continue
			}
			ch := v.Builder(idx)
			if ch == nil {
				continue
			}
			v.AddChild(ch)
			v.mounted[idx] = ch
		}
	}
	v.first, v.last = first, last
	v.BindCount = len(v.mounted)
	// Position children
	for idx, ch := range v.mounted {
		ch.SetOffset(Point{X: 0, Y: float64(idx) * v.ItemExtent})
	}
	return true
}

func (v *VirtualList) clearMounted() bool {
	if len(v.mounted) == 0 {
		return false
	}
	for idx, ch := range v.mounted {
		v.RemoveChild(ch)
		delete(v.mounted, idx)
	}
	v.first, v.last = 0, 0
	v.BindCount = 0
	return true
}

// ContentHeight returns total scrollable height.
func (v *VirtualList) ContentHeight() float64 {
	if v == nil {
		return 0
	}
	return float64(v.ItemCount) * v.ItemExtent
}

// Layout implements RenderObject.
func (v *VirtualList) Layout(c Constraints) Size {
	if sz, ok := v.LayoutSkipIfClean(c); ok {
		// Still ensure offsets if bind window set.
		return sz
	}
	w := c.MaxWidth
	if w >= Unbounded/2 {
		w = c.MinWidth
	}
	h := v.ContentHeight()
	out := c.Tighten(Size{Width: w, Height: h})
	v.setSize(out)
	// viewportH comes from OnViewportScroll (parent Viewport); do not infer from Unbounded max.
	v.rebindWindow()
	// Layout mounted children tightly to row size.
	rowC := Tight(out.Width, v.ItemExtent)
	for idx, ch := range v.mounted {
		_ = ch.Layout(rowC)
		ch.SetOffset(Point{X: 0, Y: float64(idx) * v.ItemExtent})
	}
	v.BindCount = len(v.mounted)
	v.RememberConstraints(c)
	v.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
func (v *VirtualList) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !v.NeedsPaint() && !SubtreeNeedsPaint(v) {
		return
	}
	pc.NotePaintVisit()
	paintSelf := !pc.CompositeOnly || v.NeedsPaint()
	for _, ch := range v.children {
		off := ch.Offset()
		if v.viewportH > 0 {
			top := off.Y
			bot := off.Y + v.ItemExtent
			visTop := v.scrollY
			visBot := v.scrollY + v.viewportH
			if bot < visTop || top > visBot {
				continue
			}
		}
		if paintSelf {
			// List itself dirty (e.g. scroll): redraw visible rows.
			if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
				continue
			}
			ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
			continue
		}
		// Only descendant dirty: walk dirty paths only.
		if ch.NeedsPaint() || SubtreeNeedsPaint(ch) {
			ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
		}
	}
	if paintSelf {
		v.clearPaintDirty()
	}
}

// HitTest implements RenderObject.
func (v *VirtualList) HitTest(p Point) RenderObject {
	for i := len(v.children) - 1; i >= 0; i-- {
		ch := v.children[i]
		off := ch.Offset()
		local := Point{X: p.X - off.X, Y: p.Y - off.Y}
		if hit := ch.HitTest(local); hit != nil {
			return hit
		}
	}
	sz := v.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return v
	}
	return nil
}
