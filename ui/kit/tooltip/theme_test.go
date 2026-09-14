package tooltip_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/tooltip"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Colors ride theme Tokens, never a bare literal in the widget.
func TestTooltip_PRD_TIP19_DefaultSkin(t *testing.T) {
	tok := theme.Default.Current()
	tp := tooltip.NewTooltip("theme tip")
	tp.Layout(rendering.Loose(600, 200))
	bg := tp.Background()
	spot := render.RGBA{R: tok.ColorBgSpotlight.R, G: tok.ColorBgSpotlight.G, B: tok.ColorBgSpotlight.B, A: tok.ColorBgSpotlight.A}
	if bg != spot {
		t.Fatalf("default bg=%+v want Theme spotlight %+v", bg, spot)
	}
	prim := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if bg == prim {
		t.Fatal("default skin must not be the primary brand color")
	}
	fg := tp.TextColor()
	white := render.RGBA{R: tok.ColorWhite.R, G: tok.ColorWhite.G, B: tok.ColorWhite.B, A: tok.ColorWhite.A}
	if fg != white {
		t.Fatalf("inverse text=%+v want Theme white %+v", fg, white)
	}
	// Pinning a new Theme moves the bubble (proves Token wiring).
	mod := theme.DefaultTokens()
	mod.ColorBgSpotlight = theme.RGBA(20, 20, 40, 0.9)
	tp.SetTheme(&mod)
	if tp.Background() == bg {
		t.Fatal("SetTheme must move Background")
	}
	// Provider path also moves colors.
	p := theme.NewProvider(theme.DefaultTokens())
	q := tooltip.NewTooltip("provider tip")
	q.SetProvider(p)
	before := q.Background()
	nt := p.Current()
	nt.ColorBgSpotlight = theme.RGBA(40, 20, 20, 0.9)
	p.SetBase(nt)
	if q.Background() == before {
		t.Fatal("SetProvider must move Background")
	}
	// Token spot check: radius/font track Theme.
	if tp.Radius() != tok.Radius || tp.FontSize() != tok.FontSize {
		t.Fatalf("radius=%v font=%v want Theme %v/%v", tp.Radius(), tp.FontSize(), tok.Radius, tok.FontSize)
	}
}

// Three themes + RTL snapshot idea: same tip paints under each Theme and
// mirrored placement keeps bubble size while swapping left/right.
func TestTooltip_ThemeMirror_ThreeThemesRTL(t *testing.T) {
	mk := func() *tooltip.Tooltip {
		tp := tooltip.NewTooltip("Mirror tip")
		tp.SetTriggerLabel("Mirror")
		tp.SetPlacement(tooltip.LeftTop)
		return tp
	}
	light := theme.DefaultTokens()
	dark := theme.DefaultTokens()
	dark.ColorBgContainer = theme.Hex("#141414")
	dark.ColorText = theme.RGBA(255, 255, 255, 0.88)
	dark.ColorBgSpotlight = theme.RGBA(30, 30, 30, 0.95)
	compact := theme.DefaultTokens()
	compact.FontSize = 12
	for i, tk := range []theme.Tokens{light, dark, compact} {
		tk := tk
		tp := mk()
		tp.SetTheme(&tk)
		// Theme.Token read proves wiring; bubble stays opaque.
		bg := tp.Background()
		if bg.A < 0.5 {
			t.Fatalf("theme %d bg alpha=%v", i, bg.A)
		}
		sz := tp.Layout(rendering.Loose(500, 200))
		if sz.Width <= 0 {
			t.Fatalf("theme %d layout=%v", i, sz)
		}
	}
	ltr := mk()
	rtl := mk()
	rtl.SetRTL(true)
	szL := ltr.Layout(rendering.Loose(500, 200))
	szR := rtl.Layout(rendering.Loose(500, 200))
	if szL != szR {
		t.Fatalf("RTL must keep size: %v vs %v", szL, szR)
	}
	if !rtl.IsRTL() || ltr.IsRTL() {
		t.Fatal("IsRTL flag")
	}
	if ltr.EffectivePlacement() == rtl.EffectivePlacement() {
		t.Fatal("RTL must mirror left/right placement")
	}
}
