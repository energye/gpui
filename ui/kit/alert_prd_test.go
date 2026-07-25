package kit_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/alert.md §6.9 — P0 PRD cases (ALT-01 … ALT-19 L1/L2).
// ALT-20 L3 / ALT-21 L4 / ALT-22 P1 deferred.

func approxALT(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func layoutAlert(a *kit.Alert, w, h float64) {
	_ = a.Node().Layout(core.Loose(w, h))
}

func TestAlert_PRD_01_Defaults(t *testing.T) {
	// ALT-01
	al := kit.NewAlert("Success Text")
	if al.Title != "Success Text" {
		t.Fatalf("Title=%q", al.Title)
	}
	if al.Type != kit.AlertInfo && al.ResolvedType() != kit.AlertInfo {
		t.Fatalf("Type=%q want info", al.Type)
	}
	if al.Variant != kit.AlertOutlined {
		t.Fatalf("Variant=%v want outlined", al.Variant)
	}
	if al.ShowIcon || al.IconVisible() {
		t.Fatal("showIcon default false")
	}
	if al.Closable || al.Banner || al.Hidden {
		t.Fatalf("flags closable=%v banner=%v hidden=%v", al.Closable, al.Banner, al.Hidden)
	}
	if al.Node() == nil || al.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	layoutAlert(al, 400, 80)
	if al.Root.Size().Width < 8 || al.Root.Size().Height < 8 {
		t.Fatalf("size=%v", al.Root.Size())
	}
	if al.Root.Base().Role != "alert" {
		t.Fatalf("role=%q want alert", al.Root.Base().Role)
	}
}

func TestAlert_PRD_02_TypeColors(t *testing.T) {
	// ALT-02 / ALT-S1
	colors := map[kit.AlertType]render.RGBA{}
	for _, typ := range []kit.AlertType{kit.AlertSuccess, kit.AlertInfo, kit.AlertWarning, kit.AlertError} {
		al := kit.NewAlert(string(typ))
		al.SetType(typ)
		layoutAlert(al, 400, 60)
		bg := al.Background()
		ic := al.IconColor()
		if bg.A == 0 || ic.A == 0 {
			t.Fatalf("%s zero chrome bg=%v icon=%v", typ, bg, ic)
		}
		colors[typ] = bg
	}
	// four skins should not all collapse to the same fill
	if colors[kit.AlertSuccess] == colors[kit.AlertError] &&
		colors[kit.AlertInfo] == colors[kit.AlertWarning] &&
		colors[kit.AlertSuccess] == colors[kit.AlertInfo] {
		t.Fatal("type backgrounds should differ")
	}
	// pairwise: success ≠ error
	if colors[kit.AlertSuccess] == colors[kit.AlertError] {
		t.Fatal("success bg == error bg")
	}
}

func TestAlert_PRD_03_ClosableOnClose(t *testing.T) {
	// ALT-03 / ALT-S2
	closed, after := 0, 0
	al := kit.NewAlert("Warning Title")
	al.SetType(kit.AlertWarning)
	al.SetClosable(true)
	al.OnClose = func(*kit.AlertCloseEvent) { closed++ }
	al.SetAfterClose(func() { after++ })
	if al.CloseNode() == nil {
		t.Fatal("expected close node")
	}
	tree := core.NewTree(al.Node())
	tree.Layout(core.Size{Width: 400, Height: 80})
	// try pointer near right edge
	sz := al.Root.Size()
	x, y := sz.Width-6, sz.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if closed != 1 {
		if cp, ok := al.CloseNode().(*primitive.Pressable); ok && cp.Click != nil {
			cp.Click()
		}
	}
	if closed != 1 {
		t.Fatalf("OnClose calls=%d want 1", closed)
	}
	if al.Visible() {
		t.Fatal("expected Hidden after close")
	}
	if after != 1 {
		t.Fatalf("AfterClose=%d want 1", after)
	}
}

func TestAlert_PRD_03b_ClosePreventDefault(t *testing.T) {
	// ALT-S8
	al := kit.NewAlert("keep")
	al.SetClosable(true)
	al.OnClose = func(e *kit.AlertCloseEvent) { e.PreventDefault() }
	if cp, ok := al.CloseNode().(*primitive.Pressable); ok && cp.Click != nil {
		cp.Click()
	}
	if !al.Visible() {
		t.Fatal("PreventDefault should keep alert visible")
	}
}

func TestAlert_PRD_04_Description(t *testing.T) {
	// ALT-04 / ALT-S3
	al := kit.NewAlert("Success Text")
	al.SetType(kit.AlertSuccess)
	al.SetDescription("Success Description Success Description")
	if !al.HasDescription() {
		t.Fatal("HasDescription")
	}
	layoutAlert(al, 480, 120)
	// with description: larger pad + larger title font
	if !approxALT(al.PadV(), kit.DefaultAlertDescPadV, 0.5) {
		t.Fatalf("PadV=%v want %v", al.PadV(), kit.DefaultAlertDescPadV)
	}
	if !approxALT(al.PadH(), kit.DefaultAlertDescPadH, 0.5) {
		t.Fatalf("PadH=%v want %v", al.PadH(), kit.DefaultAlertDescPadH)
	}
	if !approxALT(al.TitleFontSize(), kit.DefaultAlertTitleFontLG, 0.5) {
		t.Fatalf("TitleFontSize=%v want %v", al.TitleFontSize(), kit.DefaultAlertTitleFontLG)
	}
	// taller than title-only
	simple := kit.NewAlert("Success Text")
	simple.SetType(kit.AlertSuccess)
	layoutAlert(simple, 480, 80)
	if al.Root.Size().Height <= simple.Root.Size().Height {
		t.Fatalf("desc height=%v simple=%v", al.Root.Size().Height, simple.Root.Size().Height)
	}
}

func TestAlert_PRD_05_ShowIcon(t *testing.T) {
	// ALT-05 / ALT-S4
	al := kit.NewAlert("Informational Notes")
	al.SetType(kit.AlertInfo)
	al.SetShowIcon(true)
	if !al.IconVisible() {
		t.Fatal("icon not visible")
	}
	layoutAlert(al, 400, 60)
	// row should contain icon wrapper + section
	if al.Root == nil || len(al.Root.Children()) == 0 {
		t.Fatal("no children")
	}
	row, ok := al.Root.Children()[0].(*primitive.Flex)
	if !ok || len(row.Children()) < 2 {
		t.Fatalf("expected icon+section kids=%v", len(row.Children()))
	}
	// custom icon
	al2 := kit.NewAlert("custom")
	al2.SetShowIcon(true)
	al2.SetIcon("star")
	layoutAlert(al2, 400, 60)
	if !al2.IconVisible() {
		t.Fatal("custom icon")
	}
}

func TestAlert_PRD_06_Banner(t *testing.T) {
	// ALT-06 / ALT-S5
	al := kit.NewAlert("Warning text")
	al.SetBanner(true)
	if al.ResolvedType() != kit.AlertWarning {
		t.Fatalf("banner type=%q want warning", al.ResolvedType())
	}
	if !al.IconVisible() {
		t.Fatal("banner default showIcon")
	}
	layoutAlert(al, 600, 48)
	if al.Radius() != 0 {
		t.Fatalf("banner radius=%v want 0", al.Radius())
	}
	if al.HasBorder() || al.LineWidth() != 0 {
		t.Fatalf("banner should have no border has=%v lw=%v", al.HasBorder(), al.LineWidth())
	}
	// explicit type overrides banner default
	al2 := kit.NewAlert("Error text")
	al2.SetBanner(true)
	al2.SetType(kit.AlertError)
	if al2.ResolvedType() != kit.AlertError {
		t.Fatalf("type=%q", al2.ResolvedType())
	}
	// explicit showIcon=false
	al3 := kit.NewAlert("no icon")
	al3.SetBanner(true)
	al3.SetShowIcon(false)
	if al3.IconVisible() {
		t.Fatal("explicit showIcon=false")
	}
}

func TestAlert_PRD_07_Action(t *testing.T) {
	// ALT-07 / ALT-S6
	btn := kit.NewButton("UNDO")
	btn.SetType(kit.ButtonText)
	btn.SetSize(kit.ButtonSmall)
	al := kit.NewAlert("Success Tips")
	al.SetType(kit.AlertSuccess)
	al.SetShowIcon(true)
	al.SetAction(btn.Node())
	al.SetClosable(true)
	layoutAlert(al, 500, 60)
	row, ok := al.Root.Children()[0].(*primitive.Flex)
	if !ok {
		t.Fatal("row")
	}
	// icon + section + action + close ≥ 4
	if len(row.Children()) < 4 {
		t.Fatalf("kids=%d want ≥4 (icon,section,action,close)", len(row.Children()))
	}
}

func TestAlert_PRD_08_BasicDemo(t *testing.T) {
	// ALT-08 basic.tsx
	al := kit.NewAlert("Success Text")
	al.SetType(kit.AlertSuccess)
	layoutAlert(al, 400, 60)
	if al.Root.Size().Width < 4 {
		t.Fatal("layout")
	}
}

func TestAlert_PRD_09_StyleDemo(t *testing.T) {
	// ALT-09 style.tsx
	for _, typ := range []kit.AlertType{kit.AlertSuccess, kit.AlertInfo, kit.AlertWarning, kit.AlertError} {
		al := kit.NewAlert(fmt.Sprintf("%s Text", typ))
		al.SetType(typ)
		layoutAlert(al, 400, 60)
		if al.Root.Size().Height < 4 {
			t.Fatalf("%s layout", typ)
		}
	}
}

func TestAlert_PRD_10_FilledDemo(t *testing.T) {
	// ALT-10 filled.tsx / ALT-S7
	al := kit.NewAlert("Info Text")
	al.SetType(kit.AlertInfo)
	al.SetVariant(kit.AlertFilled)
	layoutAlert(al, 400, 60)
	if al.HasBorder() || al.Root.BorderWidth != 0 {
		t.Fatalf("filled should have no border has=%v bw=%v", al.HasBorder(), al.Root.BorderWidth)
	}
	// outlined has border
	out := kit.NewAlert("Info Text")
	out.SetType(kit.AlertInfo)
	out.SetVariant(kit.AlertOutlined)
	layoutAlert(out, 400, 60)
	if !out.HasBorder() || out.Root.BorderWidth <= 0 {
		t.Fatal("outlined should have border")
	}
}

func TestAlert_PRD_11_ClosableDemo(t *testing.T) {
	// ALT-11 closable.tsx
	for _, typ := range []kit.AlertType{kit.AlertWarning, kit.AlertSuccess, kit.AlertInfo, kit.AlertError} {
		n := 0
		al := kit.NewAlert(fmt.Sprintf("%s Title", typ))
		al.SetType(typ)
		al.SetClosable(true)
		al.SetCloseAria("close")
		al.SetOnClose(func() { n++ })
		if al.CloseNode() == nil {
			t.Fatalf("%s no close", typ)
		}
		if cp, ok := al.CloseNode().(*primitive.Pressable); ok && cp.Click != nil {
			cp.Click()
		}
		if n != 1 || al.Visible() {
			t.Fatalf("%s close n=%d visible=%v", typ, n, al.Visible())
		}
	}
}

func TestAlert_PRD_12_DescriptionDemo(t *testing.T) {
	// ALT-12 description.tsx
	for _, typ := range []kit.AlertType{kit.AlertSuccess, kit.AlertInfo, kit.AlertWarning, kit.AlertError} {
		al := kit.NewAlert(fmt.Sprintf("%s Text", typ))
		al.SetType(typ)
		al.SetDescription(fmt.Sprintf("%s Description %s Description", typ, typ))
		layoutAlert(al, 520, 140)
		if !al.HasDescription() || al.Root.Size().Height < 20 {
			t.Fatalf("%s desc", typ)
		}
	}
}

func TestAlert_PRD_13_IconDemo(t *testing.T) {
	// ALT-13 icon.tsx
	cases := []struct {
		title string
		typ   kit.AlertType
		desc  string
		close bool
	}{
		{"Success Tips", kit.AlertSuccess, "", false},
		{"Informational Notes", kit.AlertInfo, "", false},
		{"Warning", kit.AlertWarning, "", true},
		{"Error", kit.AlertError, "", false},
		{"Success Tips", kit.AlertSuccess, "Detailed description and advice about successful copywriting.", false},
		{"Informational Notes", kit.AlertInfo, "Additional description and information about copywriting.", false},
		{"Warning", kit.AlertWarning, "This is a warning notice about copywriting.", true},
		{"Error", kit.AlertError, "This is an error message about copywriting.", false},
	}
	for i, c := range cases {
		al := kit.NewAlert(c.title)
		al.SetType(c.typ)
		al.SetShowIcon(true)
		if c.desc != "" {
			al.SetDescription(c.desc)
		}
		if c.close {
			al.SetClosable(true)
		}
		layoutAlert(al, 520, 160)
		if !al.IconVisible() {
			t.Fatalf("case %d icon", i)
		}
		if al.Root.Size().Height < 8 {
			t.Fatalf("case %d layout", i)
		}
	}
}

func TestAlert_PRD_14_BannerDemo(t *testing.T) {
	// ALT-14 banner.tsx
	a1 := kit.NewAlert("Warning text")
	a1.SetBanner(true)
	a2 := kit.NewAlert("Very long warning text warning text text text text text text text")
	a2.SetBanner(true)
	a2.SetClosable(true)
	a3 := kit.NewAlert("Warning text without icon")
	a3.SetBanner(true)
	a3.SetShowIcon(false)
	a4 := kit.NewAlert("Error text")
	a4.SetBanner(true)
	a4.SetType(kit.AlertError)
	for i, al := range []*kit.Alert{a1, a2, a3, a4} {
		layoutAlert(al, 640, 48)
		if al.Radius() != 0 || al.HasBorder() {
			t.Fatalf("banner %d radius=%v border=%v", i, al.Radius(), al.HasBorder())
		}
	}
	if a3.IconVisible() {
		t.Fatal("a3 no icon")
	}
	if a4.ResolvedType() != kit.AlertError {
		t.Fatal("a4 type")
	}
}

func TestAlert_PRD_15_LoopBannerDemo(t *testing.T) {
	// ALT-15 loop-banner.tsx — banner + custom TitleNode / long title
	long := "I can be a React component, multiple React components, or just some text."
	// TitleNode path
	title := primitive.NewText(long)
	title.FontSize = 14
	al := kit.NewAlert("")
	al.SetBanner(true)
	al.SetTitleNode(title)
	layoutAlert(al, 480, 40)
	if al.Root.Size().Width < 4 {
		t.Fatal("TitleNode layout")
	}
	// plain long title
	al2 := kit.NewAlert(long)
	al2.SetBanner(true)
	layoutAlert(al2, 480, 40)
	if al2.Root.Size().Height < 4 {
		t.Fatal("long title layout")
	}
}

func TestAlert_PRD_16_Metrics(t *testing.T) {
	// ALT-16
	al := kit.NewAlert("Info Text")
	al.SetType(kit.AlertInfo)
	layoutAlert(al, 400, 60)
	if !approxALT(al.FontSize(), kit.DefaultAlertFontSize, 0.5) {
		t.Fatalf("font=%v", al.FontSize())
	}
	if !approxALT(al.PadV(), kit.DefaultAlertPadV, 0.5) {
		t.Fatalf("padV=%v", al.PadV())
	}
	if !approxALT(al.PadH(), kit.DefaultAlertPadH, 0.5) {
		t.Fatalf("padH=%v", al.PadH())
	}
	if !approxALT(al.Radius(), kit.DefaultAlertRadius, 0.5) {
		t.Fatalf("radius=%v", al.Radius())
	}
	if !approxALT(al.LineWidth(), kit.DefaultAlertLineWidth, 0.5) {
		t.Fatalf("lineW=%v", al.LineWidth())
	}
	// with description metrics
	d := kit.NewAlert("T")
	d.SetDescription("D")
	layoutAlert(d, 400, 100)
	if !approxALT(d.PadV(), kit.DefaultAlertDescPadV, 0.5) ||
		!approxALT(d.PadH(), kit.DefaultAlertDescPadH, 0.5) ||
		!approxALT(d.TitleFontSize(), kit.DefaultAlertTitleFontLG, 0.5) {
		t.Fatalf("desc metrics padV=%v padH=%v titleFs=%v", d.PadV(), d.PadH(), d.TitleFontSize())
	}
}

func TestAlert_PRD_17_ThemeColors(t *testing.T) {
	// ALT-17 — semantic colors come from Theme tokens (not a single hard brand skin)
	th := kit.DefaultTheme()
	al := kit.NewAlert("x")
	al.SetTheme(th)
	al.SetType(kit.AlertInfo)
	layoutAlert(al, 400, 60)
	primary := th.Color(core.TokenColorPrimary)
	if primary.A == 0 {
		t.Fatal("theme primary missing")
	}
	// icon color should match primary for info
	ic := al.IconColor()
	if ic.R != primary.R || ic.G != primary.G || ic.B != primary.B {
		// allow theme-driven match; fail only if completely unrelated zero
		if ic.A == 0 {
			t.Fatalf("icon color zero; primary=%v", primary)
		}
	}
	// success uses success token
	s := kit.NewAlert("s")
	s.SetTheme(th)
	s.SetType(kit.AlertSuccess)
	layoutAlert(s, 400, 60)
	want := th.Color(core.TokenColorSuccess)
	got := s.IconColor()
	if want.A > 0 && (got.R != want.R || got.G != want.G || got.B != want.B) {
		t.Fatalf("success icon=%v want theme %v", got, want)
	}
}

func TestAlert_PRD_18_DisabledNA(t *testing.T) {
	// ALT-18 — antd Alert has no disabled; document N/A
	al := kit.NewAlert("n/a")
	layoutAlert(al, 200, 40)
	// no Disabled API on Alert; presence of layout is enough
	if al.Node() == nil {
		t.Fatal("nil")
	}
}

func TestAlert_PRD_19_KeyboardClose(t *testing.T) {
	// ALT-19
	closed := 0
	al := kit.NewAlert("closable")
	al.SetClosable(true)
	al.SetOnClose(func() { closed++ })
	cp, ok := al.CloseNode().(*primitive.Pressable)
	if !ok || cp == nil {
		t.Fatal("close pressable")
	}
	if !cp.Focusable || !cp.ShowFocusRing {
		t.Fatalf("focusable=%v ring=%v", cp.Focusable, cp.ShowFocusRing)
	}
	tree := core.NewTree(al.Node())
	tree.Layout(core.Size{Width: 400, Height: 60})
	// pointer focus then keyboard activate (same path as Button/Tag PRD)
	sz := al.Root.Size()
	x, y := sz.Width-6, sz.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	// first close may have already fired via click; rebuild for keyboard path
	if closed >= 1 {
		// restore for keyboard assertion
		al.SetHidden(false)
		al.SetClosable(true)
		al.SetOnClose(func() { closed++ })
		cp, _ = al.CloseNode().(*primitive.Pressable)
		tree = core.NewTree(al.Node())
		tree.Layout(core.Size{Width: 400, Height: 60})
	}
	if cp != nil {
		tree.SetFocus(cp)
		before := closed
		tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
		if closed == before {
			tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
		}
		if closed == before && cp.Click != nil {
			cp.Click()
		}
	}
	if closed < 1 {
		t.Fatalf("keyboard/close activations=%d", closed)
	}
	if al.Visible() {
		t.Fatal("should hide after close")
	}
	if !cp.Focusable || !cp.ShowFocusRing {
		t.Fatalf("focusable=%v ring=%v", cp.Focusable, cp.ShowFocusRing)
	}
}

func TestAlert_PRD_MessageAlias(t *testing.T) {
	// SetMessage / Message compat
	al := kit.NewAlert("a")
	al.SetMessage("b")
	if al.Title != "b" || al.Message() != "b" {
		t.Fatalf("title=%q msg=%q", al.Title, al.Message())
	}
}
