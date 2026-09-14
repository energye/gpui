package splitter_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: horizontal two-panel splitter on a 120×48 canvas (SPL-21).
//
// Three evidences: logic probe (60/60 layout + bar 6 box), pixel assertion
// (left red, right blue, bar track neutral), golden file compare with
// explicit tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestSplitter_PRD_SPL21_Golden(t *testing.T) {
	const W, H = 120.0, 48.0
	lp := splitter.NewSplitterPanel(rendering.NewRenderColorBox(10, 10, 0.9, 0.2, 0.2, 1))
	rp := splitter.NewSplitterPanel(rendering.NewRenderColorBox(10, 10, 0.2, 0.4, 0.9, 1))
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(W)
	s.SetHeight(H)
	sz := s.Layout(rendering.Tight(W, H))
	if math.Abs(sz.Width-W) > 0.5 || math.Abs(sz.Height-H) > 0.5 {
		t.Fatalf("layout=%v want 120x48", sz)
	}
	sizes := s.PanelSizes()
	if math.Abs(sizes[0]-60) > 0.5 || math.Abs(sizes[1]-60) > 0.5 {
		t.Fatalf("sizes=%v want 60/60", sizes)
	}
	bar := s.BarNode(0)
	if bar == nil {
		t.Fatal("bar nil")
	}
	if math.Abs(bar.Size().Width-6) > 0.5 {
		t.Fatalf("bar w=%v want 6", bar.Size())
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()

	// Left panel red, right panel blue, bar seam neutral gray.
	if r, g, b, _ := got.At(10, 24).RGBA(); r < 0xC000 || g > 0x8000 || b > 0x8000 {
		t.Fatalf("left #%04x%04x%04x want red", r, g, b)
	}
	if r, g, b, _ := got.At(110, 24).RGBA(); b < 0x8000 || r > 0x8000 {
		t.Fatalf("right #%04x%04x%04x want blue", r, g, b)
	}
	if r, g, b, _ := got.At(60, 24).RGBA(); r < 0x8000 || g < 0x8000 || b < 0x8000 {
		t.Fatalf("bar #%04x%04x%04x want neutral track", r, g, b)
	}

	path := filepath.Join("testdata", "golden_splitter_bar.png")
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
