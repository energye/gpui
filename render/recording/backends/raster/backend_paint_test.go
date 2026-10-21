package raster

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

func loadTestImage(t *testing.T) image.Image {
	t.Helper()
	path := filepath.Join("testdata", "quadrants.png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("missing %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

func playbackRGBA(t *testing.T, r *recording.Recording) *image.RGBA {
	t.Helper()
	backend := NewBackend()
	if err := r.Playback(backend); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	img, ok := backend.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("Image type = %T, want *image.RGBA", backend.Image())
	}
	return img
}

func pixelIs(img *image.RGBA, x, y int, want render.RGBA, tol uint8) bool {
	c := img.RGBAAt(x, y)
	return absDiff(c.R, uint8(want.R*255)) <= tol &&
		absDiff(c.G, uint8(want.G*255)) <= tol &&
		absDiff(c.B, uint8(want.B*255)) <= tol
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestPlaybackStrokeRect pins the previously missing StrokeRectCommand case.
func TestPlaybackStrokeRect(t *testing.T) {
	rec := recording.NewRecorder(80, 80)
	rec.SetStrokeRGBA(1, 0, 0, 1)
	rec.SetLineWidth(6)
	rec.StrokeRectangle(20, 20, 40, 40)
	img := playbackRGBA(t, rec.FinishRecording())

	if !pixelIs(img, 20, 40, render.RGBA{R: 1, A: 1}, 20) {
		t.Fatalf("left edge = %v, want red", img.RGBAAt(20, 40))
	}
	if !pixelIs(img, 40, 40, render.RGBA{}, 2) {
		t.Fatalf("inside = %v, want transparent", img.RGBAAt(40, 40))
	}
}

// TestPlaybackClipAndReset pins SetClip/ClearClip reaching the raster context.
func TestPlaybackClipAndReset(t *testing.T) {
	rec := recording.NewRecorder(100, 100)
	rec.SetFillRGBA(1, 0, 0, 1)
	rec.DrawRectangle(20, 20, 40, 40)
	rec.Clip()
	rec.DrawRectangle(0, 0, 100, 100)
	rec.Fill()
	rec.ResetClip()
	rec.SetFillRGBA(0, 1, 0, 1)
	rec.DrawRectangle(70, 70, 20, 20)
	rec.Fill()

	img := playbackRGBA(t, rec.FinishRecording())
	if !pixelIs(img, 40, 40, render.RGBA{R: 1, A: 1}, 20) {
		t.Fatalf("inside clip = %v, want red", img.RGBAAt(40, 40))
	}
	if !pixelIs(img, 10, 10, render.RGBA{}, 2) {
		t.Fatalf("outside clip = %v, want transparent", img.RGBAAt(10, 10))
	}
	if !pixelIs(img, 75, 75, render.RGBA{G: 1, A: 1}, 20) {
		t.Fatalf("after ResetClip = %v, want green", img.RGBAAt(75, 75))
	}
}

// TestPlaybackSaveRestoreClip verifies the raster backend honors Save/Restore
// around clip state.
func TestPlaybackSaveRestoreClip(t *testing.T) {
	rec := recording.NewRecorder(100, 100)
	rec.Push()
	rec.DrawRectangle(10, 10, 20, 20)
	rec.Clip()
	rec.SetFillRGBA(1, 0, 0, 1)
	rec.DrawRectangle(0, 0, 100, 100)
	rec.Fill()
	rec.Pop()
	rec.SetFillRGBA(0, 1, 0, 1)
	rec.DrawRectangle(70, 70, 20, 20)
	rec.Fill()

	img := playbackRGBA(t, rec.FinishRecording())
	if !pixelIs(img, 15, 15, render.RGBA{R: 1, A: 1}, 20) {
		t.Fatalf("inside saved clip = %v, want red", img.RGBAAt(15, 15))
	}
	if !pixelIs(img, 50, 50, render.RGBA{}, 2) {
		t.Fatalf("outside saved clip = %v, want transparent", img.RGBAAt(50, 50))
	}
	if !pixelIs(img, 75, 75, render.RGBA{G: 1, A: 1}, 20) {
		t.Fatalf("outside restored clip = %v, want green", img.RGBAAt(75, 75))
	}
}

// TestPlaybackDrawImage pins source-aware image playback. The old backend only
// emitted an unfilled rectangle, so every sample stayed transparent.
func TestPlaybackDrawImage(t *testing.T) {
	img := loadTestImage(t)
	rec := recording.NewRecorder(80, 80)
	rec.DrawImageScaled(img, 10, 10, 32, 32)
	out := playbackRGBA(t, rec.FinishRecording())

	if !pixelIs(out, 14, 14, render.RGBA{R: 1, A: 1}, 24) {
		t.Fatalf("top-left = %v, want red", out.RGBAAt(14, 14))
	}
	if !pixelIs(out, 26, 14, render.RGBA{G: 1, A: 1}, 24) {
		t.Fatalf("top-right = %v, want green", out.RGBAAt(26, 14))
	}
	if !pixelIs(out, 14, 26, render.RGBA{B: 1, A: 1}, 24) {
		t.Fatalf("bottom-left = %v, want blue", out.RGBAAt(14, 26))
	}
	if !pixelIs(out, 26, 26, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 24) {
		t.Fatalf("bottom-right = %v, want white", out.RGBAAt(26, 26))
	}
}

// TestPlaybackDrawImageSubImage pins source coordinates against a non-zero
// image origin. ImageBufFromImage normalizes the buffer to 0,0, so the
// recording's image-relative src rect must not be offset by Bounds().Min.
func TestPlaybackDrawImageSubImage(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 64, 64))
	sub := base.SubImage(image.Rect(10, 20, 42, 52)).(*image.RGBA)
	for y := 20; y < 52; y++ {
		for x := 10; x < 42; x++ {
			c := color.RGBA{A: 255}
			switch {
			case x < 26 && y < 36:
				c.R = 255
			case x >= 26 && y < 36:
				c.G = 255
			case x < 26:
				c.B = 255
			default:
				c.R, c.G, c.B = 255, 255, 255
			}
			sub.SetRGBA(x, y, c)
		}
	}

	rec := recording.NewRecorder(80, 80)
	rec.DrawImageScaled(sub, 10, 10, 32, 32)
	out := playbackRGBA(t, rec.FinishRecording())

	if !pixelIs(out, 14, 14, render.RGBA{R: 1, A: 1}, 24) {
		t.Fatalf("sub-image top-left = %v, want red", out.RGBAAt(14, 14))
	}
	if !pixelIs(out, 26, 14, render.RGBA{G: 1, A: 1}, 24) {
		t.Fatalf("sub-image top-right = %v, want green", out.RGBAAt(26, 14))
	}
	if !pixelIs(out, 14, 26, render.RGBA{B: 1, A: 1}, 24) {
		t.Fatalf("sub-image bottom-left = %v, want blue", out.RGBAAt(14, 26))
	}
	if !pixelIs(out, 26, 26, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 24) {
		t.Fatalf("sub-image bottom-right = %v, want white", out.RGBAAt(26, 26))
	}
}

// TestPlaybackDrawText pins the font face travelling with DrawTextCommand.
// The old Playback passed nil, so raster DrawText was a no-op.
func TestPlaybackDrawText(t *testing.T) {
	fontPath := filepath.Join("..", "..", "..", "text", "testdata", "source-sans", "OTF", "SourceSans3-Regular.otf")
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("missing %s: %v", fontPath, err)
	}
	src, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatalf("load font: %v", err)
	}
	face := src.Face(28)
	if face == nil {
		t.Fatal("font face is nil")
	}

	rec := recording.NewRecorder(120, 60)
	rec.SetFont(face)
	rec.SetFillRGBA(1, 1, 1, 1)
	rec.DrawString("AB", 8, 40)
	out := playbackRGBA(t, rec.FinishRecording())

	ink := 0
	for y := 8; y < 52; y++ {
		for x := 4; x < 100; x++ {
			if out.RGBAAt(x, y).A > 0 {
				ink++
			}
		}
	}
	if ink < 30 {
		t.Fatalf("text ink = %d pixels, want >= 30", ink)
	}
}

// TestPlaybackLinearGradient pins gradient brush translation to the raster
// context instead of falling back to black.
func TestPlaybackLinearGradient(t *testing.T) {
	rec := recording.NewRecorder(80, 40)
	grad := recording.NewLinearGradientBrush(0, 0, 80, 0).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue)
	rec.SetFillStyle(grad)
	rec.FillRectangle(0, 0, 80, 40)
	out := playbackRGBA(t, rec.FinishRecording())

	left := out.RGBAAt(6, 20)
	right := out.RGBAAt(74, 20)
	if !(left.R > 150 && left.B < 100) {
		t.Fatalf("left = %v, want red-dominant", left)
	}
	if !(right.B > 150 && right.R < 100) {
		t.Fatalf("right = %v, want blue-dominant", right)
	}
}
