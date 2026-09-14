package masonry_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/masonry"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: two-box masonry with small gap on a white canvas.
//
// Three evidences: logic probe (column count + item boxes), pixel assertion
// (box centers colored, gap + corner white), golden file compare with
// explicit tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestMasonry_PRD_MAS17_GoldenBasic(t *testing.T) {
	const canvas = 120.0
	m := masonry.NewMasonry(
		masonry.MasonryItem{Key: "g1", Column: -1, Height: 40, Content: rendering.NewRenderColorBox(40, 40, 0.9, 0.2, 0.2, 1)},
		masonry.MasonryItem{Key: "g2", Column: -1, Height: 40, Content: rendering.NewRenderColorBox(40, 40, 0.2, 0.4, 0.9, 1)},
	)
	m.SetColumns(2)
	m.SetGutter(8, 8)
	if m.EffectiveColumnCount() != 2 {
		t.Fatalf("columns=%d want 2", m.EffectiveColumnCount())
	}
	sz := m.Layout(rendering.Loose(88, canvas))
	if sz.Width != 88 || sz.Height != 40 {
		t.Fatalf("layout=%v want 88x40", sz)
	}
	b0 := m.ItemBox("g1")
	b1 := m.ItemBox("g2")
	if b0.Min.X != 0 || b1.Min.X != 48 {
		t.Fatalf("offsets %v %v want 0,48", b0.Min.X, b1.Min.X)
	}

	dc := render.NewContext(int(canvas), int(canvas))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	m.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(10, 10))
	got := dc.Image()

	if r, g, b, _ := got.At(30, 30).RGBA(); r < 0x8000 || g > 0x8000 || b > 0x8000 {
		t.Fatalf("first box #%04x%04x%04x want red", r, g, b)
	}
	if r, g, b, _ := got.At(54, 30).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("gap #%04x%04x%04x want white", r, g, b)
	}
	if r, g, b, _ := got.At(78, 30).RGBA(); b < 0x8000 || r > 0x8000 {
		t.Fatalf("second box #%04x%04x%04x want blue", r, g, b)
	}
	if r, g, b, _ := got.At(4, 4).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, b)
	}

	path := filepath.Join("testdata", "golden_masonry_basic.png")
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
