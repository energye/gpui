//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"github.com/energye/gpui/render/text"
)

// SplitColorGlyphs partitions shaped glyphs into color vs outline subsets,
// preserving order. Color glyphs (CBDT bitmaps, COLR layers) cannot go
// through the R8 mask atlas; outline glyphs stay on the mask path.
// Pure-outline runs return the input slice untouched with zero allocation.
func SplitColorGlyphs(cf text.ColorFont, glyphs []text.ShapedGlyph) (color, outline []text.ShapedGlyph) {
	first := -1
	for i := range glyphs {
		if cf.GlyphType(uint16(glyphs[i].GID)) != text.GlyphTypeOutline {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, glyphs
	}
	color = make([]text.ShapedGlyph, 0, len(glyphs)-first)
	outline = make([]text.ShapedGlyph, 0, len(glyphs))
	for _, g := range glyphs {
		if cf.GlyphType(uint16(g.GID)) == text.GlyphTypeOutline {
			outline = append(outline, g)
		} else {
			color = append(color, g)
		}
	}
	return color, outline
}

// tryGPUColorGlyphText routes single-face color runs to the RGBA color atlas
// path before the strategy switch. Without this, color strings fall into the
// mask pipeline, hit the explicit refusal, and fall back every frame. Faces
// without color tables (the common case) return false before shaping, so the
// normal dispatch is untouched. Explicit Vector/Bitmap modes keep their CPU
// semantics and bypass this route.
// tryGPUColorGlyphText routes single-face color runs to the RGBA color atlas
// path before the strategy switch. Without this, color strings fall into the
// mask pipeline, hit the explicit refusal, and fall back every frame. Faces
// without color tables (the common case) return false before shaping, so the
// normal dispatch is untouched. Explicit Vector/Bitmap modes keep their CPU
// semantics and bypass this route.
func (c *Context) tryGPUColorGlyphText(s string, x, y float64) bool {
	face := c.face
	if face == nil || s == "" {
		return false
	}
	source := face.Source()
	if source == nil {
		return false
	}
	parsed := source.Parsed()
	cf, ok := parsed.(text.ColorFont)
	if !ok || !cf.HasColorTables() {
		return false
	}
	glyphs := text.LayoutGlyphs(face, s)
	color, outline := SplitColorGlyphs(cf, glyphs)
	if len(color) == 0 {
		return false
	}
	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()
	matrix, ds := c.totalMatrix(), c.DeviceScale()
	if submitted, _ := c.submitColorGlyphs(target, face, color, x, y, col, matrix, ds); !submitted {
		return false
	}
	if len(outline) > 0 {
		c.DrawShapedGlyphs(outline, face, x, y)
	}
	c.recordGPUOp()
	return true
}

// submitColorGlyphs queues pre-shaped color glyphs on the first available
// color accelerator (per-context GPU ops, then the global accelerator).
// Shared by the string shunt and the shaped entry point so the two call
// sites cannot drift apart. The second result reports whether the
// per-context accelerator exposes the color interface, so callers can
// attribute a CPU fallback exactly as before.
// submitColorGlyphs queues pre-shaped color glyphs on the first available
// color accelerator (per-context GPU ops, then the global accelerator).
// Shared by the string shunt and the shaped entry point so the two call
// sites cannot drift apart. The second result reports whether the
// per-context accelerator exposes the color interface, so callers can
// attribute a CPU fallback exactly as before.
func (c *Context) submitColorGlyphs(target GPURenderTarget, face text.Face, glyphs []text.ShapedGlyph, x, y float64, col RGBA, matrix Matrix, ds float64) (submitted, rcHasColor bool) {
	if rc := c.gpuCtxOps(); rc != nil {
		if ca, ok := rc.(GPUColorGlyphAccelerator); ok {
			rcHasColor = true
			if ca.DrawShapedColorGlyphs(target, face, glyphs, x, y, col, matrix, ds) == nil {
				return true, true
			}
		}
	}
	if a := Accelerator(); a != nil {
		if ca, ok := a.(GPUColorGlyphAccelerator); ok {
			if ca.DrawShapedColorGlyphs(target, face, glyphs, x, y, col, matrix, ds) == nil {
				return true, rcHasColor
			}
		}
	}
	return false, rcHasColor
}

// dispatchText routes one text run to the concrete rendering path selected by
// selectTextStrategy. It is the SINGLE strategy switch, shared by DrawString
// (top-level entry) and drawStringResolved (after MultiFace per-run
// resolution) so both paths can never drift apart.
//
//   - GlyphMask (Tier 6): GPU bitmap quads; falls back to glyph outlines
//     (Skia PathMask semantic — when the glyph atlas cannot hold the strike,
//     the outlines are filled as ordinary paths). MSDF is only used via the
//     explicit TextModeMSDF mode.
//   - Aliased / MSDF / Vector / Bitmap: explicit pipelines.
//   - default (TextModeAuto when no bitmap/stencil path applies): MSDF then CPU.
//
// tryGPUText attempts to render text via the GPU MSDF pipeline.
// The x, y coordinates are in user space (not pre-transformed by the CTM).
// The CTM is passed to the GPU pipeline so it can apply the full transform
// in the vertex shader, enabling correct scaling, rotation, and skew of text.
// Returns true if GPU text rendering was successful (queued for batch render).
func (c *Context) tryGPUText(s string, x, y float64) bool {
	if c.face == nil {
		if c.gpuPathAvailable() {
			c.recordCPUFallbackReason("text:no-face")
		}
		return false
	}
	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()
	if rc := c.gpuCtxOps(); rc != nil {
		if rc.DrawText(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
			c.recordGPUOp()
			return true
		}
		c.recordCPUFallbackReason("text:msdf-layout")
		return false
	}
	a := Accelerator()
	if a == nil {
		return false
	}
	c.warnGPUFallback("tryGPUText")
	if !a.CanAccelerate(AccelText) {
		c.recordCPUFallbackReason("text:no-accel")
		return false
	}
	ta, ok := a.(GPUTextAccelerator)
	if !ok {
		c.recordCPUFallbackReason("text:no-msdf-iface")
		return false
	}
	if ta.DrawText(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
		c.recordGPUOp()
		return true
	}
	c.recordCPUFallbackReason("text:msdf-draw")
	return false
}

// glyphMaskMaxSize is the maximum glyph-mask extent (device pixels) the GPU
// atlas can serve in TextModeAuto. This is NOT a pipeline-quality threshold
// (Skia/Flutter use DirectMask bitmaps for every axis-aligned size within the
// glyph-atlas budget): it is the atlas page bound — a single R8 glyph mask
// larger than the 1024px page cannot be packed. Strikes that exceed it (or
// that fail to pack) fall back to glyph outlines (Skia PathMask semantic —
// outlines filled as ordinary paths). MSDF is not part of auto-selection.
const glyphMaskMaxSize = 1024.0

// tryGPUGlyphMaskText attempts to render text via the GPU glyph mask pipeline
// (Tier 6). Glyphs are CPU-rasterized at the exact device pixel size into an
// R8 alpha atlas, then drawn as textured quads by the GPU.
// Returns true if text was successfully queued for glyph mask rendering.
// tryGPUGlyphMaskText attempts to render text via the GPU glyph mask pipeline
// (Tier 6). Glyphs are CPU-rasterized at the exact device pixel size into an
// R8 alpha atlas, then drawn as textured quads by the GPU.
// Returns true if text was successfully queued for glyph mask rendering.
func (c *Context) tryGPUGlyphMaskText(s string, x, y float64) bool {
	// Re-apply per-context LCD layout so suite tests cannot leak global layout.
	if a := Accelerator(); a != nil {
		if la, ok := a.(LCDLayoutAware); ok {
			la.SetLCDLayout(c.lcdLayout)
		}
	}
	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()
	if c.face == nil {
		if c.gpuPathAvailable() {
			c.recordCPUFallbackReason("text:glyphmask-no-face")
		}
		return false
	}
	if rc := c.gpuCtxOps(); rc != nil {
		if rc.DrawGlyphMaskText(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
			c.trackTextDamage(s, x, y)
			c.recordGPUOp()
			return true
		}
		c.recordCPUFallbackReason("text:glyphmask-layout")
		return false
	}
	a := Accelerator()
	if a == nil {
		return false
	}
	c.warnGPUFallback("tryGPUGlyphMaskText")
	gma, ok := a.(GPUGlyphMaskAccelerator)
	if !ok {
		c.recordCPUFallbackReason("text:glyphmask-no-iface")
		return false
	}
	if gma.DrawGlyphMaskText(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
		c.trackTextDamage(s, x, y)
		c.recordGPUOp()
		return true
	}
	c.recordCPUFallbackReason("text:glyphmask-draw")
	return false
}

// tryGPUTransformMask renders auto-selected vector text (rotated/sheared/
// non-uniform CTM) through the kTransformedMask semantic: the whole string's
// outline path — identical to the geometry the CPU Tier2 sketch path uses —
// is handed to the GPU accelerator, which CPU-rasterizes it to an alpha mask
// with the same Skia-AAA software filler and draws it as one textured quad.
// The GPU output therefore matches the CPU rendering bit-exactly.
// tryGPUTransformMask renders auto-selected vector text (rotated/sheared/
// non-uniform CTM) through the kTransformedMask semantic: the whole string's
// outline path — identical to the geometry the CPU Tier2 sketch path uses —
// is handed to the GPU accelerator, which CPU-rasterizes it to an alpha mask
// with the same Skia-AAA software filler and draws it as one textured quad.
// The GPU output therefore matches the CPU rendering bit-exactly.
func (c *Context) tryGPUTransformMask(s string, x, y float64) bool {
	if !c.needsOutlineTransform() {
		return false
	}
	path := c.textOutlinePath(s, x, y)
	if path == nil {
		return false
	}
	transformed := path.Transform(c.matrix)
	devicePath := transformed
	if !c.deviceMatrix.IsIdentity() {
		devicePath = transformed.Transform(c.deviceMatrix)
	}
	if devicePath == nil || devicePath.Bounds().Empty() {
		return false
	}
	target := c.gpuRenderTarget()
	col := FromColor(c.currentColor())

	if rc := c.gpuCtxOps(); rc != nil {
		if ata, ok := rc.(GPUTransformMaskTextAccelerator); ok {
			if ata.DrawGlyphMaskTransformText(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale(), devicePath) == nil {
				c.trackTextDamage(s, x, y)
				c.recordGPUOp()
				return true
			}
		}
	}
	a := Accelerator()
	if a == nil {
		return false
	}
	if ata, ok := a.(GPUTransformMaskTextAccelerator); ok {
		if ata.DrawGlyphMaskTransformText(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale(), devicePath) == nil {
			c.trackTextDamage(s, x, y)
			c.recordGPUOp()
			return true
		}
	}
	return false
}

// trackTextDamage registers the ink bounds of a GPU-queued text draw into the
// current frame + layer damage. Fill/Stroke record c.path.Bounds() inside
// Context.Fill/Stroke, so rectangle/vector draws always contribute to damage;
// GPU glyph-mask/MSDF text paths bypass Context.Fill and therefore must record
// their own bounds. Without this, an isolation layer's damage (which becomes
// the layer-RT scissor in FlushGPUWithViewDamage) only covers shape draws and
// silently clips CJK runs that extend past the last FillRect edge — the
// "合成残影" bug in ui_wr_r18_savelayer.
// tryGPUGlyphMaskTextAliased attempts to render aliased text via the GPU glyph
// mask pipeline. Same Tier 6 pipeline but with binary (0/255) rasterization.
// Returns true if text was successfully queued for aliased glyph mask rendering.
func (c *Context) tryGPUGlyphMaskTextAliased(s string, x, y float64) bool {
	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()
	if rc := c.gpuCtxOps(); rc != nil {
		if rc.DrawGlyphMaskTextAliased(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
			c.recordGPUOp()
			return true
		}
		c.recordCPUFallbackReason("text:tryGPUGlyphMaskTextAliased")
		return false
	}
	a := Accelerator()
	if a == nil {
		return false
	}
	ata, ok := a.(GPUAliasedTextAccelerator)
	if !ok {
		c.recordCPUFallbackReason("text:tryGPUGlyphMaskTextAliased")
		return false
	}
	if ata.DrawGlyphMaskTextAliased(target, c.face, s, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
		c.recordGPUOp()
		return true
	}
	c.recordCPUFallbackReason("text:tryGPUGlyphMaskTextAliased")
	return false
}

// selectTextStrategy returns the effective text rendering strategy.
//
// When TextModeAuto, the strategy is derived from the face + CTM alone:
//   - rotated/sheared/non-uniform transforms → glyph outlines (kPath);
//   - axis-aligned text within the glyph-mask size bound → glyph bitmaps;
//   - otherwise → the default MSDF→CPU fallback.
//
// Explicit modes (MSDF, Vector, Bitmap, GlyphMask) are returned as-is.
