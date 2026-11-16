//go:build !(js && wasm)

package scene_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// TestCompositeFramePacketTextured_Counters locks the P0 observation
// contract: a clean retained frame reuses the cached picture texture
// (BlitsIssued > 0) while the phase-2 walk still visits the whole tiny tree
// and culls nothing yet.
func TestCompositeFramePacketTextured_Counters(t *testing.T) {
	scene.ResetLayerIDGen()
	dc := render.NewContext(50, 50)
	defer dc.Close()
	if !gpuOffscreenActive(dc) {
		t.Skipf("GPU offscreen device unavailable: texture blits cannot be observed")
	}

	build := func(dirty bool) *scene.FramePacket {
		b := scene.NewLayerBuilder()
		b.PushOffset(0, 0)
		pl := b.AddPicture(dirty)
		pl.SetCacheKey(303)
		pl.SetRecording(recordFillRect(20, 20, 0, 0, 20, 20, 1, 0, 0, 1))
		b.Pop() // offset
		return b.BuildPacket(1, 1, 50, 50)
	}

	dc.BeginFrame()
	tex := scene.NewPictureTextureCache(dc, 0)
	st1 := scene.CompositeFramePacketTextured(build(true), dc, tex)
	if st1.LayersVisited == 0 {
		t.Fatal("LayersVisited=0 want >0")
	}
	if st1.LayersCulled != 0 {
		t.Fatalf("LayersCulled=%d want 0 (P0 has no culling)", st1.LayersCulled)
	}
	if st1.BlitsIssued != 0 {
		t.Fatalf("BlitsIssued=%d want 0 (first frame records, no blit)", st1.BlitsIssued)
	}

	dc.BeginFrame()
	st2 := scene.CompositeFramePacketTextured(build(false), dc, tex)
	if st2.LayersVisited != st1.LayersVisited {
		t.Fatalf("LayersVisited=%d want %d (same tree)", st2.LayersVisited, st1.LayersVisited)
	}
	if st2.LayersCulled != 0 {
		t.Fatalf("LayersCulled=%d want 0 (P0 has no culling)", st2.LayersCulled)
	}
	if st2.BlitsIssued == 0 {
		t.Fatal("BlitsIssued=0 want >0 (clean frame must blit the cached texture)")
	}
	if st2.RasterLayerCount != 0 {
		t.Fatalf("RasterLayerCount=%d want 0 (clean layer must not re-record)", st2.RasterLayerCount)
	}
}
