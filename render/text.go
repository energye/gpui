package render

import (
	"fmt"
	"hash/fnv"
	"image"
	"math"
	"os"
	"strings"

	"github.com/energye/gpui/render/text"
)

// forceTextMode lets a developer override text rendering globally via the
// GOGPU_TEXT_MODE env var (vector|glyphmask|msdf|bitmap|aliased) to A/B the
// rendering paths without a code change. Empty = no override.
func forceTextMode() (TextMode, bool) {
	switch os.Getenv("GOGPU_TEXT_MODE") {
	case "vector":
		return TextModeVector, true
	case "glyphmask":
		return TextModeGlyphMask, true
	case "msdf":
		return TextModeMSDF, true
	case "bitmap":
		return TextModeBitmap, true
	case "aliased":
		return TextModeAliased, true
	default:
		return TextModeAuto, false
	}
}

// Align specifies text horizontal alignment.
// This is a type alias for text.Alignment, provided for fogleman/gg compatibility.
type Align = text.Alignment

// Alignment constants re-exported from the text package for convenience.
const (
	AlignLeft   = text.AlignLeft
	AlignCenter = text.AlignCenter
	AlignRight  = text.AlignRight
)

// SetFont sets the current font face for text drawing.
// The face should be created from a FontSource.
//
// Example:
//
//	source, _ := text.NewFontSourceFromFile("font.ttf")
//	face := source.Face(12.0)
//	ctx.SetFont(face)
func (c *Context) SetFont(face text.Face) {
	c.face = face
}

// Font returns the current font face.
// Returns nil if no font has been set.
func (c *Context) Font() text.Face {
	return c.face
}

// DrawString draws text at position (x, y) where y is the baseline.
// If no font has been set with SetFont, this function does nothing.
//
// If a GPU accelerator is registered and supports text rendering (implements
// GPUTextAccelerator), the text is rendered via the GPU MSDF pipeline.
// The CTM (Current Transform Matrix) is passed to the GPU so that Scale,
// Rotate, and Skew transforms affect text rendering, not just position.
// Otherwise, the CPU text pipeline is used with transform-aware rendering:
//   - Translation-only: bitmap fast path (zero quality loss)
//   - Uniform scale ≤256px: bitmap at device size (Strategy A, Skia pattern)
//   - Everything else: glyph outlines as vector paths (Strategy B, Vello pattern)
//
// The baseline is the line on which most letters sit. Characters with
// descenders (like 'g', 'j', 'p', 'q', 'y') extend below the baseline.
func (c *Context) DrawString(s string, x, y float64) {

	if c.face == nil {
		return
	}
	c.syncPublishedFilterBeforeDraw()

	// X.06: MultiFace → per-face runs so each FontSource can use GPU glyph masks.
	if mf, ok := c.face.(*text.MultiFace); ok {
		c.drawStringMultiFace(mf, s, x, y)
		return
	}

	// Set GPU scissor rect for rectangular clips.
	defer c.setGPUClipRect()()
	defer c.applyTextDecorations(s, x, y)

	c.dispatchText(s, x, y)
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
func (c *Context) dispatchText(s string, x, y float64) {
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
// bitmap/SDF text pipelines visibly degrade. Mirrors the CPU tier selection
// (Tier 2 outlines) and Skia's transformed-text handling.
func (c *Context) needsOutlineTransform() bool {
	m := c.matrix
	// Rotation or shear: off-axis columns in the affine matrix.
	if m.B != 0 || m.D != 0 {
		return true
	}
	// Non-uniform scale (including flip): X and Y scale magnitudes differ.
	a := m.A
	if a < 0 {
		a = -a
	}
	e := m.E
	if e < 0 {
		e = -e
	}
	return a != e
}

// drawStringMultiFace renders fallback font runs (X.06). Each contiguous run
// uses a single FontSource so the GPU glyph-mask path can operate correctly.
func (c *Context) drawStringMultiFace(mf *text.MultiFace, s string, x, y float64) {
	if mf == nil || s == "" {
		return
	}
	defer c.setGPUClipRect()()
	defer c.applyTextDecorations(s, x, y)

	orig := c.face
	for _, run := range mf.Runs(s) {
		if run.Text == "" || run.Face == nil {
			continue
		}
		c.face = run.Face
		// Call DrawString with a concrete face (no MultiFace recursion).
		c.drawStringResolved(run.Text, x+run.X, y)
	}
	c.face = orig
}

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
			if sta.DrawShapedGlyphMaskText(target, face, glyphs, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
				c.recordGPUOp()
				return
			}
		}
	}

	a := Accelerator()
	if a != nil {
		if sta, ok := a.(GPUShapedTextAccelerator); ok {
			if sta.DrawShapedGlyphMaskText(target, face, glyphs, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
				c.recordGPUOp()
				return
			}
		}
	}

	// Fallback: reconstruct string is not possible from glyphs,
	// so render each glyph outline through the fill pipeline.
	c.drawShapedGlyphsAsOutlines(glyphs, face, x, y)
}

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
		if rc.DrawText(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
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
	if ta.DrawText(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
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
		if rc.DrawGlyphMaskText(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
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
	if gma.DrawGlyphMaskText(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
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
			if ata.DrawGlyphMaskTransformText(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale, devicePath) == nil {
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
		if ata.DrawGlyphMaskTransformText(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale, devicePath) == nil {
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
func (c *Context) trackTextDamage(s string, x, y float64) {
	if c.face == nil || s == "" {
		return
	}
	m := c.face.Metrics()
	ascent := m.Ascent
	if ascent < 0 {
		ascent = -ascent
	}
	descent := m.Descent
	if descent < 0 {
		descent = -descent
	}
	width := c.face.Advance(s)
	if width <= 0 {
		width = m.XHeight * float64(len(s)) //nolint:mnd // conservative fallback
	}
	// x is the text baseline origin in user space (Y-down baseline, ascent above).
	bounds := image.Rect(
		int(math.Floor(x))-1,
		int(math.Floor(y-ascent))-1,
		int(math.Ceil(x+width))+1,
		int(math.Ceil(y+descent))+1,
	)
	if bounds.Empty() {
		return
	}
	c.trackDamage(bounds)
}

// tryGPUGlyphMaskTextAliased attempts to render aliased text via the GPU glyph
// mask pipeline. Same Tier 6 pipeline but with binary (0/255) rasterization.
// Returns true if text was successfully queued for aliased glyph mask rendering.
func (c *Context) tryGPUGlyphMaskTextAliased(s string, x, y float64) bool {
	col := FromColor(c.currentColor())
	target := c.gpuRenderTarget()
	if rc := c.gpuCtxOps(); rc != nil {
		if rc.DrawGlyphMaskTextAliased(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
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
	if ata.DrawGlyphMaskTextAliased(target, c.face, s, x, y, col, c.totalMatrix(), c.deviceScale) == nil {
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
func (c *Context) selectTextStrategy() TextMode {
	if m, ok := forceTextMode(); ok {
		return m
	}
	if c.textMode != TextModeAuto {
		return c.textMode
	}
	// GOGPU_RENDER_MODE=cpu: route text straight to the CPU bitmap pipeline —
	// the window present (pixmap upload) must carry glyphs without the GPU
	// session's glyph-mask/MSDF renderers.
	if CPUOnlyMode() {
		return TextModeBitmap
	}
	// Transform-quality routing: rotated/sheared/non-uniform transforms render
	// glyph outlines as paths (GPU stencil+cover / CPU Tier 2). Skia's
	// kTransformedMask (rotated bitmap quads) is not enabled: the glyph-mask
	// pipeline renders rotated quads incorrectly under multi-draw accumulation
	// (observed black flooding in the 9-cell render_text_transform example) —
	// routing into it would render falsely. Rotated text stays on outlines.
	if c.needsOutlineTransform() {
		return TextModeVector
	}
	if c.shouldUseGlyphMask() {
		return TextModeGlyphMask
	}
	return TextModeAuto
}

// shouldUseGlyphMask returns true when auto-selection should prefer glyph
// mask rendering (Tier 6). Conditions: GPU with glyph mask support, font size
// in device pixels <= glyphMaskMaxSize. Rotated/sheared/non-uniform matrices
// were already routed to outlines by selectTextStrategy, so no axis check is
// needed here.
func (c *Context) shouldUseGlyphMask() bool {
	a := Accelerator()
	if a == nil {
		return false
	}
	if _, ok := a.(GPUGlyphMaskAccelerator); !ok {
		return false
	}

	if c.face == nil {
		return false
	}

	return c.glyphMaskDeviceSize() <= glyphMaskMaxSize
}

// glyphMaskDeviceSize returns the effective font size in device pixels,
// accounting for deviceScale and the Y scale component of the matrix.
func (c *Context) glyphMaskDeviceSize() float64 {
	deviceSize := c.face.Size() * c.deviceScale
	absScale := c.matrix.E
	if absScale < 0 {
		absScale = -absScale
	}
	if absScale != 0 {
		deviceSize *= absScale
	}
	return deviceSize
}

// DrawStringAnchored draws text with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
//
//	(0, 0) = top-left
//	(0.5, 0.5) = center
//	(1, 1) = bottom-right
//
// The text is positioned so that the anchor point is at (x, y).
func (c *Context) DrawStringAnchored(s string, x, y, ax, ay float64) {
	if c.face == nil {
		return
	}

	// Measure the text and calculate offset based on anchor.
	// The anchor maps linearly within the text bounding box:
	//   ay=0 → y is the top of the text (baseline = y + ascent)
	//   ay=0.5 → y is the vertical center (baseline = y + ascent - h/2)
	//   ay=1 → y is the bottom (baseline = y + ascent - h)
	// Formula: baseline = y + ascent - ay * h
	// where h = ascent + descent (visual bounding box, no lineGap).
	w, _ := text.Measure(s, c.face)
	metrics := c.face.Metrics()
	h := metrics.Ascent + metrics.Descent
	x -= w * ax
	y = y + metrics.Ascent - ay*h

	// Delegate to DrawString which handles TextMode routing.
	c.DrawString(s, x, y)
}

// MeasureString returns the dimensions of text in pixels.
// Returns (width, height) where:
//   - width is the horizontal advance of the text
//   - height is the line height (ascent + descent + line gap)
//
// If no font has been set, returns (0, 0).
func (c *Context) MeasureString(s string) (w, h float64) {
	if c.face == nil {
		return 0, 0
	}
	return text.Measure(s, c.face)
}

// LoadFontFace loads a font from a file and sets it as the current font.
// The size is specified in points.
//
// Deprecated: Use text.NewFontSourceFromFile and SetFont instead.
// This method is provided for convenience and backward compatibility.
//
// Example (new way):
//
//	source, err := text.NewFontSourceFromFile("font.ttf")
//	if err != nil {
//	    return err
//	}
//	face := source.Face(12.0)
//	ctx.SetFont(face)
func (c *Context) LoadFontFace(path string, points float64) error {

	source, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return err
	}
	c.face = source.Face(points)
	return nil
}

// LoadFontFaceWithVariations loads a variable font and applies axis values (X.09).
// Example: dc.LoadFontFaceWithVariations("Cantarell.ttf", 16, text.NewFontVariation("wght", 700))
func (c *Context) LoadFontFaceWithVariations(path string, points float64, vars ...text.FontVariation) error {
	source, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return err
	}
	if len(vars) > 0 {
		c.face = source.Face(points, text.WithVariations(vars...))
	} else {
		c.face = source.Face(points)
	}
	return nil
}

// FontVariationAxes returns variation axes for the current face's font source (X.09).
func (c *Context) FontVariationAxes() []text.VariationAxis {
	if c == nil || c.face == nil {
		return nil
	}
	src := c.face.Source()
	if src == nil {
		return nil
	}
	return src.VariationAxes()
}

// WordWrap wraps text to fit within the given width using word boundaries.
// Returns a slice of strings, one per wrapped line.
// If no font face is set, returns the input string as a single-element slice.
//
// This method is compatible with fogleman/gg's WordWrap.
func (c *Context) WordWrap(s string, w float64) []string {
	if c.face == nil {
		return []string{s}
	}
	results := text.WrapText(s, c.face, w, text.WrapWord)
	lines := make([]string, len(results))
	for i, r := range results {
		lines[i] = r.Text
	}
	return lines
}

// MeasureMultilineString measures text that may contain newlines.
// The lineSpacing parameter is a multiplier for the font's natural line height
// (1.0 = normal spacing, 1.5 = 50% extra space between lines).
// Returns (width, height) where width is the maximum line width and height
// is the total height of all lines with the given line spacing.
// If no font face is set, returns (0, 0).
//
// This method is compatible with fogleman/gg's MeasureMultilineString.
func (c *Context) MeasureMultilineString(s string, lineSpacing float64) (width, height float64) {
	if c.face == nil {
		return 0, 0
	}
	lines := splitLines(s)
	metrics := c.face.Metrics()
	fh := metrics.LineHeight()
	for _, line := range lines {
		lw, _ := text.Measure(line, c.face)
		if lw > width {
			width = lw
		}
	}
	// Visual height: ascent above first baseline + (n-1) inter-line gaps + descent below last baseline.
	n := float64(len(lines))
	height = (n-1)*fh*lineSpacing + metrics.Ascent + metrics.Descent
	return
}

// DrawStringWrapped wraps text to the given width and draws it with alignment.
// The text is positioned relative to (x, y) using the anchor (ax, ay):
//
//	(0, 0) = top-left of the text block is at (x, y)
//	(0.5, 0.5) = center of the text block is at (x, y)
//	(1, 1) = bottom-right of the text block is at (x, y)
//
// The lineSpacing parameter multiplies the font's natural line height
// (1.0 = normal, 1.5 = 50% extra space between lines).
// The align parameter controls horizontal alignment within the wrapped width.
// If no font face is set, this method does nothing.
//
// This method is compatible with fogleman/gg's DrawStringWrapped.
func (c *Context) DrawStringWrapped(s string, x, y, ax, ay, width, lineSpacing float64, align Align) {
	if c.face == nil {
		return
	}
	lines := c.WordWrap(s, width)
	if len(lines) == 0 {
		return
	}

	metrics := c.face.Metrics()
	fh := metrics.LineHeight()

	// Visual height of the text block:
	// - (n-1) inter-line gaps of fh*lineSpacing
	// - ascent above first baseline + descent below last baseline
	n := float64(len(lines))
	h := (n-1)*fh*lineSpacing + metrics.Ascent + metrics.Descent

	// Adjust starting position by anchor (bounding-box model):
	//   ay=0 → y is the top of the block (first baseline = y + ascent)
	//   ay=0.5 → y is the vertical center
	//   ay=1 → y is the bottom of the block
	// Formula: first_baseline = y + ascent - ay * h
	x -= ax * width
	y = y + metrics.Ascent - ay*h

	// Adjust x base for alignment
	switch align {
	case text.AlignCenter:
		x += width / 2
	case text.AlignRight:
		x += width
	}

	for _, line := range lines {
		drawX := x
		switch align {
		case text.AlignCenter:
			lw, _ := c.MeasureString(line)
			drawX = x - lw/2
		case text.AlignRight:
			lw, _ := c.MeasureString(line)
			drawX = x - lw
		}
		c.DrawString(line, drawX, y)
		y += fh * lineSpacing
	}
}

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
	// c.deviceScale to get the physical pixel size for the face.
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
func (c *Context) drawStringBitmap(s string, x, y float64) {
	p := c.totalMatrix().TransformPoint(Pt(x, y))
	c.flushGPUAccelerator()
	face := c.face
	if c.deviceScale != 1.0 {
		if source := c.face.Source(); source != nil {
			face = source.Face(c.face.Size() * c.deviceScale)
		}
	}
	text.DrawWithEmoji(c.pixmap, s, face, p.X, p.Y, c.currentColor())
}

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
	deviceFace := source.Face(deviceSize * c.deviceScale)
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
	if c.deviceScale != 1.0 {
		if source := c.face.Source(); source != nil {
			face = source.Face(c.face.Size() * c.deviceScale)
		}
	}

	// Uniform scale: create device-size face for crisp rendering.
	if !m.IsTranslationOnly() && m.B == 0 && m.D == 0 && m.A == m.E && m.A > 0 {
		deviceSize := c.face.Size() * m.A
		if source := c.face.Source(); source != nil {
			face = source.Face(deviceSize * c.deviceScale)
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
// each junction. All enterprise text renderers (Skia, Cairo, Qt) recommend or
// default to round joins for stroked text.
//
// Unlike DrawString, StrokeString always uses vector outlines regardless of the
// current TextMode — MSDF and glyph mask pipelines cannot produce stroked text.
// If no font has been set with SetFont, this function does nothing.
//
// Enterprise pattern: matches HTML5 Canvas strokeText(), Cairo show_text() + stroke(),
// Skia SkPaint::kStroke_Style + drawTextBlob.
func (c *Context) StrokeString(s string, x, y float64) {
	if c.face == nil {
		return
	}
	path := c.textOutlinePath(s, x, y)
	if path == nil {
		return
	}

	// User matrix only — doStroke() applies deviceMatrix via deviceSpacePath().
	transformedPath := path.Transform(c.matrix)
	c.trackDamage(transformedPath.Bounds())

	// Set GPU scissor rect for rectangular clips.
	defer c.setGPUClipRect()()

	// Save and restore context path — doStroke uses c.path.
	savedPath := c.path
	c.path = transformedPath
	_ = c.doStroke()
	c.path = savedPath
}

// StrokeStringAnchored strokes text outlines with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
//
//	(0, 0) = top-left
//	(0.5, 0.5) = center
//	(1, 1) = bottom-right
//
// The text is positioned so that the anchor point is at (x, y).
// The stroke width, cap, join, and dash come from the current paint state.
// Always uses vector outlines regardless of TextMode.
func (c *Context) StrokeStringAnchored(s string, x, y, ax, ay float64) {
	if c.face == nil {
		return
	}

	w, _ := text.Measure(s, c.face)
	metrics := c.face.Metrics()
	h := metrics.Ascent + metrics.Descent
	x -= w * ax
	y = y + metrics.Ascent - ay*h

	c.StrokeString(s, x, y)
}

// TextPath returns a user-space Path containing the vector outlines of text s
// positioned at (x, y) where y is the baseline. The returned path can be filled,
// stroked, or used for hit-testing with the caller's own pipeline.
//
// Returns nil if no font is set or the text produces no outlines.
//
// Enterprise pattern: matches HTML5 Canvas addText() (proposed), Cairo text_path(),
// Skia SkTextBlob → SkPath (via getPath).
func (c *Context) TextPath(s string, x, y float64) *Path {
	if c.face == nil {
		return nil
	}
	return c.textOutlinePath(s, x, y)
}

// textOutlinePath builds a user-space Path containing glyph outlines for text s
// at user-space position (x, y). Uses glyph cache for efficiency.
// Returns nil if the font has no FontSource (e.g. MultiFace) or the text
// produces no outlines.
func (c *Context) textOutlinePath(s string, x, y float64) *Path {
	source := c.face.Source()
	if source == nil {
		return nil
	}

	extractor := c.ensureOutlineExtractor()
	parsed := source.Parsed()
	fontSize := c.face.Size()

	// Use glyph cache to avoid repeated outline extraction.
	cache := c.ensureGlyphCache()
	fontID := computeTextFontID(source)
	var sizeKey int16
	switch {
	case fontSize < 0:
		sizeKey = 0
	case fontSize > 32767:
		sizeKey = 32767
	default:
		sizeKey = int16(fontSize) //nolint:gosec // bounds checked above
	}

	path := NewPath()
	hasContour := false

	shaped := text.Shape(s, c.face)
	for _, sg := range shaped {
		cacheKey := text.OutlineCacheKey{
			FontID:  fontID,
			GID:     sg.GID,
			Size:    sizeKey,
			Hinting: text.HintingNone,
		}
		outline := cache.GetOrCreate(cacheKey, func() *text.GlyphOutline {
			o, err := extractor.ExtractOutline(parsed, sg.GID, fontSize)
			if err != nil || o == nil || o.IsEmpty() {
				return nil
			}
			return o
		})
		if outline == nil {
			continue
		}

		gx := x + sg.X

		for _, seg := range outline.Segments {
			// sfnt.LoadGlyph returns Y-down coordinates (screen convention):
			// Y=0 at baseline, Y<0 above baseline, Y>0 below baseline.
			// So we ADD outlineY to baseline (no flip needed).
			switch seg.Op {
			case text.OutlineOpMoveTo:
				if hasContour {
					path.Close()
				}
				path.MoveTo(gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y))
				hasContour = true
			case text.OutlineOpLineTo:
				path.LineTo(gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y))
			case text.OutlineOpQuadTo:
				path.QuadraticTo(
					gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y),
					gx+float64(seg.Points[1].X), y+float64(seg.Points[1].Y))
			case text.OutlineOpCubicTo:
				path.CubicTo(
					gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y),
					gx+float64(seg.Points[1].X), y+float64(seg.Points[1].Y),
					gx+float64(seg.Points[2].X), y+float64(seg.Points[2].Y))
			}
		}
	}
	if hasContour {
		path.Close()
	}
	if path.isEmpty() {
		return nil
	}
	return path
}

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
func (c *Context) ensureOutlineExtractor() *text.OutlineExtractor {
	if c.outlineExtractor == nil {
		c.outlineExtractor = text.NewOutlineExtractor()
	}
	return c.outlineExtractor
}

// ensureGlyphCache lazily initializes the glyph cache reference.
// Uses the global shared cache to benefit from cross-Context reuse.
func (c *Context) ensureGlyphCache() *text.GlyphCache {
	if c.glyphCache == nil {
		c.glyphCache = text.GetGlobalGlyphCache()
	}
	return c.glyphCache
}

// computeTextFontID generates a stable hash identifier for a font source.
// Uses FNV-1a hash of font name and glyph count as a lightweight fingerprint.
// Same algorithm as internal/gpu/gpu_text.go:computeFontID.
func computeTextFontID(source *text.FontSource) uint64 {
	if source == nil {
		return 0
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s:%d", source.Name(), source.Parsed().NumGlyphs())
	return h.Sum64()
}

// fontHeight returns the font's natural line height (ascent + descent + line gap).
func (c *Context) fontHeight() float64 {
	if c.face == nil {
		return 0
	}
	return c.face.Metrics().LineHeight()
}

// splitLines splits text by line breaks, normalizing \r\n and \r to \n.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}
