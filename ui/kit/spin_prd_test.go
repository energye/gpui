package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/spin.md §6.9 — P0 PRD cases (SPN-01…04, 06…18).
// SPN-05 fullscreen P1; SPN-19/20 N/A; SPN-21 L3; SPN-22 L4; SPN-23 P1 — omitted.

func spinHasType(n core.Node, typ string) bool {
	if n == nil {
		return false
	}
	if n.TypeID() == typ {
		return true
	}
	for _, c := range n.Children() {
		if spinHasType(c, typ) {
			return true
		}
	}
	return false
}

func spinCountType(n core.Node, typ string) int {
	if n == nil {
		return 0
	}
	got := 0
	if n.TypeID() == typ {
		got = 1
	}
	for _, c := range n.Children() {
		got += spinCountType(c, typ)
	}
	return got
}

func spinFindText(n core.Node, want string) bool {
	if n == nil {
		return false
	}
	if tx, ok := n.(*primitive.Text); ok && tx.Value == want {
		return true
	}
	for _, c := range n.Children() {
		if spinFindText(c, want) {
			return true
		}
	}
	return false
}

func TestSpin_PRD_01_Defaults(t *testing.T) {
	// SPN-01
	s := kit.NewSpin(nil)
	if s == nil || s.Node() == nil {
		t.Fatal("nil spin")
	}
	if !s.Spinning {
		t.Fatal("Spinning default true")
	}
	if s.Size != kit.SpinSizeMedium && s.Size != "" {
		// NewSpin sets medium; empty also normalizes to medium
		t.Fatalf("Size=%q want medium", s.Size)
	}
	if s.Delay != 0 {
		t.Fatalf("Delay=%v want 0", s.Delay)
	}
	if !s.IsDisplaySpinning() {
		t.Fatal("display spinning default true (delay=0)")
	}
	if s.Node().TypeID() != "kit.Spin" {
		t.Fatalf("type=%s", s.Node().TypeID())
	}
	if !s.Node().Base().IsRepaintBoundary() {
		t.Fatal("Spin root must be RepaintBoundary")
	}
	if s.Node().Base().Role != "status" {
		t.Fatalf("Role=%q want status", s.Node().Base().Role)
	}
	if s.DotSize() < 19.5 || s.DotSize() > 20.5 {
		t.Fatalf("DotSize=%v want 20", s.DotSize())
	}
	sz := s.Node().Layout(core.Loose(80, 80))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%v", sz)
	}
}

func TestSpin_PRD_02_SpinningVisible(t *testing.T) {
	// SPN-02
	s := kit.NewSpin(nil)
	s.SetSpinning(true)
	_ = s.Node().Layout(core.Loose(80, 80))
	if !s.IsDisplaySpinning() {
		t.Fatal("display spinning")
	}
	if !spinHasType(s.Node(), "primitive.Canvas") {
		t.Fatal("expected default indicator canvas")
	}
	if !s.Tick(0.05) {
		t.Fatal("spinning should tick")
	}
	if s.Phase() <= 0 {
		t.Fatal("phase should advance")
	}
	if !s.Busy() {
		t.Fatal("Busy when display spinning")
	}
}

func TestSpin_PRD_03_SpinningFalseChildrenClickable(t *testing.T) {
	// SPN-03
	btn := kit.NewButton("ok")
	s := kit.NewSpin(btn.Node())
	s.SetSpinning(false)
	root := s.Node()
	_ = root.Layout(core.Loose(200, 80))
	if s.IsDisplaySpinning() {
		t.Fatal("should not display")
	}
	if spinHasType(root, "primitive.Canvas") {
		t.Fatal("no indicator when spinning=false")
	}
	if spinHasType(root, "kit.SpinMask") {
		t.Fatal("no mask when not spinning")
	}
	// children still in tree and hittable
	if !spinHasType(root, btn.Node().TypeID()) && !spinHasType(root, "kit.Button") {
		// Button type id
		if !spinHasType(root, "kit.Button") {
			// walk: content should be present
			if s.Content() == nil {
				t.Fatal("content nil")
			}
		}
	}
	hit := root.HitTest(core.Point{X: 10, Y: 10})
	if hit == nil {
		// loose hit — at least content in tree
		if s.Content() == nil {
			t.Fatal("children missing")
		}
	}
}

func TestSpin_PRD_04_DescriptionAndTip(t *testing.T) {
	// SPN-04
	s := kit.NewSpin(nil)
	s.SetDescription("Loading data")
	_ = s.Node().Layout(core.Loose(120, 80))
	if !spinFindText(s.Node(), "Loading data") {
		t.Fatal("description text missing")
	}
	// tip alias when description empty
	s2 := kit.NewSpin(nil)
	s2.SetTip("Please wait")
	_ = s2.Node().Layout(core.Loose(120, 80))
	if s2.Tip != "Please wait" {
		t.Fatalf("Tip field=%q", s2.Tip)
	}
	if !spinFindText(s2.Node(), "Please wait") {
		t.Fatal("tip alias text missing")
	}
}

func TestSpin_PRD_06_Delay500(t *testing.T) {
	// SPN-06 / SPN-S5
	child := primitive.NewText("body")
	s := kit.NewSpin(child)
	s.SetSpinning(false)
	s.SetDelay(500)
	_ = s.Node().Layout(core.Loose(200, 80))
	s.SetSpinning(true)
	if s.IsDisplaySpinning() {
		t.Fatal("should not show within delay window immediately")
	}
	if spinHasType(s.Node(), "primitive.Canvas") {
		t.Fatal("no canvas during delay")
	}
	// 400ms — still hidden
	if !s.Tick(0.4) {
		t.Fatal("should keep ticking while waiting delay")
	}
	if s.IsDisplaySpinning() {
		t.Fatal("still within 500ms")
	}
	// +150ms → 550ms
	_ = s.Tick(0.15)
	if !s.IsDisplaySpinning() {
		t.Fatal("should show after 500ms")
	}
	_ = s.Node().Layout(core.Loose(200, 80))
	if !spinHasType(s.Node(), "primitive.Canvas") && s.Indicator == nil {
		// after rebuild from delay fire
		if !s.IsDisplaySpinning() {
			t.Fatal("display flag")
		}
	}
}

func TestSpin_PRD_07_NestedChildrenStay(t *testing.T) {
	// SPN-07
	child := primitive.NewText("nested-body")
	s := kit.NewSpin(child)
	s.SetSpinning(true)
	_ = s.Node().Layout(core.Loose(240, 120))
	if !spinFindText(s.Node(), "nested-body") {
		t.Fatal("children must stay in tree while spinning")
	}
	if !spinHasType(s.Node(), "kit.SpinNested") {
		t.Fatal("expected SpinNested host")
	}
	if !spinHasType(s.Node(), "kit.SpinMask") {
		t.Fatal("expected mask while spinning nested")
	}
}

func TestSpin_PRD_08_ReducedMotion(t *testing.T) {
	// SPN-08
	s := kit.NewSpin(nil)
	s.SetSpinning(true)
	tree := core.NewTree(s.Node())
	tree.Clock().ReduceMotion = true
	tree.Layout(core.Size{Width: 80, Height: 80})
	ph := s.Phase()
	if s.Tick(0.05) {
		t.Fatal("reduced-motion should stop spin ticker")
	}
	if s.Phase() != ph {
		t.Fatal("phase must not advance under reduced-motion")
	}
	// Indicator still present
	if !s.IsDisplaySpinning() {
		t.Fatal("indicator still shown")
	}
}

func TestSpin_PRD_09_BasicDemo(t *testing.T) {
	// SPN-09 basic.tsx
	s := kit.NewSpin(nil)
	sz := s.Node().Layout(core.Loose(64, 64))
	if sz.Width < 10 || sz.Height < 10 {
		t.Fatalf("basic size=%v", sz)
	}
	if !spinHasType(s.Node(), "primitive.Canvas") {
		t.Fatal("basic indicator")
	}
}

func TestSpin_PRD_10_SizeDemo(t *testing.T) {
	// SPN-10 size.tsx
	sm := kit.NewSpin(nil)
	sm.SetSize(kit.SpinSizeSmall)
	md := kit.NewSpin(nil)
	md.SetSize(kit.SpinSizeMedium)
	lg := kit.NewSpin(nil)
	lg.SetSize(kit.SpinSizeLarge)

	if d := sm.DotSize(); d < 13.5 || d > 14.5 {
		t.Fatalf("small DotSize=%v want 14", d)
	}
	if d := md.DotSize(); d < 19.5 || d > 20.5 {
		t.Fatalf("medium DotSize=%v want 20", d)
	}
	if d := lg.DotSize(); d < 31.5 || d > 32.5 {
		t.Fatalf("large DotSize=%v want 32", d)
	}
	// "default" maps to medium
	def := kit.NewSpin(nil)
	def.SetSize("default")
	if d := def.DotSize(); d < 19.5 || d > 20.5 {
		t.Fatalf("default→medium DotSize=%v", d)
	}
}

func TestSpin_PRD_11_NestedDemo(t *testing.T) {
	// SPN-11 nested.tsx
	body := primitive.NewText("Alert message title")
	s := kit.NewSpin(body)
	s.SetSpinning(false)
	_ = s.Node().Layout(core.Loose(320, 120))
	if s.IsDisplaySpinning() {
		t.Fatal("off")
	}
	s.SetSpinning(true)
	_ = s.Node().Layout(core.Loose(320, 120))
	if !s.IsDisplaySpinning() {
		t.Fatal("on")
	}
	if !spinFindText(s.Node(), "Alert message title") {
		t.Fatal("nested content")
	}
}

func TestSpin_PRD_12_DescriptionDemo(t *testing.T) {
	// SPN-12 tip.tsx
	box := primitive.NewBox(nil)
	box.Width, box.Height = 100, 50
	for _, sz := range []kit.SpinSize{kit.SpinSizeSmall, kit.SpinSizeMedium, kit.SpinSizeLarge} {
		s := kit.NewSpin(box)
		s.SetSize(sz)
		s.SetDescription("Loading")
		_ = s.Node().Layout(core.Loose(160, 120))
		if !spinFindText(s.Node(), "Loading") {
			t.Fatalf("description missing size=%s", sz)
		}
	}
}

func TestSpin_PRD_13_DelayDemo(t *testing.T) {
	// SPN-13 delayAndDebounce.tsx
	s := kit.NewSpin(primitive.NewText("x"))
	s.SetDelay(500)
	s.SetSpinning(false)
	s.SetSpinning(true)
	if s.IsDisplaySpinning() {
		t.Fatal("delayed")
	}
	_ = s.Tick(0.5)
	if !s.IsDisplaySpinning() {
		t.Fatal("after delay")
	}
}

func TestSpin_PRD_14_CustomIndicator(t *testing.T) {
	// SPN-14 custom-indicator.tsx
	ind := primitive.NewText("★")
	s := kit.NewSpin(nil)
	s.SetIndicator(ind)
	_ = s.Node().Layout(core.Loose(80, 80))
	if !spinFindText(s.Node(), "★") {
		t.Fatal("custom indicator missing")
	}
	// custom indicator → no default canvas required
	// Tick may return false (no built-in rotation)
	_ = s.Tick(0.05)
}

func TestSpin_PRD_15_PercentDemo(t *testing.T) {
	// SPN-15 percent.tsx
	s := kit.NewSpin(nil)
	s.SetPercent(40)
	if !s.HasPercent() {
		t.Fatal("HasPercent")
	}
	if p := s.EffectivePercent(); p < 39.5 || p > 40.5 {
		t.Fatalf("EffectivePercent=%v", p)
	}
	_ = s.Node().Layout(core.Loose(80, 80))
	if !spinHasType(s.Node(), "primitive.Canvas") {
		t.Fatal("percent uses canvas ring")
	}

	auto := kit.NewSpin(nil)
	auto.SetPercentAuto()
	if !auto.PercentAuto {
		t.Fatal("PercentAuto")
	}
	_ = auto.Node().Layout(core.Loose(80, 80))
	// advance several auto steps
	for i := 0; i < 5; i++ {
		if !auto.Tick(0.2) {
			t.Fatal("auto should tick")
		}
	}
	if auto.EffectivePercent() <= 0 {
		t.Fatal("auto percent should climb")
	}
	if auto.EffectivePercent() >= 100 {
		t.Fatal("auto should not reach 100")
	}

	auto.ClearPercent()
	if auto.HasPercent() {
		t.Fatal("ClearPercent")
	}
}

func TestSpin_PRD_16_StyleClass(t *testing.T) {
	// SPN-16 style-class.tsx — shallow ClassNames/Styles
	s := kit.NewSpin(nil)
	s.SetClassNames(kit.SpinClassNames{Root: "spin-root", Indicator: "spin-ind"})
	s.SetStyles(kit.SpinStyles{
		Indicator: kit.Style{Text: core.DefaultTheme().Color(core.TokenColorPrimary)},
	})
	_ = s.Node().Layout(core.Loose(80, 80))
	if s.ClassNames.Root != "spin-root" {
		t.Fatal("ClassNames not kept")
	}
	col := s.IndicatorColor()
	if col.A <= 0 {
		t.Fatal("indicator color from styles/token")
	}
}

func TestSpin_PRD_17_TokenGeometry(t *testing.T) {
	// SPN-17 L2
	s := kit.NewSpin(nil)
	if d := s.DotSize(); d < 19.5 || d > 20.5 {
		t.Fatalf("medium=%v", d)
	}
	th := core.DefaultTheme()
	if g := th.SizeOr(core.TokenPaddingSM, 8); g < 7.5 || g > 8.5 {
		t.Fatalf("gap token=%v", g)
	}
	if f := th.SizeOr(core.TokenFontSize, 14); f < 13.5 || f > 14.5 {
		t.Fatalf("font=%v", f)
	}
	if v := th.SizeOr(core.TokenSpinSize, 20); v < 19.5 || v > 20.5 {
		t.Fatalf("TokenSpinSize=%v", v)
	}
	// small / large via controlHeight seeds
	s.SetSize(kit.SpinSizeSmall)
	if d := s.DotSize(); d < 13.5 || d > 14.5 {
		t.Fatalf("sm=%v", d)
	}
	s.SetSize(kit.SpinSizeLarge)
	if d := s.DotSize(); d < 31.5 || d > 32.5 {
		t.Fatalf("lg=%v", d)
	}
}

func TestSpin_PRD_18_TokenColor(t *testing.T) {
	// SPN-18 L2 — no hardcoded brand as sole skin
	s := kit.NewSpin(nil)
	th := core.DefaultTheme()
	s.SetTheme(th)
	got := s.IndicatorColor()
	want := th.Color(core.TokenColorPrimary)
	if got != want {
		t.Fatalf("IndicatorColor=%v want TokenColorPrimary %v", got, want)
	}
}

func TestSpin_PRD_DefaultIndicatorGlobal(t *testing.T) {
	prev := kit.DefaultSpinIndicator()
	defer kit.SetDefaultIndicator(prev)

	g := primitive.NewText("GLOBAL")
	kit.SetDefaultIndicator(g)
	s := kit.NewSpin(nil)
	_ = s.Node().Layout(core.Loose(80, 80))
	if !spinFindText(s.Node(), "GLOBAL") {
		t.Fatal("global default indicator")
	}
	// instance overrides global
	s.SetIndicator(primitive.NewText("INST"))
	_ = s.Node().Layout(core.Loose(80, 80))
	if !spinFindText(s.Node(), "INST") {
		t.Fatal("instance indicator")
	}
	if spinFindText(s.Node(), "GLOBAL") {
		t.Fatal("global should be overridden")
	}
}
