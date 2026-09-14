package button_test

import (
	"encoding/json"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
)

type tolerance struct {
	MaxDiff float64 `json:"maxDiff"`
	BadFrac float64 `json:"badFrac"`
	HardCap float64 `json:"hardCap"`
}

func loadTolerance(t *testing.T) tolerance {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "buttons.json"))
	if err != nil {
		t.Fatalf("read buttons.json: %v", err)
	}
	var raw struct {
		Tolerance tolerance `json:"tolerance"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("parse tolerance: %v", err)
	}
	return raw.Tolerance
}

// paintButton lays out b intrinsically and paints it on its exact canvas.
func paintButton(b *button.Button) image.Image {
	sz := b.Layout(rendering.Loose(1000, 1000))
	dc := render.NewContext(int(sz.Width), int(sz.Height))
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	b.Node().Paint(rendering.NewPaintContext(dc, 1))
	return dc.Image()
}

func compareGolden(t *testing.T, got image.Image, name string, tol tolerance) {
	t.Helper()
	path := filepath.Join("testdata", name)
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
	maxDiff := uint32(tol.MaxDiff * 257)
	hardCap := uint32(tol.HardCap * 257)
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
	if float64(bad)/float64(total) > tol.BadFrac {
		t.Fatalf("bad pixels %d/%d exceed %.1f%% (tolerance maxDiff=%.0f)", bad, total, tol.BadFrac*100, tol.MaxDiff)
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

// L3 golden: primary middle button (logic probe + pixel assertions + file).
//
// Three evidences: logic probe (solid variant + spaced 66.4x32 layout),
// pixel assertion (center primary, corner white, top edge primary),
// golden file compare with explicit tolerance (AA edges only). Regenerate
// with UPDATE_GOLDEN=1. Width 66.4 = "确 定" spaced (P1 autoInsertSpace).
func TestButton_PRD_BTN21_GoldenPrimary(t *testing.T) {
	tol := loadTolerance(t)
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	if b.EffectiveVariant() != button.VariantSolid {
		t.Fatal("primary must be solid for the golden")
	}
	sz := b.Layout(rendering.Loose(1000, 1000))
	if math.Abs(sz.Width-66.4) > 0.5 || math.Abs(sz.Height-32) > 0.5 {
		t.Fatalf("layout=%vx%v want 66.4x32 (spaced)", sz.Width, sz.Height)
	}
	got := paintButton(b)

	cx, cy := int(sz.Width)/2, int(sz.Height)/2
	if r, g, bl, _ := got.At(cx, cy).RGBA(); r > 0x4000 || bl < 0xC000 || g > 0xC000 {
		t.Fatalf("center #%04x%04x%04x want primary #1677ff", r, g, bl)
	}
	if r, g, bl, _ := got.At(1, 1).RGBA(); r < 0xE000 || g < 0xE000 || bl < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white (outside radius)", r, g, bl)
	}
	if r, _, bl, _ := got.At(cx, 0).RGBA(); r > 0x4000 || bl < 0xC000 {
		t.Fatalf("top edge #%04x..%04x want primary border", r, bl)
	}

	compareGolden(t, got, "golden_primary_middle.png", tol)
}

// L3 golden: default middle button (container fill, gray border, spaced).
func TestButton_GoldenDefaultMiddle(t *testing.T) {
	tol := loadTolerance(t)
	b := button.NewButton("确定")
	sz := b.Layout(rendering.Loose(1000, 1000))
	if math.Abs(sz.Width-66.4) > 0.5 || math.Abs(sz.Height-32) > 0.5 {
		t.Fatalf("layout=%vx%v want 66.4x32 (spaced)", sz.Width, sz.Height)
	}
	got := paintButton(b)

	cx, cy := int(sz.Width)/2, int(sz.Height)/2
	if r, g, bl, _ := got.At(cx, cy).RGBA(); r < 0xF000 || g < 0xF000 || bl < 0xF000 {
		t.Fatalf("center #%04x%04x%04x want container white", r, g, bl)
	}
	r, g, bl, _ := got.At(cx, 0).RGBA()
	if r < 0x8000 || r > 0xF000 || diff(r, g) > 0x0800 || diff(g, bl) > 0x0800 {
		t.Fatalf("top edge #%04x%04x%04x want gray #d9d9d9 border", r, g, bl)
	}
	if r, g, bl, _ := got.At(1, 1).RGBA(); r < 0xE000 || g < 0xE000 || bl < 0xE000 {
		t.Fatalf("corner #%04x%04x%04x want white", r, g, bl)
	}

	compareGolden(t, got, "golden_default_middle.png", tol)
}
