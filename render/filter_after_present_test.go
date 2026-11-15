//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// Regression (O13 发现第五件): CPU Apply* filters after an (offscreen) present
// must operate on the just-presented view content, not a stale pixmap, and the
// filter result must survive the next Image() readback. Before the fix the
// filter ran on the pre-present pixmap and the follow-up view sync overwrote
// the result with the pre-filter view content — two invert rounds read back
// the original image unchanged.
func TestColorFilter_AfterPresentOperatesOnPresentedContent(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 160, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.2, 0.4, 0.8)
	dc.DrawRectangle(30, 30, 60, 40)
	_ = dc.Fill()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("no offscreen")
	}
	defer rel()
	// Offscreen present (nil callback): view content is now ahead of pixmap.
	if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("present: %v", err)
	}
	invert := render.ColorFilterFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
		return 1 - r, 1 - g, 1 - b, a
	})
	// Round 1: invert — the blue block (0.2,0.4,0.8) becomes orange-ish.
	dc.ApplyColorFilter(invert)
	r, g, b, a := p1Sample(dc, 60, 50)
	if a < 10 || !(r > 140 && b < 120) {
		t.Fatalf("round-1 invert missing: rgba=%d,%d,%d,%d (want r>140 b<120)", r, g, b, a)
	}
	// Round 2: invert again — back to the presented blue block. Before the fix
	// the second round inverted the ORIGINAL view content again (sync
	// overwrote round 1), so this readback did not return to blue.
	dc.ApplyColorFilter(invert)
	r2, g2, b2, a2 := p1Sample(dc, 60, 50)
	if a2 < 10 || !(b2 > 140 && r2 < 120) {
		t.Fatalf("round-2 double-invert not restored: rgba=%d,%d,%d,%d (want≈0.2,0.4,0.8)", r2, g2, b2, a2)
	}
}

// Regression: the filter result must also survive a NEW present + Image()
// cycle — the pixmap is the sole truth after Apply*, a stale view must not
// overwrite it on the next readback.
func TestColorFilter_ResultSurvivesNextPresentCycle(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 160, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.85, 0.3, 0.2)
	dc.DrawRectangle(20, 20, 80, 50)
	_ = dc.Fill()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("no offscreen")
	}
	defer rel()
	if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("present: %v", err)
	}
	invert := render.ColorFilterFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
		return 1 - r, 1 - g, 1 - b, a
	})
	dc.ApplyColorFilter(invert)
	// Snapshot the filtered pixmap via Image() (post-fix: filter survives).
	imgA := dc.Image()
	if imgA == nil {
		t.Fatal("Image() nil after filter")
	}
	ar, ag, ab, _ := imgA.At(60, 45).RGBA()
	if ab>>8 < 150 || ar>>8 > 120 {
		t.Fatalf("filtered pixmap missing cyan: rgba=%d,%d,%d", ar>>8, ag>>8, ab>>8)
	}
	// Second present cycle + readback: result must stay the filtered content.
	if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("present 2: %v", err)
	}
	imgB := dc.Image()
	br, bg, bb, _ := imgB.At(60, 45).RGBA()
	if br != ar || bg != ag || bb != ab {
		t.Fatalf("second cycle changed filtered content: was (%d,%d,%d) now (%d,%d,%d)",
			ar>>8, ag>>8, ab>>8, br>>8, bg>>8, bb>>8)
	}
}
