package tag_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/tag"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: filled red tag chrome on a white canvas.
//
// Three evidences: logic probe (red text + borderless chrome), pixel
// assertion (interior tinted, corner white), golden file compare with
// explicit tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestTag_PRD_TAG20_GoldenFilledRed(t *testing.T) {
	const canvas = 120.0
	tg := tag.NewTag("Tag")
	tg.SetColor("red")
	if tg.EffectiveBorderWidth() != 0 {
		t.Fatalf("filled border=%v want 0", tg.EffectiveBorderWidth())
	}
	ch := tg.EffectiveChrome()
	def := tag.NewTag("Tag").EffectiveChrome()
	if ch.Bg == def.Bg {
		t.Fatal("red bg must differ from default")
	}
	sz := tg.Layout(rendering.Loose(canvas, canvas))
	if sz.Width <= 0 || math.Abs(sz.Height-tg.EffectiveHeight()) > 0.5 {
		t.Fatalf("layout=%v height want %v", sz, tg.EffectiveHeight())
	}

	dc := render.NewContext(int(canvas), int(canvas))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tg.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(10, 10))
	got := dc.Image()

	// Pixel assertion: interior carries the red tint (pale pink over white),
	// corner stays white.
	cx, cy := int(10+sz.Width/2), int(10+sz.Height/2)
	if r, g, b, _ := got.At(cx, cy).RGBA(); r < 0xE000 || g > 0xF000 || b > 0xF000 {
		t.Fatalf("interior #%04x%04x%04x want pale red", r, g, b)
	}
	if r, g, b, _ := got.At(4, 4).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, b)
	}

	path := filepath.Join("testdata", "golden_tag_filled_red.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		fh, err := os.Create(path)
		if err != nil {
			t.Fatalf("write golden: %v", err)
		}
		if err := png.Encode(fh, got); err != nil {
			fh.Close()
			t.Fatalf("encode golden: %v", err)
		}
		fh.Close()
		t.Logf("golden rewritten: %s", path)
		return
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(fh)
	fh.Close()
	if err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("bounds %v want %v", got.Bounds(), want.Bounds())
	}
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
