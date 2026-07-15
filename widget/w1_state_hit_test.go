package widget_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/widget"
)

func TestW1_ButtonStateMatrix_Draw(t *testing.T) {
	th := widget.DefaultTheme()
	dc := render.NewContext(520, 200)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, 520, 200)
	_ = dc.Fill()

	styles := []widget.ButtonStyle{widget.ButtonPrimary, widget.ButtonDefault, widget.ButtonDanger, widget.ButtonText}
	states := []struct {
		name string
		mut  func(*widget.Button)
	}{
		{"idle", func(b *widget.Button) {}},
		{"hover", func(b *widget.Button) { b.Hovered = true }},
		{"press", func(b *widget.Button) { b.Pressed = true; b.Hovered = true }},
		{"focus", func(b *widget.Button) { b.Focused = true }},
		{"disabled", func(b *widget.Button) { b.Disabled = true }},
	}
	x := 12.0
	for _, st := range styles {
		y := 16.0
		for _, s := range states {
			b := widget.Button{Bounds: widget.Rect{X: x, Y: y, W: 100, H: 28}, Label: s.name, Style: st}
			s.mut(&b)
			b.Draw(dc, th)
			y += 34
		}
		x += 120
	}
	// sample primary pressed vs idle should differ
	// layout: style0 col x=12; rows idle y=16, press y=16+34*2=84
	img := dc.Image()
	ir, ig, ib, _ := img.At(50, 30).RGBA() // idle primary
	pr, pg, pb, _ := img.At(50, 98).RGBA() // pressed primary
	if ir == pr && ig == pg && ib == pb {
		t.Fatalf("pressed primary should differ from idle")
	}
}

func TestW1_HitFirst_Stacking(t *testing.T) {
	a := widget.Button{Bounds: widget.Rect{X: 0, Y: 0, W: 100, H: 40}, Label: "A"}
	b := widget.Button{Bounds: widget.Rect{X: 50, Y: 0, W: 100, H: 40}, Label: "B"}
	// point in overlap: first wins
	if widget.HitFirst(60, 20, a, b) != 0 {
		t.Fatal("expected first hitter")
	}
	if widget.HitFirst(120, 20, a, b) != 1 {
		t.Fatal("expected second hitter")
	}
	if widget.HitFirst(300, 20, a, b) != -1 {
		t.Fatal("expected miss")
	}
	in := widget.Input{Bounds: widget.Rect{X: 0, Y: 100, W: 200, H: 60}, Label: "L"}
	if !in.HitTest(20, 130) {
		t.Fatal("input field hit")
	}
	// label-only area should miss field
	if in.HitTest(20, 105) {
		t.Fatal("label-only should not hit field")
	}
}

func TestW1_DisabledNotVisuallyEmpty(t *testing.T) {
	th := widget.DefaultTheme()
	dc := render.NewContext(160, 60)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 160, 60)
	_ = dc.Fill()
	widget.Button{
		Bounds: widget.Rect{X: 20, Y: 14, W: 120, H: 32},
		Label:  "Off", Style: widget.ButtonPrimary, Disabled: true,
	}.Draw(dc, th)
	r, g, b, a := dc.Image().At(80, 30).RGBA()
	if a == 0 || (r > 60000 && g > 60000 && b > 60000) {
		t.Fatalf("disabled primary should still paint, got %d %d %d %d", r, g, b, a)
	}
}
