package rendering

import "sync"

// ItemBuilder creates a row RenderObject for a logical index.
type ItemBuilder func(index int) RenderObject

// ItemExtentFunc returns the logical height of row index (must be > 0).
// Used for variable-height virtualization (FScroll-VAR-EXTENT).
type ItemExtentFunc func(index int) float64

// VirtualList is a vertical list that only mounts children in the visible
// window (+ cache). It is ScrollAware for use under RenderViewport.
//
// Fixed path: ItemExtentAt == nil → ContentHeight = ItemCount * ItemExtent.
// Variable path: ItemExtentAt provides per-index heights via a prefix-sum cache
// (does not mount all rows to measure).
type VirtualList struct {
	Base
	ItemCount  int
	ItemExtent float64 // fixed-row height, or fallback when ItemExtentAt returns ≤0
	// ItemExtentAt when non-nil enables variable-height mode (P2).
	ItemExtentAt ItemExtentFunc
	Builder      ItemBuilder
	CacheExtent  float64 // extra logical px above/below viewport

	// Scroll window (updated via OnViewportScroll).
	scrollY   float64
	viewportH float64

	first int // inclusive
	last  int // exclusive

	// mounted maps index → child. Guarded by mu (see below).
	mounted map[int]RenderObject

	// mu protects mounted (and the children slice mutated via AddChild/RemoveChild
	// during rebind) against concurrent read/write. The raster loop reads
	// mounted/children in Paint/Layout while a ticker goroutine may trigger
	// OnViewportScroll → rebindWindow which mutates the same map/slice. Without
	// serialization this panics with "concurrent map iteration and map write".
	mu sync.Mutex

	// BindCount is the number of currently mounted children (test metric).
	BindCount int

	// prefix[i] = Y offset of item i; prefix[ItemCount] = total content height.
	// Only used when ItemExtentAt != nil.
	prefix      []float64
	prefixValid bool
}

// NewVirtualList creates a fixed-extent virtual list. extent must be > 0.
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

// NewVariableVirtualList creates a variable-height list.
// extentAt must return > 0 for each index; fallbackExtent is used if it returns ≤0
// and as the default CacheExtent basis.
func NewVariableVirtualList(count int, fallbackExtent float64, extentAt ItemExtentFunc, builder ItemBuilder) *VirtualList {
	v := NewVirtualList(count, fallbackExtent, builder)
	v.ItemExtentAt = extentAt
	v.prefixValid = false
	return v
}

// SetItemExtentAt enables or disables variable-height mode and invalidates caches.
func (v *VirtualList) SetItemExtentAt(fn ItemExtentFunc) {
	if v == nil {
		return
	}
	v.ItemExtentAt = fn
	v.InvalidateExtents()
}

// InvalidateExtents drops the prefix cache (call after extentAt results change).
func (v *VirtualList) InvalidateExtents() {
	if v == nil {
		return
	}
	v.prefixValid = false
	v.prefix = nil
	v.MarkNeedsLayout()
}

func (v *VirtualList) variable() bool {
	return v != nil && v.ItemExtentAt != nil
}

func (v *VirtualList) extentAt(i int) float64 {
	if v.ItemExtentAt != nil {
		if e := v.ItemExtentAt(i); e > 0 {
			return e
		}
	}
	if v.ItemExtent > 0 {
		return v.ItemExtent
	}
	return 40
}

func (v *VirtualList) ensurePrefix() {
	if !v.variable() {
		return
	}
	n := v.ItemCount
	if n < 0 {
		n = 0
	}
	if v.prefixValid && len(v.prefix) == n+1 {
		return
	}
	pref := make([]float64, n+1)
	for i := 0; i < n; i++ {
		pref[i+1] = pref[i] + v.extentAt(i)
	}
	v.prefix = pref
	v.prefixValid = true
}

// offsetOf returns the Y origin of item index (0-based).
func (v *VirtualList) offsetOf(index int) float64 {
	if !v.variable() {
		if index < 0 {
			return 0
		}
		ext := v.ItemExtent
		if ext <= 0 {
			ext = 40
		}
		return float64(index) * ext
	}
	v.ensurePrefix()
	if index < 0 {
		return 0
	}
	if index >= len(v.prefix) {
		return v.prefix[len(v.prefix)-1]
	}
	return v.prefix[index]
}

// OffsetOfIndex returns the content-space Y of the top of item index.
// Variable lists use the same prefix-sum extents as layout (not index×fixedExtent).
// Indices are clamped: <0 → 0; ≥ItemCount → total content height (end sentinel).
func (v *VirtualList) OffsetOfIndex(index int) float64 {
	if v == nil {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= v.ItemCount {
		return v.ContentHeight()
	}
	return v.offsetOf(index)
}

// IndexAtOffset returns the item index whose vertical span contains content Y
// (offset[i] ≤ y < offset[i]+extent[i]). Clamped to [0, ItemCount-1].
// Uses the same prefix (variable) or fixed-extent arithmetic as rebind.
func (v *VirtualList) IndexAtOffset(y float64) int {
	if v == nil || v.ItemCount <= 0 {
		return 0
	}
	if !v.variable() {
		ext := v.ItemExtent
		if ext <= 0 {
			ext = 40
		}
		if y <= 0 {
			return 0
		}
		idx := int(y / ext)
		if idx >= v.ItemCount {
			return v.ItemCount - 1
		}
		return idx
	}
	return v.indexContaining(y)
}

// ScrollOffsetForIndex returns the scrollY that places item index at the top of
// the viewport window (content offset of that row). Callers typically pass this
// to RenderViewport.SetScrollOffset / ScrollToIndex. Clamped to valid indices.
func (v *VirtualList) ScrollOffsetForIndex(index int) float64 {
	if v == nil || v.ItemCount <= 0 {
		return 0
	}
	if index < 0 {
		index = 0
	}
	if index >= v.ItemCount {
		index = v.ItemCount - 1
	}
	return v.offsetOf(index)
}

// BoundRange returns the currently mounted index window [first, lastExclusive).
// Empty list or no bind → (0, 0).
func (v *VirtualList) BoundRange() (first, lastExclusive int) {
	if v == nil {
		return 0, 0
	}
	return v.first, v.last
}

// indexContaining returns the item index whose vertical span contains y
// (prefix[i] <= y < prefix[i+1]). Clamped to [0, count-1].
func (v *VirtualList) indexContaining(y float64) int {
	v.ensurePrefix()
	n := v.ItemCount
	if n <= 0 {
		return 0
	}
	if y <= 0 {
		return 0
	}
	total := v.prefix[n]
	if y >= total {
		return n - 1
	}
	lo, hi := 0, n-1
	for lo < hi {
		mid := (lo + hi) / 2
		if v.prefix[mid+1] <= y {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// indexAtOrAfter returns the smallest index with offset >= y (exclusive end helper).
func (v *VirtualList) indexAtOrAfter(y float64) int {
	v.ensurePrefix()
	n := v.ItemCount
	if n <= 0 {
		return 0
	}
	if y <= 0 {
		return 0
	}
	if y >= v.prefix[n] {
		return n
	}
	lo, hi := 0, n
	for lo < hi {
		mid := (lo + hi) / 2
		if v.prefix[mid] < y {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// OnViewportScroll implements ScrollAware.
func (v *VirtualList) OnViewportScroll(offsetY, viewportH float64) {
	if v == nil {
		return
	}
	v.mu.Lock()
	if offsetY < 0 {
		offsetY = 0
	}
	if viewportH < 0 {
		viewportH = 0
	}
	changed := offsetY != v.scrollY || viewportH != v.viewportH
	v.scrollY, v.viewportH = offsetY, viewportH
	if !changed && len(v.mounted) > 0 {
		v.mu.Unlock()
		return
	}
	rebound := v.rebindWindowLocked()
	v.mu.Unlock()
	if rebound {
		v.MarkNeedsLayout()
	} else {
		v.MarkNeedsPaint()
	}
}

// rebindWindowLocked updates mounted children. Returns true if set of indices
// changed. Caller must hold v.mu.
func (v *VirtualList) rebindWindowLocked() bool {
	if v.ItemCount == 0 {
		return v.clearMountedLocked()
	}
	if !v.variable() && v.ItemExtent <= 0 {
		return v.clearMountedLocked()
	}

	vh := v.viewportH
	if vh <= 0 {
		// Provisional page until OnViewportScroll — do not use content height.
		if v.variable() {
			vh = 400
		} else {
			vh = v.ItemExtent * 12
			if vh < 100 {
				vh = 400
			}
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

	var first, last int
	if v.variable() {
		v.ensurePrefix()
		first = v.indexContaining(startY)
		last = v.indexAtOrAfter(endY)
		if last < first {
			last = first
		}
		if last > v.ItemCount {
			last = v.ItemCount
		}
	} else {
		first = int(startY / v.ItemExtent)
		last = int(endY/v.ItemExtent) + 1
		if first < 0 {
			first = 0
		}
		if last > v.ItemCount {
			last = v.ItemCount
		}
		if first > last {
			first = last
		}
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
	// Mount missing (freshly mounted cells must record once; already-mounted
	// cells keep their Picture cache across scroll offset changes, R7b reuse)
	freshlyMounted := make(map[int]bool, len(v.mounted))
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
			freshlyMounted[idx] = true
		}
	}
	v.first, v.last = first, last
	v.BindCount = len(v.mounted)
	// Position children. For already-mounted cells (not fresh this rebind),
	// SetOffset + MarkNeedsLayout would set needsPaint=true and invalidate the
	// BoundaryCache entry (scroll reuse R7b). Clear needsPaint on those cells so
	// their cached Picture replays (translated) instead of re-recording each frame.
	for idx, ch := range v.mounted {
		ch.SetOffset(Point{X: 0, Y: v.offsetOf(idx)})
		if !freshlyMounted[idx] {
			ch.clearPaintDirty()
		}
	}
	return true
}

func (v *VirtualList) clearMountedLocked() bool {
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
	if v.variable() {
		v.ensurePrefix()
		if len(v.prefix) == 0 {
			return 0
		}
		return v.prefix[len(v.prefix)-1]
	}
	return float64(v.ItemCount) * v.ItemExtent
}

// Layout implements RenderObject.
func (v *VirtualList) Layout(c Constraints) Size {
	if sz, ok := v.LayoutSkipIfClean(c); ok {
		return sz
	}
	w := c.MaxWidth
	if w >= Unbounded/2 {
		w = c.MinWidth
	}
	// Prefix rebuilds lazily in ensurePrefix; call InvalidateExtents after extentAt changes.
	h := v.ContentHeight()
	out := c.Tighten(Size{Width: w, Height: h})
	v.setSize(out)
	v.mu.Lock()
	v.rebindWindowLocked()
	// Layout mounted children tightly to each row's height.
	for idx, ch := range v.mounted {
		ext := v.extentAt(idx)
		rowC := Tight(out.Width, ext)
		_ = ch.Layout(rowC)
		ch.SetOffset(Point{X: 0, Y: v.offsetOf(idx)})
	}
	v.BindCount = len(v.mounted)
	v.mu.Unlock()
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
	// Snapshot children + per-child rowH under the lock so the raster loop
	// never races a ticker-driven rebind (concurrent map/slice read-write).
	v.mu.Lock()
	type paintedChild struct {
		ch   RenderObject
		off  Point
		rowH float64
	}
	pcs := make([]paintedChild, 0, len(v.children))
	for _, ch := range v.children {
		off := ch.Offset()
		rowH := v.ItemExtent
		if v.variable() {
			for idx, m := range v.mounted {
				if m == ch {
					rowH = v.extentAt(idx)
					break
				}
			}
		}
		pcs = append(pcs, paintedChild{ch: ch, off: off, rowH: rowH})
	}
	v.mu.Unlock()
	for _, p := range pcs {
		ch := p.ch
		off := p.off
		rowH := p.rowH
		if v.viewportH > 0 {
			top := off.Y
			bot := off.Y + rowH
			visTop := v.scrollY
			visBot := v.scrollY + v.viewportH
			if bot < visTop || top > visBot {
				continue
			}
		}
		if paintSelf {
			if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
				continue
			}
			ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
			continue
		}
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
	v.mu.Lock()
	children := make([]RenderObject, len(v.children))
	copy(children, v.children)
	v.mu.Unlock()
	for i := len(children) - 1; i >= 0; i-- {
		ch := children[i]
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
