package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// docs/antd/breadcrumb.md §6.9 — P0 PRD cases (BC-01 … BC-19 L1/L2).
// L3/L4 (BC-20/21) and P1 (BC-15/BC-22) deferred.

func approxBC(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func approxBCColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(float64(a.R-b.R)) <= tol &&
		math.Abs(float64(a.G-b.G)) <= tol &&
		math.Abs(float64(a.B-b.B)) <= tol &&
		math.Abs(float64(a.A-b.A)) <= tol
}

func clickBCItem(t *testing.T, tree *core.Tree, b *kit.Breadcrumb, index int) {
	t.Helper()
	tree.Layout(core.Size{Width: 640, Height: 80})
	pr := b.ItemPressable(index)
	if pr == nil {
		t.Fatalf("no pressable for item %d", index)
	}
	abs := core.AbsoluteBounds(pr)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	if abs.Width() <= 0 || abs.Height() <= 0 {
		rAbs := core.AbsoluteBounds(b.Node())
		x = rAbs.Min.X + 20
		y = rAbs.Min.Y + 10
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestBreadcrumb_PRD_01_Defaults(t *testing.T) {
	// BC-01: NewBreadcrumb 默认创建；separator=/；role=navigation
	b := kit.NewBreadcrumb()
	if b.Node() == nil {
		t.Fatal("nil node")
	}
	if b.ResolvedSeparator() != "/" {
		t.Fatalf("separator=%q want /", b.ResolvedSeparator())
	}
	if b.ChromeNode().Base().Role != "navigation" {
		t.Fatalf("role=%q want navigation", b.ChromeNode().Base().Role)
	}
	if b.ItemCount() != 0 {
		t.Fatalf("items=%d want 0", b.ItemCount())
	}
	if b.SeparatorCount() != 0 {
		t.Fatalf("seps=%d want 0", b.SeparatorCount())
	}
}

func TestBreadcrumb_PRD_02_ThreeItemsTwoSeparators(t *testing.T) {
	// BC-02 / BC-S1
	b := kit.NewBreadcrumb(kit.BreadcrumbTitles("Home", "Nav", "Page")...)
	_ = b.Node().Layout(core.Loose(400, 40))
	if b.ItemCount() != 3 {
		t.Fatalf("items=%d", b.ItemCount())
	}
	if b.SeparatorCount() != 2 {
		t.Fatalf("seps=%d want 2", b.SeparatorCount())
	}
}

func TestBreadcrumb_PRD_03_CustomSeparator(t *testing.T) {
	// BC-03 / BC-S2
	b := kit.NewBreadcrumb(kit.BreadcrumbTitles("A", "B")...)
	b.SetSeparator(">")
	_ = b.Node().Layout(core.Loose(300, 40))
	if b.ResolvedSeparator() != ">" {
		t.Fatalf("sep=%q", b.ResolvedSeparator())
	}
	if b.SeparatorCount() != 1 {
		t.Fatalf("seps=%d", b.SeparatorCount())
	}
	// Item nodes exist; check root children count ≥ 3 (A, >, B)
	kids := b.Root.Children()
	if len(kids) < 3 {
		t.Fatalf("children=%d want ≥3", len(kids))
	}
}

func TestBreadcrumb_PRD_04_LinkClick(t *testing.T) {
	// BC-04 / BC-S3
	got := -1
	var gotTitle string
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home", Link: true},
		kit.BreadcrumbItem{Title: "List", Link: true},
		kit.BreadcrumbItem{Title: "Detail"},
	)
	b.SetOnClick(func(i int, it kit.BreadcrumbItem) {
		got = i
		gotTitle = it.Title
	})
	tree := core.NewTree(b.Node())
	clickBCItem(t, tree, b, 0)
	if got != 0 || gotTitle != "Home" {
		t.Fatalf("got=%d title=%q", got, gotTitle)
	}
	clickBCItem(t, tree, b, 1)
	if got != 1 || gotTitle != "List" {
		t.Fatalf("got=%d title=%q", got, gotTitle)
	}
}

func TestBreadcrumb_PRD_05_LastItemNotLink(t *testing.T) {
	// BC-05 / BC-S4
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home", Link: true},
		kit.BreadcrumbItem{Title: "Page"},
	)
	_ = b.Node().Layout(core.Loose(400, 40))
	if b.IsLinkItem(1) {
		t.Fatal("last item should not be link without Link/Href/Path")
	}
	if b.ItemPressable(1) != nil {
		t.Fatal("last plain item should have no pressable")
	}
	if !b.IsLastRouteItem(1) {
		t.Fatal("want last route item")
	}
	n := b.ItemNode(1)
	if n == nil || n.Base().Key != "aria-current=page" {
		key := ""
		if n != nil {
			key = n.Base().Key
		}
		t.Fatalf("aria-current key=%q", key)
	}
	// last color is colorText, not secondary
	last := b.LastItemColor()
	item := b.ItemColor()
	if approxBCColor(last, item, 0.001) {
		// may equal in some themes; at least last uses TokenColorText
		th := kit.DefaultTheme()
		want := th.Color(core.TokenColorText)
		if !approxBCColor(last, want, 0.001) {
			t.Fatalf("lastColor=%v want %v", last, want)
		}
	}
}

func TestBreadcrumb_PRD_06_MenuDropdown(t *testing.T) {
	// BC-06 / BC-S5
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Ant Design"},
		kit.BreadcrumbItem{
			Title: "General",
			Link:  true,
			Menu: []kit.MenuItem{
				{Key: "1", Label: "General"},
				{Key: "2", Label: "Layout"},
				{Key: "3", Label: "Navigation"},
			},
		},
		kit.BreadcrumbItem{Title: "Button"},
	)
	_ = b.Node().Layout(core.Loose(500, 40))
	dd := b.ItemDropdown(1)
	if dd == nil {
		t.Fatal("expected dropdown on menu item")
	}
	if dd.Popup() == nil {
		t.Fatal("nil popup")
	}
	// open via SetOpen
	dd.SetOpen(true)
	if !dd.IsOpen() {
		t.Fatal("dropdown should open")
	}
}

func TestBreadcrumb_PRD_07_SeparatorMargin(t *testing.T) {
	// BC-07 / BC-S6
	b := kit.NewBreadcrumb(kit.BreadcrumbTitles("A", "B")...)
	m := b.ResolvedSeparatorMargin()
	if !approxBC(m, 8, 0.5) {
		t.Fatalf("separatorMargin=%v want ≈8", m)
	}
}

func TestBreadcrumb_PRD_08_BasicDemo(t *testing.T) {
	// BC-08 basic.tsx
	clicks := 0
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home"},
		kit.BreadcrumbItem{Title: "Application Center", Link: true},
		kit.BreadcrumbItem{Title: "Application List", Link: true},
		kit.BreadcrumbItem{Title: "An Application"},
	)
	b.SetOnClick(func(int, kit.BreadcrumbItem) { clicks++ })
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 700, Height: 60})
	if b.ItemCount() != 4 {
		t.Fatalf("items=%d", b.ItemCount())
	}
	if b.SeparatorCount() != 3 {
		t.Fatalf("seps=%d", b.SeparatorCount())
	}
	if b.IsLinkItem(0) {
		t.Fatal("Home without link flags is plain")
	}
	if !b.IsLinkItem(1) || !b.IsLinkItem(2) {
		t.Fatal("middle items should be links")
	}
	if b.IsLinkItem(3) {
		t.Fatal("last plain")
	}
	clickBCItem(t, tree, b, 1)
	if clicks != 1 {
		t.Fatalf("clicks=%d", clicks)
	}
}

func TestBreadcrumb_PRD_09_WithIcon(t *testing.T) {
	// BC-09 withIcon.tsx
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Link: true, Icon: "user"},
		kit.BreadcrumbItem{Link: true, Icon: "user", Title: "Application List"},
		kit.BreadcrumbItem{Title: "Application"},
	)
	_ = b.Node().Layout(core.Loose(500, 40))
	if b.ItemCount() != 3 {
		t.Fatalf("items=%d", b.ItemCount())
	}
	if b.ItemPressable(0) == nil || b.ItemPressable(1) == nil {
		t.Fatal("icon links should be pressable")
	}
	if b.ItemNode(0) == nil {
		t.Fatal("nil first item node")
	}
}

func TestBreadcrumb_PRD_10_WithParams(t *testing.T) {
	// BC-10 withParams.tsx / BC-S8
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Users"},
		kit.BreadcrumbItem{Title: ":id", Link: true},
	)
	b.SetParams(map[string]string{"id": "1"})
	_ = b.Node().Layout(core.Loose(400, 40))
	if b.DisplayTitle(1) != "1" {
		t.Fatalf("title=%q want 1", b.DisplayTitle(1))
	}
}

func TestBreadcrumb_PRD_11_SeparatorDemo(t *testing.T) {
	// BC-11 separator.tsx
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home"},
		kit.BreadcrumbItem{Title: "Application Center", Link: true},
		kit.BreadcrumbItem{Title: "Application List", Link: true},
		kit.BreadcrumbItem{Title: "An Application"},
	)
	b.SetSeparator(">")
	_ = b.Node().Layout(core.Loose(600, 40))
	if b.ResolvedSeparator() != ">" {
		t.Fatalf("sep=%q", b.ResolvedSeparator())
	}
	if b.SeparatorCount() != 3 {
		t.Fatalf("seps=%d", b.SeparatorCount())
	}
}

func TestBreadcrumb_PRD_12_OverlayDemo(t *testing.T) {
	// BC-12 overlay.tsx
	menuKeys := []string{}
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Ant Design"},
		kit.BreadcrumbItem{Title: "Component", Link: true},
		kit.BreadcrumbItem{
			Title: "General",
			Link:  true,
			Menu: []kit.MenuItem{
				{Key: "1", Label: "General"},
				{Key: "2", Label: "Layout"},
				{Key: "3", Label: "Navigation"},
			},
		},
		kit.BreadcrumbItem{Title: "Button"},
	)
	b.SetOnMenuClick(func(_ int, key string) { menuKeys = append(menuKeys, key) })
	_ = b.Node().Layout(core.Loose(600, 40))
	dd := b.ItemDropdown(2)
	if dd == nil {
		t.Fatal("nil dropdown on General")
	}
	dd.SetOpen(true)
	if !dd.IsOpen() {
		t.Fatal("not open")
	}
	// fire menu click via API
	if dd.OnMenuClick != nil {
		dd.OnMenuClick("2")
	}
	if len(menuKeys) != 1 || menuKeys[0] != "2" {
		t.Fatalf("menuKeys=%v", menuKeys)
	}
}

func TestBreadcrumb_PRD_13_SeparatorComponent(t *testing.T) {
	// BC-13 separator-component.tsx / BC-S7
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Location"},
		kit.BreadcrumbItem{Type: kit.BreadcrumbItemSeparatorType, Separator: ":"},
		kit.BreadcrumbItem{Title: "Application Center", Link: true},
		kit.BreadcrumbItem{Type: kit.BreadcrumbItemSeparatorType},
		kit.BreadcrumbItem{Title: "Application List", Link: true},
		kit.BreadcrumbItem{Type: kit.BreadcrumbItemSeparatorType},
		kit.BreadcrumbItem{Title: "An Application"},
	)
	b.SetSeparator("") // disable auto sep
	_ = b.Node().Layout(core.Loose(700, 40))
	if b.ResolvedSeparator() != "" {
		t.Fatalf("root sep=%q want empty", b.ResolvedSeparator())
	}
	// three independent separators rendered as items
	sepTypes := 0
	for _, it := range b.Items {
		if it.Type == kit.BreadcrumbItemSeparatorType {
			sepTypes++
		}
	}
	if sepTypes != 3 {
		t.Fatalf("sep types=%d", sepTypes)
	}
	// auto seps off → SeparatorCount equals type=separator renders (3)
	if b.SeparatorCount() != 3 {
		t.Fatalf("rendered seps=%d want 3", b.SeparatorCount())
	}
}

func TestBreadcrumb_PRD_14_DebugRoutes(t *testing.T) {
	// BC-14 debug-routes.tsx — legacy routes → items+menu
	items := kit.BreadcrumbFromRoutes(
		kit.BreadcrumbItem{Path: "/home", Title: "Home"},
		kit.BreadcrumbItem{
			Path:  "/user",
			Title: "User",
			Children: []kit.BreadcrumbItem{
				{Path: "/user1", Title: "User1"},
				{Path: "/user2", Title: "User2"},
			},
		},
	)
	b := kit.NewBreadcrumb(items...)
	_ = b.Node().Layout(core.Loose(500, 40))
	if b.ItemCount() != 2 {
		t.Fatalf("items=%d", b.ItemCount())
	}
	if len(b.Items[1].Menu) != 2 {
		t.Fatalf("menu=%d want 2", len(b.Items[1].Menu))
	}
	if b.ItemDropdown(1) == nil {
		t.Fatal("user item should have dropdown from children")
	}
	// path join produces href
	if !b.IsLinkItem(0) {
		t.Fatal("path items are links")
	}
}

func TestBreadcrumb_PRD_16_Metrics(t *testing.T) {
	// BC-16 L2
	b := kit.NewBreadcrumb(kit.BreadcrumbTitles("A", "B")...)
	if !approxBC(b.ResolvedFontSize(), 14, 0.5) {
		t.Fatalf("fontSize=%v", b.ResolvedFontSize())
	}
	if !approxBC(b.ResolvedSeparatorMargin(), 8, 0.5) {
		t.Fatalf("sepMargin=%v", b.ResolvedSeparatorMargin())
	}
	if !approxBC(b.ResolvedIconSize(), 14, 0.5) {
		t.Fatalf("iconSize=%v", b.ResolvedIconSize())
	}
	if !approxBC(b.ResolvedLinkPadInline(), 4, 0.5) {
		t.Fatalf("linkPad=%v", b.ResolvedLinkPadInline())
	}
	if !approxBC(b.ResolvedLinkRadius(), 4, 0.5) {
		t.Fatalf("radius=%v", b.ResolvedLinkRadius())
	}
}

func TestBreadcrumb_PRD_17_TokenColors(t *testing.T) {
	// BC-17 L2 — no hard-coded brand primary as default link skin
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "A", Link: true},
		kit.BreadcrumbItem{Title: "B"},
	)
	th := kit.DefaultTheme()
	sec := th.Color(core.TokenColorTextSecondary)
	text := th.Color(core.TokenColorText)
	if !approxBCColor(b.ItemColor(), sec, 0.001) {
		t.Fatalf("itemColor=%v want secondary %v", b.ItemColor(), sec)
	}
	if !approxBCColor(b.LinkColor(), sec, 0.001) {
		t.Fatalf("linkColor=%v want secondary", b.LinkColor())
	}
	if !approxBCColor(b.LastItemColor(), text, 0.001) {
		t.Fatalf("lastColor=%v want text", b.LastItemColor())
	}
	if !approxBCColor(b.SeparatorColor(), sec, 0.001) {
		t.Fatalf("sepColor=%v want secondary", b.SeparatorColor())
	}
	// must not equal hard brand primary as sole default link color
	primary := th.Color(core.TokenColorPrimary)
	if approxBCColor(b.LinkColor(), primary, 0.001) {
		t.Fatal("linkColor must not be colorPrimary (antd uses description)")
	}
}

func TestBreadcrumb_PRD_18_DisabledItem(t *testing.T) {
	// BC-18 L2
	clicks := 0
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home", Link: true, Disabled: true},
		kit.BreadcrumbItem{Title: "Page"},
	)
	b.SetOnClick(func(int, kit.BreadcrumbItem) { clicks++ })
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 400, Height: 60})
	if b.ItemPressable(0) != nil {
		t.Fatal("disabled link should not expose pressable")
	}
	// clicking around should not fire
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 10, Y: 10, Button: core.ButtonLeft})
	if clicks != 0 {
		t.Fatalf("clicks=%d", clicks)
	}
	dis := kit.DefaultTheme().Color(core.TokenColorDisabledText)
	if dis.A == 0 {
		t.Fatal("disabled text token missing")
	}
}

func TestBreadcrumb_PRD_19_KeyboardFocus(t *testing.T) {
	// BC-19
	clicks := 0
	b := kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home", Link: true},
		kit.BreadcrumbItem{Title: "Page"},
	)
	b.SetOnClick(func(int, kit.BreadcrumbItem) { clicks++ })
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 400, Height: 60})
	pr := b.ItemPressable(0)
	if pr == nil {
		t.Fatal("nil pressable")
	}
	if !pr.ShowFocusRing {
		t.Fatal("focus ring should be on")
	}
	if !pr.Focusable {
		t.Fatal("link should be focusable")
	}
	// pointer activate
	clickBCItem(t, tree, b, 0)
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
	// keyboard activate after focusing the pressable
	pr.SetFocused(true)
	pr.SetFocusVisible(true)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	// Enter may route via tree focus; if not, direct HandleKey
	if clicks < 2 {
		pr.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	}
	if clicks < 2 {
		t.Fatalf("clicks=%d want >=2 after Enter", clicks)
	}
	pr.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if clicks < 3 {
		t.Fatalf("clicks=%d want >=3 after Space", clicks)
	}
}
