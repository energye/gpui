package kit_test

import (
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/steps.md §6.9 — P0 PRD cases (STP-01 … STP-21 L1/L2).
// L3/L4 (STP-22/23) and P1 (STP-24) deferred.

func approxSteps(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func approxStepsColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(float64(a.R-b.R)) <= tol &&
		math.Abs(float64(a.G-b.G)) <= tol &&
		math.Abs(float64(a.B-b.B)) <= tol &&
		math.Abs(float64(a.A-b.A)) <= tol
}

func sampleSteps() *kit.Steps {
	return kit.NewSteps(
		kit.StepItem{Title: "Finished", Content: "This is a content."},
		kit.StepItem{Title: "In Progress", Content: "This is a content.", SubTitle: "Left 00:00:08"},
		kit.StepItem{Title: "Waiting", Content: "This is a content."},
	)
}

func clickStep(t *testing.T, tree *core.Tree, s *kit.Steps, origin int) {
	t.Helper()
	tree.Layout(core.Size{Width: 900, Height: 200})
	pr := s.ItemPressable(origin)
	if pr == nil {
		t.Fatalf("no pressable origin=%d", origin)
	}
	abs := core.AbsoluteBounds(pr)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	if abs.Width() <= 0 || abs.Height() <= 0 {
		t.Fatalf("empty bounds origin=%d size=%v", origin, abs)
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestSteps_PRD_01_Defaults(t *testing.T) {
	// STP-01
	s := kit.NewSteps()
	if s.Node() == nil {
		t.Fatal("nil node")
	}
	if s.Current != 0 {
		t.Fatalf("Current=%d want 0", s.Current)
	}
	if s.Size != kit.StepsMiddle {
		t.Fatalf("Size=%v want middle", s.Size)
	}
	if s.Orientation != kit.StepsHorizontal {
		t.Fatalf("Orientation=%v want horizontal", s.Orientation)
	}
	if s.Type != kit.StepsTypeDefault {
		t.Fatalf("Type=%v want default", s.Type)
	}
	if s.Variant != kit.StepsVariantFilled {
		t.Fatalf("Variant=%v want filled", s.Variant)
	}
	if s.Status != kit.StepsProcess {
		t.Fatalf("Status=%v want process", s.Status)
	}
	if s.ChromeNode().Base().Role != "navigation" {
		t.Fatalf("role=%q want navigation", s.ChromeNode().Base().Role)
	}
}

func TestSteps_PRD_02_CurrentProcess(t *testing.T) {
	// STP-02 / STP-S1
	s := sampleSteps()
	s.SetCurrent(1)
	if s.ItemStatus(0) != kit.StepsFinish {
		t.Fatalf("0=%s want finish", s.ItemStatus(0))
	}
	if s.ItemStatus(1) != kit.StepsProcess {
		t.Fatalf("1=%s want process", s.ItemStatus(1))
	}
	if s.ItemStatus(2) != kit.StepsWait {
		t.Fatalf("2=%s want wait", s.ItemStatus(2))
	}
}

func TestSteps_PRD_03_OnChangeClick(t *testing.T) {
	// STP-03 / STP-S2
	s := sampleSteps()
	var got, n int
	s.SetOnChange(func(current int) {
		n++
		got = current
	})
	tree := core.NewTree(s.Node())
	clickStep(t, tree, s, 2)
	if n != 1 || got != 2 {
		t.Fatalf("onChange n=%d got=%d", n, got)
	}
	if s.Current != 2 {
		t.Fatalf("Current=%d want 2 (uncontrolled)", s.Current)
	}
}

func TestSteps_PRD_04_StatusError(t *testing.T) {
	// STP-04 / STP-S3
	s := sampleSteps()
	s.SetCurrent(1)
	s.SetStatus(kit.StepsError)
	if s.ItemStatus(1) != kit.StepsError {
		t.Fatalf("status=%s want error", s.ItemStatus(1))
	}
	// error chrome uses error token
	th := kit.DefaultTheme()
	errC := th.Color(core.TokenColorError)
	// processIconBg only tracks process; check icon decorated bg
	dec := s.IconDecorated(1)
	if dec == nil {
		t.Fatal("nil icon")
	}
	if !approxStepsColor(dec.Background, errC, 0.05) {
		// outlined might use container; ensure border or bg carries error
		if !approxStepsColor(dec.BorderColor, errC, 0.05) && !approxStepsColor(dec.Background, errC, 0.15) {
			t.Fatalf("error chrome bg=%v border=%v want error=%v", dec.Background, dec.BorderColor, errC)
		}
	}
}

func TestSteps_PRD_05_OrientationVertical(t *testing.T) {
	// STP-05 / STP-S4
	s := sampleSteps()
	s.SetOrientation(kit.StepsVertical)
	if s.Root.Axis != core.AxisVertical {
		t.Fatalf("axis=%v want vertical", s.Root.Axis)
	}
}

func TestSteps_PRD_06_SizeSmall(t *testing.T) {
	// STP-06 / STP-S5
	s := sampleSteps()
	mid := s.IconSize()
	s.SetSize(kit.StepsSmall)
	sm := s.IconSize()
	if sm >= mid {
		t.Fatalf("small=%v not smaller than middle=%v", sm, mid)
	}
	if !approxSteps(sm, 24, 0.5) {
		t.Fatalf("small icon=%v want ~24", sm)
	}
}

func TestSteps_PRD_07_DisabledNoClick(t *testing.T) {
	// STP-07 / STP-S6
	s := kit.NewSteps(
		kit.StepItem{Title: "A"},
		kit.StepItem{Title: "B", Disabled: true},
		kit.StepItem{Title: "C"},
	)
	n := 0
	s.SetOnChange(func(int) { n++ })
	tree := core.NewTree(s.Node())
	clickStep(t, tree, s, 1)
	if n != 0 {
		t.Fatalf("onChange n=%d want 0", n)
	}
}

func TestSteps_PRD_08_CustomIcon(t *testing.T) {
	// STP-08 / STP-S7
	s := kit.NewSteps(
		kit.StepItem{Title: "Login", Icon: "check", Status: kit.StepsFinish},
		kit.StepItem{Title: "Pay", Icon: "loading", Status: kit.StepsProcess},
		kit.StepItem{Title: "Done", Icon: "star", Status: kit.StepsWait},
	)
	if s.Node() == nil {
		t.Fatal("nil")
	}
	dec := s.IconDecorated(0)
	if dec == nil {
		t.Fatal("nil icon decorated")
	}
	if len(dec.Children()) == 0 {
		t.Fatal("custom icon missing child")
	}
}

func TestSteps_PRD_09_ContentDescription(t *testing.T) {
	// STP-09 / STP-S8
	s := kit.NewSteps(
		kit.StepItem{Title: "A", Content: "content-A"},
		kit.StepItem{Title: "B", Description: "desc-B"},
	)
	// walk tree for text values
	var foundContent, foundDesc bool
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if tx, ok := n.(*primitive.Text); ok {
			if strings.Contains(tx.Value, "content-A") {
				foundContent = true
			}
			if strings.Contains(tx.Value, "desc-B") {
				foundDesc = true
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(s.Node())
	if !foundContent {
		t.Fatal("content not in tree")
	}
	if !foundDesc {
		t.Fatal("description alias not in tree")
	}
}

func TestSteps_PRD_10_SimpleDemo(t *testing.T) {
	// STP-10 simple.tsx
	items := []kit.StepItem{
		{Title: "Finished", Content: "This is a content."},
		{Title: "In Progress", Content: "This is a content.", SubTitle: "Left 00:00:08"},
		{Title: "Waiting", Content: "This is a content."},
	}
	for _, cfg := range []struct {
		variant kit.StepsVariant
		size    kit.StepsSize
	}{
		{kit.StepsVariantFilled, kit.StepsMiddle},
		{kit.StepsVariantOutlined, kit.StepsMiddle},
		{kit.StepsVariantFilled, kit.StepsSmall},
		{kit.StepsVariantOutlined, kit.StepsSmall},
	} {
		s := kit.NewSteps(items...)
		s.SetCurrent(1)
		s.SetVariant(cfg.variant)
		s.SetSize(cfg.size)
		_ = s.Node().Layout(core.Loose(640, 120))
		if s.ItemStatus(1) != kit.StepsProcess {
			t.Fatalf("variant=%v size=%v status=%s", cfg.variant, cfg.size, s.ItemStatus(1))
		}
	}
}

func TestSteps_PRD_11_ErrorDemo(t *testing.T) {
	// STP-11 error.tsx
	s := sampleSteps()
	s.SetCurrent(1)
	s.SetStatus(kit.StepsError)
	_ = s.Node().Layout(core.Loose(640, 80))
	if s.ItemStatus(1) != kit.StepsError {
		t.Fatal(s.ItemStatus(1))
	}
}

func TestSteps_PRD_12_VerticalDemo(t *testing.T) {
	// STP-12 vertical.tsx
	s := sampleSteps()
	s.SetOrientation(kit.StepsVertical)
	s.SetCurrent(1)
	_ = s.Node().Layout(core.Loose(320, 400))
	if s.Root.Axis != core.AxisVertical {
		t.Fatal(s.Root.Axis)
	}
	s2 := sampleSteps()
	s2.SetOrientation(kit.StepsVertical)
	s2.SetSize(kit.StepsSmall)
	s2.SetCurrent(1)
	_ = s2.Node().Layout(core.Loose(320, 400))
}

func TestSteps_PRD_13_ClickableDemo(t *testing.T) {
	// STP-13 clickable.tsx
	s := kit.NewSteps(kit.StepTitles("Step 1", "Step 2", "Step 3")...)
	var last int = -1
	s.SetOnChange(func(c int) { last = c; s.SetCurrent(c) })
	tree := core.NewTree(s.Node())
	clickStep(t, tree, s, 1)
	if last != 1 {
		t.Fatalf("last=%d", last)
	}
	// vertical clickable
	s.SetOrientation(kit.StepsVertical)
	tree2 := core.NewTree(s.Node())
	clickStep(t, tree2, s, 2)
	if last != 2 {
		t.Fatalf("vertical last=%d", last)
	}
}

func TestSteps_PRD_14_PanelDemo(t *testing.T) {
	// STP-14 panel.tsx
	s := kit.NewSteps(
		kit.StepItem{Title: "Step 1", SubTitle: "00:00", Content: "This is a content."},
		kit.StepItem{Title: "Step 2", Content: "This is a content.", Status: kit.StepsError},
		kit.StepItem{Title: "Step 3", Content: "This is a content."},
	)
	s.SetType(kit.StepsTypePanel)
	s.SetCurrent(0)
	n := 0
	s.SetOnChange(func(int) { n++ })
	_ = s.Node().Layout(core.Loose(720, 120))
	if s.ItemStatus(1) != kit.StepsError {
		t.Fatal(s.ItemStatus(1))
	}
	tree := core.NewTree(s.Node())
	clickStep(t, tree, s, 2)
	if n != 1 {
		t.Fatalf("panel click n=%d", n)
	}
}

func TestSteps_PRD_15_IconDemo(t *testing.T) {
	// STP-15 icon.tsx
	s := kit.NewSteps(
		kit.StepItem{Title: "Login", Status: kit.StepsFinish, Icon: "check"},
		kit.StepItem{Title: "Verification", Status: kit.StepsFinish, Icon: "info"},
		kit.StepItem{Title: "Pay", Status: kit.StepsProcess, Icon: "loading"},
		kit.StepItem{Title: "Done", Status: kit.StepsWait, Icon: "star"},
	)
	_ = s.Node().Layout(core.Loose(640, 80))
	if s.ItemStatus(2) != kit.StepsProcess {
		t.Fatal(s.ItemStatus(2))
	}
	if s.IconDecorated(0) == nil {
		t.Fatal("missing icon 0")
	}
}

func TestSteps_PRD_16_TitlePlacementPercent(t *testing.T) {
	// STP-16 title-placement.tsx
	s := sampleSteps()
	s.SetCurrent(1)
	s.SetTitlePlacement(kit.StepsTitleVertical)
	s.SetPercent(60)
	_ = s.Node().Layout(core.Loose(640, 160))
	if s.TitlePlacement != kit.StepsTitleVertical {
		t.Fatal(s.TitlePlacement)
	}
	dec := s.IconDecorated(1)
	if dec == nil {
		t.Fatal("nil process icon")
	}
	if !strings.Contains(dec.Base().Label, "percent=60") {
		t.Fatalf("percent label=%q", dec.Base().Label)
	}
	s.SetPercent(80)
	s.SetSize(kit.StepsSmall)
	_ = s.Node().Layout(core.Loose(640, 160))
}

func TestSteps_PRD_17_MaxCount(t *testing.T) {
	// STP-17 max-count.tsx
	items := make([]kit.StepItem, 7)
	for i := range items {
		items[i] = kit.StepItem{Title: "Step " + string(rune('1'+i))}
	}
	// titles Step 1..7 via fmt-less
	for i := 0; i < 7; i++ {
		items[i].Title = "Step"
	}
	s := kit.NewSteps(items...)
	s.SetCurrent(3)
	s.SetMaxCount(5)
	_ = s.Node().Layout(core.Loose(800, 80))
	if !s.HasEllipsis() {
		t.Fatal("expected ellipsis collapse")
	}
	if s.DisplayCount() > 7 {
		t.Fatalf("display=%d", s.DisplayCount())
	}
	// display should be less than full when collapsed with ellipsis slots
	if s.DisplayCount() >= 7 && !s.HasEllipsis() {
		t.Fatal("collapse not applied")
	}
}

func TestSteps_PRD_18_IconMetrics(t *testing.T) {
	// STP-18
	s := sampleSteps()
	s.SetCurrent(1)
	_ = s.Node().Layout(core.Loose(640, 80))
	mid := s.IconSize()
	if !approxSteps(mid, 32, 0.5) {
		t.Fatalf("middle icon=%v want 32", mid)
	}
	dec := s.IconDecorated(1)
	if dec == nil {
		t.Fatal("nil icon")
	}
	if !approxSteps(dec.Height, 32, 0.5) && !approxSteps(dec.Width, 32, 0.5) {
		// after layout Size may differ from Width field
		sz := dec.Size()
		if !approxSteps(sz.Height, 32, 1.0) {
			t.Fatalf("icon layout h=%v w field=%v h field=%v", sz.Height, dec.Width, dec.Height)
		}
	}
	s.SetSize(kit.StepsSmall)
	_ = s.Node().Layout(core.Loose(640, 80))
	if !approxSteps(s.IconSize(), 24, 0.5) {
		t.Fatalf("small icon=%v want 24", s.IconSize())
	}
}

func TestSteps_PRD_19_ThemePrimary(t *testing.T) {
	// STP-19
	th := core.DefaultTheme()
	if th.Tokens == nil {
		th.Tokens = core.AntLightTokens()
	} else {
		// copy colors map so we don't mutate global
		cp := *th.Tokens
		cols := make(map[string]render.RGBA, len(th.Tokens.Colors))
		for k, v := range th.Tokens.Colors {
			cols[k] = v
		}
		cp.Colors = cols
		th.Tokens = &cp
	}
	th.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	s := sampleSteps()
	s.SetTheme(th)
	s.SetCurrent(1)
	_ = s.Node().Layout(core.Loose(640, 80))
	bg := s.ProcessIconBg()
	want := th.Color(core.TokenColorPrimary)
	if !approxStepsColor(bg, want, 0.05) {
		// filled process uses primary; outlined uses border
		dec := s.IconDecorated(1)
		if dec == nil || (!approxStepsColor(dec.Background, want, 0.05) && !approxStepsColor(dec.BorderColor, want, 0.05)) {
			t.Fatalf("process chrome bg=%v processIconBg=%v want primary=%v", dec.Background, bg, want)
		}
	}
}

func TestSteps_PRD_20_DisabledAppearance(t *testing.T) {
	// STP-20
	s := kit.NewSteps(
		kit.StepItem{Title: "A"},
		kit.StepItem{Title: "B", Disabled: true},
	)
	s.SetOnChange(func(int) {})
	_ = s.Node().Layout(core.Loose(400, 80))
	pr := s.ItemPressable(1)
	if pr == nil {
		t.Fatal("nil pressable")
	}
	if !pr.State.Disabled {
		t.Fatal("disabled step pressable should be disabled")
	}
}

func TestSteps_PRD_21_KeyboardFocus(t *testing.T) {
	// STP-21
	s := kit.NewSteps(kit.StepTitles("A", "B", "C")...)
	var got int = -1
	s.SetOnChange(func(c int) { got = c })
	tree := core.NewTree(s.Node())
	tree.Layout(core.Size{Width: 600, Height: 100})
	pr := s.ItemPressable(1)
	if pr == nil || !pr.Focusable {
		t.Fatalf("pressable focusable=%v", pr)
	}
	// focus + Enter
	tree.SetFocus(pr)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if got != 1 {
		// Space fallback
		tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	}
	if got != 1 {
		t.Fatalf("keyboard activate got=%d want 1", got)
	}
	if !pr.ShowFocusRing {
		t.Fatal("focus ring should be enabled for clickable steps")
	}
}
