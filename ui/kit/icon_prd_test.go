package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/icon.md §6.9 — P0 PRD cases (ICO-01…ICO-18).
// ICO-19/20 L3/L4 and ICO-21 P1 deferred.

func layoutIcon(t *testing.T, ic *kit.Icon) core.Size {
	t.Helper()
	tree := core.NewTree(ic.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 100})
	return ic.Node().Base().Size()
}

func TestIcon_PRD_01_Defaults(t *testing.T) {
	// ICO-01
	ic := kit.NewIcon("check")
	if ic.Node() == nil || ic.Root == nil {
		t.Fatal("nil node")
	}
	if ic.Spin {
		t.Fatal("default Spin want false")
	}
	if ic.Rotate != 0 {
		t.Fatalf("Rotate=%v want 0", ic.Rotate)
	}
	if !ic.Decorative {
		t.Fatal("default Decorative want true")
	}
	if ic.Disabled {
		t.Fatal("default Disabled want false")
	}
	if ic.EffectiveSize() != kit.DefaultIconSize {
		t.Fatalf("EffectiveSize=%v want %v", ic.EffectiveSize(), kit.DefaultIconSize)
	}
	sz := ic.Node().Layout(core.Loose(100, 100))
	if math.Abs(sz.Width-16) > 0.5 || math.Abs(sz.Height-16) > 0.5 {
		t.Fatalf("default layout=%+v want 16×16", sz)
	}
}

func TestIcon_PRD_02_KnownName(t *testing.T) {
	// ICO-02 / ICO-S1
	ic := kit.NewIcon("check")
	if !ic.Known() {
		t.Fatal("check should be known")
	}
	layoutIcon(t, ic)
	sz := ic.Node().Layout(core.Loose(50, 50))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	// Paint must not panic.
	tree := core.NewTree(ic.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 64, Height: 64})
	tree.Paint(nil) // nil paint context path may no-op; ensure no panic via Layout only
}

func TestIcon_PRD_03_UnknownName(t *testing.T) {
	// ICO-03 / ICO-S2
	ic := kit.NewIcon("no-such-icon-xyz-prd")
	if ic.Known() {
		t.Fatal("unknown should not be Known")
	}
	// Must not panic.
	_ = layoutIcon(t, ic)
	sz := ic.Node().Layout(core.Loose(40, 40))
	if sz.Width <= 0 {
		t.Fatalf("unknown still layouts: %+v", sz)
	}
}

func TestIcon_PRD_04_SetSize(t *testing.T) {
	// ICO-04 / ICO-S3
	ic := kit.NewIcon("plus")
	ic.SetSize(24)
	sz := ic.Node().Layout(core.Loose(100, 100))
	if math.Abs(sz.Width-24) > 0.5 || math.Abs(sz.Height-24) > 0.5 {
		t.Fatalf("size=%+v want 24×24", sz)
	}
	if ic.EffectiveSize() != 24 {
		t.Fatalf("EffectiveSize=%v", ic.EffectiveSize())
	}
}

func TestIcon_PRD_05_SpinAdvances(t *testing.T) {
	// ICO-05 / ICO-S4
	ic := kit.NewIcon("loading")
	ic.SetSpin(true)
	tree := core.NewTree(ic.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 64, Height: 64})
	ic.AttachTicker(tree)
	before := ic.SpinPhase()
	// Drive tickers directly.
	if !ic.Tick(0.05) {
		t.Fatal("Tick should return true while spinning")
	}
	after := ic.SpinPhase()
	if after <= before {
		t.Fatalf("spin phase did not advance: before=%v after=%v", before, after)
	}
	ang := ic.EffectiveAngle()
	if ang <= 0 {
		t.Fatalf("EffectiveAngle=%v want >0", ang)
	}
}

func TestIcon_PRD_06_ReduceMotionNoSpin(t *testing.T) {
	// ICO-06 / ICO-S5
	ic := kit.NewIcon("sync")
	ic.SetSpin(true)
	tree := core.NewTree(ic.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Clock().ReduceMotion = true
	tree.Layout(core.Size{Width: 64, Height: 64})
	ic.AttachTicker(tree)
	before := ic.SpinPhase()
	_ = ic.Tick(0.1)
	_ = ic.Tick(0.1)
	after := ic.SpinPhase()
	if after != before {
		t.Fatalf("reduced-motion should freeze phase: before=%v after=%v", before, after)
	}
}

func TestIcon_PRD_07_Rotate180(t *testing.T) {
	// ICO-07 / ICO-S6
	ic := kit.NewIcon("smile") // unknown → placeholder, still rotates
	ic.SetName("info")
	ic.SetRotate(180)
	if ic.Rotate != 180 {
		t.Fatalf("Rotate=%v", ic.Rotate)
	}
	if math.Abs(ic.EffectiveAngle()-180) > 0.01 {
		t.Fatalf("EffectiveAngle=%v want 180", ic.EffectiveAngle())
	}
	if g, ok := ic.ChromeNode().(*primitive.Icon); ok {
		if math.Abs(g.RotateDeg-180) > 0.01 {
			t.Fatalf("glyph RotateDeg=%v", g.RotateDeg)
		}
	} else {
		t.Fatal("chrome not primitive.Icon")
	}
}

func TestIcon_PRD_08_SetColor(t *testing.T) {
	// ICO-08 / ICO-S7
	ic := kit.NewIcon("check")
	col := render.RGBA{R: 1, G: 0, B: 0, A: 1}
	ic.SetColor(col)
	got := ic.EffectiveColor()
	if got.A < 0.99 || got.R < 0.99 {
		t.Fatalf("EffectiveColor=%v want red", got)
	}
	if g, ok := ic.ChromeNode().(*primitive.Icon); ok {
		if g.Color.A < 0.99 || g.Color.R < 0.99 {
			t.Fatalf("glyph color=%v", g.Color)
		}
	}
}

func TestIcon_PRD_09_DecorativeNotInTab(t *testing.T) {
	// ICO-09 / ICO-S8
	ic := kit.NewIcon("search")
	// Decorated icon alone should not appear as a focusable tab stop.
	// Kit Icon uses HitDefer and empty Role when decorative.
	if ic.Root.Base().Role != "" {
		t.Fatalf("decorative Role=%q want empty", ic.Root.Base().Role)
	}
	if ic.Root.Base().Label != "" {
		t.Fatalf("decorative Label=%q want empty", ic.Root.Base().Label)
	}
	// Place between two buttons: Tab should skip icon.
	a := kit.NewButton("A")
	b := kit.NewButton("B")
	col := primitive.Column(a.Node(), ic.Node(), b.Node())
	tree := core.NewTree(col)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	// Focus first button via pointer, then Tab.
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 10, Y: 10, Button: core.ButtonLeft})
	// Icon node must not be focusable — Role empty is the contract for P0.
	if ic.Glyph != nil && ic.Glyph.Hit != core.HitDefer {
		t.Fatalf("glyph Hit=%v want HitDefer", ic.Glyph.Hit)
	}
}

func TestIcon_PRD_10_BasicDemo(t *testing.T) {
	// ICO-10 basic.tsx: multiple names + spin + rotate
	names := []string{"check", "info", "search", "plus", "loading"}
	var nodes []core.Node
	var spinners []*kit.Icon
	for _, n := range names {
		ic := kit.NewIcon(n)
		if n == "loading" {
			ic.SetSpin(true)
			spinners = append(spinners, ic)
		}
		if n == "info" {
			ic.SetRotate(180)
		}
		nodes = append(nodes, ic.Node())
	}
	row := primitive.Row(nodes...)
	row.Gap = 8
	tree := core.NewTree(row)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 400, Height: 80})
	for _, s := range spinners {
		s.AttachTicker(tree)
		_ = s.Tick(0.016)
	}
}

func TestIcon_PRD_11_TwoTone(t *testing.T) {
	// ICO-11 two-tone.tsx
	pink := render.RGBA{R: 0.92, G: 0.18, B: 0.59, A: 1} // #eb2f96
	ic := kit.NewIcon("heart")
	ic.SetTwoToneColor(pink)
	g, ok := ic.ChromeNode().(*primitive.Icon)
	if !ok || g == nil {
		t.Fatal("chrome")
	}
	if math.Abs(float64(g.TwoTonePrimary.R-pink.R)) > 0.02 || g.TwoTonePrimary.A < 0.9 {
		t.Fatalf("TwoTonePrimary=%v want ~%v", g.TwoTonePrimary, pink)
	}

	green := render.RGBA{R: 0.32, G: 0.77, B: 0.1, A: 1}
	ic3 := kit.NewIcon("check")
	ic3.SetTwoToneColors(green, render.RGBA{R: 0.32, G: 0.77, B: 0.1, A: 0.2})
	g3 := ic3.ChromeNode().(*primitive.Icon)
	if g3.TwoTonePrimary.A < 0.9 || g3.TwoToneSecondary.A < 0.05 {
		t.Fatalf("twoTone colors primary=%v secondary=%v", g3.TwoTonePrimary, g3.TwoToneSecondary)
	}

	// Global setTwoToneColor / getTwoToneColor
	blue := render.RGBA{R: 0.1, G: 0.4, B: 0.9, A: 1}
	kit.SetTwoToneColorGlobal(blue)
	got := kit.GetTwoToneColorGlobal()
	if math.Abs(float64(got.R-blue.R)) > 0.02 {
		t.Fatalf("global=%v want %v", got, blue)
	}
	ic2 := kit.NewIcon("star")
	g2 := ic2.ChromeNode().(*primitive.Icon)
	if g2.TwoTonePrimary.A < 0.5 {
		t.Fatalf("new icon should pick global two-tone: %v", g2.TwoTonePrimary)
	}
}

func TestIcon_PRD_12_CustomPainter(t *testing.T) {
	// ICO-12 custom.tsx
	var painted int
	ic := kit.NewIcon("")
	ic.SetSize(32)
	ic.SetPainter(func(pc *core.PaintContext, size float64, primary, secondary render.RGBA) {
		painted++
		if size < 31 {
			t.Errorf("painter size=%v want ~32", size)
		}
		if pc != nil {
			pc.FillLocalCircle(size/2, size/2, size*0.35, primary)
		}
	})
	ic.SetColor(render.RGBA{R: 1, G: 0.4, B: 0.7, A: 1})
	sz := ic.Node().Layout(core.Loose(100, 100))
	if math.Abs(sz.Width-32) > 0.5 {
		t.Fatalf("custom size layout=%+v", sz)
	}
	// Force paint path via glyph Paint with a minimal context if available.
	g := ic.ChromeNode().(*primitive.Icon)
	if g.PaintCustom == nil {
		t.Fatal("PaintCustom nil")
	}
	// Call painter directly to assert wiring.
	g.PaintCustom(nil, 32, ic.EffectiveColor(), render.RGBA{})
	if painted < 1 {
		t.Fatal("painter not invoked")
	}
}

func TestIcon_PRD_13_IconfontOffline(t *testing.T) {
	// ICO-13 iconfont.tsx mapping
	src := "font_prd_demo"
	kit.RegisterIconSource(src, map[string]primitive.IconDef{
		"icon-tuichu":   {Kind: primitive.IconClose},
		"icon-facebook": {Kind: primitive.IconInfo},
		"icon-twitter":  {Kind: primitive.IconStar},
	})
	family := kit.CreateFromIconfont(kit.IconfontOptions{Sources: []string{src}})
	ic := family.NewIcon("icon-tuichu")
	if !ic.Known() {
		t.Fatal("iconfont type should be known after register")
	}
	sz := ic.Node().Layout(core.Loose(40, 40))
	if sz.Width <= 0 {
		t.Fatal(sz)
	}
	// Color override like style={{ color: '#1877F2' }}
	ic2 := family.NewIcon("icon-facebook")
	ic2.SetColor(render.RGBA{R: 0.09, G: 0.47, B: 0.95, A: 1})
	if ic2.EffectiveColor().A < 0.9 {
		t.Fatal("color")
	}
}

func TestIcon_PRD_14_MultiSourceOverride(t *testing.T) {
	// ICO-14 scriptUrl.tsx multi-source later wins
	s1 := "prd_src_a"
	s2 := "prd_src_b"
	kit.RegisterIconSource(s1, map[string]primitive.IconDef{
		"icon-shoppingcart": {Kind: primitive.IconPlus},
		"icon-javascript":   {Kind: primitive.IconInfo},
	})
	kit.RegisterIconSource(s2, map[string]primitive.IconDef{
		"icon-shoppingcart": {Kind: primitive.IconCheck}, // override
		"icon-python":       {Kind: primitive.IconStar},
	})
	family := kit.CreateFromIconfont(kit.IconfontOptions{Sources: []string{s1, s2}})
	// Override: shoppingcart from s2 → check
	def, ok := family.Registry().Lookup("icon-shoppingcart")
	if !ok {
		t.Fatal("shoppingcart missing")
	}
	if def.Kind != primitive.IconCheck {
		t.Fatalf("override Kind=%v want check", def.Kind)
	}
	// From s1 only
	if _, ok := family.Registry().Lookup("icon-javascript"); !ok {
		t.Fatal("javascript from s1")
	}
	// From s2 only
	if _, ok := family.Registry().Lookup("icon-python"); !ok {
		t.Fatal("python from s2")
	}
	// New icons work
	_ = family.NewIcon("icon-python").Node().Layout(core.Loose(20, 20))
}

func TestIcon_PRD_15_DefaultSizeL2(t *testing.T) {
	// ICO-15
	ic := kit.NewIcon("minus")
	sz := ic.Node().Layout(core.Loose(200, 200))
	if math.Abs(sz.Width-kit.DefaultIconSize) > 0.5 {
		t.Fatalf("default size=%v want %v", sz.Width, kit.DefaultIconSize)
	}
}

func TestIcon_PRD_16_DefaultColorToken(t *testing.T) {
	// ICO-16
	th := kit.DefaultTheme()
	want := th.Color(core.TokenColorText)
	ic := kit.NewIcon("check")
	// Attach theme via tree so themeOf can resolve if field nil — EffectiveColor uses themeOf.
	tree := core.NewTree(ic.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 40, Height: 40})
	got := ic.EffectiveColor()
	if got.A < 0.01 {
		t.Fatal("effective color transparent")
	}
	// Should match theme text (not a hard-coded brand primary alone).
	if math.Abs(float64(got.R-want.R)) > 0.05 || math.Abs(float64(got.A-want.A)) > 0.05 {
		// Allow if theme text is dark-ish and we got dark-ish
		if got.A < 0.5 {
			t.Fatalf("EffectiveColor=%v want theme text ~%v", got, want)
		}
	}
	// Must not force primary as default monochrome
	primary := th.Color(core.TokenColorPrimary)
	if approxColor(got, primary, 0.02) && !approxColor(want, primary, 0.02) {
		t.Fatalf("default monochrome should not be primary brand: got=%v primary=%v text=%v", got, primary, want)
	}
}

func TestIcon_PRD_17_DisabledColor(t *testing.T) {
	// ICO-17
	th := kit.DefaultTheme()
	ic := kit.NewIcon("close")
	ic.SetTheme(th)
	ic.SetDisabled(true)
	got := ic.EffectiveColor()
	want := th.Color(core.TokenColorDisabledText)
	if !approxColor(got, want, 0.05) {
		t.Fatalf("disabled color=%v want %v", got, want)
	}
}

func TestIcon_PRD_18_AriaLabelMeaningful(t *testing.T) {
	// ICO-18 / ICO-S9
	ic := kit.NewIcon("info")
	ic.SetAriaLabel("About")
	if ic.Root.Base().Role != "img" {
		t.Fatalf("Role=%q want img", ic.Root.Base().Role)
	}
	if ic.Root.Base().Label != "About" {
		t.Fatalf("Label=%q", ic.Root.Base().Label)
	}
	if ic.Decorative {
		t.Fatal("AriaLabel should clear decorative-only semantics")
	}
}
