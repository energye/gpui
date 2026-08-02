package scene_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/scene"
)

// TestPicture_Bounds accumulates geometry bounds from FillRect/StrokeRect and
// leaves text-only pictures with empty (unknown) bounds.
func TestPicture_Bounds(t *testing.T) {
	p := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(10, 20, 30, 40, 1, 0, 0, 1)
		r.StrokeRect(100, 200, 5, 5, 1, 0, 1, 0, 1)
	})
	b := p.Bounds
	if b.Min.X != 10 || b.Min.Y != 20 || b.Max.X != 105 || b.Max.Y != 205 {
		t.Fatalf("Bounds=%v want (10,20)-(105,205)", b)
	}
	// Text-only picture: unknown bounds stay empty.
	pt := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.DrawString("hi", 1, 2, nil, 0, 0, 0, 1)
	})
	if !pt.Bounds.Empty() {
		t.Fatalf("text-only Bounds=%v want empty", pt.Bounds)
	}
}

// TestCompositeFramePacketTextured_Fallback matches CompositeToContext output
// on the no-GPU fallback path (no offscreen textures → vector replay) while
// collecting dirty-layer damage rects.
func TestCompositeFramePacketTextured_Fallback(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(20, 10)
	b.PushClipRect(0, 0, 40, 30)
	pl := b.AddPicture(true)
	pl.SetCacheKey(101)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 40, 30, 1, 0, 0, 1)
	})
	b.Pop() // clip
	b.Pop() // offset
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0) // no GPU → record() degrades

	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if st.RasterLayerCount != 0 {
		t.Fatalf("RasterLayerCount=%d want 0 (no GPU)", st.RasterLayerCount)
	}
	// No-GPU: nothing recorded, nothing blitted, no damage rects.
	if len(st.DamageRects) != 0 {
		t.Fatalf("DamageRects=%v want none (no texture recorded)", st.DamageRects)
	}
	if st.ReplayedOps < 1 {
		t.Fatalf("ReplayedOps=%d want ≥1 (vector fallback)", st.ReplayedOps)
	}
	// Content must still be correct: red at (20+20,10+15).
	img := dc.Image()
	rr, gg, bb := sampleAt(img, 40, 25)
	if rr < 0xC000 || gg > 0x4000 || bb > 0x4000 {
		t.Fatalf("inside clip (40,25)=#%04x%04x%04x want red", rr, gg, bb)
	}
	or, og, ob := sampleAt(img, 5, 5)
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (5,5)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestCompositeFramePacketTextured_OpacityGroup keeps opacity groups on the
// blit-only path (paint-alpha fast path) and paints children correctly.
func TestCompositeFramePacketTextured_OpacityGroup(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(0, 0)
	b.PushOpacity(0.5)
	pl := b.AddPicture(true)
	pl.SetCacheKey(202)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 1, 0, 0, 1)
	})
	b.Pop() // opacity
	b.Pop() // offset
	pkt := b.BuildPacket(1, 1, 50, 50)

	dc := render.NewContext(50, 50)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0)
	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if st.ReplayedOps < 1 {
		t.Fatalf("ReplayedOps=%d want ≥1", st.ReplayedOps)
	}
	img := dc.Image()
	c := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	if c.R < 0x80 || c.G > 0x90 || c.B > 0x90 {
		t.Fatalf("opacity-50 red at (5,5)=%v want ~half red", c)
	}
}

// TestPictureTextureCache_LRU evicts entries unused since the last frame.
func TestPictureTextureCache_LRU(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	// No GPU in unit tests: entries are never created (CreateOffscreenTexture
	// returns nil) — verify the cache stays empty and counters stay zero.
	tex := scene.NewPictureTextureCache(dc, 4)
	p := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 4, 4, 1, 0, 0, 1)
	})
	_, ok := tex.RecordForTest(1, &p)
	if ok {
		t.Fatalf("record without GPU should fail")
	}
	tex.EndFrame()
	if tex.Len() != 0 {
		t.Fatalf("Len=%d want 0", tex.Len())
	}
}
