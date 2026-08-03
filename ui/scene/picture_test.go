package scene_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/scene"
)

func sampleRGB(c color.Color) (r, g, b uint32) {
	rr, gg, bb, _ := c.RGBA()
	return rr, gg, bb
}

// TestPictureRecorder_RecordsNonEmptyOps drives the shipped record path:
// FillRect/StrokeRect must produce a Valid picture with OpCount > 0.
func TestPictureRecorder_RecordsNonEmptyOps(t *testing.T) {
	rec := scene.NewPictureRecorder()
	if rec.OpCount() != 0 {
		t.Fatal("fresh recorder must be empty")
	}
	rec.FillRect(10, 10, 40, 30, 1, 0, 0, 1)
	rec.StrokeRect(5, 5, 50, 40, 2, 0, 0, 1, 1)
	if rec.OpCount() != 2 {
		t.Fatalf("recorder OpCount=%d want 2", rec.OpCount())
	}
	pic := rec.EndRecording()
	if !pic.Valid {
		t.Fatal("EndRecording must set Valid when ops present")
	}
	if pic.OpCount() != 2 {
		t.Fatalf("picture OpCount=%d want 2", pic.OpCount())
	}
	if pic.Ops[0].Kind != scene.OpFillRect || pic.Ops[1].Kind != scene.OpStrokeRect {
		t.Fatalf("op kinds %v %v", pic.Ops[0].Kind, pic.Ops[1].Kind)
	}
	if pic.Ops[0].W != 40 || pic.Ops[0].R != 1 {
		t.Fatalf("fill op fields %+v", pic.Ops[0])
	}
	// Recorder reset after EndRecording.
	if rec.OpCount() != 0 {
		t.Fatal("recorder should be empty after EndRecording")
	}
	// Empty recording is invalid.
	empty := scene.NewPictureRecorder().EndRecording()
	if empty.Valid || empty.OpCount() != 0 {
		t.Fatalf("empty recording Valid=%v ops=%d", empty.Valid, empty.OpCount())
	}
}

// TestPicture_Replay_FillRectPixels drives shipped Replay onto render.Context:
// recorded red fill must paint interior red and leave outside white.
func TestPicture_Replay_FillRectPixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(20, 15, 40, 30, 1, 0, 0, 1)
	})
	if pic.OpCount() != 1 || !pic.Valid {
		t.Fatalf("record failed: %+v", pic)
	}

	dc := render.NewContext(80, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pic.Replay(dc)

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// Interior of rect (absolute mid ~40,30).
	cr, cg, cb := sampleRGB(img.At(40, 30))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("interior (40,30)=#%04x%04x%04x want red after Replay", cr, cg, cb)
	}
	// Outside rect stays white.
	or, og, ob := sampleRGB(img.At(2, 2))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (2,2)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPictureLayer_RecordAndReplay attaches display list to PictureLayer.
// Record must leave NeedsRaster true so RasterizeDirty will apply the new ops.
func TestPictureLayer_RecordAndReplay(t *testing.T) {
	scene.ResetLayerIDGen()
	pl := scene.NewPictureLayer()
	if !pl.NeedsRaster {
		t.Fatal("new PictureLayer should NeedRaster")
	}
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(8, 8, 24, 24, 0, 0.8, 0, 1)
	})
	if !pl.NeedsRaster {
		t.Fatal("after Record, NeedsRaster must stay true until RasterizeDirty applies")
	}
	if !pl.Picture.Valid || pl.Picture.OpCount() != 1 {
		t.Fatalf("layer picture Valid=%v ops=%d", pl.Picture.Valid, pl.Picture.OpCount())
	}

	dc := render.NewContext(40, 40)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pl.Picture.Replay(dc)

	img := dc.Image()
	gr, gg, gb := sampleRGB(img.At(16, 16))
	if gg < 0xA000 {
		t.Fatalf("layer replay (16,16)=#%04x%04x%04x want green", gr, gg, gb)
	}
}

// TestRasterizeDirty_ReplaysDirtyPictureAndSkipsClean drives the natural path:
// AddPicture → Record (no manual NeedsRaster flip) → BuildPacket →
// RasterizeDirtyToContext must Replay ops (ReplayedOps>0, blue pixels).
// A second clean frame skips re-raster.
func TestRasterizeDirty_ReplaysDirtyPictureAndSkipsClean(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	pl := b.AddPicture(true)
	pl.Record(func(r *scene.PictureRecorder) {
		r.FillRect(10, 10, 20, 20, 0, 0, 1, 1)
	})
	// Natural path: Record alone must leave the layer dirty for first apply.
	if !pl.NeedsRaster {
		t.Fatal("Record must leave NeedsRaster true (no test-only re-dirty)")
	}
	if pl.Picture.OpCount() < 1 {
		t.Fatal("Record must install non-empty Ops before RasterizeDirty")
	}
	pkt := b.BuildPacket(1, 1, 64, 64)

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	// Precondition: surface still white before raster (proves Replay did the paint).
	pre := dc.Image()
	pr, pg, pb := sampleRGB(pre.At(20, 20))
	if pr < 0xC000 || pg < 0xC000 || pb < 0xC000 {
		t.Fatalf("pre-raster (20,20)=#%04x%04x%04x want white", pr, pg, pb)
	}

	st := scene.RasterizeDirtyToContext(pkt, dc)
	if st.RasterLayerCount < 1 {
		t.Fatalf("expected re-raster, stats=%+v", st)
	}
	if st.ReplayedOps < 1 {
		t.Fatalf("natural Record→RasterizeDirtyToContext must replay ops, stats=%+v", st)
	}
	if pl.NeedsRaster {
		t.Fatal("NeedsRaster should clear after RasterizeDirty")
	}
	if !pl.Picture.Valid {
		t.Fatal("Picture.Valid should be true after raster")
	}
	img := dc.Image()
	br, bg, bb := sampleRGB(img.At(20, 20))
	if bb < 0xC000 || br > 0x4000 {
		t.Fatalf("after dirty raster (20,20)=#%04x%04x%04x want blue", br, bg, bb)
	}

	// Second frame: clean — skip, no extra replay required for stats.
	pkt2 := pkt.CloneShallow()
	pkt2.DirtyLayerIDs = nil
	pkt2.FrameID = 2
	st2 := scene.RasterizeDirtyToContext(pkt2, nil)
	if st2.RasterLayerCount != 0 {
		t.Fatalf("clean frame should not re-raster, got %+v", st2)
	}
	if st2.SkippedLayerCount < 1 {
		t.Fatalf("clean frame should skip picture, got %+v", st2)
	}
	if st2.ReplayedOps != 0 {
		t.Fatalf("clean frame must not replay ops, got %+v", st2)
	}
}

// TestPicture_Invalidate_ClearsValid documents re-record flag semantics.
func TestPicture_Invalidate_ClearsValid(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 5, 5, 1, 1, 1, 1)
	})
	if !pic.Valid {
		t.Fatal("want valid")
	}
	pic.Invalidate()
	if pic.Valid {
		t.Fatal("Invalidate must clear Valid")
	}
	if pic.OpCount() != 1 {
		t.Fatal("Invalidate keeps ops until re-record/Clear")
	}
	pic.Clear()
	if pic.OpCount() != 0 || pic.Valid {
		t.Fatal("Clear must drop ops")
	}
}

// TestPicture_Replay_FillPathPixels records a triangle path and proves Replay
// paints interior blue (display-list path subset beyond rects).
func TestPicture_Replay_FillPathPixels(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(40, 10)
	p.LineTo(70, 55)
	p.LineTo(10, 55)
	p.Close()

	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillPath(p, 0, 0, 1, 1)
	})
	if pic.OpCount() != 1 || !pic.Valid {
		t.Fatalf("record path failed: ops=%d valid=%v", pic.OpCount(), pic.Valid)
	}
	if pic.Ops[0].Kind != scene.OpFillPath || pic.Ops[0].Path == nil {
		t.Fatalf("want OpFillPath with path, got %+v", pic.Ops[0])
	}
	// Mutate caller path after record — retained clone must stay intact.
	p.Clear()
	if pic.Ops[0].Path.NumVerbs() == 0 {
		t.Fatal("recorded path must be a clone independent of caller Clear")
	}

	dc := render.NewContext(80, 70)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pic.Replay(dc)

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// Centroid of triangle ≈ (40, 40).
	cr, cg, cb := sampleRGB(img.At(40, 40))
	if cb < 0xA000 || cr > 0x4000 {
		t.Fatalf("path interior (40,40)=#%04x%04x%04x want blue", cr, cg, cb)
	}
	// Outside stays white.
	or, og, ob := sampleRGB(img.At(2, 2))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (2,2)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPicture_Replay_StrokePathPixels records a stroked path and samples near the edge.
func TestPicture_Replay_StrokePathPixels(t *testing.T) {
	p := render.NewPath()
	// Horizontal line mid-canvas via closed thin rect path for stable fill-of-stroke pixels.
	p.MoveTo(10, 30)
	p.LineTo(70, 30)
	p.LineTo(70, 34)
	p.LineTo(10, 34)
	p.Close()

	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.StrokePath(p, 3, 1, 0, 0, 1)
	})
	if pic.OpCount() != 1 || pic.Ops[0].Kind != scene.OpStrokePath {
		t.Fatalf("want OpStrokePath, got ops=%d kind=%v", pic.OpCount(), pic.Ops)
	}

	dc := render.NewContext(80, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pic.Replay(dc)

	img := dc.Image()
	// Sample on the stroke band around y=30–34.
	found := false
	for y := 28; y <= 36 && !found; y++ {
		for x := 20; x <= 60; x++ {
			cr, cg, cb := sampleRGB(img.At(x, y))
			if cr > 0xA000 && cg < 0x6000 && cb < 0x6000 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected red stroke pixels after Replay StrokePath")
	}
}

// TestPicture_Replay_DrawStringPixels records text with a system face and
// asserts non-white ink after Replay (skips when no face available).
func TestPicture_Replay_DrawStringPixels(t *testing.T) {
	face, desc, err := text.LoadDefaultFace(18)
	if err != nil || face == nil {
		t.Skipf("no system face for picture text: %v", err)
	}
	t.Log("face:", desc)

	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.DrawString("Hi", 8, 28, face, 1, 0, 0, 1)
	})
	if pic.OpCount() != 1 || pic.Ops[0].Kind != scene.OpDrawString {
		t.Fatalf("want OpDrawString, got ops=%d", pic.OpCount())
	}
	if pic.Ops[0].Text != "Hi" || pic.Ops[0].Face == nil {
		t.Fatalf("text op fields %+v", pic.Ops[0])
	}

	dc := render.NewContext(80, 48)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pic.Replay(dc)

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	found := false
	for y := 0; y < 48 && !found; y++ {
		for x := 0; x < 80; x++ {
			cr, cg, cb := sampleRGB(img.At(x, y))
			if cr > 0x8000 && cg < 0x6000 && cb < 0x6000 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected red text pixels after Replay DrawString")
	}
}

// TestPicture_Replay_DrawImagePixels records a solid ImageBuf and proves Replay
// blits it (1:1 and scaled).
func TestPicture_Replay_DrawImagePixels(t *testing.T) {
	src, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer src.Dispose()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			_ = src.SetRGBA(x, y, 0, 200, 0, 255)
		}
	}

	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.DrawImage(src, 10, 10, 0, 0)   // 1:1
		r.DrawImage(src, 40, 10, 32, 32) // scaled 2×
	})
	if pic.OpCount() != 2 {
		t.Fatalf("want 2 image ops, got %d", pic.OpCount())
	}
	if pic.Ops[0].Kind != scene.OpDrawImage || pic.Ops[1].DstW != 32 {
		t.Fatalf("image op kinds/fields %+v %+v", pic.Ops[0], pic.Ops[1])
	}

	dc := render.NewContext(90, 50)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pic.Replay(dc)

	img := dc.Image()
	// 1:1 blit interior.
	cr, cg, cb := sampleRGB(img.At(18, 18))
	if cg < 0xA000 || cr > 0x4000 {
		t.Fatalf("1:1 image (18,18)=#%04x%04x%04x want green", cr, cg, cb)
	}
	// Scaled blit interior (40+16, 10+16).
	sr, sg, sb := sampleRGB(img.At(56, 26))
	if sg < 0xA000 || sr > 0x4000 {
		t.Fatalf("scaled image (56,26)=#%04x%04x%04x want green", sr, sg, sb)
	}
	// Outside stays white.
	or, og, ob := sampleRGB(img.At(2, 2))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (2,2)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPictureRecorder_MixedOps records rect+path+image in one list.
func TestPictureRecorder_MixedOps(t *testing.T) {
	p := render.NewPath()
	p.Rectangle(50, 10, 20, 20)
	src, err := render.NewImageBuf(4, 4, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Dispose()
	_ = src.SetRGBA(0, 0, 255, 0, 0, 255)

	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 0, 0, 1, 1)
		r.FillPath(p, 0, 1, 0, 1)
		r.DrawImage(src, 0, 20, 0, 0)
		r.DrawString("x", 0, 40, nil, 0, 0, 0, 1) // face nil ok at record
	})
	if pic.OpCount() != 4 {
		t.Fatalf("mixed OpCount=%d want 4", pic.OpCount())
	}
	want := []scene.PictureOpKind{
		scene.OpFillRect, scene.OpFillPath, scene.OpDrawImage, scene.OpDrawString,
	}
	for i, k := range want {
		if pic.Ops[i].Kind != k {
			t.Fatalf("op[%d] kind=%v want %v", i, pic.Ops[i].Kind, k)
		}
	}
}

// TestPicture_Bounds_IncludesPathAndImage proves the rewritten recorder folds
// path geometry and image destination rects into Picture.Bounds (previously
// only rects were noted — path/image ops were invisible to damage rects).
func TestPicture_Bounds_IncludesPathAndImage(t *testing.T) {
	p := render.NewPath()
	p.Rectangle(50, 10, 20, 20) // covers (50,10)-(70,30)
	src, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Dispose()

	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 1, 0, 0, 1) // covers (0,0)-(10,10)
		r.FillPath(p, 0, 1, 0, 1)            // covers (50,10)-(70,30)
		r.DrawImage(src, 80, 40, 0, 0)       // 1:1 covers (80,40)-(96,56)
	})
	if pic.Bounds.Empty() {
		t.Fatal("Bounds must include path and image geometry")
	}
	want := image.Rect(0, 0, 96, 56)
	if pic.Bounds != want {
		t.Fatalf("Bounds=%v want %v", pic.Bounds, want)
	}
}

// TestPicture_Bounds_StrokeInflates verifies StrokePath inflates bounds by half
// the line width so the stroke band is inside the damage rect.
func TestPicture_Bounds_StrokeInflates(t *testing.T) {
	p := render.NewPath()
	p.Rectangle(10, 10, 20, 20)
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.StrokePath(p, 6, 1, 0, 0, 1)
	})
	if pic.Bounds.Empty() {
		t.Fatal("stroke path must contribute bounds")
	}
	if pic.Bounds.Min.X > 7 || pic.Bounds.Min.Y > 7 {
		t.Fatalf("stroke bounds not inflated by half line width: %v", pic.Bounds)
	}
}

// TestPicture_Replay_ZeroAlphaDrawsNothing proves strict SkPaint alpha semantics
// after rewrite: A==0 paints nothing (no implicit opaque fallback).
func TestPicture_Replay_ZeroAlphaDrawsNothing(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(10, 10, 40, 30, 1, 0, 0, 0)
	})
	if pic.OpCount() != 1 {
		t.Fatalf("zero-alpha op must still be recorded, got %d", pic.OpCount())
	}
	dc := render.NewContext(60, 50)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pic.Replay(dc)
	img := dc.Image()
	cr, cg, cb := sampleRGB(img.At(30, 25))
	if cr < 0xC000 || cg < 0xC000 || cb < 0xC000 {
		t.Fatalf("zero-alpha replay (30,25)=#%04x%04x%04x want white", cr, cg, cb)
	}
}

// TestPictureRecorder_ClampsPaint verifies record-time normalization keeps
// color channels in [0,1] (defensive paint state in the display list).
func TestPictureRecorder_ClampsPaint(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 1.7, -0.2, 0.5, 1.4)
	})
	op := pic.Ops[0]
	if op.R != 1 || op.G != 0 || op.B != 0.5 || op.A != 1 {
		t.Fatalf("clamped paint=%+v want R=1 G=0 B=0.5 A=1", op)
	}
}
