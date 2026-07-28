package scene_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
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
