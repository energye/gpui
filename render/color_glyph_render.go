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
	"errors"
	"image"
	"image/color"
	"math"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

// COLRv1 paint-graph rasterization (OT 1.9.1, static evaluation at the
// default axis instance): the text package owns the data plane (table
// parsing, glyph type, raster cache); this package owns the pixel
// production through render.Context — gradients, clip, and composite all
// reuse the standard brush/clip/blend machinery, zero render-layer additions.

// InstallCOLRv1Renderer wires the COLRv1 paint-graph raster into the text
// package. Explicit and idempotent, same wiring discipline as
// svg.InstallSVGTableRenderer — call once where color glyph rendering is
// available.
func InstallCOLRv1Renderer() {
	text.SetCOLRv1Renderer(renderCOLRv1Glyph)
}

// maxCOLRCanvasPixels bounds the raster canvas (fail-closed against
// pathological clip boxes): 4096×4096 at the requested ppem.
const maxCOLRCanvasPixels = 4096

// colrRenderCtx carries one glyph's rendering state.
type colrRenderCtx struct {
	font       text.ParsedFont
	ext        *text.OutlineExtractor
	palette    []emoji.Color
	fg         color.RGBA
	bounds     Rect // design-space (y-up) canvas bounds
	upemCached float64
	scale      float64
	canvasW    int
	canvasH    int
	depth      int
}

// canvasSize returns the device pixel canvas size.
func (ctx *colrRenderCtx) canvasSize() (int, int) { return ctx.canvasW, ctx.canvasH }

// upem returns the font's units per em (fixed across one render).
func (ctx *colrRenderCtx) upem() float64 { return ctx.upemCached }

// mapX/mapY map design-space (y-up) coordinates to device pixels:
// px = (x−xmin)·s, py = (ymax−y)·s. The renderer draws in DEVICE space —
// no base transform sits on the dc — so every geometry piece (outlines,
// brushes, clip rects, fills) is mapped explicitly and the brush color
// evaluation space is unambiguous.
func (ctx *colrRenderCtx) mapX(x float64) float64 { return (x - ctx.bounds.Min.X) * ctx.scale }
func (ctx *colrRenderCtx) mapY(y float64) float64 { return (ctx.bounds.Max.Y - y) * ctx.scale }

// deviceMatrix conjugates a design-space affine into device space:
// D = Map ∘ M ∘ Map⁻¹ (render.Matrix is row-major: x' = A·x+B·y+C).
func (ctx *colrRenderCtx) deviceMatrix(m Matrix) Matrix {
	s := ctx.scale
	tx := -ctx.bounds.Min.X * s
	ty := ctx.bounds.Max.Y * s
	mapM := Matrix{A: s, B: 0, C: tx, D: 0, E: -s, F: ty}
	mapInv := Matrix{A: 1 / s, B: 0, C: -tx / s, D: 0, E: -1 / s, F: ty / s}
	return mulMatrix(mulMatrix(mapM, m), mapInv)
}

// renderCOLRv1Glyph is the text.COLRv1Renderer implementation: it resolves
// the CPAL palette from the font (spec: a COLR table without CPAL is
// ignored) and delegates to renderCOLRv1WithPalette.
func renderCOLRv1Glyph(font text.ParsedFont, glyph *emoji.COLRGlyph, upem, ppem float64, paletteIndex int, fg color.RGBA) (*image.RGBA, float32, float32, error) {
	if font == nil || glyph == nil || glyph.Paint == nil || upem <= 0 || ppem <= 0 {
		return nil, 0, 0, errors.New("render: invalid COLRv1 glyph render request")
	}
	cf, ok := font.(text.ColorFont)
	if !ok {
		return nil, 0, 0, errors.New("render: COLRv1 font has no color tables")
	}
	palette := cf.Palette(paletteIndex)
	if palette == nil {
		// OT 1.9.1: "If the COLR table is present in a font but no CPAL
		// table exists, then the COLR table is ignored."
		return nil, 0, 0, errors.New("render: COLRv1 glyph without CPAL palette (table ignored per spec)")
	}
	return renderCOLRv1WithPalette(font, glyph, upem, ppem, palette, fg)
}

// renderCOLRv1WithPalette renders the paint graph against an explicit
// palette (test seam; production goes through renderCOLRv1Glyph).
func renderCOLRv1WithPalette(font text.ParsedFont, glyph *emoji.COLRGlyph, upem, ppem float64, palette []emoji.Color, fg color.RGBA) (*image.RGBA, float32, float32, error) {
	if font == nil || glyph == nil || glyph.Paint == nil || upem <= 0 || ppem <= 0 {
		return nil, 0, 0, errors.New("render: invalid COLRv1 glyph render request")
	}

	// Canvas bounds: the ClipList box when present (spec: use it to size
	// the surface), otherwise the graph's boundedness computation.
	var bounds Rect
	if glyph.Clip != nil {
		bounds = Rect{
			Min: Point{X: glyph.Clip.XMin, Y: glyph.Clip.YMin},
			Max: Point{X: glyph.Clip.XMax, Y: glyph.Clip.YMax},
		}
	} else {
		b, bounded := paintGraphBounds(glyph.Paint, Identity(), font, upem, 0)
		if !bounded || b.Min.X >= b.Max.X || b.Min.Y >= b.Max.Y {
			return nil, 0, 0, errors.New("render: COLRv1 glyph is unbounded and has no clip box")
		}
		bounds = b
	}
	if bounds.Min.X >= bounds.Max.X || bounds.Min.Y >= bounds.Max.Y {
		return nil, 0, 0, errors.New("render: COLRv1 glyph bounds are empty")
	}

	s := ppem / upem
	w := int(math.Ceil(bounds.Width() * s))
	h := int(math.Ceil(bounds.Height() * s))
	if w <= 0 || h <= 0 || w*h > maxCOLRCanvasPixels*maxCOLRCanvasPixels {
		return nil, 0, 0, errors.New("render: COLRv1 glyph canvas out of range")
	}

	// Baseline pen position in frame pixels: design (0,0) through the
	// mapping px = (x−xmin)·s, py = (ymax−y)·s.
	penX := (0 - bounds.Min.X) * s
	penY := (bounds.Max.Y - 0) * s

	ctx := &colrRenderCtx{
		font:       font,
		ext:        &text.OutlineExtractor{},
		palette:    palette,
		fg:         fg,
		bounds:     bounds,
		upemCached: upem,
		scale:      s,
		canvasW:    w,
		canvasH:    h,
	}
	dc := NewContext(w, h, WithCPUOnly())
	if glyph.Clip != nil {
		// Clip box in device pixels.
		dc.ClipRect(ctx.mapX(glyph.Clip.XMin), ctx.mapY(glyph.Clip.YMax),
			ctx.mapX(glyph.Clip.XMax)-ctx.mapX(glyph.Clip.XMin),
			ctx.mapY(glyph.Clip.YMin)-ctx.mapY(glyph.Clip.YMax))
	}
	ctx.renderPaint(dc, glyph.Paint)
	frame, isRGBA := dc.Image().(*image.RGBA)
	if !isRGBA {
		return nil, 0, 0, errors.New("render: COLRv1 glyph readback is not RGBA")
	}
	return CropImageToInk(frame, penX, penY)
}

// renderPaint renders one paint node and its sub-graph onto dc (design
// space: y-up font units; the base dc mapping flips to pixels).
func (ctx *colrRenderCtx) renderPaint(dc *Context, n *emoji.PaintNode) {
	if n == nil {
		return
	}
	ctx.depth++
	defer func() { ctx.depth-- }()
	if ctx.depth > maxCOLRPaintRenderDepth {
		return
	}

	switch n.Format {
	case emoji.PaintColrLayers:
		// Bottom-up z-order, composed with simple alpha blending (= direct
		// drawing onto one surface).
		for _, ref := range n.Children {
			ctx.renderPaint(dc, ref.Node)
		}

	case emoji.PaintSolid, emoji.PaintVarSolid:
		c := ctx.resolveColor(n.PaletteIndex, n.Alpha)
		dc.SetColor(c)
		ctx.fillSurface(dc)

	case emoji.PaintLinearGradient, emoji.PaintVarLinearGradient:
		brush := ctx.buildLinearBrush(n)
		if brush == nil {
			return // ill-formed per spec: p0==p1, p0==p2, or near-parallel
		}
		dc.SetFillBrush(brush)
		ctx.fillSurface(dc)

	case emoji.PaintRadialGradient, emoji.PaintVarRadialGradient:
		brush := ctx.buildRadialBrush(n)
		if brush == nil {
			return // identical circles per spec: paint nothing
		}
		dc.SetFillBrush(brush)
		ctx.fillSurface(dc)

	case emoji.PaintSweepGradient, emoji.PaintVarSweepGradient:
		brush := ctx.buildSweepBrush(n)
		if brush == nil {
			return
		}
		dc.SetFillBrush(brush)
		ctx.fillSurface(dc)

	case emoji.PaintGlyph:
		path := ctx.glyphDevicePath(n.GlyphID)
		if path == nil {
			return
		}
		dc.Push()
		// DrawPath feeds the outline through the path verbs (transform-
		// aware); SetPath's raw append breaks clip rasterization when the
		// clip region is a strict subset of the filled area.
		dc.DrawPath(path)
		dc.Clip()
		ctx.renderPaint(dc, n.Child)
		dc.Pop()

	case emoji.PaintColrGlyph:
		// Child already resolved (cycle-guarded at parse).
		ctx.renderPaint(dc, n.Child)

	case emoji.PaintComposite:
		ctx.renderComposite(dc, n)

	default:
		// Transform family (12-31): apply the design-space matrix
		// conjugated into device space.
		dc.Push()
		dc.Transform(ctx.deviceMatrix(Matrix{A: n.XX, B: n.XY, C: n.DX, D: n.YX, E: n.YY, F: n.DY}))
		ctx.renderPaint(dc, n.Child)
		dc.Pop()
	}
}

// maxCOLRPaintRenderDepth mirrors the parse-side depth guard.
const maxCOLRPaintRenderDepth = 64

// fillSurface paints the current color/brush over the whole device canvas
// (the active clip constrains it to the enclosing PaintGlyph region).
func (ctx *colrRenderCtx) fillSurface(dc *Context) {
	dc.DrawRectangle(0, 0, float64(ctx.canvasW), float64(ctx.canvasH))
	dc.Fill()
}

// resolveColor resolves a palette reference with its COLRv1 alpha.
func (ctx *colrRenderCtx) resolveColor(paletteIndex uint16, alpha float64) color.RGBA {
	var base color.RGBA
	switch {
	case paletteIndex == emoji.ForegroundPaletteIndex:
		base = ctx.fg
	case int(paletteIndex) < len(ctx.palette):
		base = ctx.palette[paletteIndex].ToRGBA()
	default:
		return color.RGBA{} // out-of-range palette index: transparent
	}
	a := clampF01(alpha)
	return color.RGBA{
		R: base.R,
		G: base.G,
		B: base.B,
		A: uint8(float64(base.A) * a),
	}
}

// buildLinearBrush maps the spec's 3-point linear gradient onto the 2-point
// brush: p3 = orthogonal projection of the p0→p1 vector onto the line
// through p0 perpendicular to p0→p2 (the spec's stated equivalence).
// Geometry is mapped to device space (the brush evaluates there).
func (ctx *colrRenderCtx) buildLinearBrush(n *emoji.PaintNode) Brush {
	vx, vy := n.X1-n.X0, n.Y1-n.Y0
	wx, wy := n.X2-n.X0, n.Y2-n.Y0
	vLen := math.Hypot(vx, vy)
	wLen := math.Hypot(wx, wy)
	if vLen == 0 || wLen == 0 {
		return nil
	}
	cross := vx*wy - vy*wx
	if math.Abs(cross) <= 1e-9*vLen*wLen {
		return nil // near-parallel: ill-formed, must not render
	}
	// Unit normal to p0→p2; projection length of v onto it.
	nx, ny := wy/wLen, -wx/wLen
	t := vx*nx + vy*ny
	p3x, p3y := n.X0+t*nx, n.Y0+t*ny

	b := NewLinearGradientBrush(ctx.mapX(n.X0), ctx.mapY(n.Y0), ctx.mapX(p3x), ctx.mapY(p3y))
	b.Stops = ctx.colorStops(n.Line)
	b.Extend = extendMode(n.Line.Extend)
	return b
}

// buildRadialBrush maps the two-circle definition onto the brush's
// focal/center form: circle 0 (c0, r0) = focal/start, circle 1 (c1, r1) =
// center/end. Geometry is mapped to device space (uniform scale, so radii
// scale by s).
func (ctx *colrRenderCtx) buildRadialBrush(n *emoji.PaintNode) Brush {
	if n.X0 == n.X1 && n.Y0 == n.Y1 && n.R0 == n.R1 {
		return nil // identical circles: paint nothing
	}
	s := ctx.scale
	b := NewRadialGradientBrush(ctx.mapX(n.X1), ctx.mapY(n.Y1), n.R0*s, n.R1*s)
	b.Center = Point{X: ctx.mapX(n.X1), Y: ctx.mapY(n.Y1)}
	b.Focus = Point{X: ctx.mapX(n.X0), Y: ctx.mapY(n.Y0)}
	b.Stops = ctx.colorStops(n.Line)
	b.Extend = extendMode(n.Line.Extend)
	return b
}

// buildSweepBrush maps the sweep definition (angles already in counter-
// clockwise degrees with the spec bias resolved). The design space is
// y-up; the device space is y-down, so the sweep direction flips:
// device angle = −design angle.
func (ctx *colrRenderCtx) buildSweepBrush(n *emoji.PaintNode) Brush {
	if n.StartAngleDeg == n.EndAngleDeg && n.Line.Extend != emoji.ExtendPad {
		return nil // repeat/reflect with equal angles: nothing to draw
	}
	b := NewSweepGradientBrush(ctx.mapX(n.CenterX), ctx.mapY(n.CenterY), -n.StartAngleDeg*math.Pi/180.0)
	b.EndAngle = -n.EndAngleDeg * math.Pi / 180.0
	b.Stops = ctx.colorStops(n.Line)
	b.Extend = extendMode(n.Line.Extend)
	return b
}

// colorStops converts a COLRv1 color line into brush stops, ordered by
// offset (the spec's application order; stable sort keeps first-below /
// last-above semantics for duplicate offsets).
func (ctx *colrRenderCtx) colorStops(line emoji.ColorLine) []ColorStop {
	if len(line.Stops) == 0 {
		return nil
	}
	out := make([]ColorStop, 0, len(line.Stops))
	for _, s := range line.Stops {
		c := ctx.resolveColor(s.PaletteIndex, s.Alpha)
		out = append(out, ColorStop{
			Offset: s.Offset,
			Color: RGBA{
				R: float64(c.R) / 255,
				G: float64(c.G) / 255,
				B: float64(c.B) / 255,
				A: float64(c.A) / 255,
			},
		})
	}
	stableSortStops(out)
	return out
}

// glyphDevicePath extracts a glyph outline as a device-space path.
// ExtractOutline yields y-down points (glyph ink above the baseline is
// NEGATIVE y); COLRv1 paint geometry is y-up, so every y flips sign
// before the design→device mapping.
func (ctx *colrRenderCtx) glyphDevicePath(glyphID uint16) *Path {
	if ctx.font == nil {
		return nil
	}
	outline, err := ctx.ext.ExtractOutline(ctx.font, text.GlyphID(glyphID), ctx.upem())
	if err != nil || outline == nil || outline.SegmentCount() == 0 {
		return nil
	}
	flip := func(y float32) float64 { return ctx.mapY(-float64(y)) }
	fx := func(x float32) float64 { return ctx.mapX(float64(x)) }
	b := BuildPath()
	segs := outline.Segments
	closeSub := func() { b.Close() }
	for i, seg := range segs {
		switch seg.Op {
		case text.OutlineOpMoveTo:
			if i > 0 {
				closeSub()
			}
			if i+1 < len(segs) && segs[i+1].Op == text.OutlineOpMoveTo {
				continue // degenerate contour (svg text.go same policy)
			}
			if i+1 >= len(segs) {
				closeSub()
				continue
			}
			b.MoveTo(fx(seg.Points[0].X), flip(seg.Points[0].Y))
		case text.OutlineOpLineTo:
			b.LineTo(fx(seg.Points[0].X), flip(seg.Points[0].Y))
		case text.OutlineOpQuadTo:
			b.QuadTo(fx(seg.Points[0].X), flip(seg.Points[0].Y),
				fx(seg.Points[1].X), flip(seg.Points[1].Y))
		case text.OutlineOpCubicTo:
			b.CubicTo(fx(seg.Points[0].X), flip(seg.Points[0].Y),
				fx(seg.Points[1].X), flip(seg.Points[1].Y),
				fx(seg.Points[2].X), flip(seg.Points[2].Y))
		}
	}
	closeSub()
	return b.Path()
}

// renderComposite isolates source and backdrop into offscreen canvases,
// composites them with the mode, and draws the result back in device space.
func (ctx *colrRenderCtx) renderComposite(dc *Context, n *emoji.PaintNode) {
	if n.Mode == emoji.CompositeDest {
		// Result = backdrop: nothing the source adds survives.
		ctx.renderPaint(dc, n.Backdrop)
		return
	}
	mode := compositeModeToBlend(n.Mode)

	backdrop := ctx.renderLeg(n.Backdrop)
	source := ctx.renderLeg(n.Source)
	if backdrop == nil && source == nil {
		return
	}
	w, h := ctx.canvasSize()
	leg := NewContext(w, h, WithCPUOnly())
	leg.SetTransform(Identity()) // compose in device space: full-canvas images
	if backdrop != nil {
		leg.DrawImage(backdrop, 0, 0)
	}
	if source != nil {
		leg.DrawImageEx(source, DrawImageOptions{X: 0, Y: 0, BlendMode: mode})
	}
	composed, isRGBA := leg.Image().(*image.RGBA)
	if !isRGBA {
		return
	}
	// Draw back in device space (the composite result is a full-canvas
	// pixel image; the enclosing transform/clip must not displace it).
	saved := dc.GetTransform()
	dc.SetTransform(Identity())
	buf := ImageBufFromImage(composed)
	dc.DrawImageEx(buf, DrawImageOptions{
		X:         0,
		Y:         0,
		DstWidth:  float64(w),
		DstHeight: float64(h),
	})
	dc.SetTransform(saved)
}

// renderLeg renders one composite leg into its own offscreen RGBA canvas.
// The enclosing affine transform commutes with the per-pixel composite, so
// legs render with the base mapping only.
func (ctx *colrRenderCtx) renderLeg(root *emoji.PaintNode) *ImageBuf {
	if root == nil {
		return nil
	}
	w, h := ctx.canvasSize()
	leg := NewContext(w, h, WithCPUOnly())
	sub := &colrRenderCtx{
		font:       ctx.font,
		ext:        ctx.ext,
		palette:    ctx.palette,
		fg:         ctx.fg,
		bounds:     ctx.bounds,
		upemCached: ctx.upemCached,
		scale:      ctx.scale,
		canvasW:    ctx.canvasW,
		canvasH:    ctx.canvasH,
	}
	sub.renderPaint(leg, root)
	img, isRGBA := leg.Image().(*image.RGBA)
	if !isRGBA {
		return nil
	}
	return ImageBufFromImage(img)
}

// compositeModeToBlend maps the COLRv1 CompositeMode onto the render
// BlendMode family (full coverage: 12 Porter-Duff + 16 blend modes).
func compositeModeToBlend(mode uint8) BlendMode {
	switch mode {
	case emoji.CompositeClear:
		return BlendClear
	case emoji.CompositeSrc:
		return BlendCopy
	case emoji.CompositeSrcOver:
		return BlendNormal
	case emoji.CompositeDestOver:
		return BlendDestinationOver
	case emoji.CompositeSrcIn:
		return BlendSourceIn
	case emoji.CompositeDestIn:
		return BlendDestinationIn
	case emoji.CompositeSrcOut:
		return BlendSourceOut
	case emoji.CompositeDestOut:
		return BlendDestinationOut
	case emoji.CompositeSrcAtop:
		return BlendSourceAtop
	case emoji.CompositeDestAtop:
		return BlendDestinationAtop
	case emoji.CompositeXor:
		return BlendXor
	case emoji.CompositePlus:
		return BlendPlus
	case emoji.CompositeScreen:
		return BlendScreen
	case emoji.CompositeOverlay:
		return BlendOverlay
	case emoji.CompositeDarken:
		return BlendDarken
	case emoji.CompositeLighten:
		return BlendLighten
	case emoji.CompositeColorDodge:
		return BlendColorDodge
	case emoji.CompositeColorBurn:
		return BlendColorBurn
	case emoji.CompositeHardLight:
		return BlendHardLight
	case emoji.CompositeSoftLight:
		return BlendSoftLight
	case emoji.CompositeDifference:
		return BlendDifference
	case emoji.CompositeExclusion:
		return BlendExclusion
	case emoji.CompositeMultiply:
		return BlendMultiply
	case emoji.CompositeHSLHue:
		return BlendHue
	case emoji.CompositeHSLSaturation:
		return BlendSaturation
	case emoji.CompositeHSLColor:
		return BlendColor
	case emoji.CompositeHSLLuminosity:
		return BlendLuminosity
	default:
		return BlendClear // spec: unrecognized value → CLEAR
	}
}

// extendMode maps the COLRv1 extend enum onto the render ExtendMode.
func extendMode(e uint8) ExtendMode {
	switch e {
	case emoji.ExtendRepeat:
		return ExtendRepeat
	case emoji.ExtendReflect:
		return ExtendReflect
	default:
		return ExtendPad
	}
}

// paintGraphBounds computes the design-space bounds of a paint graph per
// the spec's boundedness rules. bounded=false means the graph can paint
// outside any finite region (unbounded fill without a glyph clip).
func paintGraphBounds(n *emoji.PaintNode, m Matrix, font text.ParsedFont, upem float64, depth int) (Rect, bool) {
	if n == nil || depth > maxCOLRPaintRenderDepth {
		return Rect{}, true
	}
	transformRect := func(r Rect) Rect {
		corners := [4]Point{
			{X: r.Min.X, Y: r.Min.Y}, {X: r.Max.X, Y: r.Min.Y},
			{X: r.Max.X, Y: r.Max.Y}, {X: r.Min.X, Y: r.Max.Y},
		}
		out := Rect{}
		for i, c := range corners {
			p := m.TransformPoint(c)
			if i == 0 {
				out = Rect{Min: p, Max: p}
			} else {
				if p.X < out.Min.X {
					out.Min.X = p.X
				}
				if p.Y < out.Min.Y {
					out.Min.Y = p.Y
				}
				if p.X > out.Max.X {
					out.Max.X = p.X
				}
				if p.Y > out.Max.Y {
					out.Max.Y = p.Y
				}
			}
		}
		return out
	}
	union := func(a, b Rect, ab, bb bool) (Rect, bool) {
		switch {
		case !ab:
			return b, bb
		case !bb:
			return a, true
		default:
			return Rect{
				Min: Point{X: math.Min(a.Min.X, b.Min.X), Y: math.Min(a.Min.Y, b.Min.Y)},
				Max: Point{X: math.Max(a.Max.X, b.Max.X), Y: math.Max(a.Max.Y, b.Max.Y)},
			}, true
		}
	}

	switch n.Format {
	case emoji.PaintGlyph:
		outline, err := (&text.OutlineExtractor{}).ExtractOutline(font, text.GlyphID(n.GlyphID), upem)
		if err != nil || outline == nil {
			return Rect{}, true
		}
		// ExtractOutline is y-down (ink above baseline = negative y); flip
		// into the COLRv1 y-up design space.
		b := outline.Bounds
		return transformRect(Rect{
			Min: Point{X: float64(b.MinX), Y: -float64(b.MaxY)},
			Max: Point{X: float64(b.MaxX), Y: -float64(b.MinY)},
		}), true

	case emoji.PaintSolid, emoji.PaintVarSolid,
		emoji.PaintLinearGradient, emoji.PaintVarLinearGradient,
		emoji.PaintRadialGradient, emoji.PaintVarRadialGradient,
		emoji.PaintSweepGradient, emoji.PaintVarSweepGradient:
		return Rect{}, false // inherently unbounded

	case emoji.PaintColrLayers:
		total := Rect{}
		bounded := true
		first := true
		for _, ref := range n.Children {
			r, bd := paintGraphBounds(ref.Node, m, font, upem, depth+1)
			if first {
				total, bounded, first = r, bd, false
				continue
			}
			total, bounded = union(total, r, bounded, bd)
		}
		return total, bounded

	case emoji.PaintComposite:
		br, bb := paintGraphBounds(n.Backdrop, m, font, upem, depth+1)
		sr, sb := paintGraphBounds(n.Source, m, font, upem, depth+1)
		switch n.Mode {
		case emoji.CompositeClear:
			return Rect{}, true
		case emoji.CompositeSrc, emoji.CompositeSrcOut:
			return sr, sb
		case emoji.CompositeDest, emoji.CompositeDestOut:
			return br, bb
		case emoji.CompositeSrcIn, emoji.CompositeDestIn:
			if bb {
				return union(br, sr, true, sb)
			}
			return br, bb
		default:
			r, bounded := union(br, sr, bb, sb)
			return r, bounded
		}

	default:
		// PaintColrGlyph and the transform family: child with composed
		// matrix (transforms apply to the child sub-graph).
		cm := m
		if n.Format >= emoji.PaintTransform && n.Format <= emoji.PaintSkewAroundCenter {
			cm = mulMatrix(m, Matrix{A: n.XX, B: n.XY, C: n.DX, D: n.YX, E: n.YY, F: n.DY})
		}
		return paintGraphBounds(n.Child, cm, font, upem, depth+1)
	}
}

// mulMatrix composes two matrices: apply b first, then a. render.Matrix is
// row-major (x' = A·x + B·y + C; y' = D·x + E·y + F).
func mulMatrix(a, b Matrix) Matrix {
	return Matrix{
		A: a.A*b.A + a.B*b.D,
		B: a.A*b.B + a.B*b.E,
		C: a.A*b.C + a.B*b.F + a.C,
		D: a.D*b.A + a.E*b.D,
		E: a.D*b.B + a.E*b.E,
		F: a.D*b.C + a.E*b.F + a.F,
	}
}

// clampF01 clamps to [0, 1].
func clampF01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// stableSortStops sorts stops by offset (stable for duplicate offsets).
func stableSortStops(stops []ColorStop) {
	for i := 1; i < len(stops); i++ {
		for j := i; j > 0 && stops[j].Offset < stops[j-1].Offset; j-- {
			stops[j], stops[j-1] = stops[j-1], stops[j]
		}
	}
}

// CropImageToInk crops a rendered color glyph frame to its non-transparent
// pixels and returns the baseline-relative origin: the position of the pen
// point (penX, penY in frame pixel coordinates) inside the cropped image,
// using the ColorRasterImage conventions (bitmap left = pen + OriginX,
// bitmap top = baseline − OriginY).
func CropImageToInk(img *image.RGBA, penX, penY float64) (*image.RGBA, float32, float32, error) {
	minX, minY, maxX, maxY, hasInk := imageAlphaBounds(img)
	if !hasInk {
		return nil, 0, 0, errors.New("render: color glyph rasterizes empty")
	}
	cropped := imageCropRGBA(img, minX, minY, maxX, maxY)
	// Round the baseline origin to whole pixels: color glyphs are placed
	// at integer positions (same discipline as the bitmap paths' int()
	// conversion), so GPU quads land identically on every backend instead
	// of drifting on subpixel rasterization differences.
	ox := math.Round(float64(minX) - penX)
	oy := math.Round(penY - float64(minY))
	return cropped, float32(ox), float32(oy), nil
}

// imageAlphaBounds finds the smallest box containing all pixels with
// non-zero alpha (false when the image is fully transparent).
func imageAlphaBounds(img *image.RGBA) (minX, minY, maxX, maxY int, has bool) {
	b := img.Bounds()
	minX, minY, maxX, maxY = b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[(y-b.Min.Y)*img.Stride:]
		for x := b.Min.X; x < b.Max.X; x++ {
			if row[(x-b.Min.X)*4+3] != 0 {
				has = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return
}

// imageCropRGBA returns a copy of the pixel box [minX,minY]-[maxX,maxY].
func imageCropRGBA(img *image.RGBA, minX, minY, maxX, maxY int) *image.RGBA {
	w, h := maxX-minX+1, maxY-minY+1
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcOff := (minY+y-img.Rect.Min.Y)*img.Stride + (minX-img.Rect.Min.X)*4
		copy(out.Pix[y*out.Stride:y*out.Stride+w*4], img.Pix[srcOff:srcOff+w*4])
	}
	return out
}
