package scene_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/scene"
)

// TestCompositeFallback_RunsRasterExtra locks the retained-path contract for
// OnPaint-only layers (input-box borders): an empty picture with RasterExtra
// must still paint the extra on the vector-replay fallback instead of leaving
// a transparent hole.
func TestCompositeFallback_RunsRasterExtra(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(10, 10)
	pl := b.AddPicture(true)
	pl.SetCacheKey(303)
	// Empty picture on purpose: nothing the UI thread can capture.
	pl.RasterExtra = func(dc *render.Context) {
		dc.SetRGBA(0, 0, 1, 1)
		dc.DrawRectangle(0, 0, 20, 10)
		_ = dc.Fill()
	}
	// Bounds-sized record path (same as input-box borders): the extra paints
	// in layer-local coords, positioned by the ancestor offset at composite.
	pl.ExtraBounds = image.Rect(0, 0, 20, 10)
	b.Pop() // offset
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0) // no GPU → replay fallback

	scene.CompositeFramePacketTextured(pkt, dc, tex)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	// Content must include the extra: blue at (10+10,10+5).
	img := dc.Image()
	rr, gg, bb := sampleAt(img, 20, 15)
	if bb < 0xC000 || rr > 0x4000 || gg > 0x4000 {
		t.Fatalf("extra pixel (20,15)=#%04x%04x%04x want blue", rr, gg, bb)
	}
}
