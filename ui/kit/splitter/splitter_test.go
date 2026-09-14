package splitter_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type splitterSpec struct {
	SplitBarSize          float64 `json:"splitBarSize"`
	SplitTriggerSize      float64 `json:"splitTriggerSize"`
	SplitBarDraggableSize float64 `json:"splitBarDraggableSize"`
	ContainerMain         float64 `json:"containerMain"`
	ContainerCross        float64 `json:"containerCross"`
	DragDelta             float64 `json:"dragDelta"`
	NestedDelta           float64 `json:"nestedDelta"`
	MinPx                 float64 `json:"minPx"`
	MaxPx                 float64 `json:"maxPx"`
	KeyboardStepMin       float64 `json:"keyboardStepMin"`
}

func loadSplitterSpec(t *testing.T) splitterSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "splitter.json"))
	if err != nil {
		t.Fatalf("read splitter.json: %v", err)
	}
	var s splitterSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse splitter.json: %v", err)
	}
	if s.SplitBarSize != 2 || s.SplitTriggerSize != 6 || s.SplitBarDraggableSize != 20 {
		t.Fatalf("spec bar numbers %+v want 2/6/20", s)
	}
	if s.ContainerMain != 600 || s.DragDelta != 40 {
		t.Fatalf("spec container %+v want 600/40", s)
	}
	return s
}

func newColorPanel(r, g, b float64) (*splitter.SplitterPanel, *rendering.RenderColorBox) {
	box := rendering.NewRenderColorBox(10, 10, r, g, b, 1)
	return splitter.NewSplitterPanel(box), box
}

func newTwoPanelSplitter(w, h float64) *splitter.Splitter {
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(w)
	s.SetHeight(h)
	return s
}

func layoutSplitter(t *testing.T, s *splitter.Splitter, c rendering.Constraints) rendering.Size {
	t.Helper()
	return s.Layout(c)
}

func paintSplitter(s *splitter.Splitter, w, h int) {
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func TestSplitter_PRD_SPL01(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	if s.EffectiveOrientation() != splitter.Horizontal {
		t.Fatal("default orientation horizontal")
	}
	if s.IsVertical() {
		t.Fatal("default not vertical")
	}
	if s.Lazy() || s.DestroyOnHidden() {
		t.Fatal("lazy/destroy default false")
	}
	if s.IsControlled() {
		t.Fatal("default uncontrolled")
	}
	if s.BarCount() != 1 {
		t.Fatalf("bars=%d want 1", s.BarCount())
	}
	if s.Node() == nil || s.ChromeNode() == nil {
		t.Fatal("node/chrome nil")
	}
	sz := layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if math.Abs(sz.Width-spec.ContainerMain) > 0.5 || math.Abs(sz.Height-spec.ContainerCross) > 0.5 {
		t.Fatalf("layout=%v want 600x200", sz)
	}
	sizes := s.PanelSizes()
	if len(sizes) != 2 {
		t.Fatalf("sizes=%v", sizes)
	}
	sum := sizes[0] + sizes[1]
	if math.Abs(sum-spec.ContainerMain) > 0.5 {
		t.Fatalf("sum=%v want 600", sum)
	}
	if math.Abs(sizes[0]-spec.ContainerMain/2) > 0.5 {
		t.Fatalf("even split=%v want 300", sizes)
	}
	paintSplitter(s, 120, 60)
}

func TestSplitter_PRD_SPL02(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	var got []float64
	var n int
	s.OnResize(func(v []float64) {
		n++
		got = append([]float64(nil), v...)
	})
	if !s.DragBar(0, spec.DragDelta) {
		t.Fatal("drag should apply")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	sizes := s.PanelSizes()
	base := spec.ContainerMain / 2
	if math.Abs(sizes[0]-(base+spec.DragDelta)) > 0.5 || math.Abs(sizes[1]-(base-spec.DragDelta)) > 0.5 {
		t.Fatalf("sizes=%v want 340/260", sizes)
	}
	if n != 1 {
		t.Fatalf("onResize=%d want 1", n)
	}
	if math.Abs(got[0]+got[1]-spec.ContainerMain) > 0.5 {
		t.Fatalf("payload sum=%v want 600", got)
	}
}

func TestSplitter_PRD_SPL03(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetMinPx(spec.MinPx)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	// From 300 drag to 60 (delta -240) clamps at min 100.
	if !s.DragBar(0, 60-spec.ContainerMain/2) {
		t.Fatal("clamped drag should still apply")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	sizes := s.PanelSizes()
	if math.Abs(sizes[0]-spec.MinPx) > 0.5 {
		t.Fatalf("sizes=%v want clamp 100", sizes)
	}
}

func TestSplitter_PRD_SPL04(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetMaxPx(spec.MaxPx)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	// From 300 drag to 450 (delta +150) clamps at max 400.
	if !s.DragBar(0, 150) {
		t.Fatal("clamped drag should still apply")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	sizes := s.PanelSizes()
	if math.Abs(sizes[0]-spec.MaxPx) > 0.5 {
		t.Fatalf("sizes=%v want clamp 400", sizes)
	}
}

func TestSplitter_PRD_SPL05(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	var ends [][]float64
	s.OnResizeEnd(func(v []float64) { ends = append(ends, append([]float64(nil), v...)) })
	if !s.BeginDrag(0) {
		t.Fatal("begin")
	}
	s.UpdateDrag(spec.DragDelta)
	s.EndDrag()
	if len(ends) != 1 {
		t.Fatalf("onResizeEnd=%d want 1", len(ends))
	}
	cur := s.PanelSizes()
	if len(ends[0]) != 2 || math.Abs(ends[0][0]-cur[0]) > 0.5 {
		t.Fatalf("end payload=%v cur=%v", ends[0], cur)
	}
}

func TestSplitter_PRD_SPL06(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetCollapsible(true)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	var collapsed [][]bool
	var sizes [][]float64
	s.OnCollapse(func(c []bool, v []float64) {
		collapsed = append(collapsed, append([]bool(nil), c...))
		sizes = append(sizes, append([]float64(nil), v...))
	})
	s.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	cur := s.PanelSizes()
	if math.Abs(cur[0]) > 0.5 {
		t.Fatalf("collapsed=%v want 0", cur)
	}
	if math.Abs(cur[0]+cur[1]-spec.ContainerMain) > 0.5 {
		t.Fatalf("sum=%v want 600", cur)
	}
	if len(collapsed) != 1 || !collapsed[0][0] {
		t.Fatalf("onCollapse=%v", collapsed)
	}
	if len(sizes) != 1 {
		t.Fatalf("sizes payload=%v", sizes)
	}
}

func TestSplitter_PRD_SPL07(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	s := splitter.NewSplitter(lp, rp)
	s.SetVertical(true)
	s.SetWidth(spec.ContainerCross)
	s.SetHeight(spec.ContainerMain)
	if !s.IsVertical() {
		t.Fatal("vertical flag")
	}
	sz := layoutSplitter(t, s, rendering.Tight(spec.ContainerCross, spec.ContainerMain))
	if math.Abs(sz.Width-spec.ContainerCross) > 0.5 || math.Abs(sz.Height-spec.ContainerMain) > 0.5 {
		t.Fatalf("layout=%v", sz)
	}
	bar := s.BarNode(0)
	if bar == nil {
		t.Fatal("bar nil")
	}
	bsz := bar.Size()
	// Vertical layout => horizontal bar: width >> height, height == trigger.
	if math.Abs(bsz.Height-spec.SplitTriggerSize) > 0.5 {
		t.Fatalf("bar h=%v want 6", bsz)
	}
	if bsz.Width < spec.ContainerCross-0.5 {
		t.Fatalf("bar w=%v want 200", bsz)
	}
	before := append([]float64(nil), s.PanelSizes()...)
	if !s.DragBar(0, spec.DragDelta) {
		t.Fatal("vertical drag")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerCross, spec.ContainerMain))
	after := s.PanelSizes()
	if math.Abs((after[0]-before[0])-spec.DragDelta) > 0.5 {
		t.Fatalf("height change=%v want +40", after)
	}
}

func TestSplitter_PRD_SPL08(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	bar := s.BarNode(0)
	if bar == nil {
		t.Fatal("bar nil")
	}
	bsz := bar.Size()
	// Horizontal layout => vertical bar: width == trigger 6.
	if math.Abs(bsz.Width-spec.SplitTriggerSize) > 0.5 {
		t.Fatalf("bar w=%v want 6", bsz)
	}
	if s.SplitBarSize()+0.5 > s.SplitTriggerSize() {
		t.Fatalf("hit %v must cover visual %v", s.SplitTriggerSize(), s.SplitBarSize())
	}
	// Hit center lands on the bar (hit == layout == paint).
	hit := s.Node().HitTest(rendering.Point{X: spec.ContainerMain / 2, Y: spec.ContainerCross / 2})
	if hit == nil {
		t.Fatal("center should hit something")
	}
	// Bar offset centers on the seam: seam at 300, box 297..303.
	off := bar.Offset()
	if math.Abs(off.X-(spec.ContainerMain/2-spec.SplitTriggerSize/2)) > 0.5 {
		t.Fatalf("bar x=%v want 297", off.X)
	}
	paintSplitter(s, 120, 60)
}

func TestSplitter_PRD_SPL09(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetDefaultSizePercent(50)
	rp.SetDefaultSizePercent(50)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	var got []float64
	s.OnResize(func(v []float64) { got = append([]float64(nil), v...) })
	if !s.DragBar(0, spec.DragDelta) {
		t.Fatal("drag")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	sizes := s.PanelSizes()
	base := spec.ContainerMain / 2
	if math.Abs(sizes[0]-(base+spec.DragDelta)) > 0.5 || math.Abs(sizes[1]-(base-spec.DragDelta)) > 0.5 {
		t.Fatalf("size.tsx sizes=%v want 340/260", sizes)
	}
	if math.Abs(got[0]+got[1]-spec.ContainerMain) > 0.5 {
		t.Fatalf("payload sum=%v", got)
	}
}

func TestSplitter_PRD_SPL10(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetSizePx(spec.ContainerMain / 2)
	rp.SetSizePx(spec.ContainerMain / 2)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	if !s.IsControlled() {
		t.Fatal("size set => controlled")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	before := append([]float64(nil), s.PanelSizes()...)
	var got []float64
	s.OnResize(func(v []float64) { got = append([]float64(nil), v...) })
	if !s.DragBar(0, spec.DragDelta) {
		t.Fatal("controlled drag emits")
	}
	// Controlled: geometry frozen until parent writes back.
	mid := s.PanelSizes()
	if math.Abs(mid[0]-before[0]) > 0.5 {
		t.Fatalf("controlled frozen=%v before=%v", mid, before)
	}
	if len(got) != 2 {
		t.Fatalf("onResize payload=%v", got)
	}
	s.SetPanelSizesPx(got)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	after := s.PanelSizes()
	if math.Abs(after[0]-got[0]) > 0.5 {
		t.Fatalf("after writeback=%v want %v", after, got)
	}
}

func TestSplitter_PRD_SPL11(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	s := splitter.NewSplitter(lp, rp)
	s.SetOrientation(splitter.Vertical)
	s.SetWidth(spec.ContainerCross)
	s.SetHeight(spec.ContainerMain)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerCross, spec.ContainerMain))
	before := append([]float64(nil), s.PanelSizes()...)
	if !s.DragBar(0, spec.DragDelta) {
		t.Fatal("vertical drag")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerCross, spec.ContainerMain))
	after := s.PanelSizes()
	if math.Abs((after[0]-before[0])-spec.DragDelta) > 0.5 {
		t.Fatalf("vertical.tsx heights=%v want +40", after)
	}
	bar := s.BarNode(0)
	if bar == nil {
		t.Fatal("bar nil")
	}
	if math.Abs(bar.Size().Height-spec.SplitTriggerSize) > 0.5 {
		t.Fatalf("horizontal bar h=%v", bar.Size())
	}
}

func TestSplitter_PRD_SPL12(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetCollapsible(true)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	beforeRight := s.PanelSizes()[1]
	n := 0
	s.OnCollapse(func(c []bool, v []float64) { n++ })
	s.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	cur := s.PanelSizes()
	if math.Abs(cur[0]) > 0.5 {
		t.Fatalf("left=%v want 0", cur)
	}
	if math.Abs(cur[1]-(beforeRight+spec.ContainerMain/2)) > 0.5 {
		t.Fatalf("right=%v want %v", cur[1], beforeRight+spec.ContainerMain/2)
	}
	if n != 1 {
		t.Fatalf("onCollapse=%d want 1", n)
	}
	if !s.IsCollapsed(0) {
		t.Fatal("collapsed flag")
	}
}

func TestSplitter_PRD_SPL13(t *testing.T) {
	spec := loadSplitterSpec(t)
	newCollapsible := func() *splitter.Splitter {
		lp, _ := newColorPanel(0.9, 0.2, 0.2)
		rp, _ := newColorPanel(0.2, 0.4, 0.9)
		lp.SetCollapsible(true)
		lp.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
		rp.SetCollapsible(true)
		rp.SetShowCollapsibleIcon(splitter.CollapsibleIconAlways)
		s := splitter.NewSplitter(lp, rp)
		s.SetWidth(spec.ContainerMain)
		s.SetHeight(spec.ContainerCross)
		layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
		return s
	}
	s1 := newCollapsible()
	n1 := 0
	s1.OnCollapse(func(c []bool, v []float64) { n1++ })
	s1.CollapseAt(0, splitter.CollapseStart)
	if n1 != 1 {
		t.Fatalf("start collapse=%d want 1", n1)
	}
	s2 := newCollapsible()
	n2 := 0
	s2.OnCollapse(func(c []bool, v []float64) { n2++ })
	s2.CollapseAt(0, splitter.CollapseEnd)
	if n2 != 1 {
		t.Fatalf("end collapse=%d want 1", n2)
	}
	if !s1.IsCollapsed(0) || !s2.IsCollapsed(1) {
		t.Fatal("both sides collapsible")
	}
}

func TestSplitter_PRD_SPL14(t *testing.T) {
	spec := loadSplitterSpec(t)
	p1, _ := newColorPanel(0.9, 0.2, 0.2)
	p2, _ := newColorPanel(0.2, 0.9, 0.2)
	p3, _ := newColorPanel(0.2, 0.4, 0.9)
	s := splitter.NewSplitter(p1, p2, p3)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	before := append([]float64(nil), s.PanelSizes()...)
	if !s.DragBar(1, spec.DragDelta) {
		t.Fatal("middle bar drag")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	after := s.PanelSizes()
	if math.Abs(after[0]-before[0]) > 0.5 {
		t.Fatalf("left untouched=%v vs %v", after[0], before[0])
	}
	if math.Abs((after[1]-before[1])-spec.DragDelta) > 0.5 {
		t.Fatalf("middle=%v want +40", after)
	}
	if math.Abs((before[2]-after[2])-spec.DragDelta) > 0.5 {
		t.Fatalf("right=%v want -40", after)
	}
	sum := after[0] + after[1] + after[2]
	if math.Abs(sum-spec.ContainerMain) > 0.5 {
		t.Fatalf("sum=%v want 600", sum)
	}
}

func TestSplitter_PRD_SPL15(t *testing.T) {
	spec := loadSplitterSpec(t)
	// Inner splitter lives inside outer left panel.
	il, _ := newColorPanel(0.9, 0.2, 0.2)
	ir, _ := newColorPanel(0.2, 0.9, 0.2)
	inner := splitter.NewSplitter(il, ir)
	inner.SetWidth(spec.ContainerMain / 2)
	inner.SetHeight(spec.ContainerCross)
	inner.Layout(rendering.Tight(spec.ContainerMain/2, spec.ContainerCross))

	ol := splitter.NewSplitterPanel(inner.Node())
	or, _ := newColorPanel(0.2, 0.4, 0.9)
	outer := splitter.NewSplitter(ol, or)
	outer.SetWidth(spec.ContainerMain)
	outer.SetHeight(spec.ContainerCross)
	layoutSplitter(t, outer, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	// Sync inner to its host width after outer layout.
	inner.SetWidth(outer.PanelSizes()[0])
	inner.Layout(rendering.Tight(outer.PanelSizes()[0], spec.ContainerCross))

	var outerNotes, innerNotes int
	outer.OnResize(func(v []float64) { outerNotes++ })
	inner.OnResize(func(v []float64) { innerNotes++ })

	if !outer.DragBar(0, spec.NestedDelta) {
		t.Fatal("outer drag")
	}
	layoutSplitter(t, outer, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if outerNotes != 1 || innerNotes != 0 {
		t.Fatalf("outer=%d inner=%d want 1/0", outerNotes, innerNotes)
	}
	outerSizes := outer.PanelSizes()
	if math.Abs((outerSizes[0]-spec.ContainerMain/2)-spec.NestedDelta) > 0.5 {
		t.Fatalf("outer=%v", outerSizes)
	}
	// Inner drag stays inside.
	inner.SetWidth(outerSizes[0])
	inner.Layout(rendering.Tight(outerSizes[0], spec.ContainerCross))
	beforeOuter := append([]float64(nil), outer.PanelSizes()...)
	if !inner.DragBar(0, spec.NestedDelta) {
		t.Fatal("inner drag")
	}
	inner.Layout(rendering.Tight(outerSizes[0], spec.ContainerCross))
	if innerNotes != 1 || outerNotes != 1 {
		t.Fatalf("after inner: outer=%d inner=%d", outerNotes, innerNotes)
	}
	afterOuter := outer.PanelSizes()
	if math.Abs(afterOuter[0]-beforeOuter[0]) > 0.5 {
		t.Fatalf("inner drag must not move outer %v vs %v", afterOuter, beforeOuter)
	}
}

func TestSplitter_PRD_SPL16(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	s.SetLazy(true)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	before := append([]float64(nil), s.PanelSizes()...)
	var resizes, ends int
	s.OnResize(func(v []float64) { resizes++ })
	s.OnResizeEnd(func(v []float64) { ends++ })
	if !s.BeginDrag(0) {
		t.Fatal("begin")
	}
	s.UpdateDrag(spec.DragDelta)
	// Lazy: geometry frozen mid-drag, no resize yet.
	mid := s.PanelSizes()
	if math.Abs(mid[0]-before[0]) > 0.5 {
		t.Fatalf("lazy mid=%v want frozen %v", mid, before)
	}
	if resizes != 0 {
		t.Fatalf("lazy mid resizes=%d want 0", resizes)
	}
	if math.Abs(s.PreviewDelta()-spec.DragDelta) > 0.5 {
		t.Fatalf("preview=%v want 40", s.PreviewDelta())
	}
	s.EndDrag()
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	after := s.PanelSizes()
	if math.Abs((after[0]-before[0])-spec.DragDelta) > 0.5 {
		t.Fatalf("lazy commit=%v want +40", after)
	}
	if ends != 1 {
		t.Fatalf("ends=%d want 1", ends)
	}
}

func TestSplitter_PRD_SPL17(t *testing.T) {
	spec := loadSplitterSpec(t)
	s := newTwoPanelSplitter(spec.ContainerMain, spec.ContainerCross)
	if math.Abs(s.SplitBarSize()-spec.SplitBarSize) > 0.5 {
		t.Fatalf("bar=%v want 2", s.SplitBarSize())
	}
	if math.Abs(s.SplitTriggerSize()-spec.SplitTriggerSize) > 0.5 {
		t.Fatalf("trigger=%v want 6", s.SplitTriggerSize())
	}
	if math.Abs(s.SplitBarDraggableSize()-spec.SplitBarDraggableSize) > 0.5 {
		t.Fatalf("handle=%v want 20", s.SplitBarDraggableSize())
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	bar := s.BarNode(0)
	if bar == nil {
		t.Fatal("bar nil")
	}
	if math.Abs(bar.Size().Width-spec.SplitTriggerSize) > 0.5 {
		t.Fatalf("layout box=%v want 6", bar.Size())
	}
}

func TestSplitter_PRD_SPL18(t *testing.T) {
	s := newTwoPanelSplitter(600, 200)
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	if got := s.EffectiveBarColor(); got != toRGBA(tok.ColorBorderSecondary) {
		t.Fatalf("bar=%+v want ColorBorderSecondary", got)
	}
	if got := s.EffectiveBarHoverColor(); got != toRGBA(tok.ColorFillSecondary) {
		t.Fatalf("hover=%+v want ColorFillSecondary", got)
	}
	if got := s.EffectiveBarActiveColor(); got != toRGBA(tok.ColorFill) {
		t.Fatalf("active=%+v want ColorFill", got)
	}
	primary := render.Hex("#1677ff")
	if s.EffectiveBarColor() == primary {
		t.Fatal("default bar must not hardcode brand primary")
	}
	// Custom provider recolors the preview: proves Token wiring.
	base := theme.DefaultTokens()
	base.ColorPrimary = theme.RGBA(0, 200, 0, 1)
	p := theme.NewProvider(base)
	s.SetProvider(p)
	if got := s.EffectivePreviewColor(); got.G < 0.5 {
		t.Fatalf("custom Token preview=%+v want green", got)
	}
	paintSplitter(s, 120, 60)
}

func TestSplitter_PRD_SPL19(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetResizable(false)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if s.BarResizable(0) {
		t.Fatal("bar should report non-resizable")
	}
	before := append([]float64(nil), s.PanelSizes()...)
	if s.DragBar(0, spec.DragDelta) {
		t.Fatal("non-resizable drag must be rejected")
	}
	after := s.PanelSizes()
	if math.Abs(after[0]-before[0]) > 0.5 {
		t.Fatalf("sizes moved=%v vs %v", after, before)
	}
	// Collapsible still works when resizable is off.
	lp.SetCollapsible(true)
	s.CollapseAt(0, splitter.CollapseStart)
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if math.Abs(s.PanelSizes()[0]) > 0.5 {
		t.Fatalf("fold must still work %v", s.PanelSizes())
	}
	paintSplitter(s, 120, 60)
}

func TestSplitter_PRD_SPL20(t *testing.T) {
	spec := loadSplitterSpec(t)
	lp, _ := newColorPanel(0.9, 0.2, 0.2)
	rp, _ := newColorPanel(0.2, 0.4, 0.9)
	lp.SetCollapsible(true)
	s := splitter.NewSplitter(lp, rp)
	s.SetWidth(spec.ContainerMain)
	s.SetHeight(spec.ContainerCross)
	s.SetAriaLabel("demo splitter")
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if s.Role() != "separator" || s.BarRole(0) != "separator" {
		t.Fatalf("roles %q/%q", s.Role(), s.BarRole(0))
	}
	if s.AriaLabel() != "demo splitter" || s.BarAriaLabel(0) == "" {
		t.Fatalf("aria %q/%q", s.AriaLabel(), s.BarAriaLabel(0))
	}
	if !s.Focusable() || !s.BarFocusable(0) {
		t.Fatal("bar must take Focus")
	}
	if !s.FocusBar(0) || !s.BarFocused(0) || !s.Focused() {
		t.Fatal("Focus must latch")
	}
	if !s.FocusRingVisible(0) {
		t.Fatal("Focus ring must be visible")
	}
	before := append([]float64(nil), s.PanelSizes()...)
	step := s.KeyboardStep()
	if step+0.5 < spec.KeyboardStepMin {
		t.Fatalf("step=%v want >=4", step)
	}
	if !s.HandleBarKey(0, "Right") {
		t.Fatal("arrow Right must nudge")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	after := s.PanelSizes()
	if math.Abs((after[0]-before[0])-step) > 0.5 {
		t.Fatalf("arrow step=%v want %v", after[0]-before[0], step)
	}
	if !s.HandleBarKey(0, "Left") {
		t.Fatal("arrow Left must nudge back")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	back := s.PanelSizes()
	if math.Abs(back[0]-before[0]) > 0.5 {
		t.Fatalf("arrow back=%v want %v", back, before)
	}
	// Layout matrix: Exact/Min/Max each run.
	exact := layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	loose := layoutSplitter(t, s, rendering.Loose(spec.ContainerMain, spec.ContainerCross))
	expand := layoutSplitter(t, s, rendering.Expand())
	if exact.Width <= 0 || loose.Width <= 0 || expand.Width <= 0 {
		t.Fatalf("matrix %v/%v/%v", exact, loose, expand)
	}
	s.BlurBar()
	if s.Focused() || s.FocusRingVisible(0) {
		t.Fatal("Blur clears Focus ring")
	}
	// Enter on a focused collapsible bar folds.
	s.FocusBar(0)
	if !s.KeyActivate("Enter") {
		t.Fatal("Enter must collapse")
	}
	layoutSplitter(t, s, rendering.Tight(spec.ContainerMain, spec.ContainerCross))
	if math.Abs(s.PanelSizes()[0]) > 0.5 {
		t.Fatalf("enter fold=%v", s.PanelSizes())
	}
}
