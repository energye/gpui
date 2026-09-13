package icon_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: check icon fills a 48×48 canvas (explicit black on white).
//
// Three evidences: logic probe (Known + 48 layout), pixel assertion
// (stroke pixel dark, corner white), golden file compare with explicit
// tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestIcon_PRD_ICO19(t *testing.T) {
	const canvas, edge = 48.0, 48.0
	ic := icon.NewIcon("check")
	if !ic.Known() {
		t.Fatal("check should be known")
	}
	ic.SetSize(edge)
	ic.SetColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	sz := ic.Layout(rendering.Loose(canvas, canvas))
	if math.Abs(sz.Width-edge) > 0.5 || math.Abs(sz.Height-edge) > 0.5 {
		t.Fatalf("layout=%vx%v want 48", sz.Width, sz.Height)
	}

	dc := render.NewContext(int(canvas), int(canvas))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	off := (canvas - edge) / 2
	ic.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(off, off))
	got := dc.Image()

	// Pixel assertion: midpoint of the check's long stroke is dark,
	// corner stays white. Stroke runs local (21.6,34.6)→(37.4,14.4)
	// at 48px, zero offset → canvas midpoint ≈ (29.5,24.5).
	if r, g, b, _ := got.At(29, 24).RGBA(); r > 0x8000 || g > 0x8000 || b > 0x8000 {
		t.Fatalf("stroke pixel #%04x%04x%04x want dark", r, g, b)
	}
	if r, g, b, _ := got.At(4, 4).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, b)
	}

	path := filepath.Join("testdata", "golden_check.png")
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
	const maxDiff = 12 * 257 // 8-bit units scaled to 16-bit
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
