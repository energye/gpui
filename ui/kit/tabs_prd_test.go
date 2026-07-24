package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/tabs.md §6.9 — P0 PRD cases (TAB-01 … TAB-23 L1/L2).
// L3/L4 (TAB-24/25) and P1 (TAB-26) deferred.

func approxTabs(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func sampleTabs() *kit.Tabs {
	return kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: kit.NewText("Content of Tab Pane 1").Node()},
		kit.TabItem{Key: "2", Label: "Tab 2", Children: kit.NewText("Content of Tab Pane 2").Node()},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: kit.NewText("Content of Tab Pane 3").Node()},
	)
}

func clickTab(t *testing.T, tree *core.Tree, tabs *kit.Tabs, key string) {
	t.Helper()
	tree.Layout(core.Size{Width: 720, Height: 320})
	pr := tabs.ItemPressable(key)
	if pr == nil {
		t.Fatalf("no pressable key=%s", key)
	}
	abs := core.AbsoluteBounds(pr)
	if abs.Width() <= 0 || abs.Height() <= 0 {
		t.Fatalf("empty bounds key=%s size=%v", key, abs)
	}
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestTabs_PRD_01_Defaults(t *testing.T) {
	// TAB-01
	tabs := kit.NewTabs(
		kit.TabItem{Key: "a", Label: "A"},
		kit.TabItem{Key: "b", Label: "B"},
	)
	if tabs.Node() == nil {
		t.Fatal("nil node")
	}
	if tabs.Type != kit.TabsLine {
		t.Fatalf("Type=%v want line", tabs.Type)
	}
	if tabs.Size != kit.TabsMiddle {
		t.Fatalf("Size=%v want middle", tabs.Size)
	}
	if tabs.Placement != kit.TabsTop {
		t.Fatalf("Placement=%v want top", tabs.Placement)
	}
	if tabs.ActiveKey != "a" {
		t.Fatalf("ActiveKey=%q want first selectable a", tabs.ActiveKey)
	}
	if tabs.Centered || tabs.HideAdd || tabs.DestroyOnHidden {
		t.Fatalf("flags should be false")
	}
	if tabs.ChromeNode().Base().Role != "tablist" {
		t.Fatalf("role=%q want tablist", tabs.ChromeNode().Base().Role)
	}
	if g := tabs.HorizontalGutter(); !approxTabs(g, kit.DefaultTabsHorizontalGutter, 0.5) {
		t.Fatalf("gutter=%v want %v", g, kit.DefaultTabsHorizontalGutter)
	}
}

func TestTabs_PRD_02_ClickSecondTab(t *testing.T) {
	// TAB-02 / TAB-S1
	tabs := sampleTabs()
	var got string
	var n int
	tabs.SetOnChange(func(key string) {
		n++
		got = key
	})
	tree := core.NewTree(tabs.Node())
	clickTab(t, tree, tabs, "2")
	if n != 1 || got != "2" {
		t.Fatalf("onChange n=%d got=%q", n, got)
	}
	if tabs.ActiveKey != "2" {
		t.Fatalf("ActiveKey=%q want 2", tabs.ActiveKey)
	}
	if tabs.Content("2") == nil {
		t.Fatal("panel 2 missing")
	}
}

func TestTabs_PRD_03_ControlledActiveKey(t *testing.T) {
	// TAB-03 / TAB-S2
	tabs := sampleTabs()
	tabs.SetActiveKey("1")
	var got string
	tabs.SetOnChange(func(key string) { got = key })
	tree := core.NewTree(tabs.Node())
	clickTab(t, tree, tabs, "2")
	if got != "2" {
		t.Fatalf("onChange=%q want 2", got)
	}
	if tabs.ActiveKey != "1" {
		t.Fatalf("controlled ActiveKey=%q want stay 1", tabs.ActiveKey)
	}
	// parent commits
	tabs.SetActiveKey("2")
	if tabs.ActiveKey != "2" {
		t.Fatalf("after SetActiveKey ActiveKey=%q", tabs.ActiveKey)
	}
}

func TestTabs_PRD_04_DisabledTab(t *testing.T) {
	// TAB-04 / TAB-S3
	tabs := kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1"},
		kit.TabItem{Key: "2", Label: "Tab 2", Disabled: true},
		kit.TabItem{Key: "3", Label: "Tab 3"},
	)
	var n int
	tabs.SetOnChange(func(string) { n++ })
	if tabs.ItemPressable("2") != nil {
		t.Fatal("disabled tab should not be pressable")
	}
	// programmatic activate blocked
	tabs.SetActive("2")
	if tabs.ActiveKey == "2" {
		t.Fatal("disabled key activated")
	}
	if n != 0 {
		t.Fatalf("onChange fired n=%d", n)
	}
}

func TestTabs_PRD_05_TypeCard(t *testing.T) {
	// TAB-05 / TAB-S4
	tabs := sampleTabs()
	tabs.SetType(kit.TabsCard)
	if !tabs.IsCard() {
		t.Fatal("IsCard false")
	}
	if tabs.InkVisible() {
		t.Fatal("card should hide line ink")
	}
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	host := tabs.ItemHost("1")
	if host == nil {
		t.Fatal("nil host")
	}
	if host.BorderWidth < 1 {
		t.Fatalf("card border=%v want >=1", host.BorderWidth)
	}
}

func TestTabs_PRD_06_EditableCardRemove(t *testing.T) {
	// TAB-06 / TAB-S5
	tabs := kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1"},
		kit.TabItem{Key: "2", Label: "Tab 2"},
	)
	tabs.SetType(kit.TabsEditableCard)
	var target, action string
	var n int
	tabs.SetOnEdit(func(k, a string) {
		n++
		target, action = k, a
	})
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	cp := tabs.ClosePressable("1")
	if cp == nil {
		t.Fatal("nil close pressable")
	}
	// Prefer pointer path; fall back to Click if nested layout bounds are empty.
	abs := core.AbsoluteBounds(cp)
	if abs.Width() > 0 && abs.Height() > 0 {
		x := (abs.Min.X + abs.Max.X) / 2
		y := (abs.Min.Y + abs.Max.Y) / 2
		tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
		tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	} else if cp.Click != nil {
		cp.Click()
	}
	if n != 1 || action != "remove" || target != "1" {
		t.Fatalf("onEdit n=%d target=%q action=%q bounds=%v", n, target, action, abs)
	}
	// add button also wires onEdit(add)
	if ap := tabs.AddPressable(); ap == nil {
		t.Fatal("nil add pressable")
	}
}

func TestTabs_PRD_07_PlacementLeft(t *testing.T) {
	// TAB-07 / TAB-S6
	tabs := sampleTabs()
	tabs.SetPlacement(kit.TabsLeft)
	tabs.SetTabWidth(160)
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 800, Height: 400})
	if tabs.Root.Axis != core.AxisHorizontal {
		t.Fatalf("root axis=%v want horizontal (left rail)", tabs.Root.Axis)
	}
	// first child is rail
	kids := tabs.Root.Children()
	if len(kids) < 1 {
		t.Fatal("no children")
	}
	rail, ok := kids[0].(*primitive.Decorated)
	if !ok {
		t.Fatalf("rail type %T", kids[0])
	}
	if rail.Size().Width < 150 || rail.Size().Width > 170 {
		t.Fatalf("rail width=%v want ~160", rail.Size().Width)
	}
}

func TestTabs_PRD_08_LineInk(t *testing.T) {
	// TAB-08 / TAB-S7
	tabs := sampleTabs()
	if !tabs.InkVisible() {
		t.Fatal("line ink should be visible")
	}
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	// After layout, ink slots for active exist
	if tabs.ItemHost("1") == nil {
		t.Fatal("active host missing")
	}
}

func TestTabs_PRD_09_Keyboard(t *testing.T) {
	// TAB-09 / TAB-S8
	tabs := sampleTabs()
	var got string
	tabs.SetOnChange(func(k string) { got = k })
	_ = core.NewTree(tabs.Node())
	if !tabs.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"}) {
		t.Fatal("ArrowRight not handled")
	}
	if tabs.ActiveKey != "2" || got != "2" {
		t.Fatalf("ActiveKey=%q got=%q want 2", tabs.ActiveKey, got)
	}
	tabs.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowLeft"})
	if tabs.ActiveKey != "1" {
		t.Fatalf("ActiveKey=%q want 1", tabs.ActiveKey)
	}
}

func TestTabs_PRD_10_DestroyOnHidden(t *testing.T) {
	// TAB-10 / TAB-S9
	tabs := sampleTabs()
	tabs.SetDestroyOnHidden(true)
	tabs.SetActive("1")
	if !tabs.IsMounted("1") {
		t.Fatal("active should be mounted")
	}
	tabs.SetActive("2")
	if tabs.IsMounted("1") {
		t.Fatal("hidden panel should unmount when DestroyOnHidden")
	}
	if !tabs.IsMounted("2") {
		t.Fatal("active 2 should mount")
	}
}

func TestTabs_PRD_11_SizeLadder(t *testing.T) {
	// TAB-11 / TAB-S10
	tabs := sampleTabs()
	mid := tabs.ItemHeight()
	tabs.SetSize(kit.TabsSmall)
	sm := tabs.ItemHeight()
	tabs.SetSize(kit.TabsLarge)
	lg := tabs.ItemHeight()
	if !(sm < mid && mid <= lg) {
		t.Fatalf("size ladder sm=%v mid=%v lg=%v", sm, mid, lg)
	}
	if !approxTabs(mid, kit.DefaultTabsCardHeight, 0.5) {
		t.Fatalf("middle height=%v want %v", mid, kit.DefaultTabsCardHeight)
	}
}

func TestTabs_PRD_12_DemoBasic(t *testing.T) {
	// TAB-12 basic.tsx
	tabs := kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: kit.NewText("Content of Tab Pane 1").Node()},
		kit.TabItem{Key: "2", Label: "Tab 2", Children: kit.NewText("Content of Tab Pane 2").Node()},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: kit.NewText("Content of Tab Pane 3").Node()},
	)
	tree := core.NewTree(tabs.Node())
	clickTab(t, tree, tabs, "2")
	if tabs.ActiveKey != "2" {
		t.Fatal(tabs.ActiveKey)
	}
}

func TestTabs_PRD_13_DemoDisabled(t *testing.T) {
	// TAB-13 disabled.tsx
	tabs := kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1"},
		kit.TabItem{Key: "2", Label: "Tab 2", Disabled: true},
		kit.TabItem{Key: "3", Label: "Tab 3"},
	)
	if tabs.ItemPressable("2") != nil {
		t.Fatal("disabled pressable")
	}
	tree := core.NewTree(tabs.Node())
	clickTab(t, tree, tabs, "3")
	if tabs.ActiveKey != "3" {
		t.Fatal(tabs.ActiveKey)
	}
}

func TestTabs_PRD_14_DemoCentered(t *testing.T) {
	// TAB-14 centered.tsx
	tabs := sampleTabs()
	tabs.SetCentered(true)
	if !tabs.Centered {
		t.Fatal("Centered false")
	}
	_ = tabs.Node()
	if !tabs.BarCentered() {
		t.Fatal("bar list should MainCenter when centered")
	}
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 640, Height: 200})
	if tabs.Root == nil {
		t.Fatal("nil root")
	}
}

func TestTabs_PRD_15_DemoIcon(t *testing.T) {
	// TAB-15 icon.tsx
	tabs := kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Icon: "info"},
		kit.TabItem{Key: "2", Label: "Tab 2", Icon: "star"},
	)
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	if tabs.ItemPressable("1") == nil {
		t.Fatal("no pressable with icon")
	}
	clickTab(t, tree, tabs, "2")
	if tabs.ActiveKey != "2" {
		t.Fatal(tabs.ActiveKey)
	}
}

func TestTabs_PRD_16_DemoIndicator(t *testing.T) {
	// TAB-16 custom-indicator.tsx
	tabs := sampleTabs()
	tabs.SetIndicator(kit.TabsIndicator{Size: 20, Align: kit.TabsIndicatorStart})
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	if tabs.Indicator.Size != 20 {
		t.Fatal(tabs.Indicator.Size)
	}
	if tabs.Indicator.Align != kit.TabsIndicatorStart {
		t.Fatal(tabs.Indicator.Align)
	}
}

func TestTabs_PRD_17_DemoSlide(t *testing.T) {
	// TAB-17 slide.tsx — many tabs; switch among them (overflow is host/scroll concern)
	items := make([]kit.TabItem, 0, 12)
	for i := 1; i <= 12; i++ {
		k := string(rune('a' + i - 1))
		items = append(items, kit.TabItem{Key: k, Label: "Tab-" + k})
	}
	tabs := kit.NewTabs(items...)
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 480, Height: 200})
	target := items[5].Key
	pr := tabs.ItemPressable(target)
	if pr == nil {
		t.Fatalf("missing pressable %q", target)
	}
	if pr.Click != nil {
		pr.Click()
	}
	if tabs.ActiveKey != target {
		// pointer path as fallback
		clickTab(t, tree, tabs, target)
	}
	if tabs.ActiveKey != target {
		t.Fatalf("ActiveKey=%q want %q", tabs.ActiveKey, target)
	}
}

func TestTabs_PRD_18_DemoExtra(t *testing.T) {
	// TAB-18 extra.tsx
	tabs := sampleTabs()
	left := kit.NewText("Left Extra").Node()
	right := kit.NewButton("Extra Action").Node()
	tabs.SetTabBarExtraContent(left, right)
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 720, Height: 200})
	if tabs.ExtraLeft == nil || tabs.ExtraRight == nil {
		t.Fatal("extra content missing")
	}
}

func TestTabs_PRD_19_DemoSize(t *testing.T) {
	// TAB-19 size.tsx
	for _, sz := range []kit.TabsSize{kit.TabsSmall, kit.TabsMiddle, kit.TabsLarge} {
		tabs := sampleTabs()
		tabs.SetSize(sz)
		h := tabs.ItemHeight()
		if h < 20 {
			t.Fatalf("size=%v height=%v", sz, h)
		}
		tree := core.NewTree(tabs.Node())
		tree.Layout(core.Size{Width: 600, Height: 200})
	}
}

func TestTabs_PRD_20_Metrics(t *testing.T) {
	// TAB-20 §6.2
	tabs := sampleTabs()
	if !approxTabs(tabs.HorizontalGutter(), 32, 0.5) {
		t.Fatalf("horizontalItemGutter=%v want 32", tabs.HorizontalGutter())
	}
	if !approxTabs(tabs.ItemHeight(), 40, 0.5) {
		t.Fatalf("card/line middle height=%v want 40", tabs.ItemHeight())
	}
	th := kit.DefaultTheme()
	if !approxTabs(th.SizeOr(core.TokenControlHeight, 0), 32, 0.5) {
		t.Fatal("controlHeight token")
	}
	if !approxTabs(th.SizeOr(core.TokenControlHeightLG, 0), 40, 0.5) {
		t.Fatal("controlHeightLG token")
	}
	if !approxTabs(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatal("borderRadius token")
	}
}

func TestTabs_PRD_21_TokenColors(t *testing.T) {
	// TAB-21
	tabs := sampleTabs()
	tabs.SetActive("1")
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	// ink color defaults to primary token
	if tabs.TabInkColor.A > 0 {
		t.Fatal("default ink should not hardcode TabInkColor")
	}
	// active pressable uses theme (not a baked brand-only path without token)
	if primary.A <= 0 {
		t.Fatal("primary token empty")
	}
	// Ensure no forced hex skin independent of theme: changing theme primary rebuilds ink path
	custom := kit.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	tabs.SetTheme(custom)
	_ = tabs.Node()
}

func TestTabs_PRD_22_DisabledAppearance(t *testing.T) {
	// TAB-22
	tabs := kit.NewTabs(
		kit.TabItem{Key: "1", Label: "On"},
		kit.TabItem{Key: "2", Label: "Off", Disabled: true},
	)
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	if tabs.ItemPressable("2") != nil {
		t.Fatal("disabled should not expose pressable / hover chrome")
	}
}

func TestTabs_PRD_23_KeyboardFocus(t *testing.T) {
	// TAB-23
	tabs := sampleTabs()
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	pr := tabs.ItemPressable("1")
	if pr == nil {
		t.Fatal("nil pressable")
	}
	if !pr.Focusable {
		t.Fatal("tab should be focusable")
	}
	if !pr.ShowFocusRing {
		t.Fatal("focus ring should be enabled")
	}
	if pr.Base().Role != "tab" {
		t.Fatalf("role=%q want tab", pr.Base().Role)
	}
	// keyboard activation path
	tabs.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"})
	if tabs.ActiveKey != "2" {
		t.Fatal(tabs.ActiveKey)
	}
}
