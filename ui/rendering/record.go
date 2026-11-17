//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import "github.com/energye/gpui/render"

// Recordable is the retained-mode recording contract (ENGINE_NODE_RECORD_COVERAGE_PLAN
// §5.1). Every RenderObject type that can be baked into a RepaintBoundary
// recording implements it; BoundaryCache consults the interface instead of a
// central type whitelist, so a new node type that forgets Record degrades
// honestly (its enclosing boundary refuses caching) and record_coverage_test
// turns red — silent fallback off the fast path cannot come back.
//
// Record must be semantically equivalent to Paint, which stays the single
// source of truth:
//   - own visual content (Background / OnPaint) at absolute origin (ox, oy);
//   - non-RepaintBoundary descendants at their offsets (recordChildrenInto);
//   - clips / opacity / filters / transforms as canvas state commands;
//   - RepaintBoundary descendants are NEVER baked here (they own their cache;
//     the caller walks them after a parent replay).
//
// scale is the paint walk's device scale (PaintContext.Scale): snapping inside
// OnPaint must round onto the same device grid the live path uses. Recording
// stores logical coordinates only — playback applies the live canvas CTM — so
// the recorded list itself is DPR-agnostic.
type Recordable interface {
	Record(rec render.Canvas, ox, oy, scale float64)
}

// Volatile marks a node whose pixels change every frame (video, realtime
// spinners/particles). Volatile nodes are never baked into a boundary
// recording: the enclosing boundary falls back to live paint (honest
// degradation, Flutter's "no raster cache for animating layers").
type Volatile interface {
	Volatile() bool
}

// Compile-time coverage for the engine's node family. A new type missing
// here is caught by record_coverage_test, which parses the package source.
var (
	_ Recordable = (*RenderBox)(nil)
	_ Recordable = (*RenderColorBox)(nil)
	_ Recordable = (*RenderText)(nil)
	_ Recordable = (*RenderImage)(nil)
	_ Recordable = (*RenderPictureBox)(nil)
	_ Recordable = (*AbsoluteBox)(nil)
	_ Recordable = (*RenderAlignBox)(nil)
	_ Recordable = (*RenderClipRRect)(nil)
	_ Recordable = (*RenderOpacity)(nil)
	_ Recordable = (*RenderColorFilter)(nil)
	_ Recordable = (*RenderImageFilter)(nil)
	_ Recordable = (*RenderTransform)(nil)

	_ Volatile = (*RenderSpinner)(nil)
	_ Volatile = (*RenderVideo)(nil)
)

// structuralNode marks a node that is deliberately never its own cache unit
// even though it paints: RenderViewport (the scrolling content boundary is the
// unit) and VirtualList (its mounted row window is rebuilt by the bind window).
// record_coverage_test accepts this declaration in place of Record/Volatile —
// it is the explicit "not cacheable by design" answer, not a silent omission.
type structuralNode interface {
	structuralNode()
}

// isVolatileNode reports whether n declares per-frame pixels.
func isVolatileNode(n RenderObject) bool {
	if n == nil {
		return false
	}
	v, ok := n.(Volatile)
	return ok && v != nil && v.Volatile()
}

// recordableNode reports whether n itself fulfils the recording contract.
func recordableNode(n RenderObject) bool {
	if n == nil || isVolatileNode(n) {
		return false
	}
	_, ok := n.(Recordable)
	return ok
}

// recordableTree reports whether n's own recording can be faithful: n itself
// plus every descendant that would be baked into it (non-RepaintBoundary).
// Nested boundaries are their own cache units and are skipped — a clean outer
// replay must not bake a child that may re-record independently.
func recordableTree(n RenderObject) bool {
	if !recordableNode(n) {
		return false
	}
	for _, ch := range n.Children() {
		if ch == nil || ch.IsRepaintBoundary() {
			continue
		}
		if !recordableTree(ch) {
			return false
		}
	}
	return true
}

// recordChildrenInto records all non-boundary children of n at their absolute
// offsets. Boundary children are skipped by contract (they replay themselves);
// a non-recordable child cannot reach here for a stored boundary
// (recordableTree gates Store first), so skipping keeps the walk total.
func recordChildrenInto(rec render.Canvas, n RenderObject, ox, oy, scale float64) {
	if rec == nil || n == nil {
		return
	}
	for _, ch := range n.Children() {
		if ch == nil || ch.IsRepaintBoundary() {
			continue
		}
		if r, ok := ch.(Recordable); ok {
			off := ch.Offset()
			r.Record(rec, ox+off.X, oy+off.Y, scale)
		}
	}
}
