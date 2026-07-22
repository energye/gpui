// Package core is the UI runtime (Flutter-like single tree):
//
//	Node + Constraints layout → Offset/Size
//	HitTest uses Offset (local = parentPoint − Offset)
//	Paint uses Origin = parentOrigin + Offset
//	Tree: layout / paint / overlays / tickers / focus / events
//
// core owns algorithms and contracts only — no product control names, no OS APIs.
// Drawing goes through render.Context via PaintContext.
//
// Contract: hit == layout == paint (logical pixels). See docs/LAYOUT_FOUNDATION.md.
package core
