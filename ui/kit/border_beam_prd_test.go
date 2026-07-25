package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/border-beam.md §6.9 — P0 L1/L2 (BB-01…15).
// BB-16/17 N/A; BB-18 L3 optional; BB-19 L4; BB-20 P1 — omitted.

func bbHasType(n core.Node, typ string) bool {
	if n == nil {
		return false
	}
	if n.TypeID() == typ {
		return true
	}
	for _, c := range n.Children() {
		if bbHasType(c, typ) {
			return true
		}
	}
	return false
}

func bbFindText(n core.Node, want string) bool {
	if n == nil {
		return false
	}
	if tx, ok := n.(*primitive.Text); ok && tx.Value == want {
		return true
	}
	for _, c := range n.Children() {
		if bbFindText(c, want) {
			return true
		}
	}
	return false
}

func TestBorderBeam_PRD_01_Defaults(t *testing.T) {
	// BB-01
	b := kit.NewBorderBeam(nil)
	if b == nil || b.Node() == nil {
		t.Fatal("nil BorderBeam")
	}
	if b.Node().TypeID() != "kit.BorderBeam" {
		t.Fatalf("type=%s", b.Node().TypeID())
	}
	if d := b.ResolvedDuration(); d < 5.5 || d > 6.5 {
		t.Fatalf("duration=%v want 6", d)
	}
	if s := b.ResolvedSize(); s < 99.5 || s > 100.5 {
		t.Fatalf("size=%v want 100", s)
	}
	if lw := b.ResolvedLineWidth(); lw < 0.5 || lw > 1.5 {
		t.Fatalf("lineWidth=%v want 1", lw)
	}
	if o := b.ResolvedOutset(); o != 0 {
		t.Fatalf("outset=%v want 0", o)
	}
	if b.Node().Base().Role != "presentation" {
		t.Fatalf("Role=%q want presentation", b.Node().Base().Role)
	}
	sz := b.Node().Layout(core.Loose(200, 120))
	if sz.Width < 0 || sz.Height < 0 {
		t.Fatalf("size=%v", sz)
	}
}

func TestBorderBeam_PRD_02_DefaultRunning(t *testing.T) {
	// BB-02 / BB-S1
	box := primitive.NewBox()
	box.Width, box.Height = 200, 100
	b := kit.NewBorderBeam(box)
	_ = b.Node().Layout(core.Loose(200, 100))
	if !b.IsBeamVisible() {
		t.Fatal("beam should be visible by default")
	}
	if !bbHasType(b.Node(), "kit.BorderBeamLayer") {
		t.Fatal("expected beam layer")
	}
	ph0 := b.Phase()
	if !b.Tick(0.1) {
		t.Fatal("should keep ticking while visible")
	}
	if b.Phase() <= ph0 {
		t.Fatalf("phase should advance: before=%v after=%v", ph0, b.Phase())
	}
}

func TestBorderBeam_PRD_03_ReducedMotionHides(t *testing.T) {
	// BB-03 / BB-S2 — antd hides beam under prefers-reduced-motion
	box := primitive.NewBox()
	box.Width, box.Height = 160, 80
	b := kit.NewBorderBeam(box)
	tree := core.NewTree(b.Node())
	tree.Clock().ReduceMotion = true
	tree.Layout(core.Size{Width: 200, Height: 100})
	if b.IsBeamVisible() {
		t.Fatal("reduced-motion must hide beam")
	}
	ph := b.Phase()
	if b.Tick(0.05) {
		t.Fatal("reduced-motion should stop ticker")
	}
	if b.Phase() != ph {
		t.Fatalf("phase must not advance: before=%v after=%v", ph, b.Phase())
	}
}

func TestBorderBeam_PRD_04_DurationSpeed(t *testing.T) {
	// BB-04 / BB-S3
	fast := kit.NewBorderBeam(nil)
	fast.SetDuration(3)
	slow := kit.NewBorderBeam(nil)
	slow.SetDuration(12)
	_ = fast.Tick(0.3)
	_ = slow.Tick(0.3)
	// Δphase ≈ dt/duration → fast ≈ 0.1, slow ≈ 0.025
	if fast.Phase() <= slow.Phase() {
		t.Fatalf("faster duration should advance more: fast=%v slow=%v", fast.Phase(), slow.Phase())
	}
	ratio := fast.Phase() / slow.Phase()
	// 12/3 = 4
	if ratio < 3.5 || ratio > 4.5 {
		t.Fatalf("phase ratio=%v want ~4", ratio)
	}
}

func TestBorderBeam_PRD_05_Color(t *testing.T) {
	// BB-05 / BB-S4
	b := kit.NewBorderBeam(nil)
	def := b.ResolvedColorStops()
	if len(def) < 1 {
		t.Fatal("default stops empty")
	}
	custom := render.RGBA{R: 1, G: 0.2, B: 0.3, A: 1}
	b.SetColor(custom)
	got := b.ResolvedColorStops()
	if len(got) != 1 {
		t.Fatalf("solid stops n=%d", len(got))
	}
	if got[0].Color != custom {
		t.Fatalf("color=%v want %v", got[0].Color, custom)
	}
	b.SetColorStops(
		kit.BorderBeamColorStop{Color: render.Hex("#1677ff"), Percent: 0},
		kit.BorderBeamColorStop{Color: render.Hex("#36cfc9"), Percent: 52},
		kit.BorderBeamColorStop{Color: render.Hex("#95de64"), Percent: 100},
	)
	if n := len(b.ResolvedColorStops()); n != 3 {
		t.Fatalf("gradient stops n=%d", n)
	}
	b.ClearColor()
	if !b.IsBeamVisible() {
		// still visible
	}
	cleared := b.ResolvedColorStops()
	if len(cleared) < 2 {
		t.Fatal("clear should restore default multi-stop gradient")
	}
}

func TestBorderBeam_PRD_06_Children(t *testing.T) {
	// BB-06 / BB-S5
	tx := primitive.NewText("workspace body")
	b := kit.NewBorderBeam(tx)
	_ = b.Node().Layout(core.Loose(240, 120))
	if b.Child() != tx {
		t.Fatal("Child() mismatch")
	}
	if !bbFindText(b.Node(), "workspace body") {
		t.Fatal("children text missing from tree")
	}
}

func TestBorderBeam_PRD_07_BasicDemo(t *testing.T) {
	// BB-07 basic.tsx
	card := kit.NewCard("Workspace overview")
	card.SetContent(kit.NewText("Review task status, deployment health, and recent automation activity in one panel.").Node())
	card.SetWidth(360)
	b := kit.NewBorderBeam(card.Node())
	sz := b.Node().Layout(core.Loose(400, 240))
	if sz.Width < 100 || sz.Height < 40 {
		t.Fatalf("basic size=%v", sz)
	}
	if !bbHasType(b.Node(), "kit.BorderBeamLayer") {
		t.Fatal("beam layer")
	}
	if !b.IsBeamVisible() {
		t.Fatal("basic beam visible")
	}
}

func TestBorderBeam_PRD_08_HoverDemo(t *testing.T) {
	// BB-08 hover.tsx / BB-S6
	card := kit.NewCard("Hover over the card")
	card.SetContent(kit.NewText("The border beam appears when the pointer moves over this card.").Node())
	b := kit.NewBorderBeam(card.Node())
	b.SetShowOnHover(true)
	_ = b.Node().Layout(core.Loose(360, 160))
	if b.IsBeamVisible() {
		t.Fatal("showOnHover without hover must hide beam")
	}
	b.SetHovered(true)
	if !b.IsBeamVisible() {
		t.Fatal("hovered must show beam")
	}
	ph0 := b.Phase()
	_ = b.Tick(0.1)
	if b.Phase() <= ph0 {
		t.Fatal("phase should advance while hovered")
	}
	b.SetHovered(false)
	if b.IsBeamVisible() {
		t.Fatal("leave hover hides beam")
	}
}

func TestBorderBeam_PRD_09_CustomContainerDemo(t *testing.T) {
	// BB-09 custom-container.tsx
	panel := primitive.NewDecorated(nil)
	panel.Width = 420
	panel.MinHeight = 160
	panel.Padding = primitive.All(24)
	panel.Radius = 8
	panel.BorderWidth = 1
	panel.BorderColor = render.Hex("#f0f0f0")
	panel.Background = render.Hex("#ffffff")
	body := kit.NewText("Review task status, deployment health, and recent automation activity in one custom container.")
	panel.AddChild(body.Node())
	b := kit.NewBorderBeam(panel)
	b.SetBorderRadius(8)
	_ = b.Node().Layout(core.Loose(480, 220))
	if r := b.ResolvedBorderRadius(); r < 7.5 || r > 8.5 {
		t.Fatalf("radius=%v want 8", r)
	}
	if !bbFindText(b.Node(), "Review task status, deployment health, and recent automation activity in one custom container.") {
		t.Fatal("custom container body missing")
	}
}

func TestBorderBeam_PRD_10_GradientDemo(t *testing.T) {
	// BB-10 customized-color.tsx (Ocean preset)
	ocean := []kit.BorderBeamColorStop{
		{Color: render.Hex("#1677ff"), Percent: 0},
		{Color: render.Hex("#36cfc9"), Percent: 52},
		{Color: render.Hex("#95de64"), Percent: 100},
	}
	card := kit.NewCard("Ocean")
	b := kit.NewBorderBeam(card.Node())
	b.SetColorStops(ocean...)
	_ = b.Node().Layout(core.Loose(400, 200))
	got := b.ResolvedColorStops()
	if len(got) != 3 {
		t.Fatalf("stops=%d", len(got))
	}
	if got[1].Percent != 52 {
		t.Fatalf("mid percent=%v", got[1].Percent)
	}
	// switch preset
	sunset := []kit.BorderBeamColorStop{
		{Color: render.Hex("#ff7a45"), Percent: 0},
		{Color: render.Hex("#ff4d4f"), Percent: 49},
		{Color: render.Hex("#ff85c0"), Percent: 100},
	}
	b.SetColorStops(sunset...)
	if b.ResolvedColorStops()[0].Color != render.Hex("#ff7a45") {
		t.Fatal("preset switch failed")
	}
}

func TestBorderBeam_PRD_11_DurationDemo(t *testing.T) {
	// BB-11 duration.tsx
	for _, d := range []float64{3, 6, 12} {
		card := kit.NewCard("d")
		b := kit.NewBorderBeam(card.Node())
		b.SetDuration(d)
		if got := b.ResolvedDuration(); math.Abs(got-d) > 0.01 {
			t.Fatalf("duration=%v want %v", got, d)
		}
		_ = b.Node().Layout(core.Loose(220, 120))
	}
}

func TestBorderBeam_PRD_12_SizeDemo(t *testing.T) {
	// BB-12 size.tsx
	cases := []struct {
		set  float64
		want float64
	}{
		{0, 100}, // default
		{56, 56},
		{160, 160},
	}
	for _, tc := range cases {
		b := kit.NewBorderBeam(kit.NewCard("s").Node())
		if tc.set > 0 {
			b.SetSize(tc.set)
		}
		if got := b.ResolvedSize(); math.Abs(got-tc.want) > 0.5 {
			t.Fatalf("size set=%v got=%v want=%v", tc.set, got, tc.want)
		}
	}
}

func TestBorderBeam_PRD_13_LineWidthDemo(t *testing.T) {
	// BB-13 line-width.tsx
	card := kit.NewCard("Custom line width")
	b := kit.NewBorderBeam(card.Node())
	b.SetLineWidth(2)
	_ = b.Node().Layout(core.Loose(360, 160))
	if lw := b.ResolvedLineWidth(); lw < 1.5 || lw > 2.5 {
		t.Fatalf("lineWidth=%v want 2", lw)
	}
}

func TestBorderBeam_PRD_14_Metrics(t *testing.T) {
	// BB-14 §6.2
	b := kit.NewBorderBeam(nil)
	th := core.DefaultTheme()
	b.SetTheme(th)
	if d := b.ResolvedDuration(); d < 5.5 || d > 6.5 {
		t.Fatalf("duration=%v", d)
	}
	if s := b.ResolvedSize(); s < 99.5 || s > 100.5 {
		t.Fatalf("size=%v", s)
	}
	if lw := b.ResolvedLineWidth(); lw < 0.5 || lw > 1.5 {
		t.Fatalf("lineWidth=%v", lw)
	}
	if r := b.ResolvedBorderRadius(); r < 5.5 || r > 6.5 {
		t.Fatalf("radius=%v want 6", r)
	}
	if fs := b.ResolvedFontSize(); fs < 13.5 || fs > 14.5 {
		t.Fatalf("fontSize=%v want 14", fs)
	}
}

func TestBorderBeam_PRD_15_TokenColors(t *testing.T) {
	// BB-15 default skin via Theme tokens
	th := core.DefaultTheme()
	b := kit.NewBorderBeam(nil)
	b.SetTheme(th)
	stops := b.ResolvedColorStops()
	if len(stops) < 2 {
		t.Fatal("default gradient needs ≥2 stops")
	}
	primary := th.Color(core.TokenColorPrimary)
	if stops[0].Color != primary {
		t.Fatalf("default head=%v want primary=%v", stops[0].Color, primary)
	}
	// layer must not steal hits
	btn := kit.NewButton("ok")
	b2 := kit.NewBorderBeam(btn.Node())
	root := b2.Node()
	_ = root.Layout(core.Loose(200, 80))
	for _, c := range root.Children() {
		if c.TypeID() == "kit.BorderBeamLayer" {
			if c.Base().Hit != core.HitTransparent {
				t.Fatalf("beam Hit=%v want Transparent", c.Base().Hit)
			}
		}
	}
	hit := root.HitTest(core.Point{X: 10, Y: 10})
	if hit != nil && hit.TypeID() == "kit.BorderBeamLayer" {
		t.Fatal("beam must not capture hits")
	}
}

func TestBorderBeam_PRD_16_DisabledNA(t *testing.T) {
	// BB-16 N/A — decorative control has no disabled chrome.
	t.Skip("BB-16 disabled N/A for BorderBeam")
}

func TestBorderBeam_PRD_17_KeyboardNA(t *testing.T) {
	// BB-17 N/A — beam is not focusable; children own keyboard.
	t.Skip("BB-17 keyboard/focus N/A for BorderBeam")
}
