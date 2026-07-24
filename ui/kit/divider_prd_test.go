package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/divider.md §6.9 — P0 PRD cases (DIV-01 … DIV-23).
// L3/L4 (DIV-24/25) and P1 (DIV-26) deferred.

func approxF(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestDivider_PRD_01_Defaults(t *testing.T) {
	// DIV-01
	d := kit.NewDivider()
	if d.EffectiveOrientation() != kit.DividerHorizontal {
		t.Fatalf("orientation=%v want horizontal", d.EffectiveOrientation())
	}
	if d.EffectiveVariant() != kit.DividerSolid {
		t.Fatalf("variant=%v want solid", d.EffectiveVariant())
	}
	if d.Size != kit.DividerSizeUnset {
		t.Fatalf("size=%v want unset", d.Size)
	}
	if d.Plain || d.Dashed || d.Title != "" {
		t.Fatalf("flags plain=%v dashed=%v title=%q", d.Plain, d.Dashed, d.Title)
	}
	if d.TitlePlacement != kit.DividerTitleCenter {
		t.Fatalf("placement=%v want center", d.TitlePlacement)
	}
	if d.Node() == nil {
		t.Fatal("nil node")
	}
}

func TestDivider_PRD_02_DefaultHorizontalSolid(t *testing.T) {
	// DIV-02
	d := kit.NewDivider()
	if d.IsVertical() {
		t.Fatal("want horizontal")
	}
	if d.EffectiveVariant() != kit.DividerSolid {
		t.Fatal(d.EffectiveVariant())
	}
	if !approxF(d.MarginBlock(), kit.DefaultDividerMarginUnset, 0.5) {
		t.Fatalf("marginBlock=%v want %v", d.MarginBlock(), kit.DefaultDividerMarginUnset)
	}
	if !approxF(d.LineWidth(), 1, 0.5) {
		t.Fatalf("lineW=%v", d.LineWidth())
	}
	sz := d.Node().Layout(core.Loose(200, 100))
	// height ≈ 1 + 24*2
	if sz.Height < 40 || sz.Height > 60 {
		t.Fatalf("height=%v want ~49", sz.Height)
	}
	if sz.Width < 199 {
		t.Fatalf("width=%v want stretch ~200", sz.Width)
	}
	rail := d.RailNode()
	if rail == nil || rail.Vertical {
		t.Fatal("rail missing/vertical")
	}
	if len(rail.Dash) != 0 {
		t.Fatalf("solid should have empty dash: %v", rail.Dash)
	}
}

func TestDivider_PRD_03_Vertical(t *testing.T) {
	// DIV-03
	d := kit.NewDivider()
	d.SetVertical(true)
	if !d.IsVertical() {
		t.Fatal("want vertical")
	}
	sz := d.Node().Layout(core.Loose(100, 40))
	// height ≈ 0.9 * 14 = 12.6; width ≈ 1 + 8*2 = 17
	wantH := kit.DefaultDividerPlainFont * kit.DefaultDividerVerticalEm
	if !approxF(sz.Height, wantH, 1.5) {
		t.Fatalf("height=%v want ~%v", sz.Height, wantH)
	}
	wantW := d.LineWidth() + 2*kit.DefaultDividerVerticalMarginInline
	if !approxF(sz.Width, wantW, 1.0) {
		t.Fatalf("width=%v want ~%v", sz.Width, wantW)
	}
	rail := d.RailNode()
	if rail == nil || !rail.Vertical {
		t.Fatal("rail not vertical")
	}

	// orientation takes priority over Vertical sugar
	d2 := kit.NewDivider()
	d2.SetVertical(true)
	d2.SetOrientation(kit.DividerHorizontal)
	if d2.IsVertical() {
		t.Fatal("orientation should win over Vertical sugar")
	}
}

func TestDivider_PRD_04_Dashed(t *testing.T) {
	// DIV-04
	d := kit.NewDivider()
	d.SetDashed(true)
	if d.EffectiveVariant() != kit.DividerDashed {
		t.Fatal(d.EffectiveVariant())
	}
	_ = d.Node().Layout(core.Loose(200, 40))
	if len(d.RailNode().Dash) < 2 {
		t.Fatalf("dash pattern empty: %v", d.RailNode().Dash)
	}

	d2 := kit.NewDivider()
	d2.SetVariant(kit.DividerDashed)
	if d2.EffectiveVariant() != kit.DividerDashed {
		t.Fatal(d2.EffectiveVariant())
	}
}

func TestDivider_PRD_05_Dotted(t *testing.T) {
	// DIV-05
	d := kit.NewDivider()
	d.SetVariant(kit.DividerDotted)
	if d.EffectiveVariant() != kit.DividerDotted {
		t.Fatal(d.EffectiveVariant())
	}
	// dotted wins over dashed flag
	d.SetDashed(true)
	if d.EffectiveVariant() != kit.DividerDotted {
		t.Fatal("dotted should win over dashed flag")
	}
	_ = d.Node().Layout(core.Loose(200, 40))
	dash := d.RailNode().Dash
	if len(dash) < 2 || dash[0] > dash[1] {
		// dotted on=1 off=3
		t.Logf("dotted dash=%v", dash)
	}
	if len(dash) < 2 {
		t.Fatal("empty dotted dash")
	}
}

func TestDivider_PRD_06_TitleCenter(t *testing.T) {
	// DIV-06
	d := kit.NewDivider()
	d.SetTitle("Text")
	if d.TitlePlacement != kit.DividerTitleCenter {
		t.Fatal(d.TitlePlacement)
	}
	gs, ge := d.GrowFactors()
	if !approxF(gs, 1, 0.001) || !approxF(ge, 1, 0.001) {
		t.Fatalf("grow=%v,%v want 1,1", gs, ge)
	}
	sz := d.Node().Layout(core.Loose(320, 80))
	if sz.Width < 300 {
		t.Fatalf("width=%v", sz.Width)
	}
	if d.LabelNode() == nil || d.LabelNode().Value != "Text" {
		t.Fatal("label missing")
	}
	if d.RailNode() == nil {
		t.Fatal("rail missing")
	}
}

func TestDivider_PRD_07_TitleStart(t *testing.T) {
	// DIV-07
	d := kit.NewDivider()
	d.SetTitle("Left Text")
	d.SetTitlePlacement(kit.DividerTitleStart)
	gs, ge := d.GrowFactors()
	if !approxF(gs, kit.DefaultDividerOrientationMargin, 0.001) {
		t.Fatalf("start grow=%v want %v", gs, kit.DefaultDividerOrientationMargin)
	}
	if !approxF(ge, 1-kit.DefaultDividerOrientationMargin, 0.001) {
		t.Fatalf("end grow=%v", ge)
	}
	_ = d.Node().Layout(core.Loose(320, 80))
}

func TestDivider_PRD_08_TitleEnd(t *testing.T) {
	// DIV-08
	d := kit.NewDivider()
	d.SetTitle("Right Text")
	d.SetTitlePlacement(kit.DividerTitleEnd)
	gs, ge := d.GrowFactors()
	if !approxF(ge, kit.DefaultDividerOrientationMargin, 0.001) {
		t.Fatalf("end grow=%v want %v", ge, kit.DefaultDividerOrientationMargin)
	}
	if !approxF(gs, 1-kit.DefaultDividerOrientationMargin, 0.001) {
		t.Fatalf("start grow=%v", gs)
	}
}

func TestDivider_PRD_09_PlainFont(t *testing.T) {
	// DIV-09
	d := kit.NewDivider()
	d.SetTitle("Text")
	d.SetPlain(true)
	if !approxF(d.TitleFontSize(), kit.DefaultDividerPlainFont, 0.5) {
		t.Fatalf("plain font=%v want 14", d.TitleFontSize())
	}
	_ = d.Node().Layout(core.Loose(300, 60))
	if d.LabelNode() == nil || !approxF(d.LabelNode().FontSize, 14, 0.5) {
		t.Fatalf("label font=%v", d.LabelNode().FontSize)
	}
}

func TestDivider_PRD_10_TitleFontLG(t *testing.T) {
	// DIV-10
	d := kit.NewDivider()
	d.SetTitle("Text")
	if !approxF(d.TitleFontSize(), kit.DefaultDividerTitleFont, 0.5) {
		t.Fatalf("title font=%v want 16", d.TitleFontSize())
	}
	_ = d.Node().Layout(core.Loose(300, 60))
	if d.LabelNode() == nil || !approxF(d.LabelNode().FontSize, 16, 0.5) {
		t.Fatalf("label font=%v", d.LabelNode().FontSize)
	}
}

func TestDivider_PRD_11_SizeMargins(t *testing.T) {
	// DIV-11 / DIV-17
	d := kit.NewDivider()
	for _, tc := range []struct {
		size kit.DividerSize
		want float64
	}{
		{kit.DividerSizeSmall, kit.DefaultDividerMarginSmall},
		{kit.DividerSizeMedium, kit.DefaultDividerMarginMedium},
		{kit.DividerSizeLarge, kit.DefaultDividerMarginUnset},
		{kit.DividerSizeUnset, kit.DefaultDividerMarginUnset},
	} {
		d.SetSize(tc.size)
		if !approxF(d.MarginBlock(), tc.want, 0.5) {
			t.Fatalf("size %v margin=%v want %v", tc.size, d.MarginBlock(), tc.want)
		}
		sz := d.Node().Layout(core.Loose(200, 100))
		// height ≈ line + 2*margin
		expectH := d.LineWidth() + 2*tc.want
		if !approxF(sz.Height, expectH, 1.0) {
			t.Fatalf("size %v height=%v want ~%v", tc.size, sz.Height, expectH)
		}
	}
}

func TestDivider_PRD_12_LineWidth(t *testing.T) {
	// DIV-12
	d := kit.NewDivider()
	if !approxF(d.LineWidth(), 1, 0.5) {
		t.Fatal(d.LineWidth())
	}
	_ = d.Node().Layout(core.Loose(100, 40))
	if !approxF(d.RailNode().Thickness, 1, 0.5) {
		t.Fatal(d.RailNode().Thickness)
	}
}

func TestDivider_PRD_13_LineColorToken(t *testing.T) {
	// DIV-13
	d := kit.NewDivider()
	col := d.LineColor()
	th := kit.DefaultTheme()
	split := th.Color(core.TokenColorSplit)
	if col.A <= 0 {
		t.Fatal("zero line color")
	}
	// Must match split token (not primary brand).
	if !approxColor(col, split, 0.02) {
		t.Fatalf("line=%v want split=%v", col, split)
	}
	primary := th.Color(core.TokenColorPrimary)
	if approxColor(col, primary, 0.05) {
		t.Fatalf("line must not be brand primary: %v", col)
	}
	_ = d.Node().Layout(core.Loose(100, 40))
	if d.RailNode().ColorToken != core.TokenColorSplit {
		t.Fatalf("ColorToken=%q", d.RailNode().ColorToken)
	}
}

func TestDivider_PRD_14_A11yRole(t *testing.T) {
	// DIV-14
	d := kit.NewDivider()
	_ = d.Node()
	if d.Root.Base().Role != "separator" {
		t.Fatalf("role=%q want separator", d.Root.Base().Role)
	}
	d.SetAriaLabel("section break")
	if d.Root.Base().Label != "section break" {
		t.Fatalf("label=%q", d.Root.Base().Label)
	}
}

func TestDivider_PRD_15_DemoHorizontal(t *testing.T) {
	// DIV-15 horizontal.tsx
	col := primitive.Column(
		kit.NewText("para").Node(),
		kit.NewDivider().Node(),
		kit.NewText("para").Node(),
		func() core.Node {
			d := kit.NewDivider()
			d.SetDashed(true)
			return d.Node()
		}(),
		kit.NewText("para").Node(),
	)
	sz := col.Layout(core.Loose(400, 400))
	if sz.Height < 50 {
		t.Fatalf("height=%v", sz.Height)
	}
}

func TestDivider_PRD_16_DemoWithText(t *testing.T) {
	// DIV-16 with-text.tsx
	mk := func(title string, p kit.DividerTitlePlacement) core.Node {
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetTitlePlacement(p)
		return d.Node()
	}
	col := primitive.Column(
		mk("Text", kit.DividerTitleCenter),
		mk("Left Text", kit.DividerTitleStart),
		mk("Right Text", kit.DividerTitleEnd),
	)
	sz := col.Layout(core.Loose(400, 300))
	if sz.Width < 300 || sz.Height < 40 {
		t.Fatalf("sz=%v", sz)
	}
}

func TestDivider_PRD_17_DemoSize(t *testing.T) {
	// DIV-17 size.tsx — covered by DIV-11 margins
	for _, s := range []kit.DividerSize{kit.DividerSizeSmall, kit.DividerSizeMedium, kit.DividerSizeLarge} {
		d := kit.NewDivider()
		d.SetSize(s)
		_ = d.Node().Layout(core.Loose(300, 80))
	}
}

func TestDivider_PRD_18_DemoPlain(t *testing.T) {
	// DIV-18 plain.tsx
	d := kit.NewDivider()
	d.SetTitle("Text")
	d.SetPlain(true)
	d.SetTitlePlacement(kit.DividerTitleStart)
	_ = d.Node().Layout(core.Loose(300, 60))
	if !approxF(d.TitleFontSize(), 14, 0.5) {
		t.Fatal(d.TitleFontSize())
	}
}

func TestDivider_PRD_19_DemoVertical(t *testing.T) {
	// DIV-19 vertical.tsx
	row := primitive.Row(
		kit.NewText("Text").Node(),
		func() core.Node {
			d := kit.NewDivider()
			d.SetOrientation(kit.DividerVertical)
			return d.Node()
		}(),
		kit.NewText("Link").Node(),
		func() core.Node {
			d := kit.NewDivider()
			d.SetVertical(true)
			return d.Node()
		}(),
		kit.NewText("Link").Node(),
	)
	row.CrossAlign = core.CrossCenter
	sz := row.Layout(core.Loose(400, 40))
	if sz.Width < 40 {
		t.Fatalf("sz=%v", sz)
	}
}

func TestDivider_PRD_20_DemoVariant(t *testing.T) {
	// DIV-20 variant.tsx
	green := render.Hex("#7cb305")
	titles := map[kit.DividerVariant]string{
		kit.DividerSolid:  "Solid",
		kit.DividerDotted: "Dotted",
		kit.DividerDashed: "Dashed",
	}
	for _, v := range []kit.DividerVariant{kit.DividerSolid, kit.DividerDotted, kit.DividerDashed} {
		d := kit.NewDivider()
		d.SetTitle(titles[v])
		d.SetVariant(v)
		d.SetStyle(kit.Style{Border: green})
		if d.EffectiveVariant() != v {
			t.Fatalf("variant=%v", d.EffectiveVariant())
		}
		_ = d.Node().Layout(core.Loose(300, 60))
		if !approxColor(d.LineColor(), green, 0.02) {
			t.Fatalf("style border not applied: %v", d.LineColor())
		}
	}
}

func TestDivider_PRD_21_SemanticStructure(t *testing.T) {
	// DIV-21 style-class / _semantic — structure mountable (depth P1)
	d := kit.NewDivider()
	d.SetTitle("Solid")
	_ = d.Node().Layout(core.Loose(300, 60))
	if d.Root == nil || d.RailNode() == nil || d.LabelNode() == nil {
		t.Fatal("root/rail/content structure incomplete")
	}
	// vertical rails in semantic demo
	dv := kit.NewDivider()
	dv.SetOrientation(kit.DividerVertical)
	_ = dv.Node().Layout(core.Loose(40, 40))
	if !dv.IsVertical() {
		t.Fatal("vertical semantic sample")
	}
}

func TestDivider_PRD_22_MetricsL2(t *testing.T) {
	// DIV-22
	d := kit.NewDivider()
	if !approxF(d.LineWidth(), 1, 0.5) {
		t.Fatal("lineWidth")
	}
	d.SetSize(kit.DividerSizeSmall)
	if !approxF(d.MarginBlock(), 8, 0.5) {
		t.Fatal(d.MarginBlock())
	}
	d.SetSize(kit.DividerSizeMedium)
	if !approxF(d.MarginBlock(), 16, 0.5) {
		t.Fatal(d.MarginBlock())
	}
	d.SetTitle("T")
	d.SetSize(kit.DividerSizeUnset)
	// with-text unset → 16
	if !approxF(d.MarginBlock(), kit.DefaultDividerMarginWithText, 0.5) {
		t.Fatalf("with-text margin=%v", d.MarginBlock())
	}
	if !approxF(d.TitleFontSize(), 16, 0.5) {
		t.Fatal(d.TitleFontSize())
	}
	d.SetPlain(true)
	if !approxF(d.TitleFontSize(), 14, 0.5) {
		t.Fatal(d.TitleFontSize())
	}
}

func TestDivider_PRD_23_NoHardcodedBrand(t *testing.T) {
	// DIV-23
	d := kit.NewDivider()
	col := d.LineColor()
	primary := kit.DefaultTheme().Color(core.TokenColorPrimary)
	if approxColor(col, primary, 0.05) {
		t.Fatalf("default line must not be primary brand: %v", col)
	}
	if col.A <= 0 {
		t.Fatal("transparent line")
	}
}
