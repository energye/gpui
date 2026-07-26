// Package ui is the L1 UI engine (Flutter-style pipeline) for gpui.
//
// Status: P0–P4 implemented (L1 prototype + scroll/IO). See docs/ENGINE_L1_CLOSEOUT.md.
//
// Dependency rule (hard):
//
//	ui → render → gpu
//
// This package must NEVER import github.com/energye/gpui/gpu or any subpackage.
// GPU surfaces are opened only through render.PresentTarget (native handles in,
// present out).
//
// FFI rule (hard, whole repo including examples):
//
//	NO cgo (import "C"). Use purego for native libraries.
//	See docs/ENGINE_CODING_RULES.md.
//
// Docs:
//
//	docs/ENGINE_L1_CLOSEOUT.md      — status, verify commands, limits
//	docs/ENGINE_CODING_RULES.md     — no-CGO / purego / dependency rules
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
//	ui/io         — async image decode pool (F12)
//
// Coordinates: layout/hit/pointer = logical pixels, Y-down; GPU = physical × dpr.
package ui
