package spin_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/spin"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: medium 4-dot looper on a 64×64 canvas (default theme).
//
// Three evidences: logic probe (display + 32 layout), pixel assertion
// (east dot carries primary blue, corner stays white), golden file compare
// with explicit tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestSpin_PRD_SPN21_Golden(t *testing.T) {
	const canvas, edge = 64.0, 32.0
	s := spin.NewSpin(nil)
	s.SetSize(spin.SpinLarge)
	if !s.IsDisplaySpinning() {
		t.Fatal("golden spin must display")
	}
	sz := s.Layout(rendering.Loose(canvas, canvas))
	if math.Abs(sz.Width-edge) > 0.5 || math.Abs(sz.Height-edge) > 0.5 {
		t.Fatalf("layout=%vx%v want 32", sz.Width, sz.Height)
	}

	dc := render.NewContext(int(canvas), int(canvas))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	off := (canvas - edge) / 2
	s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(off, off))
	got := dc.Image()

	// Pixel assertion: east dot center carries primary blue (#1677ff);
	// orbit=0.32*32≈10.2, center=(16,16) local → canvas ≈ (42,32).
	// Corner stays white.
	if r, g, b, _ := got.At(42, 32).RGBA(); b < 0x8000 || r > 0x8000 {
		t.Fatalf("dot pixel #%04x%04x%04x want primary blue", r, g, b)
	}
	if r, g, b, _ := got.At(4, 4).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, b)
	}

	path := filepath.Join("testdata", "golden_spin.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write golden: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode golden: %v", err)
		}
		f.Close()
		t.Logf("golden rewritten: %s", path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("bounds %v want %v", got.Bounds(), want.Bounds())
	}
	// Tolerance: AA edges may differ up to 12/255 per channel;
	// at most 0.5% of pixels may exceed it, none may exceed 64.
	const maxDiff = 12 * 257
	const hardCap = 64 * 257
	const badFrac = 0.005
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for y := 0; y < got.Bounds().Dy(); y++ {
		for x := 0; x < got.Bounds().Dx(); x++ {
			r1, g1, b1, a1 := got.At(x, y).RGBA()
			r2, g2, b2, a2 := want.At(x, y).RGBA()
			m := max4(diff(r1, r2), diff(g1, g2), diff(b1, b2), diff(a1, a2))
			if m > hardCap {
				t.Fatalf("pixel (%d,%d) diff %d exceeds hard cap", x, y, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > badFrac {
		t.Fatalf("bad pixels %d/%d exceed %.1f%% (tolerance maxDiff=%d)", bad, total, badFrac*100, maxDiff/257)
	}
}

func diff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
