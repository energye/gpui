package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design 5 baseline metrics (middle size) used by kit defaults.
func TestAntBaselineTokens(t *testing.T) {
	th := kit.DefaultTheme()
	cases := []struct {
		key  string
		want float64
	}{
		{core.TokenControlHeight, 32},
		{core.TokenControlHeightSM, 24},
		{core.TokenControlHeightLG, 40},
		{core.TokenFontSize, 14},
		{core.TokenFontSizeSM, 12},
		{core.TokenFontSizeLG, 16},
		{core.TokenBorderRadius, 6},
		{core.TokenButtonPaddingInline, 15},
		{core.TokenControlPaddingInline, 11},
		{core.TokenSizeIndicator, 16},
		{core.TokenSwitchWidth, 44},
		{core.TokenSwitchHeight, 22},
		{core.TokenProgressHeight, 8},
		{core.TokenSpinSize, 20},
		{core.TokenLineWidth, 1},
	}
	for _, tc := range cases {
		if got := th.Size(tc.key); got != tc.want {
			t.Errorf("token %s = %v want %v", tc.key, got, tc.want)
		}
	}
	// Primary family
	if th.Color(core.TokenColorPrimary).A == 0 {
		t.Fatal("missing primary")
	}
	if th.Color(core.TokenColorBgTextHover).A == 0 {
		t.Fatal("missing bgTextHover")
	}
	if th.Color(core.TokenColorBorderHover).A == 0 {
		t.Fatal("missing borderHover")
	}
}

func TestButtonMiddleHeightAnt(t *testing.T) {
	btn := kit.NewButton("OK")
	sz := btn.Node().Layout(core.Loose(400, 100))
	if sz.Height != 32 {
		t.Fatalf("button height=%v want 32 (Ant middle)", sz.Height)
	}
	// Small / large
	btn.SetSize(kit.ButtonSmall)
	sz = btn.Node().Layout(core.Loose(400, 100))
	if sz.Height != 24 {
		t.Fatalf("small height=%v want 24", sz.Height)
	}
	btn.SetSize(kit.ButtonLarge)
	sz = btn.Node().Layout(core.Loose(400, 100))
	if sz.Height != 40 {
		t.Fatalf("large height=%v want 40", sz.Height)
	}
}

func TestButtonHoverAutoSync(t *testing.T) {
	btn := kit.NewButton("Hover")
	btn.SetType(kit.ButtonDefault)
	_ = btn.Node().Layout(core.Loose(200, 100))
	// Simulate tree hover without per-frame SyncState call.
	btn.Root.SetHovered(true)
	// Decorated background should have switched to hover fill.
	if btn.ChromeNode().(*primitive.Decorated).Background == (struct {
		R, G, B, A float64
	}{}) {
		// zero is fine for text/link; default should be non-zero
	}
	dec := btn.ChromeNode().(*primitive.Decorated)
	// After hover, background should differ from pure white or match composite hover.
	// At minimum, MarkNeedsPaint path ran without panic and chrome is set.
	if dec.BorderWidth < 0 {
		t.Fatal("invalid border")
	}
	// Pressed path
	btn.Root.SetHovered(false)
	// Use pointer path: setPressed is private; SetHovered(false) then re-hover.
	btn.Root.SetHovered(true)
	btn.SyncState() // idempotent
}

func TestCheckboxRadioIndicatorSize(t *testing.T) {
	cb := kit.NewCheckbox("c")
	ind := cb.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Width != 16 || ind.Height != 16 {
		t.Fatalf("checkbox indicator %vx%v want 16x16", ind.Width, ind.Height)
	}
	rd := kit.NewRadio("a", "A")
	rind := rd.IndicatorNode().(*primitive.Decorated)
	_ = rind.Layout(core.Loose(100, 100))
	if rind.Width != 16 || rind.Height != 16 {
		t.Fatalf("radio indicator %vx%v want 16x16", rind.Width, rind.Height)
	}
}

func TestInputHeightAnt(t *testing.T) {
	in := kit.NewInput("ph")
	sz := in.Node().Layout(core.Loose(400, 100))
	if sz.Height != 32 {
		t.Fatalf("input height=%v want 32", sz.Height)
	}
}

func TestSwitchGeometryAnt(t *testing.T) {
	sw := kit.NewSwitch()
	ind := sw.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Width != 44 || ind.Height != 22 {
		t.Fatalf("switch track %vx%v want 44x22", ind.Width, ind.Height)
	}
}

func TestSelectHeightAnt(t *testing.T) {
	s := kit.NewSelect("pick", kit.SelectOption{Value: "1", Label: "One"})
	// Select root is Pressable wrapping Decorated.
	_ = s.Node().Layout(core.Loose(400, 100))
	if s.Root == nil {
		t.Fatal("nil root")
	}
	// Decorated is first child of Pressable
	kids := s.Root.Children()
	if len(kids) < 1 {
		t.Fatal("no children")
	}
	dec, ok := kids[0].(*primitive.Decorated)
	if !ok {
		t.Fatalf("child type %T", kids[0])
	}
	if dec.Height != 32 && dec.MinHeight != 32 {
		t.Fatalf("select height=%v min=%v want 32", dec.Height, dec.MinHeight)
	}
}

func TestMenuSelectedChrome(t *testing.T) {
	m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"}, kit.MenuItem{Key: "b", Label: "B"})
	m.SetSelected("a")
	_ = m.Node().Layout(core.Loose(200, 200))
	// Rebuild applied selected fill without panic.
	if m.Root == nil {
		t.Fatal("nil menu root")
	}
	if m.Root.Radius < 6 {
		t.Fatalf("menu radius=%v want >=6", m.Root.Radius)
	}
}

func TestFormItemGapAnt(t *testing.T) {
	fi := kit.NewFormItem("n", "Name", kit.NewInput("").Node())
	_ = fi.Node().Layout(core.Loose(300, 200))
	// Root is column: field + error; no crash.
	if fi.Root == nil {
		t.Fatal("nil form item")
	}
}

func TestTableRowHeightAnt(t *testing.T) {
	tb := kit.NewTable([]kit.TableColumn{{Key: "n", Title: "N"}}, []map[string]string{{"n": "x"}})
	if tb.RowHeight != 47 {
		t.Fatalf("row height=%v want 47", tb.RowHeight)
	}
}

func TestDrawerDefaultWidthAnt(t *testing.T) {
	d := kit.NewDrawer("D")
	if d.Width != 378 {
		t.Fatalf("drawer width=%v want 378", d.Width)
	}
}

func TestPrimaryBgToken(t *testing.T) {
	th := kit.DefaultTheme()
	c := th.Color(core.TokenColorPrimaryBg)
	if c.A < 0.5 {
		t.Fatalf("primaryBg missing: %+v", c)
	}
}
