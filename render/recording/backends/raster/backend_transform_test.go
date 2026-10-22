package raster

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// TestPlaybackTransformAppliesOnce pins the display-list coordinate space:
// recorded geometry is user-space and the recorded CTM is applied exactly once,
// at playback (SkPicture semantics). A recorder that bakes the CTM into the
// coordinates *and* emits ConcatCommand lands every transformed draw
// twice-translated.
func TestPlaybackTransformAppliesOnce(t *testing.T) {
	rec := recording.NewRecorder(120, 120)
	rec.Push()
	rec.Translate(30, 0)
	rec.SetFillRGBA(1, 0, 0, 1)
	rec.DrawRectangle(0, 0, 20, 20)
	rec.Fill()
	rec.Pop()

	img := playbackRGBA(t, rec.FinishRecording())
	if !pixelIs(img, 40, 10, render.RGBA{R: 1, A: 1}, 20) {
		t.Fatalf("x=40 (one translation) = %v, want red", img.RGBAAt(40, 10))
	}
	if !pixelIs(img, 10, 10, render.RGBA{}, 2) {
		t.Fatalf("x=10 (untouched by the translate) = %v, want transparent", img.RGBAAt(10, 10))
	}
	if !pixelIs(img, 70, 10, render.RGBA{}, 2) {
		t.Fatalf("x=70 (translation applied twice) = %v, want transparent", img.RGBAAt(70, 10))
	}
}

// TestPlaybackPathTransformAppliesOnce is the path-building counterpart:
// MoveTo/LineTo store user-space vertices and the translation travels as a
// ConcatCommand, so a transformed path lands translated exactly once.
func TestPlaybackPathTransformAppliesOnce(t *testing.T) {
	rec := recording.NewRecorder(120, 120)
	rec.Push()
	rec.Translate(30, 0)
	rec.SetStrokeRGBA(1, 0, 0, 1)
	rec.SetLineWidth(4)
	rec.MoveTo(10, 10)
	rec.LineTo(10, 50)
	rec.Stroke()
	rec.Pop()

	img := playbackRGBA(t, rec.FinishRecording())
	if !pixelIs(img, 40, 30, render.RGBA{R: 1, A: 1}, 20) {
		t.Fatalf("x=40 (user x=10 + one translation) = %v, want red", img.RGBAAt(40, 30))
	}
	if !pixelIs(img, 70, 30, render.RGBA{}, 2) {
		t.Fatalf("x=70 (translation applied twice) = %v, want transparent", img.RGBAAt(70, 30))
	}
}
