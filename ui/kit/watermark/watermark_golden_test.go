package watermark_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: watermark tile block fills a 96×64 viewport (explicit white
// bg, dark single-row text, rotate=0, gap=8 for dense deterministic pixels).
//
// Real glyphs only: face via SetFace, DrawString via Abs. Without a face the
// mark stays empty (never a bar). Needs a system face, otherwise Skip.
//
// Three evidences: logic probe (HasMark + mark size + Layout non-zero),
// pixel assertion (tile zone carries dark glyph ink, far corner stays white),
// golden file compare with explicit tolerance (AA edges only).
// Regenerate with UPDATE_GOLDEN=1.
func TestWatermark_PRD_WMGolden(t *testing.T) {
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(16)
	if err != nil || face == nil {
		t.Skipf("golden needs a system face for real glyphs: %v", err)
	}
	t.Logf("golden face: %s", desc)
	const vw, vh = 96.0, 64.0
	host := rendering.NewRenderColorBox(vw, vh, 1, 1, 1, 1)
	wm := watermark.NewWatermark(host)
	wm.SetContent("WM")
	wm.SetRotate(0)
	wm.SetGap(8, 8)
	wm.SetOffset(0, 0)
	wm.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	wm.SetFace(face)
	if !wm.HasMark() {
		t.Fatal("golden watermark should mark")
	}
	mw, mh := wm.ResolvedMarkSize()
	if mw <= 0 || mh <= 0 {
		t.Fatalf("mark=%vx%v", mw, mh)
	}
	if wm.Node() == nil {
		t.Fatal("Node must be non-nil")
	}
	sz := wm.Layout(rendering.Tight(vw, vh))
	if math.Abs(sz.Width-vw) > 0.5 || math.Abs(sz.Height-vh) > 0.5 {
		t.Fatalf("layout=%vx%v want %vx%v", sz.Width, sz.Height, vw, vh)
	}
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout must be non-zero, got %vx%v", sz.Width, sz.Height)
	}

	dc := render.NewContext(int(vw), int(vh))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	wm.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()

	// Pixel assertion: first tile zone carries dark glyph ink (real text).
	dark := 0
	for y := 0; y < 24; y++ {
		for x := 0; x < 30; x++ {
			r, g, b, _ := got.At(x, y).RGBA()
			if r/257 < 110 && g/257 < 110 && b/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("tile zone dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}
	// Horizontal gap band y=50..52 sits between tile rows (28..48, 56..76).
	white := 0
	for y := 50; y < 52; y++ {
		for x := 0; x < int(vw); x++ {
			r, g, b, _ := got.At(x, y).RGBA()
			if r/257 > 240 && g/257 > 240 && b/257 > 240 {
				white++
			}
		}
	}
	if white < 20 {
		t.Fatalf("gap band white pixels=%d want >=20", white)
	}

	path := filepath.Join("testdata", "golden_mark.png")
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
	const maxDiff = 12 * 257
	const hardCap = 64 * 257
	const badFrac = 0.005
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for y := 0; y < got.Bounds().Dy(); y++ {
		for x := 0; x < got.Bounds().Dx(); x++ {
			r1, g1, b1, a1 := got.At(x, y).RGBA()
			r2, g2, b2, a2 := want.At(x, y).RGBA()
			m := max4(wmDiff(r1, r2), wmDiff(g1, g2), wmDiff(b1, b2), wmDiff(a1, a2))
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

func wmDiff(a, b uint32) uint32 {
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
