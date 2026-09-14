package button_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type p1Spec struct {
	PresetColors map[string]string `json:"presetColors"`
	P1           struct {
		Sample   string `json:"autoInsertSample"`
		Spaced   string `json:"autoInsertSpaced"`
		Gradient struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"gradient"`
		HtmlTypes []string `json:"htmlTypes"`
		Nodes     []string `json:"semanticNodes"`
	} `json:"p1"`
}

func loadP1Spec(t *testing.T) p1Spec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "buttons.json"))
	if err != nil {
		t.Fatalf("read buttons.json: %v", err)
	}
	var s p1Spec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse p1 spec: %v", err)
	}
	if len(s.PresetColors) == 0 || s.P1.Sample == "" {
		t.Fatalf("bad p1 spec %+v", s)
	}
	return s
}

func loadLatinFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("P1 true-text needs a system face: %v", err)
	}
	return face
}

func loadCJKFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		t.Skipf("P1 CJK true-text needs a CJK face: %v", err)
	}
	t.Logf("cjk face: %s", desc)
	return face
}

// BTN-25: loading delay + custom icon (delay到期前不转).
func TestButton_PRD_BTN25_LoadingDelay(t *testing.T) {
	b := button.NewButton("确定")
	b.SetLoadingConfig(button.LoadingConfig{Delay: 200 * time.Millisecond, Icon: "custom-spin"})
	if !b.Loading() {
		t.Fatal("loading flag must be on after SetLoadingConfig")
	}
	if b.LoadingDelay() != 200*time.Millisecond {
		t.Fatalf("delay=%v want 200ms", b.LoadingDelay())
	}
	if b.LoadingIcon() != "custom-spin" {
		t.Fatalf("icon=%q want custom-spin", b.LoadingIcon())
	}
	if b.HasSpinner() {
		t.Fatal("spinner must be hidden before the delay elapses")
	}
	if lw := b.ContentWidth(); lw <= 0 {
		t.Fatalf("content width=%v must stay positive while waiting", lw)
	}
	// Clicks swallow both before and after the delay (B-S2).
	n := 0
	b.OnClick = func() { n++ }
	sz := b.Layout(rendering.Loose(1000, 1000))
	if b.PointerDown(sz.Width/2, sz.Height/2) {
		t.Fatal("loading must swallow pointer down before delay")
	}
	time.Sleep(250 * time.Millisecond)
	if !b.HasSpinner() {
		t.Fatal("spinner must show after the delay elapses")
	}
	if b.PointerDown(sz.Width/2, sz.Height/2) {
		t.Fatal("loading must swallow pointer down after delay")
	}
	b.PointerUp(sz.Width/2, sz.Height/2)
	if n != 0 {
		t.Fatalf("loading clicks=%d want 0", n)
	}
	// Separate setters + negative clears.
	c := button.NewButton("确定")
	c.SetLoading(true)
	c.SetLoadingDelay(-5 * time.Millisecond)
	if c.LoadingDelay() != 0 {
		t.Fatalf("negative delay=%v want 0", c.LoadingDelay())
	}
	c.SetLoadingIcon("alt-spin")
	if c.LoadingIcon() != "alt-spin" {
		t.Fatalf("icon=%q", c.LoadingIcon())
	}
	if !c.HasSpinner() {
		t.Fatal("zero-delay loading must spinner immediately")
	}
	c.SetLoading(false)
	if c.HasSpinner() {
		t.Fatal("spinner must clear after SetLoading(false)")
	}
}

// BTN-26: autoInsertSpace two-Han spacing.
func TestButton_PRD_BTN26_AutoInsertSpace(t *testing.T) {
	spec := loadP1Spec(t)
	b := button.NewButton(spec.P1.Sample)
	if !b.AutoInsertSpace() {
		t.Fatal("autoInsertSpace defaults true (5.17.0)")
	}
	if got := b.DisplayLabel(); got != spec.P1.Spaced {
		t.Fatalf("display=%q want %q", got, spec.P1.Spaced)
	}
	wSpaced := b.Layout(rendering.Loose(1000, 1000)).Width
	b.SetAutoInsertSpace(false)
	if b.AutoInsertSpace() {
		t.Fatal("switch must stick")
	}
	if got := b.DisplayLabel(); got != spec.P1.Sample {
		t.Fatalf("off display=%q want %q", got, spec.P1.Sample)
	}
	wPlain := b.Layout(rendering.Loose(1000, 1000)).Width
	if !(wSpaced > wPlain) {
		t.Fatalf("spaced width=%v must exceed plain=%v", wSpaced, wPlain)
	}
	// Non two-Han labels never insert.
	for _, s := range []string{"确定好", "确", "OK", "确 定"} {
		x := button.NewButton(s)
		if x.DisplayLabel() != s {
			t.Fatalf("%q display=%q want unchanged", s, x.DisplayLabel())
		}
	}
}

// BTN-27: wave close + reduced-motion kills the ripple.
func TestButton_PRD_BTN27_WaveDisabled(t *testing.T) {
	b := button.NewButton("确定")
	if b.WaveDisabled() || b.ReducedMotion() {
		t.Fatal("wave defaults: enabled, motion on")
	}
	if !b.WaveActive() {
		t.Fatal("default wave must be active")
	}
	b.SetWaveDisabled(true)
	if !b.WaveDisabled() || b.WaveActive() {
		t.Fatal("WaveDisabled(true) must deactivate the ripple")
	}
	b.SetWaveDisabled(false)
	b.SetReducedMotion(true)
	if !b.ReducedMotion() || b.WaveActive() {
		t.Fatal("reduced-motion must deactivate the ripple")
	}
	// Pixel proof: pressed wave paints an outer ring, closed does not.
	paint := func(waveOff bool) []uint8 {
		x := button.NewButton("确定")
		x.SetType(button.ButtonPrimary)
		x.SetWaveDisabled(waveOff)
		sz := x.Layout(rendering.Loose(1000, 1000))
		x.PointerDown(sz.Width/2, sz.Height/2)
		W, H := int(sz.Width)+12, int(sz.Height)+12
		dc := render.NewContext(W, H)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		x.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(6, 6))
		img := dc.Image()
		var out []uint8
		for y := 0; y < H; y += 2 {
			for xx := 0; xx < W; xx += 2 {
				r, g, bl, _ := img.At(xx, y).RGBA()
				out = append(out, uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			}
		}
		return out
	}
	on, off := paint(false), paint(true)
	if len(on) != len(off) {
		t.Fatal("wave snapshot size mismatch")
	}
	diff := 0
	for i := range on {
		if on[i] != off[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("wave on/off snapshots identical: ripple missing")
	}
}

// P1 PresetColors full palette (13 hues, file-driven).
func TestButton_P1_PresetColors(t *testing.T) {
	spec := loadP1Spec(t)
	if len(spec.PresetColors) != 13 {
		t.Fatalf("preset count=%d want 13", len(spec.PresetColors))
	}
	seen := map[render.RGBA]string{}
	for name := range spec.PresetColors {
		b := button.NewButton("确定")
		b.SetVariant(button.VariantSolid)
		b.SetColor(button.ButtonColor(name))
		f := b.Fill()
		if f.A == 0 {
			t.Fatalf("%s solid fill transparent", name)
		}
		if prev, dup := seen[f]; dup {
			// Pink is the deprecated alias of magenta (presetPrimaryColors
			// pink = magenta): duplicate fill is official, not a bug.
			if !((name == "pink" && prev == "magenta") || (name == "magenta" && prev == "pink")) {
				t.Fatalf("%s fill duplicates %s (%+v)", name, prev, f)
			}
		}
		seen[f] = name
		// Outlined preset tints the border too.
		o := button.NewButton("确定")
		o.SetVariant(button.VariantOutlined)
		o.SetColor(button.ButtonColor(name))
		if o.BorderColor().A == 0 {
			t.Fatalf("%s outlined border transparent", name)
		}
	}
}

// P1 success/warning map to theme tokens.
func TestButton_P1_SuccessWarning(t *testing.T) {
	tok := theme.Default.Current()
	s := button.NewButton("确定")
	s.SetVariant(button.VariantSolid)
	s.SetColor(button.ColorSuccess)
	wantS := render.RGBA{R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: tok.ColorSuccess.A}
	if got := s.Fill(); got != wantS {
		t.Fatalf("success fill=%+v want %+v", got, wantS)
	}
	w := button.NewButton("确定")
	w.SetVariant(button.VariantSolid)
	w.SetColor(button.ColorWarning)
	wantW := render.RGBA{R: tok.ColorWarning.R, G: tok.ColorWarning.G, B: tok.ColorWarning.B, A: tok.ColorWarning.A}
	if got := w.Fill(); got != wantW {
		t.Fatalf("warning fill=%+v want %+v", got, wantW)
	}
	// Explicit preset color wins over the danger sugar (§6.3).
	p := button.NewButton("确定")
	p.SetVariant(button.VariantSolid)
	p.SetColor(button.ColorBlue)
	p.SetDanger(true)
	if got := p.Fill(); got == (render.RGBA{R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A}) {
		t.Fatal("explicit preset must win over danger flag")
	}
}

// P1 href/target/htmlType desktop mapping.
func TestButton_P1_HrefTargetHtmlType(t *testing.T) {
	spec := loadP1Spec(t)
	if len(spec.P1.HtmlTypes) != 3 {
		t.Fatalf("htmlTypes=%v want 3", spec.P1.HtmlTypes)
	}
	b := button.NewButton("确定")
	if b.HtmlTypeName() != button.HtmlButton || b.IsLink() {
		t.Fatalf("defaults html=%s link=%v", b.HtmlTypeName(), b.IsLink())
	}
	b.SetHref("https://ant.design/components/button")
	b.SetTarget("_blank")
	if !b.IsLink() || b.Href() != "https://ant.design/components/button" || b.Target() != "_blank" {
		t.Fatalf("href=%q target=%q link=%v", b.Href(), b.Target(), b.IsLink())
	}
	b.SetHtmlType(button.HtmlSubmit)
	if b.HtmlTypeName() != button.HtmlSubmit {
		t.Fatalf("html=%s", b.HtmlTypeName())
	}
	b.SetHtmlType("bogus")
	if b.HtmlTypeName() != button.HtmlButton {
		t.Fatalf("bogus html=%s want button", b.HtmlTypeName())
	}
	// Click still fires OnClick, then offers OnNavigate exactly once.
	clicks, navs := 0, 0
	b.OnClick = func() { clicks++ }
	b.OnNavigate = func(href, target string) {
		navs++
		if href != "https://ant.design/components/button" || target != "_blank" {
			t.Errorf("navigate(%q,%q)", href, target)
		}
	}
	sz := b.Layout(rendering.Loose(1000, 1000))
	b.PointerDown(sz.Width/2, sz.Height/2)
	b.PointerUp(sz.Width/2, sz.Height/2)
	if clicks != 1 || navs != 1 {
		t.Fatalf("clicks=%d navs=%d want 1/1", clicks, navs)
	}
	// Plain button has no navigation.
	p := button.NewButton("确定")
	pn := 0
	p.OnNavigate = func(_, _ string) { pn++ }
	pc := 0
	p.OnClick = func() { pc++ }
	psz := p.Layout(rendering.Loose(1000, 1000))
	p.PointerDown(psz.Width/2, psz.Height/2)
	p.PointerUp(psz.Width/2, psz.Height/2)
	if pc != 1 || pn != 0 {
		t.Fatalf("plain clicks=%d navs=%d want 1/0", pc, pn)
	}
}

// P1 classNames/styles semantic hooks.
func TestButton_P1_SemanticHooks(t *testing.T) {
	spec := loadP1Spec(t)
	if len(spec.P1.Nodes) != 3 {
		t.Fatalf("semantic nodes=%v want 3", spec.P1.Nodes)
	}
	b := button.NewButton("确定")
	b.SetClassName(button.SemanticRoot, "my-btn")
	b.SetClassNames(map[button.SemanticKey]string{button.SemanticRoot: "my-btn", button.SemanticLabel: "my-label"})
	if b.ClassName(button.SemanticRoot) != "my-btn" || b.ClassName(button.SemanticLabel) != "my-label" {
		t.Fatalf("classNames root=%q label=%q", b.ClassName(button.SemanticRoot), b.ClassName(button.SemanticLabel))
	}
	before := b.Fill()
	beforeText := b.TextColor()
	b.SetSemanticStyle(button.SemanticRoot, button.Style{Bg: render.RGBA{R: 0.1, G: 0.2, B: 0.3, A: 1}, UseBg: true})
	b.SetSemanticStyle(button.SemanticLabel, button.Style{Text: render.RGBA{R: 0.9, G: 0.1, B: 0.1, A: 1}, UseText: true})
	b.SetSemanticStyle(button.SemanticIcon, button.Style{Text: render.RGBA{R: 0.1, G: 0.8, B: 0.1, A: 1}, UseText: true})
	if b.Fill() == before {
		t.Fatal("root semantic Bg must move the fill")
	}
	if b.TextColor() == beforeText {
		t.Fatal("label semantic Text must move the label ink")
	}
	x := button.NewButton("确定")
	x.SetIcon("search")
	if got := x.IconColor(); got != x.TextColor() {
		t.Fatalf("default icon ink must follow text: %+v vs %+v", got, x.TextColor())
	}
	x.SetSemanticStyle(button.SemanticIcon, button.Style{Text: render.RGBA{R: 0, G: 1, B: 0, A: 1}, UseText: true})
	if got := x.IconColor(); got.G != 1 {
		t.Fatalf("icon hook=%+v want green", got)
	}
	// Hooks never move layout.
	y := button.NewButton("确定")
	a := y.Layout(rendering.Loose(1000, 1000))
	y.SetClassName(button.SemanticRoot, "late")
	y.SetSemanticStyle(button.SemanticRoot, button.Style{Bg: render.RGBA{R: 0, G: 0, B: 0, A: 1}, UseBg: true})
	after := y.Layout(rendering.Loose(1000, 1000))
	if a != after {
		t.Fatalf("semantic hooks moved layout %v -> %v", a, after)
	}
}

// P1 ConfigProvider global defaults (button-owned staging).
func TestButton_P1_GlobalDefaults(t *testing.T) {
	button.ResetGlobalConfig()
	defer button.ResetGlobalConfig()
	button.SetGlobalConfig(button.GlobalConfig{
		Size:            button.ButtonSmall,
		Variant:         button.VariantFilled,
		Color:           button.ColorBlue,
		AutoInsertSpace: &[]bool{false}[0],
		WaveDisabled:    &[]bool{true}[0],
		LoadingIcon:     "global-spin",
		HasLoadingIcon:  true,
		HtmlType:        button.HtmlSubmit,
		HasHtmlType:     true,
	})
	b := button.NewButton("确定")
	if b.Size() != button.ButtonSmall {
		t.Fatalf("global size=%s", b.Size())
	}
	if b.EffectiveVariant() != button.VariantFilled {
		t.Fatalf("global variant=%s", b.EffectiveVariant())
	}
	if b.ColorName() != button.ColorBlue {
		t.Fatalf("global color=%s", b.ColorName())
	}
	if b.AutoInsertSpace() || !b.WaveDisabled() {
		t.Fatal("global bools must apply")
	}
	if b.LoadingIcon() != "global-spin" || b.HtmlTypeName() != button.HtmlSubmit {
		t.Fatalf("global icon=%q html=%s", b.LoadingIcon(), b.HtmlTypeName())
	}
	// Explicit props win over globals.
	b.SetSize(button.ButtonLarge)
	b.SetColor(button.ColorGold)
	if b.Size() != button.ButtonLarge || b.ColorName() != button.ColorGold {
		t.Fatal("explicit must win over global")
	}
}

// P1 gradient background (linear-gradient demo hook).
func TestButton_P1_Gradient(t *testing.T) {
	spec := loadP1Spec(t)
	from := theme.Hex(spec.P1.Gradient.From)
	to := theme.Hex(spec.P1.Gradient.To)
	fx := render.RGBA{R: from.R, G: from.G, B: from.B, A: from.A}
	tx := render.RGBA{R: to.R, G: to.G, B: to.B, A: to.A}
	b := button.NewButton("渐变")
	b.SetType(button.ButtonPrimary)
	b.SetGradient(fx, tx)
	if !b.HasGradient() {
		t.Fatal("gradient must stick")
	}
	if f, tt, ok := b.GradientColors(); !ok || f != fx || tt != tx {
		t.Fatalf("stops=%+v/%+v ok=%v", f, tt, ok)
	}
	// Pixel proof: gradient ends differ, solid middle does not.
	sz := b.Layout(rendering.Loose(1000, 1000))
	paint := func(x *button.Button) (l, r uint32) {
		dc := render.NewContext(int(sz.Width), int(sz.Height))
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		x.Node().Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		lr, lg, lb, _ := img.At(4, int(sz.Height)/2).RGBA()
		rr, rg, rb, _ := img.At(int(sz.Width)-5, int(sz.Height)/2).RGBA()
		return lr/257<<16 | lg/257<<8 | lb/257, rr/257<<16 | rg/257<<8 | rb/257
	}
	l, r := paint(b)
	if l == r {
		t.Fatalf("gradient ends identical #%06x (no blend?)", l)
	}
	plain := button.NewButton("渐变")
	plain.SetType(button.ButtonPrimary)
	plain.Layout(rendering.Loose(1000, 1000))
	pl, pr := paint(plain)
	if pl != pr {
		t.Fatalf("solid ends #%06x vs #%06x want equal", pl, pr)
	}
	b.ClearGradient()
	if b.HasGradient() {
		t.Fatal("ClearGradient must clear")
	}
}

// True-text chain: headless estimates width with no ink; with a face there is ink.
// Layout/Node contract: New() already intrinsic, Node() never 0.
func TestButton_P1_TrueText(t *testing.T) {
	face := loadLatinFace(t)
	countDark := func(b *button.Button) int {
		sz := b.Layout(rendering.Loose(1000, 1000))
		dc := render.NewContext(int(sz.Width), int(sz.Height))
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		b.Node().Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		dark := 0
		for y := 0; y < int(sz.Height); y++ {
			for x := 0; x < int(sz.Width); x++ {
				rr, gg, bb, _ := img.At(x, y).RGBA()
				if rr/257 < 110 && gg/257 < 110 && bb/257 < 110 {
					dark++
				}
			}
		}
		return dark
	}
	plain := button.NewButton("Hello")
	if n := countDark(plain); n != 0 {
		t.Fatalf("no-face dark=%d want 0 (black bar banned)", n)
	}
	with := button.NewButton("Hello")
	with.SetTextFace(face)
	if n := countDark(with); n < 30 {
		t.Fatalf("with-face dark=%d want >=30 (real glyphs missing)", n)
	}
	with.SetTextFace(nil)
	if n := countDark(with); n != 0 {
		t.Fatalf("cleared face dark=%d want 0", n)
	}
	// Node() directly after New() already has size.
	nb := button.NewButton("确定")
	ns := nb.Node().Size()
	if ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("Node size=%v want >0 right after New", ns)
	}
	if lo := nb.LaidOut(); lo.Width <= 0 || lo.Height <= 0 {
		t.Fatalf("LaidOut=%v want >0", lo)
	}
}

// CJK true-text needs a CJK-capable face; Skip clearly when absent.
func TestButton_P1_TrueTextCJK(t *testing.T) {
	face := loadCJKFace(t)
	b := button.NewButton("确定")
	b.SetTextFace(face)
	sz := b.Layout(rendering.Loose(1000, 1000))
	dc := render.NewContext(int(sz.Width), int(sz.Height))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	b.Node().Paint(rendering.NewPaintContext(dc, 1))
	img := dc.Image()
	dark := 0
	for y := 0; y < int(sz.Height); y++ {
		for x := 0; x < int(sz.Width); x++ {
			rr, gg, bb, _ := img.At(x, y).RGBA()
			if rr/257 < 110 && gg/257 < 110 && bb/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("CJK dark=%d want >=30", dark)
	}
}

// Deprecated iconPosition alias still mirrors placement.
func TestButton_P1_IconPositionAlias(t *testing.T) {
	b := button.NewButton("确定")
	b.SetIcon("search")
	b.SetIconPosition(button.IconPositionEnd)
	if b.IconPlacement() != button.IconEnd || b.IconPosition() != button.IconEnd {
		t.Fatalf("alias placement=%s", b.IconPlacement())
	}
}

// BTN-22 human-eye side-by-side needs a reviewer: documented Skip.
func TestButton_PRD_BTN22_HumanEyeNA(t *testing.T) {
	t.Skip("L4 BTN-22 needs human side-by-side sign-off against ant.design; no automated assertion")
}
