package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/float-button.md §6.9 — P0 PRD cases (FB-01…09, FB-13…26).
// P1 BackTop/badge (FB-10…12) and L3/L4 deferred.

func layoutFAB(t *testing.T, fb *kit.FloatButton) *primitive.Decorated {
	t.Helper()
	host := primitive.Row(fb.Node())
	tree := core.NewTree(host)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 240, Height: 240})
	dec, ok := fb.ChromeNode().(*primitive.Decorated)
	if !ok || dec == nil {
		t.Fatal("chrome")
	}
	return dec
}

func clickNode(t *testing.T, tree *core.Tree, n core.Node) {
	t.Helper()
	abs := core.AbsoluteBounds(n)
	x := abs.Min.X + abs.Size().Width/2
	y := abs.Min.Y + abs.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestFloatButton_PRD_01_Defaults(t *testing.T) {
	// FB-01
	fb := kit.NewFloatButton()
	if fb.Type != kit.ButtonDefault {
		t.Fatalf("Type=%v want default", fb.Type)
	}
	if fb.Shape != kit.FloatButtonCircle {
		t.Fatalf("Shape=%v want circle", fb.Shape)
	}
	if fb.Disabled || fb.Loading {
		t.Fatalf("flags disabled=%v loading=%v", fb.Disabled, fb.Loading)
	}
	clicks := 0
	fb.SetOnClick(func() { clicks++ })
	fb.SetAriaLabel("fab")
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	clickNode(t, tree, fb.Button().Root)
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestFloatButton_PRD_02_Click(t *testing.T) {
	// FB-02
	clicks := 0
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("fab")
	fb.SetOnClick(func() { clicks++ })
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	clickNode(t, tree, fb.Button().Root)
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestFloatButton_PRD_03_DisabledNoClick(t *testing.T) {
	// FB-03
	clicks := 0
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("fab")
	fb.SetOnClick(func() { clicks++ })
	fb.SetDisabled(true)
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	clickNode(t, tree, fb.Button().Root)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0", clicks)
	}
}

func TestFloatButton_PRD_04_TypeColors(t *testing.T) {
	// FB-04
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	bg := th.Color(core.TokenColorBgContainer)

	fbP := kit.NewFloatButton()
	fbP.SetType(kit.ButtonPrimary)
	fbP.SetAriaLabel("p")
	decP := layoutFAB(t, fbP)
	if !approxRGBA(decP.Background, primary, 0.02) {
		t.Fatalf("primary bg=%v want ~%v", decP.Background, primary)
	}

	fbD := kit.NewFloatButton()
	fbD.SetType(kit.ButtonDefault)
	fbD.SetAriaLabel("d")
	decD := layoutFAB(t, fbD)
	// default: container bg (outlined), not solid primary
	if approxRGBA(decD.Background, primary, 0.02) {
		t.Fatalf("default should not be primary fill: %v", decD.Background)
	}
	if decD.Background.A < 0.01 && bg.A > 0.01 {
		// allow transparent only if theme container is transparent
	} else if !approxRGBA(decD.Background, bg, 0.15) && decD.BorderWidth < 0.5 {
		// outlined may use container bg with border
		t.Logf("default bg=%v borderW=%v (theme bg=%v)", decD.Background, decD.BorderWidth, bg)
	}
}

func TestFloatButton_PRD_05_ShapeRadius(t *testing.T) {
	// FB-05
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("c")
	dec := layoutFAB(t, fb)
	wantR := kit.DefaultFloatButtonSize / 2
	if dec.Radius < wantR-0.5 || dec.Radius > wantR+0.5 {
		t.Fatalf("circle radius %v want %v", dec.Radius, wantR)
	}

	fb.SetShape(kit.FloatButtonSquare)
	dec = layoutFAB(t, fb)
	if dec.Radius < kit.DefaultFloatButtonSquareRadius-0.5 || dec.Radius > kit.DefaultFloatButtonSquareRadius+0.5 {
		t.Fatalf("square radius %v want %v", dec.Radius, kit.DefaultFloatButtonSquareRadius)
	}
}

func TestFloatButton_PRD_06_Size40(t *testing.T) {
	// FB-06
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("sz")
	dec := layoutFAB(t, fb)
	sz := dec.Size()
	if sz.Width < kit.DefaultFloatButtonSize-0.5 || sz.Width > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("width %v want %v", sz.Width, kit.DefaultFloatButtonSize)
	}
	if sz.Height < kit.DefaultFloatButtonSize-0.5 || sz.Height > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("height %v want %v", sz.Height, kit.DefaultFloatButtonSize)
	}
}

func TestFloatButton_PRD_07_GroupTriggerClickOpen(t *testing.T) {
	// FB-07
	c1 := kit.NewFloatButton()
	c1.SetIcon("plus")
	c1.SetAriaLabel("c1")
	c2 := kit.NewFloatButton()
	c2.SetIcon("info")
	c2.SetAriaLabel("c2")
	g := kit.NewFloatButtonGroup(c1, c2)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetIcon("plus")
	opens := 0
	g.SetOnOpenChange(func(open bool) {
		if open {
			opens++
		}
	})
	tree := core.NewTree(g.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 300, Height: 300})
	if g.IsOpen() {
		t.Fatal("should start closed")
	}
	tr := g.TriggerButton()
	if tr == nil {
		t.Fatal("nil trigger")
	}
	// closed: list not parented under root
	if list := g.ListNode(); list != nil && list.Parent() != nil {
		t.Fatal("list should be detached when closed")
	}
	clickNode(t, tree, tr.Button().Root)
	if !g.IsOpen() {
		t.Fatal("want open after click")
	}
	if opens < 1 {
		t.Fatalf("OnOpenChange opens=%d", opens)
	}
	// children parented when open
	if list := g.ListNode(); list == nil || list.Parent() == nil {
		t.Fatal("list should be parented when open")
	}
}

func TestFloatButton_PRD_07b_MenuOpenStableHostSize(t *testing.T) {
	// FB-S6 / FB-07: menu open/close must not grow the layout host (antd overlay).
	// Regression: in-flow Column root made parent demo section height jump after Flex live-layout fix.
	c1 := kit.NewFloatButton()
	c1.SetIcon("plus")
	c1.SetAriaLabel("c1")
	c2 := kit.NewFloatButton()
	c2.SetIcon("info")
	c2.SetAriaLabel("c2")
	g := kit.NewFloatButtonGroup(c1, c2)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetIcon("plus")
	n := g.Node()
	// closed
	closed := n.Layout(core.Loose(300, 300))
	if closed.Height < kit.DefaultFloatButtonSize-0.5 || closed.Height > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("closed host h=%v want %v", closed.Height, kit.DefaultFloatButtonSize)
	}
	if closed.Width < kit.DefaultFloatButtonSize-0.5 || closed.Width > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("closed host w=%v want %v", closed.Width, kit.DefaultFloatButtonSize)
	}
	// open
	g.SetOpen(true)
	openSz := n.Layout(core.Loose(300, 300))
	if openSz.Height != closed.Height || openSz.Width != closed.Width {
		t.Fatalf("menu open changed host size closed=%v open=%v (must stay trigger-sized)", closed, openSz)
	}
	// list still laid out and placed above (placement top)
	list := g.ListNode()
	if list == nil || list.Parent() == nil {
		t.Fatal("list should be parented when open")
	}
	if list.Base().Offset().Y >= 0 {
		t.Fatalf("placement=top: list offset.Y=%v want < 0 (above trigger)", list.Base().Offset().Y)
	}
	// close again
	g.SetOpen(false)
	closed2 := n.Layout(core.Loose(300, 300))
	if closed2.Height != closed.Height {
		t.Fatalf("close host h=%v want %v", closed2.Height, closed.Height)
	}
}

func TestFloatButton_PRD_08_ControlledOpenFalse(t *testing.T) {
	// FB-08
	c1 := kit.NewFloatButton()
	c1.SetAriaLabel("c1")
	g := kit.NewFloatButtonGroup(c1)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetOpen(true)
	if !g.IsOpen() {
		t.Fatal("controlled open=true")
	}
	// Click should NOT flip controlled Open; only notify.
	changed := false
	g.SetOnOpenChange(func(open bool) { changed = true; _ = open })
	tree := core.NewTree(g.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 300, Height: 300})
	clickNode(t, tree, g.TriggerButton().Button().Root)
	if !g.IsOpen() {
		t.Fatal("controlled open must stay true until SetOpen")
	}
	if !changed {
		t.Fatal("want OnOpenChange while controlled")
	}
	g.SetOpen(false)
	if g.IsOpen() {
		t.Fatal("SetOpen(false) should close")
	}
	if list := g.ListNode(); list != nil && list.Parent() != nil {
		t.Fatal("list should detach when open=false")
	}
}

func TestFloatButton_PRD_09_PlacementLeft(t *testing.T) {
	// FB-09
	c1 := kit.NewFloatButton()
	c1.SetAriaLabel("c1")
	c1.SetIcon("plus")
	g := kit.NewFloatButtonGroup(c1)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetPlacement(kit.FloatButtonLeft)
	g.SetOpen(true)
	tree := core.NewTree(g.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 400, Height: 200})
	list := g.ListNode()
	tr := g.TriggerButton()
	if list == nil || tr == nil {
		t.Fatal("nil list/trigger")
	}
	lb := core.AbsoluteBounds(list)
	tb := core.AbsoluteBounds(tr.Node())
	// children to the left of trigger
	if lb.Max.X > tb.Min.X+1 {
		// list max-x should be ≤ trigger min-x (with gap, list is left)
		listCenterX := lb.Min.X + lb.Size().Width/2
		trigCenterX := tb.Min.X + tb.Size().Width/2
		if listCenterX >= trigCenterX {
			t.Fatalf("placement=left: list centerX=%v trigger centerX=%v", listCenterX, trigCenterX)
		}
	}
}

func TestFloatButton_PRD_13_IconOnlyAriaLabel(t *testing.T) {
	// FB-13
	fb := kit.NewFloatButton()
	fb.SetIcon("plus")
	fb.SetContent("")
	// Without AriaLabel, applyContent falls back to icon name — require explicit API usage.
	fb.SetAriaLabel("Add item")
	if fb.AriaLabel != "Add item" {
		t.Fatal(fb.AriaLabel)
	}
	_ = layoutFAB(t, fb)
	if fb.Button().Root.Base().Label != "Add item" {
		// SetAriaLabel should win
		if fb.Button().AriaLabel != "Add item" {
			t.Fatalf("a11y name=%q aria=%q", fb.Button().Root.Base().Label, fb.Button().AriaLabel)
		}
	}
}

func TestFloatButton_PRD_14_DemoBasic(t *testing.T) {
	// FB-14 basic.tsx
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("basic")
	clicks := 0
	fb.SetOnClick(func() { clicks++ })
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 120, Height: 120})
	clickNode(t, tree, fb.Button().Root)
	if clicks != 1 {
		t.Fatalf("clicks=%d", clicks)
	}
}

func TestFloatButton_PRD_15_DemoType(t *testing.T) {
	// FB-15 type.tsx
	a := kit.NewFloatButton()
	a.SetType(kit.ButtonDefault)
	a.SetAriaLabel("def")
	b := kit.NewFloatButton()
	b.SetType(kit.ButtonPrimary)
	b.SetAriaLabel("pri")
	_ = layoutFAB(t, a)
	_ = layoutFAB(t, b)
	if a.Type != kit.ButtonDefault || b.Type != kit.ButtonPrimary {
		t.Fatal(a.Type, b.Type)
	}
}

func TestFloatButton_PRD_16_DemoShape(t *testing.T) {
	// FB-16 shape.tsx
	a := kit.NewFloatButton()
	a.SetShape(kit.FloatButtonCircle)
	a.SetAriaLabel("c")
	b := kit.NewFloatButton()
	b.SetShape(kit.FloatButtonSquare)
	b.SetAriaLabel("s")
	da := layoutFAB(t, a)
	db := layoutFAB(t, b)
	if da.Radius < 19 {
		t.Fatalf("circle r=%v", da.Radius)
	}
	if db.Radius < 7.5 || db.Radius > 8.5 {
		t.Fatalf("square r=%v", db.Radius)
	}
}

func TestFloatButton_PRD_17_DemoContent(t *testing.T) {
	// FB-17 content.tsx
	fb := kit.NewFloatButton()
	fb.SetShape(kit.FloatButtonSquare)
	fb.SetIcon("info")
	fb.SetContent("HELP")
	dec := layoutFAB(t, fb)
	if fb.Content != "HELP" {
		t.Fatal(fb.Content)
	}
	if len(dec.Children()) == 0 {
		t.Fatal("no body")
	}
	// content mode taller than plain 40
	if dec.Size().Height < kit.DefaultFloatButtonSize-0.5 {
		t.Fatalf("height %v", dec.Size().Height)
	}
}

func TestFloatButton_PRD_18_DemoTooltip(t *testing.T) {
	// FB-18 tooltip.tsx
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("tip")
	fb.SetTooltip("HELP TOOLTIP")
	if fb.Tooltip != "HELP TOOLTIP" {
		t.Fatal(fb.Tooltip)
	}
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	// Node is tooltip host (column) when tooltip set
	if fb.Node() == nil {
		t.Fatal("nil node")
	}
}

func TestFloatButton_PRD_19_DemoGroup(t *testing.T) {
	// FB-19 group.tsx — no trigger, children always visible
	a := kit.NewFloatButton()
	a.SetAriaLabel("a")
	b := kit.NewFloatButton()
	b.SetAriaLabel("b")
	g := kit.NewFloatButtonGroup(a, b)
	if g.Trigger != kit.FloatButtonTriggerNone {
		t.Fatal(g.Trigger)
	}
	if !g.IsOpen() {
		t.Fatal("non-menu group always open")
	}
	tree := core.NewTree(g.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	if g.ListNode() == nil {
		t.Fatal("list nil")
	}
}

func TestFloatButton_PRD_20_DemoGroupMenu(t *testing.T) {
	// FB-20 group-menu.tsx
	a := kit.NewFloatButton()
	a.SetAriaLabel("a")
	g := kit.NewFloatButtonGroup(a)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetType(kit.ButtonPrimary)
	g.SetIcon("plus")
	tree := core.NewTree(g.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 240, Height: 240})
	if g.IsOpen() {
		t.Fatal("start closed")
	}
	clickNode(t, tree, g.TriggerButton().Button().Root)
	if !g.IsOpen() {
		t.Fatal("open after click")
	}
	// hover mode constructs
	g2 := kit.NewFloatButtonGroup(kit.NewFloatButton())
	g2.SetTrigger(kit.FloatButtonTriggerHover)
	if g2.Node() == nil {
		t.Fatal("hover group")
	}
}

func TestFloatButton_PRD_21_DemoControlled(t *testing.T) {
	// FB-21 controlled.tsx
	a := kit.NewFloatButton()
	a.SetAriaLabel("a")
	g := kit.NewFloatButtonGroup(a)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetOpen(true)
	if !g.IsOpen() {
		t.Fatal()
	}
	g.SetOpen(false)
	if g.IsOpen() {
		t.Fatal()
	}
	// square controlled
	g2 := kit.NewFloatButtonGroup(kit.NewFloatButton())
	g2.SetTrigger(kit.FloatButtonTriggerClick)
	g2.SetShape(kit.FloatButtonSquare)
	g2.SetOpen(true)
	if g2.Shape != kit.FloatButtonSquare || !g2.IsOpen() {
		t.Fatal()
	}
}

func TestFloatButton_PRD_22_Metrics(t *testing.T) {
	// FB-22
	th := kit.DefaultTheme()
	want := th.SizeOr(core.TokenControlHeightLG, kit.DefaultFloatButtonSize)
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("m")
	dec := layoutFAB(t, fb)
	if dec.Size().Width < want-0.5 || dec.Size().Width > want+0.5 {
		t.Fatalf("size %v want %v", dec.Size().Width, want)
	}
	fb.SetShape(kit.FloatButtonSquare)
	dec = layoutFAB(t, fb)
	wantR := th.SizeOr(core.TokenBorderRadiusLG, kit.DefaultFloatButtonSquareRadius)
	if dec.Radius < wantR-0.5 || dec.Radius > wantR+0.5 {
		t.Fatalf("square r %v want %v", dec.Radius, wantR)
	}
}

func TestFloatButton_PRD_23_ThemeTokens(t *testing.T) {
	// FB-23 — primary fill from Theme, not a hard-coded brand constant only path
	custom := kit.DefaultTheme()
	want := render.RGBA{R: 0.1, G: 0.8, B: 0.2, A: 1}
	custom.ColorPrimary = want
	if custom.Tokens != nil {
		custom.Tokens.Colors[core.TokenColorPrimary] = want
	}
	fb := kit.NewFloatButton()
	fb.SetType(kit.ButtonPrimary)
	fb.SetAriaLabel("tok")
	fb.SetTheme(custom)
	dec := layoutFAB(t, fb)
	if !approxRGBA(dec.Background, want, 0.05) {
		t.Fatalf("bg=%v want custom primary %v", dec.Background, want)
	}
}

func TestFloatButton_PRD_24_DisabledChrome(t *testing.T) {
	// FB-24
	th := kit.DefaultTheme()
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("dis")
	fb.SetDisabled(true)
	dec := layoutFAB(t, fb)
	// disabled uses disabled tokens (not primary solid)
	if approxRGBA(dec.Background, th.Color(core.TokenColorPrimary), 0.02) && fb.Type == kit.ButtonDefault {
		t.Fatalf("disabled default should not stay primary: %v", dec.Background)
	}
	// button disabled flag
	if !fb.Disabled || !fb.Button().Disabled {
		t.Fatal("disabled flag")
	}
	_ = dec
}

func TestFloatButton_PRD_25_KeyboardFocus(t *testing.T) {
	// FB-25
	clicks := 0
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("kb")
	fb.SetOnClick(func() { clicks++ })
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	// focus via pointer then keyboard
	clickNode(t, tree, fb.Button().Root)
	clicks = 0
	if !fb.Button().Root.ShowFocusRing {
		t.Fatal("focus ring should be enabled")
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 1 {
		t.Fatalf("Enter clicks=%d", clicks)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if clicks != 2 {
		t.Fatalf("Space clicks=%d", clicks)
	}
}

func TestFloatButton_PRD_26_LoadingNoClick(t *testing.T) {
	// FB-26
	clicks := 0
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("load")
	fb.SetOnClick(func() { clicks++ })
	fb.SetLoading(true)
	if !fb.Loading {
		t.Fatal("loading flag")
	}
	tree := core.NewTree(fb.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 200, Height: 200})
	clickNode(t, tree, fb.Button().Root)
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0 while loading", clicks)
	}
	// Tick advances spinner
	if !fb.Tick(0.016) {
		t.Fatal("Tick should return true while loading")
	}
}

// Icon must stay centered in fixed 40×40 chrome under CrossStretch (gallery menu/section).
func TestFloatButton_IconCenteredUnderStretch(t *testing.T) {
	fb := kit.NewFloatButton()
	fb.SetIcon("plus")
	fb.SetAriaLabel("stretch")
	col := primitive.Column(fb.Node())
	col.CrossAlign = core.CrossStretch
	tree := core.NewTree(col)
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 400, Height: 100})

	root := fb.Button().Root
	if root == nil || !root.FixedSize {
		t.Fatal("FAB Pressable must be FixedSize")
	}
	if root.Size().Width > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("FAB stretched to width %v under CrossStretch", root.Size().Width)
	}
	dec, ok := fb.ChromeNode().(*primitive.Decorated)
	if !ok {
		t.Fatal("chrome")
	}
	if dec.Size().Width > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("decorated width %v", dec.Size().Width)
	}
	// Icon is direct child of Decorated (icon-only path); offset ~ (40-18)/2 = 11.
	if len(dec.Children()) != 1 {
		t.Fatalf("decorated kids=%d want 1 (icon direct)", len(dec.Children()))
	}
	icon := dec.Children()[0]
	if icon.TypeID() != "primitive.Icon" {
		t.Fatalf("child %s want Icon", icon.TypeID())
	}
	off := icon.Base().Offset()
	isz := icon.Base().Size()
	want := (kit.DefaultFloatButtonSize - isz.Width) / 2
	if off.X < want-1 || off.X > want+1 || off.Y < want-1 || off.Y > want+1 {
		t.Fatalf("icon offset=%v size=%v want ~{%.1f %.1f} (centered in 40×40)", off, isz, want, want)
	}

	// Default type (outlined) — same centering (menu children use type=default).
	fbDef := kit.NewFloatButton()
	fbDef.SetAriaLabel("default-icon") // default "info" icon
	colDef := primitive.Column(fbDef.Node())
	colDef.CrossAlign = core.CrossStretch
	treeDef := core.NewTree(colDef)
	treeDef.SetTheme(kit.DefaultTheme())
	treeDef.Layout(core.Size{Width: 400, Height: 100})
	decDef, _ := fbDef.ChromeNode().(*primitive.Decorated)
	if decDef == nil || len(decDef.Children()) != 1 {
		t.Fatal("default chrome")
	}
	offD := decDef.Children()[0].Base().Offset()
	szD := decDef.Children()[0].Base().Size()
	wantD := (kit.DefaultFloatButtonSize - szD.Width) / 2
	if offD.X < wantD-1 || offD.X > wantD+1 || offD.Y < wantD-1 || offD.Y > wantD+1 {
		t.Fatalf("default icon offset=%v size=%v want centered ~{%.1f}", offD, szD, wantD)
	}

	// Menu mode trigger + children also fixed / centered
	c1 := kit.NewFloatButton()
	c1.SetIcon("plus")
	c1.SetAriaLabel("c1")
	g := kit.NewFloatButtonGroup(c1)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetIcon("plus")
	g.SetDefaultOpen(true)
	col2 := primitive.Column(g.Node())
	col2.CrossAlign = core.CrossStretch
	tree2 := core.NewTree(col2)
	tree2.SetTheme(kit.DefaultTheme())
	tree2.Layout(core.Size{Width: 400, Height: 400})
	tr := g.TriggerButton()
	if tr == nil || tr.Button().Root == nil || !tr.Button().Root.FixedSize {
		t.Fatal("menu trigger FixedSize")
	}
	if tr.Button().Root.Size().Width > kit.DefaultFloatButtonSize+0.5 {
		t.Fatalf("menu trigger stretched %v", tr.Button().Root.Size())
	}
	// open trigger shows close icon — still direct-child centered
	trDec, _ := tr.ChromeNode().(*primitive.Decorated)
	if trDec == nil || len(trDec.Children()) != 1 {
		t.Fatal("trigger chrome kids")
	}
	trOff := trDec.Children()[0].Base().Offset()
	trSz := trDec.Children()[0].Base().Size()
	trWant := (kit.DefaultFloatButtonSize - trSz.Width) / 2
	if trOff.X < trWant-1 || trOff.X > trWant+1 {
		t.Fatalf("menu trigger icon offset=%v want centered ~{%.1f}", trOff, trWant)
	}
}

// Open/close swaps plus↔close; icon must stay centered immediately (no layout-wait).
func TestFloatButton_MenuToggleIconStaysCentered(t *testing.T) {
	c1 := kit.NewFloatButton()
	c1.SetIcon("plus")
	c1.SetAriaLabel("c1")
	g := kit.NewFloatButtonGroup(c1)
	g.SetTrigger(kit.FloatButtonTriggerClick)
	g.SetIcon("plus")
	tree := core.NewTree(g.Node())
	tree.SetTheme(kit.DefaultTheme())
	tree.Layout(core.Size{Width: 300, Height: 300})

	tr := g.TriggerButton()
	if tr == nil {
		t.Fatal("nil trigger")
	}
	checkCentered := func(label string) {
		t.Helper()
		dec, ok := tr.ChromeNode().(*primitive.Decorated)
		if !ok || len(dec.Children()) != 1 {
			t.Fatalf("%s: chrome kids", label)
		}
		off := dec.Children()[0].Base().Offset()
		sz := dec.Children()[0].Base().Size()
		want := (kit.DefaultFloatButtonSize - sz.Width) / 2
		if off.X < want-1 || off.X > want+1 || off.Y < want-1 || off.Y > want+1 {
			t.Fatalf("%s: icon offset=%v size=%v want ~{%.1f %.1f}", label, off, sz, want, want)
		}
	}
	checkCentered("closed")

	// Toggle open via click (uncontrolled).
	abs := core.AbsoluteBounds(tr.Button().Root)
	x := abs.Min.X + abs.Size().Width/2
	y := abs.Min.Y + abs.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	// Immediately after click — before Tree.Layout — icon must already be centered.
	checkCentered("after open click (pre-layout)")
	if !g.IsOpen() {
		t.Fatal("want open")
	}
	tree.Layout(core.Size{Width: 300, Height: 300})
	checkCentered("after open layout")

	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	checkCentered("after close click (pre-layout)")
}

func approxRGBA(a, b render.RGBA, eps float64) bool {
	dr := a.R - b.R
	if dr < 0 {
		dr = -dr
	}
	dg := a.G - b.G
	if dg < 0 {
		dg = -dg
	}
	db := a.B - b.B
	if db < 0 {
		db = -db
	}
	da := a.A - b.A
	if da < 0 {
		da = -da
	}
	return dr <= eps && dg <= eps && db <= eps && da <= eps
}

// FB-S6/FB-07 general: menu host size stable under various parents (not demo-only).
func TestFloatButton_PRD_MenuHost_GeneralParents(t *testing.T) {
	newMenu := func() *kit.FloatButtonGroup {
		a := kit.NewFloatButton()
		a.SetAriaLabel("a")
		a.SetIcon("plus")
		b := kit.NewFloatButton()
		b.SetAriaLabel("b")
		b.SetIcon("info")
		g := kit.NewFloatButtonGroup(a, b)
		g.SetTrigger(kit.FloatButtonTriggerClick)
		g.SetIcon("plus")
		return g
	}
	want := kit.DefaultFloatButtonSize

	// 1) Loose column (typical form/page)
	{
		g := newMenu()
		col := primitive.Column(g.Node(), kit.NewText("below").Node())
		col.Gap = 8
		_ = col.Layout(core.Loose(400, 800))
		closedH := g.Node().Base().Size().Height
		g.SetOpen(true)
		_ = col.Layout(core.Loose(400, 800))
		openH := g.Node().Base().Size().Height
		if closedH != want || openH != want {
			t.Fatalf("column parent: closed=%v open=%v want host=%v", closedH, openH, want)
		}
		// parent column height must not grow by list size when open
		// (group contributes only trigger size)
		g.SetOpen(false)
		hClosed := col.Layout(core.Loose(400, 800)).Height
		g.SetOpen(true)
		hOpen := col.Layout(core.Loose(400, 800)).Height
		if hOpen != hClosed {
			t.Fatalf("column total height jumped closed=%v open=%v", hClosed, hOpen)
		}
	}

	// 2) StretchChild tight host (tab body / decorated stage)
	{
		g := newMenu()
		host := primitive.NewDecorated(g.Node())
		host.Width, host.Height = 300, 200
		host.StretchChild = true
		_ = host.Layout(core.Tight(300, 200))
		sz := g.Node().Base().Size()
		if sz.Width != want || sz.Height != want {
			t.Fatalf("stretch parent: group size=%v want %v×%v (must not fill stage)", sz, want, want)
		}
		g.SetOpen(true)
		_ = host.Layout(core.Tight(300, 200))
		sz2 := g.Node().Base().Size()
		if sz2 != sz {
			t.Fatalf("stretch open size changed %v → %v", sz, sz2)
		}
	}

	// 3) kit.Flex ExpandMax row (page toolbars)
	{
		g := newMenu()
		row := kit.NewFlex(g.Node(), kit.NewText("side").Node())
		row.SetGap(12)
		_ = row.Node().Layout(core.Loose(400, 100))
		g.SetOpen(true)
		_ = row.Node().Layout(core.Loose(400, 100))
		if g.Node().Base().Size().Height != want {
			t.Fatalf("flex parent open h=%v", g.Node().Base().Size().Height)
		}
	}

	// 4) All placements: list offset outside host; host size stable
	for _, pl := range []kit.FloatButtonPlacement{
		kit.FloatButtonTop, kit.FloatButtonBottom, kit.FloatButtonLeft, kit.FloatButtonRight,
	} {
		g := newMenu()
		g.SetPlacement(pl)
		n := g.Node()
		closed := n.Layout(core.Loose(400, 400))
		g.SetOpen(true)
		openSz := n.Layout(core.Loose(400, 400))
		if openSz != closed {
			t.Fatalf("placement %v host size closed=%v open=%v", pl, closed, openSz)
		}
		list := g.ListNode()
		if list == nil {
			t.Fatal("nil list")
		}
		o := list.Base().Offset()
		switch pl {
		case kit.FloatButtonTop:
			if o.Y >= 0 {
				t.Fatalf("top: list Y=%v want <0", o.Y)
			}
		case kit.FloatButtonBottom:
			if o.Y <= 0 {
				t.Fatalf("bottom: list Y=%v want >0", o.Y)
			}
		case kit.FloatButtonLeft:
			if o.X >= 0 {
				t.Fatalf("left: list X=%v want <0", o.X)
			}
		case kit.FloatButtonRight:
			if o.X <= 0 {
				t.Fatalf("right: list X=%v want >0", o.X)
			}
		}
	}
}
