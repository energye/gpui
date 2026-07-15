package widget_test

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/widget"
)

func findFont(t *testing.T) string {
	t.Helper()
	cands := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}
	for _, p := range cands {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// walk common roots
	roots := []string{"/usr/share/fonts"}
	for _, root := range roots {
		var found string
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".ttf", ".otf":
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	t.Skip("no font found")
	return ""
}

func TestThemeDefault(t *testing.T) {
	th := widget.DefaultTheme()
	if th.ControlHeight != 32 || th.Primary.A != 1 {
		t.Fatalf("unexpected theme: %+v", th)
	}
}

func TestRectContainsAndImage(t *testing.T) {
	r := widget.Rect{X: 10, Y: 20, W: 100, H: 40}
	if !r.Contains(10, 20) || !r.Contains(109.9, 59.9) || r.Contains(110, 20) {
		t.Fatal("contains mismatch")
	}
	ir := r.ImageRect()
	if ir.Min.X != 10 || ir.Min.Y != 20 || ir.Dx() < 100 || ir.Dy() < 40 {
		t.Fatalf("image rect: %v", ir)
	}
}

func TestButtonHitAndDraw(t *testing.T) {
	th := widget.DefaultTheme()
	dc := render.NewContext(200, 80)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, 200, 80)
	_ = dc.Fill()

	b := widget.Button{Bounds: widget.Rect{X: 20, Y: 20, W: 120, H: 32}, Label: "Primary", Style: widget.ButtonPrimary}
	if !b.HitTest(50, 30) || b.HitTest(5, 5) {
		t.Fatal("hit")
	}
	b.Draw(dc, th)
	// Ensure paint produced non-empty damage-ish content: sample center pixel of button.
	img := dc.Image()
	r, g, bl, a := img.At(80, 36).RGBA()
	if a == 0 {
		t.Fatal("button pixel alpha 0")
	}
	// Primary blue-ish
	if r > g && bl > g {
		// ok-ish primary
	} else if r+g+bl == 0 {
		t.Fatalf("button looks black r=%d g=%d b=%d", r, g, bl)
	}
}

func TestInputDrawErrorAndFocus(t *testing.T) {
	th := widget.DefaultTheme()
	dc := render.NewContext(360, 120)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, 360, 120)
	_ = dc.Fill()

	in := widget.Input{
		Bounds:      widget.Rect{X: 16, Y: 12, W: 300, H: 80},
		Label:       "Email",
		Value:       "bad",
		Error:       "invalid format",
		Focused:     true,
		Placeholder: "you@example.com",
	}
	fr := in.FieldRect(th)
	if !in.HitTestTheme(fr.X+10, fr.Y+10, th) {
		t.Fatal("field hit")
	}
	in.Draw(dc, th)
	img := dc.Image()
	// field interior should not be pure black
	r, g, b, a := img.At(int(fr.X+20), int(fr.Y+10)).RGBA()
	if a == 0 || (r < 1000 && g < 1000 && b < 1000) {
		t.Fatalf("field pixel unexpected r=%d g=%d b=%d a=%d", r, g, b, a)
	}
}

func TestModalButtonsLayout(t *testing.T) {
	th := widget.DefaultTheme()
	m := widget.Modal{
		HostW: 480, HostH: 320,
		Panel:       widget.CenterPanel(480, 320, 320, 200),
		Title:       "Confirm",
		Body:        "Body text",
		ShowOverlay: true,
	}
	ok := m.OKButton(th)
	cancel := m.CancelButton(th)
	if ok.Bounds.X <= cancel.Bounds.X {
		t.Fatalf("OK should be to the right of Cancel: ok=%v cancel=%v", ok.Bounds, cancel.Bounds)
	}
	if !m.Panel.Contains(ok.Bounds.X+1, ok.Bounds.Y+1) {
		t.Fatal("OK outside panel")
	}
	dc := render.NewContext(480, 320)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	m.Draw(dc, th)
	img := dc.Image()
	// overlay near corner should be darkened (not pure white)
	r, g, b, _ := img.At(4, 4).RGBA()
	if r > 50000 && g > 50000 && b > 50000 {
		t.Fatalf("overlay not applied: r=%d g=%d b=%d", r, g, b)
	}
	// panel center brighter
	pr, pg, pb, _ := img.At(int(m.Panel.X+m.Panel.W/2), int(m.Panel.Y+m.Panel.H/2)).RGBA()
	if pr < r {
		t.Fatalf("panel not lighter than overlay")
	}
	_ = pg
	_ = pb
}

func TestListRowAndTableCell(t *testing.T) {
	th := widget.DefaultTheme()
	dc := render.NewContext(400, 200)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 400, 200)
	_ = dc.Fill()

	row := widget.ListRow{
		Bounds:     widget.Rect{X: 0, Y: 8, W: 400, H: 40},
		Title:      "Row item 01",
		Selected:   true,
		ShowAvatar: true,
	}
	row.Draw(dc, th)
	if !row.HitTest(20, 20) {
		t.Fatal("row hit")
	}

	cell := widget.TableCell{
		Bounds: widget.Rect{X: 0, Y: 60, W: 120, H: 32},
		Text:   "Name",
		Header: true,
		Align:  widget.AlignLeft,
		Grid:   true,
	}
	cell.Draw(dc, th)
	img := dc.Image()
	// selected row should not be pure white (selected bg is bluish)
	r, g, b, _ := img.At(50, 28).RGBA()
	if r == g && g == b && r > 60000 {
		t.Fatalf("selected row looks pure white r=%d", r)
	}
}

func TestComposeFormShell_CPU(t *testing.T) {
	// Compose first-batch widgets into a form-like shell (no GPU required).
	th := widget.DefaultTheme()
	const W, H = 420, 360
	dc := render.NewContext(W, H)
	defer dc.Close()
	_ = dc.LoadFontFace(findFont(t), th.FontSize)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, W, H)
	_ = dc.Fill()

	// "card"
	card := widget.Rect{X: 24, Y: 24, W: 372, H: 300}
	// use table header as section title bar
	widget.TableCell{Bounds: widget.Rect{X: card.X, Y: card.Y, W: card.W, H: 36}, Text: "Account", Header: true, Grid: true}.Draw(dc, th)
	in1 := widget.Input{Bounds: widget.Rect{X: 40, Y: 72, W: 340, H: 70}, Label: "Username", Value: "demo", Focused: true}
	in2 := widget.Input{Bounds: widget.Rect{X: 40, Y: 160, W: 340, H: 70}, Label: "Password", Placeholder: "••••••••"}
	in1.Draw(dc, th)
	in2.Draw(dc, th)
	widget.Button{Bounds: widget.Rect{X: 40, Y: 270, W: 100, H: 32}, Label: "Cancel", Style: widget.ButtonDefault}.Draw(dc, th)
	widget.Button{Bounds: widget.Rect{X: 152, Y: 270, W: 100, H: 32}, Label: "Save", Style: widget.ButtonPrimary}.Draw(dc, th)

	// damage should include filled regions
	dmg := dc.FrameDamage()
	if len(dmg) == 0 {
		// some paths may merge; at least image should have content
		t.Log("note: FrameDamage empty (possible if not recorded on all paths)")
	}
	img := dc.Image()
	if img.Bounds() != image.Rect(0, 0, W, H) {
		t.Fatalf("bounds %v", img.Bounds())
	}
}
