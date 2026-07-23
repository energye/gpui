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
//
// # Composition bands (Flutter Overlay)
//
// Hosts with a retained compositor (ui/layer) must paint in two bands:
//
//	PaintMain      → mainBase + LayerBandMain layers (ScrollViewport, Spin, …)
//	PaintOverlays  → overlayBase + LayerBandOverlay layers (Modal mask/panel, …)
//
// Present order: mainBase → mainLayers → overlayBase → overlayLayers.
// Baking portals into the same base as deferred main layers lets Scroll textures
// blit above Modal masks — always use PaintMain/PaintOverlays + dual BlitTo.
//
// HitTest is already overlays-first (unchanged).
//
// Portal kit controls (Modal, Drawer, Message, Tooltip, Select, …) mount via
// OverlayHost. In-tree RepaintBoundary isolation is Main-band only.
package core
