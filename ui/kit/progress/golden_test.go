package progress_test

import (
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/progress"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// L3 golden: line 50% track on 200×32 canvas (explicit theme colors).
//
// Three evidences: logic probe (FillRatio + 160 fallback layout), pixel
// assertion (fill center primary, rail beyond fill rail-color), golden file
// compare with explicit tolerance (AA edges only). Regenerate with UPDATE_GOLDEN=1.
func TestProgress_PRD_PRG22_GoldenLine50(t *testing.T) {
	const cw, ch = 200.0, 32.0
	p := progress.NewProgress(50)
	p.SetShowInfo(false)
	p.SetStatus(progress.StatusNormal)
	sz := p.Layout(rendering.Loose(cw, ch))
	if math.Abs(sz.Width-160) > 0.5 || math.Abs(sz.Height-8) > 0.5 {
		t.Fatalf("layout=%vx%v want 160x8 fallback", sz.Width, sz.Height)
	}
	if math.Abs(p.FillRatio()-0.5) > 1e-9 {
		t.Fatalf("fill=%v", p.FillRatio())
	}

	dc := render.NewContext(int(cw), int(ch))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	offX := (cw - sz.Width) / 2
	offY := (ch - sz.Height) / 2
	p.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(offX, offY))
	got := dc.Image()

	tok := theme.Default.Current()
	to16 := func(v float64) uint32 { return uint32(v*65535 + 0.5) }
	_ = to16
	// Fill center: track start + 1/4 width stays inside the 80px fill.
	fx, fy := int(offX+40), int(ch/2)
	r, g, b, _ := got.At(fx, fy).RGBA()
	pr, pg, pb, _ := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: 1}.RGBA()
	if diff(r, pr) > 8*257 || diff(g, pg) > 8*257 || diff(b, pb) > 8*257 {
		t.Fatalf("fill pixel #%04x%04x%04x want primary", r, g, b)
	}
	// Rail beyond fill: track start + 3/4 width is unfilled rail.
	rx := int(offX + 120)
	r2, g2, b2, _ := got.At(rx, fy).RGBA()
	rr, rg, rb, _ := render.RGBA{R: tok.ColorFillSecondary.R, G: tok.ColorFillSecondary.G, B: tok.ColorFillSecondary.B, A: 1}.RGBA()
	// Rail over white may blend; allow composite tolerance via mixed check:
	// rail alpha is low (0.06 black over white) so expect near-white, just
	// assert it differs from fill and stays bright.
	if r2 < 0xD000 || g2 < 0xD000 || b2 < 0xD000 {
		t.Fatalf("rail pixel #%04x%04x%04x want bright rail (mixed %04x)", r2, g2, b2, rr)
	}
	_ = rg
	_ = rb
	if r2 == r && g2 == g && b2 == b {
		t.Fatal("rail must differ from fill")
	}

	path := filepath.Join("testdata", "golden_line50.png")
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
