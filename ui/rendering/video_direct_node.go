//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"github.com/energye/gpui/render"
)

// RenderVideo shows one decoded stream through the P2 direct bridge.
// Upload happens on the tick (UploadFrame copies synchronously, so the
// Poll pix may be recycled right after); Paint only draws the uploaded
// frame (DrawCurrent, zero-upload quad, fallback counted). It never
// decodes, never imports video/ffmpeg, never runs per-pixel loops.
//
// Layering: this node knows render only (same as RenderImage); the Player
// stays in the example, which Polls on the tick and feeds frames here.
type RenderVideo struct {
	Base
	Width, Height float64
	bridge        *render.VideoBridge
	smallW        float64
	smallH        float64
	fw            int
	fh            int
	hasFrame      bool
	drewOK        bool
	directLast    bool
	smallDrew     bool
	firstMean     float64
	firstSet      bool
	lastMean      float64
}

// NewRenderVideo creates a single-view video node of logical size w×h.
func NewRenderVideo(w, h float64, bridge *render.VideoBridge) *RenderVideo {
	v := &RenderVideo{Width: w, Height: h, bridge: bridge}
	v.Init(v)
	return v
}

// NewRenderVideoPiP creates a video node with a same-source second view
// (smallW×smallH overlay at the bottom-right): the second draw reuses the
// uploaded frame without re-uploading (ShowSeq same seq redraw).
func NewRenderVideoPiP(w, h, smallW, smallH float64, bridge *render.VideoBridge) *RenderVideo {
	v := &RenderVideo{Width: w, Height: h, bridge: bridge, smallW: smallW, smallH: smallH}
	v.Init(v)
	return v
}

// UploadFrame uploads one new frame on the tick and marks paint dirty.
// pix is copied synchronously into the texture (and the fallback shadow),
// so Poll may recycle it right after this returns.
func (v *RenderVideo) UploadFrame(fw, fh int, pix []byte) bool {
	if v == nil || v.bridge == nil || fw < 1 || fh < 1 {
		return false
	}
	need := fw * fh * 4
	if need <= 0 || len(pix) < need {
		return false
	}
	if !v.bridge.UploadFrame(fw, fh, pix[:need]) {
		return false
	}
	v.fw, v.fh = fw, fh
	v.hasFrame = true
	m := sampleMean(pix[:need])
	if !v.firstSet {
		v.firstMean, v.firstSet = m, true
	}
	v.lastMean = m
	v.MarkNeedsPaint()
	return true
}

// SetFrame keeps the old Paint-fed shape: it uploads immediately.
// Prefer UploadFrame (same cost, clearer tick/paint split).
func (v *RenderVideo) SetFrame(fw, fh int, pix []byte) bool {
	return v.UploadFrame(fw, fh, pix)
}

// sampleMean estimates frame brightness by strided sampling (cheap
// non-black gate, not a checksum).
func sampleMean(pix []byte) float64 {
	if len(pix) == 0 {
		return 0
	}
	const stride = 4096
	var sum uint64
	var n uint64
	for i := 0; i < len(pix); i += stride {
		sum += uint64(pix[i])
		n++
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

// VideoNodeStats snapshots what Paint proved: drewOK (something drawn),
// directLast (fast path), smallDrew (PiP second view), first/last frame
// brightness (non-black gate), seq (distinct frames fed).
type VideoNodeStats struct {
	DrewOK     bool
	DirectLast bool
	SmallDrew  bool
	FirstMean  float64
	LastMean   float64
}

// NodeStats snapshots the node.
func (v *RenderVideo) NodeStats() VideoNodeStats {
	if v == nil {
		return VideoNodeStats{}
	}
	return VideoNodeStats{
		DrewOK: v.drewOK, DirectLast: v.directLast, SmallDrew: v.smallDrew,
		FirstMean: v.firstMean, LastMean: v.lastMean,
	}
}

// Layout implements RenderObject.
func (v *RenderVideo) Layout(c Constraints) Size {
	if sz, ok := v.LayoutSkipIfClean(c); ok {
		return sz
	}
	out := c.Tighten(Size{Width: v.Width, Height: v.Height})
	v.setSize(out)
	v.RememberConstraints(c)
	v.clearLayoutDirty()
	return out
}

// Paint implements RenderObject — draws the latest frame via the bridge.
// No new frame fed means Paint still runs (steady damage comes from the
// bridge draws); frames below are skipped by the caller (no SetFrame, no
// upload, damage stays clean).
func (v *RenderVideo) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !v.NeedsPaint() {
		return
	}
	pc.NotePaintVisit()
	if !v.hasFrame || v.bridge == nil {
		v.clearPaintDirty()
		return
	}
	sz := v.size
	ax, ay := pc.Abs(0, 0)
	direct, ok := v.bridge.DrawCurrent(pc.DC, render.VideoDrawOptions{
		X: ax, Y: ay, DstWidth: sz.Width, DstHeight: sz.Height, Opacity: 1,
	})
	v.drewOK, v.directLast = ok, direct
	if ok && v.smallW > 0 && v.smallH > 0 {
		sx := ax + sz.Width - v.smallW - 12
		sy := ay + sz.Height - v.smallH - 12
		_, ok2 := v.bridge.DrawCurrent(pc.DC, render.VideoDrawOptions{
			X: sx, Y: sy, DstWidth: v.smallW, DstHeight: v.smallH, Opacity: 1,
		})
		if ok2 {
			v.smallDrew = true
		}
	}
	v.clearPaintDirty()
}

// HitTest implements RenderObject.
func (v *RenderVideo) HitTest(p Point) RenderObject {
	sz := v.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return v
	}
	return nil
}
