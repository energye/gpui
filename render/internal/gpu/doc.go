//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

// Package gpu implements the GPU rendering backend used by the gg runtime.
//
// It sits on top of gpu/hal and talks to whichever backend was registered
// (pure-Go WebGPU, or the GLES backend). The package owns scene upload,
// tile-based rasterization, per-subsystem pipelines and GPU resource caching.
//
// # Architecture
//
//	Scene / draw elements
//	  -> tilecompute: path encode, then pathtag_reduce/scan, draw_reduce/leaf,
//	     clip_leaf, path_count, backdrop, coarse, path_tiling, fine (WGSL)
//	  -> per-subsystem pipelines (stencil, image, glyph mask/color, MSDF text,
//	     video YUV, SDF, convex, depth-clip, dual-texture blend)
//	  -> composite / present
//
// Key entry points:
//
//   - GPUShared: process-wide device/queue sharing across windows
//   - VelloAccelerator: compute (vello-style) scene rasterization
//   - RenderSceneAuto: scene-to-GPU rendering used by embedders
//
// # Resource budgets
//
// GPU memory is budgeted per subsystem instead of through one global manager:
//
//   - ImageCache: entry and byte LRU caps (SetBudgets to tune)
//   - glyph atlases: page-level LRU eviction
//   - depth-clip mask cache: count/byte budget
//   - TexturePool: render-target reuse (deprecated, kept for the shared cache)
//
// # Legacy helpers
//
// Backend is a minimal device holder used by the legacy command-recording and
// texture helpers (command_encoder.go, commands.go, gpu_texture.go, atlas.go).
// They are internal and kept for tests and callers that construct a device
// directly; new rendering code should use GPUShared/VelloAccelerator.
//
// # Blend modes
//
// Porter-Duff modes are handled by the stencil/composite pipelines; the 15
// advanced modes (Multiply, Screen, Overlay, Hue, Saturation, Color,
// Luminosity, Darken, Lighten, ColorDodge, ColorBurn, HardLight, SoftLight,
// Difference, Exclusion) are handled by dual_tex_blend.go.
//
// # Thread safety
//
// GPUShared and VelloAccelerator are safe for concurrent use from multiple
// goroutines. Internal synchronization is handled via mutexes.
//
// # Related packages
//
//   - github.com/energye/gpui/render: core 2D graphics library
//   - github.com/energye/gpui/render/scene: scene graph
//   - github.com/energye/gpui/gpu/hal: backend-neutral GPU interfaces
package gpu
