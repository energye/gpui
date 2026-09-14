package divider_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/divider"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: horizontal solid divider fills a 200×49 canvas.
//
// Three evidences: logic probe (MarginBlock + 200 layout), pixel assertion
// (rail pixel dark, margin white), golden file compare with explicit
// tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestDivider_PRD_DIV02_GoldenHorizontal(t *testing.T) {
	const w, h = 200.0, 49.0
	d := divider.NewDivider()
	sz := d.Layout(rendering.Tight(w, h))
	if math.Abs(sz.Width-w) > 0.5 || math.Abs(sz.Height-h) > 0.5 {
		t.Fatalf("layout=%v want %vx%v", sz, w, h)
	}
	if math.Abs(d.MarginBlock()-24) > 0.5 {
		t.Fatalf("margin=%v want 24", d.MarginBlock())
	}

	dc := render.NewContext(int(w), int(h))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	d.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(0, 0))
	got := dc.Image()

	// Pixel assertion: middle of the rail carries the theme line color
	// (#f0f0f0, light but not pure white), top margin stays white.
	// Rail sits at y=h/2=24.
	lc := d.LineColor()
	_ = lc
	if r, g, b, _ := got.At(100, 24).RGBA(); r == 0xFFFF && g == 0xFFFF && b == 0xFFFF {
		t.Fatalf("rail pixel #%04x%04x%04x want line color, not pure white", r, g, b)
	}
	if r, g, b, _ := got.At(100, 4).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("margin #%04x%04x%04x want white", r, g, b)
	}

	path := filepath.Join("testdata", "golden_divider_horizontal.png")
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
