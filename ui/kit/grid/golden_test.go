package grid_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/grid"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: two span-12 columns paint side by side (gutter 0).
//
// Three evidences: logic probe (600/600 layout at 1200 scaled to 240),
// pixel assertion (left blue, right red), golden compare with tolerance.
func TestGrid_PRD_GRD20(t *testing.T) {
	const W, H = 240.0, 40.0
	left := rendering.NewRenderColorBox(120, 40, 0.09, 0.47, 1, 1)
	right := rendering.NewRenderColorBox(120, 40, 1, 0.3, 0.2, 1)
	a := grid.NewCol(left)
	b := grid.NewCol(right)
	a.SetSpan(12)
	b.SetSpan(12)
	r := grid.NewRow(a, b)
	sz := r.Layout(rendering.Tight(W, H))
	if math.Abs(sz.Width-W) > 0.5 || math.Abs(sz.Height-H) > 0.5 {
		t.Fatalf("layout=%vx%v want %vx%v", sz.Width, sz.Height, W, H)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	r.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(0, 0))
	got := dc.Image()

	if rr, gg, bb, _ := got.At(60, 20).RGBA(); rr > 0x4000 || gg > 0x9000 || bb < 0xC000 {
		t.Fatalf("left #%04x%04x%04x want blue", rr, gg, bb)
	}
	if rr, gg, bb, _ := got.At(180, 20).RGBA(); rr < 0xC000 || gg > 0x7000 || bb > 0x7000 {
		t.Fatalf("right #%04x%04x%04x want red", rr, gg, bb)
	}

	path := filepath.Join("testdata", "golden_grid.png")
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
		t.Fatalf("bad pixels %d/%d exceed %.1f%%", bad, total, badFrac*100)
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
