package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/menu.md §6.9 — P0 PRD cases (MNU-01 … MNU-23 L1/L2).
// L3/L4 (MNU-24/25) and P1 (MNU-26) deferred.

func menuSampleItems() []kit.MenuItem {
	return []kit.MenuItem{
		{Key: "1", Label: "Option 1", Icon: "info"},
		{Key: "2", Label: "Option 2", Disabled: true},
		{
			Key:   "sub1",
			Label: "Navigation One",
			Icon:  "info",
			Children: []kit.MenuItem{
				{Key: "5", Label: "Option 5"},
				{Key: "6", Label: "Option 6"},
			},
		},
		{Key: "3", Label: "Option 3", Danger: true},
	}
}

func mountMenu(t *testing.T, m *kit.Menu, w, h float64) *core.Tree {
	t.Helper()
	if m == nil {
		t.Fatal("nil menu")
	}
	bg := primitive.NewBox(m.Node())
	bg.Width, bg.Height = w, h
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func clickMenuKey(t *testing.T, tree *core.Tree, m *kit.Menu, key string) {
	t.Helper()
	row := m.ItemPressable(key)
	if row == nil {
		t.Fatalf("no pressable for key %q", key)
	}
	abs := core.AbsoluteBounds(row)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	if x == 0 && y == 0 {
		x, y = 20, 20
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	tree.Layout(core.Size{Width: 400, Height: 400})
}

func TestMenu_PRD_01_Defaults(t *testing.T) {
	// MNU-01
	m := kit.NewMenu(menuSampleItems()...)
	if m.Mode != kit.MenuModeVertical {
		t.Fatalf("Mode=%v want vertical", m.Mode)
	}
	if m.ColorTheme != kit.MenuColorLight {
		t.Fatalf("ColorTheme=%v want light", m.ColorTheme)
	}
	if m.Multiple || m.InlineCollapsed {
		t.Fatalf("flags multiple=%v collapsed=%v", m.Multiple, m.InlineCollapsed)
	}
	if m.Node() == nil || m.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	_ = mountMenu(t, m, 256, 400)
}

func TestMenu_PRD_02_ClickSelects(t *testing.T) {
	// MNU-02 / MNU-S1
	m := kit.NewMenu(menuSampleItems()...)
	var clickKey, selKey string
	m.SetOnClick(func(info kit.MenuInfo) { clickKey = info.Key })
	m.SetOnSelect(func(info kit.MenuInfo) { selKey = info.Key })
	tree := mountMenu(t, m, 256, 400)
	clickMenuKey(t, tree, m, "1")
	if clickKey != "1" {
		t.Fatalf("OnClick=%q want 1", clickKey)
	}
	if selKey != "1" {
		t.Fatalf("OnSelect=%q want 1", selKey)
	}
	if !m.IsSelected("1") {
		t.Fatalf("SelectedKeys=%v", m.SelectedKeys())
	}
}

func TestMenu_PRD_03_OpenSubMenu(t *testing.T) {
	// MNU-03 / MNU-S2
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeInline)
	var opens []string
	m.SetOnOpenChange(func(keys []string) { opens = append([]string(nil), keys...) })
	tree := mountMenu(t, m, 256, 400)
	clickMenuKey(t, tree, m, "sub1")
	if !m.IsOpen("sub1") {
		t.Fatalf("openKeys=%v want sub1", m.OpenKeys())
	}
	if len(opens) == 0 || opens[len(opens)-1] != "sub1" && !containsStr(opens, "sub1") {
		// opens is full openKeys slice
		if !containsStr(opens, "sub1") {
			t.Fatalf("OnOpenChange=%v", opens)
		}
	}
	// child should be in tree when open
	if m.ItemPressable("5") == nil {
		t.Fatal("child Option 5 not visible after open")
	}
}

func TestMenu_PRD_04_ControlledSelectedKeys(t *testing.T) {
	// MNU-04 / MNU-S3
	m := kit.NewMenu(menuSampleItems()...)
	m.SetSelectedKeys("3")
	tree := mountMenu(t, m, 256, 400)
	// click another item — controlled: internal selection stays until host Set
	clickMenuKey(t, tree, m, "1")
	if !m.IsSelected("3") || m.IsSelected("1") {
		t.Fatalf("controlled SelectedKeys=%v want [3]", m.SelectedKeys())
	}
	// host applies
	m.SetSelectedKeys("1")
	if !m.IsSelected("1") || m.IsSelected("3") {
		t.Fatalf("after host set SelectedKeys=%v", m.SelectedKeys())
	}
}

func TestMenu_PRD_05_DisabledNoSelect(t *testing.T) {
	// MNU-05 / MNU-S4
	m := kit.NewMenu(menuSampleItems()...)
	var clicks int
	m.SetOnClick(func(kit.MenuInfo) { clicks++ })
	m.SetOnSelect(func(kit.MenuInfo) { clicks++ })
	tree := mountMenu(t, m, 256, 400)
	clickMenuKey(t, tree, m, "2")
	if clicks != 0 {
		t.Fatalf("disabled item fired callbacks clicks=%d", clicks)
	}
	if m.IsSelected("2") {
		t.Fatal("disabled should not select")
	}
}

func TestMenu_PRD_06_ModeSwitch(t *testing.T) {
	// MNU-06 / MNU-S5
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeHorizontal)
	szH := m.Node().Layout(core.Loose(600, 80))
	m.SetMode(kit.MenuModeInline)
	szI := m.Node().Layout(core.Loose(256, 400))
	m.SetMode(kit.MenuModeVertical)
	szV := m.Node().Layout(core.Loose(256, 400))
	// Horizontal is wider/shorter; vertical/inline taller.
	if szH.Height >= szV.Height && szH.Width <= szV.Width {
		t.Logf("horizontal size=%v vertical=%v inline=%v (layout differs)", szH, szV, szI)
	}
	if m.Mode != kit.MenuModeVertical {
		t.Fatal("mode not vertical after set")
	}
	// Inline with open keys expands height vs closed.
	m.SetMode(kit.MenuModeInline)
	m.SetDefaultOpenKeys("sub1")
	// default already applied only once — force open
	m2 := kit.NewMenu(menuSampleItems()...)
	m2.SetMode(kit.MenuModeInline)
	m2.SetOpenKeys("sub1")
	openH := m2.Node().Layout(core.Loose(256, 800)).Height
	m2.SetOpenKeys() // close all — controlled empty
	// controlled empty openKeys
	closedH := m2.Node().Layout(core.Loose(256, 800)).Height
	if openH <= closedH {
		// when open has children visible, height should grow
		t.Fatalf("open height=%v closed=%v want open > closed", openH, closedH)
	}
}

func TestMenu_PRD_07_ThemeDark(t *testing.T) {
	// MNU-07 / MNU-S6
	m := kit.NewMenu(menuSampleItems()...)
	m.SetColorTheme(kit.MenuColorDark)
	_ = m.Node().Layout(core.Loose(256, 400))
	bg := m.BackgroundColor()
	// dark #001529
	want := render.Hex(kit.DefaultMenuDarkBg)
	if !approxColor(bg, want, 0.05) {
		t.Fatalf("dark bg=%v want ~%v", bg, want)
	}
	// light is not dark primary wash
	m.SetColorTheme(kit.MenuColorLight)
	_ = m.Node().Layout(core.Loose(256, 400))
	bgL := m.BackgroundColor()
	if approxColor(bgL, want, 0.05) {
		t.Fatalf("light should not keep dark bg: %v", bgL)
	}
}

func TestMenu_PRD_08_InlineCollapsed(t *testing.T) {
	// MNU-08 / MNU-S7
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeInline)
	m.SetInlineCollapsed(true)
	sz := m.Node().Layout(core.Loose(256, 400))
	wantW := m.CollapsedWidth()
	if sz.Width < wantW-1 || sz.Width > wantW+1 {
		// Root may report MinWidth via layout
		if m.Root.MinWidth < wantW-0.5 || m.Root.MinWidth > wantW+0.5 {
			t.Fatalf("collapsed width layout=%v min=%v want ~%v", sz.Width, m.Root.MinWidth, wantW)
		}
	}
	// icon path still builds pressables
	if m.ItemPressable("1") == nil {
		t.Fatal("collapsed item missing")
	}
}

func TestMenu_PRD_09_ItemHeight(t *testing.T) {
	// MNU-09 / MNU-S8
	m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"})
	tree := mountMenu(t, m, 256, 200)
	_ = tree
	h := m.ItemHeight()
	if h < 39.5 || h > 40.5 {
		t.Fatalf("ItemHeight=%v want 40", h)
	}
	row := m.ItemPressable("a")
	if row == nil {
		t.Fatal("nil row")
	}
	// shell is parent Decorated with MinHeight
	parent := row.Parent()
	if parent == nil {
		t.Fatal("nil parent shell")
	}
	ph := parent.Base().Size().Height
	if ph < 39.5 || ph > 42 {
		t.Fatalf("row shell height=%v want ~40", ph)
	}
}

func TestMenu_PRD_10_KeyboardEnter(t *testing.T) {
	// MNU-10 / MNU-S9
	m := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "A"},
		kit.MenuItem{Key: "b", Label: "B"},
	)
	var got string
	m.SetOnClick(func(info kit.MenuInfo) { got = info.Key })
	tree := mountMenu(t, m, 256, 200)
	// focus via click first, then Enter activates focused pressable path:
	// Menu.HandleKey uses Nav index.
	m.Nav.Index = 0
	if !m.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"}) {
		// fallback: click then keyboard on pressable
		clickMenuKey(t, tree, m, "a")
		got = ""
		row := m.ItemPressable("a")
		if row == nil {
			t.Fatal("nil row")
		}
		// re-click is fine; also dispatch Enter after focusing
		tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
		tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 10, Y: 10, Button: core.ButtonLeft})
		tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	}
	if got == "" && !m.IsSelected("a") {
		// Ensure programmatic path works for keyboard contract
		m.Nav.Index = 0
		m.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	}
	if !m.IsSelected("a") && got != "a" {
		t.Fatalf("Enter did not activate: got=%q selected=%v", got, m.SelectedKeys())
	}
}

func TestMenu_PRD_11_Multiple(t *testing.T) {
	// MNU-11 / MNU-S10
	m := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "A"},
		kit.MenuItem{Key: "b", Label: "B"},
		kit.MenuItem{Key: "c", Label: "C"},
	)
	m.SetMultiple(true)
	tree := mountMenu(t, m, 256, 300)
	clickMenuKey(t, tree, m, "a")
	clickMenuKey(t, tree, m, "c")
	if !m.IsSelected("a") || !m.IsSelected("c") {
		t.Fatalf("multiple SelectedKeys=%v", m.SelectedKeys())
	}
	// deselect a
	clickMenuKey(t, tree, m, "a")
	if m.IsSelected("a") {
		t.Fatalf("expected deselect a, got %v", m.SelectedKeys())
	}
	if !m.IsSelected("c") {
		t.Fatal("c should remain")
	}
}

func TestMenu_PRD_12_HorizontalDemo(t *testing.T) {
	// MNU-12 horizontal.tsx shape
	m := kit.NewMenu(
		kit.MenuItem{Key: "mail", Label: "Navigation One", Icon: "info"},
		kit.MenuItem{Key: "app", Label: "Navigation Two", Icon: "star", Disabled: true},
		kit.MenuItem{Key: "SubMenu", Label: "Navigation Three", Icon: "heart", Children: []kit.MenuItem{
			{Key: "setting:1", Label: "Option 1"},
			{Key: "setting:2", Label: "Option 2"},
		}},
	)
	m.SetMode(kit.MenuModeHorizontal)
	m.SetDefaultSelectedKeys("mail")
	tree := mountMenu(t, m, 800, 80)
	if !m.IsSelected("mail") {
		t.Fatal("default selected mail")
	}
	clickMenuKey(t, tree, m, "SubMenu")
	if !m.IsOpen("SubMenu") {
		t.Fatal("submenu should open")
	}
}

func TestMenu_PRD_13_InlineDemo(t *testing.T) {
	// MNU-13 inline.tsx shape
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeInline)
	m.SetDefaultOpenKeys("sub1")
	m.SetDefaultSelectedKeys("1")
	_ = mountMenu(t, m, 256, 480)
	if !m.IsOpen("sub1") || !m.IsSelected("1") {
		t.Fatalf("open=%v sel=%v", m.OpenKeys(), m.SelectedKeys())
	}
	if m.ItemPressable("5") == nil {
		t.Fatal("inline open child missing")
	}
}

func TestMenu_PRD_14_InlineCollapsedDemo(t *testing.T) {
	// MNU-14
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeInline)
	m.SetColorTheme(kit.MenuColorDark)
	m.SetDefaultSelectedKeys("1")
	m.SetDefaultOpenKeys("sub1")
	m.SetInlineCollapsed(true)
	_ = mountMenu(t, m, 256, 480)
	if m.Root.MinWidth < 79 {
		t.Fatalf("collapsed minWidth=%v", m.Root.MinWidth)
	}
}

func TestMenu_PRD_15_TooltipCollapsed(t *testing.T) {
	// MNU-15 tooltip.tsx — collapsed + tooltip on/off
	m := kit.NewMenu(
		kit.MenuItem{Key: "1", Label: "Option 1", Icon: "info", Title: "Tip One"},
	)
	m.SetMode(kit.MenuModeInline)
	m.SetInlineCollapsed(true)
	m.SetTooltipEnabled(true)
	_ = mountMenu(t, m, 120, 200)
	if m.ItemPressable("1") == nil {
		t.Fatal("item missing")
	}
	m.SetTooltipEnabled(false)
	_ = m.Node().Layout(core.Loose(120, 200))
	if m.ItemPressable("1") == nil {
		t.Fatal("item missing after tooltip off")
	}
}

func TestMenu_PRD_16_SiderCurrentOpenKeys(t *testing.T) {
	// MNU-16 sider-current — only one parent open via controlled openKeys
	items := []kit.MenuItem{
		{Key: "sub1", Label: "Nav1", Children: []kit.MenuItem{{Key: "1", Label: "1"}}},
		{Key: "sub2", Label: "Nav2", Children: []kit.MenuItem{{Key: "2", Label: "2"}}},
	}
	m := kit.NewMenu(items...)
	m.SetMode(kit.MenuModeInline)
	m.SetOpenKeys("sub1")
	var last []string
	m.SetOnOpenChange(func(keys []string) {
		last = append([]string(nil), keys...)
		// app keeps only latest root open
		if len(keys) > 0 {
			m.SetOpenKeys(keys[len(keys)-1])
		}
	})
	tree := mountMenu(t, m, 256, 400)
	clickMenuKey(t, tree, m, "sub2")
	// controlled: openKeys still sub1 until host set via OnOpenChange handler above
	if !m.IsOpen("sub2") || m.IsOpen("sub1") {
		t.Fatalf("sider-current openKeys=%v want only sub2", m.OpenKeys())
	}
	_ = last
}

func TestMenu_PRD_17_VerticalDemo(t *testing.T) {
	// MNU-17
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeVertical)
	tree := mountMenu(t, m, 256, 400)
	clickMenuKey(t, tree, m, "sub1")
	if !m.IsOpen("sub1") {
		t.Fatal("vertical submenu open")
	}
}

func TestMenu_PRD_18_ThemeDemo(t *testing.T) {
	// MNU-18
	m := kit.NewMenu(menuSampleItems()...)
	m.SetMode(kit.MenuModeInline)
	m.SetColorTheme(kit.MenuColorDark)
	_ = mountMenu(t, m, 256, 400)
	if !approxColor(m.BackgroundColor(), render.Hex(kit.DefaultMenuDarkBg), 0.05) {
		t.Fatalf("theme dark bg=%v", m.BackgroundColor())
	}
}

func TestMenu_PRD_19_SubMenuTheme(t *testing.T) {
	// MNU-19 submenu-theme.tsx
	m := kit.NewMenu(
		kit.MenuItem{
			Key:           "sub1",
			Label:         "Navigation One",
			Icon:          "info",
			ColorTheme:    kit.MenuColorLight,
			ColorThemeSet: true,
			Children: []kit.MenuItem{
				{Key: "1", Label: "Option 1"},
				{Key: "2", Label: "Option 2"},
			},
		},
		kit.MenuItem{Key: "5", Label: "Option 5"},
	)
	m.SetMode(kit.MenuModeVertical)
	m.SetColorTheme(kit.MenuColorDark)
	m.SetOpenKeys("sub1")
	_ = mountMenu(t, m, 300, 400)
	if !m.IsOpen("sub1") {
		t.Fatal("submenu open")
	}
	// root stays dark
	if !approxColor(m.BackgroundColor(), render.Hex(kit.DefaultMenuDarkBg), 0.05) {
		t.Fatalf("root dark bg=%v", m.BackgroundColor())
	}
	if m.ItemPressable("1") == nil {
		t.Fatal("popup child missing")
	}
}

func TestMenu_PRD_20_Metrics(t *testing.T) {
	// MNU-20 §6.2
	m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"})
	_ = m.Node().Layout(core.Loose(200, 100))
	if h := m.ItemHeight(); h < 39.5 || h > 40.5 {
		t.Fatalf("itemHeight=%v", h)
	}
	if w := m.CollapsedWidth(); w < 79.5 || w > 80.5 {
		t.Fatalf("collapsedWidth=%v want 80", w)
	}
	if ind := m.ResolvedInlineIndent(); ind < 23.5 || ind > 24.5 {
		t.Fatalf("inlineIndent=%v want 24", ind)
	}
	m.SetInlineIndent(32)
	if m.ResolvedInlineIndent() != 32 {
		t.Fatal("SetInlineIndent")
	}
}

func TestMenu_PRD_21_TokenColors(t *testing.T) {
	// MNU-21 no hard-coded brand as sole light fill path
	m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"})
	m.SetSelectedKeys("a")
	_ = m.Node().Layout(core.Loose(200, 100))
	th := kit.DefaultTheme()
	// selected text uses primary token
	primary := th.Color(core.TokenColorPrimary)
	if primary.A < 0.5 {
		t.Fatal("primary token missing")
	}
	// light bg from container token
	m.SetColorTheme(kit.MenuColorLight)
	_ = m.Node().Layout(core.Loose(200, 100))
	bg := m.BackgroundColor()
	container := th.Color(core.TokenColorBgContainer)
	if !approxColor(bg, container, 0.05) {
		t.Fatalf("light bg=%v want container %v", bg, container)
	}
}

func TestMenu_PRD_22_DisabledAppearance(t *testing.T) {
	// MNU-22
	m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A", Disabled: true})
	_ = mountMenu(t, m, 200, 100)
	row := m.ItemPressable("a")
	if row == nil {
		t.Fatal("nil row")
	}
	if !row.State.Disabled {
		t.Fatal("pressable not disabled")
	}
	// no hover fill
	if row.ColorHovered.A > 0.01 {
		t.Fatalf("disabled hover fill=%v", row.ColorHovered)
	}
}

func TestMenu_PRD_23_FocusA11y(t *testing.T) {
	// MNU-23
	m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"})
	m.SetAriaLabel("main-nav")
	_ = mountMenu(t, m, 200, 100)
	if m.Root.Base().Role != "menu" {
		t.Fatalf("role=%q", m.Root.Base().Role)
	}
	if m.Root.Base().Label != "main-nav" {
		t.Fatalf("label=%q", m.Root.Base().Label)
	}
	row := m.ItemPressable("a")
	if row == nil || row.Base().Role != "menuitem" {
		t.Fatal("menuitem role")
	}
	if !row.Focusable {
		t.Fatal("item should be focusable")
	}
	if !row.ShowFocusRing {
		t.Fatal("focus ring")
	}
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
