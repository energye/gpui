package watermark_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
)

// P0 font full: color/size/weight/family/style/align each round-trips and
// participates in tile measurement. Numbers come from testdata/watermark.json
// defaults (see TestWatermark_PRD_WM13/WM14); per-line WatermarkText font wins.
func TestWatermark_PRD_WMFontFull(t *testing.T) {
	f := loadWatermark(t)
	wm := watermark.NewWatermark(sizedBox(300, 200))
	wm.SetContent("Ant Design")
	if wm.EffectiveFontSize() != f.FontSize {
		t.Fatalf("default fontsize=%v want %v", wm.EffectiveFontSize(), f.FontSize)
	}
	if wm.EffectiveFontFamily() != f.FontFamily {
		t.Fatalf("family=%q want %q", wm.EffectiveFontFamily(), f.FontFamily)
	}
	if wm.EffectiveFontStyle() != f.FontStyle {
		t.Fatalf("style=%q want %q", wm.EffectiveFontStyle(), f.FontStyle)
	}
	if wm.EffectiveTextAlign() != f.TextAlign {
		t.Fatalf("align=%q want %q", wm.EffectiveTextAlign(), f.TextAlign)
	}
	wm.SetFontColor(render.RGBA{R: 0.2, G: 0.1, B: 0.8, A: 0.9})
	wm.SetFontSize(20)
	wm.SetFontWeight("bold")
	wm.SetFontFamily("serif")
	wm.SetFontStyle("italic")
	wm.SetTextAlign("right")
	if wm.EffectiveFontSize() != 20 {
		t.Fatalf("fontsize=%v want 20", wm.EffectiveFontSize())
	}
	if wm.EffectiveFontWeight() != "bold" {
		t.Fatalf("weight=%q", wm.EffectiveFontWeight())
	}
	if wm.EffectiveFontFamily() != "serif" {
		t.Fatalf("family=%q", wm.EffectiveFontFamily())
	}
	if wm.EffectiveFontStyle() != "italic" {
		t.Fatalf("style=%q", wm.EffectiveFontStyle())
	}
	if wm.EffectiveTextAlign() != "right" {
		t.Fatalf("align=%q", wm.EffectiveTextAlign())
	}
	got := wm.EffectiveFontColor()
	if got.R != 0.2 || got.G != 0.1 || got.B != 0.8 {
		t.Fatalf("color=%+v", got)
	}
	// Per-line font wins over global for that row.
	wm.SetContentLines(
		watermark.WatermarkContentLine{Text: "Top"},
		watermark.WatermarkContentLine{
			Text:    "Bottom",
			Font:    watermark.WatermarkFont{FontSize: 12, TextAlign: "left"},
			HasFont: true,
		},
	)
	lines := wm.ContentLines()
	if len(lines) != 2 || lines[1].Text != "Bottom" || !lines[1].HasFont {
		t.Fatalf("lines=%+v", lines)
	}
	if _, mh := wm.ResolvedMarkSize(); mh <= 0 {
		t.Fatal("multi-line mark height must be positive")
	}
	// Struct SetFont path covers the same knobs in one call.
	wm.SetFont(watermark.WatermarkFont{
		Color:      render.RGBA{R: 0, G: 0, B: 0, A: 1},
		HasColor:   true,
		FontSize:   18,
		FontWeight: "bolder",
		FontFamily: "sans-serif",
		FontStyle:  "oblique",
		TextAlign:  "center",
	})
	if wm.EffectiveFontSize() != 18 || wm.EffectiveFontWeight() != "bolder" {
		t.Fatalf("SetFont not applied: %+v", wm.EffectiveFontColor())
	}
	paintOK(wm, 64, 64)
}

// True text: with a face the tile zone carries glyph ink via
// SetFont+DrawString+Abs. Without fonts this Skips (env lacks glyphs).
func TestWatermark_PRD_WM02_TrueText(t *testing.T) {
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(16)
	if err != nil || face == nil {
		t.Skipf("true text needs a system face: %v", err)
	}
	t.Logf("face: %s", desc)
	host := rendering.NewRenderColorBox(160, 80, 1, 1, 1, 1)
	wm := watermark.NewWatermark(host)
	wm.SetContent("Ant Design")
	wm.SetRotate(0)
	wm.SetGap(20, 20)
	wm.SetOffset(0, 0)
	wm.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	wm.SetFace(face)
	sz := wm.Layout(rendering.Tight(160, 80))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout must be non-zero, got %v", sz)
	}
	dc := render.NewContext(160, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	wm.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()
	dark := 0
	for y := 0; y < 80; y++ {
		for x := 0; x < 160; x++ {
			r, g, b, _ := got.At(x, y).RGBA()
			if r/257 < 110 && g/257 < 110 && b/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("dark pixels=%d want >=30 (real glyphs missing?)", dark)
	}
}

// No-face empty: without a face no bar is drawn, canvas stays white.
// Watermark without face may be empty (spec allows empty).
func TestWatermark_PRD_WM02_NoFaceEmpty(t *testing.T) {
	host := rendering.NewRenderColorBox(96, 64, 1, 1, 1, 1)
	wm := watermark.NewWatermark(host)
	wm.SetContent("WM")
	wm.SetRotate(0)
	wm.SetGap(8, 8)
	wm.SetOffset(0, 0)
	wm.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 1})
	// No SetFace on purpose.
	wm.Layout(rendering.Tight(96, 64))
	dc := render.NewContext(96, 64)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	wm.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()
	for y := 0; y < 64; y++ {
		for x := 0; x < 96; x++ {
			r, g, b, _ := got.At(x, y).RGBA()
			if r/257 < 250 || g/257 < 250 || b/257 < 250 {
				t.Fatalf("no-face pixel (%d,%d) #%02x%02x%02x want white (no bar)", x, y, r/257, g/257, b/257)
			}
		}
	}
}

// Layout/Node non-zero: host size follows child, mark never expands it,
// Node is the single tree entry (hit==layout==paint).
func TestWatermark_PRD_WM08_LayoutNode(t *testing.T) {
	kid := sizedBox(300, 200)
	wm := watermark.NewWatermark(kid)
	wm.SetContent("Ant Design")
	if wm.Node() == nil {
		t.Fatal("Node must be non-nil")
	}
	sz := wm.Layout(rendering.Tight(300, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout must be non-zero, got %v", sz)
	}
	if sz.Width != 300 || sz.Height != 200 {
		t.Fatalf("layout=%v want 300x200 (host follows child)", sz)
	}
	if !wm.HasMark() {
		t.Fatal("content must mark")
	}
	if wm.TiledCount(300, 200) <= 0 {
		t.Fatal("tiled count must be positive")
	}
}

// Explicit width/height: measured text vs explicit mark size.
func TestWatermark_PRD_WM10_ExplicitSize(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(300, 200))
	wm.SetContent("Ant Design")
	autoW, autoH := wm.ResolvedMarkSize()
	wm.SetWidth(140)
	wm.SetHeight(40)
	expW, expH := wm.ResolvedMarkSize()
	if expW != 140 || expH != 40 {
		t.Fatalf("explicit=%vx%v want 140x40", expW, expH)
	}
	if autoW == 140 && autoH == 40 {
		t.Logf("measured coincides with explicit (ok): %vx%v", autoW, autoH)
	}
	wm.SetWidth(0)
	wm.SetHeight(0)
	backW, backH := wm.ResolvedMarkSize()
	if backW <= 0 || backH <= 0 {
		t.Fatal("cleared size must re-measure")
	}
}

// Wrap copies mark config for Modal/Drawer content (portal.tsx desktop map).
func TestWatermark_PRD_WM12_WrapCopy(t *testing.T) {
	src := watermark.NewWatermark(sizedBox(300, 200))
	src.SetContent("Ant Design")
	src.SetRotate(-15)
	src.SetGap(80, 80)
	src.SetOffset(40, 40)
	src.SetFontSize(20)
	src.SetFontColor(render.RGBA{R: 0.5, G: 0, B: 0, A: 1})
	src.SetFontWeight("bold")
	src.SetZIndex(10)
	dst := src.Wrap(sizedBox(200, 120))
	if len(dst.ContentLines()) != 1 || dst.ResolvedRotate() != -15 {
		t.Fatal("Wrap must copy content+rotate")
	}
	gx, gy := dst.ResolvedGap()
	if gx != 80 || gy != 80 {
		t.Fatalf("gap=%v,%v", gx, gy)
	}
	ox, oy := dst.ResolvedOffset()
	if ox != 40 || oy != 40 {
		t.Fatalf("offset=%v,%v", ox, oy)
	}
	if dst.EffectiveFontSize() != 20 || dst.ResolvedZIndex() != 10 {
		t.Fatal("Wrap must copy font size+zIndex")
	}
	if dst.EffectiveFontWeight() != "bold" {
		t.Fatal("Wrap must copy weight")
	}
}

// P1 staged: browser-only or later items explicitly Skipped with reasons
// (spec §6.8 P1). Not silent drops.
func TestWatermark_PRD_WM15_DisabledNA(t *testing.T) {
	t.Skip("N/A: watermark is decoration with no disabled state (spec §6.9 WM-15)")
}

func TestWatermark_PRD_WM16_KeyboardNA(t *testing.T) {
	t.Skip("N/A: decoration never takes focus, no keyboard path (spec §6.9 WM-16)")
}
func TestWatermark_PRD_WM19_P1Staged(t *testing.T) {
	t.Run("SemanticClassNames", func(t *testing.T) {
		t.Skip("P1 staged: semantic classNames/styles depth maps to SetStyle hook only (no CSS engine on desktop)")
	})
	t.Run("MutationObserver", func(t *testing.T) {
		t.Skip("P1 browser-only: DOM MutationObserver rebuild has no desktop equal; desktop uses NotifyRemoved/OnRemove")
	})
	t.Run("HttpDecode", func(t *testing.T) {
		t.Skip("P1 host duty: true HTTP image URL decode lives in host; kit only receives pixels via SetImagePixels")
	})
	t.Run("AlternateClip", func(t *testing.T) {
		t.Skip("P1 staged: alternate clip pixel parity with官网 canvas is later")
	})
	t.Run("DebugHash", func(t *testing.T) {
		t.Skip("P1 staged: debug example逐像素哈希 with官网 is not done")
	})
	t.Run("ConfigProvider", func(t *testing.T) {
		t.Skip("P1 staged: ConfigProvider global watermark defaults are later; per-instance SetTheme/Provider only")
	})
}
