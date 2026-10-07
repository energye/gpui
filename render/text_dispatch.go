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
	"image"
	"image/color"
	"image/draw"

	"github.com/energye/gpui/render/text"
)

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
func (c *Context) dispatchText(s string, x, y float64) {
	// Color-font runs bypass the mask strategy switch: the mask pipeline
	// explicitly refuses color glyphs, so without this shunt every such
	// string would fall back on every frame.
	mode := c.textMode
	if m, ok := forceTextMode(); ok {
		mode = m
	}
	if mode != TextModeVector && mode != TextModeBitmap {
		if c.tryGPUColorGlyphText(s, x, y) {
			return
		}
	}
	switch c.selectTextStrategy() {
	case TextModeGlyphMask:
		if c.tryGPUGlyphMaskText(s, x, y) {
			return
		}
		c.drawStringAsOutlines(s, x, y)
	case TextModeAliased:
		// Aliased text through glyph mask pipeline with binary rasterization.
		// Same Tier 6 atlas + GPU path, but NoAAFiller instead of AnalyticFiller.
		if c.tryGPUGlyphMaskTextAliased(s, x, y) {
			return
		}
		// CPU fallback: per-glyph NoAAFiller rasterization (binary 0/255 masks).
		c.drawStringCPUAliased(s, x, y)
	case TextModeMSDF:
		// Try GPU MSDF first; fall back to CPU if unavailable.
		if c.tryGPUText(s, x, y) {
			return
		}
		c.drawStringCPU(s, x, y)
	case TextModeVector:
		// Auto-selected outlines (rotated/sheared/non-uniform CTM) render via
		// a CPU-rasterized whole-string alpha mask (kTransformedMask semantic)
		// so the GPU output matches the CPU Skia-AAA fill bit-exactly. An
		// EXPLICIT TextModeVector is preserved as pure vector outlines.
		if c.textMode != TextModeVector && c.tryGPUTransformMask(s, x, y) {
			return
		}
		// Vector text is rendered as glyph outline paths through the normal
		// fill pipeline (doFill). This routes through GPU stencil+cover when
		// a SurfaceTarget is active, or CPU when standalone. No explicit
		// flush here — doFill() manages GPU/CPU routing and any necessary
		// flush internally. An explicit flush would create a mid-frame
		// render pass with LoadOpClear, wiping previously drawn content.
		c.drawStringAsOutlines(s, x, y)
	case TextModeBitmap:
		// Skip GPU entirely, use CPU pipeline directly.
		c.flushGPUAccelerator()
		c.drawStringCPU(s, x, y)
	default: // TextModeAuto — current behavior
		if c.tryGPUText(s, x, y) {
			return
		}
		c.drawStringCPU(s, x, y)
	}
}

// needsOutlineTransform reports whether the current CTM contains rotation,
// shear, or non-uniform scale — transforms under which fixed-resolution
// bitmap/SDF text pipelines visibly degrade.
// drawStringMultiFace renders fallback font runs (X.06). Each contiguous run
// uses a single FontSource so the GPU glyph-mask path can operate correctly.
// Runs may resolve to a nested MultiFace (composite chains); those recurse
// with a depth cap instead of reaching dispatch with a sourceless face.
func (c *Context) drawStringMultiFace(mf *text.MultiFace, s string, x, y float64) {
	if mf == nil || s == "" {
		return
	}
	defer c.setGPUClipRect()()
	defer c.applyTextDecorations(s, x, y)

	orig := c.face
	c.drawFaceRuns(mf, s, x, y, 0)
	c.face = orig
}

func (c *Context) drawFaceRuns(mf *text.MultiFace, s string, x, y float64, depth int) {
	for _, run := range mf.Runs(s) {
		if run.Text == "" || run.Face == nil {
			continue
		}
		if inner, ok := run.Face.(*text.MultiFace); ok && depth < nestedFaceMaxDepth {
			c.drawFaceRuns(inner, run.Text, x+run.X, y, depth+1)
			continue
		}
		c.face = run.Face
		// Resolved face (no MultiFace recursion beyond the cap above).
		c.drawStringResolved(run.Text, x+run.X, y)
	}
}

// drawStringResolved is DrawString after MultiFace resolution (no decorations/clip re-entry).
// drawStringResolved is DrawString after MultiFace resolution (no decorations/clip re-entry).
func (c *Context) drawStringResolved(s string, x, y float64) {
	// Shared strategy dispatch — single source of truth with DrawString.
	c.dispatchText(s, x, y)
}

// DrawShapedGlyphs renders pre-shaped glyphs through the GPU text pipeline
// without re-shaping. This implements the ADR-022 "shape once" guarantee:
// glyphs are shaped at scene recording time, then rendered here with stored
// positions. Falls back to DrawString (re-shaping) if the GPU accelerator
// doesn't implement GPUShapedTextAccelerator.
//
// Enterprise pattern: matches Skia drawTextBlob, Vello draw_glyphs.
// DrawShapedGlyphs renders pre-shaped glyphs through the GPU text pipeline
// without re-shaping. This implements the ADR-022 "shape once" guarantee:
// glyphs are shaped at scene recording time, then rendered here with stored
// positions. Falls back to DrawString (re-shaping) if the GPU accelerator
// doesn't implement GPUShapedTextAccelerator.
//
// Enterprise pattern: matches Skia drawTextBlob, Vello draw_glyphs.
func (c *Context) DrawShapedGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64) {
	if face == nil || len(glyphs) == 0 {
		return
	}

	defer c.setGPUClipRect()()

	// TextModeVector opts out of the glyph-mask accelerator and renders the
	// pre-shaped glyphs as vector outlines (same glyph.X positions). Other
	// modes need the original string to re-render, which we don't have here.
	// The GOGPU_TEXT_MODE=vector env override also routes here.
	mode := c.textMode
	if m, ok := forceTextMode(); ok {
		mode = m
	}
	if mode == TextModeVector {
		c.drawShapedGlyphsAsOutlines(glyphs, face, x, y)
		return
	}

	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()

	if rc := c.gpuCtxOps(); rc != nil {
		if sta, ok := rc.(GPUShapedTextAccelerator); ok {
			if sta.DrawShapedGlyphMaskText(target, face, glyphs, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
				c.recordGPUOp()
				return
			}
		}
	}

	a := Accelerator()
	if a != nil {
		if sta, ok := a.(GPUShapedTextAccelerator); ok {
			if sta.DrawShapedGlyphMaskText(target, face, glyphs, x, y, col, c.totalMatrix(), c.DeviceScale()) == nil {
				c.recordGPUOp()
				return
			}
		}
	}

	// Fallback: reconstruct string is not possible from glyphs,
	// so render each glyph outline through the fill pipeline.
	c.drawShapedGlyphsAsOutlines(glyphs, face, x, y)
}

// DrawShapedColorGlyphs renders pre-shaped color glyphs (CBDT bitmaps,
// COLR layers) at glyph.X positions plus the origin (x, y baseline).
// The GPU path packs CPU-rasterized RGBA into color atlas pages; without a
// color accelerator the glyphs composite onto the CPU pixmap (translation
// CTM) or fall back to outlines. Glyphs the font renders as plain outlines
// are skipped — they belong to the mask pipeline, not this call.
// DrawShapedColorGlyphs renders pre-shaped color glyphs (CBDT bitmaps,
// COLR layers) at glyph.X positions plus the origin (x, y baseline).
// The GPU path packs CPU-rasterized RGBA into color atlas pages; without a
// color accelerator the glyphs composite onto the CPU pixmap (translation
// CTM) or fall back to outlines. Glyphs the font renders as plain outlines
// are skipped — they belong to the mask pipeline, not this call.
func (c *Context) DrawShapedColorGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64) {
	if face == nil || len(glyphs) == 0 {
		return
	}

	defer c.setGPUClipRect()()

	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()

	if submitted, rcHasColor := c.submitColorGlyphs(target, face, glyphs, x, y, col, c.totalMatrix(), c.DeviceScale()); submitted {
		c.recordGPUOp()
		return
	} else if rcHasColor {
		c.recordCPUFallbackReason("text:color-layout")
	}

	c.drawShapedColorGlyphsCPU(glyphs, face, x, y)
}

// drawShapedColorGlyphsCPU composites cached color RGBA onto the CPU pixmap
// under translation-only CTMs; other transforms use outlines (same tiering
// as the string bitmap path).
// drawShapedColorGlyphsCPU composites cached color RGBA onto the CPU pixmap
// under translation-only CTMs; other transforms use outlines (same tiering
// as the string bitmap path).
func (c *Context) drawShapedColorGlyphsCPU(glyphs []text.ShapedGlyph, face text.Face, x, y float64) {
	m := c.totalMatrix()
	if m.B != 0 || m.D != 0 || m.A != m.E {
		c.drawShapedGlyphsAsOutlines(glyphs, face, x, y)
		return
	}
	source := face.Source()
	if source == nil || c.pixmap == nil {
		return
	}
	parsed := source.Parsed()
	cf, ok := parsed.(text.ColorFont)
	if !ok || !cf.HasColorTables() {
		return
	}
	ppem := uint16(face.Size()*c.DeviceScale() + 0.5)
	if ppem < 1 {
		ppem = 1
	}
	fg := color.RGBAModel.Convert(c.currentColor()).(color.RGBA)
	if c.colorRasterCache == nil {
		c.colorRasterCache = text.NewColorRasterCache(64)
	}
	cache := c.colorRasterCache
	c.flushGPUAccelerator()
	for _, glyph := range glyphs {
		if cf.GlyphType(uint16(glyph.GID)) == text.GlyphTypeOutline {
			continue
		}
		img, err := cache.Image(parsed, uint16(glyph.GID), ppem, 0, fg)
		if err != nil {
			continue
		}
		dev := m.TransformPoint(Pt(x+glyph.X, y))
		dx := int(dev.X + float64(img.OriginX) + 0.5)
		dy := int(dev.Y - float64(img.OriginY) + 0.5)
		w := img.Pix.Bounds().Dx()
		h := img.Pix.Bounds().Dy()
		draw.Draw(c.pixmap, image.Rect(dx, dy, dx+w, dy+h), img.Pix, image.Point{}, draw.Over)
	}
}

// drawShapedGlyphsAsOutlines renders pre-shaped glyphs as vector outlines.
// CPU fallback when GPU shaped text is unavailable.
// When the face has variations, uses go-text for outline extraction (gvar support).
// drawShapedGlyphsAsOutlines renders pre-shaped glyphs as vector outlines.
// CPU fallback when GPU shaped text is unavailable.
// When the face has variations, uses go-text for outline extraction (gvar support).
func (c *Context) drawShapedGlyphsAsOutlines(glyphs []text.ShapedGlyph, face text.Face, x, y float64) {
	source := face.Source()
	if source == nil {
		return
	}

	parsed := source.Parsed()
	extractor := text.NewOutlineExtractor()

	outlineFunc := func(gid text.GlyphID) *text.GlyphOutline {
		outline, err := extractor.ExtractOutline(parsed, gid, face.Size())
		if err != nil {
			return nil
		}
		return outline
	}

	for _, glyph := range glyphs {
		outline := outlineFunc(glyph.GID)
		if outline == nil || outline.IsEmpty() {
			continue
		}

		glyphX := x + glyph.X
		glyphY := y + glyph.Y
		path := NewPath()
		for _, seg := range outline.Segments {
			switch seg.Op {
			case text.OutlineOpMoveTo:
				path.MoveTo(glyphX+float64(seg.Points[0].X), glyphY+float64(seg.Points[0].Y))
			case text.OutlineOpLineTo:
				path.LineTo(glyphX+float64(seg.Points[0].X), glyphY+float64(seg.Points[0].Y))
			case text.OutlineOpQuadTo:
				path.QuadraticTo(
					glyphX+float64(seg.Points[0].X), glyphY+float64(seg.Points[0].Y),
					glyphX+float64(seg.Points[1].X), glyphY+float64(seg.Points[1].Y))
			case text.OutlineOpCubicTo:
				path.CubicTo(
					glyphX+float64(seg.Points[0].X), glyphY+float64(seg.Points[0].Y),
					glyphX+float64(seg.Points[1].X), glyphY+float64(seg.Points[1].Y),
					glyphX+float64(seg.Points[2].X), glyphY+float64(seg.Points[2].Y))
			}
		}
		c.SetFillRule(FillRuleNonZero)
		_ = c.FillPath(path)
	}
}

// tryGPUText attempts to render text via the GPU MSDF pipeline.
// The x, y coordinates are in user space (not pre-transformed by the CTM).
// The CTM is passed to the GPU pipeline so it can apply the full transform
// in the vertex shader, enabling correct scaling, rotation, and skew of text.
// Returns true if GPU text rendering was successful (queued for batch render).
// drawStringCPU selects the optimal CPU text rendering strategy based on the CTM.
// Three-tier decision tree modeled after Skia (QR decomposition, 256px threshold)
// and Cairo (three-matrix model):
//
//   - Tier 0: Translation-only → bitmap fast path (no quality loss)
//   - Tier 1: Uniform positive scale ≤256px → bitmap at device size (Strategy A)
//   - Tier 2: Everything else → glyph outlines as vector paths (Strategy B)
func (c *Context) drawStringCPU(s string, x, y float64) {
	m := c.matrix

	// Tier 0: Translation-only → bitmap fast path (no quality loss).
	if m.IsTranslationOnly() {
		c.drawStringBitmap(s, x, y)
		return
	}

	// Tier 1: Uniform positive scale ≤256px → bitmap at device size (Strategy A).
	// Skia threshold: kSkSideTooBigForAtlas = 256.
	// deviceSize here is in user-scaled units; drawStringScaled multiplies by
	// c.DeviceScale() to get the physical pixel size for the face.
	if m.B == 0 && m.D == 0 && m.A == m.E && m.A > 0 {
		deviceSize := c.face.Size() * m.A
		if deviceSize > 0 && deviceSize <= 256 {
			c.drawStringScaled(s, x, y, deviceSize)
			return
		}
	}

	// Tier 2: Everything else → glyph outlines as paths (Strategy B, Vello pattern).
	c.drawStringAsOutlines(s, x, y)
}

// drawStringBitmap renders text via the bitmap rasterizer at the transformed position.
// This is the fast path for identity/translation-only CTMs where no quality loss occurs.
// drawStringBitmap renders text via the bitmap rasterizer at the transformed position.
// This is the fast path for identity/translation-only CTMs where no quality loss occurs.
func (c *Context) drawStringBitmap(s string, x, y float64) {
	p := c.totalMatrix().TransformPoint(Pt(x, y))
	c.flushGPUAccelerator()
	face := c.face
	if c.DeviceScale() != 1.0 {
		if source := c.face.Source(); source != nil {
			face = source.Face(c.face.Size() * c.DeviceScale())
		}
	}
	text.DrawWithEmoji(c.pixmap, s, face, p.X, p.Y, c.currentColor())
}

// drawStringScaled renders text via bitmap rasterization at the device pixel size.
// Strategy A: Create a face at the scaled size, render at the transformed position.
// Falls back to drawStringBitmap if the face doesn't have a FontSource (e.g. MultiFace).
// drawStringScaled renders text via bitmap rasterization at the device pixel size.
// Strategy A: Create a face at the scaled size, render at the transformed position.
// Falls back to drawStringBitmap if the face doesn't have a FontSource (e.g. MultiFace).
func (c *Context) drawStringScaled(s string, x, y float64, deviceSize float64) {
	source := c.face.Source()
	if source == nil {
		c.drawStringBitmap(s, x, y) // MultiFace fallback
		return
	}
	// Scale deviceSize by deviceScale for actual physical pixel rendering.
	deviceFace := source.Face(deviceSize * c.DeviceScale())
	p := c.totalMatrix().TransformPoint(Pt(x, y))
	c.flushGPUAccelerator()
	text.Draw(c.pixmap, s, deviceFace, p.X, p.Y, c.currentColor())
}

// drawStringCPUAliased renders text with binary (non-anti-aliased) coverage on CPU.
// Uses GlyphMaskRasterizer.RasterizeAliased (NoAAFiller) to produce per-glyph R8
// masks with only 0 or 255 values, then composites via draw.DrawMask.
//
// For rotation/skew, routes through drawStringAsOutlines with AA disabled so the
// normal fill pipeline uses NoAAFiller for vector outlines.
// drawStringCPUAliased renders text with binary (non-anti-aliased) coverage on CPU.
// Uses GlyphMaskRasterizer.RasterizeAliased (NoAAFiller) to produce per-glyph R8
// masks with only 0 or 255 values, then composites via draw.DrawMask.
//
// For rotation/skew, routes through drawStringAsOutlines with AA disabled so the
// normal fill pipeline uses NoAAFiller for vector outlines.
func (c *Context) drawStringCPUAliased(s string, x, y float64) {
	m := c.matrix

	// Non-trivial transforms: route through vector outlines with AA disabled.
	// IsTranslationOnly = identity + translation.
	// Uniform positive scale: B=0, D=0, A=E, A>0.
	if !m.IsTranslationOnly() && !(m.B == 0 && m.D == 0 && m.A == m.E && m.A > 0) {
		saved := c.paint.Antialias
		c.paint.Antialias = false
		c.drawStringAsOutlines(s, x, y)
		c.paint.Antialias = saved
		return
	}

	p := c.totalMatrix().TransformPoint(Pt(x, y))
	c.flushGPUAccelerator()

	face := c.face
	if c.DeviceScale() != 1.0 {
		if source := c.face.Source(); source != nil {
			face = source.Face(c.face.Size() * c.DeviceScale())
		}
	}

	// Uniform scale: create device-size face for crisp rendering.
	if !m.IsTranslationOnly() && m.B == 0 && m.D == 0 && m.A == m.E && m.A > 0 {
		deviceSize := c.face.Size() * m.A
		if source := c.face.Source(); source != nil {
			face = source.Face(deviceSize * c.DeviceScale())
		}
	}

	text.DrawAliased(c.pixmap, s, face, p.X, p.Y, c.currentColor())
}

// StrokeString strokes text outlines at position (x, y) where y is the baseline.
// The stroke width, cap, join, and dash come from the current paint state.
//
// For thick strokes (lineWidth > 2), use [Context.SetLineJoin] with [LineJoinRound]
// to avoid miter spikes at glyph segment junctions. Glyph outlines contain many
// short curve segments, and the default [LineJoinMiter] produces sharp spikes at
// each junction. All enterprise text renderers recommend or
// default to round joins for stroked text.
//
// Unlike DrawString, StrokeString always uses vector outlines regardless of the
// current TextMode — MSDF and glyph mask pipelines cannot produce stroked text.
// If no font has been set with SetFont, this function does nothing.
//
// Enterprise pattern: matches HTML5 Canvas strokeText(), Cairo show_text() + stroke(),
// Skia SkPaint::kStroke_Style + drawTextBlob.
// drawStringAsOutlines renders text by converting glyph vector outlines to a Path
// and filling through the normal multi-tier pipeline (GPU → CoverageFiller → Analytic).
// Strategy B (Vello pattern): handles rotation, non-uniform scale, shear, mirroring,
// and extreme scales that exceed the bitmap threshold.
//
// Design: all glyphs are composed into ONE path for a single efficient fill call.
// Outlines are built in user space, then path.Transform(CTM) converts to device space.
// The device-space path is routed through doFill() so that GPU accelerator can render
// it to the surface (stencil+cover) when SurfaceTarget is active, or CPU renders
// to pixmap in standalone mode.
func (c *Context) drawStringAsOutlines(s string, x, y float64) {
	path := c.textOutlinePath(s, x, y)
	if path == nil {
		// MultiFace fallback: textOutlinePath returns nil when Source() is nil.
		if c.face != nil && c.face.Source() == nil {
			c.drawStringBitmap(s, x, y)
		}
		return
	}

	// User matrix only — doFill() applies deviceMatrix via deviceSpacePath().
	transformedPath := path.Transform(c.matrix)

	// Route through the normal fill pipeline (doFill) so GPU accelerator
	// can render to the surface when SurfaceTarget is active. Without this,
	// text rendered via renderer.Fill() goes to CPU pixmap which is never
	// composited in zero-copy RenderDirect mode. (#184)
	//
	// Save and restore context path/paint state — doFill uses c.path and c.paint.
	savedPath := c.path
	savedFillRule := c.paint.FillRule
	c.path = transformedPath
	c.paint.FillRule = FillRuleNonZero
	_ = c.doFill()
	c.path = savedPath
	c.paint.FillRule = savedFillRule
}

// ensureOutlineExtractor lazily initializes the outline extractor.
