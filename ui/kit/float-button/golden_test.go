package float_button_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	float_button "github.com/energye/gpui/ui/kit/float-button"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: primary circle button on a 64×64 canvas (FB-27).
//
// Three evidences: logic probe (40 layout + primary Token bg), pixel
// assertion (blue disc center, white plus arm, white corner), golden file
// compare with explicit tolerance (AA edges only). Regenerate with
// UPDATE_GOLDEN=1.
func TestFloatButton_PRD_FB27_Golden(t *testing.T) {
	const canvas, edge = 64.0, 40.0
	b := float_button.NewFloatButton()
	b.SetType(float_button.ButtonTypePrimary)
	b.SetShape(float_button.FloatButtonShapeCircle)
	b.SetIcon("plus")
	sz := b.Layout(rendering.Loose(canvas, canvas))
	if math.Abs(sz.Width-edge) > 0.5 || math.Abs(sz.Height-edge) > 0.5 {
		t.Fatalf("layout=%vx%v want 40x40", sz.Width, sz.Height)
	}
	bg := b.EffectiveBackground()
	if bg.B < 0.9 || bg.R > 0.4 {
		t.Fatalf("primary bg=%+v want blue Token", bg)
	}

	dc := render.NewContext(int(canvas), int(canvas))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	off := (canvas - edge) / 2
	b.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(off, off))
	got := dc.Image()

	// Disc probe is off the plus cross (arms cover x=32 / y=32);
	// (26,26) sits on bare primary fill.
	if r, g, bl, _ := got.At(26, 26).RGBA(); bl < 0xC000 || r > 0x8000 || g > 0xB000 {
		t.Fatalf("disc #%04x%04x%04x want primary blue", r, g, bl)
	}
	if r, g, bl, _ := got.At(3, 3).RGBA(); r < 0xE000 || g < 0xE000 || bl < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, bl)
	}
	if r, g, bl, _ := got.At(30, 32).RGBA(); r < 0x9000 || g < 0x9000 || bl < 0x9000 {
		t.Fatalf("plus arm #%04x%04x%04x want pale icon stroke", r, g, bl)
	}

	path := filepath.Join("testdata", "golden_primary.png")
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
