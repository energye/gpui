//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering_test

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	gputypes "github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// videoFrame builds a small non-black RGBA frame. Every pixel is mid-gray and
// opaque so a sampled destination reads either "frame gray" (drawn) or
// "clear white" (not drawn) with no ambiguity; width must stay 256-byte-row
// aligned (w*4 % 256 == 0) because the bridge's RGBA upload route has no
// padded-stride path.
func videoFrame(w, h int) []byte {
	pix := make([]byte, w*h*4)
	for i := range pix {
		pix[i] = 0x80
	}
	for i := 3; i < len(pix); i += 4 {
		pix[i] = 0xff
	}
	return pix
}

// cpuOnlyVideoBridge returns a bridge on a device that can satisfy the pool
// acquire/upload calls without a GPU session, paired with a CPU-forced
// context: DrawVideoSlot then fails closed and the bridge converges to its
// block-copy fallback, which really lands pixels in the pixmap.
func cpuOnlyVideoBridge(t *testing.T) *render.VideoBridge {
	t.Helper()
	inst, err := noop.Backend{}.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		t.Fatalf("noop CreateInstance: %v", err)
	}
	ad, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("noop RequestAdapter: %v", err)
	}
	dev, err := ad.RequestDevice(nil)
	if err != nil {
		t.Fatalf("noop RequestDevice: %v", err)
	}
	return render.NewVideoBridge(dev)
}

// livePaintedRed reports the red channel (0..65535) at (x,y) after painting
// root into a fresh CPU context — the FullPaint reference for "does this node
// actually reach the screen".
func livePaintedRed(t *testing.T, root rendering.RenderObject, w, h, x, y int) uint32 {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	root.Paint(rendering.NewPaintContext(dc, 1))
	r, _, _, _ := dc.Image().At(x, y).RGBA()
	return r
}

// contentOverVideo reports whether the retained layer tree carries anything
// drawable over the video node's rect — either a non-empty picture or a texture
// layer resolving to wantID (video's retained representation since S2b C4).
func contentOverVideo(l scene.Layer, wantID uint64) bool {
	if l == nil {
		return false
	}
	switch t := l.(type) {
	case *scene.PictureLayer:
		if t != nil && t.Recording.PaintOpCount() > 0 {
			return true
		}
	case *scene.TextureLayer:
		if t != nil && t.TextureID == wantID && t.W > 0 && t.H > 0 {
			return true
		}
	}
	for _, c := range l.Children() {
		if contentOverVideo(c, wantID) {
			return true
		}
	}
	return false
}

// retainedPaintedRed composites the built packet through the retained texture
// path and reports the red channel at (x,y) — the retained counterpart of
// livePaintedRed.
func retainedPaintedRed(t *testing.T, root rendering.RenderObject, w, h, x, y int) uint32 {
	t.Helper()
	pkt := rendering.BuildLayerTree(root).BuildPacket(7, 1, float64(w), float64(h))
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0)
	scene.CompositeFramePacketTextured(pkt, dc, tex)
	r, _, _, _ := dc.Image().At(x, y).RGBA()
	return r
}

// TestLayerTree_VideoLeafRetainedRepresentation locks that a video node with a
// real uploaded frame reaches the screen on BOTH present paths: the live
// FullPaint walk (reference) and the UI-built retained packet (the path every
// examples/video window actually runs).
//
// The retained build walk never calls node.Paint — video rides the tree as a
// scene.TextureLayer naming the route by registry id (S2b C4). This test was the
// red proof that the layer did not exist while the hole was open.
func TestLayerTree_VideoLeafRetainedRepresentation(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	br := cpuOnlyVideoBridge(t)
	defer br.Close()

	const vw, vh = 120, 80
	v := rendering.NewRenderVideo(vw, vh, br)
	if !v.UploadPlanes(64, 32, nil, nil, videoFrame(64, 32)) {
		t.Fatalf("UploadPlanes rejected a valid RGBA frame (bridge has a device)")
	}

	root := rendering.NewRenderBox(v)
	root.Layout(rendering.Tight(400, 300))

	const fullWhite = 65535
	liveRed := livePaintedRed(t, root, 400, 300, 30, 30)
	if liveRed == fullWhite {
		t.Fatal("live FullPaint walk left the video rect white: the reference path itself does not draw video")
	}

	lb := rendering.BuildLayerTree(root)
	pkt := lb.BuildPacket(7, 1, 400, 300)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	if !contentOverVideo(pkt.Root, br.ID()) {
		t.Fatalf("retained packet carries nothing drawable over the video node (%dx%d) while the live walk paints it (red=%d, clear=65535): expected a scene.TextureLayer naming route id %d",
			vw, vh, liveRed, br.ID())
	}
	if st := v.NodeStats(); st.RecordSkips != 0 {
		t.Fatalf("RecordSkips=%d want 0: the retained build must not record a video paint, it must hand over a texture layer", st.RecordSkips)
	}

	// The retained path must land the same pixels, not just a shape.
	retainedRed := retainedPaintedRed(t, root, 400, 300, 30, 30)
	if retainedRed == fullWhite {
		t.Fatalf("retained composite left the video rect white (live walk red=%d): the texture layer names the route but never reached the screen", liveRed)
	}
}

// TestLayerTree_VideoTextureRepaintsInPlace is the reason a texture layer must
// never raster-cache: the producer rewrites its texture with no UI-tree change,
// so a cached blit would freeze the frame. Same packet, two composites, new
// upload in between → the sampled pixel must move.
func TestLayerTree_VideoTextureRepaintsInPlace(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	br := cpuOnlyVideoBridge(t)
	defer br.Close()

	const vw, vh = 120, 80
	v := rendering.NewRenderVideo(vw, vh, br)
	if !v.UploadPlanes(64, 32, nil, nil, videoFrame(64, 32)) {
		t.Fatal("UploadPlanes rejected frame 1")
	}
	root := rendering.NewRenderBox(v)
	root.Layout(rendering.Tight(400, 300))
	pkt := rendering.BuildLayerTree(root).BuildPacket(7, 1, 400, 300)

	dc := render.NewContext(400, 300)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0)

	scene.CompositeFramePacketTextured(pkt, dc, tex)
	firstR, firstG, _, _ := dc.Image().At(30, 30).RGBA()

	// Second decoded frame: darker, and no layer-tree change at all.
	dark := videoFrame(64, 32)
	for i := 0; i < len(dark); i += 4 {
		dark[i] = 0x20
	}
	if !v.UploadPlanes(64, 32, nil, nil, dark) {
		t.Fatal("UploadPlanes rejected frame 2")
	}
	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	secondR, secondG, _, _ := dc.Image().At(30, 30).RGBA()

	if st.TexturesPainted != 1 {
		t.Fatalf("TexturesPainted=%d want 1 (stats=%+v)", st.TexturesPainted, st)
	}
	if firstR == secondR && firstG == secondG {
		t.Fatalf("video rect unchanged across an in-place texture rewrite: (%d,%d) then (%d,%d) — the retained path cached what it must repaint",
			firstR>>8, firstG>>8, secondR>>8, secondG>>8)
	}
}

// TestLayerTree_VideoTextureMissDamagesRect locks the closed-producer damage
// contract: a texture layer whose source is gone paints nothing, but its rect
// must still be damaged. Steady retained frames do not clear, so without the
// damage the last video frame would stay frozen on screen.
func TestLayerTree_VideoTextureMissDamagesRect(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	br := cpuOnlyVideoBridge(t)

	const vw, vh = 120, 80
	v := rendering.NewRenderVideo(vw, vh, br)
	if !v.UploadPlanes(64, 32, nil, nil, videoFrame(64, 32)) {
		t.Fatal("UploadPlanes rejected a valid frame")
	}
	root := rendering.NewRenderBox(v)
	root.Layout(rendering.Tight(400, 300))
	pkt := rendering.BuildLayerTree(root).BuildPacket(7, 1, 400, 300)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}

	dc := render.NewContext(400, 300)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0)

	first := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if first.TexturesPainted != 1 {
		t.Fatalf("first composite TexturesPainted=%d want 1 (stats=%+v)", first.TexturesPainted, first)
	}

	br.Close()
	second := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if second.TexturesMissed != 1 {
		t.Fatalf("after Close TexturesMissed=%d want 1 (stats=%+v)", second.TexturesMissed, second)
	}
	if len(second.DamageRects) == 0 {
		t.Fatal("after Close the texture layer miss added no damage: the last video frame would stay frozen")
	}
}
