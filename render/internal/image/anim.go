//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package image

import (
	"fmt"
	"image"
	"image/color"
	"time"
)

// FrameBlend says how a frame's pixels meet what is already on the canvas.
// Values mirror the upstream frame-compositing enums (Skia/Flutter
// SkCodecAnimation::Blend, mapped 1:1 in the reference implementations):
// Src replaces the canvas rectangle, SrcOver composites by per-pixel alpha.
type FrameBlend uint8

const (
	FrameBlendSrc FrameBlend = iota
	FrameBlendSrcOver
)

// String names the blend mode for probes and logs.
func (b FrameBlend) String() string {
	if b == FrameBlendSrcOver {
		return "src-over"
	}
	return "src"
}

// FrameDisposal says what to do with a frame's rectangle once the frame stops
// being shown. Values mirror SkCodecAnimation::DisposalMethod — note the
// upstream names kKeep/kRestoreBGColor/kRestorePrevious; the legacy kDiscard /
// kNoBlend spellings are deliberately not reintroduced here.
type FrameDisposal uint8

const (
	FrameDisposalKeep FrameDisposal = iota
	FrameDisposalRestoreBGColor
	FrameDisposalRestorePrevious
)

// String names the disposal method for probes and logs.
func (d FrameDisposal) String() string {
	switch d {
	case FrameDisposalRestoreBGColor:
		return "restore-bg"
	case FrameDisposalRestorePrevious:
		return "restore-previous"
	default:
		return "keep"
	}
}

// NoRequiredFrame means a frame can be decoded without any earlier frame.
const NoRequiredFrame = -1

// FrameInfo is one frame's playback and compositing metadata. Field-for-field
// it mirrors the reference frame model (Flutter's ImageGenerator::FrameInfo:
// duration, required frame, disposal method + disposal rect + blend).
type FrameInfo struct {
	// Duration is how long the frame is shown. Zero means "show it, then move
	// on" and is legal (animated WebP encoders emit it).
	Duration time.Duration
	// RequiredFrame is the frame that must be composited first, or
	// NoRequiredFrame when this frame stands alone. A sub-rectangle frame
	// always depends on what is already on the canvas.
	RequiredFrame int
	Blend         FrameBlend
	Disposal      FrameDisposal
	// DisposalRect is the frame's rectangle on the canvas, and the area the
	// disposal method applies to.
	DisposalRect image.Rectangle
}

// animSource is one container's view of its frames. Containers stay far from
// compositing: they report metadata and hand over decoded sub-images whose
// image.Rectangle bounds say where on the canvas they belong.
type animSource interface {
	frameCount() int
	// repetition returns extra plays after the first one; negative means loop
	// forever (the upstream SkCodec convention).
	repetition() int
	frameInfo(i int) FrameInfo
	rawFrame(i int) (image.Image, error)
	// backgroundColor is what DisposalRestoreBGColor paints. GIF restores to
	// its logical-screen background colour, APNG and WebP to transparent —
	// measured on this repo's own samples against two independent decoders.
	backgroundColor() color.RGBA
}

// AnimatedImage decodes a multi-frame container and composites frames onto a
// canvas, reusing one *ImageBuf so frame advance does not create a new GPU
// texture entry per frame (generation ids are the texture cache key).
//
// Frames must be walked in order; asking for an earlier index replays from
// frame 0, which is the same cost model the reference implementations use for
// frames that depend on a previous one.
type AnimatedImage struct {
	src      animSource
	w, h     int
	cur      *ImageBuf
	canvas   []uint8 // RGBA, non-premultiplied working canvas
	needSave []uint8 // snapshot for FrameDisposalRestorePrevious
	applied  int     // last composited frame index, -1 before the first
	disposed bool
}

// newAnimatedImage wires a container to the compositor.
func newAnimatedImage(src animSource, w, h int) (*AnimatedImage, error) {
	if src == nil {
		return nil, fmt.Errorf("image: animated source is nil")
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("image: animated canvas %dx%d must be positive", w, h)
	}
	if n := src.frameCount(); n <= 0 {
		return nil, fmt.Errorf("image: animated source reports %d frames", n)
	}
	buf, err := NewImageBuf(w, h, FormatRGBA8)
	if err != nil {
		return nil, err
	}
	a := &AnimatedImage{
		src:      src,
		w:        w,
		h:        h,
		cur:      buf,
		canvas:   make([]uint8, w*h*4),
		needSave: make([]uint8, w*h*4),
		applied:  -1,
	}
	return a, nil
}

// FrameCount is the number of frames in the container.
func (a *AnimatedImage) FrameCount() int {
	if a == nil {
		return 0
	}
	return a.src.frameCount()
}

// RepetitionCount is how many extra plays follow the first one. Negative means
// loop forever. Containers count differently: APNG's num_plays is the *total*
// number of plays, so it maps to num_plays-1, while GIF's LoopCount and WebP's
// loop_count already count repetitions (with 0 meaning forever in both).
func (a *AnimatedImage) RepetitionCount() int {
	if a == nil {
		return 0
	}
	return a.src.repetition()
}

// FrameInfo returns frame i's metadata; out-of-range indices return an error
// value with a zero Duration rather than panicking.
func (a *AnimatedImage) FrameInfo(i int) (FrameInfo, error) {
	if a == nil || i < 0 || i >= a.src.frameCount() {
		return FrameInfo{}, fmt.Errorf("image: frame index %d out of range (count %d)", i, a.frameCountOrZero())
	}
	return a.src.frameInfo(i), nil
}

func (a *AnimatedImage) frameCountOrZero() int {
	if a == nil {
		return 0
	}
	return a.src.frameCount()
}

// Canvas reports the logical canvas size every frame is composited onto.
func (a *AnimatedImage) Canvas() (int, int) {
	if a == nil {
		return 0, 0
	}
	return a.w, a.h
}

// Image returns the reused buffer holding the most recently decoded frame.
// The pointer stays the same across DecodeFrame calls on purpose: the GPU
// texture cache keys on the buffer's generation id, so a stable buffer plus
// MarkPixelsDirty rewires one texture per animation instead of one per frame.
func (a *AnimatedImage) Image() *ImageBuf {
	if a == nil {
		return nil
	}
	return a.cur
}

// Dispose releases the canvas buffer. Decoding after Dispose fails.
func (a *AnimatedImage) Dispose() {
	if a == nil || a.disposed {
		return
	}
	a.disposed = true
	a.cur.Dispose()
	a.canvas = nil
	a.needSave = nil
}

// DecodeFrame composites frame i onto the canvas and publishes it through
// Image(). Frames before i are replayed when needed; the current frame is a
// no-op so a redraw loop can call it every tick.
func (a *AnimatedImage) DecodeFrame(i int) error {
	if a == nil {
		return fmt.Errorf("image: nil AnimatedImage")
	}
	if a.disposed {
		return fmt.Errorf("image: AnimatedImage already disposed")
	}
	n := a.src.frameCount()
	if i < 0 || i >= n {
		return fmt.Errorf("image: frame index %d out of range (count %d)", i, n)
	}
	if i == a.applied {
		return nil
	}
	if i != a.applied+1 {
		a.resetCanvas()
		a.applied = -1
	}
	for k := a.applied + 1; k <= i; k++ {
		if err := a.stepTo(k); err != nil {
			return err
		}
	}
	a.publish()
	return nil
}

// resetCanvas clears the working canvas to fully transparent.
func (a *AnimatedImage) resetCanvas() {
	for i := range a.canvas {
		a.canvas[i] = 0
	}
}

// stepTo applies frame k: first the previous frame's disposal, then the paint.
func (a *AnimatedImage) stepTo(k int) error {
	info := a.src.frameInfo(k)
	if k == 0 {
		a.resetCanvas()
	} else {
		prev := a.src.frameInfo(k - 1)
		if err := a.applyDisposal(prev); err != nil {
			return err
		}
	}
	if info.Disposal == FrameDisposalRestorePrevious {
		copy(a.needSave, a.canvas)
	}
	raw, err := a.src.rawFrame(k)
	if err != nil {
		return fmt.Errorf("image: frame %d: %w", k, err)
	}
	a.paint(raw, info)
	a.applied = k
	return nil
}

// applyDisposal restores the previous frame's rectangle. FrameDisposalKeep
// leaves everything as painted.
func (a *AnimatedImage) applyDisposal(prev FrameInfo) error {
	switch prev.Disposal {
	case FrameDisposalKeep:
		return nil
	case FrameDisposalRestorePrevious:
		copy(a.canvas, a.needSave)
		return nil
	case FrameDisposalRestoreBGColor:
		bg := a.src.backgroundColor()
		r := prev.DisposalRect
		if r.Empty() {
			return nil
		}
		if !r.In(a.canvasRect()) {
			r = r.Intersect(a.canvasRect())
			if r.Empty() {
				return nil
			}
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				off := (y*a.w + x) * 4
				a.canvas[off], a.canvas[off+1], a.canvas[off+2], a.canvas[off+3] = bg.R, bg.G, bg.B, bg.A
			}
		}
		return nil
	default:
		return fmt.Errorf("image: unknown disposal %d", prev.Disposal)
	}
}

func (a *AnimatedImage) canvasRect() image.Rectangle {
	return image.Rect(0, 0, a.w, a.h)
}

// paint composites one decoded sub-frame onto the canvas. The sub-image's own
// bounds give its canvas offset; pixels outside the canvas are dropped instead
// of growing the canvas, matching how the reference decoders clip.
func (a *AnimatedImage) paint(raw image.Image, info FrameInfo) {
	b := raw.Bounds()
	dst := a.canvasRect()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if y < dst.Min.Y || y >= dst.Max.Y {
			continue
		}
		for x := b.Min.X; x < b.Max.X; x++ {
			if x < dst.Min.X || x >= dst.Max.X {
				continue
			}
			// color.Color.RGBA() 返回**预乘**值；直接拿它当直色会把半透明帧算暗
			// （参照帧抓到的第一个真错）。走 NRGBAModel 拿回非预乘 8 位分量。
			c := color.NRGBAModel.Convert(raw.At(x, y)).(color.NRGBA)
			sr, sg, sbb, sa := c.R, c.G, c.B, c.A
			off := (y*a.w + x) * 4
			if info.Blend == FrameBlendSrc {
				a.canvas[off], a.canvas[off+1], a.canvas[off+2], a.canvas[off+3] = sr, sg, sbb, sa
				continue
			}
			if sa == 255 {
				a.canvas[off], a.canvas[off+1], a.canvas[off+2], a.canvas[off+3] = sr, sg, sbb, 255
				continue
			}
			if sa == 0 {
				continue // fully transparent source leaves the canvas alone
			}
			dr, dg, db, da := a.canvas[off], a.canvas[off+1], a.canvas[off+2], a.canvas[off+3]
			a.canvas[off], a.canvas[off+1], a.canvas[off+2], a.canvas[off+3] =
				blendOver(sr, sg, sbb, sa, dr, dg, db, da)
		}
	}
}

// blendOver is Porter-Duff source-over on non-premultiplied 8-bit channels.
// With an opaque destination it reduces to the plain weighted average, which is
// what the reference decoders produce for a half-transparent frame over content
// (measured: (60,80,240,128) over (200,40,40) → (130,60,140,255)).
func blendOver(sr, sg, sb, sa, dr, dg, db, da uint8) (uint8, uint8, uint8, uint8) {
	as := float32(sa) / 255
	ad := float32(da) / 255
	oa := as + ad*(1-as)
	if oa <= 0 {
		return 0, 0, 0, 0
	}
	chanOut := func(s, d uint8) uint8 {
		v := (float32(s)*as + float32(d)*ad*(1-as)) / oa
		if v <= 0 {
			return 0
		}
		if v >= 255 {
			return 255
		}
		return uint8(v + 0.5)
	}
	return chanOut(sr, dr), chanOut(sg, dg), chanOut(sb, db), uint8(oa*255 + 0.5)
}

// publish copies the working canvas into the reused ImageBuf and marks it
// dirty so the GPU path rewrites the same texture instead of allocating one.
func (a *AnimatedImage) publish() {
	copy(a.cur.Data(), a.canvas)
	a.cur.MarkPixelsDirty()
}
