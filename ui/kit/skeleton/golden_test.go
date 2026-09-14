package skeleton_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/skeleton"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: basic skeleton (title + 3 rows, last 61%) on white canvas.
//
// Three evidences: logic probe (title 38% + last-row 61% + 200x112 layout),
// pixel assertion (title gray, gap white, last-row tail white), golden file
// compare with explicit tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestSkeleton_PRD_SKL19_GoldenBasic(t *testing.T) {
	const canvasW, canvasH = 220, 140
	s := skeleton.NewSkeleton()
	sz := s.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-200) > 0.5 || math.Abs(sz.Height-112) > 0.5 {
		t.Fatalf("layout=%v want 200x112", sz)
	}
	if got := s.EffectiveTitleWidth(200); math.Abs(got-200*0.38) > 1.0 {
		t.Fatalf("title=%v want 76", got)
	}
	ws := s.EffectiveParagraphWidths(200)
	if len(ws) != 3 || math.Abs(ws[2]-200*0.61) > 1.0 {
		t.Fatalf("rows=%v want last 122", ws)
	}

	dc := render.NewContext(canvasW, canvasH)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(10, 10))
	got := dc.Image()

	// Pixel assertion: title inside gray, gap white, last-row tail white.
	if r, g, b, _ := got.At(20, 18).RGBA(); r > 0xF800 || g > 0xF800 || b > 0xF800 {
		t.Fatalf("title #%04x%04x%04x want gray", r, g, b)
	}
	if r, g, b, _ := got.At(20, 34).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("gap #%04x%04x%04x want white", r, g, b)
	}
	if r, g, b, _ := got.At(20, 116).RGBA(); r > 0xF800 || g > 0xF800 || b > 0xF800 {
		t.Fatalf("last row #%04x%04x%04x want gray", r, g, b)
	}
	if r, g, b, _ := got.At(170, 116).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("last tail #%04x%04x%04x want white", r, g, b)
	}

	path := filepath.Join("testdata", "golden_skeleton_basic.png")
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
