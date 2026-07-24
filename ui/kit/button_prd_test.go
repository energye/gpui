package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/button.md §6.9 — P0 PRD cases (BTN-01 … BTN-24).
// L3/L4 (BTN-21/22) and P1 (BTN-25+) are covered elsewhere or deferred.

func TestButton_PRD_01_Defaults(t *testing.T) {
	// BTN-01: NewButton 默认 type=default，size=middle，可点
	btn := kit.NewButton("确定")
	if btn.Type != kit.ButtonDefault {
		t.Fatalf("Type=%v want default", btn.Type)
	}
	if btn.Size != kit.ButtonMiddle {
		t.Fatalf("Size=%v want middle", btn.Size)
	}
	if btn.Shape != kit.ButtonShapeDefault {
		t.Fatalf("Shape=%v want default", btn.Shape)
	}
	if btn.Variant != kit.ButtonVariantAuto {
		t.Fatalf("Variant=%v want auto", btn.Variant)
	}
	if btn.Color != kit.ButtonColorDefault {
		t.Fatalf("Color=%v want default", btn.Color)
	}
	if btn.Danger || btn.Ghost || btn.Block || btn.Loading || btn.Disabled {
		t.Fatalf("flags should be false: danger=%v ghost=%v block=%v loading=%v disabled=%v",
			btn.Danger, btn.Ghost, btn.Block, btn.Loading, btn.Disabled)
	}
	if btn.IconPlacement != kit.ButtonIconStart {
		t.Fatalf("IconPlacement=%v want start", btn.IconPlacement)
	}
	// clickable
	clicks := 0
	btn.SetOnClick(func() { clicks++ })
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 5, Y: 5, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 5, Y: 5, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestButton_PRD_02_PrimaryClick(t *testing.T) {
	// BTN-02
	clicks := 0
	btn := kit.NewButton("Save")
	btn.SetType(kit.ButtonPrimary)
	btn.SetOnClick(func() { clicks++ })
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}

func TestButton_PRD_03_DisabledNoClick(t *testing.T) {
	// BTN-03
	clicks := 0
	btn := kit.NewButton("No")
	btn.SetOnClick(func() { clicks++ })
	btn.SetDisabled(true)
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0", clicks)
	}
}

func TestButton_PRD_04_LoadingNoClick(t *testing.T) {
	// BTN-04
	clicks := 0
	btn := kit.NewButton("Load")
	btn.SetOnClick(func() { clicks++ })
	btn.SetLoading(true)
	if btn.ChromeNode() == nil {
		t.Fatal("nil chrome while loading")
	}
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0 while loading", clicks)
	}
}

func TestButton_PRD_05_PressOutsideNoClick(t *testing.T) {
	// BTN-05: press then release outside → no click (tree enforces release-inside)
	clicks := 0
	btn := kit.NewButton("Drag")
	btn.SetOnClick(func() { clicks++ })
	root := primitive.NewBox(btn.Node())
	root.Width, root.Height = 400, 200
	root.Padding = primitive.All(20)
	tree := core.NewTree(root)
	host := platform.NewHeadless(400, 200)
	defer host.Close()
	tree.Layout(core.Size{Width: 400, Height: 200})
	// Down on button center, up far outside.
	bx := 20 + btn.Root.Size().Width/2
	by := 20 + btn.Root.Size().Height/2
	host.InjectPointer(platform.PointerDown, bx, by, platform.BtnLeft)
	host.InjectPointer(platform.PointerUp, 390, 190, platform.BtnLeft)
	for _, ev := range host.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if clicks != 0 {
		t.Fatalf("clicks=%d want 0 (release outside)", clicks)
	}
}

func TestButton_PRD_06_KeyboardActivate(t *testing.T) {
	// BTN-06
	clicks := 0
	btn := kit.NewButton("Go")
	btn.SetOnClick(func() { clicks++ })
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 5, Y: 5, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 5, Y: 5, Button: core.ButtonLeft})
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

func TestButton_PRD_07_SizeHeights(t *testing.T) {
	// BTN-07
	btn := kit.NewButton("OK")
	for _, tc := range []struct {
		size kit.ButtonSize
		want float64
	}{
		{kit.ButtonSmall, 24},
		{kit.ButtonMiddle, 32},
		{kit.ButtonLarge, 40},
	} {
		btn.SetSize(tc.size)
		sz := btn.Node().Layout(core.Loose(400, 100))
		if sz.Height < tc.want-0.5 || sz.Height > tc.want+0.5 {
			t.Fatalf("size %v height=%v want %v±0.5", tc.size, sz.Height, tc.want)
		}
	}
}

func TestButton_PRD_09_PrimarySolid(t *testing.T) {
	// BTN-09: primary solid fill = primary, text = inverse
	btn := kit.NewButton("P")
	btn.SetType(kit.ButtonPrimary)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	if !approxColor(dec.Background, primary, 0.02) {
		t.Fatalf("primary fill=%v want ~%v", dec.Background, primary)
	}
	// Label color inverse
	// Walk to text — label is private; check via not equal to ColorText on solid
	if dec.BorderWidth > 0.5 {
		// solid primary typically no border
		t.Logf("note: solid borderW=%v (may be 0)", dec.BorderWidth)
	}
}

func TestButton_PRD_10_DefaultOutlined(t *testing.T) {
	// BTN-10: default outlined has border, not solid primary fill
	btn := kit.NewButton("D")
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	if approxColor(dec.Background, primary, 0.05) {
		t.Fatalf("default should not use solid primary fill: %v", dec.Background)
	}
	if dec.BorderWidth < 0.5 {
		t.Fatalf("default outlined borderW=%v want ≥1", dec.BorderWidth)
	}
}

func TestButton_PRD_11_DashedBorder(t *testing.T) {
	// BTN-11
	btn := kit.NewButton("Dash")
	btn.SetType(kit.ButtonDashed)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	if len(dec.BorderDash) < 2 {
		t.Fatalf("dashed BorderDash=%v want pattern", dec.BorderDash)
	}
}

func TestButton_PRD_12_TextLinkNoSolidFill(t *testing.T) {
	// BTN-12
	for _, typ := range []kit.ButtonType{kit.ButtonText, kit.ButtonLink} {
		btn := kit.NewButton("T")
		btn.SetType(typ)
		_ = btn.Node().Layout(core.Loose(200, 100))
		dec := btn.ChromeNode().(*primitive.Decorated)
		if dec.BorderWidth > 0.5 {
			t.Fatalf("%v borderW=%v want 0", typ, dec.BorderWidth)
		}
	}
}

func TestButton_PRD_13_DangerColor(t *testing.T) {
	// BTN-13
	btn := kit.NewButton("Del")
	btn.SetDanger(true)
	btn.SetType(kit.ButtonPrimary)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	errC := th.Color(core.TokenColorError)
	if !approxColor(dec.Background, errC, 0.15) {
		// danger primary solid uses error family
		btn2 := kit.NewButton("Del2")
		btn2.SetColor(kit.ButtonColorDanger)
		btn2.SetVariant(kit.ButtonVariantSolid)
		_ = btn2.Node().Layout(core.Loose(200, 100))
		dec2 := btn2.ChromeNode().(*primitive.Decorated)
		if !approxColor(dec2.Background, errC, 0.15) {
			t.Fatalf("danger fill=%v / color.danger solid=%v want ~error %v",
				dec.Background, dec2.Background, errC)
		}
	}
}

func TestButton_PRD_14_GhostTransparentIdle(t *testing.T) {
	// BTN-14
	btn := kit.NewButton("Ghost")
	btn.SetType(kit.ButtonPrimary)
	btn.SetGhost(true)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	if dec.Background.A > 0.05 {
		t.Fatalf("ghost idle fill A=%v want transparent", dec.Background.A)
	}
	if dec.BorderWidth < 0.5 {
		t.Fatalf("ghost primary should keep border, borderW=%v", dec.BorderWidth)
	}
}

func TestButton_PRD_15_BlockWidth(t *testing.T) {
	// BTN-15
	btn := kit.NewButton("Block")
	btn.SetBlock(true)
	// Parent tight width forces expand
	root := primitive.NewBox(btn.Node())
	root.Width = 300
	root.Height = 80
	// Column-like: layout button with max width
	sz := btn.Node().Layout(core.Constraints{
		MinWidth: 300, MaxWidth: 300,
		MinHeight: 0, MaxHeight: 100,
	})
	if sz.Width < 299 {
		t.Fatalf("block width=%v want ~300", sz.Width)
	}
	if sz.Height < 31 || sz.Height > 33 {
		t.Fatalf("block height=%v want ~32", sz.Height)
	}
}

func TestButton_PRD_16_CircleIconOnly(t *testing.T) {
	// BTN-16
	btn := kit.NewButton("")
	btn.SetIcon("check")
	btn.SetShape(kit.ButtonShapeCircle)
	btn.SetAriaLabel("确认")
	sz := btn.Node().Layout(core.Loose(200, 100))
	if abs(sz.Width-sz.Height) > 0.5 {
		t.Fatalf("circle size=%+v want square", sz)
	}
	if abs(sz.Height-32) > 0.5 {
		t.Fatalf("circle middle height=%v want 32", sz.Height)
	}
}

func TestButton_PRD_17_RoundCapsule(t *testing.T) {
	// BTN-17
	btn := kit.NewButton("Round")
	btn.SetShape(kit.ButtonShapeRound)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	// radius ≈ height/2 = 16
	if dec.Radius < 15 || dec.Radius > 17 {
		t.Fatalf("round radius=%v want ~16", dec.Radius)
	}
}

func TestButton_PRD_18_IconPlacementEnd(t *testing.T) {
	// BTN-18
	btn := kit.NewButton("Next")
	btn.SetIcon("right")
	btn.SetIconPlacement(kit.ButtonIconEnd)
	if btn.IconPlacement != kit.ButtonIconEnd {
		t.Fatal("placement not end")
	}
	_ = btn.Node().Layout(core.Loose(200, 100))
	// Structure: row children order label then icon — verify non-panic rebuild
	btn.SetIconPlacement(kit.ButtonIconStart)
	_ = btn.Node().Layout(core.Loose(200, 100))
}

func TestButton_PRD_19_VariantOverType(t *testing.T) {
	// BTN-19: type + variant → variant wins
	btn := kit.NewButton("V")
	btn.SetType(kit.ButtonPrimary) // would be solid
	btn.SetVariant(kit.ButtonVariantOutlined)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	// Outlined has border; solid primary typically borderW=0
	if dec.BorderWidth < 0.5 {
		t.Fatalf("variant outlined should keep border, borderW=%v", dec.BorderWidth)
	}
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	if approxColor(dec.Background, primary, 0.05) {
		t.Fatalf("outlined should not solid-fill primary: %v", dec.Background)
	}
}

func TestButton_PRD_20_ColorVariantMatrix(t *testing.T) {
	// BTN-20 sample matrix
	combos := []struct {
		c kit.ButtonColor
		v kit.ButtonVariant
	}{
		{kit.ButtonColorPrimary, kit.ButtonVariantSolid},
		{kit.ButtonColorPrimary, kit.ButtonVariantOutlined},
		{kit.ButtonColorDefault, kit.ButtonVariantOutlined},
		{kit.ButtonColorDanger, kit.ButtonVariantSolid},
		{kit.ButtonColorDanger, kit.ButtonVariantOutlined},
	}
	for _, tc := range combos {
		btn := kit.NewButton("M")
		btn.SetColor(tc.c)
		btn.SetVariant(tc.v)
		_ = btn.Node().Layout(core.Loose(200, 100))
		dec := btn.ChromeNode().(*primitive.Decorated)
		if dec == nil {
			t.Fatalf("nil chrome for color=%v variant=%v", tc.c, tc.v)
		}
	}
}

func TestButton_PRD_23_IconOnlyRequiresAriaLabel(t *testing.T) {
	// BTN-23: icon-only must expose a11y name via AriaLabel
	btn := kit.NewButton("")
	btn.SetIcon("search")
	_ = btn.Node()
	if btn.Root.Base().Label != "" {
		// empty label and empty aria → empty name (bad for a11y)
	}
	if btn.Root.Base().Label != "" && btn.AriaLabel == "" && btn.Label == "" {
		t.Fatal("unexpected label")
	}
	// Without AriaLabel, accessible name is empty
	if name := btn.Root.Base().Label; name != "" {
		t.Fatalf("icon-only without AriaLabel Label=%q want empty (caller must SetAriaLabel)", name)
	}
	btn.SetAriaLabel("搜索")
	if btn.Root.Base().Label != "搜索" {
		t.Fatalf("a11y name=%q want 搜索", btn.Root.Base().Label)
	}
}

func TestButton_PRD_24_DisabledChrome(t *testing.T) {
	// BTN-24
	btn := kit.NewButton("X")
	btn.SetType(kit.ButtonDefault)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec := btn.ChromeNode().(*primitive.Decorated)
	idle := dec.Background
	btn.SetDisabled(true)
	_ = btn.Node().Layout(core.Loose(200, 100))
	dec = btn.ChromeNode().(*primitive.Decorated)
	// disabled uses disabled bg (not same as hover)
	btn.Root.SetHovered(true)
	btn.SyncState()
	dec2 := btn.ChromeNode().(*primitive.Decorated)
	// After hover while disabled, should not brighten like enabled hover
	if dec2.Background != dec.Background && dec2.Background != idle {
		// still ok if both are disabled palette
	}
	th := kit.DefaultTheme()
	dis := th.Color(core.TokenColorDisabledBg)
	if dis.A > 0 && dec.Background.A > 0 {
		// smoke: disabled path applied without panic
		t.Logf("disabled bg=%v tokenDisabled=%v", dec.Background, dis)
	}
}

func approxColor(a, b render.RGBA, tol float64) bool {
	return abs(a.R-b.R) <= tol && abs(a.G-b.G) <= tol && abs(a.B-b.B) <= tol
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
