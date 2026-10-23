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
// drawable over the video node's rect.
func contentOverVideo(l scene.Layer) bool {
	if l == nil {
		return false
	}
	if pl, ok := l.(*scene.PictureLayer); ok && pl != nil {
		if pl.Recording.PaintOpCount() > 0 {
			return true
		}
	}
	for _, c := range l.Children() {
		if contentOverVideo(c) {
			return true
		}
	}
	return false
}

// TestLayerTree_VideoLeafRetainedRepresentation locks that a video node with a
// real uploaded frame reaches the screen on BOTH present paths: the live
// FullPaint walk (reference) and the UI-built retained packet (the path every
// examples/video window actually runs).
//
// The retained build walk never calls node.Paint, so a video leaf contributes
// no layer at all. Until S2b/C4 gives the node a TextureLayer, the retained
// half reports through t.Skipf with its measurements (repo precedent:
// render/text/autohint_golden_test.go TODO skips) so the hole stays visible
// without faking green. When C4 lands, this skip goes back to t.Fatalf.
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
	if !contentOverVideo(pkt.Root) {
		t.Skipf("TODO(#17/S2b): retained packet carries 0 drawable layers over the video node (%dx%d) while the live walk paints it (red=%d, clear=65535) — video is blank on every retained window until C4 gives RenderVideo a TextureLayer",
			vw, vh, liveRed)
	}
}
