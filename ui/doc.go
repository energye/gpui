// Package ui is the L1 UI engine (Flutter-style pipeline) for gpui.
//
// Status: P0–P3 implemented (experience prototype). See docs/ENGINE_L1_CLOSEOUT.md.
//
// Dependency rule (hard):
//
//	ui → render → gpu
//
// This package must NEVER import github.com/energye/gpui/gpu or any subpackage.
// GPU surfaces are opened only through render.PresentTarget (native handles in,
// present out).
//
// Docs:
//
//	docs/ENGINE_L1_CLOSEOUT.md      — status, verify commands, limits
//	docs/ENGINE_FLUTTER_SKIA_ARCH.md — architecture
//	docs/ENGINE_PHASE_P0_P3.md      — phase tasks
//
// Subpackages:
//
//	ui/platform   — Host, NativeSurface, events, vsync waiter
//	ui/scheduler  — frame modes, ticker registry, metrics
//	ui/raster     — raster thread queue (SubmitLatest, pending)
//	ui/embedder   — App (clear) · PipelineApp (tree + async present)
//	ui/rendering  — RenderObject, PipelineOwner, Spinner, BuildLayerTree
//	ui/painting   — PaintingContext (logical px, Y-down, CompositeOnly)
//	ui/scene      — Layer tree, FramePacket COW, RasterizeDirty
//	ui/animation  — Controller (auto-unregister ticker)
//
// Coordinates: layout/hit/pointer = logical pixels, Y-down; GPU = physical × dpr.
package ui
