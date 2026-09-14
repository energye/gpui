package alert_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
)

// L3 golden: fixed font-free chrome (scale=1). Three evidences: logic probe
// (type/visible/layout), pixel assertion (shell light, icon saturated,
// corner keeps shell), golden file compare with explicit tolerance.
// Regenerate with UPDATE_GOLDEN=1.
func TestAlert_PRD_ALT20_Golden(t *testing.T) {
	const w, h = 320.0, 96.0
	a := alert.NewAlert("Success")
	a.SetType(alert.AlertSuccess)
	a.SetShowIcon(true)
	a.SetClosable(true)
	if a.Type() != alert.AlertSuccess || !a.Visible() || !a.IconVisible() {
		t.Fatal("logic probe")
	}
	sz := a.Layout(rendering.Tight(w, h))
	if math.Abs(sz.Width-w) > 0.5 || math.Abs(sz.Height-h) > 0.5 {
		t.Fatalf("layout=%v want %dx%d", sz, int(w), int(h))
	}

	dc := render.NewContext(int(w), int(h))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	a.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()

	// Pixel assertion: shell interior is a light success tint (not white,
	// green channel strongest), icon disc is saturated success green.
	r, g, b, _ := got.At(10, 10).RGBA()
	r8, g8, b8 := r/257, g/257, b/257
	if r8 < 200 || g8 < 230 || b8 < 200 {
		t.Fatalf("shell #%02x%02x%02x want light tint", r8, g8, b8)
	}
	// Icon sits left-center: pad 12 + 14/2 ≈ 19, mid height. The disc is
	// success green; probe the left green ring (center holds the white mark).
	ir, ig, ib, _ := got.At(15, int(h/2)).RGBA()
	ir8, ig8, ib8 := ir/257, ig/257, ib/257
	if !(ig8 > ir8+40 && ig8 > ib8+40) {
		t.Fatalf("icon #%02x%02x%02x want saturated green", ir8, ig8, ib8)
	}

	path := filepath.Join("testdata", "golden_success.png")
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

// Second key state: error + description + closable (taller shell).
func TestAlert_Golden_ErrorDesc(t *testing.T) {
	const w, h = 360.0, 140.0
	a := alert.NewAlert("Error title")
	a.SetType(alert.AlertError)
	a.SetDescription("Something went wrong")
	a.SetShowIcon(true)
	a.SetClosable(true)
	sz := a.Layout(rendering.Tight(w, h))
	if sz.Width != w || sz.Height != h {
		t.Fatalf("layout=%v", sz)
	}
	dc := render.NewContext(int(w), int(h))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	a.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()
	// Shell is light red tint (red channel strongest).
	r, g, b, _ := got.At(10, 10).RGBA()
	r8, g8 := r/257, g/257
	if r8 < 220 || r8 < g8 {
		t.Fatalf("error shell #%04x%04x%04x want red tint", r, g, b)
	}
	path := filepath.Join("testdata", "golden_error_desc.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write golden: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode: %v", err)
		}
		f.Close()
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode: %v", err)
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
				t.Fatalf("pixel (%d,%d) diff %d", x, y, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > badFrac {
		t.Fatalf("bad %d/%d", bad, total)
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
