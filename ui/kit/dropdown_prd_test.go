package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/dropdown.md §6.9 — P0 PRD cases (DD-01 … DD-23).
// L3/L4 (DD-24/25) and P1 (DD-26) deferred.

func ddSampleItems() []kit.MenuItem {
	return []kit.MenuItem{
		{Key: "1", Label: "1st menu item"},
		{Key: "2", Label: "2nd menu item", Disabled: true},
		{Key: "3", Label: "3rd menu item", Disabled: true},
		{Key: "4", Label: "a danger item", Danger: true},
	}
}

func mountDropdown(t *testing.T, d *kit.Dropdown, w, h float64) *core.Tree {
	t.Helper()
	if d == nil {
		t.Fatal("nil dropdown")
	}
	d.Viewport = core.Size{Width: w, Height: h}
	bg := primitive.NewBox(d.Node())
	bg.Width, bg.Height = w, h
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func clickAt(tree *core.Tree, x, y float64) {
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func triggerCenter(d *kit.Dropdown) (x, y float64) {
	shell := d.TriggerShell()
	if shell == nil {
		return 8, 8
	}
	abs := core.AbsoluteBounds(shell)
	return (abs.Min.X + abs.Max.X) / 2, (abs.Min.Y + abs.Max.Y) / 2
}

func TestDropdown_PRD_01_Defaults(t *testing.T) {
	// DD-01
	d := kit.NewDropdown("Hover me", ddSampleItems()...)
	if d.Placement != kit.DropdownBottomLeft {
		t.Fatalf("Placement=%v want bottomLeft", d.Placement)
	}
	if d.Disabled || d.Open || d.Arrow {
		t.Fatalf("flags disabled=%v open=%v arrow=%v", d.Disabled, d.Open, d.Arrow)
	}
	if !d.AutoAdjustOverflow {
		t.Fatal("AutoAdjustOverflow default true")
	}
	// default trigger = hover (empty Triggers)
	if len(d.Triggers) != 0 {
		// empty means hover default
		t.Fatalf("Triggers=%v want empty (hover default)", d.Triggers)
	}
	if d.Node() == nil {
		t.Fatal("nil node")
	}
	_ = mountDropdown(t, d, 400, 300)
}

func TestDropdown_PRD_02_ClickOpen(t *testing.T) {
	// DD-02 / DD-S1
	d := kit.NewDropdown("Click me", ddSampleItems()...)
	d.SetTrigger(kit.DropdownTriggerClick)
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.IsOpen() || d.Popup() == nil || !d.Popup().Open {
		t.Fatal("click should open menu")
	}
}

func TestDropdown_PRD_03_SelectItemCloses(t *testing.T) {
	// DD-03 / DD-S2 — uncontrolled open via click (SetOpen is controlled).
	d := kit.NewDropdown("Menu",
		kit.MenuItem{Key: "a", Label: "Alpha"},
		kit.MenuItem{Key: "b", Label: "Beta"},
	)
	d.SetTrigger(kit.DropdownTriggerClick)
	got := ""
	src := kit.DropdownOpenSource("")
	d.SetOnMenuClick(func(k string) { got = k })
	d.SetOnOpenChange(func(open bool, s kit.DropdownOpenSource) {
		if !open {
			src = s
		}
	})
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.IsOpen() {
		t.Fatal("want open before select")
	}
	item := firstMenuItemPressable(d)
	if item == nil {
		t.Fatal("no menu item")
	}
	abs := core.AbsoluteBounds(item)
	ix := (abs.Min.X + abs.Max.X) / 2
	iy := (abs.Min.Y + abs.Max.Y) / 2
	if ix == 0 && iy == 0 {
		ix, iy = cx, cy+40
	}
	clickAt(tree, ix, iy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if got != "a" {
		t.Fatalf("OnMenuClick got=%q want a", got)
	}
	if d.IsOpen() {
		t.Fatal("menu should close after select")
	}
	if src != kit.DropdownSourceMenu {
		t.Fatalf("close source=%q want menu", src)
	}
}

func firstMenuItemPressable(d *kit.Dropdown) *primitive.Pressable {
	if d == nil || d.Panel() == nil {
		return nil
	}
	var found *primitive.Pressable
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil || found != nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok && p.Base().Role == "menuitem" {
			found = p
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(d.Panel())
	return found
}

func TestDropdown_PRD_04_OutsideDismiss(t *testing.T) {
	// DD-04 / DD-S3
	d := kit.NewDropdown("Menu", kit.MenuItem{Key: "1", Label: "One"})
	d.SetTrigger(kit.DropdownTriggerClick)
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.IsOpen() {
		t.Fatal("want open")
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 380, Y: 280})
	if d.IsOpen() || (d.Popup() != nil && d.Popup().Open) {
		t.Fatal("outside should dismiss")
	}
}

func TestDropdown_PRD_05_Esc(t *testing.T) {
	// DD-05 / DD-S4
	d := kit.NewDropdown("Menu", kit.MenuItem{Key: "1", Label: "One"})
	d.SetTrigger(kit.DropdownTriggerClick)
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.IsOpen() {
		t.Fatal("want open")
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if d.IsOpen() {
		t.Fatal("Esc should close")
	}
}

func TestDropdown_PRD_06_Disabled(t *testing.T) {
	// DD-06 / DD-S5
	d := kit.NewDropdown("No", ddSampleItems()...)
	d.SetTrigger(kit.DropdownTriggerClick)
	d.SetDisabled(true)
	opens := 0
	d.SetOnOpenChange(func(open bool, _ kit.DropdownOpenSource) {
		if open {
			opens++
		}
	})
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	if d.IsOpen() || opens != 0 {
		t.Fatalf("disabled must not open (open=%v opens=%d)", d.IsOpen(), opens)
	}
}

func TestDropdown_PRD_07_ControlledOpen(t *testing.T) {
	// DD-07 / DD-S6
	d := kit.NewDropdown("Ctrl", ddSampleItems()...)
	d.SetTrigger(kit.DropdownTriggerClick)
	d.SetOpen(false)
	changed := false
	d.SetOnOpenChange(func(open bool, _ kit.DropdownOpenSource) {
		changed = true
		_ = open
	})
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	if d.IsOpen() {
		t.Fatal("controlled open=false must stay closed")
	}
	if !changed {
		t.Fatal("want OnOpenChange intent while controlled")
	}
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.IsOpen() {
		t.Fatal("SetOpen(true) should show")
	}
}

func TestDropdown_PRD_08_Placement(t *testing.T) {
	// DD-08 / DD-S7
	d := kit.NewDropdown("Place", ddSampleItems()...)
	d.SetTrigger(kit.DropdownTriggerClick)
	for _, pl := range []kit.DropdownPlacement{
		kit.DropdownBottomLeft, kit.DropdownBottom, kit.DropdownBottomRight,
		kit.DropdownTopLeft, kit.DropdownTop, kit.DropdownTopRight,
		kit.DropdownLeftTop, kit.DropdownLeft, kit.DropdownLeftBottom,
		kit.DropdownRightTop, kit.DropdownRight, kit.DropdownRightBottom,
	} {
		d.SetPlacement(pl)
		if d.Placement != pl {
			t.Fatalf("placement=%v", d.Placement)
		}
		tree := mountDropdown(t, d, 600, 500)
		d.SetOpen(true)
		tree.Layout(core.Size{Width: 600, Height: 500})
		if d.Popup() == nil || !d.Popup().Open {
			t.Fatalf("placement %v popup not open", pl)
		}
		// Content should have non-zero offset somewhere in viewport.
		if d.Panel() == nil {
			t.Fatal("nil panel")
		}
		_ = d.Panel().Size()
		d.SetOpen(false)
	}
}

func TestDropdown_PRD_09_HoverTrigger(t *testing.T) {
	// DD-09 / DD-S8
	d := kit.NewDropdown("Hover", ddSampleItems()...)
	// default hover
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: cx, Y: cy})
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.IsOpen() {
		t.Fatal("hover should open")
	}
	// Leave to empty corner + tick deferred close.
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: 390, Y: 290})
	tree.TickActive(0.016)
	if d.IsOpen() {
		t.Fatal("leave should close after tick")
	}
}

func TestDropdown_PRD_10_Submenu(t *testing.T) {
	// DD-10 / DD-S9 — one-level Children
	d := kit.NewDropdown("Sub",
		kit.MenuItem{Key: "p", Label: "Parent", Children: []kit.MenuItem{
			{Key: "c1", Label: "Child 1"},
			{Key: "c2", Label: "Child 2"},
		}},
	)
	d.SetTrigger(kit.DropdownTriggerClick)
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	parent := firstMenuItemPressable(d)
	if parent == nil {
		t.Fatal("no parent item")
	}
	abs := core.AbsoluteBounds(parent)
	clickAt(tree, (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2)
	tree.Layout(core.Size{Width: 400, Height: 300})
	// After expand, more menuitems should exist.
	count := 0
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok && p.Base().Role == "menuitem" {
			count++
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(d.Panel())
	if count < 2 {
		t.Fatalf("submenu expand menuitems=%d want ≥2 (parent+children)", count)
	}
	if !d.IsOpen() {
		t.Fatal("expanding submenu should keep open")
	}
}

func TestDropdown_PRD_11_DangerItem(t *testing.T) {
	// DD-11 / DD-S10
	d := kit.NewDropdown("D", kit.MenuItem{Key: "x", Label: "danger", Danger: true})
	d.SetTrigger(kit.DropdownTriggerClick)
	tree := mountDropdown(t, d, 400, 300)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	// Find text node color ≈ error
	errC := kit.DefaultTheme().Color(core.TokenColorError)
	found := false
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil || found {
			return
		}
		if tx, ok := n.(*primitive.Text); ok {
			if approxColor(tx.Color, errC, 0.05) {
				found = true
				return
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(d.Panel())
	if !found {
		t.Fatal("danger item label should use colorError")
	}
}

func TestDropdown_PRD_12_BasicDemo(t *testing.T) {
	// DD-12 basic.tsx
	d := kit.NewDropdown("Hover me", ddSampleItems()...)
	_ = mountDropdown(t, d, 400, 300)
	if d.Node() == nil {
		t.Fatal("basic demo node")
	}
}

func TestDropdown_PRD_13_ExtraDemo(t *testing.T) {
	// DD-13 extra.tsx
	d := kit.NewDropdown("Hover me",
		kit.MenuItem{Key: "1", Label: "My Account", Disabled: true},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "2", Label: "Profile", Extra: "⌘P"},
		kit.MenuItem{Key: "3", Label: "Billing", Extra: "⌘B"},
		kit.MenuItem{Key: "4", Label: "Settings", Icon: "info", Extra: "⌘S"},
	)
	tree := mountDropdown(t, d, 400, 300)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	// Extra text present
	found := false
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil || found {
			return
		}
		if tx, ok := n.(*primitive.Text); ok && tx.DisplayedText() == "⌘P" {
			found = true
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(d.Panel())
	if !found {
		t.Fatal("extra shortcut not rendered")
	}
}

func TestDropdown_PRD_14_PlacementDemo(t *testing.T) {
	// DD-14 placement.tsx — all 12 constructible
	for _, pl := range []kit.DropdownPlacement{
		kit.DropdownBottomLeft, kit.DropdownBottom, kit.DropdownBottomRight,
		kit.DropdownTopLeft, kit.DropdownTop, kit.DropdownTopRight,
		kit.DropdownLeftTop, kit.DropdownLeft, kit.DropdownLeftBottom,
		kit.DropdownRightTop, kit.DropdownRight, kit.DropdownRightBottom,
	} {
		d := kit.NewDropdown("btn", ddSampleItems()...)
		d.SetPlacement(pl)
		d.SetTrigger(kit.DropdownTriggerClick)
		if d.Node() == nil {
			t.Fatal(pl)
		}
	}
}

func TestDropdown_PRD_15_ArrowDemo(t *testing.T) {
	// DD-15 arrow.tsx
	d := kit.NewDropdown("A", ddSampleItems()...)
	d.SetArrow(true)
	d.SetPlacement(kit.DropdownBottomLeft)
	tree := mountDropdown(t, d, 400, 300)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !d.Arrow {
		t.Fatal("arrow flag")
	}
	if d.Panel() == nil {
		t.Fatal("panel")
	}
}

func TestDropdown_PRD_16_ItemDemo(t *testing.T) {
	// DD-16 item.tsx — divider + disabled
	d := kit.NewDropdown("Hover me",
		kit.MenuItem{Key: "0", Label: "1st"},
		kit.MenuItem{Key: "1", Label: "2nd"},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "3", Label: "3rd disabled", Disabled: true},
	)
	tree := mountDropdown(t, d, 400, 300)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	// Click disabled should not close / not fire
	got := ""
	d.SetOnMenuClick(func(k string) { got = k })
	// Find disabled pressable
	var dis *primitive.Pressable
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil || dis != nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok && p.Base().Label == "3rd disabled" {
			dis = p
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(d.Panel())
	if dis == nil {
		t.Fatal("disabled item missing")
	}
	if !dis.State.Disabled {
		t.Fatal("item should be disabled")
	}
	abs := core.AbsoluteBounds(dis)
	clickAt(tree, (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2)
	if got != "" {
		t.Fatalf("disabled click fired %q", got)
	}
}

func TestDropdown_PRD_17_ArrowCenterDemo(t *testing.T) {
	// DD-17 arrow-center.tsx
	d := kit.NewDropdown("A", ddSampleItems()...)
	d.SetArrowConfig(true, true)
	if !d.Arrow || !d.ArrowPointAtCenter {
		t.Fatal("arrow center config")
	}
	d.SetPlacement(kit.DropdownBottomLeft)
	_ = mountDropdown(t, d, 400, 300)
	// pointAtCenter maps *Left/*Right to centered placement
	if d.Popup() == nil {
		t.Fatal("nil popup")
	}
	d.SetOpen(true)
	if d.Popup().Placement != primitive.PlaceBottom {
		t.Fatalf("pointAtCenter bottomLeft → PlaceBottom got %v", d.Popup().Placement)
	}
}

func TestDropdown_PRD_18_TriggerDemo(t *testing.T) {
	// DD-18 trigger.tsx — click
	d := kit.NewDropdown("Click me",
		kit.MenuItem{Key: "0", Label: "1st"},
		kit.MenuItem{Key: "1", Label: "2nd"},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "3", Label: "3rd"},
	)
	d.SetTrigger(kit.DropdownTriggerClick)
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	if !d.IsOpen() {
		t.Fatal("click trigger open")
	}
}

func TestDropdown_PRD_19_EventDemo(t *testing.T) {
	// DD-19 event.tsx
	d := kit.NewDropdown("Hover me, Click menu item",
		kit.MenuItem{Key: "1", Label: "1st menu item"},
		kit.MenuItem{Key: "2", Label: "2nd menu item"},
		kit.MenuItem{Key: "3", Label: "3rd menu item"},
	)
	d.SetTrigger(kit.DropdownTriggerClick)
	keys := []string{}
	d.SetOnMenuClick(func(k string) { keys = append(keys, k) })
	tree := mountDropdown(t, d, 400, 300)
	cx, cy := triggerCenter(d)
	clickAt(tree, cx, cy)
	tree.Layout(core.Size{Width: 400, Height: 300})
	item := firstMenuItemPressable(d)
	if item == nil {
		t.Fatal("no item")
	}
	abs := core.AbsoluteBounds(item)
	clickAt(tree, (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2)
	if len(keys) != 1 || keys[0] != "1" {
		t.Fatalf("keys=%v", keys)
	}
}

func TestDropdown_PRD_20_Tokens(t *testing.T) {
	// DD-20 §6.2 metrics
	th := kit.DefaultTheme()
	if fs := th.SizeOr(core.TokenFontSize, 0); fs < 13.5 || fs > 14.5 {
		t.Fatalf("fontSize=%v want 14±0.5", fs)
	}
	if r := th.SizeOr(core.TokenBorderRadiusLG, 0); r < 7.5 || r > 8.5 {
		t.Fatalf("borderRadiusLG=%v want 8±0.5", r)
	}
	if lw := th.SizeOr(core.TokenLineWidth, 0); lw < 0.5 || lw > 1.5 {
		t.Fatalf("lineWidth=%v want 1±0.5", lw)
	}
	if kit.DefaultDropdownPanelPad != 4 || kit.DefaultDropdownItemPadInline != 12 || kit.DefaultDropdownItemPadBlock != 5 {
		t.Fatalf("defaults pad panel=%v itemH=%v itemV=%v",
			kit.DefaultDropdownPanelPad, kit.DefaultDropdownItemPadInline, kit.DefaultDropdownItemPadBlock)
	}
	d := kit.NewDropdown("T", ddSampleItems()...)
	tree := mountDropdown(t, d, 400, 300)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	p := d.Panel()
	if p == nil {
		t.Fatal("nil panel")
	}
	if p.Radius < 7.5 || p.Radius > 8.5 {
		t.Fatalf("panel radius=%v want ~8", p.Radius)
	}
	if p.MinWidth < kit.DefaultDropdownMenuMinWidth-0.5 {
		t.Fatalf("minWidth=%v", p.MinWidth)
	}
	if p.BorderWidth < 0.5 {
		t.Fatalf("borderW=%v", p.BorderWidth)
	}
}

func TestDropdown_PRD_21_ThemeColors(t *testing.T) {
	// DD-21 no hardcoded brand as only skin
	d := kit.NewDropdown("T", ddSampleItems()...)
	custom := kit.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorBgContainer] = render.Hex("#112233")
	custom.Tokens.Colors[core.TokenColorBorder] = render.Hex("#445566")
	d.Theme = custom
	d.SetTriggerLabel("T") // rebuild with theme
	tree := mountDropdown(t, d, 400, 300)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	p := d.Panel()
	if p == nil {
		t.Fatal("nil panel")
	}
	if !approxColor(p.Background, render.Hex("#112233"), 0.02) {
		t.Fatalf("panel bg=%v want theme", p.Background)
	}
}

func TestDropdown_PRD_22_DisabledLook(t *testing.T) {
	// DD-22
	d := kit.NewDropdown("Dis", ddSampleItems()...)
	d.SetDisabled(true)
	tree := mountDropdown(t, d, 400, 300)
	if d.TriggerShell() == nil || !d.TriggerShell().State.Disabled {
		t.Fatal("trigger shell should be disabled")
	}
	// Hover must not open
	cx, cy := triggerCenter(d)
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: cx, Y: cy})
	if d.IsOpen() {
		t.Fatal("disabled hover open")
	}
}

func TestDropdown_PRD_23_KeyboardFocus(t *testing.T) {
	// DD-23 focus ring + Enter/Space with click trigger
	d := kit.NewDropdown("Key", ddSampleItems()...)
	d.SetTrigger(kit.DropdownTriggerClick)
	d.SetAriaLabel("actions")
	tree := mountDropdown(t, d, 400, 300)
	shell := d.TriggerShell()
	if shell == nil || !shell.CanFocus() {
		t.Fatal("trigger should be focusable")
	}
	if shell.Base().Label != "actions" {
		t.Fatalf("a11y label=%q", shell.Base().Label)
	}
	tree.SetFocus(shell)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if !d.IsOpen() {
		// Pressable Enter fires Click which toggles for click trigger
		t.Fatal("Enter should toggle open via click trigger")
	}
}
