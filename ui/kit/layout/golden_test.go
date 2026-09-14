package layout_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: sider+content row paints dark shell left, body right.
//
// Three evidences: logic probe (200/200 split), pixel assertion
// (sider dark, content light), golden file compare with explicit
// tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestLayout_PRD_LAY21(t *testing.T) {
	const W, H = 400.0, 300.0
	s := layout.NewSider()
	s.SetCollapsible(true)
	c := layout.NewContent()
	l := layout.NewLayout(s, c)
	sz := l.Layout(rendering.Tight(W, H))
	if math.Abs(sz.Width-W) > 0.5 || math.Abs(sz.Height-H) > 0.5 {
		t.Fatalf("layout=%v want %vx%v", sz, W, H)
	}
	sx, _, sw, _ := absRect(s.Node())
	cx, _, cw, _ := absRect(c.Node())
	if math.Abs(sw-200) > 0.5 || math.Abs(cw-200) > 0.5 {
		t.Fatalf("split sider=%v content=%v want 200/200 (sx=%v cx=%v)", sw, cw, sx, cx)
	}

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	l.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()

	// Pixel assertion: sider interior is dark shell #001529, content is light body.
	if r, g, b, _ := got.At(100, 150).RGBA(); r > 0x2000 || g > 0x3000 || b > 0x5000 {
		t.Fatalf("sider pixel #%04x%04x%04x want dark #001529", r, g, b)
	}
	if r, g, b, _ := got.At(300, 150).RGBA(); r < 0xE000 || g < 0xE000 || b < 0xE000 {
		t.Fatalf("content #%04x%04x%04x want light body", r, g, b)
	}

	path := filepath.Join("testdata", "golden_row.png")
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
