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
	"errors"
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// ErrNoCanvas is returned by CanvasBackend.Begin when the backend was built
// without a target canvas.
var ErrNoCanvas = errors.New("recording: CanvasBackend has no canvas")

// CanvasBackend replays a Recording onto a render.Canvas.
//
// This is the primary playback path — the counterpart of SkPicture::playback
// (SkCanvas*): the recording is a display list in user space, and the target
// canvas supplies the coordinate system it lands in. Playback composes onto the
// canvas's existing CTM and clip instead of replacing them, so a picture
// recorded on the UI thread in layer-local logical pixels rasterizes correctly
// under the layer offset and the device scale that the raster thread already
// set up. Begin pushes the canvas state and End pops it (plus restores the
// brushes, which Context.Pop does not track), so a recording cannot leak into
// whatever the canvas paints next.
//
// Taking the interface rather than the live context is what makes recordings
// composable: replaying onto a *Recorder folds the commands into the surrounding
// recording, so one display list can be reused inside another and the same call
// works while recording and while rasterizing.
//
// The raster image backend is a thin wrapper around this type; both share one
// interpretation of the command stream.
type CanvasBackend struct {
	dc           render.Canvas
	fillBrush    render.Brush
	strokeBrush  render.Brush
	statePushed  bool
	savedBrushes bool
}

// Ensure CanvasBackend satisfies the backend contract.
var _ Backend = (*CanvasBackend)(nil)

// NewCanvasBackend returns a backend that draws into dc. The canvas keeps its
// own size, scale and clip; the recording is positioned by composing onto them
// at playback.
func NewCanvasBackend(dc render.Canvas) *CanvasBackend {
	return &CanvasBackend{dc: dc}
}

// Begin records the state to restore at End and protects the context's CTM
// and clip. The width/height of the recording is ignored: the target context
// defines the pixel space.
func (b *CanvasBackend) Begin(_, _ int) error {
	if b.dc == nil {
		return ErrNoCanvas
	}
	b.fillBrush, b.strokeBrush = b.dc.FillBrush(), b.dc.StrokeBrush()
	b.savedBrushes = true
	b.dc.Push()
	b.statePushed = true
	return nil
}

// End restores the context state Begin captured.
func (b *CanvasBackend) End() error {
	if b.statePushed {
		b.dc.Pop()
		b.statePushed = false
	}
	if b.savedBrushes {
		b.dc.SetFillBrush(b.fillBrush)
		b.dc.SetStrokeBrush(b.strokeBrush)
		b.savedBrushes = false
	}
	return nil
}

// Save pushes the context state.
func (b *CanvasBackend) Save() { b.dc.Push() }

// Restore pops the context state.
func (b *CanvasBackend) Restore() { b.dc.Pop() }

// Concat multiplies m into the current CTM, composing with the matrix the
// context already had rather than replacing it.
func (b *CanvasBackend) Concat(m Matrix) { b.dc.Transform(m) }

// SetClip intersects the clip region with the path, in user space under the
// current CTM.
func (b *CanvasBackend) SetClip(path *render.Path, rule FillRule) {
	if path == nil {
		return
	}
	b.dc.SetFillRule(rule)
	b.dc.DrawPath(path)
	b.dc.Clip()
}

// ClearClip resets the clip to the full context.
func (b *CanvasBackend) ClearClip() { b.dc.ResetClip() }

// FillPath fills the path with the brush.
func (b *CanvasBackend) FillPath(path *render.Path, brush Brush, rule FillRule) {
	if path == nil {
		return
	}
	ApplyBrush(b.dc, brush, true)
	b.dc.SetFillRule(rule)
	b.dc.DrawPath(path)
	_ = b.dc.Fill()
}

// StrokePath strokes the path with the brush and stroke style.
func (b *CanvasBackend) StrokePath(path *render.Path, brush Brush, stroke Stroke) {
	if path == nil {
		return
	}
	ApplyBrush(b.dc, brush, false)
	ApplyStroke(b.dc, stroke)
	b.dc.DrawPath(path)
	_ = b.dc.Stroke()
}

// FillRect fills an axis-aligned rectangle kept in user space.
func (b *CanvasBackend) FillRect(rect Rect, brush Brush) {
	ApplyBrush(b.dc, brush, true)
	b.dc.DrawRectangle(rect.MinX, rect.MinY, rect.Width(), rect.Height())
	_ = b.dc.Fill()
}

// StrokeRect strokes an axis-aligned rectangle kept in user space.
func (b *CanvasBackend) StrokeRect(rect Rect, brush Brush, stroke Stroke) {
	ApplyBrush(b.dc, brush, false)
	ApplyStroke(b.dc, stroke)
	b.dc.DrawRectangle(rect.MinX, rect.MinY, rect.Width(), rect.Height())
	_ = b.dc.Stroke()
}

// DrawImage replays the recorded options onto the context unchanged. The image
// arrives as the *render.ImageBuf the caller drew with, so no pixel copy and no
// GPU texture identity loss happens per playback. Option defaults (zero opacity
// or interpolation meaning "unspecified") are Context's business: forwarding
// verbatim is what makes replay bit-identical to the live call.
func (b *CanvasBackend) DrawImage(img *render.ImageBuf, opts render.DrawImageOptions) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageEx(img, opts)
}

// DrawImageNine stretches img's edges between center to fill dst.
func (b *CanvasBackend) DrawImageNine(img *render.ImageBuf, center image.Rectangle, dst Rect) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageNine(img, center, dst.MinX, dst.MinY, dst.Width(), dst.Height())
}

// DrawImageQuad maps img onto an arbitrary quad.
func (b *CanvasBackend) DrawImageQuad(img *render.ImageBuf, corners [4]render.Point, opts render.QuadDrawOptions) {
	if img == nil || img.Disposed() {
		return
	}
	_ = b.dc.DrawImageQuad(img, corners, opts)
}

// DrawImageRounded draws img clipped to a rounded rectangle.
func (b *CanvasBackend) DrawImageRounded(img *render.ImageBuf, x, y, radius float64) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageRounded(img, x, y, radius)
}

// DrawImageCircular draws img clipped to a circle.
func (b *CanvasBackend) DrawImageCircular(img *render.ImageBuf, cx, cy, radius float64) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageCircular(img, cx, cy, radius)
}

// DrawAtlas replays a sprite batch. Context returns a degradation report that
// only exists once rasterized, so the replay result is dropped here.
func (b *CanvasBackend) DrawAtlas(img *render.ImageBuf, sprites []render.AtlasSprite, opts render.AtlasDrawOptions) {
	if img == nil || img.Disposed() || len(sprites) == 0 {
		return
	}
	_, _ = b.dc.DrawAtlasEx(img, sprites, opts)
}

// DrawDepthSprites replays a depth-tagged sprite batch.
func (b *CanvasBackend) DrawDepthSprites(img *render.ImageBuf, sprites []render.DepthSprite, opts render.DepthDrawOptions) {
	if img == nil || img.Disposed() || len(sprites) == 0 {
		return
	}
	_, _ = b.dc.DrawDepthSprites(img, sprites, opts)
}

// DrawVertices replays an immediate-mode mesh under the paint it was recorded
// with, so the vertex fallback colour does not drift to whatever the previous
// command left on the context.
func (b *CanvasBackend) DrawVertices(positions []render.Point, colors []render.RGBA, mode render.VertexMode, brush Brush) {
	ApplyBrush(b.dc, brush, true)
	b.dc.DrawVertices(positions, colors, mode)
}

// DrawMesh replays an indexed or triangle-list mesh under the paint it was
// recorded with, for the same reason as DrawVertices.
func (b *CanvasBackend) DrawMesh(mesh render.Mesh, brush Brush) {
	ApplyBrush(b.dc, brush, true)
	b.dc.DrawMesh(mesh)
}

// DrawGlyphs replays a shaped-glyph run.
func (b *CanvasBackend) DrawGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64, colored bool, brush Brush) {
	if len(glyphs) == 0 || face == nil {
		return
	}
	ApplyBrush(b.dc, brush, true)
	if colored {
		b.dc.DrawShapedColorGlyphs(glyphs, face, x, y)
		return
	}
	b.dc.DrawShapedGlyphs(glyphs, face, x, y)
}

// ClipRect intersects the clip region with a user-space rectangle.
func (b *CanvasBackend) ClipRect(rect Rect) {
	b.dc.ClipRect(rect.MinX, rect.MinY, rect.Width(), rect.Height())
}

// ClipRectOp combines the clip region with a user-space rectangle using op. The
// live method takes the same four floats, so replay is the same call.
func (b *CanvasBackend) ClipRectOp(rect Rect, op render.ClipOp) {
	b.dc.ClipRectOp(rect.MinX, rect.MinY, rect.Width(), rect.Height(), op)
}

// ClipPathOp applies the recorded path to the clip with op. The rule travels as
// an argument because Context.ClipPathOp reads it off the paint: replay sets the
// rule the recorder had, draws the path, then clips — the exact call sequence
// the live path went through.
func (b *CanvasBackend) ClipPathOp(path *render.Path, rule FillRule, op render.ClipOp) {
	if path == nil {
		return
	}
	b.dc.SetFillRule(rule)
	b.dc.DrawPath(path)
	b.dc.ClipPathOp(op)
}

// SetTextDecoration sets the decoration bitset for text drawn afterwards.
func (b *CanvasBackend) SetTextDecoration(deco render.TextDecoration) {
	b.dc.SetTextDecoration(deco)
}

// SetTextMode selects the text rendering strategy for subsequent text.
func (b *CanvasBackend) SetTextMode(mode render.TextMode) { b.dc.SetTextMode(mode) }

// SetLCDLayout sets the LCD subpixel order for subsequent ClearType text.
func (b *CanvasBackend) SetLCDLayout(layout render.LCDLayout) { b.dc.SetLCDLayout(layout) }

// SetAntiAlias toggles anti-aliasing for subsequent geometry and clips.
func (b *CanvasBackend) SetAntiAlias(enabled bool) { b.dc.SetAntiAlias(enabled) }

// ApplyBlur attaches a blur post-effect.
func (b *CanvasBackend) ApplyBlur(radius float64) { b.dc.ApplyBlur(radius) }

// ApplyColorMatrix attaches a 4x5 colour matrix post-effect.
func (b *CanvasBackend) ApplyColorMatrix(matrix [20]float32) { b.dc.ApplyColorMatrix(matrix) }

// ApplyDropShadow attaches a drop-shadow post-effect.
func (b *CanvasBackend) ApplyDropShadow(offsetX, offsetY, blurRadius float64, color render.RGBA) {
	b.dc.ApplyDropShadow(offsetX, offsetY, blurRadius, color)
}

// ApplyGrayscale desaturates subsequent draws.
func (b *CanvasBackend) ApplyGrayscale() { b.dc.ApplyGrayscale() }

// PushLayer opens a composited layer.
func (b *CanvasBackend) PushLayer(blend render.BlendMode, opacity float64) {
	b.dc.PushLayer(blend, opacity)
}

// PushLayerIsolated opens a forced true save-layer.
func (b *CanvasBackend) PushLayerIsolated(opacity float64) { b.dc.PushLayerIsolated(opacity) }

// PushBackdropLayer opens a layer seeded from the parent's contents.
func (b *CanvasBackend) PushBackdropLayer(blend render.BlendMode, opacity float64) {
	b.dc.PushBackdropLayer(blend, opacity)
}

// PopLayer composites the innermost layer.
func (b *CanvasBackend) PopLayer() { b.dc.PopLayer() }

// DrawText draws text at the baseline origin (x, y) in user space.
func (b *CanvasBackend) DrawText(s string, x, y float64, face text.Face, brush Brush) {
	if s == "" || face == nil {
		return
	}
	ApplyBrush(b.dc, brush, true)
	b.dc.SetFont(face)
	b.dc.DrawString(s, x, y)
}

// ApplyBrush sets the dc's fill or stroke paint from a recorded brush.
//
// A recorded pattern brush is replayed as a pattern, not as a brush: an image
// pattern is a render.Pattern (render.Brush is sealed and has no image form), and
// Context.SetFillPattern is what the live call went through, so the rebuilt
// pattern lands in the same paint fields and samples the same texels. When the
// pattern has nothing left to sample — its image was disposed between recording
// and replay — the paint becomes fully transparent instead of guessing a colour:
// painting nothing is the honest degradation, painting black is not.
func ApplyBrush(dc render.Canvas, brush Brush, fill bool) {
	if pat, ok := ToRenderPattern(brush); ok {
		if pat == nil {
			pat = render.NewSolidPattern(render.Transparent)
		}
		if fill {
			dc.SetFillPattern(pat)
		} else {
			dc.SetStrokePattern(pat)
		}
		return
	}
	set := dc.SetStrokeBrush
	if fill {
		set = dc.SetFillBrush
	}
	set(ToRenderBrush(brush))
}

// ApplyStroke copies a recorded stroke style onto the context.
func ApplyStroke(dc render.Canvas, stroke Stroke) {
	dc.SetLineWidth(stroke.Width)
	dc.SetLineCap(stroke.Cap)
	dc.SetLineJoin(stroke.Join)
	dc.SetMiterLimit(stroke.MiterLimit)

	if len(stroke.DashPattern) > 0 {
		dc.SetDash(stroke.DashPattern...)
		dc.SetDashOffset(stroke.DashOffset)
	} else {
		dc.ClearDash()
	}
}
