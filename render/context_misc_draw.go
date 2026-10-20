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
	"math"

	"github.com/energye/gpui/render/text"
)

// DrawColor fills the current clip with col using mode. It is the render-layer
// counterpart of Skia's Canvas::drawColor / Flutter's Canvas.drawColor: unlike
// DrawPaint it ignores the current paint (brush, alpha, blend mode) and paints
// the whole clip, not the current path.
//
// The current path and paint state are left untouched.
func (c *Context) DrawColor(col RGBA, mode BlendMode) {
	if c == nil || c.pixmap == nil {
		return
	}
	var p *Paint
	if c.paint != nil {
		p = c.paint.Clone()
	} else {
		p = NewPaint()
	}
	p.SetBrush(Solid(col))
	p.BlendMode = mode
	p.Antialias = false
	// The fill is a pixel-aligned device-space rectangle. A stale transform
	// scale or coverage callback from the caller's paint must not leak in;
	// doFill installs the active clip/mask coverage itself.
	p.TransformScale = 1
	p.ClipCoverage = nil
	p.MaskCoverage = nil
	c.fillClipWithPaint(p)
}

// DrawPaint fills the current clip with the current paint. It is the
// render-layer counterpart of Skia's Canvas::drawPaint / Flutter's
// Canvas.drawPaint: the current path is ignored (and preserved), while the
// brush, alpha, blend mode, fill rule and layer opacity are honored.
func (c *Context) DrawPaint() {
	if c == nil || c.pixmap == nil {
		return
	}
	var p *Paint
	if c.paint != nil {
		p = c.paint.Clone()
	} else {
		p = NewPaint()
	}
	// The clip cover is a device-space rectangle; a caller's transform scale
	// belongs to the current path, not to DrawPaint's full-clip fill.
	p.TransformScale = 1
	c.fillClipWithPaint(p)
}

// fillClipWithPaint paints the current clip with p without touching the
// caller's current path or paint. The cover is built directly in post-CTM
// user space: deviceMatrix maps it to the exact device-space clip AABB, so a
// rotated/scaled CTM cannot leave an uncovered wedge. Clip coverage is applied
// by doFill, so the small AA expansion is clipped back to the real shape.
func (c *Context) fillClipWithPaint(p *Paint) {
	if c == nil || c.pixmap == nil || p == nil {
		return
	}
	// Advanced blend modes need destination sampling. The direct GPU fill
	// path only has fixed-function blend state and would silently drop the
	// draw; isolate it in a layer and composite that layer with the requested
	// mode (the renderer's dual-tex advanced path handles it on GPU).
	if IsAdvancedBlendMode(p.BlendMode) {
		c.PushLayer(p.BlendMode, 1.0)
		q := p.Clone()
		q.BlendMode = BlendNormal
		c.fillClipWithPaintDirect(q)
		c.PopLayer()
		return
	}
	c.fillClipWithPaintDirect(p)
}

// fillClipWithPaintDirect covers the active clip with p using a temporary
// post-CTM cover path. The caller must have resolved advanced blend modes.
func (c *Context) fillClipWithPaintDirect(p *Paint) {
	if c == nil || c.pixmap == nil || p == nil {
		return
	}

	deviceRect := image.Rect(0, 0, c.pixmap.Width(), c.pixmap.Height())
	if b, ok := c.clipDeviceBounds(); ok {
		// Two device pixels of AA pad: clip coverage (rect, RRect SDF, path
		// mask or difference mask) multiplies the fill back to the true edge.
		deviceRect = b.Inset(-2).Intersect(deviceRect)
	}
	if deviceRect.Empty() {
		return
	}

	invDevice := c.deviceMatrix.Invert()
	cover := NewPath()
	corners := [4]Point{
		{X: float64(deviceRect.Min.X), Y: float64(deviceRect.Min.Y)},
		{X: float64(deviceRect.Max.X), Y: float64(deviceRect.Min.Y)},
		{X: float64(deviceRect.Max.X), Y: float64(deviceRect.Max.Y)},
		{X: float64(deviceRect.Min.X), Y: float64(deviceRect.Max.Y)},
	}
	for i, corner := range corners {
		q := invDevice.TransformPoint(corner)
		if i == 0 {
			cover.MoveTo(q.X, q.Y)
		} else {
			cover.LineTo(q.X, q.Y)
		}
	}
	cover.Close()

	savedPath, savedPaint := c.path, c.paint
	c.path, c.paint = cover, p
	defer func() {
		c.path, c.paint = savedPath, savedPaint
	}()
	_ = c.Fill()
}

// ClipBounds returns the current clip bounds in local logical coordinates,
// rounded outward to whole pixels. It is modeled after
// Canvas.getLocalClipBounds: with no clip the full canvas is returned, and the
// result is the conservative AABB in the current user coordinate system.
//
// The clip stack itself stores device-space bounds; this method undoes the
// total CTM for callers. It never mutates clip or paint state.
func (c *Context) ClipBounds() image.Rectangle {
	if c == nil || c.pixmap == nil {
		return image.Rectangle{}
	}
	dev := image.Rect(0, 0, c.pixmap.Width(), c.pixmap.Height())
	if b, ok := c.clipDeviceBounds(); ok {
		dev = b
	}

	inv := c.totalMatrix().Invert()
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	corners := [4]Point{
		{X: float64(dev.Min.X), Y: float64(dev.Min.Y)},
		{X: float64(dev.Max.X), Y: float64(dev.Min.Y)},
		{X: float64(dev.Max.X), Y: float64(dev.Max.Y)},
		{X: float64(dev.Min.X), Y: float64(dev.Max.Y)},
	}
	for _, corner := range corners {
		q := inv.TransformPoint(corner)
		if q.X < minX {
			minX = q.X
		}
		if q.X > maxX {
			maxX = q.X
		}
		if q.Y < minY {
			minY = q.Y
		}
		if q.Y > maxY {
			maxY = q.Y
		}
	}
	return image.Rect(
		int(math.Floor(minX)),
		int(math.Floor(minY)),
		int(math.Ceil(maxX)),
		int(math.Ceil(maxY)),
	)
}

// DrawShadow draws a path-shaped drop shadow only; the path itself is not
// drawn. path is in the current user coordinate system (the same contract as
// FillPath). elevation is in logical pixels and transparentOccluder is kept
// for Flutter/Skia API parity.
//
// The implementation follows Flutter Impeller's directional spot-shadow model:
// the silhouette is translated down by the elevation, blurred by
// (800/600)*elevation (the Skia light-radius ratio used by Flutter), and
// composited with the tonal spot color (25% of col before tonal adjustment).
// The shadow is built in an isolated layer so it never includes the occluder
// and is clipped by the current clip. It is an approximation of Skia's full
// ambient+spot projection: there is no ambient pass and no perspective
// silhouette transform. transparentOccluder has no effect because this API
// never renders the occluder itself (Impeller's drawShadow ignores it too).
func (c *Context) DrawShadow(path *Path, col RGBA, elevation float64, transparentOccluder bool) {
	if c == nil || c.pixmap == nil || path == nil || path.isEmpty() || elevation <= 0 || col.A <= 0 {
		return
	}
	_ = transparentOccluder

	spot := shadowSpotColor(col)
	if spot.A <= 0 {
		return
	}

	dpr := c.DeviceScale()
	if dpr <= 0 {
		dpr = 1
	}
	// Skia/Impeller compute the shadow in device space: lift the occluder by
	// elevation*dpr device pixels and blur with the same device-space scale.
	// Convert the lift back to local units so the current CTM (including DPR)
	// produces that device offset.
	tm := c.totalMatrix()
	scaleY := math.Hypot(tm.B, tm.E)
	if scaleY <= 1e-9 {
		return
	}
	offsetY := elevation * dpr / scaleY
	blur := (800.0 / 600.0) * elevation * dpr

	c.PushLayerIsolated(1.0)
	c.Push()
	c.Translate(0, offsetY)
	savedPaint := c.paint
	p := NewPaint()
	p.SetBrush(Solid(spot))
	p.BlendMode = BlendNormal
	c.paint = p
	c.DrawPath(path)
	_ = c.Fill()
	c.paint = savedPaint
	c.Pop()

	c.ApplyBlur(blur)
	c.PopLayer()
}

// shadowSpotColor ports Flutter Impeller's ComputeTonalColors spot color path
// (DlDispatcherBase::drawShadow). Input and output are straight-alpha RGBA.
func shadowSpotColor(col RGBA) RGBA {
	a := clamp01(col.A * 0.25)
	maxC := math.Max(col.R, math.Max(col.G, col.B))
	minC := math.Min(col.R, math.Min(col.G, col.B))
	luminance := (minC + maxC) * 0.5

	alphaAdjust := (2.6 + (-2.66667+1.06667*a)*a) * a
	colorAlpha := (3.544762 + (-4.891428+2.3466*luminance)*luminance) * luminance
	colorAlpha = clamp01(alphaAdjust * colorAlpha)
	greyscaleAlpha := clamp01(a * (1 - 0.4*luminance))
	colorScale := colorAlpha * (1 - greyscaleAlpha)
	tonalAlpha := colorScale + greyscaleAlpha
	unpremul := 0.0
	if tonalAlpha != 0 {
		unpremul = colorScale / tonalAlpha
	}
	return RGBA{
		R: clamp01(col.R * unpremul),
		G: clamp01(col.G * unpremul),
		B: clamp01(col.B * unpremul),
		A: clamp01(tonalAlpha),
	}
}

// DrawParagraph draws a laid-out text paragraph at (x, y), where y is the
// paragraph's top edge (the layout's line baselines are added internally).
// Glyphs are rendered from the pre-shaped runs in layout, so shaping and line
// breaking are performed once by render/text. The current paint color is used
// for all runs; per-run colors/styles are a ParagraphBuilder concern that the
// render layer does not own yet.
//
// The current path and paint are left untouched. Color-glyph runs are not
// supported by this entry point (use DrawShapedColorGlyphs for those).
func (c *Context) DrawParagraph(layout *text.Layout, x, y float64) {
	if c == nil || layout == nil || len(layout.Lines) == 0 {
		return
	}
	for i := range layout.Lines {
		line := &layout.Lines[i]
		baseline := y + line.Y
		for j := range line.Runs {
			run := &line.Runs[j]
			if run.Face == nil || len(run.Glyphs) == 0 {
				continue
			}
			c.DrawShapedGlyphs(run.Glyphs, run.Face, x, baseline)
		}
	}
}
