package scene_test

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

func replayOnWhite(t *testing.T, w, h int, pic scene.Picture) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pic.Replay(dc)
	return dc.Image()
}

// TestPicture_Replay_FillCirclePixels drives the new circle op onto
// render.Context: interior red, outside white.
func TestPicture_Replay_FillCirclePixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillCircle(40, 30, 15, 1, 0, 0, 1)
	})
	if pic.OpCount() != 1 || !pic.Valid {
		t.Fatalf("record failed: %+v", pic)
	}
	img := replayOnWhite(t, 80, 60, pic)
	cr, cg, cb := sampleRGB(img.At(40, 30))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("center (40,30)=#%04x%04x%04x want red", cr, cg, cb)
	}
	or, og, ob := sampleRGB(img.At(40, 52))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside (40,52)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPicture_Replay_StrokeCirclePixels: ring paints, hole stays white.
func TestPicture_Replay_StrokeCirclePixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.StrokeCircle(40, 30, 15, 4, 0, 0, 1, 1)
	})
	img := replayOnWhite(t, 80, 60, pic)
	rr, rg, rb := sampleRGB(img.At(55, 30))
	if rb < 0xC000 || rr > 0x4000 || rg > 0x4000 {
		t.Fatalf("ring (55,30)=#%04x%04x%04x want blue", rr, rg, rb)
	}
	hr, hg, hb := sampleRGB(img.At(40, 30))
	if hr < 0xC000 || hg < 0xC000 || hb < 0xC000 {
		t.Fatalf("hole (40,30)=#%04x%04x%04x want white", hr, hg, hb)
	}
}

// TestPicture_Replay_OvalPixels: bbox corners stay white, center paints.
func TestPicture_Replay_OvalPixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillOval(20, 20, 40, 20, 0, 0.7, 0, 1)
	})
	img := replayOnWhite(t, 80, 60, pic)
	gr, gg, gb := sampleRGB(img.At(40, 30))
	if gg < 0x9000 || gr > 0x6000 || gb > 0x6000 {
		t.Fatalf("center (40,30)=#%04x%04x%04x want green", gr, gg, gb)
	}
	cr, cg, cb := sampleRGB(img.At(20, 20))
	if cr < 0xC000 || cg < 0xC000 || cb < 0xC000 {
		t.Fatalf("corner (20,20)=#%04x%04x%04x want white", cr, cg, cb)
	}
}

// TestPicture_Replay_RoundRectPixels: rounded corners cut, center paints.
func TestPicture_Replay_RoundRectPixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRoundRect(20, 15, 40, 30, 8, 1, 0, 0, 1)
	})
	img := replayOnWhite(t, 80, 60, pic)
	cr, cg, cb := sampleRGB(img.At(40, 30))
	if cr < 0xC000 || cg > 0x4000 || cb > 0x4000 {
		t.Fatalf("center (40,30)=#%04x%04x%04x want red", cr, cg, cb)
	}
	or, og, ob := sampleRGB(img.At(20, 15))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("cut corner (20,15)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPicture_Replay_StrokeLinePixels: midpoint paints, off-line stays white.
func TestPicture_Replay_StrokeLinePixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.StrokeLine(10, 30, 70, 30, 6, 1, 0, 0, 1)
	})
	img := replayOnWhite(t, 80, 60, pic)
	mr, mg, mb := sampleRGB(img.At(40, 30))
	if mr < 0xC000 || mg > 0x4000 || mb > 0x4000 {
		t.Fatalf("mid (40,30)=#%04x%04x%04x want red", mr, mg, mb)
	}
	or, og, ob := sampleRGB(img.At(40, 20))
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("off-line (40,20)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestPicture_Replay_FullArcPixels: full-sweep sector covers the disc.
func TestPicture_Replay_FullArcPixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillArc(40, 30, 15, 0, 2*math.Pi, 0, 0, 1, 1)
	})
	if pic.Ops[0].Angle2 != 2*math.Pi {
		t.Fatalf("arc angles not retained: %+v", pic.Ops[0])
	}
	img := replayOnWhite(t, 80, 60, pic)
	br, bg, bb := sampleRGB(img.At(40, 30))
	if bb < 0xC000 || br > 0x4000 || bg > 0x4000 {
		t.Fatalf("center (40,30)=#%04x%04x%04x want blue", br, bg, bb)
	}
}

// TestPicture_Replay_LinearGradientPixels: left dark, right bright.
func TestPicture_Replay_LinearGradientPixels(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillLinearGradient(10, 10, 60, 20, 10, 0, 70, 0,
			0, 0, 0, 1, 1, 1, 1, 1)
	})
	img := replayOnWhite(t, 80, 60, pic)
	lr, _, _ := sampleRGB(img.At(14, 20))
	rr, _, _ := sampleRGB(img.At(66, 20))
	if lr > 0x5000 {
		t.Fatalf("left (14,20) red=%04x want dark", lr)
	}
	if rr < 0xB000 {
		t.Fatalf("right (66,20) red=%04x want bright", rr)
	}
	if !(lr < rr) {
		t.Fatalf("gradient not increasing: left=%04x right=%04x", lr, rr)
	}
}

// TestPicture_GeoOps_Guards locks facade-mirror guards: invalid geometry
// records nothing, zero alpha replays nothing, Lw<=0 still strokes.
func TestPicture_GeoOps_Guards(t *testing.T) {
	rec := scene.NewPictureRecorder()
	rec.FillCircle(10, 10, 0, 1, 0, 0, 1)
	rec.FillOval(10, 10, 0, 10, 1, 0, 0, 1)
	rec.FillRoundRect(10, 10, 10, 0, 4, 1, 0, 0, 1)
	rec.FillArc(10, 10, -3, 0, 1, 1, 0, 0, 1)
	rec.FillLinearGradient(10, 10, 0, 10, 0, 0, 1, 1, 0, 0, 0, 1, 1, 1, 1, 1)
	if rec.OpCount() != 0 {
		t.Fatalf("invalid geometry must record nothing, got %d", rec.OpCount())
	}

	// Zero alpha replays nothing.
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillCircle(40, 30, 15, 1, 0, 0, 0)
		r.StrokeLine(10, 30, 70, 30, 6, 1, 0, 0, 0)
	})
	img := replayOnWhite(t, 80, 60, pic)
	for _, pt := range [][2]int{{40, 30}, {20, 30}, {60, 30}} {
		pr, pg, pb := sampleRGB(img.At(pt[0], pt[1]))
		if pr < 0xC000 || pg < 0xC000 || pb < 0xC000 {
			t.Fatalf("(%d,%d)=#%04x%04x%04x want white for zero alpha", pt[0], pt[1], pr, pg, pb)
		}
	}

	// LineWidth<=0 normalizes to 1 and still paints (1px AA line covers
	// half the sampled pixel row, so thresholds allow partial coverage).
	pic2 := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.StrokeLine(10, 30, 70, 30, 0, 1, 0, 0, 1)
	})
	img2 := replayOnWhite(t, 80, 60, pic2)
	mr, mg, mb := sampleRGB(img2.At(40, 30))
	if mr < 0xC000 || mg > 0x9000 || mb > 0x9000 {
		t.Fatalf("lw=0 mid (40,30)=#%04x%04x%04x want red-tinted", mr, mg, mb)
	}
}

// TestPicture_GeoOps_Bounds checks damage-rect coverage for the new ops.
func TestPicture_GeoOps_Bounds(t *testing.T) {
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillCircle(40, 30, 10, 1, 0, 0, 1)
	})
	b := pic.Bounds
	if b.Min.X > 30 || b.Min.Y > 20 || b.Max.X < 50 || b.Max.Y < 40 {
		t.Fatalf("circle bounds %+v must cover disc box (30,20)-(50,40)", b)
	}

	pic2 := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillLinearGradient(10, 10, 60, 20, 10, 0, 70, 0,
			0, 0, 0, 1, 1, 1, 1, 1)
	})
	b2 := pic2.Bounds
	if b2.Dx() < 60 || b2.Dy() < 20 {
		t.Fatalf("gradient bounds %+v must cover fill rect", b2)
	}
}
