//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

import (
	"image"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// A retained compositor needs two facts about a recording before it allocates
// anything: the footprint the content lands in (to size a texture and report
// damage) and whether the content can paint at all (so invisible layers never
// get one). Both are computed here, on the UI thread, in one forward pass over
// the command stream at FinishRecording — measuring them at raster time would
// mean reading shared font faces and live command data from the raster thread,
// which is the sharing this layer exists to remove.
//
// The walk is conservative only in the direction that can waste memory, never
// clip content:
//   - clips shrink what reaches the screen but are ignored (bounds stay a
//     superset of the geometry, as Picture.Bounds always was);
//   - paint alpha never shrinks the footprint (alpha belongs to visibility);
//   - a blur or drop shadow acts on the whole surface, so a recording carrying
//     one reports the full canvas rather than guessing at the filter's spread.
//
// Stroke bands and text ink get the treatment the live rasterizer gives them:
// the band is outset in user space before the CTM is applied (so a scaled
// playback scales the band too), and a glyph run's extent comes from its own
// face's metrics plus per-glyph ink boxes, because a MultiFace's aggregate
// metrics describe only its first face while CJK ink rises above a Latin ascent.

// contentSummary is the outcome of the command-stream walk.
type contentSummary struct {
	bounds  image.Rectangle
	visible bool
}

// summarize walks the recorded stream with a simulated CTM — the same matrix
// algebra playback performs (ConcatCommand composes as current.multiply(m)) —
// and folds every drawing command's footprint into one rectangle.
func summarize(width, height int, cmds []Command, pool *ResourcePool) contentSummary {
	w := &contentWalk{ctm: Identity(), pool: pool}
	for _, cmd := range cmds {
		w.step(cmd)
	}
	out := contentSummary{visible: w.visible}
	if w.has {
		out.bounds = image.Rect(floorInt(w.minX), floorInt(w.minY), ceilInt(w.maxX), ceilInt(w.maxY))
	}
	if w.surfaceWide && width > 0 && height > 0 {
		out.bounds = out.bounds.Union(image.Rect(0, 0, width, height))
	}
	return out
}

// contentWalk carries the state the footprint depends on: the simulated CTM with
// its Save/Restore stack, the accumulated float extent (converted to integers
// only once, at the end) and the visibility flag.
type contentWalk struct {
	ctm   Matrix
	stack []Matrix
	pool  *ResourcePool

	minX, minY, maxX, maxY float64
	has                    bool
	visible                bool
	surfaceWide            bool
}

// step applies one command. Each drawing case states its own visibility rule
// (mirroring what replay skips) and then folds geometry regardless of alpha.
func (w *contentWalk) step(cmd Command) {
	switch c := cmd.(type) {
	case SaveCommand:
		w.stack = append(w.stack, w.ctm)
	case RestoreCommand:
		if n := len(w.stack); n > 0 {
			w.ctm = w.stack[n-1]
			w.stack = w.stack[:n-1]
		}
	case ConcatCommand:
		w.ctm = w.ctm.Multiply(c.Matrix)

	case FillRectCommand:
		w.note(rectCanFill(c.Rect) && brushCanPaint(w.brush(c.Brush)))
		w.addRect(c.Rect, 0)
	case StrokeRectCommand:
		w.note(rectCanStroke(c.Rect) && brushCanPaint(w.brush(c.Brush)))
		w.addRect(c.Rect, strokeOutset(c.Stroke))
	case FillPathCommand:
		w.note(pathPaintable(w.getPath(c.Path)) && brushCanPaint(w.brush(c.Brush)))
		w.addPath(c.Path, 0)
	case StrokePathCommand:
		w.note(pathPaintable(w.getPath(c.Path)) && brushCanPaint(w.brush(c.Brush)))
		w.addPath(c.Path, strokeOutset(c.Stroke))

	case DrawImageCommand:
		img := w.image(c.Image)
		w.note(imageCanPaint(img))
		if r, ok := imageDstRect(img, c.Opts); ok {
			w.addRect(r, 0)
		}
	case DrawImageNineCommand:
		w.note(imageCanPaint(w.image(c.Image)))
		w.addRect(c.Dst, 0)
	case DrawImageQuadCommand:
		w.note(imageCanPaint(w.image(c.Image)))
		w.addPointsBox(c.Corners)
	case DrawImageRoundedCommand:
		img := w.image(c.Image)
		w.note(imageCanPaint(img))
		if iw, ih := img.Bounds(); iw > 0 && ih > 0 {
			w.addRect(NewRect(c.X, c.Y, float64(iw), float64(ih)), 0)
		}
	case DrawImageCircularCommand:
		img := w.image(c.Image)
		w.note(imageCanPaint(img))
		if c.Radius > 0 {
			w.addRect(NewRect(c.CX-c.Radius, c.CY-c.Radius, 2*c.Radius, 2*c.Radius), 0)
		}
	case DrawAtlasCommand:
		w.note(imageCanPaint(w.image(c.Image)) && len(c.Sprites) > 0)
		w.addSprites(c.Sprites)
	case DrawDepthSpritesCommand:
		sprites := make([]render.AtlasSprite, len(c.Sprites))
		for i, d := range c.Sprites {
			sprites[i] = d.Sprite
		}
		w.note(imageCanPaint(w.image(c.Image)) && len(sprites) > 0)
		w.addSprites(sprites)
	case DrawVerticesCommand:
		w.note(vertexRunCanPaint(c.Positions, c.Colors, w.brush(c.Brush)))
		w.addPoints(c.Positions)
	case DrawMeshCommand:
		w.note(vertexRunCanPaint(c.Positions, c.Colors, w.brush(c.Brush)) && indicesCanDraw(c.Indices))
		w.addPoints(c.Positions)

	case DrawTextCommand:
		face := w.face(c.Font)
		w.note(c.Text != "" && face != nil && brushCanPaint(w.brush(c.Brush)))
		w.addGlyphRun(c.Text, c.X, c.Y, face, 0)
	case StrokeTextCommand:
		face := w.face(c.Font)
		w.note(c.Text != "" && face != nil && brushCanPaint(w.brush(c.Brush)))
		w.addGlyphRun(c.Text, c.X, c.Y, face, strokeOutset(c.Stroke))
	case DrawGlyphsCommand:
		face := w.face(c.Face)
		w.note(len(c.Glyphs) > 0 && face != nil && brushCanPaint(w.brush(c.Brush)))
		w.addShapedRun(c.Glyphs, face, c.X, c.Y, 0)

	case ApplyBlurCommand:
		if c.Radius > 0 {
			w.surfaceWide = true
		}
	case ApplyDropShadowCommand:
		w.surfaceWide = true

	case SetClipCommand, ClearClipCommand, ClipRectCommand, ClipRoundRectCommand,
		ClipRectOpCommand, ClipPathOpCommand,
		SetTextDecorationCommand, SetTextModeCommand, SetLCDLayoutCommand,
		PushLayerCommand, PushLayerIsolatedCommand,
		PushBackdropLayerCommand, PopLayerCommand, ApplyColorMatrixCommand,
		ApplyGrayscaleCommand,
		SetFillStyleCommand, SetStrokeStyleCommand, SetLineWidthCommand,
		SetLineCapCommand, SetLineJoinCommand, SetMiterLimitCommand,
		SetDashCommand, SetFillRuleCommand, SetAntiAliasCommand:
		// State, not drawing: these change what later content looks like where it
		// already is, so they contribute neither footprint nor visibility.
	}
}

// note records that this command can put a pixel on the surface.
func (w *contentWalk) note(canPaint bool) {
	if canPaint {
		w.visible = true
	}
}

// --------------------------------------------------------------------------
// Resource access
// --------------------------------------------------------------------------

func (w *contentWalk) image(ref ImageRef) *render.ImageBuf {
	if w.pool == nil || !ref.IsValid() {
		return nil
	}
	return w.pool.GetImage(ref)
}

func (w *contentWalk) face(ref FontRef) text.Face {
	if w.pool == nil || !ref.IsValid() {
		return nil
	}
	return w.pool.GetFont(ref)
}

func (w *contentWalk) brush(ref BrushRef) Brush {
	if w.pool == nil || !ref.IsValid() {
		return nil
	}
	return w.pool.GetBrush(ref)
}

func (w *contentWalk) getPath(ref PathRef) *render.Path {
	if w.pool == nil || !ref.IsValid() {
		return nil
	}
	return w.pool.GetPath(ref)
}

// brushCanPaint mirrors the replay's own skip conditions: a zero-alpha solid
// paints nothing, a gradient needs a non-zero stop, and a nil or pattern brush
// does paint — ToRenderBrush renders a nil brush as solid black, and a pattern
// brush samples its image (replay routes it to SetFillPattern, not to a colour).
// A pattern whose image is gone paints nothing, and reporting that here would
// only save a texture nobody can predict at record time, so the safe direction
// (waste a texture, never drop content) is kept.
func brushCanPaint(b Brush) bool {
	switch br := b.(type) {
	case nil:
		return true
	case SolidBrush:
		return br.Color.A != 0
	case *LinearGradientBrush:
		return stopsCanPaint(br.Stops)
	case *RadialGradientBrush:
		return stopsCanPaint(br.Stops)
	case *SweepGradientBrush:
		return stopsCanPaint(br.Stops)
	default:
		return true
	}
}

func stopsCanPaint(stops []GradientStop) bool {
	for _, s := range stops {
		if s.Color.A != 0 {
			return true
		}
	}
	return false
}

// imageCanPaint is the replay's guard: a buffer that was disposed between
// recording and finishing contributes nothing.
func imageCanPaint(img *render.ImageBuf) bool {
	return img != nil && !img.Disposed()
}

// vertexRunCanPaint mirrors Context.DrawVertices: fewer than three vertices
// rasterize no triangle, a full colour-per-vertex list paints with those colours
// and ignores the paint, and a shorter list falls back to the recorded brush.
func vertexRunCanPaint(positions []render.Point, colors []render.RGBA, brush Brush) bool {
	if len(positions) < 3 {
		return false
	}
	if len(colors) == len(positions) {
		for _, c := range colors {
			if c.A != 0 {
				return true
			}
		}
		return false
	}
	return brushCanPaint(brush)
}

// indicesCanDraw mirrors the indexed branch of Context.DrawMesh: an empty index
// list means the positions themselves are the triangle list, and anything short
// of one complete group draws nothing.
func indicesCanDraw(indices []uint16) bool {
	return len(indices) == 0 || len(indices) >= 3
}

// rectCanFill / rectCanStroke state what the rasterizer has to work with: a fill
// needs area in both axes, a stroke only needs a segment to follow. A zero-width
// rectangle is recorded just the same (Recorder mirrors Context, which validates
// nothing here), so this is decided from the geometry, not from the command count.
func rectCanFill(r Rect) bool {
	return r.MaxX > r.MinX && r.MaxY > r.MinY
}

func rectCanStroke(r Rect) bool {
	return r.MaxX > r.MinX || r.MaxY > r.MinY
}

// pathPaintable is the same rule one level up: a path holding only move-to and
// close verbs is not empty, yet has no segment for a fill or a stroke to put
// down, so it paints no pixel.
func pathPaintable(p *render.Path) bool {
	if p == nil {
		return false
	}
	paintable := false
	p.Iterate(func(verb render.PathVerb, _ []float64) {
		if verb != render.MoveTo && verb != render.Close {
			paintable = true
		}
	})
	return paintable
}

// --------------------------------------------------------------------------
// Geometry
// --------------------------------------------------------------------------

// addRect folds a user-space rect, optionally outset for a stroke band, through
// the current CTM.
func (w *contentWalk) addRect(r Rect, outset float64) {
	w.addPointsBox([4]render.Point{
		{X: r.MinX - outset, Y: r.MinY - outset},
		{X: r.MaxX + outset, Y: r.MinY - outset},
		{X: r.MaxX + outset, Y: r.MaxY + outset},
		{X: r.MinX - outset, Y: r.MaxY + outset},
	})
}

// addSprites folds one atlas batch; every sprite carries its own destination
// box, pivot and rotation.
func (w *contentWalk) addSprites(sprites []render.AtlasSprite) {
	for _, s := range sprites {
		r, ok := spriteRect(s)
		if !ok {
			continue
		}
		w.addRect(r, 0)
	}
}

// addPointsBox transforms user-space corners and unions their extent. A
// degenerate box (zero width or height) is skipped: it covers no pixel, and
// letting it set the extent would report bounds that no texture can use.
func (w *contentWalk) addPointsBox(pts [4]render.Point) {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		if !finitePoint(p) {
			return
		}
		tp := w.ctm.TransformPoint(p)
		minX = math.Min(minX, tp.X)
		minY = math.Min(minY, tp.Y)
		maxX = math.Max(maxX, tp.X)
		maxY = math.Max(maxY, tp.Y)
	}
	w.span(minX, minY, maxX, maxY)
}

// addPoints folds an arbitrary point list (vertex meshes).
func (w *contentWalk) addPoints(pts []render.Point) {
	if len(pts) == 0 {
		return
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		if !finitePoint(p) {
			return
		}
		tp := w.ctm.TransformPoint(p)
		minX = math.Min(minX, tp.X)
		minY = math.Min(minY, tp.Y)
		maxX = math.Max(maxX, tp.X)
		maxY = math.Max(maxY, tp.Y)
	}
	w.span(minX, minY, maxX, maxY)
}

// addPath folds a pooled path's own bounds, outset for a stroke band. Path
// bounds are the control-point box, which already contains the curve.
func (w *contentWalk) addPath(ref PathRef, outset float64) {
	path := w.getPath(ref)
	if path == nil || path.NumVerbs() == 0 {
		return
	}
	b := path.Bounds()
	w.addRect(Rect{
		MinX: float64(b.Min.X),
		MinY: float64(b.Min.Y),
		MaxX: float64(b.Max.X),
		MaxY: float64(b.Max.Y),
	}, outset)
}

// span unions a transformed extent, ignoring degenerate boxes.
func (w *contentWalk) span(minX, minY, maxX, maxY float64) {
	if minX >= maxX || minY >= maxY {
		return
	}
	if !w.has {
		w.minX, w.minY, w.maxX, w.maxY = minX, minY, maxX, maxY
		w.has = true
		return
	}
	w.minX = math.Min(w.minX, minX)
	w.minY = math.Min(w.minY, minY)
	w.maxX = math.Max(w.maxX, maxX)
	w.maxY = math.Max(w.maxY, maxY)
}

// --------------------------------------------------------------------------
// Text
// --------------------------------------------------------------------------

// addGlyphRun folds a string's footprint with the face it was recorded with. A
// recording has no context-font fallback — playback draws nothing without a
// face — so a missing face contributes no extent.
func (w *contentWalk) addGlyphRun(s string, x, y float64, face text.Face, outset float64) {
	if s == "" || face == nil {
		return
	}
	runs := []text.FaceRun{{Face: face, Text: s, X: 0}}
	if mf, ok := face.(*text.MultiFace); ok {
		runs = mf.Runs(s)
	}
	for _, run := range runs {
		if run.Face == nil || run.Text == "" {
			continue
		}
		m := run.Face.Metrics()
		w.addRect(Rect{
			MinX: x + run.X,
			MinY: y - m.Ascent,
			MaxX: x + run.X + run.Face.Advance(run.Text),
			MaxY: y + m.Descent,
		}, outset)
		// Per-glyph ink: accents, descenders and right-side bearings exceed the
		// metrics estimate. The pen advances on the whole-pixel grid the glyph
		// mask engine uses, and the 2px pad absorbs raster pixel-fit and AA.
		pen := 0.0
		for g := range run.Face.Glyphs(run.Text) {
			gb := g.Bounds
			if gb.MinX == 0 && gb.MaxX == 0 && gb.MinY == 0 && gb.MaxY == 0 {
				pen += math.Round(g.Advance)
				continue
			}
			w.addPointsBox([4]render.Point{
				{X: x + run.X + pen + gb.MinX - 2, Y: y + g.Y + gb.MinY - 2},
				{X: x + run.X + pen + gb.MaxX + 2, Y: y + g.Y + gb.MinY - 2},
				{X: x + run.X + pen + gb.MaxX + 2, Y: y + g.Y + gb.MaxY + 2},
				{X: x + run.X + pen + gb.MinX - 2, Y: y + g.Y + gb.MaxY + 2},
			})
			pen += math.Round(g.Advance)
		}
	}
}

// addShapedRun folds a pre-shaped glyph run: the glyphs already carry their
// final pen positions, so the extent comes from X/XAdvance plus the face's
// vertical metrics.
func (w *contentWalk) addShapedRun(glyphs []text.ShapedGlyph, face text.Face, x, y, outset float64) {
	if len(glyphs) == 0 || face == nil {
		return
	}
	minX, maxX := glyphs[0].X, glyphs[0].X+glyphs[0].XAdvance
	for _, g := range glyphs[1:] {
		minX = math.Min(minX, g.X)
		maxX = math.Max(maxX, g.X+g.XAdvance)
	}
	m := face.Metrics()
	w.addRect(Rect{
		MinX: x + minX,
		MinY: y - m.Ascent,
		MaxX: x + maxX,
		MaxY: y + m.Descent,
	}, outset)
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// strokeOutset is the user-space reach of a stroke band beyond the path itself:
// half the width, half again when the cap extends the endpoints, and for joins
// either half again (round/bevel) or the full miter length. A width ≤0 strokes
// at 1, matching replay.
func strokeOutset(st Stroke) float64 {
	width := st.Width
	if width <= 0 {
		width = 1
	}
	half := width / 2
	pad := half
	if st.Cap != LineCapButt {
		pad += half
	}
	if st.Join == LineJoinMiter {
		limit := st.MiterLimit
		if limit < 1 {
			limit = 1
		}
		pad = math.Max(pad, half*limit)
	} else {
		pad += half
	}
	return pad
}

// imageDstRect resolves where a recorded image draw lands: the destination size
// when given, otherwise the source rect's (or the buffer's) native size.
func imageDstRect(img *render.ImageBuf, opts render.DrawImageOptions) (Rect, bool) {
	srcW, srcH := 0.0, 0.0
	if opts.SrcRect != nil {
		srcW, srcH = float64(opts.SrcRect.Dx()), float64(opts.SrcRect.Dy())
	} else if imageCanPaint(img) {
		w, h := img.Bounds()
		srcW, srcH = float64(w), float64(h)
	}
	dstW, dstH := opts.DstWidth, opts.DstHeight
	if dstW <= 0 {
		dstW = srcW
	}
	if dstH <= 0 {
		dstH = srcH
	}
	if dstW <= 0 || dstH <= 0 {
		return Rect{}, false
	}
	return NewRect(opts.X, opts.Y, dstW, dstH), true
}

// spriteRect resolves one atlas sprite's destination box, honouring the pivot
// and rotation the batch applies (flips do not change an axis-aligned box).
func spriteRect(s render.AtlasSprite) (Rect, bool) {
	w, h := s.DstW, s.DstH
	if w <= 0 {
		w = s.SrcW
	}
	if h <= 0 {
		h = s.SrcH
	}
	if w <= 0 || h <= 0 {
		return Rect{}, false
	}
	px, py := s.DstX+s.PivotX, s.DstY+s.PivotY
	corners := [4]render.Point{
		{X: s.DstX, Y: s.DstY},
		{X: s.DstX + w, Y: s.DstY},
		{X: s.DstX + w, Y: s.DstY + h},
		{X: s.DstX, Y: s.DstY + h},
	}
	if s.Rot != 0 {
		cos, sin := math.Cos(s.Rot), math.Sin(s.Rot)
		for i, p := range corners {
			dx, dy := p.X-px, p.Y-py
			corners[i] = render.Point{X: px + dx*cos - dy*sin, Y: py + dx*sin + dy*cos}
		}
	}
	minX, minY := corners[0].X, corners[0].Y
	maxX, maxY := corners[0].X, corners[0].Y
	for _, p := range corners[1:] {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	return Rect{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}, true
}

func finitePoint(p render.Point) bool {
	return !math.IsNaN(p.X) && !math.IsNaN(p.Y) &&
		!math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0)
}

// floorInt/ceilInt clamp before converting, so a runaway coordinate cannot wrap
// into the opposite edge of the accumulated bounds.
func floorInt(v float64) int { return int(clampCoord(math.Floor(v))) }

func ceilInt(v float64) int { return int(clampCoord(math.Ceil(v))) }

const coordLimit = float64(math.MaxInt32)

func clampCoord(v float64) float64 {
	if v < -coordLimit {
		return -coordLimit
	}
	if v > coordLimit {
		return coordLimit
	}
	return v
}
