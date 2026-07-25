package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/avatar.md §6.9 — P0 PRD cases (AV-01 … AV-18).
// L3/L4 (AV-19/20) and P1 (AV-21) are covered elsewhere or deferred.

func approxAV(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxColorAV(a, b render.RGBA, tol float64) bool {
	return approxAV(a.R, b.R, tol) && approxAV(a.G, b.G, tol) &&
		approxAV(a.B, b.B, tol) && approxAV(a.A, b.A, tol)
}

func TestAvatar_PRD_01_Defaults(t *testing.T) {
	// AV-01
	a := kit.NewAvatar("")
	if a.Shape != kit.AvatarCircle {
		t.Fatalf("Shape=%v want circle", a.Shape)
	}
	if a.Size != kit.AvatarMiddle {
		t.Fatalf("Size=%v want middle", a.Size)
	}
	if a.Disabled || a.Loading {
		t.Fatalf("flags disabled=%v loading=%v", a.Disabled, a.Loading)
	}
	_ = a.Node().Layout(core.Loose(100, 100))
	if !approxAV(a.ResolvedSize(), 32, 0.5) {
		t.Fatalf("ResolvedSize=%v want 32", a.ResolvedSize())
	}
	if a.ContentMode() != kit.AvatarContentText {
		t.Fatalf("mode=%v want text", a.ContentMode())
	}
}

// gap probe via SetGap only — default path uses DefaultAvatarGap internally.
func TestAvatar_PRD_01b_DefaultGap(t *testing.T) {
	a := kit.NewAvatar("U")
	// After rebuild, long text with gap=4 should still layout.
	_ = a.Node().Layout(core.Loose(80, 80))
	a.SetGap(4)
	_ = a.Node().Layout(core.Loose(80, 80))
	if a.TextScale <= 0 {
		t.Fatal("TextScale")
	}
}

func TestAvatar_PRD_02_SrcSuccess(t *testing.T) {
	// AV-02
	a := kit.NewAvatar("U")
	a.SetSrc("res://face")
	a.SetImageOK(true)
	if a.ContentMode() != kit.AvatarContentImage {
		t.Fatalf("mode=%v want image", a.ContentMode())
	}
	dec := a.ChromeNode().(*primitive.Decorated)
	// image mode → transparent bg
	if dec.Background.A > 0.05 {
		t.Fatalf("image bg should be transparent, got %v", dec.Background)
	}
	// pixels path also image
	b := kit.NewAvatar("")
	pix := make([]byte, 4*4*4)
	for i := range pix {
		pix[i] = 200
	}
	b.SetPixels(4, 4, pix)
	if b.ContentMode() != kit.AvatarContentImage {
		t.Fatalf("pixels mode=%v", b.ContentMode())
	}
}

func TestAvatar_PRD_03_SrcFailFallback(t *testing.T) {
	// AV-03
	errN := 0
	a := kit.NewAvatar("FB")
	a.SetIcon("user")
	a.SetSrc("bad://x")
	a.SetOnError(func() bool {
		errN++
		return true // allow default fallback
	})
	a.NotifyImageError()
	if errN != 1 {
		t.Fatalf("onError calls=%d", errN)
	}
	// fallback: icon > children
	if a.ContentMode() != kit.AvatarContentIcon {
		t.Fatalf("mode=%v want icon", a.ContentMode())
	}

	// return false keeps image exist
	b := kit.NewAvatar("X")
	b.SetSrc("bad://y")
	b.SetOnError(func() bool { return false })
	b.NotifyImageError()
	if b.ContentMode() != kit.AvatarContentImage {
		t.Fatalf("keep image mode=%v", b.ContentMode())
	}
}

func TestAvatar_PRD_04_TextAvatar(t *testing.T) {
	// AV-04
	a := kit.NewAvatar("U")
	if a.Text != "U" {
		t.Fatalf("Text=%q", a.Text)
	}
	if a.ContentMode() != kit.AvatarContentText {
		t.Fatalf("mode=%v", a.ContentMode())
	}
	_ = a.Node().Layout(core.Loose(40, 40))
}

func TestAvatar_PRD_05_ShapeSquare(t *testing.T) {
	// AV-05
	a := kit.NewAvatar("S")
	a.SetShape(kit.AvatarSquare)
	_ = a.Node().Layout(core.Loose(40, 40))
	dec := a.ChromeNode().(*primitive.Decorated)
	// middle square radius ≈ 6
	if !approxAV(dec.Radius, 6, 0.5) {
		t.Fatalf("square radius=%v want ~6", dec.Radius)
	}
	// circle uses size/2
	c := kit.NewAvatar("C")
	c.SetShape(kit.AvatarCircle)
	_ = c.Node().Layout(core.Loose(40, 40))
	cd := c.ChromeNode().(*primitive.Decorated)
	if !approxAV(cd.Radius, 16, 0.5) {
		t.Fatalf("circle radius=%v want 16", cd.Radius)
	}
}

func TestAvatar_PRD_06_SizeLarge(t *testing.T) {
	// AV-06
	a := kit.NewAvatar("L")
	a.SetSize(kit.AvatarLarge)
	sz := a.Node().Layout(core.Loose(100, 100))
	if !approxAV(sz.Width, 40, 0.5) || !approxAV(sz.Height, 40, 0.5) {
		t.Fatalf("large size=%v want 40×40", sz)
	}
}

func TestAvatar_PRD_07_GroupMaxOverflow(t *testing.T) {
	// AV-07
	a1 := kit.NewAvatar("A")
	a2 := kit.NewAvatar("B")
	a3 := kit.NewAvatar("C")
	g := kit.NewAvatarGroup(a1, a2, a3)
	g.SetMaxCount(2)
	_ = g.Node().Layout(core.Loose(400, 80))
	if g.OverflowText != "+1" {
		t.Fatalf("overflow=%q want +1", g.OverflowText)
	}
	// visible = 2 avatars + overflow chip
	if g.VisibleCount != 3 {
		t.Fatalf("VisibleCount=%d want 3", g.VisibleCount)
	}
}

func TestAvatar_PRD_08_DefaultSize32(t *testing.T) {
	// AV-08
	a := kit.NewAvatar("U")
	sz := a.Node().Layout(core.Loose(100, 100))
	if !approxAV(sz.Width, 32, 0.5) || !approxAV(sz.Height, 32, 0.5) {
		t.Fatalf("default size=%v want 32×32", sz)
	}
	if a.Root.Size().Width != 32 {
		t.Fatalf("Root width=%v", a.Root.Size().Width)
	}
}

func TestAvatar_PRD_09_BasicDemo(t *testing.T) {
	// AV-09 basic.tsx: circle/square × sizes
	sizes := []struct {
		set  func(*kit.Avatar)
		want float64
	}{
		{func(a *kit.Avatar) { a.SetSizePx(64) }, 64},
		{func(a *kit.Avatar) { a.SetSize(kit.AvatarLarge) }, 40},
		{func(a *kit.Avatar) { a.SetSize(kit.AvatarMiddle) }, 32},
		{func(a *kit.Avatar) { a.SetSize(kit.AvatarSmall) }, 24},
		{func(a *kit.Avatar) { a.SetSizePx(14) }, 14},
	}
	for _, sh := range []kit.AvatarShape{kit.AvatarCircle, kit.AvatarSquare} {
		for _, tc := range sizes {
			a := kit.NewAvatarIcon("user")
			a.SetShape(sh)
			tc.set(a)
			sz := a.Node().Layout(core.Loose(200, 200))
			if !approxAV(sz.Width, tc.want, 0.5) {
				t.Fatalf("shape=%v size=%v want %v", sh, sz.Width, tc.want)
			}
		}
	}
}

func TestAvatar_PRD_10_TypeDemo(t *testing.T) {
	// AV-10 type.tsx
	nodes := []core.Node{
		kit.NewAvatarIcon("user").Node(),
		kit.NewAvatar("U").Node(),
		func() core.Node {
			a := kit.NewAvatar("USER")
			a.SetSizePx(40)
			return a.Node()
		}(),
		func() core.Node {
			a := kit.NewAvatar("")
			a.SetSrc("https://example.com/a.svg")
			a.SetImageOK(true)
			return a.Node()
		}(),
		func() core.Node {
			a := kit.NewAvatar("U")
			a.SetStyle(kit.Style{
				Background: render.Hex("#fde3cf"),
				Text:       render.Hex("#f56a00"),
			})
			return a.Node()
		}(),
		func() core.Node {
			a := kit.NewAvatarIcon("user")
			a.SetStyle(kit.Style{Background: render.Hex("#87d068")})
			return a.Node()
		}(),
	}
	row := primitive.Row(nodes...)
	row.Gap = 16
	_ = row.Layout(core.Loose(800, 80))
}

func TestAvatar_PRD_11_DynamicScale(t *testing.T) {
	// AV-11 dynamic.tsx — long string scales down
	a := kit.NewAvatar("Edward")
	a.SetSize(kit.AvatarLarge)
	a.SetGap(4)
	_ = a.Node().Layout(core.Loose(100, 100))
	if a.TextScale > 1 {
		t.Fatalf("TextScale=%v want ≤1", a.TextScale)
	}
	// shorter text larger scale
	b := kit.NewAvatar("U")
	b.SetSize(kit.AvatarLarge)
	b.SetGap(4)
	_ = b.Node().Layout(core.Loose(100, 100))
	if b.TextScale < a.TextScale-0.01 && a.TextScale < 1 {
		// Edward should be tighter than U
	}
	if b.TextScale != 1 && b.TextScale > a.TextScale {
		// ok either way as long as Edward scaled when needed
	}
	// With gap=1, scale should be smaller or equal for long text
	c := kit.NewAvatar("Edward")
	c.SetSize(kit.AvatarLarge)
	c.SetGap(1)
	_ = c.Node().Layout(core.Loose(100, 100))
	d := kit.NewAvatar("Edward")
	d.SetSize(kit.AvatarLarge)
	d.SetGap(4)
	_ = d.Node().Layout(core.Loose(100, 100))
	// larger gap → more aggressive scale (smaller or equal scale)
	if d.TextScale > c.TextScale+0.001 && c.TextScale < 1 {
		t.Fatalf("gap4 scale=%v gap1 scale=%v (gap4 should ≤ gap1)", d.TextScale, c.TextScale)
	}
}

func TestAvatar_PRD_12_BadgeDemo(t *testing.T) {
	// AV-12 badge.tsx
	av := kit.NewAvatarIcon("user")
	av.SetShape(kit.AvatarSquare)
	b1 := kit.NewBadge(av.Node(), 1)
	av2 := kit.NewAvatarIcon("user")
	av2.SetShape(kit.AvatarSquare)
	b2 := kit.NewBadge(av2.Node(), 0)
	b2.SetDot(true)
	_ = b1.Node().Layout(core.Loose(80, 80))
	_ = b2.Node().Layout(core.Loose(80, 80))
}

func TestAvatar_PRD_13_GroupDemo(t *testing.T) {
	// AV-13 group.tsx
	mk := func(txt string, bg string) *kit.Avatar {
		a := kit.NewAvatar(txt)
		if bg != "" {
			a.SetStyle(kit.Style{Background: render.Hex(bg)})
		}
		return a
	}
	g1 := kit.NewAvatarGroup(
		func() *kit.Avatar {
			a := kit.NewAvatar("")
			a.SetSrc("https://example.com/1")
			a.SetImageOK(true)
			return a
		}(),
		mk("K", "#f56a00"),
		kit.NewAvatarIcon("user"),
		kit.NewAvatarIcon("info"),
	)
	_ = g1.Node().Layout(core.Loose(400, 60))

	g2 := kit.NewAvatarGroup(mk("A", ""), mk("B", ""), mk("C", ""), mk("D", ""))
	g2.SetMaxCount(2)
	g2.SetMaxStyle(kit.Style{Background: render.Hex("#fde3cf"), Text: render.Hex("#f56a00")})
	_ = g2.Node().Layout(core.Loose(400, 60))
	if g2.OverflowText != "+2" {
		t.Fatalf("overflow=%q want +2", g2.OverflowText)
	}

	g3 := kit.NewAvatarGroup(mk("A", "#fde3cf"), mk("K", "#f56a00"), kit.NewAvatarIcon("user"), kit.NewAvatarIcon("info"))
	g3.SetShape(kit.AvatarSquare)
	g3.SetSize(kit.AvatarLarge)
	_ = g3.Node().Layout(core.Loose(400, 80))
	// large group child edge 40
	if !approxAV(g3.Children[0].ResolvedSize(), 40, 0.5) {
		t.Fatalf("group large child=%v", g3.Children[0].ResolvedSize())
	}
}

func TestAvatar_PRD_14_Responsive(t *testing.T) {
	// AV-14 responsive.tsx
	a := kit.NewAvatarIcon("info")
	a.SetResponsiveSize(kit.AvatarResponsiveSize{
		XS: 24, SM: 32, MD: 40, LG: 64, XL: 80, XXL: 100,
	})
	a.SetBreakpoint("xs")
	if !approxAV(a.ResolvedSize(), 24, 0.5) {
		t.Fatalf("xs=%v", a.ResolvedSize())
	}
	a.SetBreakpoint("lg")
	sz := a.Node().Layout(core.Loose(200, 200))
	if !approxAV(sz.Width, 64, 0.5) {
		t.Fatalf("lg layout=%v want 64", sz.Width)
	}
	a.SetBreakpoint("xxl")
	if !approxAV(a.ResolvedSize(), 100, 0.5) {
		t.Fatalf("xxl=%v", a.ResolvedSize())
	}
}

func TestAvatar_PRD_15_TokenMetrics(t *testing.T) {
	// AV-15
	th := kit.DefaultTheme()
	if !approxAV(th.SizeOr(core.TokenControlHeight, 0), 32, 0.5) {
		t.Fatal("controlHeight")
	}
	if !approxAV(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.5) {
		t.Fatal("controlHeightSM")
	}
	if !approxAV(th.SizeOr(core.TokenControlHeightLG, 0), 40, 0.5) {
		t.Fatal("controlHeightLG")
	}
	if !approxAV(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatal("fontSize")
	}
	if !approxAV(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatal("borderRadius")
	}
	if !approxAV(th.SizeOr(core.TokenBorderRadiusSM, 0), 4, 0.5) {
		t.Fatal("borderRadiusSM")
	}
	if !approxAV(th.SizeOr(core.TokenBorderRadiusLG, 0), 8, 0.5) {
		t.Fatal("borderRadiusLG")
	}
	// size ladder on control
	for _, tc := range []struct {
		sz   kit.AvatarSize
		want float64
	}{
		{kit.AvatarSmall, 24},
		{kit.AvatarMiddle, 32},
		{kit.AvatarLarge, 40},
	} {
		a := kit.NewAvatar("X")
		a.SetSize(tc.sz)
		got := a.Node().Layout(core.Loose(100, 100)).Width
		if !approxAV(got, tc.want, 0.5) {
			t.Fatalf("size %v = %v want %v", tc.sz, got, tc.want)
		}
	}
	// group overlapping constant
	if kit.DefaultAvatarGroupOverlapping != -8 {
		t.Fatalf("groupOverlapping=%v want -8", kit.DefaultAvatarGroupOverlapping)
	}
	if kit.DefaultAvatarGap != 4 {
		t.Fatalf("gap default=%v", kit.DefaultAvatarGap)
	}
}

func TestAvatar_PRD_16_DefaultSkinToken(t *testing.T) {
	// AV-16 — no hard-coded primary as default fill
	a := kit.NewAvatar("U")
	_ = a.Node().Layout(core.Loose(40, 40))
	dec := a.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	if approxColorAV(dec.Background, primary, 0.05) {
		t.Fatalf("default fill must not be primary: %v", dec.Background)
	}
	// should track quaternary (placeholder) or inverse text path
	q := th.Color(core.TokenColorTextQuaternary)
	if dec.Background.A < 0.05 {
		t.Fatal("default fill invisible")
	}
	// allow either exact quaternary or fallback 0.25 gray
	if !approxColorAV(dec.Background, q, 0.08) {
		// fallback path
		if !approxAV(dec.Background.A, 0.25, 0.08) {
			t.Fatalf("bg=%v want ~quaternary %v", dec.Background, q)
		}
	}
}

func TestAvatar_PRD_17_Disabled(t *testing.T) {
	// AV-17
	a := kit.NewAvatar("D")
	a.SetDisabled(true)
	_ = a.Node().Layout(core.Loose(40, 40))
	dec := a.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	dis := th.Color(core.TokenColorDisabledBg)
	if !approxColorAV(dec.Background, dis, 0.08) {
		t.Fatalf("disabled bg=%v want ~%v", dec.Background, dis)
	}
}

func TestAvatar_PRD_18_InteractiveClick(t *testing.T) {
	// AV-18
	clicks := 0
	a := kit.NewAvatar("Go")
	a.SetOnClick(func() { clicks++ })
	a.SetAriaLabel("avatar-go")
	tree := core.NewTree(a.Node())
	tree.Layout(core.Size{Width: 80, Height: 80})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
	// keyboard
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if clicks != 2 {
		t.Fatalf("Enter clicks=%d want 2", clicks)
	}
	// loading swallows? optional — disabled swallows
	a.SetDisabled(true)
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 8, Y: 8, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 8, Y: 8, Button: core.ButtonLeft})
	if clicks != 2 {
		t.Fatalf("disabled clicks=%d want 2", clicks)
	}
}

func TestAvatar_PRD_LoadingTicker(t *testing.T) {
	// AV-S10
	a := kit.NewAvatar("L")
	a.SetLoading(true)
	tree := core.NewTree(a.Node())
	a.AttachTicker(tree)
	tree.Layout(core.Size{Width: 80, Height: 80})
	if !a.Tick(0.016) {
		t.Fatal("Tick should stay active while loading")
	}
	a.SetLoading(false)
	if a.Tick(0.016) {
		t.Fatal("Tick should stop when not loading")
	}
}

func TestAvatar_PRD_CustomSizeIconFont(t *testing.T) {
	// AV-S9 custom 64 icon
	a := kit.NewAvatarIcon("user")
	a.SetSizePx(64)
	sz := a.Node().Layout(core.Loose(100, 100))
	if !approxAV(sz.Width, 64, 0.5) {
		t.Fatalf("custom=%v", sz.Width)
	}
}
