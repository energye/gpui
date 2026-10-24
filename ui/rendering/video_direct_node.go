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
// Upload happens on the tick (UploadPlanes copies synchronously, so the
// Poll planes may be recycled right after); Paint only draws the uploaded
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
	planesLast    bool
	firstMean     float64
	firstSet      bool
	lastMean      float64
	// recordSkips counts Paints that hit a recording walk: video draws GPU
	// texture handles, which a display list cannot describe (see Paint). The
	// retained tree carries video as a scene.TextureLayer instead, so a steady
	// retained window keeps this at zero.
	recordSkips int
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

// UploadPlanes uploads one new frame on the tick and marks paint dirty.
// This is the only window-facing feed (P3-A layer 5): y/uv carry NV12
// planes, pix carries the RGBA fallback for odd-size or exotic frames
// (nil-safe, shapes the decoder never emits together). Planes go
// through the GPU YUV convert; the fallback shape goes through the
// bridge RGBA route and counts one fallback. All slices are copied
// synchronously, so Poll may recycle them right after this returns.
func (v *RenderVideo) UploadPlanes(fw, fh int, y, uv, pix []byte) bool {
	if v == nil || v.bridge == nil || fw < 1 || fh < 1 {
		return false
	}
	if len(y) > 0 && len(uv) > 0 {
		yN, uvN := fw*fh, fw*fh/2
		if yN > 0 && uvN > 0 && len(y) >= yN && len(uv) >= uvN &&
			v.bridge.UploadPlanesFrame(fw, fh, y[:yN], uv[:uvN]) {
			v.planesLast = true
			v.noteFrame(fw, fh, y[:yN])
			return true
		}
		// Plane route rejected (odd size / no plane device / upload error):
		// fall through to the RGBA shape instead of dropping the frame.
	}
	need := fw * fh * 4
	if need <= 0 || len(pix) < need {
		return false
	}
	if !v.bridge.UploadFrame(fw, fh, pix[:need]) {
		return false
	}
	v.planesLast = false
	v.noteFrame(fw, fh, pix[:need])
	return true
}

// noteFrame records size, brightness gate, and paint dirtiness.
func (v *RenderVideo) noteFrame(fw, fh int, sample []byte) {
	v.fw, v.fh = fw, fh
	v.hasFrame = true
	m := sampleMean(sample)
	if !v.firstSet {
		v.firstMean, v.firstSet = m, true
	}
	v.lastMean = m
	v.MarkNeedsPaint()
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

// VideoNodeStats snapshots what the bridge proved: DrewOK/SmallDrew (something
// drawn at least once), DrewLast/SmallDrewLast (the most recent paint outcome —
// gates that assert the current picture must use the Last pair, or a route that
// stopped painting would still pass on its old ever-drew flag), DirectLast
// (fast path), first/last frame brightness (non-black gate), planesLast
// (last upload took the NV12 planes path, not the RGBA fallback shape),
// RecordSkips (Paints that met a recording walk and could not draw).
//
// The draw outcomes come from the bridge, not from this node: the retained path
// paints texture layers from the raster job, so the producer is the only place
// that sees whether pixels reached the screen (see render.VideoBridge.PaintStats).
type VideoNodeStats struct {
	DrewOK        bool
	DrewLast      bool
	DirectLast    bool
	SmallDrew     bool
	SmallDrewLast bool
	FirstMean     float64
	LastMean      float64
	PlanesLast    bool
	RecordSkips   int
}

// NodeStats snapshots the node.
func (v *RenderVideo) NodeStats() VideoNodeStats {
	if v == nil {
		return VideoNodeStats{}
	}
	st := VideoNodeStats{
		FirstMean: v.firstMean, LastMean: v.lastMean, PlanesLast: v.planesLast,
		RecordSkips: v.recordSkips,
	}
	if v.bridge != nil {
		ps := v.bridge.PaintStats()
		st.DrewOK, st.DirectLast = ps.ViewOK[0], ps.ViewDirect[0]
		st.DrewLast = ps.ViewLastOK[0]
		if len(ps.ViewOK) > 1 {
			st.SmallDrew = ps.ViewOK[1]
			st.SmallDrewLast = ps.ViewLastOK[1]
		}
	}
	return st
}

// TextureLeaves implements TextureLeafProvider: the retained tree gets the
// route's registry id plus this node's quads, never a pointer to the bridge.
// Coordinates are layer-local (0-based) — the ancestor offset chain positions
// them, which is how the PiP overlay lands without any absolute math here.
// No frame yet (or a closed route) means no leaf: the builder then keeps the
// ordinary empty picture slot rather than a layer that can never paint.
func (v *RenderVideo) TextureLeaves() []TextureLeaf {
	if v == nil || v.bridge == nil || !v.hasFrame {
		return nil
	}
	id := v.bridge.ID()
	if id == 0 {
		return nil
	}
	sz := v.size
	if sz.Width <= 0 || sz.Height <= 0 {
		return nil
	}
	leaves := []TextureLeaf{{TextureID: id, W: sz.Width, H: sz.Height}}
	if v.smallW > 0 && v.smallH > 0 {
		leaves = append(leaves, TextureLeaf{
			TextureID: id,
			X:         sz.Width - v.smallW - 12,
			Y:         sz.Height - v.smallH - 12,
			W:         v.smallW, H: v.smallH, DrawIndex: 1,
		})
	}
	return leaves
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
// bridge draws); frames below are skipped by the caller (no UploadPlanes,
// no upload, damage stays clean).
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
	dc, live := pc.LiveContext()
	if !live {
		// Video rides GPU texture handles (bridge → DrawVideoSlot/Planes), which a
		// display list cannot describe. Counted, never silent: S2b C4 puts video on
		// a scene.TextureLayer so the retained tree carries it instead.
		v.recordSkips++
		v.clearPaintDirty()
		return
	}
	// Both views draw through the texture-source entry, so the route's per-view
	// accounting sees exactly the draws the retained path would file: "did
	// pixels reach the screen" is decided in one place, not per present path.
	_, ok := v.bridge.PaintTexture(dc, render.TexturePaintOptions{
		X: ax, Y: ay, Width: sz.Width, Height: sz.Height, Opacity: 1,
	})
	if ok && v.smallW > 0 && v.smallH > 0 {
		sx := ax + sz.Width - v.smallW - 12
		sy := ay + sz.Height - v.smallH - 12
		v.bridge.PaintTexture(dc, render.TexturePaintOptions{
			X: sx, Y: sy, Width: v.smallW, Height: v.smallH,
			Opacity: 1, DrawIndex: 1,
		})
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
