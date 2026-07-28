package scene_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/scene"
)

func sampleAt(img interface{ At(int, int) color.Color }, x, y int) (r, g, b uint32) {
	rr, gg, bb, _ := img.At(x, y).RGBA()
	return rr, gg, bb
}

// TestCompositeToContext_OffsetClipPicture: offset + clip_rect + picture fill
// must paint interior red and leave outside-clip white.
func TestCompositeToContext_OffsetClipPicture(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(20, 10)
	b.PushClipRect(0, 0, 40, 30)
	pl := b.AddPicture(true)
	pl.Record(func(r *scene.PictureRecorder) {
		// Draw larger than clip so clip must bite.
		r.FillRect(-5, -5, 80, 80, 1, 0, 0, 1)
	})
	b.Pop() // clip
	b.Pop() // offset

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	st := scene.CompositeToContext(b.Root(), dc)
	if st.PicturesDrawn < 1 || st.ClipsApplied < 1 {
		t.Fatalf("stats=%+v want picture+clip", st)
	}

	img := dc.Image()
	// Inside offset+clip: abs (20+20, 10+15)=(40,25) red.
	rr, gg, bb := sampleAt(img, 40, 25)
	if rr < 0xC000 || gg > 0x4000 || bb > 0x4000 {
		t.Fatalf("inside clip (40,25)=#%04x%04x%04x want red", rr, gg, bb)
	}
	// Outside clip but on canvas: (5,5) white.
	or, og, ob := sampleAt(img, 5, 5)
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (5,5)=#%04x%04x%04x want white", or, og, ob)
	}
	// Just outside clip right edge: offset 20 + clip w 40 → x>=60 white.
	xr, xg, xb := sampleAt(img, 70, 25)
	if xr < 0xC000 || xg < 0xC000 || xb < 0xC000 {
		t.Fatalf("right of clip (70,25)=#%04x%04x%04x want white", xr, xg, xb)
	}
}

// TestCompositeToContext_TransformTranslate moves picture content by TX,TY.
func TestCompositeToContext_TransformTranslate(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushTransform(50, 20, 0, 1, 1)
	pl := b.AddPicture(true)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 20, 20, 0, 0, 1, 1)
	})
	b.Pop()

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	st := scene.CompositeToContext(b.Root(), dc)
	if st.TransformsApplied < 1 || st.PicturesDrawn < 1 {
		t.Fatalf("stats=%+v", st)
	}
	img := dc.Image()
	// Blue should appear near (50+10, 20+10)=(60,30), not at (10,10).
	br, bg, bb := sampleAt(img, 60, 30)
	if bb < 0xC000 || br > 0x4000 {
		t.Fatalf("(60,30)=#%04x%04x%04x want blue after translate", br, bg, bb)
	}
	or, og, ob := sampleAt(img, 10, 10)
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("(10,10)=#%04x%04x%04x want white (untransformed origin)", or, og, ob)
	}
}

// TestCompositeToContext_ClipRRectRoundsCorner: hard-rect corner void stays near-white.
func TestCompositeToContext_ClipRRectRoundsCorner(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushClipRRect(10, 10, 60, 60, 20)
	pl := b.AddPicture(true)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(10, 10, 60, 60, 1, 0, 0, 1)
	})
	b.Pop()

	dc := render.NewContext(90, 90)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	_ = scene.CompositeToContext(b.Root(), dc)
	img := dc.Image()
	// Interior center red.
	cr, cg, cb := sampleAt(img, 40, 40)
	if cr < 0xC000 || cg > 0x4000 {
		t.Fatalf("center want red got #%04x%04x%04x", cr, cg, cb)
	}
	// Top-left hard corner of bounds is outside r=20 arc.
	tr, tg, tb := sampleAt(img, 11, 11)
	if tg < 0xA000 || tb < 0xA000 {
		t.Fatalf("rrect corner void (11,11)=#%04x%04x%04x want near-white", tr, tg, tb)
	}
}

// TestCompositeFramePacket_OverlayAboveMain draws overlay picture after main.
func TestCompositeFramePacket_OverlayAboveMain(t *testing.T) {
	scene.ResetLayerIDGen()
	main := scene.NewLayerBuilder()
	pl := main.AddPicture(true)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 80, 80, 0, 0, 1, 1) // blue full
	})
	ov := scene.NewLayerBuilder()
	opl := ov.AddPicture(true)
	opl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(20, 20, 20, 20, 1, 0, 0, 1) // red patch
	})
	pkt := main.BuildPacket(1, 1, 80, 80)
	pkt.Overlay = ov.Root()

	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	st := scene.CompositeFramePacket(pkt, dc)
	if st.PicturesDrawn < 2 {
		t.Fatalf("want ≥2 pictures, stats=%+v", st)
	}
	img := dc.Image()
	// Overlay red on top.
	rr, rg, rb := sampleAt(img, 30, 30)
	if rr < 0xC000 || rg > 0x4000 {
		t.Fatalf("overlay center want red got #%04x%04x%04x", rr, rg, rb)
	}
	// Main blue still visible outside overlay.
	br, bg, bb := sampleAt(img, 70, 70)
	if bb < 0xA000 {
		t.Fatalf("main corner want blue-ish got #%04x%04x%04x", br, bg, bb)
	}
}

// TestCompositeToContext_OpacityDimsContent: opacity layer reduces painted alpha.
func TestCompositeToContext_OpacityDimsContent(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOpacity(0.4)
	pl := b.AddPicture(true)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(10, 10, 40, 40, 1, 0, 0, 1)
	})
	b.Pop()

	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	_ = scene.CompositeToContext(b.Root(), dc)
	img := dc.Image()
	rr, rg, rb := sampleAt(img, 30, 30)
	// Blended over white at 0.4 → R high but not full; G/B elevated from white mix.
	if rr < 0x6000 {
		t.Fatalf("expected visible red tint #%04x%04x%04x", rr, rg, rb)
	}
	if rr > 0xF000 && rg < 0x2000 && rb < 0x2000 {
		t.Fatalf("looks fully opaque red #%04x%04x%04x — opacity layer ineffective", rr, rg, rb)
	}
}
