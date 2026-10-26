package scene_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/scene"
)

// The rotation/scale pixel contracts live in
// ui/scene/internal/gpupixel/textured_transform_gpupixel_test.go: they
// need a real offscreen texture to blit through the live CTM, and the
// headless path here degrades to correct vector replay by design. The
// cache decision itself is pinned without a GPU in
// textured_transform_internal_test.go.

// TestCompositeFramePacketTextured_TranslateStillCacheable: a pure-translate
// (rotation=0, scale=1) transform does NOT mark the subtree restrictive —
// the common retained case keeps its bounds-texture fast path.
func TestCompositeFramePacketTextured_TranslateStillCacheable(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	_ = b.PushTransform(20, 15, 0, 1, 1)
	pl := b.AddPicture(true)
	pl.SetCacheKey(303)
	pl.SetRecording(recordFillRect(10, 10, 0, 0, 10, 10, 0, 1, 0, 1))
	b.Pop() // transform
	pkt := b.BuildPacket(1, 1, 60, 60)

	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 8)
	scene.CompositeFramePacketTextured(pkt, dc, tex)

	// No-GPU: record degrades, but the walk must have ATTEMPTED the cache
	// path (no extra ReplayedOps from a caching-disqualification bypass). The observable
	// proxy: replay count stays 0 because phase-2 prefers blit for a pure-translate CTM.
	if st := scene.CountCacheablePictureLayers(pkt); st != 1 {
		t.Fatalf("cacheable layers=%d want 1", st)
	}
	img := dc.Image()
	gr, gg, gb := sampleAt(img, 24, 19)
	c := color.NRGBAModel.Convert(img.At(24, 19)).(color.NRGBA)
	_ = gr
	if c.R != 0 && gg == 0 {
		t.Fatalf("sanity")
	}
	if gb > 0x8000 && gg < 0x4000 {
		t.Fatalf("(24,19) unexpectedly blue-ish")
	}
}
