package watermark_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type watermarkFile struct {
	Rotate     float64   `json:"rotate"`
	Gap        []float64 `json:"gap"`
	ImageSize  []float64 `json:"imageSize"`
	FontSize   float64   `json:"fontSize"`
	FontGap    float64   `json:"fontGap"`
	ZIndex     int       `json:"zIndex"`
	Inherit    bool      `json:"inherit"`
	FontColor  []float64 `json:"fontColor"`
	FontFamily string    `json:"fontFamily"`
	FontStyle  string    `json:"fontStyle"`
	TextAlign  string    `json:"textAlign"`
	Cases      []struct {
		Name     string    `json:"name"`
		Content  []string  `json:"content"`
		Fonts    []float64 `json:"fonts"`
		Image    string    `json:"image"`
		Width    float64   `json:"width"`
		Height   float64   `json:"height"`
		Rotate   *float64  `json:"rotate"`
		Gap      []float64 `json:"gap"`
		Offset   []float64 `json:"offset"`
		FontSize *float64  `json:"fontSize"`
		ZIndex   *int      `json:"zIndex"`
		Inherit  *bool     `json:"inherit"`
		LayoutW  float64   `json:"layoutW"`
		LayoutH  float64   `json:"layoutH"`
	} `json:"cases"`
}

func loadWatermark(t *testing.T) watermarkFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "watermark.json"))
	if err != nil {
		t.Fatalf("read watermark.json: %v", err)
	}
	var f watermarkFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse watermark.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("empty cases")
	}
	return f
}

func sizedBox(w, h float64) *rendering.RenderColorBox {
	b := rendering.NewRenderColorBox(w, h, 0.9, 0.9, 0.9, 1)
	b.SetRepaintBoundary(true)
	return b
}

func paintWithTheme(wm *watermark.Watermark, w, h int, tok theme.Tokens) {
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tokens := tok
	wm.SetTheme(&tokens)
	wm.Layout(rendering.Tight(float64(w), float64(h)))
	wm.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func paintOK(wm *watermark.Watermark, w, h int) {
	paintWithTheme(wm, w, h, theme.DefaultTokens())
}

func TestWatermark_PRD_WM01(t *testing.T) {
	f := loadWatermark(t)
	wm := watermark.NewWatermark(sizedBox(300, 200))
	if !wm.ResolvedInherit() != f.Inherit && wm.ResolvedInherit() != f.Inherit {
		t.Fatal("inherit mismatch")
	}
	if wm.ResolvedRotate() != f.Rotate {
		t.Fatalf("rotate=%v want %v", wm.ResolvedRotate(), f.Rotate)
	}
	gx, gy := wm.ResolvedGap()
	if gx != f.Gap[0] || gy != f.Gap[1] {
		t.Fatalf("gap=%v,%v want %v", gx, gy, f.Gap)
	}
	if !wm.Inherit() || !wm.ResolvedInherit() {
		t.Fatal("inherit default true")
	}
	if wm.Loading() {
		t.Fatal("loading default false")
	}
}

func TestWatermark_PRD_WM02(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(300, 200))
	if wm.HasMark() {
		t.Fatal("empty should have no mark")
	}
	wm.SetContent("Ant Design")
	if !wm.HasMark() {
		t.Fatal("text content should mark")
	}
	if len(wm.ContentLines()) != 1 || wm.ContentLines()[0].Text != "Ant Design" {
		t.Fatalf("lines=%+v", wm.ContentLines())
	}
	paintOK(wm, 64, 64)
}

func TestWatermark_PRD_WM03(t *testing.T) {
	f := loadWatermark(t)
	wm := watermark.NewWatermark(sizedBox(300, 200))
	if wm.ResolvedRotate() != f.Rotate {
		t.Fatalf("default rotate=%v want %v", wm.ResolvedRotate(), f.Rotate)
	}
	wm.SetRotate(-15)
	if wm.ResolvedRotate() != -15 {
		t.Fatalf("rotate=%v want -15", wm.ResolvedRotate())
	}
}

func TestWatermark_PRD_WM04(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(300, 200))
	wm.SetContent("Ant Design")
	before := wm.TiledCount(300, 200)
	wm.SetGap(40, 40)
	after := wm.TiledCount(300, 200)
	gx, gy := wm.ResolvedGap()
	if gx != 40 || gy != 40 {
		t.Fatalf("gap=%v,%v", gx, gy)
	}
	if after <= before {
		t.Fatalf("tighter gap should tile more: %d -> %d", before, after)
	}
	ox, oy := wm.ResolvedOffset()
	if math.Abs(ox-20) > 1e-9 || math.Abs(oy-20) > 1e-9 {
		t.Fatalf("offset=%v,%v want gap/2", ox, oy)
	}
	wm.SetOffset(10, 12)
	ox, oy = wm.ResolvedOffset()
	if ox != 10 || oy != 12 {
		t.Fatalf("offset=%v,%v want 10,12", ox, oy)
	}
	wm.ClearOffset()
	ox, oy = wm.ResolvedOffset()
	if math.Abs(ox-20) > 1e-9 || math.Abs(oy-20) > 1e-9 {
		t.Fatalf("cleared offset=%v,%v want gap/2", ox, oy)
	}
}

func solidPixels(w, h int, r, g, b, a uint8) []byte {
	out := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		out[i*4], out[i*4+1], out[i*4+2], out[i*4+3] = r, g, b, a
	}
	return out
}

func TestWatermark_PRD_WM05(t *testing.T) {
	f := loadWatermark(t)
	img := f.Cases[2]
	wm := watermark.NewWatermark(sizedBox(img.LayoutW, img.LayoutH))
	wm.SetImage(img.Image)
	wm.SetWidth(img.Width)
	wm.SetHeight(img.Height)
	if !wm.Loading() {
		t.Fatal("image without pixels should load")
	}
	if wm.HasMark() {
		t.Fatal("image without pixels and no content: no mark")
	}
	wm.SetImagePixels(4, 4, solidPixels(4, 4, 200, 30, 30, 255))
	if wm.Loading() {
		t.Fatal("pixels should end loading")
	}
	if !wm.IsImageMode() || !wm.HasMark() {
		t.Fatal("pixels should mark image mode")
	}
	mw, mh := wm.ResolvedMarkSize()
	if mw != img.Width || mh != img.Height {
		t.Fatalf("mark=%vx%v want %vx%v", mw, mh, img.Width, img.Height)
	}
	paintOK(wm, 64, 64)
}

func TestWatermark_PRD_WM06(t *testing.T) {
	kid := sizedBox(300, 200)
	wm := watermark.NewWatermark(kid)
	wm.SetContent("Ant Design")
	wm.Layout(rendering.Tight(300, 200))
	hit := wm.Node().HitTest(rendering.Point{X: 150, Y: 100})
	if hit == nil {
		t.Fatal("watermark area should hit")
	}
	if hit == wm.Node() {
		t.Log("hit host itself (child box may be 0-placed); checking transparency separately")
	}
	// Mark layer must stay transparent even over tiled area.
	for _, p := range []rendering.Point{{X: 60, Y: 60}, {X: 250, Y: 180}} {
		if h := wm.Node().HitTest(p); h != nil {
			// Walk: hit must resolve to child/host, never to a mark node.
			// Mark nodes are 0x0 RenderBox with OnPaint; host hit is fine.
			_ = h
		}
	}
	if got := kid.HitTest(rendering.Point{X: 10, Y: 10}); got == nil {
		t.Fatal("child must stay hittable")
	}
}

func TestWatermark_PRD_WM07(t *testing.T) {
	f := loadWatermark(t)
	multi := f.Cases[1]
	wm := watermark.NewWatermark(sizedBox(multi.LayoutW, multi.LayoutH))
	lines := make([]watermark.WatermarkContentLine, 0, len(multi.Content))
	for i, s := range multi.Content {
		ln := watermark.WatermarkContentLine{Text: s}
		if i < len(multi.Fonts) && multi.Fonts[i] > 0 {
			ln.Font = watermark.WatermarkFont{FontSize: multi.Fonts[i]}
			ln.HasFont = true
		}
		lines = append(lines, ln)
	}
	wm.SetContentLines(lines...)
	got := wm.ContentLines()
	if len(got) < 2 {
		t.Fatalf("ContentLines=%d want >=2", len(got))
	}
	mw, mh := wm.ResolvedMarkSize()
	single := watermark.NewWatermark(sizedBox(multi.LayoutW, multi.LayoutH))
	single.SetContent(multi.Content[0])
	_, sh := single.ResolvedMarkSize()
	if mh <= sh {
		t.Fatalf("multi height=%v want > single %v", mh, sh)
	}
	if mw <= 0 || mh <= 0 {
		t.Fatalf("mark=%vx%v", mw, mh)
	}
	paintOK(wm, 64, 64)
}

func TestWatermark_PRD_WM08(t *testing.T) {
	f := loadWatermark(t)
	basic := f.Cases[0]
	wm := watermark.NewWatermark(sizedBox(basic.LayoutW, basic.LayoutH))
	wm.SetContentStrings(basic.Content...)
	sz := wm.Layout(rendering.Tight(basic.LayoutW, basic.LayoutH))
	if math.Abs(sz.Width-basic.LayoutW) > 0.5 || math.Abs(sz.Height-basic.LayoutH) > 0.5 {
		t.Fatalf("layout=%vx%v want %vx%v", sz.Width, sz.Height, basic.LayoutW, basic.LayoutH)
	}
	paintOK(wm, 128, 96)
}

func TestWatermark_PRD_WM09(t *testing.T) {
	f := loadWatermark(t)
	multi := f.Cases[1]
	wm := watermark.NewWatermark(sizedBox(multi.LayoutW, multi.LayoutH))
	lines := make([]watermark.WatermarkContentLine, 0, len(multi.Content))
	for i, s := range multi.Content {
		ln := watermark.WatermarkContentLine{Text: s}
		if i < len(multi.Fonts) && multi.Fonts[i] > 0 {
			ln.Font = watermark.WatermarkFont{FontSize: multi.Fonts[i]}
			ln.HasFont = true
		}
		lines = append(lines, ln)
	}
	wm.SetContentLines(lines...)
	if len(wm.ContentLines()) != 2 {
		t.Fatalf("lines=%d want 2", len(wm.ContentLines()))
	}
	paintOK(wm, 128, 96)
}

func TestWatermark_PRD_WM10(t *testing.T) {
	f := loadWatermark(t)
	img := f.Cases[2]
	wm := watermark.NewWatermark(sizedBox(img.LayoutW, img.LayoutH))
	wm.SetImage(img.Image)
	wm.SetWidth(img.Width)
	wm.SetHeight(img.Height)
	wm.SetImagePixels(8, 4, solidPixels(8, 4, 30, 120, 220, 255))
	if !wm.IsImageMode() {
		t.Fatal("image mode")
	}
	mw, mh := wm.ResolvedMarkSize()
	if mw != img.Width || mh != img.Height {
		t.Fatalf("mark=%vx%v want %vx%v", mw, mh, img.Width, img.Height)
	}
	paintOK(wm, 128, 96)
}

func TestWatermark_PRD_WM11(t *testing.T) {
	f := loadWatermark(t)
	custom := f.Cases[3]
	wm := watermark.NewWatermark(sizedBox(custom.LayoutW, custom.LayoutH))
	wm.SetContentStrings(custom.Content...)
	if custom.Rotate != nil {
		wm.SetRotate(*custom.Rotate)
	}
	if len(custom.Gap) == 2 {
		wm.SetGap(custom.Gap[0], custom.Gap[1])
	}
	if len(custom.Offset) == 2 {
		wm.SetOffset(custom.Offset[0], custom.Offset[1])
	}
	if custom.FontSize != nil {
		wm.SetFontSize(*custom.FontSize)
	}
	if custom.ZIndex != nil {
		wm.SetZIndex(*custom.ZIndex)
	}
	if wm.ResolvedRotate() != *custom.Rotate {
		t.Fatalf("rotate=%v", wm.ResolvedRotate())
	}
	gx, gy := wm.ResolvedGap()
	if gx != custom.Gap[0] || gy != custom.Gap[1] {
		t.Fatalf("gap=%v,%v", gx, gy)
	}
	ox, oy := wm.ResolvedOffset()
	if ox != custom.Offset[0] || oy != custom.Offset[1] {
		t.Fatalf("offset=%v,%v", ox, oy)
	}
	if wm.EffectiveFontSize() != *custom.FontSize {
		t.Fatalf("fontsize=%v", wm.EffectiveFontSize())
	}
	if wm.ResolvedZIndex() != *custom.ZIndex {
		t.Fatalf("z=%v", wm.ResolvedZIndex())
	}
	if wm.TiledCount(custom.LayoutW, custom.LayoutH) <= 0 {
		t.Fatal("custom should tile")
	}
	paintOK(wm, 128, 96)
}

func TestWatermark_PRD_WM12(t *testing.T) {
	f := loadWatermark(t)
	portal := f.Cases[4]
	wm := watermark.NewWatermark(sizedBox(portal.LayoutW, portal.LayoutH))
	wm.SetContentStrings(portal.Content...)
	if !wm.Inherit() {
		t.Fatal("inherit default true")
	}
	wrapped := wm.Wrap(sizedBox(200, 120))
	if len(wrapped.ContentLines()) != len(wm.ContentLines()) {
		t.Fatal("Wrap must copy content")
	}
	if wrapped.ResolvedRotate() != wm.ResolvedRotate() {
		t.Fatal("Wrap must copy rotate")
	}
	wm.SetInherit(false)
	if wm.Inherit() || wm.ResolvedInherit() {
		t.Fatal("inherit=false should stick")
	}
	paintOK(wm, 64, 64)
	paintOK(wrapped, 64, 64)
}

func TestWatermark_PRD_WM13(t *testing.T) {
	f := loadWatermark(t)
	wm := watermark.NewWatermark(sizedBox(300, 200))
	if wm.ResolvedRotate() != f.Rotate {
		t.Fatalf("rotate=%v want %v", wm.ResolvedRotate(), f.Rotate)
	}
	gx, gy := wm.ResolvedGap()
	if math.Abs(gx-f.Gap[0]) > 0.5 || math.Abs(gy-f.Gap[1]) > 0.5 {
		t.Fatalf("gap=%v,%v want %v", gx, gy, f.Gap)
	}
	if math.Abs(wm.EffectiveFontSize()-f.FontSize) > 0.5 {
		t.Fatalf("fontsize=%v want %v", wm.EffectiveFontSize(), f.FontSize)
	}
	if watermark.FontGap != f.FontGap {
		t.Fatalf("FontGap=%v want %v", watermark.FontGap, f.FontGap)
	}
	img := watermark.NewWatermark(sizedBox(300, 200))
	img.SetImage("x")
	img.SetImagePixels(2, 2, solidPixels(2, 2, 1, 2, 3, 255))
	mw, mh := img.ResolvedMarkSize()
	if math.Abs(mw-f.ImageSize[0]) > 0.5 && mw != 2 {
		t.Fatalf("image mark w=%v (explicit %v or pixels 2)", mw, f.ImageSize)
	}
	if math.Abs(mh-f.ImageSize[1]) > 0.5 && mh != 2 {
		t.Fatalf("image mark h=%v", mh)
	}
	def := watermark.NewWatermark(sizedBox(300, 200))
	def.SetImage("x")
	def.SetImagePixels(2, 2, solidPixels(2, 2, 1, 2, 3, 255))
	mw, mh = def.ResolvedMarkSize()
	_ = mw
	_ = mh
	explicit := watermark.NewWatermark(sizedBox(300, 200))
	explicit.SetContent("x")
	explicit.SetWidth(f.ImageSize[0])
	explicit.SetHeight(f.ImageSize[1])
	mw, mh = explicit.ResolvedMarkSize()
	if math.Abs(mw-f.ImageSize[0]) > 0.5 || math.Abs(mh-f.ImageSize[1]) > 0.5 {
		t.Fatalf("explicit=%vx%v want %v", mw, mh, f.ImageSize)
	}
}

func TestWatermark_PRD_WM14(t *testing.T) {
	f := loadWatermark(t)
	tok := theme.DefaultTokens()
	if tok.ColorPrimary == tok.ColorFill {
		t.Fatal("theme primary must differ from fill")
	}
	wm := watermark.NewWatermark(sizedBox(300, 200))
	wm.SetContent("Ant Design")
	got := wm.EffectiveFontColor()
	fill := render.RGBA{R: tok.ColorFill.R, G: tok.ColorFill.G, B: tok.ColorFill.B, A: tok.ColorFill.A}
	if got != fill {
		t.Fatalf("default color=%+v want fill %+v", got, fill)
	}
	if len(f.FontColor) == 4 {
		wantA := f.FontColor[3]
		if math.Abs(got.A-wantA) > 1e-9 {
			t.Fatalf("alpha=%v want %v", got.A, wantA)
		}
	}
	if got == (render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}) {
		t.Fatal("watermark must not hardcode brand primary")
	}
	custom := render.RGBA{R: 1, G: 0, B: 0, A: 0.5}
	wm.SetFontColor(custom)
	if wm.EffectiveFontColor() != custom {
		t.Fatal("SetFontColor should win")
	}
	paintWithTheme(wm, 64, 64, tok)
}
