package core

import "sort"

// OverlayEntry is one floating layer above the main tree (C-Overlay / C-PortalHost).
type OverlayEntry struct {
	ID   string
	Node Node
	// ZOrder higher paints later (on top).
	ZOrder int
	// Modal when true absorbs outside pointer hits (via Mask sibling typically).
	Modal bool
}

// OverlayHost is a root-level stack of portal contents.
//
// entries is kept sorted by ZOrder ascending (stable for equal Z) on every Push
// so hot-path Entries() is a plain snapshot — layout/paint/hit call it many times
// per frame while Push/Remove are rare (open/close portals).
type OverlayHost struct {
	entries []OverlayEntry // invariant: sorted by ZOrder ascending, stable
	seq     int
}

// NewOverlayHost creates an empty overlay stack.
func NewOverlayHost() *OverlayHost {
	return &OverlayHost{}
}

// Entries returns a snapshot in ZOrder ascending (already maintained by Push).
// The slice is a copy; mutating it does not affect the host.
func (h *OverlayHost) Entries() []OverlayEntry {
	if h == nil || len(h.entries) == 0 {
		return nil
	}
	out := make([]OverlayEntry, len(h.entries))
	copy(out, h.entries)
	return out
}

// Push adds or replaces an entry by ID. Empty ID gets an auto id.
// After mutation the stack is re-sorted stably by ZOrder so equal-Z relative
// order matches the historical "in-place update then stable sort" behavior.
func (h *OverlayHost) Push(e OverlayEntry) string {
	if h == nil {
		return ""
	}
	if e.ID == "" {
		h.seq++
		e.ID = formatOverlayID(h.seq)
	}
	for i := range h.entries {
		if h.entries[i].ID == e.ID {
			h.entries[i] = e
			h.keepSorted()
			return e.ID
		}
	}
	h.entries = append(h.entries, e)
	h.keepSorted()
	return e.ID
}

// keepSorted restores ZOrder ascending with stable ties (equal Z keep relative order).
func (h *OverlayHost) keepSorted() {
	if len(h.entries) < 2 {
		return
	}
	sort.SliceStable(h.entries, func(i, j int) bool {
		return h.entries[i].ZOrder < h.entries[j].ZOrder
	})
}

// Remove drops an entry by ID. Remaining entries stay sorted.
func (h *OverlayHost) Remove(id string) {
	if h == nil || id == "" {
		return
	}
	for i := range h.entries {
		if h.entries[i].ID == id {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			return
		}
	}
}

// Clear removes all overlays.
func (h *OverlayHost) Clear() {
	if h != nil {
		h.entries = nil
	}
}

// Len returns entry count.
func (h *OverlayHost) Len() int {
	if h == nil {
		return 0
	}
	return len(h.entries)
}

func formatOverlayID(n int) string {
	// avoid strconv import cycle concerns — tiny helper
	if n <= 0 {
		return "ov-0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return "ov-" + string(buf[i:])
}

// PaintOffsetParent applies an extra paint/hit transform to children without
// mutating child layout Offset (Flutter Transform / scroll offset).
// AbsoluteOffset, DefaultPaintChildren, and DefaultHitTest must all include it.
type PaintOffsetParent interface {
	// ContentPaintOffset is added to each direct child's paint origin and
	// subtracted when converting parent-local hit points into child-local space.
	// ScrollViewport returns Point{-ScrollX, -ScrollY}.
	ContentPaintOffset() Point
}

// AbsoluteOffset walks parents to compute absolute top-left of n.
// Includes PaintOffsetParent transforms so scrolled content tracks Scroll without layout Offset thrash.
func AbsoluteOffset(n Node) Point {
	var p Point
	for cur := n; cur != nil; {
		o := cur.Base().Offset()
		p.X += o.X
		p.Y += o.Y
		parent := cur.Parent()
		if parent != nil {
			if po, ok := parent.(PaintOffsetParent); ok {
				off := po.ContentPaintOffset()
				p.X += off.X
				p.Y += off.Y
			}
		}
		cur = parent
	}
	return p
}

// AbsoluteBounds returns the absolute rect of n after layout.
func AbsoluteBounds(n Node) Rect {
	if n == nil {
		return Rect{}
	}
	o := AbsoluteOffset(n)
	s := n.Base().Size()
	return NewRect(o.X, o.Y, s.Width, s.Height)
}
