package render

import (
	"image"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render/text"
)

func TestTrackDamage_HiDPI_ScalesToPhysical(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(2.0))
	defer dc.Close()

	dc.DrawRectangle(0, 0, 400, 300)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}

	r := rects[0]
	if r.Dx() != 800 || r.Dy() != 600 {
		t.Errorf("damage rect = %v; want (0,0)-(800,600) physical", r)
	}
}

func TestTrackDamage_Scale1_NoScaling(t *testing.T) {
	dc := NewContext(400, 300)
	defer dc.Close()

	dc.DrawRectangle(0, 0, 400, 300)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}

	r := rects[0]
	if r.Dx() != 400 || r.Dy() != 300 {
		t.Errorf("damage rect = %v; want (0,0)-(400,300)", r)
	}
}

func TestTrackDamage_HiDPI_PartialRect(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(2.0))
	defer dc.Close()

	dc.DrawRectangle(100, 50, 200, 150)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}

	r := rects[0]
	want := image.Rect(200, 100, 600, 400)
	if r != want {
		t.Errorf("damage rect = %v; want %v (physical = logical × 2)", r, want)
	}
}

func TestTrackDamageRect_HiDPI_PublicAPI(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(2.0))
	defer dc.Close()

	dc.TrackDamageRect(image.Rect(10, 20, 100, 80))

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}

	r := rects[0]
	want := image.Rect(20, 40, 200, 160)
	if r != want {
		t.Errorf("TrackDamageRect = %v; want %v (physical = logical × 2)", r, want)
	}
}

func TestTrackDamage_Scale3(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(3.0))
	defer dc.Close()

	dc.DrawRectangle(0, 0, 100, 100)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}

	r := rects[0]
	if r.Dx() != 300 || r.Dy() != 300 {
		t.Errorf("damage rect = %v; want 300×300 (physical = 100 × 3)", r)
	}
}

func TestTrackDamage_HiDPI_Stroke(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(2.0))
	defer dc.Close()

	dc.SetLineWidth(2)
	dc.DrawLine(10, 10, 200, 150)
	dc.Stroke()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects from Stroke")
	}

	r := rects[0]
	if r.Min.X < 18 || r.Min.Y < 18 || r.Max.X > 402 || r.Max.Y > 302 {
		t.Logf("stroke damage rect = %v (physical)", r)
	}
	if r.Dx() < 2 || r.Dy() < 2 {
		t.Errorf("stroke damage rect too small: %v", r)
	}
}

func TestTrackDamage_FractionalScale(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(1.5))
	defer dc.Close()

	dc.DrawRectangle(0, 0, 100, 100)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}

	r := rects[0]
	if r.Dx() != 150 || r.Dy() != 150 {
		t.Errorf("damage rect = %v; want 150×150 (physical = 100 × 1.5)", r)
	}
}

func TestTrackDamage_FractionalCoords(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(2.0))
	defer dc.Close()

	dc.DrawCircle(50.5, 75.5, 30)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects from circle")
	}

	r := rects[0]
	// Circle at (50.5, 75.5) r=30 → bounds ~(20.5, 45.5)-(80.5, 105.5)
	// Scaled ×2 with Floor/Ceil → (41, 91)-(161, 211)
	if r.Min.X > 42 || r.Min.Y > 92 {
		t.Errorf("damage rect Min too large: %v", r)
	}
	if r.Max.X < 160 || r.Max.Y < 210 {
		t.Errorf("damage rect Max too small: %v", r)
	}
}

func TestTrackDamage_MultipleRectsScaled(t *testing.T) {
	dc := NewContext(400, 300, WithDeviceScale(2.0))
	defer dc.Close()

	dc.DrawRectangle(0, 0, 50, 50)
	dc.Fill()
	dc.DrawRectangle(200, 200, 50, 50)
	dc.Fill()

	rects := dc.FrameDamage()
	if len(rects) < 2 {
		t.Fatalf("expected ≥2 damage rects, got %d", len(rects))
	}

	r0 := rects[0]
	r1 := rects[1]

	if r0 != image.Rect(0, 0, 100, 100) {
		t.Errorf("rect[0] = %v; want (0,0)-(100,100)", r0)
	}
	if r1 != image.Rect(400, 400, 500, 500) {
		t.Errorf("rect[1] = %v; want (400,400)-(500,500)", r1)
	}
}

// Text damage must land where the glyphs paint under the current CTM. A
// retained recording replays layer-local text under the layer's translate
// (the C11 HUD draws at y=22 below translate(0,728)); damage built from raw
// x/y dirtied the top of the window instead of the band.
func TestTrackDamage_TextAppliesCTM(t *testing.T) {
	src, err := text.NewFontSourceFromFile(filepath.Join("text", "testdata", "ahem.ttf"))
	if err != nil {
		t.Skipf("test font unavailable (%v)", err)
	}
	dc := NewContext(400, 300)
	defer dc.Close()
	dc.SetFont(src.Face(12))
	dc.SetDamageTracking(true)
	dc.ResetFrameDamage()
	dc.Translate(0, 200)
	dc.trackTextDamage("AB", 10, 22)

	rects := dc.FrameDamage()
	if len(rects) != 1 {
		t.Fatalf("damage rects=%v want exactly one", rects)
	}
	r := rects[0]
	// Identity-CTM ink box would be roughly (9, 8)-(35, 25); the translate
	// must move it down by 200, never leave any part at the top.
	if r.Min.Y < 200 || r.Max.Y > 230 {
		t.Fatalf("text damage y=%d..%d want inside 200..230 (CTM applied once)", r.Min.Y, r.Max.Y)
	}
	if r.Min.X < 9 || r.Max.X > 40 {
		t.Fatalf("text damage x=%d..%d want the ink run at x≈10", r.Min.X, r.Max.X)
	}
}

// A stroked hairline's damage is recorded from the path-space outline (the
// user CTM is already baked in). Inverse-transforming it applied the
// translate twice, so a bottom HUD hairline at y=728 dirtied y=0 and the
// damage union spanned the whole window.
func TestTrackDamage_StrokeOutlineUsesPathSpace(t *testing.T) {
	dc := NewContext(400, 300)
	defer dc.Close()
	dc.SetDamageTracking(true)
	dc.ResetFrameDamage()
	dc.Translate(0, 280)
	dc.SetLineWidth(1)
	dc.DrawLine(0, 0.5, 200, 0.5)
	_ = dc.Stroke()

	damage := dc.FrameDamageUnion()
	if damage.Min.Y < 279 || damage.Max.Y > 282 {
		t.Fatalf("stroke damage y=%d..%d want the hairline at 280 (CTM applied once)", damage.Min.Y, damage.Max.Y)
	}
	if damage.Max.X < 200 {
		t.Fatalf("stroke damage x ends at %d, want ≥200", damage.Max.X)
	}
}
