package rendering

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

// TestTextPathCardRecords pins roster #5: a paint closure that draws text
// outlines (FillPath/StrokePath over render.TextOutlinePath) must produce the
// SAME pixels through a recording walk + replay as through the live context —
// pre-fix the closure required a live context, the S3 full-frame recording
// refused it, and the live screen lost the card while the snapshot re-render
// had it (ui_render_base_textstroke screen-vs-snapshot 17.15%).
func TestTextPathCardRecords(t *testing.T) {
	face := textPathTestFace(t)
	const W, H = 200, 120

	paint := func(dc render.Canvas) {
		dc.SetFont(face)
		dc.SetRGBA(0.9, 0.1, 0.1, 1)
		_ = dc.FillPath(render.TextOutlinePath(face, "O", 40, 90))
	}

	// Live reference.
	live := render.NewContext(W, H)
	defer live.Close()
	live.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	paint(live)
	liveImg := live.Image()

	// Recorded walk + raster replay.
	rec := recording.NewRecorder(W, H)
	rec.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	paint(rec)
	fin := rec.FinishRecording()

	replay := render.NewContext(W, H)
	defer replay.Close()
	replay.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	if err := fin.Playback(recording.NewCanvasBackend(replay)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	replayImg := replay.Image()

	diff := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			r1, g1, b1, a1 := liveImg.At(x, y).RGBA()
			r2, g2, b2, a2 := replayImg.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
			}
		}
	}
	if diff > 0 {
		t.Fatalf("replayed recording differs from live in %d px — TextPath content did not record", diff)
	}
	// And the ring really drew (not a both-empty pass).
	ink := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if _, _, _, a := liveImg.At(x, y).RGBA(); a != 0 {
				ink++
			}
		}
	}
	if ink < 200 {
		t.Fatalf("live reference has only %d ink px — fixture broken", ink)
	}
	_ = color.Black
	_ = image.Rect(0, 0, 1, 1)
}

func textPathTestFace(t *testing.T) text.Face {
	t.Helper()
	for _, p := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	} {
		if _, err := text.NewFontSourceFromFile(p); err == nil {
			src, err2 := text.NewFontSourceFromFile(p)
			if err2 == nil {
				return src.Face(48)
			}
		}
	}
	t.Skip("no TTF font available")
	return nil
}

// TestStrokeStringRecords pins roster #5 cut 2: StrokeString /
// StrokeStringAnchored through a recording walk + replay must match the live
// context pixel for pixel — pre-fix the replay painted FILLED glyphs (the
// DrawText approximation) and the anchored variant silently lost its anchor
// offset (up to a text-width shift).
func TestStrokeStringRecords(t *testing.T) {
	face := textPathTestFace(t)
	const W, H = 220, 140

	paint := func(dc render.Canvas) {
		dc.SetFont(face)
		dc.SetLineWidth(6)
		dc.SetRGBA(0.1, 0.2, 0.9, 1)
		dc.StrokeString("O", 40, 90)
		dc.SetRGBA(0.9, 0.6, 0.1, 1)
		dc.StrokeStringAnchored("Ag", 150, 70, 0.5, 0.5)
	}

	live := render.NewContext(W, H)
	defer live.Close()
	live.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	paint(live)
	liveImg := live.Image()

	rec := recording.NewRecorder(W, H)
	rec.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	paint(rec)
	fin := rec.FinishRecording()

	replay := render.NewContext(W, H)
	defer replay.Close()
	replay.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	if err := fin.Playback(recording.NewCanvasBackend(replay)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	replayImg := replay.Image()

	diff := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			r1, g1, b1, a1 := liveImg.At(x, y).RGBA()
			r2, g2, b2, a2 := replayImg.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
			}
		}
	}
	if diff > 0 {
		t.Fatalf("replayed stroked text differs from live in %d px — StrokeText replay is not a real stroke or the anchor was dropped", diff)
	}
}
