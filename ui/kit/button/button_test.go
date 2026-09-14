package button_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type sizeRow struct {
	Size          string  `json:"size"`
	Height        float64 `json:"height"`
	FontSize      float64 `json:"fontSize"`
	PaddingInline float64 `json:"paddingInline"`
	Radius        float64 `json:"radius"`
	Icon          float64 `json:"icon"`
	Gap           float64 `json:"gap"`
}

type specFile struct {
	Source        string    `json:"source"`
	Sizes         []sizeRow `json:"sizes"`
	MinTarget     float64   `json:"minTarget"`
	ContrastFloor float64   `json:"contrastFloor"`
	P0Cases       []string  `json:"p0Cases"`
	ColorMatrix   []struct {
		Color   string `json:"color"`
		Variant string `json:"variant"`
	} `json:"colorMatrix"`
}

func loadSpec(t *testing.T) specFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "buttons.json"))
	if err != nil {
		t.Fatalf("read buttons.json: %v", err)
	}
	var f specFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse buttons.json: %v", err)
	}
	if len(f.Sizes) != 3 || len(f.P0Cases) == 0 {
		t.Fatalf("bad spec %+v", f)
	}
	return f
}

func sizeOf(f specFile, name string) sizeRow {
	for _, s := range f.Sizes {
		if s.Size == name {
			return s
		}
	}
	return sizeRow{}
}

// click fires one in-bounds press-release through the pointer path.
func click(b *button.Button) {
	sz := b.Layout(rendering.Loose(1000, 1000))
	b.PointerDown(sz.Width/2, sz.Height/2)
	b.PointerUp(sz.Width/2, sz.Height/2)
}

func TestButton_PRD_BTN01(t *testing.T) {
	b := button.NewButton("确定")
	if b.Type() != button.ButtonDefault || b.Size() != button.ButtonMiddle {
		t.Fatalf("defaults type=%s size=%s", b.Type(), b.Size())
	}
	if b.EffectiveVariant() != button.VariantOutlined {
		t.Fatalf("default variant=%s want outlined", b.EffectiveVariant())
	}
	if b.Role() != "button" || b.AriaName() != "确定" {
		t.Fatalf("role=%s aria=%s", b.Role(), b.AriaName())
	}
	n := 0
	b.OnClick = func() { n++ }
	click(b)
	if n != 1 {
		t.Fatalf("clicks=%d want 1", n)
	}
}

func TestButton_PRD_BTN02(t *testing.T) {
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	if b.EffectiveVariant() != button.VariantSolid {
		t.Fatalf("primary variant=%s want solid", b.EffectiveVariant())
	}
	n := 0
	b.OnClick = func() { n++ }
	click(b)
	if n != 1 {
		t.Fatalf("clicks=%d want 1", n)
	}
}

func TestButton_PRD_BTN03(t *testing.T) {
	b := button.NewButton("确定")
	b.SetDisabled(true)
	n := 0
	b.OnClick = func() { n++ }
	sz := b.Layout(rendering.Loose(1000, 1000))
	if b.PointerDown(sz.Width/2, sz.Height/2) {
		t.Fatal("disabled must swallow pointer down")
	}
	b.PointerUp(sz.Width/2, sz.Height/2)
	// Keyboard: disabled node cannot take focus, manager has no primary.
	mgr := focus.NewManager()
	mgr.Register(b.FocusNode())
	if b.FocusNode().RequestFocus() {
		t.Fatal("disabled must not focus")
	}
	mgr.HandleKey(focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true})
	if n != 0 {
		t.Fatalf("disabled clicks=%d want 0", n)
	}
}

func TestButton_PRD_BTN04(t *testing.T) {
	b := button.NewButton("确定")
	b.SetLoading(true)
	if !b.HasSpinner() {
		t.Fatal("loading must show spinner")
	}
	n := 0
	b.OnClick = func() { n++ }
	sz := b.Layout(rendering.Loose(1000, 1000))
	if b.PointerDown(sz.Width/2, sz.Height/2) {
		t.Fatal("loading must swallow pointer down")
	}
	b.PointerUp(sz.Width/2, sz.Height/2)
	if n != 0 {
		t.Fatalf("loading clicks=%d want 0", n)
	}
	// Loading stacked with disabled still swallows.
	b.SetDisabled(true)
	click(b)
	if n != 0 {
		t.Fatalf("loading+disabled clicks=%d want 0", n)
	}
}

func TestButton_PRD_BTN05(t *testing.T) {
	b := button.NewButton("确定")
	n := 0
	b.OnClick = func() { n++ }
	sz := b.Layout(rendering.Loose(1000, 1000))
	cx, cy := sz.Width/2, sz.Height/2
	if !b.PointerDown(cx, cy) {
		t.Fatal("down inside should press")
	}
	b.PointerMove(sz.Width+100, cy) // drag out of bounds
	if b.PointerUp(sz.Width+100, cy) {
		t.Fatal("release outside must not click")
	}
	if n != 0 {
		t.Fatalf("out-release clicks=%d want 0", n)
	}
}

func TestButton_PRD_BTN06(t *testing.T) {
	b := button.NewButton("确定")
	n := 0
	b.OnClick = func() { n++ }
	mgr := focus.NewManager()
	mgr.Register(b.FocusNode())
	if !b.FocusNode().RequestFocus() {
		t.Fatal("button should take focus")
	}
	if !b.Focused() {
		t.Fatal("focused flag must follow manager")
	}
	mgr.HandleKey(focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true})
	mgr.HandleKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true})
	if n != 2 {
		t.Fatalf("keyboard clicks=%d want 2 (space+enter)", n)
	}
}

func TestButton_PRD_BTN07(t *testing.T) {
	f := loadSpec(t)
	cases := map[string]button.ButtonSize{"small": button.ButtonSmall, "middle": button.ButtonMiddle, "large": button.ButtonLarge}
	for _, row := range f.Sizes {
		b := button.NewButton("确定")
		b.SetSize(cases[row.Size])
		if math.Abs(b.Height()-row.Height) > 0.5 {
			t.Fatalf("%s height=%v want %v", row.Size, b.Height(), row.Height)
		}
		sz := b.Layout(rendering.Loose(1000, 1000))
		if math.Abs(sz.Height-row.Height) > 0.5 {
			t.Fatalf("%s layout height=%v want %v", row.Size, sz.Height, row.Height)
		}
		if math.Abs(b.FontSize()-row.FontSize) > 1e-9 {
			t.Fatalf("%s font=%v want %v", row.Size, b.FontSize(), row.FontSize)
		}
	}
}

func TestButton_PRD_BTN08(t *testing.T) {
	f := loadSpec(t)
	mid := sizeOf(f, "middle")
	b := button.NewButton("确定")
	sz := b.Layout(rendering.Loose(1000, 1000))
	pad := (sz.Width - b.ContentWidth()) / 2
	if math.Abs(pad-mid.PaddingInline) > 1.0 {
		t.Fatalf("middle padding=%v want ~%v", pad, mid.PaddingInline)
	}
}

func TestButton_PRD_BTN09(t *testing.T) {
	tok := theme.Default.Current()
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	want := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if got := b.Fill(); got != want {
		t.Fatalf("primary fill=%+v want %+v", got, want)
	}
	inv := render.RGBA{R: tok.OnPrimary.R, G: tok.OnPrimary.G, B: tok.OnPrimary.B, A: tok.OnPrimary.A}
	if got := b.TextColor(); got != inv {
		t.Fatalf("primary text=%+v want inverse %+v", got, inv)
	}
}

func TestButton_PRD_BTN10(t *testing.T) {
	tok := theme.Default.Current()
	b := button.NewButton("确定")
	if !b.HasBorder() || b.DashedBorder() {
		t.Fatal("default outlined must have a solid border")
	}
	want := render.RGBA{R: tok.ColorBgContainer.R, G: tok.ColorBgContainer.G, B: tok.ColorBgContainer.B, A: tok.ColorBgContainer.A}
	if got := b.Fill(); got != want {
		t.Fatalf("default fill=%+v want container %+v", got, want)
	}
	prim := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if got := b.Fill(); got == prim {
		t.Fatal("default must not sit on the primary fill")
	}
}

func TestButton_PRD_BTN11(t *testing.T) {
	b := button.NewButton("确定")
	b.SetType(button.ButtonDashed)
	if !b.DashedBorder() {
		t.Fatal("dashed type must report a dashed border")
	}
	if b.EffectiveVariant() != button.VariantDashed {
		t.Fatalf("variant=%s want dashed", b.EffectiveVariant())
	}
}

func TestButton_PRD_BTN12(t *testing.T) {
	for _, typ := range []button.ButtonType{button.ButtonText, button.ButtonLink} {
		b := button.NewButton("确定")
		b.SetType(typ)
		if b.Fill().A != 0 || b.HasBorder() {
			t.Fatalf("%s must be borderless transparent (fill=%+v)", typ, b.Fill())
		}
	}
}

func TestButton_PRD_BTN13(t *testing.T) {
	tok := theme.Default.Current()
	errCol := render.RGBA{R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A}
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	b.SetDanger(true)
	if got := b.Fill(); got != errCol {
		t.Fatalf("danger fill=%+v want error %+v", got, errCol)
	}
	c := button.NewButton("确定")
	c.SetColor(button.ColorDanger)
	if got := c.BorderColor(); got != errCol {
		t.Fatalf("color=danger outlined border=%+v want error %+v", got, errCol)
	}
}

func TestButton_PRD_BTN14(t *testing.T) {
	tok := theme.Default.Current()
	b := button.NewButton("确定")
	b.SetGhost(true)
	if b.Fill().A != 0 {
		t.Fatalf("ghost fill alpha=%v want 0", b.Fill().A)
	}
	white := render.RGBA{R: tok.ColorWhite.R, G: tok.ColorWhite.G, B: tok.ColorWhite.B, A: tok.ColorWhite.A}
	if got := b.TextColor(); got != white {
		t.Fatalf("ghost text=%+v want %+v", got, white)
	}
	if got := b.BorderColor(); got != white {
		t.Fatalf("ghost border=%+v want %+v", got, white)
	}
}

func TestButton_PRD_BTN15(t *testing.T) {
	f := loadSpec(t)
	mid := sizeOf(f, "middle")
	b := button.NewButton("确定")
	b.SetBlock(true)
	sz := b.Layout(rendering.Loose(400, 100))
	if math.Abs(sz.Width-400) > 0.5 {
		t.Fatalf("block width=%v want 400", sz.Width)
	}
	if math.Abs(sz.Height-mid.Height) > 0.5 {
		t.Fatalf("block height=%v want %v", sz.Height, mid.Height)
	}
}

func TestButton_PRD_BTN16(t *testing.T) {
	f := loadSpec(t)
	mid := sizeOf(f, "middle")
	b := button.NewButton("")
	b.SetIcon("search")
	b.SetShape(button.ButtonShapeCircle)
	sz := b.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-sz.Height) > 0.5 {
		t.Fatalf("circle %vx%v must be square", sz.Width, sz.Height)
	}
	if math.Abs(sz.Height-mid.Height) > 0.5 {
		t.Fatalf("circle edge=%v want %v", sz.Height, mid.Height)
	}
}

func TestButton_PRD_BTN17(t *testing.T) {
	b := button.NewButton("确定")
	b.SetShape(button.ButtonShapeRound)
	sz := b.Layout(rendering.Loose(1000, 1000))
	if math.Abs(b.Radius()-sz.Height/2) > 1e-9 {
		t.Fatalf("round radius=%v want h/2=%v", b.Radius(), sz.Height/2)
	}
}

func TestButton_PRD_BTN18(t *testing.T) {
	b := button.NewButton("确定")
	b.SetIcon("search")
	b.Layout(rendering.Loose(1000, 1000))
	if !b.IconFirst() {
		t.Fatal("start placement must lead with icon (LTR)")
	}
	if !(b.LeadCenterX() < b.TextCenterX()) {
		t.Fatalf("lead %v must precede text %v", b.LeadCenterX(), b.TextCenterX())
	}
	b.SetIconPlacement(button.IconEnd)
	b.Layout(rendering.Loose(1000, 1000))
	if b.IconFirst() {
		t.Fatal("end placement must trail with icon (LTR)")
	}
	if !(b.TextCenterX() < b.LeadCenterX()) {
		t.Fatalf("text %v must precede lead %v", b.TextCenterX(), b.LeadCenterX())
	}
}

func TestButton_PRD_BTN19(t *testing.T) {
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	b.SetVariant(button.VariantOutlined)
	if b.EffectiveVariant() != button.VariantOutlined {
		t.Fatalf("explicit variant must win: %s", b.EffectiveVariant())
	}
	if b.Fill().A == 0 || b.BorderColor().A == 0 {
		t.Fatal("outlined override must paint container + border")
	}
}

func TestButton_PRD_BTN20(t *testing.T) {
	f := loadSpec(t)
	tok := theme.Default.Current()
	prim := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	errCol := render.RGBA{R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A}
	cont := render.RGBA{R: tok.ColorBgContainer.R, G: tok.ColorBgContainer.G, B: tok.ColorBgContainer.B, A: tok.ColorBgContainer.A}
	for _, row := range f.ColorMatrix {
		b := button.NewButton("确定")
		b.SetColor(button.ButtonColor(row.Color))
		switch button.ButtonVariant(row.Variant) {
		case button.VariantSolid:
			b.SetVariant(button.VariantSolid)
		case button.VariantOutlined:
			b.SetVariant(button.VariantOutlined)
		}
		switch row.Color {
		case "primary":
			got := b.Fill()
			if row.Variant == "solid" && got != prim {
				t.Fatalf("primary/solid fill=%+v", got)
			}
			if row.Variant == "outlined" && b.BorderColor() != prim {
				t.Fatalf("primary/outlined border=%+v", b.BorderColor())
			}
		case "danger":
			got := b.Fill()
			if row.Variant == "solid" && got != errCol {
				t.Fatalf("danger/solid fill=%+v", got)
			}
			if row.Variant == "outlined" && b.BorderColor() != errCol {
				t.Fatalf("danger/outlined border=%+v", b.BorderColor())
			}
		case "default":
			if row.Variant == "solid" && b.Fill().A != 1 {
				t.Fatalf("default/solid must be opaque: %+v", b.Fill())
			}
			if row.Variant == "outlined" && (b.Fill() != cont || !b.HasBorder()) {
				t.Fatalf("default/outlined=%+v", b.Fill())
			}
		}
	}
}

func TestButton_PRD_BTN23(t *testing.T) {
	b := button.NewButton("")
	b.SetIcon("search")
	if b.HasAccessibleName() {
		t.Fatal("icon-only without label must fail the a11y name check")
	}
	b.SetAriaLabel("search")
	if !b.HasAccessibleName() || b.AriaName() != "search" || b.Role() != "button" {
		t.Fatalf("aria=%q role=%q", b.AriaName(), b.Role())
	}
}

func TestButton_PRD_BTN24(t *testing.T) {
	tok := theme.Default.Current()
	disBg := render.RGBA{R: tok.ColorFillTertiary.R, G: tok.ColorFillTertiary.G, B: tok.ColorFillTertiary.B, A: tok.ColorFillTertiary.A}
	disTx := render.RGBA{R: tok.ColorTextDisabled.R, G: tok.ColorTextDisabled.G, B: tok.ColorTextDisabled.B, A: tok.ColorTextDisabled.A}
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	b.SetDisabled(true)
	if got := b.Fill(); got != disBg {
		t.Fatalf("disabled fill=%+v want %+v", got, disBg)
	}
	if got := b.TextColor(); got != disTx {
		t.Fatalf("disabled text=%+v want %+v", got, disTx)
	}
	// Hover must not restyle a disabled button.
	before := b.Fill()
	b.PointerMove(5, 5)
	if got := b.Fill(); got != before {
		t.Fatalf("disabled hover changed fill %+v -> %+v", before, got)
	}
}

// Layout matrix: Exact, small-Max (clamp) and large-Max (intrinsic) each
// assert size without GPU.
func TestButton_LayoutMatrix(t *testing.T) {
	b := button.NewButton("确定")
	exact := b.Layout(rendering.Tight(200, 50))
	if exact.Width != 200 || exact.Height != 50 {
		t.Fatalf("exact=%+v want 200x50", exact)
	}
	b2 := button.NewButton("确定")
	big := b2.Layout(rendering.Loose(1000, 1000))
	if math.Abs(big.Height-32) > 0.5 || big.Width <= 0 || big.Width > 1000 {
		t.Fatalf("max=%+v want intrinsic ~66x32 (spaced)", big)
	}
	b3 := button.NewButton("确定")
	small := b3.Layout(rendering.Loose(40, 10))
	if math.Abs(small.Width-40) > 1e-9 || math.Abs(small.Height-10) > 1e-9 {
		t.Fatalf("min-clamp=%+v want 40x10", small)
	}
}

// Accessibility: focus reachability, role/name, 44px hit floor, contrast.
func TestButton_A11y(t *testing.T) {
	f := loadSpec(t)
	b := button.NewButton("确定")
	if !b.Focusable() {
		t.Fatal("enabled button must be focusable")
	}
	mgr := focus.NewManager()
	other := focus.NewFocusNode("other")
	mgr.Register(other)
	mgr.Register(b.FocusNode())
	mgr.FocusNext()
	if mgr.Primary() != other {
		t.Fatalf("tab order primary=%v", mgr.Primary())
	}
	mgr.FocusNext()
	if mgr.Primary() != b.FocusNode() || !b.Focused() {
		t.Fatal("tab must reach the button with a visible ring flag")
	}
	if b.Role() != "button" || b.AriaName() != "确定" {
		t.Fatalf("role=%s aria=%s", b.Role(), b.AriaName())
	}
	sz := b.Layout(rendering.Loose(1000, 1000))
	_ = sz
	if hs := b.HitSize(); hs.Width < f.MinTarget || hs.Height < f.MinTarget {
		t.Fatalf("hit=%+v want >= %.0fpx", hs, f.MinTarget)
	}
	for _, tc := range []struct {
		name string
		set  func(*button.Button)
	}{
		{"primary", func(x *button.Button) { x.SetType(button.ButtonPrimary) }},
		{"default", func(*button.Button) {}},
		{"danger", func(x *button.Button) { x.SetType(button.ButtonPrimary); x.SetDanger(true) }},
	} {
		x := button.NewButton("确定")
		tc.set(x)
		if r := button.ContrastRatio(x.TextColor(), x.Fill()); r < f.ContrastFloor {
			t.Fatalf("%s contrast=%.2f below %.1f", tc.name, r, f.ContrastFloor)
		}
	}
	// Focus ring paints outside the chrome: bigger canvas, ring pixel is accent.
	x := button.NewButton("确定")
	x.SetType(button.ButtonPrimary)
	m := focus.NewManager()
	m.Register(x.FocusNode())
	x.FocusNode().RequestFocus()
	bsz := x.Layout(rendering.Loose(1000, 1000))
	W, H := int(bsz.Width)+8, int(bsz.Height)+8
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	x.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(4, 4))
	got := dc.Image()
	// Ring left stroke sits near canvas x=1 at mid height.
	if r, g, bl, _ := got.At(1, 4+int(bsz.Height)/2).RGBA(); bl < 0x8000 || r > 0xC000 || g > 0xE000 {
		t.Fatalf("focus ring pixel #%04x%04x%04x want primary blue", r, g, bl)
	}
}

// Theme: default/dark/compact token sets drive paint; RTL mirrors order.
func TestButton_ThemeTokens(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "buttons.json"))
	if err != nil {
		t.Fatalf("read buttons.json: %v", err)
	}
	var raw struct {
		DarkTheme struct {
			ColorBgContainer string    `json:"colorBgContainer"`
			ColorText        []float64 `json:"colorText"`
			ColorBorder      string    `json:"colorBorder"`
			ColorPrimary     string    `json:"colorPrimary"`
		} `json:"darkTheme"`
		CompactTheme struct {
			ControlHeight   float64 `json:"controlHeight"`
			ControlHeightSM float64 `json:"controlHeightSM"`
			ControlHeightLG float64 `json:"controlHeightLG"`
			FontSize        float64 `json:"fontSize"`
		} `json:"compactTheme"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("parse theme rows: %v", err)
	}

	def := button.NewButton("确定")
	defFill := def.Fill()
	if math.Abs(def.Height()-32) > 0.5 {
		t.Fatalf("default height=%v want 32 (theme ControlHeight)", def.Height())
	}

	dark := theme.DefaultTokens()
	dark.ColorBgContainer = theme.Hex(raw.DarkTheme.ColorBgContainer)
	dark.ColorBorder = theme.Hex(raw.DarkTheme.ColorBorder)
	dark.ColorPrimary = theme.Hex(raw.DarkTheme.ColorPrimary)
	if len(raw.DarkTheme.ColorText) == 4 {
		v := raw.DarkTheme.ColorText
		dark.ColorText = theme.RGBA(v[0], v[1], v[2], v[3])
	}
	d := button.NewButton("确定")
	d.SetTheme(&dark)
	if d.Fill() == defFill {
		t.Fatal("dark theme must move the default fill off white")
	}
	if math.Abs(d.Height()-32) > 0.5 {
		t.Fatalf("dark keeps geometry: height=%v", d.Height())
	}
	// Provider path matches pinned tokens.
	p := theme.NewProvider(dark)
	via := button.NewButton("确定")
	via.SetProvider(p)
	if via.Fill() != d.Fill() {
		t.Fatalf("provider fill=%+v want %+v", via.Fill(), d.Fill())
	}

	compact := theme.DefaultTokens()
	compact.ControlHeight, compact.ControlHeightSM, compact.ControlHeightLG = raw.CompactTheme.ControlHeight, raw.CompactTheme.ControlHeightSM, raw.CompactTheme.ControlHeightLG
	compact.FontSize = raw.CompactTheme.FontSize
	c := button.NewButton("确定")
	c.SetTheme(&compact)
	if math.Abs(c.Height()-raw.CompactTheme.ControlHeight) > 0.5 {
		t.Fatalf("compact height=%v want %v", c.Height(), raw.CompactTheme.ControlHeight)
	}

	// RTL: start placement trails the icon; centers mirror LTR exactly.
	ltr := button.NewButton("确定")
	ltr.SetIcon("search")
	lw := ltr.Layout(rendering.Loose(1000, 1000))
	rtl := button.NewButton("确定")
	rtl.SetIcon("search")
	rtl.SetRTL(true)
	rtl.Layout(rendering.Loose(1000, 1000))
	if ltr.IconFirst() == rtl.IconFirst() {
		t.Fatal("RTL must flip IconFirst")
	}
	if math.Abs(rtl.LeadCenterX()-(lw.Width-ltr.LeadCenterX())) > 1e-9 {
		t.Fatalf("rtl lead=%v want mirror %v", rtl.LeadCenterX(), lw.Width-ltr.LeadCenterX())
	}
	// RTL snapshot paints and moves ink vs LTR.
	paint := func(x *button.Button) []uint8 {
		s := x.Layout(rendering.Loose(1000, 1000))
		dc := render.NewContext(int(s.Width)+4, int(s.Height)+4)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		x.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(2, 2))
		img := dc.Image()
		px := make([]uint8, 0, 64)
		for y := 0; y < int(s.Height)+4; y += 2 {
			for xx := 0; xx < int(s.Width)+4; xx += 2 {
				r, g, bl, _ := img.At(xx, y).RGBA()
				px = append(px, uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			}
		}
		return px
	}
	a, bb := paint(ltr), paint(rtl)
	if len(a) != len(bb) {
		t.Fatal("rtl snapshot size mismatch")
	}
	moved := 0
	for i := range a {
		if a[i] != bb[i] {
			moved++
		}
	}
	if moved == 0 {
		t.Fatal("rtl snapshot identical to ltr: icon did not mirror")
	}
}
