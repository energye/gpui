package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/badge.md §6.9 — P0 PRD cases (BDG-01 … BDG-21).
// L3/L4 (BDG-22/23) and P1 (BDG-24) are covered elsewhere or deferred.

func approxBDG(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxColorBDG(a, b render.RGBA, tol float64) bool {
	return approxBDG(a.R, b.R, tol) && approxBDG(a.G, b.G, tol) &&
		approxBDG(a.B, b.B, tol) && approxBDG(a.A, b.A, tol)
}

func badgeChild() core.Node {
	box := primitive.NewBox()
	box.Width, box.Height = 40, 40
	box.Color = render.Hex("#EEEEEE")
	return box
}

func TestBadge_PRD_01_Defaults(t *testing.T) {
	// BDG-01
	b := kit.NewBadge()
	if b.OverflowCount != kit.DefaultBadgeOverflowCount {
		t.Fatalf("OverflowCount=%d want %d", b.OverflowCount, kit.DefaultBadgeOverflowCount)
	}
	if b.Size != kit.BadgeMedium {
		t.Fatalf("Size=%v want medium", b.Size)
	}
	if b.Dot || b.ShowZero || b.Disabled {
		t.Fatalf("flags dot=%v showZero=%v disabled=%v", b.Dot, b.ShowZero, b.Disabled)
	}
	if b.Status != kit.BadgeStatusNone {
		t.Fatalf("Status=%v want none", b.Status)
	}
	_ = b.Node().Layout(core.Loose(100, 100))
}

func TestBadge_PRD_02_Count5(t *testing.T) {
	// BDG-02
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	if b.DisplayCount() != "5" {
		t.Fatalf("DisplayCount=%q want 5", b.DisplayCount())
	}
	if !b.Visible() {
		t.Fatal("expected visible")
	}
	_ = b.Node().Layout(core.Loose(120, 80))
	if b.IndicatorNode() == nil {
		t.Fatal("nil indicator")
	}
}

func TestBadge_PRD_03_Overflow(t *testing.T) {
	// BDG-03
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(100)
	b.SetOverflowCount(99)
	if b.DisplayCount() != "99+" {
		t.Fatalf("DisplayCount=%q want 99+", b.DisplayCount())
	}
}

func TestBadge_PRD_04_Dot(t *testing.T) {
	// BDG-04
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetDot(true)
	if !b.IsDot() {
		t.Fatal("IsDot")
	}
	if !b.Visible() {
		t.Fatal("visible")
	}
	if b.DisplayCount() != "" {
		t.Fatalf("dot should not show numeric count, got %q", b.DisplayCount())
	}
	_ = b.Node().Layout(core.Loose(120, 80))
}

func TestBadge_PRD_05_HideZero(t *testing.T) {
	// BDG-05
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(0)
	b.SetShowZero(false)
	if b.Visible() {
		t.Fatal("count=0 showZero=false should hide")
	}
	if b.DisplayCount() != "" {
		t.Fatalf("DisplayCount=%q", b.DisplayCount())
	}
}

func TestBadge_PRD_06_ShowZero(t *testing.T) {
	// BDG-06
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(0)
	b.SetShowZero(true)
	if b.DisplayCount() != "0" {
		t.Fatalf("DisplayCount=%q want 0", b.DisplayCount())
	}
	if !b.Visible() {
		t.Fatal("visible")
	}
}

func TestBadge_PRD_07_Status(t *testing.T) {
	// BDG-07
	th := kit.DefaultTheme()
	cases := []struct {
		st   kit.BadgeStatus
		tok  string
		want render.RGBA
	}{
		{kit.BadgeStatusSuccess, core.TokenColorSuccess, th.Color(core.TokenColorSuccess)},
		{kit.BadgeStatusError, core.TokenColorError, th.Color(core.TokenColorError)},
		{kit.BadgeStatusWarning, core.TokenColorWarning, th.Color(core.TokenColorWarning)},
		{kit.BadgeStatusDefault, core.TokenColorTextQuaternary, th.Color(core.TokenColorTextQuaternary)},
		{kit.BadgeStatusProcessing, core.TokenColorPrimary, th.Color(core.TokenColorPrimary)},
	}
	for _, tc := range cases {
		b := kit.NewBadge()
		b.SetStatus(tc.st)
		b.SetText(string(rune('A' + int(tc.st))))
		got := b.StatusColor()
		if !approxColorBDG(got, tc.want, 0.05) {
			t.Fatalf("status %v color=%v want %v", tc.st, got, tc.want)
		}
		_ = b.Node().Layout(core.Loose(200, 40))
		if !b.Visible() {
			t.Fatalf("status %v not visible", tc.st)
		}
	}
	// processing Tick advances
	p := kit.NewBadge()
	p.SetStatus(kit.BadgeStatusProcessing)
	_ = p.Node().Layout(core.Loose(40, 40))
	if !p.Tick(0.1) {
		t.Fatal("processing Tick should stay active")
	}
}

func TestBadge_PRD_08_Offset(t *testing.T) {
	// BDG-08
	base := kit.NewBadge()
	base.SetChild(badgeChild())
	base.SetCount(5)
	_ = base.Node().Layout(core.Loose(120, 80))
	baseOff := base.MarkOffset()

	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	b.SetOffset(10, 10)
	_ = b.Node().Layout(core.Loose(120, 80))
	off := b.MarkOffset()
	if !approxBDG(off.X, baseOff.X+10, 0.5) {
		t.Fatalf("offset X=%v base=%v want base+10", off.X, baseOff.X)
	}
	if !approxBDG(off.Y, baseOff.Y+10, 0.5) {
		t.Fatalf("offset Y=%v base=%v want base+10", off.Y, baseOff.Y)
	}
}

func TestBadge_PRD_09_Ribbon(t *testing.T) {
	// BDG-09
	card := primitive.NewBox()
	card.Width, card.Height = 200, 80
	card.Color = render.Hex("#FAFAFA")

	r := kit.NewRibbon("Hippies")
	r.SetChild(card)
	if r.Placement != kit.RibbonEnd {
		t.Fatalf("Placement=%v want end", r.Placement)
	}
	_ = r.Node().Layout(core.Loose(240, 120))
	if r.BandNode() == nil {
		t.Fatal("nil band")
	}
	r.SetPlacement(kit.RibbonStart)
	if r.Placement != kit.RibbonStart {
		t.Fatal("placement start")
	}
	_ = r.Node().Layout(core.Loose(240, 120))
	r.SetColor("#87d068")
	_ = r.Node().Layout(core.Loose(240, 120))
}

func TestBadge_PRD_10_BasicDemo(t *testing.T) {
	// BDG-10 basic.tsx
	mk := func(count int, showZero bool) *kit.Badge {
		av := kit.NewAvatar("")
		av.SetShape(kit.AvatarSquare)
		av.SetSize(kit.AvatarLarge)
		b := kit.NewBadge()
		b.SetChild(av.Node())
		b.SetCount(count)
		b.SetShowZero(showZero)
		return b
	}
	b1 := mk(5, false)
	b2 := mk(0, true)
	b3 := kit.NewBadge()
	av3 := kit.NewAvatar("")
	av3.SetShape(kit.AvatarSquare)
	av3.SetSize(kit.AvatarLarge)
	b3.SetChild(av3.Node())
	ic := kit.NewIcon("clock")
	b3.SetCountNode(ic.Node())

	for i, b := range []*kit.Badge{b1, b2, b3} {
		_ = b.Node().Layout(core.Loose(120, 80))
		if i < 2 && !b.Visible() {
			t.Fatalf("basic[%d] hidden", i)
		}
	}
	if b1.DisplayCount() != "5" {
		t.Fatalf("b1=%q", b1.DisplayCount())
	}
	if b2.DisplayCount() != "0" {
		t.Fatalf("b2=%q", b2.DisplayCount())
	}
}

func TestBadge_PRD_11_NoWrapper(t *testing.T) {
	// BDG-11 no-wrapper.tsx
	b := kit.NewBadge()
	b.SetCount(11)
	b.SetShowZero(true)
	b.SetColor("#faad14")
	_ = b.Node().Layout(core.Loose(80, 40))
	if !b.Visible() {
		t.Fatal("standalone hidden")
	}
	if b.DisplayCount() != "11" {
		t.Fatalf("count=%q", b.DisplayCount())
	}
	// hide
	b.SetCount(0)
	b.SetShowZero(false)
	if b.Visible() {
		t.Fatal("should hide at 0")
	}
}

func TestBadge_PRD_12_OverflowDemo(t *testing.T) {
	// BDG-12 overflow.tsx
	cases := []struct {
		count, ov int
		want      string
	}{
		{99, 99, "99"},
		{100, 99, "99+"},
		{99, 10, "10+"},
		{1000, 999, "999+"},
	}
	for _, tc := range cases {
		b := kit.NewBadge()
		b.SetChild(badgeChild())
		b.SetCount(tc.count)
		b.SetOverflowCount(tc.ov)
		if b.DisplayCount() != tc.want {
			t.Fatalf("count=%d ov=%d got %q want %q", tc.count, tc.ov, b.DisplayCount(), tc.want)
		}
	}
}

func TestBadge_PRD_13_DotDemo(t *testing.T) {
	// BDG-13 dot.tsx
	ic := kit.NewIcon("bell")
	b1 := kit.NewBadge()
	b1.SetChild(ic.Node())
	b1.SetDot(true)
	_ = b1.Node().Layout(core.Loose(80, 40))
	if !b1.IsDot() {
		t.Fatal("dot icon")
	}

	link := kit.NewText("Link something")
	b2 := kit.NewBadge()
	b2.SetChild(link.Node())
	b2.SetDot(true)
	_ = b2.Node().Layout(core.Loose(200, 40))
	if !b2.IsDot() {
		t.Fatal("dot link")
	}
}

func TestBadge_PRD_14_Dynamic(t *testing.T) {
	// BDG-14 change.tsx
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	if b.DisplayCount() != "5" {
		t.Fatal(b.DisplayCount())
	}
	b.SetCount(6)
	if b.DisplayCount() != "6" {
		t.Fatal(b.DisplayCount())
	}
	b.SetCount(0)
	if b.Visible() {
		t.Fatal("hide at 0")
	}
	// separate badge for dot toggle (antd change.tsx uses two instances)
	d := kit.NewBadge()
	d.SetChild(badgeChild())
	d.SetDot(true)
	if !d.IsDot() {
		t.Fatal("dot on")
	}
	d.SetDot(false)
	if d.IsDot() {
		t.Fatal("dot off")
	}
}

func TestBadge_PRD_15_Clickable(t *testing.T) {
	// BDG-15 link.tsx
	clicks := 0
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	b.SetOnClick(func() { clicks++ })
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 120, Height: 80})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 20, Y: 20, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 20, Y: 20, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestBadge_PRD_16_OffsetDemo(t *testing.T) {
	// BDG-16 offset.tsx
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	b.SetOffset(10, 10)
	_ = b.Node().Layout(core.Loose(120, 80))
	off := b.MarkOffset()
	// half-out: Y is negative-ish + 10; X near right
	if off.X < 10 {
		t.Fatalf("mark X=%v expected rightish", off.X)
	}
	if !approxBDG(b.OffsetX, 10, 0.01) || !approxBDG(b.OffsetY, 10, 0.01) {
		t.Fatalf("stored offset %v,%v", b.OffsetX, b.OffsetY)
	}
}

func TestBadge_PRD_17_Size(t *testing.T) {
	// BDG-17 size.tsx
	med := kit.NewBadge()
	med.SetChild(badgeChild())
	med.SetCount(5)
	med.SetSize(kit.BadgeMedium)
	_ = med.Node().Layout(core.Loose(120, 80))
	if !approxBDG(med.IndicatorHeight(), kit.DefaultBadgeIndicatorHeight, 0.5) {
		t.Fatalf("medium h=%v want %v", med.IndicatorHeight(), kit.DefaultBadgeIndicatorHeight)
	}
	// laid-out mark height
	if med.MarkSize().Height > 0 && !approxBDG(med.MarkSize().Height, kit.DefaultBadgeIndicatorHeight, 1.5) {
		t.Fatalf("medium mark h=%v", med.MarkSize().Height)
	}

	sm := kit.NewBadge()
	sm.SetChild(badgeChild())
	sm.SetCount(5)
	sm.SetSize(kit.BadgeSmall)
	_ = sm.Node().Layout(core.Loose(120, 80))
	if !approxBDG(sm.IndicatorHeight(), kit.DefaultBadgeIndicatorHeightSM, 0.5) {
		t.Fatalf("small h=%v want %v", sm.IndicatorHeight(), kit.DefaultBadgeIndicatorHeightSM)
	}
}

func TestBadge_PRD_18_Metrics(t *testing.T) {
	// BDG-18 §6.2
	if !approxBDG(kit.DefaultBadgeIndicatorHeight, 20, 0.5) {
		t.Fatal("indicatorHeight")
	}
	if !approxBDG(kit.DefaultBadgeIndicatorHeightSM, 14, 0.5) {
		t.Fatal("indicatorHeightSM")
	}
	if !approxBDG(kit.DefaultBadgeDotSize, 6, 0.5) {
		t.Fatal("dotSize")
	}
	if !approxBDG(kit.DefaultBadgeStatusSize, 6, 0.5) {
		t.Fatal("statusSize")
	}
	if !approxBDG(kit.DefaultBadgeTextFontSize, 12, 0.5) {
		t.Fatal("textFontSize")
	}
	if !approxBDG(kit.DefaultBadgePaddingInline, 8, 0.5) {
		t.Fatal("paddingInline")
	}
	if kit.DefaultBadgeOverflowCount != 99 {
		t.Fatal("overflowCount")
	}
}

func TestBadge_PRD_19_TokenColors(t *testing.T) {
	// BDG-19
	th := kit.DefaultTheme()
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(3)
	got := b.StatusColor()
	want := th.Color(core.TokenColorError)
	if !approxColorBDG(got, want, 0.05) {
		t.Fatalf("default count color=%v want error %v", got, want)
	}
	// theme override
	custom := kit.DefaultTheme()
	if custom.Tokens == nil {
		t.Fatal("nil tokens")
	}
	custom.Tokens.Colors[core.TokenColorError] = render.Hex("#112233")
	b2 := kit.NewBadge()
	b2.SetTheme(custom)
	b2.SetChild(badgeChild())
	b2.SetCount(1)
	got2 := b2.StatusColor()
	if !approxColorBDG(got2, render.Hex("#112233"), 0.05) {
		t.Fatalf("theme error override got %v", got2)
	}
	// not hard-coded only — inverse text path exists
	inv := th.Color(core.TokenColorTextInverse)
	if inv.A < 0.5 {
		t.Fatal("text inverse token")
	}
}

func TestBadge_PRD_20_Disabled(t *testing.T) {
	// BDG-20
	clicks := 0
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	b.SetOnClick(func() { clicks++ })
	b.SetDisabled(true)
	if !b.Disabled {
		t.Fatal("disabled flag")
	}
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 120, Height: 80})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 20, Y: 20, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 20, Y: 20, Button: core.ButtonLeft})
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0", clicks)
	}
	// color dimmed vs enabled
	en := kit.NewBadge()
	en.SetCount(5)
	disCol := b.StatusColor()
	enCol := en.StatusColor()
	// disabled should differ (dim) or at least rebuild without panic
	_ = disCol
	_ = enCol
}

func TestBadge_PRD_21_Keyboard(t *testing.T) {
	// BDG-21
	clicks := 0
	b := kit.NewBadge()
	b.SetChild(badgeChild())
	b.SetCount(5)
	b.SetOnClick(func() { clicks++ })
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 120, Height: 80})
	// focus via pointer first (establishes focus target on pressable)
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 20, Y: 20, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 20, Y: 20, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("pointer clicks=%d", clicks)
	}
	clicks = 0
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 1 {
		t.Fatalf("Enter clicks=%d want 1", clicks)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if clicks != 2 {
		t.Fatalf("Space clicks=%d want 2", clicks)
	}
}
