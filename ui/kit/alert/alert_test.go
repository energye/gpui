package alert_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
)

// Queue→Layout→Frame→Reset headless: no real window, one PRD id per test.

func TestAlert_PRD_ALT01_Defaults(t *testing.T) {
	a := alert.NewAlert("hello")
	if a.Title() != "hello" {
		t.Fatalf("title=%q", a.Title())
	}
	if a.Type() != alert.AlertInfo || a.Variant() != alert.AlertOutlined {
		t.Fatalf("type=%s variant=%s", a.Type(), a.Variant())
	}
	if a.ShowIcon() || a.Closable() || a.IsBanner() {
		t.Fatal("defaults showIcon/closable/banner")
	}
	if !a.Visible() || a.Hidden() {
		t.Fatal("default visible")
	}
	if a.Role() != "alert" || a.Focusable() {
		t.Fatalf("role=%q focusable", a.Role())
	}
	sz := a.Layout(rendering.Loose(800, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	a.Reset()
}

func TestAlert_PRD_ALT02_Types(t *testing.T) {
	types := []alert.AlertType{alert.AlertSuccess, alert.AlertInfo, alert.AlertWarning, alert.AlertError}
	bgs := map[string]bool{}
	ics := map[string]bool{}
	bds := map[string]bool{}
	for _, tp := range types {
		a := alert.NewAlert("t")
		a.SetType(tp)
		if a.Type() != tp {
			t.Fatalf("type=%s want %s", a.Type(), tp)
		}
		bg := a.Background()
		ic := a.IconColor()
		bd := a.BorderColor()
		bgs[sprintRGBA(bg)] = true
		ics[sprintRGBA(ic)] = true
		bds[sprintRGBA(bd)] = true
		sz := a.Layout(rendering.Loose(600, 200))
		if sz.Width <= 0 {
			t.Fatalf("%s layout=%v", tp, sz)
		}
	}
	if len(bgs) != 4 || len(ics) != 4 {
		t.Fatalf("semantic colors must differ: bg=%d icon=%d", len(bgs), len(ics))
	}
	if len(bds) != 4 {
		t.Fatalf("border colors must differ: %d", len(bds))
	}
}

func TestAlert_PRD_ALT03_Closable(t *testing.T) {
	a := alert.NewAlert("close me")
	a.SetClosable(true)
	onClose, after := 0, 0
	a.SetOnClose(func(e *alert.AlertCloseEvent) { onClose++ })
	a.SetAfterClose(func() { after++ })
	if !a.CloseFocusable() {
		t.Fatal("close should be focusable")
	}
	if !a.ClickClose() {
		t.Fatal("ClickClose should hide")
	}
	if a.Visible() || !a.Hidden() || onClose != 1 || after != 1 {
		t.Fatalf("visible=%v onClose=%d after=%d", a.Visible(), onClose, after)
	}
	sz := a.Layout(rendering.Loose(600, 200))
	if sz.Width != 0 || sz.Height != 0 {
		t.Fatalf("hidden layout=%v want 0", sz)
	}
	// PreventDefault keeps visible (ALT-S8).
	b := alert.NewAlert("keep")
	b.SetClosable(true)
	b.SetOnClose(func(e *alert.AlertCloseEvent) { e.PreventDefault() })
	called := false
	b.SetAfterClose(func() { called = true })
	if b.ClickClose() || !b.Visible() || called {
		t.Fatal("PreventDefault must keep visible and skip afterClose")
	}
	// Reset reopens for next frame.
	a.Reset()
	if !a.Visible() {
		t.Fatal("Reset should reopen")
	}
}

func TestAlert_PRD_ALT04_Description(t *testing.T) {
	a := alert.NewAlert("title")
	a.SetDescription("helper")
	if !a.HasDescription() {
		t.Fatal("HasDescription")
	}
	if a.TitleFontSize() != 16 {
		t.Fatalf("title font=%v want 16", a.TitleFontSize())
	}
	if a.PadH() != 24 || a.PadV() != 20 {
		t.Fatalf("pad=%v,%v want 24,20", a.PadH(), a.PadV())
	}
	sz := a.Layout(rendering.Loose(600, 300))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	plain := alert.NewAlert("title")
	if plain.TitleFontSize() != 14 || plain.PadH() != 12 || plain.PadV() != 8 {
		t.Fatalf("plain title=%v pad=%v,%v", plain.TitleFontSize(), plain.PadH(), plain.PadV())
	}
}

func TestAlert_PRD_ALT05_ShowIcon(t *testing.T) {
	a := alert.NewAlert("t")
	if a.IconVisible() {
		t.Fatal("default no icon")
	}
	a.SetShowIcon(true)
	if !a.IconVisible() || !a.ShowIcon() {
		t.Fatal("showIcon visible")
	}
	a.SetIcon("check-circle")
	if a.IconName() != "check-circle" {
		t.Fatalf("icon=%q", a.IconName())
	}
	sz := a.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

func TestAlert_PRD_ALT06_Banner(t *testing.T) {
	a := alert.NewAlert("top")
	a.SetBanner(true)
	if !a.IsBanner() {
		t.Fatal("banner")
	}
	if a.Type() != alert.AlertWarning {
		t.Fatalf("banner default type=%s want warning", a.Type())
	}
	if !a.ShowIcon() || !a.IconVisible() {
		t.Fatal("banner default showIcon")
	}
	if a.Radius() != 0 || a.HasBorder() || a.LineWidth() != 0 {
		t.Fatalf("banner chrome radius=%v border=%v lw=%v", a.Radius(), a.HasBorder(), a.LineWidth())
	}
	sz := a.Layout(rendering.Constraints{MaxWidth: 800, MaxHeight: 200})
	if sz.Width != 800 {
		t.Fatalf("banner should fill width: %v", sz)
	}
}

func TestAlert_PRD_ALT07_Action(t *testing.T) {
	a := alert.NewAlert("with action")
	btn := rendering.NewRenderColorBox(60, 28, 0.1, 0.4, 0.9, 1)
	a.SetAction(btn)
	if !a.HasAction() || !a.ActionVisible() {
		t.Fatal("action visible")
	}
	if a.Action() != rendering.RenderObject(btn) {
		t.Fatal("action node")
	}
	sz := a.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

func TestAlert_PRD_ALT08_Basic(t *testing.T) {
	a := alert.NewAlert("Success Text")
	a.SetType(alert.AlertSuccess)
	sz := a.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("basic layout=%v", sz)
	}
	if !a.Visible() {
		t.Fatal("visible")
	}
}

func TestAlert_PRD_ALT09_FourTypes(t *testing.T) {
	for _, tp := range []alert.AlertType{alert.AlertSuccess, alert.AlertInfo, alert.AlertWarning, alert.AlertError} {
		a := alert.NewAlert(string(tp))
		a.SetType(tp)
		sz := a.Layout(rendering.Loose(600, 200))
		if sz.Width <= 0 {
			t.Fatalf("%s layout=%v", tp, sz)
		}
	}
}

func TestAlert_PRD_ALT10_Filled(t *testing.T) {
	a := alert.NewAlert("filled")
	a.SetVariant(alert.AlertFilled)
	if a.HasBorder() || a.LineWidth() != 0 {
		t.Fatal("filled has no border")
	}
	sz := a.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	o := alert.NewAlert("outlined")
	if !o.HasBorder() || o.LineWidth() == 0 {
		t.Fatal("outlined must border")
	}
}

func TestAlert_PRD_ALT11_ClosableExamples(t *testing.T) {
	for _, tp := range []alert.AlertType{alert.AlertSuccess, alert.AlertInfo, alert.AlertWarning, alert.AlertError} {
		a := alert.NewAlert("x")
		a.SetType(tp)
		a.SetClosable(true)
		if !a.ClickClose() || a.Visible() {
			t.Fatalf("%s close", tp)
		}
	}
}

func TestAlert_PRD_ALT12_DescriptionExamples(t *testing.T) {
	for _, tp := range []alert.AlertType{alert.AlertSuccess, alert.AlertInfo, alert.AlertWarning, alert.AlertError} {
		a := alert.NewAlert("t")
		a.SetType(tp)
		a.SetDescription("d")
		if !a.HasDescription() {
			t.Fatalf("%s desc", tp)
		}
		sz := a.Layout(rendering.Loose(600, 300))
		if sz.Width <= 0 {
			t.Fatalf("%s layout=%v", tp, sz)
		}
	}
}

func TestAlert_PRD_ALT13_IconCombos(t *testing.T) {
	cases := []struct {
		show, desc, close bool
	}{
		{true, false, false},
		{true, true, false},
		{true, true, true},
		{false, false, true},
		{true, false, true},
	}
	for i, c := range cases {
		a := alert.NewAlert("t")
		a.SetShowIcon(c.show)
		if c.desc {
			a.SetDescription("d")
		}
		a.SetClosable(c.close)
		if a.IconVisible() != c.show {
			t.Fatalf("case %d icon=%v want %v", i, a.IconVisible(), c.show)
		}
		sz := a.Layout(rendering.Loose(600, 300))
		if sz.Width <= 0 {
			t.Fatalf("case %d layout=%v", i, sz)
		}
	}
}

func TestAlert_PRD_ALT14_BannerCombo(t *testing.T) {
	a := alert.NewAlert("banner text")
	a.SetBanner(true)
	a.SetDescription("sub")
	a.SetClosable(true)
	if !a.IconVisible() || !a.HasDescription() || !a.Closable() {
		t.Fatal("banner combo")
	}
	sz := a.Layout(rendering.Constraints{MaxWidth: 900, MaxHeight: 300})
	if sz.Width != 900 {
		t.Fatalf("banner width=%v", sz)
	}
}

func TestAlert_PRD_ALT15_LoopBanner(t *testing.T) {
	a := alert.NewAlert("loop")
	a.SetBanner(true)
	long := rendering.NewRenderText("Very long scrolling announcement text for loop-banner marquee slot")
	long.FontSize = 14
	a.SetTitleNode(long)
	if a.TitleNode() == nil {
		t.Fatal("TitleNode")
	}
	sz := a.Layout(rendering.Constraints{MaxWidth: 900, MaxHeight: 300})
	if sz.Width != 900 || sz.Height <= 0 {
		t.Fatalf("loop layout=%v", sz)
	}
}

func TestAlert_PRD_ALT15B_ActionButtons(t *testing.T) {
	// Single-button variants (UNDO / Detail / Done).
	for _, label := range []string{"UNDO", "Detail", "Done"} {
		_ = label
		a := alert.NewAlert("action single")
		a.SetAction(rendering.NewRenderColorBox(64, 28, 0.09, 0.47, 1, 1))
		a.SetShowIcon(true)
		if !a.ActionVisible() {
			t.Fatal("single action visible")
		}
		sz := a.Layout(rendering.Loose(700, 200))
		if sz.Width <= 0 {
			t.Fatalf("single %v", sz)
		}
	}
	// Accept+Decline vertical pair + description coexistence.
	holder := rendering.NewRenderBox(
		rendering.NewRenderColorBox(70, 26, 0.09, 0.47, 1, 1),
		rendering.NewRenderColorBox(70, 26, 0.6, 0.6, 0.6, 1),
	)
	a := alert.NewAlert("action double")
	a.SetDescription("with double row")
	a.SetAction(holder)
	a.SetShowIcon(true)
	if !a.ActionVisible() || !a.HasDescription() {
		t.Fatal("double action + desc")
	}
	sz := a.Layout(rendering.Loose(700, 300))
	if sz.Width <= 0 {
		t.Fatalf("double %v", sz)
	}
	// Action clickable proxy: node hit inside shell.
	if a.Action() == nil {
		t.Fatal("action node")
	}
}

func sprintRGBA(v interface{ RGBA() (uint32, uint32, uint32, uint32) }) string {
	r, g, b, ac := v.RGBA()
	return fmt.Sprintf("%d-%d-%d-%d", r, g, b, ac)
}
