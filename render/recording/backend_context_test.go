package recording

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// redRectAt records a 20x20 red square 30 user units along x.
func redSquare(t *testing.T) *Recording {
	t.Helper()
	rec := NewRecorder(120, 120)
	rec.SetRGBA(1, 0, 0, 1)
	rec.Push()
	rec.Translate(30, 0)
	rec.DrawRectangle(0, 0, 20, 20)
	rec.Fill()
	rec.Pop()
	return rec.FinishRecording()
}

func rgbaAt(img *image.RGBA, x, y int) render.RGBA {
	c := img.RGBAAt(x, y)
	return render.RGBA{
		R: float64(c.R) / 255,
		G: float64(c.G) / 255,
		B: float64(c.B) / 255,
		A: float64(c.A) / 255,
	}
}

func isRed(c render.RGBA) bool {
	return c.R > 0.8 && c.G < 0.3 && c.B < 0.3 && c.A > 0.8
}

// A recording replays on top of the target context's own CTM — that is what
// lets one picture serve a layer at any offset: the raster thread sets the
// layer transform, the display list composes onto it.
func TestContextBackendComposesOntoPlaybackCTM(t *testing.T) {
	dc := render.NewContext(120, 120)
	base := render.Translate(0, 60).Multiply(render.Scale(2, 2))
	dc.SetTransform(base)

	if err := redSquare(t).Playback(NewContextBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}

	img, ok := dc.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("image type = %T, want *image.RGBA", dc.Image())
	}

	// User square 30..50 x 0..20 under the base lands at 60..100 x 60..100.
	if c := rgbaAt(img, 80, 80); !isRed(c) {
		t.Errorf("inside the composed square (80,80) = %v, want red", c)
	}
	// Where it would land with no base transform must stay untouched.
	if c := rgbaAt(img, 40, 10); c.A > 0.01 {
		t.Errorf("pre-base position (40,10) = %v, want untouched", c)
	}
	// Where it would land if the base were clobbered instead of composed.
	if c := rgbaAt(img, 40, 80); c.A > 0.01 {
		t.Errorf("base-ignored position (40,80) = %v, want untouched", c)
	}

	if got := dc.GetTransform(); got != base {
		t.Errorf("CTM after playback = %+v, want the base matrix %+v (playback must not leak state)", got, base)
	}
}

// The point of user-space geometry: the same recording rasterizes at the
// device resolution instead of being upscaled, so HiDPI edges land on device
// pixels rather than smearing across two.
func TestContextBackendReplaysAtDeviceScale(t *testing.T) {
	dc := render.NewContext(120, 120)
	dc.SetDeviceScale(2)

	if err := redSquare(t).Playback(NewContextBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}

	img, ok := dc.Image().(*image.RGBA)
	if !ok {
		t.Fatalf("image type = %T, want *image.RGBA", dc.Image())
	}
	if bounds := img.Bounds(); bounds.Dx() != 240 || bounds.Dy() != 240 {
		t.Fatalf("pixmap = %v, want 240x240 physical pixels", bounds.Dx())
	}

	// User square 30..50 maps to device 60..100: the left edge must be crisp.
	if c := rgbaAt(img, 59, 20); c.A > 0.01 {
		t.Errorf("just left of the device edge (59,20) = %v, want clean (no upscale smear)", c)
	}
	if c := rgbaAt(img, 60, 20); !isRed(c) {
		t.Errorf("first device column (60,20) = %v, want fully red", c)
	}
	if c := rgbaAt(img, 99, 20); !isRed(c) {
		t.Errorf("last device column (99,20) = %v, want fully red", c)
	}
	if c := rgbaAt(img, 100, 20); c.A > 0.01 {
		t.Errorf("just past the right edge (100,20) = %v, want clean", c)
	}
}

// A recording must not decide what the context paints afterwards: the brushes
// it used are restored at End, since Context.Pop does not track them.
func TestContextBackendRestoresBrushes(t *testing.T) {
	dc := render.NewContext(60, 60)
	beforeFill := dc.FillBrush()

	if err := redSquare(t).Playback(NewContextBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}

	afterFill := dc.FillBrush()
	if beforeFill == nil || afterFill == nil {
		t.Fatalf("brush getters returned nil: before=%v after=%v", beforeFill, afterFill)
	}
	if beforeFill.ColorAt(0, 0) != afterFill.ColorAt(0, 0) {
		t.Errorf("fill brush after playback = %v, want the pre-playback brush %v",
			afterFill.ColorAt(0, 0), beforeFill.ColorAt(0, 0))
	}
}
