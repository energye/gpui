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

// ErrNoContext is returned by ContextBackend.Begin when the backend was built
// without a target context.
var ErrNoContext = errors.New("recording: ContextBackend has no render.Context")

// ContextBackend replays a Recording onto a live *render.Context.
//
// This is the primary playback path — the counterpart of SkPicture::playback
// (SkCanvas*): the recording is a display list in user space, and the target
// context supplies the coordinate system it lands in. Playback composes onto
// the context's existing CTM and clip instead of replacing them, so a picture
// recorded on the UI thread in layer-local logical pixels rasterizes correctly
// under the layer offset and the device scale that the raster thread already
// set up. Begin pushes the context state and End pops it (plus restores the
// brushes, which Context.Pop does not track), so a recording cannot leak into
// whatever the context paints next.
//
// The raster image backend is a thin wrapper around this type; both share one
// interpretation of the command stream.
type ContextBackend struct {
	dc           *render.Context
	fillBrush    render.Brush
	strokeBrush  render.Brush
	statePushed  bool
	savedBrushes bool
}

// Ensure ContextBackend satisfies the backend contract.
var _ Backend = (*ContextBackend)(nil)

// NewContextBackend returns a backend that draws into dc. The context keeps
// its own size, scale and clip; the recording is positioned by composing onto
// them at playback.
func NewContextBackend(dc *render.Context) *ContextBackend {
	return &ContextBackend{dc: dc}
}

// Begin records the state to restore at End and protects the context's CTM
// and clip. The width/height of the recording is ignored: the target context
// defines the pixel space.
func (b *ContextBackend) Begin(_, _ int) error {
	if b.dc == nil {
		return ErrNoContext
	}
	b.fillBrush, b.strokeBrush = b.dc.FillBrush(), b.dc.StrokeBrush()
	b.savedBrushes = true
	b.dc.Push()
	b.statePushed = true
	return nil
}

// End restores the context state Begin captured.
func (b *ContextBackend) End() error {
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
func (b *ContextBackend) Save() { b.dc.Push() }

// Restore pops the context state.
func (b *ContextBackend) Restore() { b.dc.Pop() }

// Concat multiplies m into the current CTM, composing with the matrix the
// context already had rather than replacing it.
func (b *ContextBackend) Concat(m Matrix) { b.dc.Transform(m) }

// SetClip intersects the clip region with the path, in user space under the
// current CTM.
func (b *ContextBackend) SetClip(path *render.Path, rule FillRule) {
	if path == nil {
		return
	}
	b.dc.SetFillRule(rule)
	b.dc.DrawPath(path)
	b.dc.Clip()
}

// ClearClip resets the clip to the full context.
func (b *ContextBackend) ClearClip() { b.dc.ResetClip() }

// FillPath fills the path with the brush.
func (b *ContextBackend) FillPath(path *render.Path, brush Brush, rule FillRule) {
	if path == nil {
		return
	}
	ApplyBrush(b.dc, brush, true)
	b.dc.SetFillRule(rule)
	b.dc.DrawPath(path)
	_ = b.dc.Fill()
}

// StrokePath strokes the path with the brush and stroke style.
func (b *ContextBackend) StrokePath(path *render.Path, brush Brush, stroke Stroke) {
	if path == nil {
		return
	}
	ApplyBrush(b.dc, brush, false)
	ApplyStroke(b.dc, stroke)
	b.dc.DrawPath(path)
	_ = b.dc.Stroke()
}

// FillRect fills an axis-aligned rectangle kept in user space.
func (b *ContextBackend) FillRect(rect Rect, brush Brush) {
	ApplyBrush(b.dc, brush, true)
	b.dc.DrawRectangle(rect.MinX, rect.MinY, rect.Width(), rect.Height())
	_ = b.dc.Fill()
}

// StrokeRect strokes an axis-aligned rectangle kept in user space.
func (b *ContextBackend) StrokeRect(rect Rect, brush Brush, stroke Stroke) {
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
func (b *ContextBackend) DrawImage(img *render.ImageBuf, opts render.DrawImageOptions) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageEx(img, opts)
}

// DrawImageNine stretches img's edges between center to fill dst.
func (b *ContextBackend) DrawImageNine(img *render.ImageBuf, center image.Rectangle, dst Rect) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageNine(img, center, dst.MinX, dst.MinY, dst.Width(), dst.Height())
}

// DrawImageQuad maps img onto an arbitrary quad.
func (b *ContextBackend) DrawImageQuad(img *render.ImageBuf, corners [4]render.Point, opts render.QuadDrawOptions) {
	if img == nil || img.Disposed() {
		return
	}
	_ = b.dc.DrawImageQuad(img, corners, opts)
}

// DrawImageRounded draws img clipped to a rounded rectangle.
func (b *ContextBackend) DrawImageRounded(img *render.ImageBuf, x, y, radius float64) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageRounded(img, x, y, radius)
}

// DrawImageCircular draws img clipped to a circle.
func (b *ContextBackend) DrawImageCircular(img *render.ImageBuf, cx, cy, radius float64) {
	if img == nil || img.Disposed() {
		return
	}
	b.dc.DrawImageCircular(img, cx, cy, radius)
}

// DrawAtlas replays a sprite batch. Context returns a degradation report that
// only exists once rasterized, so the replay result is dropped here.
func (b *ContextBackend) DrawAtlas(img *render.ImageBuf, sprites []render.AtlasSprite, opts render.AtlasDrawOptions) {
	if img == nil || img.Disposed() || len(sprites) == 0 {
		return
	}
	_, _ = b.dc.DrawAtlasEx(img, sprites, opts)
}

// DrawDepthSprites replays a depth-tagged sprite batch.
func (b *ContextBackend) DrawDepthSprites(img *render.ImageBuf, sprites []render.DepthSprite, opts render.DepthDrawOptions) {
	if img == nil || img.Disposed() || len(sprites) == 0 {
		return
	}
	_, _ = b.dc.DrawDepthSprites(img, sprites, opts)
}

// DrawVertices replays an immediate-mode mesh under the paint it was recorded
// with, so the vertex fallback colour does not drift to whatever the previous
// command left on the context.
func (b *ContextBackend) DrawVertices(positions []render.Point, colors []render.RGBA, mode render.VertexMode, brush Brush) {
	ApplyBrush(b.dc, brush, true)
	b.dc.DrawVertices(positions, colors, mode)
}

// DrawGlyphs replays a shaped-glyph run.
func (b *ContextBackend) DrawGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64, colored bool, brush Brush) {
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
func (b *ContextBackend) ClipRect(rect Rect) {
	b.dc.ClipRect(rect.MinX, rect.MinY, rect.Width(), rect.Height())
}

// SetTextDecoration sets the decoration bitset for text drawn afterwards.
func (b *ContextBackend) SetTextDecoration(deco render.TextDecoration) {
	b.dc.SetTextDecoration(deco)
}

// ApplyBlur attaches a blur post-effect.
func (b *ContextBackend) ApplyBlur(radius float64) { b.dc.ApplyBlur(radius) }

// ApplyColorMatrix attaches a 4x5 colour matrix post-effect.
func (b *ContextBackend) ApplyColorMatrix(matrix [20]float32) { b.dc.ApplyColorMatrix(matrix) }

// ApplyDropShadow attaches a drop-shadow post-effect.
func (b *ContextBackend) ApplyDropShadow(offsetX, offsetY, blurRadius float64, color render.RGBA) {
	b.dc.ApplyDropShadow(offsetX, offsetY, blurRadius, color)
}

// ApplyGrayscale desaturates subsequent draws.
func (b *ContextBackend) ApplyGrayscale() { b.dc.ApplyGrayscale() }

// PushLayer opens a composited layer.
func (b *ContextBackend) PushLayer(blend render.BlendMode, opacity float64) {
	b.dc.PushLayer(blend, opacity)
}

// PushLayerIsolated opens a forced true save-layer.
func (b *ContextBackend) PushLayerIsolated(opacity float64) { b.dc.PushLayerIsolated(opacity) }

// PushBackdropLayer opens a layer seeded from the parent's contents.
func (b *ContextBackend) PushBackdropLayer(blend render.BlendMode, opacity float64) {
	b.dc.PushBackdropLayer(blend, opacity)
}

// PopLayer composites the innermost layer.
func (b *ContextBackend) PopLayer() { b.dc.PopLayer() }

// DrawText draws text at the baseline origin (x, y) in user space.
func (b *ContextBackend) DrawText(s string, x, y float64, face text.Face, brush Brush) {
	if s == "" || face == nil {
		return
	}
	ApplyBrush(b.dc, brush, true)
	b.dc.SetFont(face)
	b.dc.DrawString(s, x, y)
}

// ApplyBrush sets the dc's fill or stroke brush from a recorded brush.
//
// PatternBrush is not translated: render's image pattern needs a live context
// to build (CreateImagePattern) and the recording stores only an image
// reference. A pattern brush therefore paints solid black, exactly as the
// raster backend has always done.
func ApplyBrush(dc *render.Context, brush Brush, fill bool) {
	set := dc.SetStrokeBrush
	if fill {
		set = dc.SetFillBrush
	}
	set(BrushToGG(brush))
}

// ApplyStroke copies a recorded stroke style onto the context.
func ApplyStroke(dc *render.Context, stroke Stroke) {
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
