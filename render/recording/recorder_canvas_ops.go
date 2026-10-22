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

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// This file is the render.Canvas parity layer of the Recorder: every method
// here has the same name and signature as its *render.Context twin and records
// a command instead of rasterizing. Paint code written against a live context
// therefore runs unchanged against a recorder, which is what lets the UI thread
// build a display list and the raster thread only execute it.
//
// Coordinate rule (see Recorder's doc): geometry reaches the pool in user space.
// Verbs recorded through MoveTo/LineTo are already user-space, so they store
// as-is. Paths copied *raw* (SetPath/AppendPath) are the exception — a live
// Context does not apply its CTM to them, so the recording pre-multiplies by
// the inverse CTM and playback's concat stream cancels it again.

// Ensure the Recorder really is a draw target for the same paint code a live
// context accepts. This is the build-time half of the display-list contract;
// the behavioural half is the live-vs-replay pixel parity test.
var _ render.Canvas = (*Recorder)(nil)

// --------------------------------------------------------------------------
// Path loading
// --------------------------------------------------------------------------

// SetPath replaces the current path with p.
func (r *Recorder) SetPath(p *render.Path) {
	r.pathBasis = r.transform
	if p == nil {
		r.currentPath = render.NewPath()
		return
	}
	r.currentPath = r.rawPathSpace(p)
}

// AppendPath appends the elements of p to the current path.
func (r *Recorder) AppendPath(p *render.Path) {
	if p == nil {
		return
	}
	r.currentPath.Append(r.rawPathSpace(p))
}

// rawPathSpace re-expresses a caller-supplied path, whose coordinates a live
// Context would treat as already transformed, in the user space the recording
// keeps. Playback reapplies the CTM, so the two cancel.
func (r *Recorder) rawPathSpace(p *render.Path) *render.Path {
	return p.Transform(r.transform.Invert())
}

// DrawPath replays the elements of p through the current transform, replacing
// the current path — the transform-aware counterpart of SetPath.
func (r *Recorder) DrawPath(p *render.Path) {
	r.ClearPath()
	if p == nil {
		return
	}
	p.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			r.MoveTo(coords[0], coords[1])
		case render.LineTo:
			r.LineTo(coords[0], coords[1])
		case render.QuadTo:
			r.QuadraticTo(coords[0], coords[1], coords[2], coords[3])
		case render.CubicTo:
			r.CubicTo(coords[0], coords[1], coords[2], coords[3], coords[4], coords[5])
		case render.Close:
			r.ClosePath()
		}
	})
}

// FillPath replays p through the current transform, fills it and clears the path.
func (r *Recorder) FillPath(p *render.Path) error {
	r.DrawPath(p)
	return r.Fill()
}

// StrokePath replays p through the current transform, strokes it and clears the path.
func (r *Recorder) StrokePath(p *render.Path) error {
	r.DrawPath(p)
	return r.Stroke()
}

// --------------------------------------------------------------------------
// Clipping
// --------------------------------------------------------------------------

// ClipRect sets a rectangular clipping region in user space.
func (r *Recorder) ClipRect(x, y, w, h float64) {
	r.commands = append(r.commands, ClipRectCommand{Rect: NewRect(x, y, w, h)})
}

// --------------------------------------------------------------------------
// Images
// --------------------------------------------------------------------------

// DrawImage draws an image buffer at (x, y) in user space.
func (r *Recorder) DrawImage(img *render.ImageBuf, x, y float64) {
	r.DrawImageEx(img, render.DrawImageOptions{X: x, Y: y})
}

// DrawImageEx draws an image buffer with the full option set.
func (r *Recorder) DrawImageEx(img *render.ImageBuf, opts render.DrawImageOptions) {
	if img == nil || img.Disposed() {
		return
	}
	r.commands = append(r.commands, DrawImageCommand{
		Image: r.resources.AddImageBuf(img),
		Opts:  cloneDrawImageOptions(opts),
	})
}

// DrawImageNine stretches img's edges between center to fill the destination
// rectangle, keeping the corners at native size.
func (r *Recorder) DrawImageNine(img *render.ImageBuf, center image.Rectangle, dstX, dstY, dstW, dstH float64) {
	if img == nil || img.Disposed() {
		return
	}
	r.commands = append(r.commands, DrawImageNineCommand{
		Image:  r.resources.AddImageBuf(img),
		Center: center,
		Dst:    NewRect(dstX, dstY, dstW, dstH),
	})
}

// DrawImageQuad maps img onto an arbitrary quad.
func (r *Recorder) DrawImageQuad(img *render.ImageBuf, corners [4]render.Point, opts render.QuadDrawOptions) error {
	if img == nil || img.Disposed() {
		return nil
	}
	r.commands = append(r.commands, DrawImageQuadCommand{
		Image:   r.resources.AddImageBuf(img),
		Corners: corners,
		Opts:    opts,
	})
	return nil
}

// DrawImageRounded draws an image clipped to a rounded rectangle.
func (r *Recorder) DrawImageRounded(img *render.ImageBuf, x, y, radius float64) {
	if img == nil || img.Disposed() {
		return
	}
	r.commands = append(r.commands, DrawImageRoundedCommand{
		Image:  r.resources.AddImageBuf(img),
		X:      x,
		Y:      y,
		Radius: radius,
	})
}

// DrawImageCircular draws an image clipped to a circle.
func (r *Recorder) DrawImageCircular(img *render.ImageBuf, cx, cy, radius float64) {
	if img == nil || img.Disposed() {
		return
	}
	r.commands = append(r.commands, DrawImageCircularCommand{
		Image:  r.resources.AddImageBuf(img),
		CX:     cx,
		CY:     cy,
		Radius: radius,
	})
}

// cloneDrawImageOptions deep-copies the SrcRect pointer so later mutation of
// the caller's rectangle cannot rewrite an already-recorded command.
func cloneDrawImageOptions(opts render.DrawImageOptions) render.DrawImageOptions {
	if opts.SrcRect != nil {
		src := *opts.SrcRect
		opts.SrcRect = &src
	}
	return opts
}

// --------------------------------------------------------------------------
// Sprites and meshes
// --------------------------------------------------------------------------

// DrawAtlas draws a batch of sprites sampled from one image.
func (r *Recorder) DrawAtlas(img *render.ImageBuf, sprites []render.AtlasSprite) {
	r.DrawAtlasEx(img, sprites, render.AtlasDrawOptions{})
}

// DrawAtlasEx draws a sprite batch with options. Context reports which route it
// took (GPU batch or CPU fallback) only after rasterizing, so recording returns
// the zero result; the draw itself is replayed faithfully.
func (r *Recorder) DrawAtlasEx(img *render.ImageBuf, sprites []render.AtlasSprite, opts render.AtlasDrawOptions) (render.AtlasDrawResult, error) {
	if img == nil || img.Disposed() {
		return render.AtlasDrawResult{Skipped: true}, nil
	}
	r.commands = append(r.commands, DrawAtlasCommand{
		Image:   r.resources.AddImageBuf(img),
		Sprites: cloneAtlasSprites(sprites),
		Opts:    opts,
	})
	return render.AtlasDrawResult{}, nil
}

// DrawDepthSprites draws a depth-tagged sprite batch.
func (r *Recorder) DrawDepthSprites(img *render.ImageBuf, sprites []render.DepthSprite, opts render.DepthDrawOptions) (render.DepthDrawResult, error) {
	if img == nil || img.Disposed() {
		return render.DepthDrawResult{Skipped: true}, nil
	}
	r.commands = append(r.commands, DrawDepthSpritesCommand{
		Image:   r.resources.AddImageBuf(img),
		Sprites: cloneDepthSprites(sprites),
		Opts:    opts,
	})
	return render.DepthDrawResult{}, nil
}

// DrawVertices records an immediate-mode mesh. Context falls back to the
// current fill paint when colors is shorter than positions, so the paint at
// record time travels with the command.
func (r *Recorder) DrawVertices(positions []render.Point, colors []render.RGBA, mode render.VertexMode) {
	pos := make([]render.Point, len(positions))
	copy(pos, positions)
	col := make([]render.RGBA, len(colors))
	copy(col, colors)
	r.commands = append(r.commands, DrawVerticesCommand{
		Positions: pos,
		Colors:    col,
		Mode:      mode,
		Brush:     r.resources.AddBrush(r.fillBrush),
	})
}

func cloneAtlasSprites(in []render.AtlasSprite) []render.AtlasSprite {
	out := make([]render.AtlasSprite, len(in))
	copy(out, in)
	return out
}

func cloneDepthSprites(in []render.DepthSprite) []render.DepthSprite {
	out := make([]render.DepthSprite, len(in))
	copy(out, in)
	return out
}

// --------------------------------------------------------------------------
// Text
// --------------------------------------------------------------------------

// Font returns the current font face, mirroring render.Context.Font.
func (r *Recorder) Font() text.Face { return r.fontFace }

// SetTextDecoration sets the decoration bitset for text drawn afterwards.
// Decoration is Context state, so the recording replays the call in stream
// order; the recorder itself keeps no copy.
func (r *Recorder) SetTextDecoration(d render.TextDecoration) {
	r.commands = append(r.commands, SetTextDecorationCommand{Deco: d})
}

// DrawShapedGlyphs draws an already-shaped glyph run at the baseline origin.
func (r *Recorder) DrawShapedGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64) {
	r.recordGlyphs(glyphs, face, x, y, false)
}

// DrawShapedColorGlyphs draws a glyph run using each glyph's own colour.
func (r *Recorder) DrawShapedColorGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64) {
	r.recordGlyphs(glyphs, face, x, y, true)
}

func (r *Recorder) recordGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64, colored bool) {
	if len(glyphs) == 0 {
		return
	}
	run := make([]text.ShapedGlyph, len(glyphs))
	copy(run, glyphs)
	ref := FontRef(InvalidRef)
	if face != nil {
		ref = r.resources.AddFont(face)
	}
	r.commands = append(r.commands, DrawGlyphsCommand{
		Glyphs:  run,
		Face:    ref,
		X:       x,
		Y:       y,
		Colored: colored,
		Brush:   r.resources.AddBrush(r.fillBrush),
	})
}

// --------------------------------------------------------------------------
// Layers and post-effects
// --------------------------------------------------------------------------

// PushLayer opens a layer composited onto the parent with blend and opacity.
func (r *Recorder) PushLayer(blend render.BlendMode, opacity float64) {
	r.commands = append(r.commands, PushLayerCommand{Blend: blend, Opacity: opacity})
}

// PushLayerIsolated opens a forced true save-layer (Flutter saveLayer).
func (r *Recorder) PushLayerIsolated(opacity float64) {
	r.commands = append(r.commands, PushLayerIsolatedCommand{Opacity: opacity})
}

// PushBackdropLayer opens a layer seeded from the parent's contents.
func (r *Recorder) PushBackdropLayer(blend render.BlendMode, opacity float64) {
	r.commands = append(r.commands, PushBackdropLayerCommand{Blend: blend, Opacity: opacity})
}

// PopLayer composites the innermost layer.
func (r *Recorder) PopLayer() {
	r.commands = append(r.commands, PopLayerCommand{})
}

// ApplyBlur attaches a blur post-effect to subsequent draws.
func (r *Recorder) ApplyBlur(radius float64) {
	r.commands = append(r.commands, ApplyBlurCommand{Radius: radius})
}

// ApplyColorMatrix attaches a 4x5 colour matrix post-effect.
func (r *Recorder) ApplyColorMatrix(matrix [20]float32) {
	r.commands = append(r.commands, ApplyColorMatrixCommand{Matrix: matrix})
}

// ApplyDropShadow attaches a drop-shadow post-effect.
func (r *Recorder) ApplyDropShadow(offsetX, offsetY, blurRadius float64, color render.RGBA) {
	r.commands = append(r.commands, ApplyDropShadowCommand{
		OffsetX: offsetX, OffsetY: offsetY, BlurRadius: blurRadius, Color: color,
	})
}

// ApplyGrayscale desaturates subsequent draws.
func (r *Recorder) ApplyGrayscale() {
	r.commands = append(r.commands, ApplyGrayscaleCommand{})
}
