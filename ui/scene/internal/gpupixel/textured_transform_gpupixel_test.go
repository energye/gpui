package gpupixel

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// paintWhite fills the composite surface and presents it so the offscreen
// view starts white: the outside-of-content probes below must prove the
// transform blit does not leak, and white is the same oracle the old
// headless tests used.
func paintWhite(t *testing.T, dc *render.Context, view render.TextureView, w, h int) {
	t.Helper()
	for i := 0; i < 2; i++ {
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		if err := dc.PresentFrame(view, uint32(w), uint32(h), func() error { return nil }); err != nil { //nolint:gosec // test dimensions are small constants
			t.Fatalf("white backdrop present %d: %v", i, err)
		}
	}
}

func readbackView(t *testing.T, view render.TextureView, w, h int) func(x, y int) (uint8, uint8, uint8) {
	t.Helper()
	out := render.NewContext(w, h)
	t.Cleanup(func() { out.Close() })
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, w, h)
	if err := out.FlushGPU(); err != nil {
		t.Fatalf("readback flush: %v", err)
	}
	img := out.Image()
	return func(x, y int) (uint8, uint8, uint8) {
		r, g, b, _ := img.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
	}
}

func assertPixel(t *testing.T, what string, r, g, b uint8, wantRed, wantBlue, wantWhite bool) {
	t.Helper()
	switch {
	case wantRed:
		if r < 180 || g > 100 || b > 100 {
			t.Fatalf("%s rgb=%d,%d,%d want red", what, r, g, b)
		}
	case wantBlue:
		if b < 180 || r > 100 || g > 100 {
			t.Fatalf("%s rgb=%d,%d,%d want blue", what, r, g, b)
		}
	case wantWhite:
		if r < 180 || g < 180 || b < 180 {
			t.Fatalf("%s rgb=%d,%d,%d want white (transform blit leaked)", what, r, g, b)
		}
	}
}

// TestCompositeFramePacketTextured_RotateBlitPixels is the GPU pixel
// contract for rotated content: the picture records untransformed
// (pushRecordCTM strips rotation) and phase 2 blits it through the live
// CTM. Center and rotated edge stay red; the axis-aligned bounds corner
// outside the rotated footprint stays white (no double-transform shard).
func TestCompositeFramePacketTextured_RotateBlitPixels(t *testing.T) {
	requireGPU(t)
	const W, H = 100, 100
	dc := render.NewContext(W, H)
	defer dc.Close()
	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	scene.ResetLayerIDGen()
	lb := scene.NewLayerBuilder()
	lb.PushTransform(50, 50, math.Pi/4, 1, 1)
	pl := lb.AddPicture(true)
	pl.SetCacheKey(9011)
	pl.SetRecording(gpuiFillRect(40, 40, -20, -20, 40, 40, 1, 0, 0, 1))
	lb.Pop()
	pkt := lb.BuildPacket(1, 1, W, H)

	paintWhite(t, dc, view, W, H)
	tex := scene.NewPictureTextureCache(dc, 8)
	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !tex.Has(9011) {
		t.Fatalf("rotated picture must enter the texture cache")
	}
	if st.ReplayedOps != 0 {
		t.Fatalf("ReplayedOps=%d want 0 (rotated subtree must blit, not replay)", st.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame flush: %v", err)
	}

	sample := readbackView(t, view, W, H)
	r, g, b := sample(50, 50)
	assertPixel(t, "center (50,50)", r, g, b, true, false, false)
	r, g, b = sample(22, 22)
	assertPixel(t, "outside corner (22,22)", r, g, b, false, false, true)
	r, g, b = sample(63, 50)
	assertPixel(t, "rotated edge (63,50)", r, g, b, true, false, false)
}

// TestCompositeFramePacketTextured_ScaleBlitPixels: non-1 scale also caches
// untransformed and blits scaled; content lands at the scaled geometry and
// nothing leaks past it.
func TestCompositeFramePacketTextured_ScaleBlitPixels(t *testing.T) {
	requireGPU(t)
	const W, H = 100, 100
	dc := render.NewContext(W, H)
	defer dc.Close()
	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	scene.ResetLayerIDGen()
	lb := scene.NewLayerBuilder()
	lb.PushTransform(10, 10, 0, 2, 2) // 10×10 local rect paints as 20×20
	pl := lb.AddPicture(true)
	pl.SetCacheKey(9012)
	pl.SetRecording(gpuiFillRect(10, 10, 0, 0, 10, 10, 0, 0, 1, 1))
	lb.Pop()
	pkt := lb.BuildPacket(1, 1, W, H)

	paintWhite(t, dc, view, W, H)
	tex := scene.NewPictureTextureCache(dc, 8)
	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !tex.Has(9012) {
		t.Fatalf("scaled picture must enter the texture cache")
	}
	if st.ReplayedOps != 0 {
		t.Fatalf("ReplayedOps=%d want 0 (scaled subtree must blit, not replay)", st.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame flush: %v", err)
	}

	sample := readbackView(t, view, W, H)
	r, g, b := sample(25, 25)
	assertPixel(t, "inside scaled (25,25)", r, g, b, false, true, false)
	r, g, b = sample(35, 35)
	assertPixel(t, "beyond scaled (35,35)", r, g, b, false, false, true)
}
