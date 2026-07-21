// Package core is the UI framework runtime (L3a): single Node tree, layout,
// hit-testing, event routing, paint context, and frame pipeline.
//
// core owns algorithms and contracts; it has no product control names and does
// not call OS APIs. Drawing goes only through render.Context via PaintContext.
//
// See docs/UI_FRAMEWORK_MAP.md §1, §5.3 P0, §9, §12 M0.
package core
