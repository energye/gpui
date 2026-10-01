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

// RenderVideo shows one decoded stream through the P2 direct bridge
// (render.VideoBridge): the window feeds raw RGBA frames, Paint draws via
// ShowSeq (upload once, zero-upload quad, fallback counted). It never
// decodes, never imports video/ffmpeg, never runs per-pixel loops: frame
// bytes arrive by SetFrame (one block copy into an owned buffer) and the
// bridge owns upload/draw/fallback.
//
// Layering: this node knows render only (same as RenderImage); the Player
// stays in the example, which Polls on the tick and feeds frames here.
type RenderVideo struct {
	Base
	Width, Height float64
	bridge        *render.VideoBridge
	smallW        float64
	smallH        float64
	buf           []byte
	fw            int
	fh            int
	seq           uint64
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

// SetFrame installs one decoded frame (fw×fh RGBA). It block-copies pix
// into an owned buffer (Poll recycles its Pix on the next call) and marks
// paint dirty. Bad args are rejected without touching the current picture.
func (v *RenderVideo) SetFrame(fw, fh int, pix []byte) bool {
	if v == nil || fw < 1 || fh < 1 {
		return false
	}
	need := fw * fh * 4
	if need <= 0 || len(pix) < need {
		return false
	}
	if len(v.buf) != need {
		v.buf = make([]byte, need)
		v.fw, v.fh = fw, fh
	}
	copy(v.buf, pix[:need])
	v.seq++
	v.hasFrame = true
	m := sampleMean(pix[:need])
	if !v.firstSet {
		v.firstMean, v.firstSet = m, true
	}
	v.lastMean = m
	v.MarkNeedsPaint()
	return true
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
	Seq        uint64
}

// NodeStats snapshots the node.
func (v *RenderVideo) NodeStats() VideoNodeStats {
	if v == nil {
		return VideoNodeStats{}
	}
	return VideoNodeStats{
		DrewOK: v.drewOK, DirectLast: v.directLast, SmallDrew: v.smallDrew,
		FirstMean: v.firstMean, LastMean: v.lastMean, Seq: v.seq,
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
	direct, ok := v.bridge.ShowSeq(v.seq, pc.DC, v.fw, v.fh, v.buf, render.VideoDrawOptions{
		X: ax, Y: ay, DstWidth: sz.Width, DstHeight: sz.Height, Opacity: 1,
	})
	v.drewOK, v.directLast = ok, direct
	if ok && v.smallW > 0 && v.smallH > 0 {
		sx := ax + sz.Width - v.smallW - 12
		sy := ay + sz.Height - v.smallH - 12
		_, ok2 := v.bridge.ShowSeq(v.seq, pc.DC, v.fw, v.fh, v.buf, render.VideoDrawOptions{
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
