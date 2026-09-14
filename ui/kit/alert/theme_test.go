package alert_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Colors must come from theme tokens, never a bare hex in the widget.
func TestAlert_PRD_ALT17_ThemeColors(t *testing.T) {
	tok := theme.Default.Current()
	a := alert.NewAlert("theme")
	a.SetType(alert.AlertInfo)
	bg := a.Background()
	// Info shell is a lightened ColorInfo, not the raw brand nor white.
	info := render.RGBA{R: tok.ColorInfo.R, G: tok.ColorInfo.G, B: tok.ColorInfo.B, A: tok.ColorInfo.A}
	if bg == info {
		t.Fatal("background must be tinted, not raw Theme ColorInfo")
	}
	if bg.R < 0.8 || bg.G < 0.8 {
		t.Fatalf("info bg=%+v want light tint", bg)
	}
	ic := a.IconColor()
	if ic != info && !(ic.R == info.R && ic.G == info.G && ic.B == info.B) {
		t.Fatalf("icon=%+v want Theme ColorInfo %+v", ic, info)
	}
	// Pinning a new Theme moves the shell (proves Token wiring).
	mod := theme.DefaultTokens()
	mod.ColorInfo = theme.Hex("#ff0000")
	a.SetTheme(&mod)
	bg2 := a.Background()
	if bg2 == bg {
		t.Fatal("SetTheme must move Background")
	}
	if bg2.R < 0.99 || bg2.R < bg2.G {
		t.Fatalf("red-tinted bg=%+v want red strongest", bg2)
	}
	// Provider path also moves colors.
	p := theme.NewProvider(theme.DefaultTokens())
	b := alert.NewAlert("p")
	b.SetProvider(p)
	before := b.Background()
	nt := p.Current()
	nt.ColorInfo = theme.Hex("#00aa00")
	p.SetBase(nt)
	if b.Background() == before {
		t.Fatal("SetProvider must move Background")
	}
	// Token spot check: radius/lineWidth track Theme.
	if a.Radius() != tok.RadiusLG || a.LineWidth() != tok.LineWidth {
		t.Fatalf("radius=%v lw=%v want Theme %v/%v", a.Radius(), a.LineWidth(), tok.RadiusLG, tok.LineWidth)
	}
}

// Three themes + RTL snapshot idea: same alert paints under each Theme and
// mirrored placement keeps shell size while swapping icon/close sides.
func TestAlert_ThemeMirror_ThreeThemesRTL(t *testing.T) {
	mk := func() *alert.Alert {
		a := alert.NewAlert("Mirror")
		a.SetDescription("desc")
		a.SetShowIcon(true)
		a.SetClosable(true)
		return a
	}
	light := theme.DefaultTokens()
	dark := theme.DefaultTokens()
	dark.ColorBgContainer = theme.Hex("#141414")
	dark.ColorText = theme.RGBA(255, 255, 255, 0.88)
	dark.ColorInfo = theme.Hex("#1668dc")
	compact := theme.DefaultTokens()
	compact.FontSize = 12
	compact.FontSizeLG = 14
	for i, tk := range []theme.Tokens{light, dark, compact} {
		tk := tk
		a := mk()
		a.SetTheme(&tk)
		// Theme.Token read proves wiring; background must stay opaque light/dark tint.
		bg := a.Background()
		if bg.A < 0.99 {
			t.Fatalf("theme %d bg alpha=%v", i, bg.A)
		}
	}
	// RTL mirrors icon vs close without resizing the shell.
	ltr := mk()
	rtl := mk()
	rtl.SetRTL(true)
	szL := ltr.Layout(rendering.Loose(500, 300))
	szR := rtl.Layout(rendering.Loose(500, 300))
	if szL != szR {
		t.Fatalf("RTL must keep size: %v vs %v", szL, szR)
	}
	if !rtl.IsRTL() || ltr.IsRTL() {
		t.Fatal("IsRTL flag")
	}
}
