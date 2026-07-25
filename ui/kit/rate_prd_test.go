package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// docs/antd/rate.md §6.9 — P0 PRD cases (RAT-01 … RAT-21).
// L3/L4 (RAT-22/23) and P1 (RAT-24) deferred.

func layoutRate(t *testing.T, r *kit.Rate) *core.Tree {
	t.Helper()
	tree := core.NewTree(r.Node())
	tree.Layout(core.Size{Width: 480, Height: 80})
	return tree
}

func clickStar(t *testing.T, tree *core.Tree, r *kit.Rate, index1Based int, half bool) {
	t.Helper()
	stars := r.StarNodes()
	if index1Based < 1 || index1Based > len(stars) {
		t.Fatalf("star index %d out of range 1..%d", index1Based, len(stars))
	}
	star := stars[index1Based-1]
	tree.Layout(core.Size{Width: 480, Height: 80})
	abs := core.AbsoluteBounds(star)
	x := (abs.Min.X + abs.Max.X) / 2
	if half {
		x = abs.Min.X + (abs.Max.X-abs.Min.X)*0.25
	} else {
		x = abs.Min.X + (abs.Max.X-abs.Min.X)*0.75
	}
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func hoverStar(t *testing.T, tree *core.Tree, r *kit.Rate, index1Based int, half bool) {
	t.Helper()
	stars := r.StarNodes()
	if index1Based < 1 || index1Based > len(stars) {
		t.Fatalf("star index %d out of range 1..%d", index1Based, len(stars))
	}
	star := stars[index1Based-1]
	tree.Layout(core.Size{Width: 480, Height: 80})
	abs := core.AbsoluteBounds(star)
	x := abs.Min.X + (abs.Max.X-abs.Min.X)*0.75
	if half {
		x = abs.Min.X + (abs.Max.X-abs.Min.X)*0.25
	}
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: x, Y: y})
}

func approxRate(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func approxRateColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(float64(a.R)-float64(b.R)) <= tol &&
		math.Abs(float64(a.G)-float64(b.G)) <= tol &&
		math.Abs(float64(a.B)-float64(b.B)) <= tol &&
		math.Abs(float64(a.A)-float64(b.A)) <= tol
}

func TestRate_PRD_01_Defaults(t *testing.T) {
	// RAT-01
	r := kit.NewRate()
	if r.Value != 0 {
		t.Fatalf("Value=%v want 0", r.Value)
	}
	if r.Count != kit.DefaultRateCount {
		t.Fatalf("Count=%d want %d", r.Count, kit.DefaultRateCount)
	}
	if !r.AllowClear {
		t.Fatal("AllowClear default want true")
	}
	if r.AllowHalf {
		t.Fatal("AllowHalf default want false")
	}
	if r.Disabled || r.Controlled {
		t.Fatalf("flags want false: disabled=%v controlled=%v", r.Disabled, r.Controlled)
	}
	if !r.Keyboard {
		t.Fatal("Keyboard default want true")
	}
	if r.Size != kit.RateMiddle {
		t.Fatalf("Size=%v want middle", r.Size)
	}
	if r.Node() == nil || r.Root == nil {
		t.Fatal("nil node")
	}
	if r.Root.Base().Role != "radiogroup" {
		t.Fatalf("role=%q want radiogroup", r.Root.Base().Role)
	}
	if len(r.StarNodes()) != 5 {
		t.Fatalf("stars=%d want 5", len(r.StarNodes()))
	}
}

func TestRate_PRD_02_ClickThirdStar(t *testing.T) {
	// RAT-02 / RAT-S1
	r := kit.NewRate()
	var got []float64
	r.SetOnChange(func(v float64) { got = append(got, v) })
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 3, false)
	if r.Value != 3 {
		t.Fatalf("Value=%v want 3", r.Value)
	}
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("onChange=%v want [3]", got)
	}
}

func TestRate_PRD_03_AllowHalf(t *testing.T) {
	// RAT-03 / RAT-S2
	r := kit.NewRate()
	r.SetAllowHalf(true)
	var got float64 = -1
	r.SetOnChange(func(v float64) { got = v })
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 3, true)
	if r.Value != 2.5 {
		t.Fatalf("Value=%v want 2.5", r.Value)
	}
	if got != 2.5 {
		t.Fatalf("onChange=%v want 2.5", got)
	}
}

func TestRate_PRD_04_AllowClear(t *testing.T) {
	// RAT-04 / RAT-S3
	r := kit.NewRate()
	r.SetDefaultValue(3)
	var got []float64
	r.SetOnChange(func(v float64) { got = append(got, v) })
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 3, false)
	if r.Value != 0 {
		t.Fatalf("Value=%v want 0 after re-click", r.Value)
	}
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("onChange=%v want [0]", got)
	}
	// allowClear=false keeps value
	r2 := kit.NewRate()
	r2.SetAllowClear(false)
	r2.SetDefaultValue(3)
	n := 0
	r2.SetOnChange(func(float64) { n++ })
	tree2 := layoutRate(t, r2)
	clickStar(t, tree2, r2, 3, false)
	if r2.Value != 3 || n != 0 {
		t.Fatalf("allowClear=false: value=%v n=%d", r2.Value, n)
	}
}

func TestRate_PRD_05_Disabled(t *testing.T) {
	// RAT-05 / RAT-S4
	r := kit.NewRate()
	r.SetDefaultValue(2)
	r.SetDisabled(true)
	n := 0
	r.SetOnChange(func(float64) { n++ })
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 4, false)
	if n != 0 || r.Value != 2 {
		t.Fatalf("disabled: n=%d value=%v", n, r.Value)
	}
}

func TestRate_PRD_06_Count10(t *testing.T) {
	// RAT-06 / RAT-S5
	r := kit.NewRate()
	r.SetCount(10)
	if r.Count != 10 {
		t.Fatalf("Count=%d", r.Count)
	}
	if len(r.StarNodes()) != 10 {
		t.Fatalf("stars=%d want 10", len(r.StarNodes()))
	}
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 10, false)
	if r.Value != 10 {
		t.Fatalf("Value=%v want 10", r.Value)
	}
}

func TestRate_PRD_07_Keyboard(t *testing.T) {
	// RAT-07 / RAT-S6
	r := kit.NewRate()
	r.SetDefaultValue(2)
	tree := layoutRate(t, r)
	// Focus first star then arrow right.
	stars := r.StarNodes()
	tree.SetFocus(stars[0])
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"})
	if r.Value != 3 {
		t.Fatalf("after ArrowRight Value=%v want 3", r.Value)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowLeft"})
	if r.Value != 2 {
		t.Fatalf("after ArrowLeft Value=%v want 2", r.Value)
	}
	r.SetAllowHalf(true)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"})
	if r.Value != 2.5 {
		t.Fatalf("half keyboard Value=%v want 2.5", r.Value)
	}
	// keyboard=false ignores
	r.SetKeyboard(false)
	prev := r.Value
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"})
	if r.Value != prev {
		t.Fatalf("keyboard=false changed value %v→%v", prev, r.Value)
	}
}

func TestRate_PRD_08_Tooltips(t *testing.T) {
	// RAT-08 / RAT-S7
	r := kit.NewRate()
	r.SetTooltips([]string{"terrible", "bad", "normal", "good", "wonderful"})
	if r.TooltipAt(2) != "normal" {
		t.Fatalf("TooltipAt(2)=%q", r.TooltipAt(2))
	}
	tree := layoutRate(t, r)
	hoverStar(t, tree, r, 3, false)
	if tip := r.HoverTooltip(); tip != "normal" {
		t.Fatalf("HoverTooltip=%q want normal", tip)
	}
}

func TestRate_PRD_09_Controlled(t *testing.T) {
	// RAT-09 / RAT-S8
	r := kit.NewRate()
	r.SetControlled(true)
	r.SetValue(1)
	var got []float64
	r.SetOnChange(func(v float64) { got = append(got, v) })
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 4, false)
	if r.Value != 1 {
		t.Fatalf("controlled local Value=%v want 1 until parent sets", r.Value)
	}
	if len(got) != 1 || got[0] != 4 {
		t.Fatalf("onChange=%v want [4]", got)
	}
	r.SetValue(4)
	if r.Value != 4 {
		t.Fatal("parent SetValue(4) should stick")
	}
}

func TestRate_PRD_10_BasicDemo(t *testing.T) {
	// RAT-10 basic.tsx
	r := kit.NewRate()
	if r.Value != 0 || r.Count != 5 {
		t.Fatalf("basic defaults value=%v count=%d", r.Value, r.Count)
	}
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 4, false)
	if r.Value != 4 {
		t.Fatalf("basic click Value=%v", r.Value)
	}
}

func TestRate_PRD_11_SizeDemo(t *testing.T) {
	// RAT-11 size.tsx
	lg := kit.NewRate()
	lg.SetSize(kit.RateLarge)
	md := kit.NewRate()
	sm := kit.NewRate()
	sm.SetSize(kit.RateSmall)
	if !approxRate(lg.StarSize(), kit.DefaultRateStarSizeLG, 0.5) {
		t.Fatalf("large starSize=%v want %v", lg.StarSize(), kit.DefaultRateStarSizeLG)
	}
	if !approxRate(md.StarSize(), kit.DefaultRateStarSize, 0.5) {
		t.Fatalf("middle starSize=%v want %v", md.StarSize(), kit.DefaultRateStarSize)
	}
	if !approxRate(sm.StarSize(), kit.DefaultRateStarSizeSM, 0.5) {
		t.Fatalf("small starSize=%v want %v", sm.StarSize(), kit.DefaultRateStarSizeSM)
	}
}

func TestRate_PRD_12_HalfDemo(t *testing.T) {
	// RAT-12 half.tsx
	r := kit.NewRate()
	r.SetAllowHalf(true)
	r.SetDefaultValue(2.5)
	if r.Value != 2.5 {
		t.Fatalf("defaultValue half=%v", r.Value)
	}
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 4, true)
	if r.Value != 3.5 {
		t.Fatalf("half click Value=%v want 3.5", r.Value)
	}
}

func TestRate_PRD_13_TextDemo(t *testing.T) {
	// RAT-13 text.tsx — tooltips + controlled value + label lookup
	desc := []string{"terrible", "bad", "normal", "good", "wonderful"}
	r := kit.NewRate()
	r.SetTooltips(desc)
	r.SetControlled(true)
	r.SetValue(3)
	r.SetOnChange(func(v float64) { r.SetValue(v) })
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 5, false)
	if r.Value != 5 {
		t.Fatalf("text demo Value=%v", r.Value)
	}
	if desc[int(r.Value)-1] != "wonderful" {
		t.Fatal("label mapping failed")
	}
}

func TestRate_PRD_14_DisabledDemo(t *testing.T) {
	// RAT-14 disabled.tsx
	r := kit.NewRate()
	r.SetDefaultValue(2)
	r.SetDisabled(true)
	if r.Value != 2 || !r.Disabled {
		t.Fatalf("disabled demo value=%v disabled=%v", r.Value, r.Disabled)
	}
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 5, false)
	if r.Value != 2 {
		t.Fatal("disabled should not change")
	}
}

func TestRate_PRD_15_ClearDemo(t *testing.T) {
	// RAT-15 clear.tsx
	a := kit.NewRate()
	a.SetDefaultValue(3)
	b := kit.NewRate()
	b.SetDefaultValue(3)
	b.SetAllowClear(false)
	treeA := layoutRate(t, a)
	treeB := layoutRate(t, b)
	clickStar(t, treeA, a, 3, false)
	clickStar(t, treeB, b, 3, false)
	if a.Value != 0 {
		t.Fatalf("allowClear true → %v", a.Value)
	}
	if b.Value != 3 {
		t.Fatalf("allowClear false → %v", b.Value)
	}
}

func TestRate_PRD_16_CharacterDemo(t *testing.T) {
	// RAT-16 character.tsx — string characters
	r := kit.NewRate()
	r.SetCharacter("A")
	r.SetAllowHalf(true)
	if len(r.StarNodes()) != 5 {
		t.Fatal("character stars missing")
	}
	tree := layoutRate(t, r)
	clickStar(t, tree, r, 2, true)
	if r.Value != 1.5 {
		t.Fatalf("character half Value=%v", r.Value)
	}
	r.SetCharacter("好")
	if r.Node() == nil {
		t.Fatal("nil after character change")
	}
}

func TestRate_PRD_17_CharacterFunctionDemo(t *testing.T) {
	// RAT-17 character-function.tsx
	r := kit.NewRate()
	r.SetDefaultValue(2)
	r.SetCharacterAt(func(index int) string {
		return string(rune('1' + index))
	})
	if len(r.StarNodes()) != 5 {
		t.Fatal("fn stars missing")
	}
	// Root identity across SetValue
	tree := layoutRate(t, r)
	root0 := r.Root
	r.SetValue(4)
	if r.Root != root0 {
		t.Fatal("SetValue replaced Root")
	}
	_ = tree
}

func TestRate_PRD_18_Metrics(t *testing.T) {
	// RAT-18 L2 §6.2
	r := kit.NewRate()
	if !approxRate(r.StarSize(), kit.DefaultRateStarSize, 0.5) {
		t.Fatalf("middle size=%v", r.StarSize())
	}
	if !approxRate(r.StarGap(), kit.DefaultRateStarGap, 0.5) {
		t.Fatalf("gap=%v want %v", r.StarGap(), kit.DefaultRateStarGap)
	}
	// Layout geometry: stars hug content; measure first→last star span
	// (Root as tree viewport root expands to 480 — not product chrome width).
	tree := layoutRate(t, r)
	_ = tree
	stars := r.StarNodes()
	if len(stars) != 5 {
		t.Fatalf("stars=%d", len(stars))
	}
	for i, s := range stars {
		sz := s.Base().Size()
		if !approxRate(sz.Width, kit.DefaultRateStarSize, 0.5) || !approxRate(sz.Height, kit.DefaultRateStarSize, 0.5) {
			t.Fatalf("star[%d] size=%v want %v²", i, sz, kit.DefaultRateStarSize)
		}
	}
	first := core.AbsoluteBounds(stars[0])
	last := core.AbsoluteBounds(stars[4])
	wantW := 5*kit.DefaultRateStarSize + 4*kit.DefaultRateStarGap
	gotW := last.Max.X - first.Min.X
	if !approxRate(gotW, wantW, 1.0) {
		t.Fatalf("star span width=%v want ~%v", gotW, wantW)
	}
	gotH := first.Max.Y - first.Min.Y
	if !approxRate(gotH, kit.DefaultRateStarSize, 1.0) {
		t.Fatalf("star height=%v want ~%v", gotH, kit.DefaultRateStarSize)
	}
}

func TestRate_PRD_19_TokenColors(t *testing.T) {
	// RAT-19 L2 — no hard-coded brand-only path; Style override works
	r := kit.NewRate()
	fill := r.StarFillColor()
	want := render.Hex(kit.DefaultRateStarColor)
	if !approxRateColor(fill, want, 0.05) {
		t.Fatalf("fill=%v want yellow6 %v", fill, want)
	}
	empty := r.StarEmptyColor()
	if empty.A < 0.02 {
		t.Fatalf("empty alpha too low: %v", empty)
	}
	// Style.Text overrides star color
	r.SetStyle(kit.Style{Text: render.Hex("#FF0000")})
	if !approxRateColor(r.StarFillColor(), render.Hex("#FF0000"), 0.05) {
		t.Fatalf("style override fill=%v", r.StarFillColor())
	}
}

func TestRate_PRD_20_DisabledAppearance(t *testing.T) {
	// RAT-20 L2
	r := kit.NewRate()
	r.SetDefaultValue(3)
	base := r.StarFillColor()
	r.SetDisabled(true)
	dis := r.StarFillColor()
	// Disabled fill should differ (lower contrast) from active gold.
	if approxRateColor(base, dis, 0.01) {
		t.Fatalf("disabled fill should differ from active: base=%v dis=%v", base, dis)
	}
	// Hover preview must not apply while disabled
	tree := layoutRate(t, r)
	hoverStar(t, tree, r, 5, false)
	if r.HoverValue() != 0 {
		t.Fatalf("disabled hover=%v want 0", r.HoverValue())
	}
}

func TestRate_PRD_21_FocusKeyboard(t *testing.T) {
	// RAT-21 L1 a11y
	r := kit.NewRate()
	tree := layoutRate(t, r)
	stars := r.StarNodes()
	if stars[0].Base().Role != "radio" {
		t.Fatalf("star role=%q", stars[0].Base().Role)
	}
	tree.SetFocus(stars[0])
	if tree.Focus() != stars[0] {
		t.Fatal("focus not set")
	}
	// Space/Enter not required for Rate; arrows adjust (RAT-07).
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "End"})
	if r.Value != 5 {
		t.Fatalf("End key Value=%v want 5", r.Value)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Home"})
	if r.Value != 0 {
		t.Fatalf("Home key Value=%v want 0", r.Value)
	}
}

func TestRate_PRD_RootIdentity(t *testing.T) {
	r := kit.NewRate()
	r.SetDefaultValue(2)
	tree := layoutRate(t, r)
	root0 := r.Root
	r.SetValue(4)
	if r.Root != root0 {
		t.Fatal("SetValue replaced Root")
	}
	r.SetCount(7)
	// Count rebuilds children but Root Flex identity stays.
	if r.Root != root0 {
		t.Fatal("SetCount replaced Root")
	}
	if len(r.StarNodes()) != 7 {
		t.Fatalf("stars=%d", len(r.StarNodes()))
	}
	_ = tree
}

func TestRate_PRD_HoverChange(t *testing.T) {
	r := kit.NewRate()
	var last float64 = -1
	r.SetOnHoverChange(func(v float64) { last = v })
	tree := layoutRate(t, r)
	hoverStar(t, tree, r, 4, false)
	if last != 4 {
		t.Fatalf("hoverChange=%v want 4", last)
	}
	// Leave Rate
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: 400, Y: 400})
	if last != 0 {
		t.Fatalf("leave hoverChange=%v want 0", last)
	}
}
