package kit_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/slider.md §6.9 — P0 PRD cases (SLD-01 … SLD-23).
// L3/L4 (SLD-24/25) and P1 (SLD-26) deferred.

func layoutSlider(t *testing.T, s *kit.Slider, w, h float64) *core.Tree {
	t.Helper()
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 48
	}
	tree := core.NewTree(s.Node())
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func sliderPointer(t *testing.T, tree *core.Tree, s *kit.Slider, typ core.PointerType, ratio float64) {
	t.Helper()
	n := s.Node()
	tree.Layout(core.Size{Width: 320, Height: 48})
	if s.IsVertical() {
		tree.Layout(core.Size{Width: 80, Height: 300})
	}
	abs := core.AbsoluteBounds(n)
	// Map ratio onto the painted rail: inset ≈ handle/2 + hitPad (5+4=9 default).
	const inset = 9.0
	var x, y float64
	if s.IsVertical() {
		x = (abs.Min.X + abs.Max.X) / 2
		// vertical: bottom=min → ratio 0 at bottom of rail
		railH := (abs.Max.Y - abs.Min.Y) - 2*inset
		if railH < 1 {
			railH = abs.Max.Y - abs.Min.Y
		}
		y = abs.Max.Y - inset - railH*ratio
	} else {
		railW := (abs.Max.X - abs.Min.X) - 2*inset
		if railW < 1 {
			railW = abs.Max.X - abs.Min.X
		}
		x = abs.Min.X + inset + railW*ratio
		y = abs.Min.Y + inset // near handle row
		// prefer vertical center of control box when short
		if abs.Max.Y-abs.Min.Y < 40 {
			y = (abs.Min.Y + abs.Max.Y) / 2
		}
	}
	// clamp into bounds
	if x >= abs.Max.X {
		x = abs.Max.X - 0.5
	}
	if x < abs.Min.X {
		x = abs.Min.X + 0.5
	}
	if y >= abs.Max.Y {
		y = abs.Max.Y - 0.5
	}
	if y < abs.Min.Y {
		y = abs.Min.Y + 0.5
	}
	tree.DispatchPointer(&core.PointerEvent{Type: typ, X: x, Y: y, Button: core.ButtonLeft})
}

func sliderDragTo(t *testing.T, tree *core.Tree, s *kit.Slider, ratio float64) {
	t.Helper()
	sliderPointer(t, tree, s, core.PointerDown, ratio)
	sliderPointer(t, tree, s, core.PointerMove, ratio)
	sliderPointer(t, tree, s, core.PointerUp, ratio)
}

func sliderClickRatio(t *testing.T, tree *core.Tree, s *kit.Slider, ratio float64) {
	t.Helper()
	sliderPointer(t, tree, s, core.PointerDown, ratio)
	sliderPointer(t, tree, s, core.PointerUp, ratio)
}

func approxSlider(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestSlider_PRD_01_Defaults(t *testing.T) {
	// SLD-01
	s := kit.NewSlider(0)
	if s.Value != 0 {
		t.Fatalf("Value=%v want 0", s.Value)
	}
	if s.Min != kit.DefaultSliderMin || s.Max != kit.DefaultSliderMax {
		t.Fatalf("range=%v..%v", s.Min, s.Max)
	}
	if s.Step != kit.DefaultSliderStep {
		t.Fatalf("Step=%v", s.Step)
	}
	if s.Disabled || s.Controlled || s.Range {
		t.Fatalf("flags disabled=%v controlled=%v range=%v", s.Disabled, s.Controlled, s.Range)
	}
	if !s.Keyboard {
		t.Fatal("Keyboard default want true")
	}
	if !s.Included {
		t.Fatal("Included default want true")
	}
	if s.Dots {
		t.Fatal("Dots default want false")
	}
	if s.Orientation != kit.SliderHorizontal {
		t.Fatalf("Orientation=%v want horizontal", s.Orientation)
	}
	if s.TooltipOpen != kit.SliderTooltipAuto {
		t.Fatalf("TooltipOpen=%v want Auto", s.TooltipOpen)
	}
	if s.Node() == nil || s.Root == nil {
		t.Fatal("nil node")
	}
	if s.Root.Base().Role != "slider" {
		t.Fatalf("role=%q want slider", s.Root.Base().Role)
	}
	// NewSlider(default) seeds value
	s2 := kit.NewSlider(40)
	if s2.Value != 40 {
		t.Fatalf("NewSlider(40)=%v", s2.Value)
	}
}

func TestSlider_PRD_02_DragToMax(t *testing.T) {
	// SLD-02 / SLD-S1
	s := kit.NewSlider(10)
	var got []float64
	s.SetOnChange(func(v float64) { got = append(got, v) })
	tree := layoutSlider(t, s, 320, 48)
	sliderDragTo(t, tree, s, 1.0)
	if !approxSlider(s.Value, 100, 1) {
		t.Fatalf("Value=%v want ~100", s.Value)
	}
	if len(got) == 0 || !approxSlider(got[len(got)-1], 100, 1) {
		t.Fatalf("onChange=%v", got)
	}
}

func TestSlider_PRD_03_ClampMinMax(t *testing.T) {
	// SLD-03 / SLD-S2
	s := kit.NewSlider(0)
	s.SetMin(10)
	s.SetMax(50)
	s.SetValue(0)
	if s.Value != 10 {
		t.Fatalf("clamp low Value=%v want 10", s.Value)
	}
	s.SetValue(999)
	if s.Value != 50 {
		t.Fatalf("clamp high Value=%v want 50", s.Value)
	}
}

func TestSlider_PRD_04_Step10(t *testing.T) {
	// SLD-04 / SLD-S3
	s := kit.NewSlider(0)
	s.SetStep(10)
	s.SetValue(23)
	if s.Value != 20 {
		t.Fatalf("Value=%v want 20 (step 10)", s.Value)
	}
	s.SetValue(26)
	if s.Value != 30 {
		t.Fatalf("Value=%v want 30", s.Value)
	}
	// interaction snap
	var last float64 = -1
	s.SetOnChange(func(v float64) { last = v })
	tree := layoutSlider(t, s, 320, 48)
	sliderClickRatio(t, tree, s, 0.23)
	if last >= 0 && math.Mod(last, 10) != 0 {
		t.Fatalf("onChange=%v not multiple of 10", last)
	}
	if math.Mod(s.Value, 10) != 0 {
		t.Fatalf("Value=%v not multiple of 10", s.Value)
	}
}

func TestSlider_PRD_05_RangeTwoHandles(t *testing.T) {
	// SLD-05 / SLD-S4
	s := kit.NewSlider(0)
	s.SetRange(true)
	s.SetDefaultValues(20, 50)
	lo, hi := s.Values()
	if lo != 20 || hi != 50 {
		t.Fatalf("Values=%v,%v want 20,50", lo, hi)
	}
	var gotLo, gotHi float64
	s.SetOnRangeChange(func(a, b float64) { gotLo, gotHi = a, b })
	tree := layoutSlider(t, s, 320, 48)
	// drag low handle area toward 0
	sliderDragTo(t, tree, s, 0.05)
	lo, hi = s.Values()
	if lo > hi {
		t.Fatalf("crossed lo=%v hi=%v", lo, hi)
	}
	if lo > 15 {
		t.Fatalf("expected low moved down, lo=%v (rangeCB=%v,%v)", lo, gotLo, gotHi)
	}
}

func TestSlider_PRD_06_MarksClick(t *testing.T) {
	// SLD-06 / SLD-S5
	s := kit.NewSlider(0)
	s.SetMarks(
		kit.SliderMark{Value: 0, Label: "0°C"},
		kit.SliderMark{Value: 26, Label: "26°C"},
		kit.SliderMark{Value: 37, Label: "37°C"},
		kit.SliderMark{Value: 100, Label: "100°C"},
	)
	s.SetDefaultValue(0)
	tree := layoutSlider(t, s, 320, 64)
	// click near 37% along rail
	sliderClickRatio(t, tree, s, 0.37)
	if !approxSlider(s.Value, 37, 2) && s.Value != 26 && s.Value != 37 {
		// markNear may snap if within hit radius; else ratio snap to step
		// with default step=1, 0.37*100=37
		if !approxSlider(s.Value, 37, 1) {
			t.Fatalf("Value=%v want ~37 (mark)", s.Value)
		}
	}
	// step=null → only marks
	s2 := kit.NewSlider(0)
	s2.SetMarks(
		kit.SliderMark{Value: 0, Label: "0"},
		kit.SliderMark{Value: 26, Label: "26"},
		kit.SliderMark{Value: 37, Label: "37"},
		kit.SliderMark{Value: 100, Label: "100"},
	)
	s2.SetStepNull(true)
	s2.SetDefaultValue(0)
	tree2 := layoutSlider(t, s2, 320, 64)
	sliderClickRatio(t, tree2, s2, 0.40)
	v := s2.Value
	if v != 0 && v != 26 && v != 37 && v != 100 {
		t.Fatalf("step=null Value=%v not a mark", v)
	}
}

func TestSlider_PRD_07_Disabled(t *testing.T) {
	// SLD-07 / SLD-S6
	s := kit.NewSlider(0)
	s.SetDefaultValue(40)
	s.SetDisabled(true)
	n := 0
	s.SetOnChange(func(float64) { n++ })
	tree := layoutSlider(t, s, 320, 48)
	sliderDragTo(t, tree, s, 0.9)
	if n != 0 || s.Value != 40 {
		t.Fatalf("disabled: n=%d value=%v", n, s.Value)
	}
}

func TestSlider_PRD_08_KeyboardRight(t *testing.T) {
	// SLD-08 / SLD-S7
	s := kit.NewSlider(0)
	s.SetDefaultValue(20)
	s.SetStep(10)
	tree := layoutSlider(t, s, 320, 48)
	tree.SetFocus(s.Node())
	if !s.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"}) {
		t.Fatal("ArrowRight not handled")
	}
	if s.Value != 30 {
		t.Fatalf("Value=%v want 30", s.Value)
	}
	s.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowLeft"})
	if s.Value != 20 {
		t.Fatalf("after left Value=%v want 20", s.Value)
	}
	s.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Home"})
	if s.Value != 0 {
		t.Fatalf("Home Value=%v want 0", s.Value)
	}
	s.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "End"})
	if s.Value != 100 {
		t.Fatalf("End Value=%v want 100", s.Value)
	}
}

func TestSlider_PRD_09_Vertical(t *testing.T) {
	// SLD-09 / SLD-S8
	s := kit.NewSlider(30)
	s.SetVertical(true)
	if !s.IsVertical() {
		t.Fatal("want vertical")
	}
	tree := layoutSlider(t, s, 80, 300)
	sz := s.Node().Base().Size()
	if sz.Height < 100 {
		t.Fatalf("vertical height=%v want tall", sz.Height)
	}
	// drag toward top → higher value
	sliderDragTo(t, tree, s, 0.9)
	if s.Value < 70 {
		t.Fatalf("vertical drag high ratio Value=%v want >=70", s.Value)
	}
}

func TestSlider_PRD_10_Tooltip(t *testing.T) {
	// SLD-10 / SLD-S9
	s := kit.NewSlider(30)
	tree := layoutSlider(t, s, 320, 48)
	if s.TooltipVisible() {
		t.Fatal("auto tooltip should be hidden at rest")
	}
	sliderPointer(t, tree, s, core.PointerDown, 0.3)
	if !s.TooltipVisible() {
		t.Fatal("tooltip should show while dragging")
	}
	txt := s.TooltipText()
	if txt == "" {
		t.Fatal("empty tip text while dragging")
	}
	sliderPointer(t, tree, s, core.PointerUp, 0.3)

	s2 := kit.NewSlider(30)
	s2.SetTooltipOpen(kit.SliderTooltipAlways)
	_ = layoutSlider(t, s2, 320, 48)
	if !s2.TooltipVisible() || s2.TooltipText() == "" {
		t.Fatalf("always: visible=%v text=%q", s2.TooltipVisible(), s2.TooltipText())
	}

	s3 := kit.NewSlider(30)
	s3.SetTooltipOpen(kit.SliderTooltipNever)
	tree3 := layoutSlider(t, s3, 320, 48)
	sliderPointer(t, tree3, s3, core.PointerDown, 0.5)
	if s3.TooltipVisible() {
		t.Fatal("never: should stay hidden")
	}
}

func TestSlider_PRD_11_OnChangeComplete(t *testing.T) {
	// SLD-11 / SLD-S10
	s := kit.NewSlider(0)
	var changes, completes int
	var lastComplete []float64
	s.SetOnChange(func(float64) { changes++ })
	s.SetOnChangeComplete(func(vals []float64) {
		completes++
		lastComplete = append([]float64(nil), vals...)
	})
	tree := layoutSlider(t, s, 320, 48)
	sliderPointer(t, tree, s, core.PointerDown, 0.4)
	sliderPointer(t, tree, s, core.PointerMove, 0.5)
	sliderPointer(t, tree, s, core.PointerMove, 0.6)
	if completes != 0 {
		t.Fatalf("complete during drag=%d", completes)
	}
	if changes == 0 {
		t.Fatal("expected onChange during drag")
	}
	sliderPointer(t, tree, s, core.PointerUp, 0.6)
	if completes != 1 {
		t.Fatalf("complete=%d want 1", completes)
	}
	if len(lastComplete) != 1 {
		t.Fatalf("complete vals=%v", lastComplete)
	}
}

func TestSlider_PRD_12_BasicDemo(t *testing.T) {
	// SLD-12 basic.tsx — defaultValue + disabled toggle + range sibling
	s := kit.NewSlider(30)
	if s.Value != 30 {
		t.Fatalf("defaultValue=%v", s.Value)
	}
	r := kit.NewSlider(0)
	r.SetRange(true)
	r.SetDefaultValues(20, 50)
	lo, hi := r.Values()
	if lo != 20 || hi != 50 {
		t.Fatalf("range default=%v,%v", lo, hi)
	}
	s.SetDisabled(true)
	n := 0
	s.SetOnChange(func(float64) { n++ })
	tree := layoutSlider(t, s, 320, 48)
	sliderClickRatio(t, tree, s, 0.8)
	if n != 0 {
		t.Fatal("disabled basic should not change")
	}
	_ = r.Node().Layout(core.Loose(320, 48))
}

func TestSlider_PRD_13_InputNumberDemo(t *testing.T) {
	// SLD-13 input-number.tsx — controlled slider + integer/decimal step
	s := kit.NewSlider(1)
	s.SetMin(1)
	s.SetMax(20)
	s.SetControlled(true)
	s.SetValue(1)
	var last float64
	s.SetOnChange(func(v float64) {
		last = v
		s.SetValue(v)
	})
	tree := layoutSlider(t, s, 320, 48)
	sliderClickRatio(t, tree, s, 0.5)
	if last < 1 || last > 20 {
		t.Fatalf("integer last=%v", last)
	}
	if s.Value != last {
		t.Fatalf("controlled value=%v last=%v", s.Value, last)
	}

	d := kit.NewSlider(0)
	d.SetMin(0)
	d.SetMax(1)
	d.SetStep(0.01)
	d.SetValue(0.5)
	if !approxSlider(d.Value, 0.5, 1e-9) {
		t.Fatalf("decimal Value=%v", d.Value)
	}
	d.SetValue(0.234)
	if !approxSlider(d.Value, 0.23, 1e-9) {
		t.Fatalf("decimal snap Value=%v want 0.23", d.Value)
	}
}

func TestSlider_PRD_14_IconSliderDemo(t *testing.T) {
	// SLD-14 icon-slider.tsx — min/max + controlled value for mid threshold
	s := kit.NewSlider(0)
	s.SetMin(0)
	s.SetMax(20)
	s.SetControlled(true)
	s.SetValue(0)
	mid := 10.0
	var value float64
	s.SetOnChange(func(v float64) {
		value = v
		s.SetValue(v)
	})
	tree := layoutSlider(t, s, 320, 48)
	sliderClickRatio(t, tree, s, 0.75)
	if value < mid {
		t.Fatalf("value=%v want >= mid for smile side", value)
	}
	if s.Value != value {
		t.Fatalf("controlled=%v value=%v", s.Value, value)
	}
}

func TestSlider_PRD_15_TipFormatterDemo(t *testing.T) {
	// SLD-15 tip-formatter.tsx
	s := kit.NewSlider(40)
	s.SetTooltipFormatter(func(v float64) string { return fmt.Sprintf("%.0f%%", v) })
	s.SetTooltipOpen(kit.SliderTooltipAlways)
	_ = layoutSlider(t, s, 320, 48)
	if s.TooltipText() != "40%" {
		t.Fatalf("formatter tip=%q want 40%%", s.TooltipText())
	}
	s2 := kit.NewSlider(40)
	s2.SetTooltipFormatterNull(true)
	s2.SetTooltipOpen(kit.SliderTooltipAlways)
	_ = layoutSlider(t, s2, 320, 48)
	if s2.TooltipVisible() || s2.TooltipText() != "" {
		t.Fatalf("formatter null still visible text=%q", s2.TooltipText())
	}
}

func TestSlider_PRD_16_EventDemo(t *testing.T) {
	// SLD-16 event.tsx — onChange + onChangeComplete both fire correctly
	s := kit.NewSlider(30)
	var log []string
	s.SetOnChange(func(v float64) { log = append(log, "c") })
	s.SetOnChangeComplete(func(vals []float64) { log = append(log, "done") })
	tree := layoutSlider(t, s, 320, 48)
	sliderDragTo(t, tree, s, 0.55)
	if len(log) < 2 || log[len(log)-1] != "done" {
		t.Fatalf("event log=%v", log)
	}
	// range sibling
	r := kit.NewSlider(0)
	r.SetRange(true)
	r.SetStep(10)
	r.SetDefaultValues(20, 50)
	var rDone int
	r.SetOnChangeComplete(func([]float64) { rDone++ })
	treeR := layoutSlider(t, r, 320, 48)
	sliderDragTo(t, treeR, r, 0.1)
	if rDone != 1 {
		t.Fatalf("range complete=%d", rDone)
	}
}

func TestSlider_PRD_17_MarksDemo(t *testing.T) {
	// SLD-17 mark.tsx
	marks := []kit.SliderMark{
		{Value: 0, Label: "0°C"},
		{Value: 26, Label: "26°C"},
		{Value: 37, Label: "37°C"},
		{Value: 100, Label: "100°C", Color: render.Hex("#f50")},
	}
	s := kit.NewSlider(37)
	s.SetMarks(marks...)
	if len(s.Marks) != 4 {
		t.Fatalf("marks=%d", len(s.Marks))
	}
	_ = layoutSlider(t, s, 320, 80)

	s2 := kit.NewSlider(37)
	s2.SetMarks(marks...)
	s2.SetIncluded(false)
	_ = layoutSlider(t, s2, 320, 80)

	s3 := kit.NewSlider(37)
	s3.SetMarks(marks...)
	s3.SetStep(10)
	s3.SetValue(37)
	// 37 with step 10 → 40
	if s3.Value != 40 {
		t.Fatalf("marks+step Value=%v want 40", s3.Value)
	}

	s4 := kit.NewSlider(37)
	s4.SetMarks(marks...)
	s4.SetStepNull(true)
	s4.SetValue(40)
	if s4.Value != 37 && s4.Value != 26 && s4.Value != 100 && s4.Value != 0 {
		t.Fatalf("step null snap Value=%v", s4.Value)
	}
}

func TestSlider_PRD_18_VerticalDemo(t *testing.T) {
	// SLD-18 vertical.tsx
	s := kit.NewSlider(30)
	s.SetVertical(true)
	_ = layoutSlider(t, s, 80, 300)

	r := kit.NewSlider(0)
	r.SetVertical(true)
	r.SetRange(true)
	r.SetStep(10)
	r.SetDefaultValues(20, 50)
	lo, hi := r.Values()
	if lo != 20 || hi != 50 {
		t.Fatalf("vert range=%v,%v", lo, hi)
	}
	_ = layoutSlider(t, r, 80, 300)

	m := kit.NewSlider(0)
	m.SetVertical(true)
	m.SetRange(true)
	m.SetMarks(
		kit.SliderMark{Value: 0, Label: "0°C"},
		kit.SliderMark{Value: 26, Label: "26°C"},
		kit.SliderMark{Value: 37, Label: "37°C"},
		kit.SliderMark{Value: 100, Label: "100°C"},
	)
	m.SetDefaultValues(26, 37)
	_ = layoutSlider(t, m, 120, 300)
}

func TestSlider_PRD_19_ShowTooltipDemo(t *testing.T) {
	// SLD-19 show-tooltip.tsx — tooltip={{ open: true }}
	s := kit.NewSlider(30)
	s.SetTooltipOpen(kit.SliderTooltipAlways)
	_ = layoutSlider(t, s, 320, 48)
	if !s.TooltipVisible() {
		t.Fatal("open:true should show tip")
	}
	if s.TooltipText() != "30" {
		t.Fatalf("tip=%q want 30", s.TooltipText())
	}
}

func TestSlider_PRD_20_Metrics(t *testing.T) {
	// SLD-20 §6.2 geometry
	s := kit.NewSlider(0)
	if !approxSlider(s.ControlSize(), kit.DefaultSliderControlSize, 0.5) {
		t.Fatalf("controlSize=%v want %v", s.ControlSize(), kit.DefaultSliderControlSize)
	}
	if !approxSlider(s.RailSize(), kit.DefaultSliderRailSize, 0.5) {
		t.Fatalf("railSize=%v", s.RailSize())
	}
	if !approxSlider(s.HandleSize(), kit.DefaultSliderHandleSize, 0.5) {
		t.Fatalf("handleSize=%v", s.HandleSize())
	}
	if !approxSlider(s.DotSize(), kit.DefaultSliderDotSize, 0.5) {
		t.Fatalf("dotSize=%v", s.DotSize())
	}
	if !approxSlider(s.HandleLineWidth(), kit.DefaultSliderHandleLineWidth, 0.5) {
		t.Fatalf("handleLine=%v", s.HandleLineWidth())
	}
	if !approxSlider(s.FocusOutset(), kit.DefaultSliderFocusOutset, 0.1) {
		t.Fatalf("focusOutset=%v", s.FocusOutset())
	}
	// theme-derived: controlHeightLG/4
	th := kit.DefaultTheme()
	want := th.SizeOr(core.TokenControlHeightLG, 40) / 4
	if !approxSlider(s.ControlSize(), want, 0.5) {
		t.Fatalf("controlSize theme=%v want %v", s.ControlSize(), want)
	}
}

func TestSlider_PRD_21_ThemeColors(t *testing.T) {
	// SLD-21 no sole hard-coded brand; theme tokens resolve
	th := kit.DefaultTheme()
	if th.Color(core.TokenColorPrimary).A < 0.5 {
		t.Fatal("default primary missing")
	}
	if th.Color(core.TokenColorPrimaryBorder).A < 0.2 {
		t.Fatal("default primaryBorder missing")
	}
	if th.Color(core.TokenColorFillSecondary).A < 0.02 {
		t.Fatal("default fillSecondary missing")
	}
	custom := kit.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	custom.Tokens.Colors[core.TokenColorPrimaryBorder] = render.Hex("#FF88CC")

	s := kit.NewSlider(50)
	s.SetTheme(custom)
	_ = layoutSlider(t, s, 320, 48)
	// getters re-resolve metrics from custom theme without hard-coded brand sole path
	if s.HandleSize() <= 0 || s.RailSize() <= 0 {
		t.Fatal("metrics zero under custom theme")
	}
	if s.Node() == nil {
		t.Fatal("nil")
	}
}

func TestSlider_PRD_22_DisabledChrome(t *testing.T) {
	// SLD-22
	s := kit.NewSlider(40)
	s.SetDisabled(true)
	_ = layoutSlider(t, s, 320, 48)
	if s.Root.Cursor != core.CursorDefault {
		t.Fatalf("disabled cursor=%v", s.Root.Cursor)
	}
	// no interaction
	n := 0
	s.SetOnChange(func(float64) { n++ })
	tree := layoutSlider(t, s, 320, 48)
	sliderClickRatio(t, tree, s, 0.9)
	if n != 0 {
		t.Fatal("disabled chrome still interactive")
	}
}

func TestSlider_PRD_23_FocusKeyboard(t *testing.T) {
	// SLD-23
	s := kit.NewSlider(10)
	s.SetStep(5)
	tree := layoutSlider(t, s, 320, 48)
	if !s.Node().(interface{ CanFocus() bool }).CanFocus() {
		t.Fatal("should be focusable")
	}
	tree.SetFocus(s.Node())
	// focus-visible path
	if fv, ok := s.Node().(interface{ SetFocusVisible(bool) }); ok {
		fv.SetFocusVisible(true)
	}
	if !s.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"}) {
		t.Fatal("key not handled when focused")
	}
	if s.Value != 15 {
		t.Fatalf("Value=%v want 15", s.Value)
	}
}

func TestSlider_PRD_ExpandWidth(t *testing.T) {
	// gallery host: Width=0 + ExpandWidth
	s := kit.NewSlider(16)
	s.SetMin(0)
	s.SetMax(64)
	s.SetWidth(0)
	host := primitive.NewDecorated(s.Node())
	host.ExpandWidth = true
	host.Height = 32
	host.StretchChild = true
	sz := host.Layout(core.Constraints{MinWidth: 280, MaxWidth: 280, MaxHeight: 40})
	if sz.Width < 100 {
		t.Fatalf("host width=%v", sz.Width)
	}
	ss := s.Node().Base().Size()
	if ss.Width < 50 {
		t.Fatalf("slider too narrow w=%v", ss.Width)
	}
}
