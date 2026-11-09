//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import "github.com/energye/gpui/render"

// newSVGOffscreen creates the CPU-rasterized offscreen contexts SVG uses for
// pattern tiles, masks and filter intermediates.
//
// Document.Render pins the root bake to the software rasterizer
// (render.WithCPUOnly) so the output is identical inside a GPU window and in a
// pure-CPU process. Nested offscreens must follow the same policy: with a GPU
// accelerator registered (render/gpu imported by the window), an offscreen
// without WithCPUOnly is painted by the GPU path, whose anti-aliased canvas
// edge coverage differs from the CPU rasterizer. Measured on the tilePatt
// pattern tile: column 0 ended at alpha 236/255, so every tiled instance
// sampled a 7.5%-transparent column → one dark vertical seam per tile
// (P3.8 批② tilePatt 专项).
func newSVGOffscreen(w, h int) *render.Context {
	return render.NewContext(w, h, render.WithCPUOnly())
}
