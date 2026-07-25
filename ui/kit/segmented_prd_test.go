package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/segmented.md §6.9 — P0 PRD cases (SEG-01 … SEG-20).
// L3/L4 (SEG-21/22) and P1 (SEG-23) deferred.

func layoutSeg(t *testing.T, s *kit.Segmented, w, h float64) *core.Tree {
	t.Helper()
	if w <= 0 {
		w = 480
	}
	if h <= 0 {
		h = 120
	}
	tree := core.NewTree(s.Node())
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func clickSegOption(t *testing.T, tree *core.Tree, s *kit.Segmented, index int) {
	t.Helper()
	nodes := s.OptionNodes()
	if index < 0 || index >= len(nodes) || nodes[index] == nil {
		t.Fatalf("option index %d out of range 0..%d", index, len(nodes)-1)
	}
	tree.Layout(core.Size{Width: 480, Height: 120})
	abs := core.AbsoluteBounds(nodes[index])
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func approxSeg(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func approxSegColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(float64(a.R)-float64(b.R)) <= tol &&
		math.Abs(float64(a.G)-float64(b.G)) <= tol &&
		math.Abs(float64(a.B)-float64(b.B)) <= tol &&
		math.Abs(float64(a.A)-float64(b.A)) <= tol
}

func TestSegmented_PRD_01_Defaults(t *testing.T) {
	// SEG-01
	s := kit.NewSegmented("Daily", "Weekly", "Monthly")
	if s.Value != "Daily" {
		t.Fatalf("Value=%q want first option Daily", s.Value)
	}
	if s.Size != kit.SegmentedMiddle {
		t.Fatalf("Size=%v want middle", s.Size)
	}
	if s.Orientation != kit.SegmentedHorizontal {
		t.Fatalf("Orientation=%v want horizontal", s.Orientation)
	}
	if s.Shape != kit.SegmentedShapeDefault {
		t.Fatalf("Shape=%v want default", s.Shape)
	}
	if s.Disabled || s.Block || s.Controlled {
		t.Fatalf("flags want false: disabled=%v block=%v controlled=%v",
			s.Disabled, s.Block, s.Controlled)
	}
	if s.Node() == nil || s.Root == nil {
		t.Fatal("nil node")
	}
	if s.Root.Base().Role != "radiogroup" {
		t.Fatalf("role=%q want radiogroup", s.Root.Base().Role)
	}
	if len(s.OptionNodes()) != 3 {
		t.Fatalf("options=%d want 3", len(s.OptionNodes()))
	}
	if s.SelectedIndex() != 0 {
		t.Fatalf("SelectedIndex=%d want 0", s.SelectedIndex())
	}
}

func TestSegmented_PRD_02_ChangeOption(t *testing.T) {
	// SEG-02 / SEG-S1
	s := kit.NewSegmented("A", "B", "C")
	var got []string
	s.SetOnChange(func(v string) { got = append(got, v) })
	tree := layoutSeg(t, s, 400, 80)
	clickSegOption(t, tree, s, 1)
	if s.Value != "B" {
		t.Fatalf("Value=%q want B", s.Value)
	}
	if len(got) != 1 || got[0] != "B" {
		t.Fatalf("onChange=%v want [B]", got)
	}
	// re-click selected → no repeated onChange
	clickSegOption(t, tree, s, 1)
	if len(got) != 1 {
		t.Fatalf("re-click onChange=%v want still [B]", got)
	}
}

func TestSegmented_PRD_03_Block(t *testing.T) {
	// SEG-03 / SEG-S2
	s := kit.NewSegmented("123", "456", "longtext")
	s.SetBlock(true)
	host := primitive.NewDecorated(s.Node())
	host.Width, host.Height = 360, 48
	host.StretchChild = true
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 360, Height: 48})
	rootW := s.Root.Size().Width
	if rootW < 350 {
		t.Fatalf("block track width=%v want ~360", rootW)
	}
	nodes := s.OptionNodes()
	if len(nodes) != 3 {
		t.Fatalf("opts=%d", len(nodes))
	}
	// equal-ish share of width
	w0 := nodes[0].Base().Size().Width
	w1 := nodes[1].Base().Size().Width
	w2 := nodes[2].Base().Size().Width
	if !approxSeg(w0, w1, 2) || !approxSeg(w1, w2, 2) {
		t.Fatalf("block item widths not equal: %v %v %v", w0, w1, w2)
	}
}

func TestSegmented_PRD_04_DisabledOption(t *testing.T) {
	// SEG-04 / SEG-S3
	s := kit.NewSegmentedOptions(
		kit.SegmentedOption{Label: "Daily", Value: "Daily"},
		kit.SegmentedOption{Label: "Weekly", Value: "Weekly", Disabled: true},
		kit.SegmentedOption{Label: "Monthly", Value: "Monthly"},
	)
	n := 0
	s.SetOnChange(func(string) { n++ })
	tree := layoutSeg(t, s, 400, 80)
	clickSegOption(t, tree, s, 1)
	if n != 0 || s.Value != "Daily" {
		t.Fatalf("disabled option: n=%d value=%q", n, s.Value)
	}
	// whole disabled
	s2 := kit.NewSegmented("A", "B")
	s2.SetDisabled(true)
	n2 := 0
	s2.SetOnChange(func(string) { n2++ })
	tree2 := layoutSeg(t, s2, 300, 80)
	clickSegOption(t, tree2, s2, 1)
	if n2 != 0 || s2.Value != "A" {
		t.Fatalf("whole disabled: n=%d value=%q", n2, s2.Value)
	}
}

func TestSegmented_PRD_05_SizeHeights(t *testing.T) {
	// SEG-05 / SEG-S4
	s := kit.NewSegmented("OK")
	for _, tc := range []struct {
		size kit.SegmentedSize
		want float64
	}{
		{kit.SegmentedSmall, 24},
		{kit.SegmentedMiddle, 32},
		{kit.SegmentedLarge, 40},
	} {
		s.SetSize(tc.size)
		if !approxSeg(s.ControlHeight(), tc.want, 0.5) {
			t.Fatalf("size %v controlH=%v want %v", tc.size, s.ControlHeight(), tc.want)
		}
		sz := s.Node().Layout(core.Loose(400, 100))
		if sz.Height < tc.want-0.5 || sz.Height > tc.want+1.5 {
			t.Fatalf("size %v layout height=%v want ~%v", tc.size, sz.Height, tc.want)
		}
	}
}

func TestSegmented_PRD_06_Controlled(t *testing.T) {
	// SEG-06 / SEG-S5
	s := kit.NewSegmented("Map", "Transit", "Satellite")
	s.SetControlled(true)
	s.SetValue("Map")
	var got string
	s.SetOnChange(func(v string) { got = v })
	tree := layoutSeg(t, s, 400, 80)
	clickSegOption(t, tree, s, 2)
	if got != "Satellite" {
		t.Fatalf("onChange=%q want Satellite", got)
	}
	if s.Value != "Map" {
		t.Fatalf("controlled Value=%q want Map until parent SetValue", s.Value)
	}
	s.SetValue("Satellite")
	if s.Value != "Satellite" {
		t.Fatalf("after SetValue Value=%q", s.Value)
	}
	// programmatic SetValue must not fire OnChange
	got = ""
	s.SetValue("Transit")
	if got != "" {
		t.Fatalf("SetValue fired OnChange=%q", got)
	}
}

func TestSegmented_PRD_07_IconOnly(t *testing.T) {
	// SEG-07 / SEG-S6
	s := kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "List", Icon: "check", AriaLabel: "List"},
		kit.SegmentedOption{Value: "Kanban", Icon: "user", AriaLabel: "Kanban"},
	)
	if s.Value != "List" {
		t.Fatalf("Value=%q want List", s.Value)
	}
	nodes := s.OptionNodes()
	if len(nodes) != 2 {
		t.Fatalf("opts=%d", len(nodes))
	}
	if nodes[0].Base().Label != "List" {
		t.Fatalf("a11y name=%q want List", nodes[0].Base().Label)
	}
	sz := s.Node().Layout(core.Loose(200, 40))
	if sz.Width < 8 || sz.Height < 20 {
		t.Fatalf("icon-only layout=%v", sz)
	}
	var got string
	s.SetOnChange(func(v string) { got = v })
	tree := layoutSeg(t, s, 240, 80)
	clickSegOption(t, tree, s, 1)
	if got != "Kanban" || s.Value != "Kanban" {
		t.Fatalf("got=%q value=%q", got, s.Value)
	}
}

func TestSegmented_PRD_08_Keyboard(t *testing.T) {
	// SEG-08 / SEG-S7
	s := kit.NewSegmented("A", "B", "C")
	var got []string
	s.SetOnChange(func(v string) { got = append(got, v) })
	tree := layoutSeg(t, s, 400, 80)
	// Focus first option then arrow.
	nodes := s.OptionNodes()
	tree.SetFocus(nodes[0])
	if !s.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"}) {
		t.Fatal("ArrowRight not handled")
	}
	if s.Value != "B" {
		t.Fatalf("Value=%q want B after ArrowRight", s.Value)
	}
	if len(got) != 1 || got[0] != "B" {
		t.Fatalf("onChange=%v", got)
	}
	// Enter on focused item still works via Pressable.
	tree.SetFocus(nodes[2])
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if s.Value != "C" {
		t.Fatalf("Enter Value=%q want C", s.Value)
	}
}

func TestSegmented_PRD_09_BasicDemo(t *testing.T) {
	// SEG-09 basic.tsx
	s := kit.NewSegmented("Daily", "Weekly", "Monthly", "Quarterly", "Yearly")
	var got string
	s.SetOnChange(func(v string) { got = v })
	tree := layoutSeg(t, s, 560, 80)
	clickSegOption(t, tree, s, 3)
	if got != "Quarterly" || s.Value != "Quarterly" {
		t.Fatalf("got=%q value=%q", got, s.Value)
	}
}

func TestSegmented_PRD_10_VerticalDemo(t *testing.T) {
	// SEG-10 vertical.tsx
	s := kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "List", Icon: "check", AriaLabel: "List"},
		kit.SegmentedOption{Value: "Kanban", Icon: "user", AriaLabel: "Kanban"},
	)
	s.SetOrientation(kit.SegmentedVertical)
	sz := s.Node().Layout(core.Loose(200, 200))
	if sz.Height < 40 {
		t.Fatalf("vertical height=%v too small", sz.Height)
	}
	nodes := s.OptionNodes()
	_ = layoutSeg(t, s, 200, 200)
	y0 := core.AbsoluteBounds(nodes[0]).Min.Y
	y1 := core.AbsoluteBounds(nodes[1]).Min.Y
	if y1 <= y0 {
		t.Fatalf("vertical items y0=%v y1=%v want stacked", y0, y1)
	}
	// vertical sugar
	s2 := kit.NewSegmented("A", "B")
	s2.SetVertical(true)
	_ = s2.Node().Layout(core.Loose(100, 100))
	if s2.Orientation != kit.SegmentedVertical {
		t.Fatalf("SetVertical Orientation=%v", s2.Orientation)
	}
}

func TestSegmented_PRD_11_BlockDemo(t *testing.T) {
	// SEG-11 block.tsx
	s := kit.NewSegmentedOptions(
		kit.SegmentedOption{Label: "123", Value: "123"},
		kit.SegmentedOption{Label: "456", Value: "456"},
		kit.SegmentedOption{Label: "longtext-longtext-longtext-longtext", Value: "long"},
	)
	s.SetBlock(true)
	host := primitive.NewDecorated(s.Node())
	host.Width = 400
	host.Height = 48
	host.StretchChild = true
	_ = host.Layout(core.Loose(400, 48))
	if s.Root.Size().Width < 390 {
		t.Fatalf("block demo width=%v", s.Root.Size().Width)
	}
}

func TestSegmented_PRD_12_ShapeRound(t *testing.T) {
	// SEG-12 shape.tsx
	s := kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "light", Icon: "star", AriaLabel: "light"},
		kit.SegmentedOption{Value: "dark", Icon: "heart", AriaLabel: "dark"},
	)
	s.SetShape(kit.SegmentedShapeRound)
	_ = s.Node().Layout(core.Loose(200, 40))
	if s.TrackRadius() < 100 {
		t.Fatalf("round trackR=%v want ~9999", s.TrackRadius())
	}
	if s.ItemRadius() < 100 {
		t.Fatalf("round itemR=%v want ~9999", s.ItemRadius())
	}
	dec := s.ChromeNode().(*primitive.Decorated)
	if dec.Radius < 100 {
		t.Fatalf("chrome radius=%v", dec.Radius)
	}
}

func TestSegmented_PRD_13_DisabledDemo(t *testing.T) {
	// SEG-13 disabled.tsx
	all := kit.NewSegmented("Map", "Transit", "Satellite")
	all.SetDisabled(true)
	n := 0
	all.SetOnChange(func(string) { n++ })
	tree := layoutSeg(t, all, 400, 80)
	clickSegOption(t, tree, all, 1)
	if n != 0 {
		t.Fatal("disabled whole still fires")
	}

	mixed := kit.NewSegmentedOptions(
		kit.SegmentedOption{Label: "Daily", Value: "Daily"},
		kit.SegmentedOption{Label: "Weekly", Value: "Weekly", Disabled: true},
		kit.SegmentedOption{Label: "Monthly", Value: "Monthly"},
		kit.SegmentedOption{Label: "Quarterly", Value: "Quarterly", Disabled: true},
		kit.SegmentedOption{Label: "Yearly", Value: "Yearly"},
	)
	tree2 := layoutSeg(t, mixed, 560, 80)
	clickSegOption(t, tree2, mixed, 2)
	if mixed.Value != "Monthly" {
		t.Fatalf("value=%q want Monthly", mixed.Value)
	}
	clickSegOption(t, tree2, mixed, 1)
	if mixed.Value != "Monthly" {
		t.Fatalf("disabled weekly stole value=%q", mixed.Value)
	}
}

func TestSegmented_PRD_14_ControlledDemo(t *testing.T) {
	// SEG-14 controlled.tsx
	s := kit.NewSegmented("Map", "Transit", "Satellite")
	s.SetControlled(true)
	s.SetValue("Map")
	s.SetOnChange(func(v string) { s.SetValue(v) })
	tree := layoutSeg(t, s, 400, 80)
	clickSegOption(t, tree, s, 1)
	if s.Value != "Transit" {
		t.Fatalf("controlled parent sync Value=%q", s.Value)
	}
}

func TestSegmented_PRD_15_CustomRender(t *testing.T) {
	// SEG-15 custom.tsx — LabelNode custom content
	mk := func(title, sub string) core.Node {
		col := primitive.Column(
			primitive.NewText(title),
			primitive.NewText(sub),
		)
		col.Gap = 2
		col.CrossAlign = core.CrossCenter
		return col
	}
	s := kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "spring", LabelNode: mk("Spring", "Jan-Mar")},
		kit.SegmentedOption{Value: "summer", LabelNode: mk("Summer", "Apr-Jun")},
		kit.SegmentedOption{Value: "autumn", LabelNode: mk("Autumn", "Jul-Sept")},
		kit.SegmentedOption{Value: "winter", LabelNode: mk("Winter", "Oct-Dec")},
	)
	if s.Value != "spring" {
		t.Fatalf("Value=%q", s.Value)
	}
	tree := layoutSeg(t, s, 480, 120)
	clickSegOption(t, tree, s, 2)
	if s.Value != "autumn" {
		t.Fatalf("Value=%q want autumn", s.Value)
	}
	// custom nodes present under options
	if len(s.OptionNodes()) != 4 {
		t.Fatal("missing custom options")
	}
}

func TestSegmented_PRD_16_DynamicOptions(t *testing.T) {
	// SEG-16 dynamic.tsx
	s := kit.NewSegmented("Daily", "Weekly", "Monthly")
	root0 := s.Root
	s.SetOptionsStrings("Daily", "Weekly", "Monthly", "Quarterly", "Yearly")
	if len(s.Options) != 5 {
		t.Fatalf("options=%d want 5", len(s.Options))
	}
	if len(s.OptionNodes()) != 5 {
		t.Fatalf("nodes=%d want 5", len(s.OptionNodes()))
	}
	// Root identity may stay (rebuild reuses Root)
	if s.Root != root0 {
		t.Fatal("SetOptions replaced Root identity")
	}
	// Value still valid
	if s.Value != "Daily" {
		t.Fatalf("Value=%q after grow", s.Value)
	}
	tree := layoutSeg(t, s, 560, 80)
	clickSegOption(t, tree, s, 4)
	if s.Value != "Yearly" {
		t.Fatalf("Value=%q want Yearly", s.Value)
	}
}

func TestSegmented_PRD_17_TokenMetrics(t *testing.T) {
	// SEG-17 §6.2
	s := kit.NewSegmented("A", "B")
	if !approxSeg(s.ControlHeight(), 32, 0.5) {
		t.Fatalf("middle H=%v want 32", s.ControlHeight())
	}
	if !approxSeg(s.TrackPadding(), 2, 0.5) {
		t.Fatalf("trackPad=%v want 2", s.TrackPadding())
	}
	if !approxSeg(s.ItemMinHeight(), 28, 0.5) {
		t.Fatalf("itemMinH=%v want 28", s.ItemMinHeight())
	}
	if !approxSeg(s.ItemPadH(), 11, 0.5) {
		t.Fatalf("itemPadH=%v want 11", s.ItemPadH())
	}
	if !approxSeg(s.TrackRadius(), 6, 0.5) {
		t.Fatalf("trackR=%v want 6", s.TrackRadius())
	}
	if !approxSeg(s.ItemRadius(), 4, 0.5) {
		t.Fatalf("itemR=%v want 4", s.ItemRadius())
	}
	s.SetSize(kit.SegmentedSmall)
	if !approxSeg(s.ControlHeight(), 24, 0.5) || !approxSeg(s.ItemPadH(), 7, 0.5) {
		t.Fatalf("small H=%v padH=%v", s.ControlHeight(), s.ItemPadH())
	}
	s.SetSize(kit.SegmentedLarge)
	if !approxSeg(s.ControlHeight(), 40, 0.5) {
		t.Fatalf("large H=%v", s.ControlHeight())
	}
}

func TestSegmented_PRD_18_ThemeTokensNoHardBrand(t *testing.T) {
	// SEG-18
	th := kit.DefaultTheme()
	s := kit.NewSegmented("A", "B")
	s.SetTheme(th)
	_ = s.Node().Layout(core.Loose(200, 40))
	if !approxSegColor(s.TrackBG(), th.Color(core.TokenColorBgLayout), 0.02) {
		t.Fatalf("trackBG=%v want TokenColorBgLayout %v", s.TrackBG(), th.Color(core.TokenColorBgLayout))
	}
	if !approxSegColor(s.SelectedBG(), th.Color(core.TokenColorBgContainer), 0.02) {
		t.Fatalf("selectedBG=%v want container %v", s.SelectedBG(), th.Color(core.TokenColorBgContainer))
	}
	// custom theme override
	custom := kit.DefaultTheme()
	if custom.Tokens == nil {
		t.Fatal("nil tokens")
	}
	custom.Tokens.Colors[core.TokenColorBgLayout] = render.Hex("#112233")
	s2 := kit.NewSegmented("X", "Y")
	s2.SetTheme(custom)
	if !approxSegColor(s2.TrackBG(), render.Hex("#112233"), 0.02) {
		t.Fatalf("custom trackBG=%v", s2.TrackBG())
	}
	// not solid primary brand as track
	primary := th.Color(core.TokenColorPrimary)
	if approxSegColor(s.TrackBG(), primary, 0.05) {
		t.Fatalf("track should not be primary brand %v", s.TrackBG())
	}
}

func TestSegmented_PRD_19_DisabledAppearance(t *testing.T) {
	// SEG-19
	s := kit.NewSegmented("A", "B")
	s.SetDisabled(true)
	_ = s.Node().Layout(core.Loose(200, 40))
	nodes := s.OptionNodes()
	p := nodes[0].(*primitive.Pressable)
	if !p.State.Disabled {
		t.Fatal("pressable not disabled")
	}
	// hover should not change selected value or fire
	n := 0
	s.SetOnChange(func(string) { n++ })
	tree := layoutSeg(t, s, 200, 80)
	abs := core.AbsoluteBounds(nodes[1])
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: (abs.Min.X + abs.Max.X) / 2, Y: (abs.Min.Y + abs.Max.Y) / 2})
	clickSegOption(t, tree, s, 1)
	if n != 0 || s.Value != "A" {
		t.Fatalf("disabled interaction n=%d value=%q", n, s.Value)
	}
}

func TestSegmented_PRD_20_FocusRingKeyboard(t *testing.T) {
	// SEG-20
	s := kit.NewSegmented("A", "B", "C")
	tree := layoutSeg(t, s, 400, 80)
	nodes := s.OptionNodes()
	p := nodes[0].(*primitive.Pressable)
	if !p.ShowFocusRing {
		t.Fatal("ShowFocusRing want true")
	}
	tree.SetFocus(nodes[0])
	if !p.ShowFocusRing {
		t.Fatal("focus ring chrome missing")
	}
	// Space activates focused pressable
	var got string
	s.SetOnChange(func(v string) { got = v })
	tree.SetFocus(nodes[2])
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if s.Value != "C" && got != "C" {
		t.Fatalf("Space value=%q got=%q want C", s.Value, got)
	}
}

func TestSegmented_PRD_SetValueKeepsRootAndChildren(t *testing.T) {
	// regression: SetValue must not tear option tree (SEG §6.11)
	s := kit.NewSegmented("a", "b", "c")
	root := s.Node()
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 400, Height: 40})
	before := root.Base().Children()
	if len(before) != 1 {
		t.Fatalf("children=%d", len(before))
	}
	row0 := before[0]
	r0 := s.Root
	s.SetValue("b")
	if s.Root != r0 {
		t.Fatal("SetValue replaced Root")
	}
	after := root.Base().Children()
	if len(after) != 1 || after[0] != row0 {
		t.Fatal("SetValue rebuilt option tree")
	}
	if s.Value != "b" {
		t.Fatal(s.Value)
	}
}

func TestSegmented_PRD_DefaultValue(t *testing.T) {
	s := kit.NewSegmented("A", "B", "C")
	s.SetDefaultValue("C")
	if s.Value != "C" || s.SelectedIndex() != 2 {
		t.Fatalf("default Value=%q idx=%d", s.Value, s.SelectedIndex())
	}
}
