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

	"github.com/energye/gpui/render/text"
)

// Canvas is the drawing surface interface — the SkCanvas analogue.
//
// Two implementations satisfy it: a live *Context, which rasterizes as calls
// arrive, and recording.Recorder, which turns the same calls into a display
// list. Paint code typed against Canvas therefore runs unchanged on either
// target, which is what lets the UI thread build a picture and the raster
// thread only execute it (Flutter's render/paint split).
//
// The surface is deliberately the *describable* part of Context. Methods whose
// arguments only exist on the thread that owns the GPU, or whose result is a
// raster-time fact, stay on *Context and are not part of this contract:
// GPU texture handles (DrawGPUTexture*), flushing and presentation
// (FlushGPU/PresentFrame*), offscreen passes, masks, encode/export, SetPixel,
// WritePixels and damage tracking. Text outlines (TextPath) are live-only too:
// they need the context's own glyph-outline extractor and mask cache, so a
// display list cannot produce them.
//
// All geometry is user space: the current transform applies to path verbs,
// shapes, image destinations and text origins, and the device scale folds in
// only at the rasterization boundary (Context.totalMatrix).
type Canvas interface {
	// Surface extent and clearing
	Width() int
	Height() int
	Clear()
	ClearWithColor(col RGBA)

	// State
	Push()
	Pop()

	// Transform
	Identity()
	Translate(x, y float64)
	Scale(x, y float64)
	Rotate(angle float64)
	RotateAbout(angle, x, y float64)
	Shear(x, y float64)
	Transform(m Matrix)
	SetTransform(m Matrix)
	GetTransform() Matrix

	// Path building
	MoveTo(x, y float64)
	LineTo(x, y float64)
	QuadraticTo(cx, cy, x, y float64)
	CubicTo(c1x, c1y, c2x, c2y, x, y float64)
	ClosePath()
	ClearPath()
	NewSubPath()
	SetPath(p *Path)
	AppendPath(p *Path)
	DrawPath(p *Path)
	GetCurrentPoint() (x, y float64, ok bool)

	// Paint state
	SetRGB(r, g, b float64)
	SetRGBA(r, g, b, a float64)
	SetFillBrush(b Brush)
	SetStrokeBrush(b Brush)
	// FillBrush / StrokeBrush report the current paint brushes. A display-list
	// player needs them: it saves the ambient brushes on entry and puts them
	// back on exit, so replaying onto a canvas cannot leak paint state into
	// whatever the caller draws next.
	FillBrush() Brush
	StrokeBrush() Brush
	SetFillPattern(pattern Pattern)
	SetStrokePattern(pattern Pattern)
	SetFillRule(rule FillRule)
	SetLineWidth(width float64)
	SetLineCap(lineCap LineCap)
	SetLineJoin(join LineJoin)
	SetMiterLimit(limit float64)
	SetStroke(stroke Stroke)
	SetDash(lengths ...float64)
	SetDashOffset(offset float64)
	ClearDash()
	SetAntiAlias(enabled bool)
	AntiAlias() bool

	// Draw
	Fill() error
	Stroke() error
	FillPreserve() error
	StrokePreserve() error
	FillPath(p *Path) error
	StrokePath(p *Path) error

	// Clip
	Clip()
	ClipPreserve()
	ClipRect(x, y, w, h float64)
	ClipRectOp(x, y, w, h float64, op ClipOp)
	ClipPathOp(op ClipOp)
	ClipRoundRect(x, y, w, h, radius float64)
	ResetClip()

	// Shapes
	DrawPoint(x, y, r float64)
	DrawLine(x1, y1, x2, y2 float64)
	DrawRectangle(x, y, w, h float64)
	DrawRoundedRectangle(x, y, w, h, r float64)
	DrawCircle(x, y, r float64)
	DrawEllipse(x, y, rx, ry float64)
	DrawArc(x, y, r, angle1, angle2 float64)
	DrawEllipticalArc(x, y, rx, ry, angle1, angle2 float64)

	// Images and meshes
	DrawImage(img *ImageBuf, x, y float64)
	DrawImageEx(img *ImageBuf, opts DrawImageOptions)
	DrawImageNine(img *ImageBuf, center image.Rectangle, dstX, dstY, dstW, dstH float64)
	DrawImageQuad(img *ImageBuf, corners [4]Point, opts QuadDrawOptions) error
	DrawImageRounded(img *ImageBuf, x, y, radius float64)
	DrawImageCircular(img *ImageBuf, cx, cy, radius float64)
	DrawAtlas(img *ImageBuf, sprites []AtlasSprite)
	DrawAtlasEx(img *ImageBuf, sprites []AtlasSprite, opts AtlasDrawOptions) (AtlasDrawResult, error)
	DrawDepthSprites(img *ImageBuf, sprites []DepthSprite, opts DepthDrawOptions) (DepthDrawResult, error)
	DrawVertices(positions []Point, colors []RGBA, mode VertexMode)
	DrawVerticesEx(positions []Point, colors []RGBA, mode VertexMode, opts VertDrawOptions) (VertDrawResult, error)
	DrawMesh(mesh Mesh)
	DrawMeshEx(mesh Mesh, opts VertDrawOptions) (VertDrawResult, error)

	// Text
	SetFont(face text.Face)
	Font() text.Face
	SetTextDecoration(d TextDecoration)
	SetTextMode(mode TextMode)
	SetLCDLayout(layout LCDLayout)
	DrawString(s string, x, y float64)
	DrawStringAnchored(s string, x, y, ax, ay float64)
	DrawStringWrapped(s string, x, y, ax, ay, width, lineSpacing float64, align Align)
	StrokeString(s string, x, y float64)
	StrokeStringAnchored(s string, x, y, ax, ay float64)
	DrawShapedGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64)
	DrawShapedColorGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64)
	MeasureString(s string) (w, h float64)
	MeasureMultilineString(s string, lineSpacing float64) (width, height float64)
	WordWrap(s string, w float64) []string

	// Compositing layers
	PushLayer(blendMode BlendMode, opacity float64)
	PushLayerIsolated(opacity float64)
	PushBackdropLayer(blendMode BlendMode, opacity float64)
	PopLayer()

	// Post-effects
	ApplyBlur(radius float64)
	ApplyColorMatrix(matrix [20]float32)
	ApplyDropShadow(offsetX, offsetY, blurRadius float64, color RGBA)
	ApplyGrayscale()
}

// The contract is compile-checked on both sides: *Context here, Recorder in
// render/recording (which imports render, so the check cannot live here).
var _ Canvas = (*Context)(nil)
