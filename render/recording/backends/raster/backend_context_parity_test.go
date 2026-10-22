package raster

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// paintParity draws the same scene through any render.Canvas. Both targets of
// the contract take it: a live *render.Context, which rasterizes as the calls
// arrive, and a *recording.Recorder, which stores them as a display list. A
// pixel difference between the two is a bug in the recording layer, not in the
// scene — the hand-written adapter this test used to need is gone because the
// Recorder now satisfies render.Canvas for real.
//
// The cases are deliberate: a transform changed while a path is open, a
// rotated fill, a stroke under scale, nested save/restore, a raw-copied path,
// a rectangular clip, an image draw, an opacity layer, a post-effect and a
// mesh that inherits the fill paint.
func paintParity(p render.Canvas) {
	p.Push()
	p.Translate(20, 15)
	p.SetRGBA(1, 0, 0, 1)
	p.MoveTo(0, 0)
	p.LineTo(40, 0)
	// CTM changes mid-path: live bakes these verbs with the new matrix.
	p.Translate(5, 8)
	p.LineTo(30, 30)
	p.ClosePath()
	_ = p.Fill()
	p.Pop()

	p.Push()
	p.Translate(60, 30)
	p.Rotate(0.6)
	p.SetRGBA(0, 0, 1, 1)
	p.ClearPath()
	p.MoveTo(0, 0)
	p.QuadraticTo(20, -18, 40, 0)
	p.CubicTo(48, 12, 30, 26, 12, 20)
	p.ClosePath()
	_ = p.Fill()
	p.Pop()

	p.Push()
	p.Translate(18, 70)
	p.Scale(2, 1)
	p.SetRGBA(0, 1, 0, 1)
	p.SetLineWidth(3)
	p.ClearPath()
	p.MoveTo(0, 0)
	p.LineTo(30, 14)
	p.LineTo(56, 2)
	_ = p.Stroke()
	p.Pop()

	// Raw-copied path: a live Context does not apply the CTM to these
	// coordinates, so the recording has to preserve that too.
	p.Push()
	p.Translate(70, 8)
	p.SetRGBA(1, 1, 0, 1)
	_ = p.FillPath(diamondPath())
	p.Pop()

	// Rectangular clip under a transform, then content that straddles it.
	p.Push()
	p.Translate(10, 96)
	p.ClipRect(0, 0, 24, 20)
	p.SetRGBA(1, 0, 1, 1)
	p.DrawRectangle(-6, -6, 48, 40)
	_ = p.Fill()
	p.Pop()

	// Image draw through the recorded options.
	p.Push()
	p.Translate(44, 96)
	p.DrawImageEx(parityImage(), render.DrawImageOptions{
		DstWidth:  32,
		DstHeight: 16,
		Opacity:   1,
	})
	p.Pop()

	// Group opacity: overlapping draws must composite among themselves.
	p.Push()
	p.PushLayerIsolated(0.5)
	p.SetRGBA(0, 0, 0, 1)
	p.DrawCircle(150, 104, 12)
	_ = p.Fill()
	p.DrawCircle(162, 104, 12)
	_ = p.Fill()
	p.PopLayer()
	p.Pop()

	// Immediate-mode mesh with no per-vertex colours: Context falls back to the
	// fill paint, so the recording has to carry the paint that was current at
	// record time rather than whatever the replay's previous command left set.
	p.Push()
	p.Translate(104, 96)
	p.Scale(1.5, 1)
	p.SetRGBA(1, 0.5, 0, 1)
	p.DrawVertices([]render.Point{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 10, Y: 20}},
		nil, render.VertexModeTriangles)
	p.Pop()
}

func diamondPath() *render.Path {
	path := render.NewPath()
	path.MoveTo(0, -10)
	path.LineTo(10, 0)
	path.LineTo(0, 10)
	path.LineTo(-10, 0)
	path.Close()
	return path
}

// parityImage is a 16x16 buffer with a hard left/right split so a misplaced
// source rectangle or a lost pixel copy shows up as a colour difference.
func parityImage() *render.ImageBuf {
	buf, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		panic(err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			col := render.RGBA{R: 0, G: 0, B: 1, A: 1}
			if x >= 8 {
				col = render.RGBA{R: 1, G: 0, B: 0, A: 1}
			}
			buf.SetRGBA(x, y, byte(col.R*255), byte(col.G*255), byte(col.B*255), 255)
		}
	}
	return buf
}

func TestPlaybackMatchesLiveContextPaint(t *testing.T) {
	const w, h = 200, 130

	live := render.NewContext(w, h)
	paintParity(live)
	liveImg, ok := live.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("live image type = %T, want *image.RGBA", live.Image())
	}

	rec := recording.NewRecorder(w, h)
	paintParity(rec)
	played := playbackRGBA(t, rec.FinishRecording())

	if diff := countPixelDiff(liveImg, played); diff != 0 {
		t.Errorf("playback differs from live painting in %d of %d pixels", diff, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if liveImg.RGBAAt(x, y) != played.RGBAAt(x, y) {
					t.Errorf("first diff at (%d,%d): live=%v replay=%v", x, y,
						liveImg.RGBAAt(x, y), played.RGBAAt(x, y))
					return
				}
			}
		}
	}
}

func countPixelDiff(a, b *image.RGBA) int {
	diff := 0
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				diff++
			}
		}
	}
	return diff
}
