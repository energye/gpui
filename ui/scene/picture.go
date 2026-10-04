//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package scene holds retained scene types for L1 (Layer tree, FramePacket, Picture).
package scene

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// PictureOpKind identifies a recorded draw command in a Picture display list.
type PictureOpKind int

const (
	// OpFillRect fills an axis-aligned rectangle.
	OpFillRect PictureOpKind = iota + 1
	// OpStrokeRect strokes an axis-aligned rectangle.
	OpStrokeRect
	// OpFillPath fills a retained vector path (cloned at record time).
	OpFillPath
	// OpStrokePath strokes a retained vector path.
	OpStrokePath
	// OpDrawString draws a text run at baseline (X,Y); Face optional (else DC.Font).
	OpDrawString
	// OpDrawShapedGlyphs replays pre-shaped glyphs at pen (X,Y) baseline.
	// Same representation Paint submits (bulk or rebased partition); replay
	// calls DrawShapedGlyphs (IsColor=false) or DrawShapedColorGlyphs.
	OpDrawShapedGlyphs
	// OpDrawImage draws an ImageBuf at (X,Y); DstW/DstH >0 scales, else 1:1.
	OpDrawImage
	// OpFillCircle fills a circle centered at (X,Y) with Radius.
	OpFillCircle
	// OpStrokeCircle strokes a circle centered at (X,Y) with Radius.
	OpStrokeCircle
	// OpFillOval fills an ellipse in the box (X,Y,W,H).
	OpFillOval
	// OpStrokeOval strokes an ellipse in the box (X,Y,W,H).
	OpStrokeOval
	// OpFillArc fills a circular sector centered at (X,Y) from Angle1 to Angle2.
	OpFillArc
	// OpStrokeArc strokes a circular arc centered at (X,Y) from Angle1 to Angle2.
	OpStrokeArc
	// OpFillRoundRect fills a rounded rect (X,Y,W,H) with uniform Radius.
	OpFillRoundRect
	// OpStrokeRoundRect strokes a rounded rect (X,Y,W,H) with uniform Radius.
	OpStrokeRoundRect
	// OpStrokeLine strokes a segment from (X,Y) to (X2,Y2).
	OpStrokeLine
	// OpFillLinearGradient fills rect (X,Y,W,H) with a two-stop linear
	// gradient along (GX0,GY0)→(GX1,GY1), stops (R,G,B,A) and (R1,G1,B1,A1).
	OpFillLinearGradient
)

// PictureOp is one retained draw command (SkPicture draw-op subset).
// Coordinates are absolute on the target Context at replay time (Y-down).
// Color components (R,G,B,A) are premultiplied-style 0..1 paint values; the
// recorder normalizes them at record time so replay is a pure forward pass.
type PictureOp struct {
	Kind       PictureOpKind
	X, Y, W, H float64
	R, G, B, A float64
	// LineWidth is used by OpStrokeRect / OpStrokePath (≤0 → 1 at replay).
	LineWidth float64
	// Path is a deep-cloned path for OpFillPath / OpStrokePath (nil otherwise).
	Path *render.Path
	// Text is the UTF-8 run for OpDrawString.
	Text string
	// Face is an optional font for OpDrawString; if nil, replay uses dc.Font().
	// OpDrawShapedGlyphs requires Face (nil replays nothing).
	Face text.Face
	// Glyphs carries the shaped run for OpDrawShapedGlyphs (detached copy).
	Glyphs []text.ShapedGlyph
	// IsColor selects DrawShapedColorGlyphs at replay (color emoji runs).
	IsColor bool
	// Image is a retained buffer for OpDrawImage (not cloned; caller owns lifetime).
	Image *render.ImageBuf
	// DstW, DstH scale destination for OpDrawImage when both >0; else 1:1 DrawImage.
	DstW, DstH float64
	// Radius is the circle/arc radius (OpFillCircle/OpStrokeCircle/OpFillArc/
	// OpStrokeArc) or the uniform corner radius (OpFillRoundRect/OpStrokeRoundRect).
	Radius float64
	// Angle1, Angle2 are the arc endpoints in radians (OpFillArc/OpStrokeArc).
	Angle1, Angle2 float64
	// X2, Y2 is the segment end point (OpStrokeLine).
	X2, Y2 float64
	// GX0, GY0, GX1, GY1 are the gradient endpoints (OpFillLinearGradient).
	GX0, GY0, GX1, GY1 float64
	// R1, G1, B1, A1 is the second gradient stop (OpFillLinearGradient).
	R1, G1, B1, A1 float64
}

// Picture is a retained, immutable display list (SkPicture analogue).
// After EndRecording the ops slice must be treated as read-only: the recorder
// hands over a detached copy, so no aliasing with caller-owned slices remains.
//
// Valid is NOT part of the display list; it is a cache flag owned by the layer
// / boundary that hosts the picture (true = ops may be replayed without a
// re-record). Zero value means "no retained picture" (empty ops, invalid).
type Picture struct {
	// ID is stable identity for cache keys (optional).
	ID uint64
	// Valid is the host-side re-record flag (not display-list state).
	Valid bool
	// Ops is the retained display list (nil/empty = no recorded content).
	// Read-only after recording.
	Ops []PictureOp
	// Bounds is the union of op geometry in logical coordinates (retained
	// compositing damage rects). Text-only pictures keep this empty (unknown
	// extent) — callers fall back to a conservative surface rect.
	Bounds image.Rectangle
}

// OpCount returns the number of recorded draw ops.
func (p *Picture) OpCount() int {
	if p == nil {
		return 0
	}
	return len(p.Ops)
}

// IsEmpty reports whether there are no recorded ops.
func (p *Picture) IsEmpty() bool {
	return p == nil || len(p.Ops) == 0
}

// hasVisibleOps reports whether replaying p can emit any drawing work. It
// mirrors applyPictureOp's skip conditions: zero-alpha, empty, or
// missing-resource ops queue nothing into the record stream.
func (p *Picture) hasVisibleOps() bool {
	if p == nil {
		return false
	}
	for i := range p.Ops {
		op := &p.Ops[i]
		switch op.Kind {
		case OpFillRect, OpStrokeRect:
			if op.W > 0 && op.H > 0 && op.A != 0 {
				return true
			}
		case OpFillPath, OpStrokePath:
			if op.A != 0 && op.Path != nil && op.Path.NumVerbs() > 0 {
				return true
			}
		case OpDrawString:
			if op.Text != "" && op.A != 0 {
				return true
			}
		case OpDrawShapedGlyphs:
			if len(op.Glyphs) > 0 && op.Face != nil && op.A != 0 {
				return true
			}
		case OpDrawImage:
			if op.Image != nil && !op.Image.Disposed() {
				return true
			}
		case OpFillCircle, OpStrokeCircle:
			if op.Radius > 0 && op.A != 0 {
				return true
			}
		case OpFillOval, OpStrokeOval:
			if op.W > 0 && op.H > 0 && op.A != 0 {
				return true
			}
		case OpFillArc, OpStrokeArc:
			if op.Radius > 0 && op.A != 0 {
				return true
			}
		case OpFillRoundRect, OpStrokeRoundRect:
			if op.W > 0 && op.H > 0 && op.A != 0 {
				return true
			}
		case OpStrokeLine:
			if op.A != 0 {
				return true
			}
		case OpFillLinearGradient:
			if op.W > 0 && op.H > 0 && (op.A != 0 || op.A1 != 0) {
				return true
			}
		}
	}
	return false
}

// Invalidate marks the picture for re-record (does not clear Ops until re-record).
func (p *Picture) Invalidate() {
	if p == nil {
		return
	}
	p.Valid = false
}

// Clear drops all ops and marks invalid.
func (p *Picture) Clear() {
	if p == nil {
		return
	}
	p.Ops = nil
	p.Valid = false
}

// Replay applies the display list onto dc (CPU/GPU Context). This is the
// SkPicture::playback analogue: a pure forward pass — ops carry normalized
// paint state recorded earlier, replay never re-derives color semantics.
// No-op if p is nil/empty or dc is nil. Does not change Valid/NeedsRaster flags.
func (p *Picture) Replay(dc *render.Context) {
	if p == nil || dc == nil || len(p.Ops) == 0 {
		return
	}
	for i := range p.Ops {
		applyPictureOp(dc, &p.Ops[i])
	}
}

// applyPictureOp forwards one recorded op onto the Context. Alpha semantics are
// strict (SkPaint): A==0 paints nothing; callers pass explicit alpha at record
// time — there is no implicit opaque fallback.
func opStrokeWidth(lw float64) float64 {
	if lw <= 0 {
		return 1
	}
	return lw
}

func setOpFill(dc *render.Context, op *PictureOp) {
	dc.SetRGBA(op.R, op.G, op.B, op.A)
}

func setOpStroke(dc *render.Context, op *PictureOp) {
	dc.SetRGBA(op.R, op.G, op.B, op.A)
	dc.SetLineWidth(opStrokeWidth(op.LineWidth))
}

func drawRoundRectShape(dc *render.Context, op *PictureOp) {
	rad := op.Radius
	if rad < 0 {
		rad = 0
	}
	if rad <= 0 {
		dc.DrawRectangle(op.X, op.Y, op.W, op.H)
	} else {
		dc.DrawRoundedRectangle(op.X, op.Y, op.W, op.H, rad)
	}
}

func applyPictureOp(dc *render.Context, op *PictureOp) {
	if dc == nil || op == nil {
		return
	}
	switch op.Kind {
	case OpFillRect:
		if op.W <= 0 || op.H <= 0 || op.A == 0 {
			return
		}
		setOpFill(dc, op)
		dc.DrawRectangle(op.X, op.Y, op.W, op.H)
		_ = dc.Fill()
	case OpStrokeRect:
		if op.W <= 0 || op.H <= 0 || op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		dc.DrawRectangle(op.X, op.Y, op.W, op.H)
		_ = dc.Stroke()
	case OpFillPath:
		if op.Path == nil || op.Path.NumVerbs() == 0 || op.A == 0 {
			return
		}
		setOpFill(dc, op)
		_ = dc.FillPath(op.Path)
	case OpStrokePath:
		if op.Path == nil || op.Path.NumVerbs() == 0 || op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		_ = dc.StrokePath(op.Path)
	case OpDrawString:
		if op.Text == "" || op.A == 0 {
			return
		}
		if op.Face != nil {
			dc.SetFont(op.Face)
		}
		if dc.Font() == nil {
			return
		}
		dc.SetRGBA(op.R, op.G, op.B, op.A)
		dc.DrawString(op.Text, op.X, op.Y)
	case OpDrawShapedGlyphs:
		if len(op.Glyphs) == 0 || op.Face == nil || op.A == 0 {
			return
		}
		dc.SetRGBA(op.R, op.G, op.B, op.A)
		if op.IsColor {
			dc.DrawShapedColorGlyphs(op.Glyphs, op.Face, op.X, op.Y)
			return
		}
		dc.DrawShapedGlyphs(op.Glyphs, op.Face, op.X, op.Y)
	case OpDrawImage:
		if op.Image == nil || op.Image.Disposed() {
			return
		}
		if op.DstW > 0 && op.DstH > 0 {
			dc.DrawImageEx(op.Image, render.DrawImageOptions{
				X:         op.X,
				Y:         op.Y,
				DstWidth:  op.DstW,
				DstHeight: op.DstH,
			})
		} else {
			dc.DrawImage(op.Image, op.X, op.Y)
		}
	case OpFillCircle:
		if op.Radius <= 0 || op.A == 0 {
			return
		}
		setOpFill(dc, op)
		dc.DrawCircle(op.X, op.Y, op.Radius)
		_ = dc.Fill()
	case OpStrokeCircle:
		if op.Radius <= 0 || op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		dc.DrawCircle(op.X, op.Y, op.Radius)
		_ = dc.Stroke()
	case OpFillOval:
		if op.W <= 0 || op.H <= 0 || op.A == 0 {
			return
		}
		setOpFill(dc, op)
		dc.DrawEllipse(op.X+op.W/2, op.Y+op.H/2, op.W/2, op.H/2)
		_ = dc.Fill()
	case OpStrokeOval:
		if op.W <= 0 || op.H <= 0 || op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		dc.DrawEllipse(op.X+op.W/2, op.Y+op.H/2, op.W/2, op.H/2)
		_ = dc.Stroke()
	case OpFillArc:
		if op.Radius <= 0 || op.A == 0 {
			return
		}
		setOpFill(dc, op)
		dc.DrawArc(op.X, op.Y, op.Radius, op.Angle1, op.Angle2)
		_ = dc.Fill()
	case OpStrokeArc:
		if op.Radius <= 0 || op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		dc.DrawArc(op.X, op.Y, op.Radius, op.Angle1, op.Angle2)
		_ = dc.Stroke()
	case OpFillRoundRect:
		if op.W <= 0 || op.H <= 0 || op.A == 0 {
			return
		}
		setOpFill(dc, op)
		drawRoundRectShape(dc, op)
		_ = dc.Fill()
	case OpStrokeRoundRect:
		if op.W <= 0 || op.H <= 0 || op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		drawRoundRectShape(dc, op)
		_ = dc.Stroke()
	case OpStrokeLine:
		if op.A == 0 {
			return
		}
		setOpStroke(dc, op)
		dc.DrawLine(op.X, op.Y, op.X2, op.Y2)
		_ = dc.Stroke()
	case OpFillLinearGradient:
		if op.W <= 0 || op.H <= 0 {
			return
		}
		br := render.NewLinearGradientBrush(op.GX0, op.GY0, op.GX1, op.GY1).
			AddColorStop(0, render.RGBA{R: op.R, G: op.G, B: op.B, A: op.A}).
			AddColorStop(1, render.RGBA{R: op.R1, G: op.G1, B: op.B1, A: op.A1})
		dc.SetFillBrush(br)
		dc.DrawRectangle(op.X, op.Y, op.W, op.H)
		_ = dc.Fill()
	}
}

// PictureRecorder builds a Picture display list (SkPictureRecorder subset).
// Lifecycle: draw ops onto the recorder, then EndRecording / Finish freezes a
// detached display list and resets the recorder (it may be reused for the next
// recording session — Flutter beginRecording semantics).
type PictureRecorder struct {
	ops    []PictureOp
	minX   float64
	minY   float64
	maxX   float64
	maxY   float64
	hasBnd bool
}

// NewPictureRecorder starts an empty recording session.
func NewPictureRecorder() *PictureRecorder {
	return &PictureRecorder{}
}

// clamp01 normalizes a color channel to [0,1] at record time so replay ops
// carry well-formed paint state.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// noteGeometry unions an op rect (logical coords) into the recorded bounds
// using float64 accumulation (no int truncation until EndRecording).
// Text ops have unknown extent and are skipped (bounds stay conservative-unknown).
func (r *PictureRecorder) noteGeometry(x, y, w, h float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	if !r.hasBnd {
		r.minX, r.minY, r.maxX, r.maxY = x, y, x+w, y+h
		r.hasBnd = true
		return
	}
	if x < r.minX {
		r.minX = x
	}
	if y < r.minY {
		r.minY = y
	}
	if x+w > r.maxX {
		r.maxX = x + w
	}
	if y+h > r.maxY {
		r.maxY = y + h
	}
}

// FillRect records a filled rectangle (R,G,B,A in 0..1, alpha strict).
func (r *PictureRecorder) FillRect(x, y, w, h, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	r.noteGeometry(x, y, w, h)
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillRect,
		X:    x, Y: y, W: w, H: h,
		R: clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokeRect records a stroked rectangle.
func (r *PictureRecorder) StrokeRect(x, y, w, h, lineWidth, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	r.noteGeometry(x, y, w, h)
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeRect,
		X:    x, Y: y, W: w, H: h,
		R: clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
		LineWidth: lineWidth,
	})
}

// FillPath records a filled path. The path is deep-cloned so later mutation of
// the caller's path does not affect the retained display list. The path bounds
// are folded into the recorded Bounds for damage rects.
func (r *PictureRecorder) FillPath(p *render.Path, red, gre, blu, a float64) {
	if r == nil || p == nil || p.NumVerbs() == 0 {
		return
	}
	clone := p.Clone()
	if !clone.Bounds().Empty() {
		b := clone.Bounds()
		r.noteGeometry(float64(b.Min.X), float64(b.Min.Y), float64(b.Dx()), float64(b.Dy()))
	}
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillPath,
		Path: clone,
		R:    clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokePath records a stroked path (path is deep-cloned; bounds inflated by
// half the line width to cover the stroke band).
func (r *PictureRecorder) StrokePath(p *render.Path, lineWidth, red, gre, blu, a float64) {
	if r == nil || p == nil || p.NumVerbs() == 0 {
		return
	}
	clone := p.Clone()
	if !clone.Bounds().Empty() {
		b := clone.Bounds()
		inflate := lineWidth / 2
		if inflate < 0 {
			inflate = 0
		}
		r.noteGeometry(float64(b.Min.X)-inflate, float64(b.Min.Y)-inflate,
			float64(b.Dx())+2*inflate, float64(b.Dy())+2*inflate)
	}
	r.ops = append(r.ops, PictureOp{
		Kind:      OpStrokePath,
		Path:      clone,
		LineWidth: lineWidth,
		R:         clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// DrawString records a text run at baseline (x,y). face may be nil: replay then
// requires the target Context to already have a font via SetFont. Text extent
// is unknown at record time, so bounds stay conservative-unknown.
func (r *PictureRecorder) DrawString(s string, x, y float64, face text.Face, red, gre, blu, a float64) {
	if r == nil || s == "" {
		return
	}
	r.ops = append(r.ops, PictureOp{
		Kind: OpDrawString,
		X:    x, Y: y,
		Text: s,
		Face: face,
		R:    clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// DrawShapedGlyphs records pre-shaped glyphs at pen (x,y) baseline.
// The slice is deep-copied; later layout mutation cannot affect the display
// list. isColor selects the color-glyph replay path. face is required.
func (r *PictureRecorder) DrawShapedGlyphs(glyphs []text.ShapedGlyph, x, y float64, face text.Face, isColor bool, red, gre, blu, a float64) {
	if r == nil || len(glyphs) == 0 || face == nil {
		return
	}
	cp := append([]text.ShapedGlyph(nil), glyphs...)
	r.ops = append(r.ops, PictureOp{
		Kind:    OpDrawShapedGlyphs,
		X:       x,
		Y:       y,
		Glyphs:  cp,
		Face:    face,
		IsColor: isColor,
		R:       clamp01(red),
		G:       clamp01(gre),
		B:       clamp01(blu),
		A:       clamp01(a),
	})
}

// DrawImage records an image draw at (x,y). If dstW and dstH are both >0 the
// image is scaled into that box; otherwise it is drawn 1:1. The ImageBuf is
// retained by reference (not pixel-copied); dispose only after pictures that
// reference it are dropped. The destination rect is folded into Bounds.
func (r *PictureRecorder) DrawImage(img *render.ImageBuf, x, y, dstW, dstH float64) {
	if r == nil || img == nil || img.Disposed() {
		return
	}
	if dstW > 0 && dstH > 0 {
		r.noteGeometry(x, y, dstW, dstH)
	} else {
		w, h := img.Bounds()
		r.noteGeometry(x, y, float64(w), float64(h))
	}
	r.ops = append(r.ops, PictureOp{
		Kind: OpDrawImage,
		X:    x, Y: y,
		Image: img,
		DstW:  dstW, DstH: dstH,
	})
}

// FillCircle records a filled circle centered at (cx,cy). radius<=0 draws
// nothing (mirrors the UI facade guard).
func (r *PictureRecorder) FillCircle(cx, cy, radius, red, gre, blu, a float64) {
	if r == nil || radius <= 0 {
		return
	}
	r.noteGeometry(cx-radius, cy-radius, radius*2, radius*2)
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillCircle,
		X:    cx, Y: cy,
		Radius: radius,
		R:      clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokeCircle records a stroked circle. Bounds inflate by half the line
// width to cover the stroke band (effective width ≤0 counts as 1).
func (r *PictureRecorder) StrokeCircle(cx, cy, radius, lineWidth, red, gre, blu, a float64) {
	if r == nil || radius <= 0 {
		return
	}
	inflate := lineWidth / 2
	if inflate <= 0 {
		inflate = 0.5
	}
	r.noteGeometry(cx-radius-inflate, cy-radius-inflate,
		radius*2+2*inflate, radius*2+2*inflate)
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeCircle,
		X:    cx, Y: cy,
		Radius:    radius,
		LineWidth: lineWidth,
		R:         clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// FillOval records a filled ellipse in the box (x,y,w,h).
func (r *PictureRecorder) FillOval(x, y, w, h, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	r.noteGeometry(x, y, w, h)
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillOval,
		X:    x, Y: y, W: w, H: h,
		R: clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokeOval records a stroked ellipse in the box (x,y,w,h).
func (r *PictureRecorder) StrokeOval(x, y, w, h, lineWidth, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	inflate := lineWidth / 2
	if inflate <= 0 {
		inflate = 0.5
	}
	r.noteGeometry(x-inflate, y-inflate, w+2*inflate, h+2*inflate)
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeOval,
		X:    x, Y: y, W: w, H: h,
		LineWidth: lineWidth,
		R:         clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// FillArc records a filled circular sector centered at (cx,cy).
// Bounds stay conservative (full disc): sector-exact bounds would need
// angle-span analysis at record time.
func (r *PictureRecorder) FillArc(cx, cy, radius, angle1, angle2, red, gre, blu, a float64) {
	if r == nil || radius <= 0 {
		return
	}
	r.noteGeometry(cx-radius, cy-radius, radius*2, radius*2)
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillArc,
		X:    cx, Y: cy,
		Radius: radius,
		Angle1: angle1,
		Angle2: angle2,
		R:      clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokeArc records a stroked circular arc centered at (cx,cy).
func (r *PictureRecorder) StrokeArc(cx, cy, radius, angle1, angle2, lineWidth, red, gre, blu, a float64) {
	if r == nil || radius <= 0 {
		return
	}
	inflate := lineWidth / 2
	if inflate <= 0 {
		inflate = 0.5
	}
	r.noteGeometry(cx-radius-inflate, cy-radius-inflate,
		radius*2+2*inflate, radius*2+2*inflate)
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeArc,
		X:    cx, Y: cy,
		Radius:    radius,
		Angle1:    angle1,
		Angle2:    angle2,
		LineWidth: lineWidth,
		R:         clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// FillRoundRect records a filled rounded rect with uniform corner radius.
// Negative radius normalizes to 0 (sharp rect, mirrors the UI facade).
func (r *PictureRecorder) FillRoundRect(x, y, w, h, radius, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	r.noteGeometry(x, y, w, h)
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillRoundRect,
		X:    x, Y: y, W: w, H: h,
		Radius: radius,
		R:      clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokeRoundRect records a stroked rounded rect with uniform corner radius.
func (r *PictureRecorder) StrokeRoundRect(x, y, w, h, radius, lineWidth, red, gre, blu, a float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	inflate := lineWidth / 2
	if inflate <= 0 {
		inflate = 0.5
	}
	r.noteGeometry(x-inflate, y-inflate, w+2*inflate, h+2*inflate)
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeRoundRect,
		X:    x, Y: y, W: w, H: h,
		Radius:    radius,
		LineWidth: lineWidth,
		R:         clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// StrokeLine records a stroked segment from (x1,y1) to (x2,y2).
func (r *PictureRecorder) StrokeLine(x1, y1, x2, y2, lineWidth, red, gre, blu, a float64) {
	if r == nil {
		return
	}
	inflate := lineWidth / 2
	if inflate <= 0 {
		inflate = 0.5
	}
	minX, maxX := x1, x2
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := y1, y2
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	r.noteGeometry(minX-inflate, minY-inflate,
		maxX-minX+2*inflate, maxY-minY+2*inflate)
	r.ops = append(r.ops, PictureOp{
		Kind: OpStrokeLine,
		X:    x1, Y: y1,
		X2: x2, Y2: y2,
		LineWidth: lineWidth,
		R:         clamp01(red), G: clamp01(gre), B: clamp01(blu), A: clamp01(a),
	})
}

// FillLinearGradient records a two-stop linear gradient fill over rect
// (x,y,w,h) along (x0,y0)→(x1,y1). Coordinates are absolute at record time,
// matching the UI facade after origin resolution.
func (r *PictureRecorder) FillLinearGradient(x, y, w, h, x0, y0, x1, y1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	if r == nil || w <= 0 || h <= 0 {
		return
	}
	r.noteGeometry(x, y, w, h)
	r.ops = append(r.ops, PictureOp{
		Kind: OpFillLinearGradient,
		X:    x, Y: y, W: w, H: h,
		GX0: x0, GY0: y0, GX1: x1, GY1: y1,
		R: clamp01(r0), G: clamp01(g0), B: clamp01(b0), A: clamp01(a0),
		R1: clamp01(r1), G1: clamp01(g1), B1: clamp01(b1), A1: clamp01(a1),
	})
}

// OpCount returns ops recorded so far (before EndRecording).
func (r *PictureRecorder) OpCount() int {
	if r == nil {
		return 0
	}
	return len(r.ops)
}

// EndRecording finishes the session and returns a Valid Picture with a detached
// copy of ops. The recorder is reset (empty) and may be reused for the next
// recording session.
func (r *PictureRecorder) EndRecording() Picture {
	if r == nil {
		return Picture{}
	}
	ops := append([]PictureOp(nil), r.ops...)
	r.ops = nil
	var b image.Rectangle
	if r.hasBnd {
		// ceil keeps sub-pixel extents conservative (no truncation loss).
		b = image.Rect(int(r.minX), int(r.minY), int(ceilF(r.maxX)), int(ceilF(r.maxY)))
	}
	r.minX, r.minY, r.maxX, r.maxY = 0, 0, 0, 0
	r.hasBnd = false
	return Picture{
		Valid:  len(ops) > 0,
		Ops:    ops,
		Bounds: b,
	}
}

// ceilF returns the smallest int ≥ v for negative-tolerant conservative bounds.
func ceilF(v float64) int {
	i := int(v)
	if float64(i) < v {
		return i + 1
	}
	return i
}

// Finish writes the recording into dst (replaces Ops), sets Valid from non-empty,
// and resets the recorder. dst may be nil (no-op besides reset).
func (r *PictureRecorder) Finish(dst *Picture) {
	pic := r.EndRecording()
	if dst == nil {
		return
	}
	dst.Ops = pic.Ops
	dst.Valid = pic.Valid
	dst.Bounds = pic.Bounds
}

// RecordInto records via fn into an existing Picture (clears previous ops).
func RecordInto(dst *Picture, fn func(*PictureRecorder)) {
	if dst == nil || fn == nil {
		return
	}
	rec := NewPictureRecorder()
	fn(rec)
	rec.Finish(dst)
}

// RecordPicture is a convenience that returns a new recorded Picture.
func RecordPicture(fn func(*PictureRecorder)) Picture {
	rec := NewPictureRecorder()
	if fn != nil {
		fn(rec)
	}
	return rec.EndRecording()
}
