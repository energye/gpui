package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/watermark.md §6.9 — P0 L1/L2 cases (WM-01…14; WM-15/16 N/A; WM-17 L3; WM-18 L4; WM-19 P1 omitted).

func wmHasType(n core.Node, typ string) bool {
	if n == nil {
		return false
	}
	if n.TypeID() == typ {
		return true
	}
	for _, c := range n.Children() {
		if wmHasType(c, typ) {
			return true
		}
	}
	return false
}

func wmFindText(n core.Node, want string) bool {
	if n == nil {
		return false
	}
	if tx, ok := n.(*primitive.Text); ok && tx.Value == want {
		return true
	}
	for _, c := range n.Children() {
		if wmFindText(c, want) {
			return true
		}
	}
	return false
}

func TestWatermark_PRD_01_Defaults(t *testing.T) {
	// WM-01
	w := kit.NewWatermark(nil)
	if w == nil || w.Node() == nil {
		t.Fatal("nil watermark")
	}
	if w.Node().TypeID() != "kit.Watermark" {
		t.Fatalf("type=%s", w.Node().TypeID())
	}
	if r := w.ResolvedRotate(); r < -22.5 || r > -21.5 {
		t.Fatalf("rotate=%v want -22", r)
	}
	gx, gy := w.ResolvedGap()
	if gx < 99.5 || gx > 100.5 || gy < 99.5 || gy > 100.5 {
		t.Fatalf("gap=%v,%v want 100,100", gx, gy)
	}
	ox, oy := w.ResolvedOffset()
	if ox < 49.5 || ox > 50.5 || oy < 49.5 || oy > 50.5 {
		t.Fatalf("offset=%v,%v want 50,50", ox, oy)
	}
	if !w.ResolvedInherit() {
		t.Fatal("inherit default true")
	}
	if fs := w.ResolvedFontSize(); fs < 15.5 || fs > 16.5 {
		t.Fatalf("fontSize=%v want 16", fs)
	}
	if w.Node().Base().Role != "presentation" {
		t.Fatalf("Role=%q want presentation", w.Node().Base().Role)
	}
	sz := w.Node().Layout(core.Loose(200, 120))
	if sz.Width < 0 || sz.Height < 0 {
		t.Fatalf("size=%v", sz)
	}
}

func TestWatermark_PRD_02_TextContent(t *testing.T) {
	// WM-02 / WM-S1
	body := primitive.NewBox()
	body.Width, body.Height = 200, 100
	w := kit.NewWatermark(body)
	w.SetContent("Ant Design")
	if !w.HasMark() {
		t.Fatal("HasMark with content")
	}
	if w.ContentText() != "Ant Design" {
		t.Fatalf("content=%q", w.ContentText())
	}
	_ = w.Node().Layout(core.Loose(200, 100))
	if !wmHasType(w.Node(), "kit.WatermarkMark") {
		t.Fatal("expected mark layer")
	}
}

func TestWatermark_PRD_03_Rotate(t *testing.T) {
	// WM-03 / WM-S2
	w := kit.NewWatermark(nil)
	w.SetContent("R")
	if w.ResolvedRotate() != kit.DefaultWatermarkRotate {
		t.Fatalf("default rotate=%v", w.ResolvedRotate())
	}
	w.SetRotate(0)
	if w.ResolvedRotate() != 0 {
		t.Fatal("explicit 0")
	}
	w.SetRotate(-45)
	if w.ResolvedRotate() != -45 {
		t.Fatalf("got %v", w.ResolvedRotate())
	}
}

func TestWatermark_PRD_04_Gap(t *testing.T) {
	// WM-04 / WM-S3
	w := kit.NewWatermark(nil)
	w.SetContent("G")
	gx0, gy0 := w.ResolvedGap()
	w.SetGap(40, 60)
	gx, gy := w.ResolvedGap()
	if gx != 40 || gy != 60 {
		t.Fatalf("gap=%v,%v", gx, gy)
	}
	if gx == gx0 && gy == gy0 {
		t.Fatal("gap should change from default")
	}
	// offset follows gap/2 until SetOffset
	ox, oy := w.ResolvedOffset()
	if ox != 20 || oy != 30 {
		t.Fatalf("offset from gap/2 = %v,%v", ox, oy)
	}
	w.SetOffset(10, 12)
	ox, oy = w.ResolvedOffset()
	if ox != 10 || oy != 12 {
		t.Fatalf("explicit offset=%v,%v", ox, oy)
	}
}

func TestWatermark_PRD_05_Image(t *testing.T) {
	// WM-05 / WM-S4
	w := kit.NewWatermark(nil)
	w.SetImage("logo")
	w.SetWidth(130)
	w.SetHeight(30)
	if !w.HasMark() {
		t.Fatal("image src intends mark")
	}
	if !w.Loading {
		t.Fatal("loading until pixels")
	}
	pix := make([]byte, 4*4*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = 200, 100, 50, 255
	}
	w.SetImagePixels(4, 4, pix)
	if !w.UsesImageMark() {
		t.Fatal("image mark after pixels")
	}
	if w.Loading {
		t.Fatal("loading cleared")
	}
	mw, mh := w.ResolvedMarkSize()
	if mw < 129.5 || mw > 130.5 || mh < 29.5 || mh > 30.5 {
		t.Fatalf("mark size=%v,%v", mw, mh)
	}
	_ = w.Node().Layout(core.Loose(300, 200))
}

func TestWatermark_PRD_06_ChildrenClickable(t *testing.T) {
	// WM-06 / WM-S5
	btn := kit.NewButton("ok")
	w := kit.NewWatermark(btn.Node())
	w.SetContent("WM")
	root := w.Node()
	_ = root.Layout(core.Loose(200, 80))
	if !wmHasType(root, "kit.WatermarkMark") {
		t.Fatal("mark present")
	}
	// Mark must be HitTransparent
	for _, c := range root.Children() {
		if c.TypeID() == "kit.WatermarkMark" {
			if c.Base().Hit != core.HitTransparent {
				t.Fatalf("mark Hit=%v want Transparent", c.Base().Hit)
			}
		}
	}
	hit := root.HitTest(core.Point{X: 10, Y: 10})
	if hit == nil {
		// body may still be hittable deeper
		if w.Child() == nil {
			t.Fatal("child missing")
		}
	} else if hit.TypeID() == "kit.WatermarkMark" {
		t.Fatal("mark must not capture hits")
	}
}

func TestWatermark_PRD_07_MultiLine(t *testing.T) {
	// WM-07 / WM-S6
	w := kit.NewWatermark(nil)
	w.SetContentLines(
		kit.WatermarkContentLine{Text: "Ant Design"},
		kit.WatermarkContentLine{
			Text:    "Happy Working",
			Font:    kit.WatermarkFont{FontSize: 12},
			HasFont: true,
		},
	)
	lines := w.ContentLines()
	if len(lines) != 2 {
		t.Fatalf("lines=%d", len(lines))
	}
	if lines[1].Font.FontSize != 12 || !lines[1].HasFont {
		t.Fatal("per-line font")
	}
	if !w.HasMark() {
		t.Fatal("HasMark")
	}
	_ = w.Node().Layout(core.Loose(240, 160))
}

func TestWatermark_PRD_08_BasicDemo(t *testing.T) {
	// WM-08 basic.tsx
	box := primitive.NewBox()
	box.Height = 500
	box.Width = 320
	w := kit.NewWatermark(box)
	w.SetContent("Ant Design")
	sz := w.Node().Layout(core.Loose(400, 500))
	if sz.Height < 100 {
		t.Fatalf("basic size=%v", sz)
	}
	if w.ContentText() != "Ant Design" {
		t.Fatal("content")
	}
}

func TestWatermark_PRD_09_MultiLineDemo(t *testing.T) {
	// WM-09 multi-line.tsx
	box := primitive.NewBox()
	box.Height = 500
	w := kit.NewWatermark(box)
	w.SetContentLines(
		kit.WatermarkContentLine{Text: "Ant Design"},
		kit.WatermarkContentLine{Text: "Happy Working", Font: kit.WatermarkFont{FontSize: 12}, HasFont: true},
	)
	_ = w.Node().Layout(core.Loose(400, 500))
	if len(w.ContentLines()) != 2 {
		t.Fatal("two lines")
	}
}

func TestWatermark_PRD_10_ImageDemo(t *testing.T) {
	// WM-10 image.tsx
	box := primitive.NewBox()
	box.Height = 500
	w := kit.NewWatermark(box)
	w.SetHeight(30)
	w.SetWidth(130)
	w.SetImage("https://example.com/wm.png")
	pix := make([]byte, 8*8*4)
	for i := range pix {
		pix[i] = 128
	}
	w.SetImagePixels(8, 8, pix)
	_ = w.Node().Layout(core.Loose(400, 500))
	if !w.UsesImageMark() {
		t.Fatal("image mode")
	}
}

func TestWatermark_PRD_11_CustomDemo(t *testing.T) {
	// WM-11 custom.tsx — content/font/zIndex/rotate/gap/offset configurable
	body := primitive.NewBox()
	body.Width, body.Height = 400, 300
	w := kit.NewWatermark(body)
	w.SetContent("Ant Design")
	w.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 0.15})
	w.SetFontSize(16)
	w.SetZIndex(11)
	w.SetRotate(-22)
	w.SetGap(100, 100)
	w.SetOffset(50, 50)
	_ = w.Node().Layout(core.Loose(400, 300))
	if w.ResolvedZIndex() != 11 {
		t.Fatalf("z=%d", w.ResolvedZIndex())
	}
	if w.ResolvedRotate() != -22 {
		t.Fatal("rotate")
	}
	ox, oy := w.ResolvedOffset()
	if ox != 50 || oy != 50 {
		t.Fatalf("offset=%v,%v", ox, oy)
	}
}

func TestWatermark_PRD_12_PortalInherit(t *testing.T) {
	// WM-12 portal.tsx — inherit default + Wrap for Modal/Drawer content
	outer := kit.NewWatermark(primitive.NewText("page"))
	outer.SetContent("Ant Design")
	if !outer.ResolvedInherit() {
		t.Fatal("inherit default true")
	}
	// Simulate Modal body with inherited mark
	modalBody := primitive.NewBox()
	modalBody.Width, modalBody.Height = 200, 120
	wrapped := outer.Wrap(modalBody)
	if wrapped.ContentText() != "Ant Design" {
		t.Fatal("wrap copies content")
	}
	if wrapped.ResolvedRotate() != outer.ResolvedRotate() {
		t.Fatal("wrap copies rotate")
	}
	_ = wrapped.Node().Layout(core.Loose(200, 120))

	// inherit=false path: still can open drawer without Wrap
	noInh := kit.NewWatermark(nil)
	noInh.SetContent("Ant Design")
	noInh.SetInherit(false)
	if noInh.ResolvedInherit() {
		t.Fatal("inherit false")
	}
	// Drawer content without Wrap → plain node (no mark)
	plain := primitive.NewText("drawer body")
	if wmHasType(plain, "kit.Watermark") {
		t.Fatal("plain has no watermark")
	}

	// onRemove
	called := false
	outer.SetOnRemove(func() { called = true })
	outer.NotifyRemoved()
	if !called {
		t.Fatal("onRemove")
	}
}

func TestWatermark_PRD_13_Metrics(t *testing.T) {
	// WM-13 L2 §6.2
	w := kit.NewWatermark(nil)
	if kit.DefaultWatermarkRotate != -22 {
		t.Fatal("rotate const")
	}
	if kit.DefaultWatermarkGapX != 100 || kit.DefaultWatermarkGapY != 100 {
		t.Fatal("gap const")
	}
	if kit.DefaultWatermarkWidth != 120 || kit.DefaultWatermarkHeight != 64 {
		t.Fatal("mark size const")
	}
	if kit.DefaultWatermarkFontSize != 16 {
		t.Fatal("fontSize const")
	}
	if kit.DefaultWatermarkFontGap != 3 {
		t.Fatal("FontGap const")
	}
	approx := func(got, want, tol float64) bool {
		d := got - want
		if d < 0 {
			d = -d
		}
		return d <= tol
	}
	if !approx(w.ResolvedRotate(), -22, 0.5) {
		t.Fatal(w.ResolvedRotate())
	}
	gx, gy := w.ResolvedGap()
	if !approx(gx, 100, 0.5) || !approx(gy, 100, 0.5) {
		t.Fatal(gx, gy)
	}
	if !approx(w.ResolvedFontSize(), 16, 0.5) {
		t.Fatal(w.ResolvedFontSize())
	}
	// image defaults
	w.SetImage("x")
	pix := []byte{0, 0, 0, 255}
	w.SetImagePixels(1, 1, pix)
	mw, mh := w.ResolvedMarkSize()
	if !approx(mw, 120, 0.5) || !approx(mh, 64, 0.5) {
		t.Fatalf("image mark default %v,%v", mw, mh)
	}
}

func TestWatermark_PRD_14_DefaultColor(t *testing.T) {
	// WM-14 — no brand primary as default ink
	w := kit.NewWatermark(nil)
	w.SetContent("c")
	col := w.ResolvedColor()
	primary := render.Hex("#1677FF")
	if col.R == primary.R && col.G == primary.G && col.B == primary.B && col.A > 0.5 {
		t.Fatal("default color must not be brand primary")
	}
	def := kit.DefaultWatermarkColor()
	if col.A < 0.05 {
		t.Fatal("need visible alpha")
	}
	// close to documented rgba(0,0,0,.15)
	if def.A < 0.10 || def.A > 0.20 {
		t.Fatalf("default A=%v", def.A)
	}
	// Theme must not force brand as only path — Style override works
	w.SetFontColor(render.RGBA{R: 0.2, G: 0.2, B: 0.2, A: 0.2})
	c2 := w.ResolvedColor()
	if c2.A < 0.15 || c2.A > 0.25 {
		t.Fatalf("font color override A=%v", c2.A)
	}
}

func TestWatermark_PRD_15_DisabledNA(t *testing.T) {
	// WM-15 N/A — decorative; no disabled chrome
	t.Log("N/A: Watermark has no disabled state in antd")
}

func TestWatermark_PRD_16_KeyboardNA(t *testing.T) {
	// WM-16 N/A — presentation role, no focus
	w := kit.NewWatermark(nil)
	w.SetContent("x")
	_ = w.Node().Layout(core.Loose(100, 80))
	if w.Node().Base().Role != "presentation" {
		t.Fatal("decorative role")
	}
	t.Log("N/A: no keyboard path for decorative watermark")
}

func TestWatermark_PRD_ImageErrorFallback(t *testing.T) {
	// FAQ / WM-S8
	w := kit.NewWatermark(nil)
	w.SetContent("fallback")
	w.SetImage("bad")
	w.NotifyImageError()
	if w.UsesImageMark() {
		t.Fatal("no image after error")
	}
	if !w.HasMark() {
		t.Fatal("text fallback still marks")
	}
	if w.ContentText() != "fallback" {
		t.Fatal("content kept")
	}
}

func TestWatermark_PRD_LoadingTicker(t *testing.T) {
	w := kit.NewWatermark(nil)
	w.SetImage("pending")
	if !w.Loading {
		t.Fatal("loading")
	}
	tree := core.NewTree(w.Node())
	w.AttachTicker(tree)
	if !w.Tick(0.05) {
		t.Fatal("ticker while loading")
	}
	w.SetImagePixels(1, 1, []byte{1, 2, 3, 255})
	if w.Tick(0.05) {
		t.Fatal("ticker stops after pixels")
	}
}
