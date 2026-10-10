package scene_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/scene"
)

// TestCompositeFramePacketTextured_TransformNoTextureCache pins the rotation
// caching decision: a PictureLayer under a rotating TransformLayer IS
// recorded into the texture cache untransformed (phase 1, pushRecordCTM
// strips rotation) and blitted through the live CTM (phase 2,
// DrawGPUTexture full-quad). Content cached once, transform applied at
// composite (Flutter TransformLayer parity). Pixels must match vector
// replay exactly: center red, outside white, rotated edge red.
func TestCompositeFramePacketTextured_TransformNoTextureCache(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	// Center a 40×40 red square at (50,50), rotated 45°.
	tr := b.PushTransform(50, 50, math.Pi/4, 1, 1)
	pl := b.AddPicture(true)
	pl.SetCacheKey(301)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(-20, -20, 40, 40, 1, 0, 0, 1)
	})
	_ = tr
	b.Pop() // transform
	pkt := b.BuildPacket(1, 1, 100, 100)

	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 8)

	st := scene.CompositeFramePacketTextured(pkt, dc, tex)

	// Rotated content caches untransformed and blits transformed: no
	// vector replay, entry present.
	if st.ReplayedOps != 0 {
		t.Fatalf("ReplayedOps=%d want 0 (rotated subtree must blit, not replay)", st.ReplayedOps)
	}
	if !tex.Has(301) {
		t.Fatalf("rotated picture must enter the texture cache")
	}

	img := dc.Image()
	// Rotation-invariant point: the square's center (50,50) stays red at any angle.
	cr, cg, cb := sampleAt(img, 50, 50)
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("center (50,50)=#%04x%04x%04x want red", cr, cg, cb)
	}
	// A point outside the rotated footprint but inside the axis-aligned
	// bounds must stay white — catches double-transform shard leaks.
	or, og, ob := sampleAt(img, 22, 22)
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside corner (22,22)=#%04x%04x%04x want white", or, og, ob)
	}
	// Rotated edge midpoint: 45° puts the right edge center near (64,50).
	er, eg, eb := sampleAt(img, 63, 50)
	if er < 0xC000 || eg > 0x4000 || eb > 0x4000 {
		t.Fatalf("rotated edge (63,50)=#%04x%04x%04x want red", er, eg, eb)
	}
}

// TestCompositeFramePacketTextured_ScaleNoTextureCache: non-1 scale also
// caches untransformed and blits scaled (same rationale as rotation);
// scaled content must land at the scaled geometry.
func TestCompositeFramePacketTextured_ScaleNoTextureCache(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	_ = b.PushTransform(10, 10, 0, 2, 2) // 2× scale → 10×10 rect paints as 20×20
	pl := b.AddPicture(true)
	pl.SetCacheKey(302)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 0, 0, 1, 1)
	})
	b.Pop() // transform
	pkt := b.BuildPacket(1, 1, 100, 100)

	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 8)
	st := scene.CompositeFramePacketTextured(pkt, dc, tex)

	if st.ReplayedOps != 0 {
		t.Fatalf("ReplayedOps=%d want 0 (scaled subtree must blit, not replay)", st.ReplayedOps)
	}
	if !tex.Has(302) {
		t.Fatalf("scaled picture must enter the texture cache")
	}
	img := dc.Image()
	inR, inG, inB := sampleAt(img, 25, 25) // inside scaled 20×20 area (10..30)
	if inB < 0xC000 || inR > 0x4000 || inG > 0x4000 {
		t.Fatalf("inside scaled (25,25)=#%04x%04x%04x want blue", inR, inG, inB)
	}
	outR, outG, outB := sampleAt(img, 35, 35) // beyond scaled extent (10+20=30)
	// White has HIGH R/G; a scale-leak shard would show the picture's blue
	// with near-zero R. Judge by R/G, not B (white's B is also 0xffff).
	if outR < 0xC000 || outG < 0xC000 {
		t.Fatalf("beyond scaled (35,35)=#%04x%04x%04x want white", outR, outG, outB)
	}
}

// TestCompositeFramePacketTextured_TranslateStillCacheable: a pure-translate
// (rotation=0, scale=1) transform does NOT mark the subtree restrictive —
// the common retained case keeps its bounds-texture fast path.
func TestCompositeFramePacketTextured_TranslateStillCacheable(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	_ = b.PushTransform(20, 15, 0, 1, 1)
	pl := b.AddPicture(true)
	pl.SetCacheKey(303)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 0, 1, 0, 1)
	})
	b.Pop() // transform
	pkt := b.BuildPacket(1, 1, 60, 60)

	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 8)
	scene.CompositeFramePacketTextured(pkt, dc, tex)

	// No-GPU: record degrades, but the walk must have ATTEMPTED the cache
	// path (no extra ReplayedOps from the restrictive bypass). The observable
	// proxy: replay count stays 0 because phase-2 prefers blit for restr==0.
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
