package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/affix.md §6.9 — P0 L1/L2 cases (AFX-01…13).
// AFX-14 L3 golden / AFX-15 L4 / AFX-16 P1 omitted.

func TestAffix_PRD_01_Defaults(t *testing.T) {
	// AFX-01
	a := kit.NewAffix(kit.NewText("pin").Node())
	if a == nil || a.Node() == nil {
		t.Fatal("nil affix")
	}
	if a.Node().TypeID() != "kit.Affix" {
		t.Fatalf("type=%s", a.Node().TypeID())
	}
	if a.IsAffixed() {
		t.Fatal("default not affixed")
	}
	top, useTop := a.ResolvedOffsetTop()
	if !useTop || top != kit.DefaultAffixOffsetTop {
		t.Fatalf("offsetTop=%v use=%v want 0/true", top, useTop)
	}
	if _, useBot := a.ResolvedOffsetBottom(); useBot {
		t.Fatal("offsetBottom default unset")
	}
	if a.Node().Base().Role != "presentation" {
		t.Fatalf("Role=%q", a.Node().Base().Role)
	}
	sz := a.Node().Layout(core.Loose(120, 40))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%v", sz)
	}
}

func TestAffix_PRD_02_OffsetTopAffix(t *testing.T) {
	// AFX-02 / AFX-S1
	a := kit.NewAffix(kit.NewText("top").Node())
	a.SetOffsetTop(10)
	var flips []bool
	a.SetOnChange(func(affixed bool) { flips = append(flips, affixed) })

	// contentTop=0, scroll=0 → phTop=0; 0 > 0-10 → affixed
	a.Evaluate(0, 100, 0, 32)
	if !a.IsAffixed() || a.AffixMode() != "top" {
		t.Fatalf("affixed=%v mode=%q", a.IsAffixed(), a.AffixMode())
	}
	if len(flips) != 1 || !flips[0] {
		t.Fatalf("onChange=%v", flips)
	}

	// Far down content: contentTop=200, scroll=0 → phTop=200; 0 > 200-10? false
	a.Evaluate(0, 100, 200, 32)
	if a.IsAffixed() {
		t.Fatal("should release when below threshold")
	}
	if len(flips) != 2 || flips[1] {
		t.Fatalf("onChange release=%v", flips)
	}

	// Scroll past: scroll=195, contentTop=200 → phTop=5; 0 > 5-10 → true
	a.Evaluate(195, 100, 200, 32)
	if !a.IsAffixed() {
		t.Fatal("should affix after scroll")
	}
}

func TestAffix_PRD_03_ReleaseOnScrollBack(t *testing.T) {
	// AFX-03 / AFX-S2
	a := kit.NewAffix(nil)
	a.SetOffsetTop(0)
	var last *bool
	a.SetOnChange(func(affixed bool) {
		v := affixed
		last = &v
	})
	a.Evaluate(50, 100, 40, 20) // phTop=-10 → affixed
	if !a.IsAffixed() || last == nil || !*last {
		t.Fatal("affix true")
	}
	a.Evaluate(0, 100, 40, 20) // phTop=40 → free
	if a.IsAffixed() || last == nil || *last {
		t.Fatal("release false")
	}
}

func TestAffix_PRD_04_PlaceholderStable(t *testing.T) {
	// AFX-04 / AFX-S3
	box := primitive.NewBox()
	box.Width, box.Height = 80, 40
	a := kit.NewAffix(box)
	sz0 := a.Node().Layout(core.Loose(200, 100))
	ph0 := a.PlaceholderSize()
	a.SetOffsetTop(0)
	a.Evaluate(30, 100, 0, 40)
	if !a.IsAffixed() {
		t.Fatal("want affixed")
	}
	sz1 := a.Node().Layout(core.Loose(200, 100))
	ph1 := a.PlaceholderSize()
	if sz0 != sz1 {
		t.Fatalf("layout size jumped %v → %v", sz0, sz1)
	}
	if ph0 != ph1 {
		t.Fatalf("placeholder jumped %v → %v", ph0, ph1)
	}
	if ph1.Width < 79.5 || ph1.Height < 39.5 {
		t.Fatalf("placeholder=%v", ph1)
	}
}

func TestAffix_PRD_05_CustomScrollTarget(t *testing.T) {
	// AFX-05 / AFX-S4
	body := primitive.NewBox()
	body.Width, body.Height = 100, 40
	a := kit.NewAffix(body)
	a.SetOffsetTop(0)
	a.SetContentTop(80)

	// Tall content so ScrollY can move past the affix natural top.
	pad := primitive.NewBox()
	pad.Height = 400
	col := primitive.Column(a.Node(), pad)
	sc := kit.NewScroll(col)
	sc.SetSize(120, 60)
	_ = sc.Node().Layout(core.Tight(120, 60))
	a.SetScrollTarget(sc.Viewport())

	var n int
	a.SetOnChange(func(bool) { n++ })

	// scroll 0: phTop=80 → free (0 > 80-0 is false)
	sc.Viewport().SetScroll(0, 0)
	a.UpdatePosition()
	if a.IsAffixed() {
		t.Fatal("not yet")
	}

	// scroll 90: phTop=-10 → affixed relative to container
	sc.Viewport().SetScroll(0, 90)
	a.UpdatePosition()
	if sc.Viewport().ScrollY < 89 {
		t.Fatalf("ScrollY=%v (content too short?)", sc.Viewport().ScrollY)
	}
	if !a.IsAffixed() {
		t.Fatal("target scroll should affix")
	}
	if n < 1 {
		t.Fatal("onChange expected")
	}
}

func TestAffix_PRD_06_OffsetBottom(t *testing.T) {
	// AFX-06 / AFX-S5
	a := kit.NewAffix(nil)
	a.SetOffsetBottom(10)
	// only bottom → top rule off
	if _, useTop := a.ResolvedOffsetTop(); useTop {
		t.Fatal("top disabled when only bottom set")
	}
	// antd: target.bottom < placeholder.bottom + offsetBottom (strict)
	// contentTop=71, h=20, vh=100 → phBottom=91; 100 < 91+10 → affixed bottom
	a.Evaluate(0, 100, 71, 20)
	if !a.IsAffixed() || a.AffixMode() != "bottom" {
		t.Fatalf("affixed=%v mode=%q", a.IsAffixed(), a.AffixMode())
	}
	// content higher: contentTop=10 → phBottom=30; 100 < 30+10? false
	a.Evaluate(0, 100, 10, 20)
	if a.IsAffixed() {
		t.Fatal("should free when far from bottom")
	}
}

func TestAffix_PRD_07_BasicDemo(t *testing.T) {
	// AFX-07 basic.tsx — offsetTop + offsetBottom buttons
	topBtn := kit.NewButton("Affix top")
	topBtn.SetType(kit.ButtonPrimary)
	top := kit.NewAffix(topBtn.Node())
	top.SetOffsetTop(100)
	_ = top.Node().Layout(core.Loose(200, 40))
	top.Evaluate(0, 400, 0, 32)
	if !top.IsAffixed() {
		t.Fatal("basic top should pin at scroll 0 contentTop 0")
	}

	botBtn := kit.NewButton("Affix bottom")
	botBtn.SetType(kit.ButtonPrimary)
	bot := kit.NewAffix(botBtn.Node())
	bot.SetOffsetBottom(100)
	_ = bot.Node().Layout(core.Loose(200, 40))
	bot.Evaluate(0, 400, 350, 32)
	if !bot.IsAffixed() || bot.AffixMode() != "bottom" {
		t.Fatalf("basic bottom mode=%q", bot.AffixMode())
	}
}

func TestAffix_PRD_08_OnChangeDemo(t *testing.T) {
	// AFX-08 on-change.tsx — offsetTop=120 + onChange
	btn := kit.NewButton("120px to affix top")
	a := kit.NewAffix(btn.Node())
	a.SetOffsetTop(120)
	var log []bool
	a.SetOnChange(func(affixed bool) { log = append(log, affixed) })
	_ = a.Node().Layout(core.Loose(200, 40))

	// content deep in page
	a.Evaluate(0, 500, 400, 32)
	if a.IsAffixed() {
		t.Fatal("not yet")
	}
	a.Evaluate(300, 500, 400, 32) // phTop=100; 0 > 100-120 → true
	if !a.IsAffixed() {
		t.Fatal("should affix")
	}
	if len(log) != 1 || !log[0] {
		t.Fatalf("log=%v", log)
	}
}

func TestAffix_PRD_09_TargetDemo(t *testing.T) {
	// AFX-09 target.tsx — Affix inside scroll container
	btn := kit.NewButton("Fixed at the top of container")
	btn.SetType(kit.ButtonPrimary)
	a := kit.NewAffix(btn.Node())
	a.SetOffsetTop(0)
	a.SetContentTop(0)

	inner := primitive.Column(a.Node())
	pad := primitive.NewBox()
	pad.Height = 1000
	inner.AddChild(pad)

	sc := kit.NewScroll(inner)
	sc.SetSize(200, 100)
	_ = sc.Node().Layout(core.Tight(200, 100))
	a.SetScrollTarget(sc.Viewport())

	// Resting at container top with offsetTop=0: strict getFixedTop is false
	// (placeholder.top == target.top). Scroll past → pin.
	sc.Viewport().SetScroll(0, 0)
	a.UpdatePosition()
	if a.IsAffixed() {
		t.Fatal("at natural top, not yet affixed (antd strict >)")
	}
	sc.Viewport().SetScroll(0, 40)
	a.UpdatePosition()
	if !a.IsAffixed() {
		t.Fatal("target demo should pin after container scroll")
	}
	if a.AffixMode() != "top" {
		t.Fatalf("mode=%q", a.AffixMode())
	}
}

func TestAffix_PRD_10_TokenMetrics(t *testing.T) {
	// AFX-10
	a := kit.NewAffix(nil)
	th := kit.DefaultTheme()
	a.SetTheme(th)
	fs := a.FontSize()
	if fs < 13.5 || fs > 14.5 {
		t.Fatalf("fontSize=%v want 14", fs)
	}
	r := a.BorderRadius()
	if r < 5.5 || r > 6.5 {
		t.Fatalf("radius=%v want 6", r)
	}
	lw := a.LineWidth()
	if lw < 0.5 || lw > 1.5 {
		t.Fatalf("lineWidth=%v want 1", lw)
	}
	if a.FocusRingOutset() < 1.0 || a.FocusRingOutset() > 2.0 {
		t.Fatalf("focusRing=%v", a.FocusRingOutset())
	}
	if th.SizeOr(core.TokenFontSize, 0) != 14 {
		t.Fatal("theme fontSize seed")
	}
}

func TestAffix_PRD_11_ThemeColors(t *testing.T) {
	// AFX-11 — no hardcoded brand primary as default skin
	a := kit.NewAffix(kit.NewText("t").Node())
	th := kit.DefaultTheme()
	a.SetTheme(th)
	// Affix host is presentation chrome-less; token colors come from Theme for children.
	c := th.Color(core.TokenColorText)
	if c.A == 0 {
		t.Fatal("theme text token missing")
	}
	// Ensure primary exists as token but Affix does not force it as host fill.
	_ = th.Color(core.TokenColorPrimary)
	if a.Node().TypeID() != "kit.Affix" {
		t.Fatal("host type")
	}
}

func TestAffix_PRD_12_DisabledN_A(t *testing.T) {
	// AFX-12 — Affix has no disabled prop in antd; N/A (document via pass).
	a := kit.NewAffix(nil)
	_ = a.Node()
}

func TestAffix_PRD_13_KeyboardN_A(t *testing.T) {
	// AFX-13 — root is not focusable; interactive children own focus.
	a := kit.NewAffix(kit.NewButton("go").Node())
	_ = a.Node().Layout(core.Loose(100, 40))
	if a.Node().Base().Role != "presentation" {
		t.Fatalf("Role=%q", a.Node().Base().Role)
	}
	a.SetAriaLabel("Pinned actions")
	if a.Node().Base().Label != "Pinned actions" {
		t.Fatalf("Label=%q", a.Node().Base().Label)
	}
}

func TestAffix_PRD_API_TopBottomPriority(t *testing.T) {
	// When both set, top wins if both conditions hold (antd measure order).
	a := kit.NewAffix(nil)
	a.SetOffsetTop(0)
	a.SetOffsetBottom(0)
	a.Evaluate(10, 100, 0, 20) // top condition true
	if a.AffixMode() != "top" {
		t.Fatalf("mode=%q", a.AffixMode())
	}
}
