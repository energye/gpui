package border_beam_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	borderbeam "github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: red beam on a 120x80 container, phase pinned at 0.15.
//
// Three evidences: logic probe (visible + host layout), pixel assertion
// (beam pixel red, interior white), golden file compare with explicit
// tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestBorderBeam_GoldenBeam(t *testing.T) {
	const canvasW, canvasH = 160.0, 120.0
	b := borderbeam.NewBorderBeam(rendering.NewRenderColorBox(120, 80, 1, 1, 1, 1))
	b.SetColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
	b.SetSize(40)
	b.SetLineWidth(4)
	b.SetBorderRadius(8)
	b.SetDuration(6)
	if !b.IsBeamVisible() {
		t.Fatal("beam should be visible")
	}
	b.Tick(0.9)
	if math.Abs(b.Phase()-0.15) > 1e-9 {
		t.Fatalf("phase=%v want 0.15", b.Phase())
	}
	sz := b.Layout(rendering.Loose(canvasW, canvasH))
	if math.Abs(sz.Width-120) > 0.5 || math.Abs(sz.Height-80) > 0.5 {
		t.Fatalf("layout=%vx%v want 120x80", sz.Width, sz.Height)
	}

	dc := render.NewContext(int(canvasW), int(canvasH))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	offX, offY := (canvasW-120)/2, (canvasH-80)/2
	b.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(offX, offY))
	got := dc.Image()

	// Pixel assertion: phase 0.15 lands the 40px segment on the top edge
	// (local x in [66,106], y=0) -> canvas (106,20) is solid red.
	if r, g, bl, _ := got.At(106, 20).RGBA(); r < 0x8000 || g > 0x8000 || bl > 0x8000 {
		t.Fatalf("beam pixel #%04x%04x%04x want red", r, g, bl)
	}
	// Interior stays container white.
	if r, g, bl, _ := got.At(80, 60).RGBA(); r < 0xE000 || g < 0xE000 || bl < 0xE000 {
		t.Fatalf("interior #%04x%04x%04x want white", r, g, bl)
	}

	path := filepath.Join("testdata", "golden_beam.png")
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
