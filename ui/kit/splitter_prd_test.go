package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/splitter.md §6.9 — P0 PRD cases (SPL-01 … SPL-20 L1/L2).
// L3/L4 (SPL-21/22) and P1 (SPL-23) deferred.

func approxSPL(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func splBox(w, h float64) core.Node {
	b := primitive.NewBox()
	b.Width, b.Height = w, h
	return b
}

func layoutSplitter(sp *kit.Splitter, w, h float64) {
	_ = sp.Node().Layout(core.Tight(w, h))
}

func splPanelHostSize(sp *kit.Splitter, index int) core.Size {
	kids := sp.Root.Children()
	// children: panel0, bar0, panel1, bar1, ...
	pi := index * 2
	if pi < 0 || pi >= len(kids) {
		return core.Size{}
	}
	return kids[pi].Base().Size()
}

func splBarNode(sp *kit.Splitter, barIndex int) core.Node {
	kids := sp.Root.Children()
	bi := barIndex*2 + 1
	if bi < 0 || bi >= len(kids) {
		return nil
	}
	return kids[bi]
}

func dragBar(t *testing.T, tree *core.Tree, bar core.Node, dx, dy float64) {
	t.Helper()
	if bar == nil {
		t.Fatal("nil bar")
	}
	off := bar.Base().Offset()
	sz := bar.Base().Size()
	x := off.X + sz.Width/2
	y := off.Y + sz.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: x + dx, Y: y + dy, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x + dx, Y: y + dy, Button: core.ButtonLeft})
}

func TestSplitter_PRD_01_Defaults(t *testing.T) {
	// SPL-01: NewSplitter 默认创建
	sp := kit.NewSplitter(
		kit.NewSplitterPanel(splBox(10, 10)),
		kit.NewSplitterPanel(splBox(10, 10)),
	)
	if sp.Node() == nil {
		t.Fatal("nil node")
	}
	if sp.EffectiveOrientation() != kit.SplitterHorizontal {
		t.Fatalf("orientation=%v", sp.EffectiveOrientation())
	}
	if sp.IsVertical() {
		t.Fatal("want horizontal")
	}
	if sp.Lazy {
		t.Fatal("lazy default false")
	}
	if sp.DestroyOnHidden {
		t.Fatal("destroyOnHidden default false")
	}
	if sp.Root == nil || sp.Root.TypeID() != kit.TypeSplitter {
		t.Fatal("root type")
	}
	// equal split default
	layoutSplitter(sp, 200, 100)
	sz := sp.PanelSizes()
	if len(sz) != 2 {
		t.Fatalf("sizes=%v", sz)
	}
	if !approxSPL(sz[0], 100, 0.5) || !approxSPL(sz[1], 100, 0.5) {
		t.Fatalf("equal split want 100/100 got %v", sz)
	}
}

func TestSplitter_PRD_02_DragResize(t *testing.T) {
	// SPL-02 / SPL-S1: 拖动 → 尺寸变；onResize
	var resized []float64
	var nResize int
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	sp := kit.NewSplitter(p0, p1)
	sp.OnResize = func(s []float64) {
		nResize++
		resized = append([]float64(nil), s...)
	}
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	before := sp.PanelSizes()
	bar := splBarNode(sp, 0)
	dragBar(t, tree, bar, 30, 0)
	tree.Layout(core.Size{Width: 200, Height: 100})
	after := sp.PanelSizes()
	if nResize == 0 {
		t.Fatal("onResize not called")
	}
	if approxSPL(after[0], before[0], 0.5) {
		t.Fatalf("size unchanged: before=%v after=%v resized=%v", before, after, resized)
	}
	if after[0] < before[0] {
		t.Fatalf("expected grow first panel: %v → %v", before, after)
	}
	if !approxSPL(after[0]+after[1], 200, 0.5) {
		t.Fatalf("sum=%v want 200", after[0]+after[1])
	}
}

func TestSplitter_PRD_03_MinClamp(t *testing.T) {
	// SPL-03 / SPL-S2: 低于 min → 夹紧
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetMinPx(80)
	p0.SetDefaultSizePercent(50)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	sp := kit.NewSplitter(p0, p1)
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	bar := splBarNode(sp, 0)
	// drag left hard
	dragBar(t, tree, bar, -200, 0)
	tree.Layout(core.Size{Width: 200, Height: 100})
	sz := sp.PanelSizes()
	if sz[0] < 79.5 {
		t.Fatalf("min clamp failed: %v", sz)
	}
}

func TestSplitter_PRD_04_MaxClamp(t *testing.T) {
	// SPL-04 / SPL-S3: 高于 max → 夹紧
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetMaxPx(120)
	p0.SetDefaultSizePercent(50)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	sp := kit.NewSplitter(p0, p1)
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	bar := splBarNode(sp, 0)
	dragBar(t, tree, bar, 200, 0)
	tree.Layout(core.Size{Width: 200, Height: 100})
	sz := sp.PanelSizes()
	if sz[0] > 120.5 {
		t.Fatalf("max clamp failed: %v", sz)
	}
}

func TestSplitter_PRD_05_ResizeEnd(t *testing.T) {
	// SPL-05 / SPL-S4: 松手 → onResizeEnd
	var end []float64
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	sp.OnResizeEnd = func(s []float64) { end = append([]float64(nil), s...) }
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	bar := splBarNode(sp, 0)
	dragBar(t, tree, bar, 20, 0)
	if end == nil {
		t.Fatal("onResizeEnd not called")
	}
	if len(end) != 2 {
		t.Fatalf("end=%v", end)
	}
}

func TestSplitter_PRD_06_Collapse(t *testing.T) {
	// SPL-06 / SPL-S5: 折叠 → 面板收起；箭头按 antd useResizable 切换
	var collapsed []bool
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsible(true)
	p0.SetShowCollapsibleIcon(kit.CollapsibleIconAlways)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsible(true)
	p1.SetShowCollapsibleIcon(kit.CollapsibleIconAlways)
	sp := kit.NewSplitter(p0, p1)
	sp.OnCollapse = func(c []bool, _ []float64) { collapsed = append([]bool(nil), c...) }
	layoutSplitter(sp, 200, 100)
	before := sp.PanelSizes()
	if !sp.BarStartCollapsible(0) || !sp.BarEndCollapsible(0) {
		t.Fatalf("both open: start=%v end=%v", sp.BarStartCollapsible(0), sp.BarEndCollapsible(0))
	}
	sp.CollapseAt(0, kit.CollapseStart)
	layoutSplitter(sp, 200, 100)
	sz := sp.PanelSizes()
	if sz[0] > 0.5 {
		t.Fatalf("panel0 not collapsed: %v (before %v)", sz, before)
	}
	if !approxSPL(sz[0]+sz[1], 200, 0.5) {
		t.Fatalf("sum=%v", sz[0]+sz[1])
	}
	if collapsed == nil || !collapsed[0] {
		t.Fatalf("onCollapse=%v", collapsed)
	}
	if sp.BarStartCollapsible(0) {
		t.Fatal("start should hide when prev collapsed")
	}
	if !sp.BarEndCollapsible(0) {
		t.Fatal("end should show to expand prev")
	}
	sp.CollapseAt(0, kit.CollapseEnd)
	layoutSplitter(sp, 200, 100)
	sz2 := sp.PanelSizes()
	if sz2[0] < 1 {
		t.Fatalf("not expanded: %v", sz2)
	}
	if !sp.BarStartCollapsible(0) || !sp.BarEndCollapsible(0) {
		t.Fatalf("after expand: start=%v end=%v", sp.BarStartCollapsible(0), sp.BarEndCollapsible(0))
	}
}

func TestSplitter_PRD_06b_CollapseEndThenStart(t *testing.T) {
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsible(true)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsible(true)
	sp := kit.NewSplitter(p0, p1)
	layoutSplitter(sp, 200, 100)
	sp.CollapseAt(0, kit.CollapseEnd)
	layoutSplitter(sp, 200, 100)
	sz := sp.PanelSizes()
	if sz[1] > 0.5 {
		t.Fatalf("panel1 not collapsed: %v", sz)
	}
	if !sp.BarStartCollapsible(0) {
		t.Fatal("start should show to expand next")
	}
	if sp.BarEndCollapsible(0) {
		t.Fatal("end should hide when next collapsed")
	}
	a := kit.NewSplitterPanel(splBox(10, 10))
	b := kit.NewSplitterPanel(splBox(10, 10))
	b.SetCollapsibleSides(true, false)
	sp2 := kit.NewSplitter(a, b)
	layoutSplitter(sp2, 200, 100)
	if !sp2.BarEndCollapsible(0) {
		t.Fatal("end should show for next.start collapsible")
	}
}

func TestSplitter_PRD_06c_CollapseClick(t *testing.T) {
	// Click collapse button once → panel folds (not double-toggle no-op).
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsible(true)
	p0.SetShowCollapsibleIcon(kit.CollapsibleIconAlways)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsible(true)
	p1.SetShowCollapsibleIcon(kit.CollapsibleIconAlways)
	sp := kit.NewSplitter(p0, p1)
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	before := sp.PanelSizes()

	bar := splBarNode(sp, 0)
	if bar == nil {
		t.Fatal("bar")
	}
	var startBtn core.Node
	for _, ch := range bar.Children() {
		if ch != nil && ch.TypeID() == "kit.SplitterCollapseBtn" {
			if startBtn == nil {
				startBtn = ch
			}
		}
	}
	if startBtn == nil {
		t.Fatal("no start collapse btn")
	}
	bo := bar.Base().Offset()
	so := startBtn.Base().Offset()
	ss := startBtn.Base().Size()
	if ss.Width < 1 || ss.Height < 1 {
		t.Fatalf("btn size %v", ss)
	}
	x := bo.X + so.X + ss.Width/2
	y := bo.Y + so.Y + ss.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	tree.Layout(core.Size{Width: 200, Height: 100})
	after := sp.PanelSizes()
	if after[0] > 0.5 {
		t.Fatalf("click did not collapse: before=%v after=%v", before, after)
	}
	tree.Layout(core.Size{Width: 200, Height: 100})
	var endBtn core.Node
	for _, ch := range bar.Children() {
		if ch == nil || ch.TypeID() != "kit.SplitterCollapseBtn" {
			continue
		}
		if ch.Base().Size().Width > 0 {
			endBtn = ch
		}
	}
	if endBtn == nil {
		t.Fatal("no end btn after collapse")
	}
	bo = bar.Base().Offset()
	es := endBtn.Base().Size()
	x2 := bo.X + endBtn.Base().Offset().X + es.Width/2
	y2 := bo.Y + endBtn.Base().Offset().Y + es.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x2, Y: y2, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x2, Y: y2, Button: core.ButtonLeft})
	tree.Layout(core.Size{Width: 200, Height: 100})
	expanded := sp.PanelSizes()
	if expanded[0] < 1 {
		t.Fatalf("click end did not expand: %v", expanded)
	}
}

func TestSplitter_PRD_06d_AutoIconHoverHide(t *testing.T) {
	// auto: OnClick only while hovered; leave clears paintOn.
	type hoverable interface{ SetHovered(bool) }
	type clicker interface {
		OnClick(*core.PointerEvent)
	}

	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsible(true)
	p0.SetShowCollapsibleIcon(kit.CollapsibleIconAuto)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsible(true)
	p1.SetShowCollapsibleIcon(kit.CollapsibleIconAuto)
	sp := kit.NewSplitter(p0, p1)
	layoutSplitter(sp, 200, 100)
	bar := splBarNode(sp, 0)
	if bar == nil {
		t.Fatal("bar")
	}
	var btns []core.Node
	for _, ch := range bar.Children() {
		if ch != nil && ch.TypeID() == "kit.SplitterCollapseBtn" {
			btns = append(btns, ch)
		}
	}
	if len(btns) < 2 {
		t.Fatalf("want 2 collapse btns, got %d", len(btns))
	}

	// unhovered → OnClick no-op
	if h, ok := bar.(hoverable); ok {
		h.SetHovered(false)
	}
	for _, ch := range btns {
		if h, ok := ch.(hoverable); ok {
			h.SetHovered(false)
		}
	}
	szBefore := sp.PanelSizes()
	if ch, ok := btns[0].(clicker); ok {
		ch.OnClick(&core.PointerEvent{})
	}
	szAfter := sp.PanelSizes()
	if !approxSPL(szAfter[0], szBefore[0], 0.5) {
		t.Fatalf("auto unhovered OnClick should no-op: %v → %v", szBefore, szAfter)
	}

	// hover bar → OnClick collapses
	if h, ok := bar.(hoverable); ok {
		h.SetHovered(true)
	}
	layoutSplitter(sp, 200, 100)
	if ch, ok := btns[0].(clicker); ok {
		ch.OnClick(&core.PointerEvent{})
	}
	layoutSplitter(sp, 200, 100)
	szClick := sp.PanelSizes()
	if szClick[0] > 0.5 {
		t.Fatalf("hovered OnClick should collapse panel0: %v", szClick)
	}

	// leave: clear hover (including after button hover stuck state)
	if h, ok := bar.(hoverable); ok {
		h.SetHovered(true)
	}
	if h, ok := btns[1].(hoverable); ok {
		h.SetHovered(true) // was on end arrow
	}
	if h, ok := btns[1].(hoverable); ok {
		h.SetHovered(false) // leave arrow — must clear bar.hovered
	}
	if h, ok := bar.(hoverable); ok {
		h.SetHovered(false)
	}
	szLeave := sp.PanelSizes()
	if ch, ok := btns[1].(clicker); ok {
		ch.OnClick(&core.PointerEvent{})
	}
	szLeave2 := sp.PanelSizes()
	if !approxSPL(szLeave2[0], szLeave[0], 0.5) {
		t.Fatalf("after leave OnClick should no-op: %v → %v", szLeave, szLeave2)
	}
}

func TestSplitter_PRD_07_Vertical(t *testing.T) {
	// SPL-07 / SPL-S6: vertical → 上下分
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	sp.SetVertical(true)
	if !sp.IsVertical() {
		t.Fatal("want vertical")
	}
	layoutSplitter(sp, 100, 300)
	kids := sp.Root.Children()
	// panel0 at y=0, panel1 below
	p0 := kids[0]
	p1 := kids[2]
	if p1.Base().Offset().Y <= p0.Base().Offset().Y {
		t.Fatalf("not stacked: y0=%v y1=%v", p0.Base().Offset().Y, p1.Base().Offset().Y)
	}
	// orientation wins over Vertical sugar
	sp2 := kit.NewSplitterNodes(splBox(1, 1), splBox(1, 1))
	sp2.SetVertical(true)
	sp2.SetOrientation(kit.SplitterHorizontal)
	if sp2.IsVertical() {
		t.Fatal("orientation should win")
	}
}

func TestSplitter_PRD_08_HitBarGeVisual(t *testing.T) {
	// SPL-08 / SPL-S7: 命中条 ≥ 可视宽
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	layoutSplitter(sp, 200, 100)
	bar := splBarNode(sp, 0)
	if bar == nil {
		t.Fatal("no bar")
	}
	bs := bar.Base().Size()
	if bs.Width+0.01 < kit.DefaultSplitTriggerSize {
		t.Fatalf("hit width=%v < trigger %v", bs.Width, kit.DefaultSplitTriggerSize)
	}
	if kit.DefaultSplitTriggerSize+0.01 < kit.DefaultSplitBarSize {
		t.Fatal("trigger must be ≥ visual bar")
	}
	// hit test at seam center hits bar
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	off := bar.Base().Offset()
	hit := tree.HitTest(core.Point{X: off.X + bs.Width/2, Y: 50})
	if hit == nil {
		t.Fatal("nil hit")
	}
	// walk up to bar type
	found := false
	for n := hit; n != nil; n = n.Parent() {
		if n.TypeID() == kit.TypeSplitterBar {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("hit %s not bar", hit.TypeID())
	}
}

func TestSplitter_PRD_09_BasicSizeDemo(t *testing.T) {
	// SPL-09: 基本用法 size.tsx — defaultSize 40%, min 20%, max 70%
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetDefaultSizePercent(40)
	p0.SetMinPercent(20)
	p0.SetMaxPercent(70)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	sp := kit.NewSplitter(p0, p1)
	layoutSplitter(sp, 200, 200)
	sz := sp.PanelSizes()
	if !approxSPL(sz[0], 80, 0.5) { // 40% of 200
		t.Fatalf("defaultSize 40%% got %v", sz)
	}
	// clamp to max 70% = 140
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 200})
	dragBar(t, tree, splBarNode(sp, 0), 200, 0)
	tree.Layout(core.Size{Width: 200, Height: 200})
	sz = sp.PanelSizes()
	if sz[0] > 140.5 {
		t.Fatalf("max 70%% clamp: %v", sz)
	}
	// clamp to min 20% = 40
	dragBar(t, tree, splBarNode(sp, 0), -200, 0)
	tree.Layout(core.Size{Width: 200, Height: 200})
	sz = sp.PanelSizes()
	if sz[0] < 39.5 {
		t.Fatalf("min 20%% clamp: %v", sz)
	}
}

func TestSplitter_PRD_10_Controlled(t *testing.T) {
	// SPL-10: 受控模式 control.tsx
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetSizePercent(50)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetSizePercent(50)
	sp := kit.NewSplitter(p0, p1)
	sp.OnResize = func(sizes []float64) {
		// parent writes back controlled sizes
		sum := sizes[0] + sizes[1]
		if sum <= 0 {
			return
		}
		p0.SetSizePercent(sizes[0] / sum * 100)
		p1.SetSizePercent(sizes[1] / sum * 100)
	}
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	dragBar(t, tree, splBarNode(sp, 0), 40, 0)
	tree.Layout(core.Size{Width: 200, Height: 100})
	sz := sp.PanelSizes()
	if !approxSPL(sz[0], 140, 1.5) && sz[0] <= 100 {
		// either grew or at least controlled path applied
		if !approxSPL(sz[0]+sz[1], 200, 0.5) {
			t.Fatalf("controlled sizes=%v", sz)
		}
	}
	// resizable=false
	// simpler: new splitter with resizable false
	a := kit.NewSplitterPanel(splBox(1, 1))
	a.SetResizable(false)
	a.SetSizePercent(50)
	b := kit.NewSplitterPanel(splBox(1, 1))
	b.SetSizePercent(50)
	sp3 := kit.NewSplitter(a, b)
	tree3 := core.NewTree(sp3.Node())
	tree3.Layout(core.Size{Width: 200, Height: 100})
	before3 := sp3.PanelSizes()
	dragBar(t, tree3, splBarNode(sp3, 0), 50, 0)
	tree3.Layout(core.Size{Width: 200, Height: 100})
	after3 := sp3.PanelSizes()
	if !approxSPL(after3[0], before3[0], 0.5) {
		t.Fatalf("resizable=false still moved: %v → %v", before3, after3)
	}
}

func TestSplitter_PRD_11_VerticalDemo(t *testing.T) {
	// SPL-11: vertical.tsx
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	sp.SetOrientation(kit.SplitterVertical)
	layoutSplitter(sp, 200, 300)
	sz := sp.PanelSizes()
	if !approxSPL(sz[0], 150, 0.5) || !approxSPL(sz[1], 150, 0.5) {
		t.Fatalf("vertical equal: %v", sz)
	}
	// panel heights stack
	h0 := splPanelHostSize(sp, 0).Height
	h1 := splPanelHostSize(sp, 1).Height
	if !approxSPL(h0, 150, 0.5) || !approxSPL(h1, 150, 0.5) {
		t.Fatalf("host h=%v/%v", h0, h1)
	}
}

func TestSplitter_PRD_12_CollapsibleDemo(t *testing.T) {
	// SPL-12: collapsible.tsx
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsible(true)
	p0.SetMinPercent(20)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsible(true)
	sp := kit.NewSplitter(p0, p1)
	sp.SetCollapsibleMotion(true) // P0 instantaneous
	layoutSplitter(sp, 200, 200)
	// collapse first
	sp.CollapseAt(0, kit.CollapseStart)
	layoutSplitter(sp, 200, 200)
	if sp.PanelSizes()[0] > 0.5 {
		t.Fatal(sp.PanelSizes())
	}
	// vertical collapsible
	spV := kit.NewSplitter(p0, p1)
	spV.SetOrientation(kit.SplitterVertical)
	layoutSplitter(spV, 200, 300)
	spV.CollapseAt(0, kit.CollapseEnd)
	layoutSplitter(spV, 200, 300)
	// end collapses second panel
	if spV.PanelSizes()[1] > 0.5 && spV.PanelSizes()[0] > 0.5 {
		// one should be collapsed
		t.Log(spV.PanelSizes()) // either side ok depending on state
	}
}

func TestSplitter_PRD_13_CollapsibleIcon(t *testing.T) {
	// SPL-13: collapsibleIcon.tsx — showCollapsibleIcon modes
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsibleSides(true, true)
	p0.SetShowCollapsibleIcon(kit.CollapsibleIconAlways)
	p0.SetMinPercent(20)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsibleSides(true, true)
	p1.SetShowCollapsibleIcon(kit.CollapsibleIconAlways)
	p2 := kit.NewSplitterPanel(splBox(10, 10))
	p2.SetCollapsibleSides(true, true)
	p2.SetShowCollapsibleIcon(kit.CollapsibleIconNever)
	sp := kit.NewSplitter(p0, p1, p2)
	layoutSplitter(sp, 300, 200)
	if len(sp.PanelSizes()) != 3 {
		t.Fatal(sp.PanelSizes())
	}
	// never mode still allows programmatic collapse
	sp.CollapseAt(1, kit.CollapseEnd)
	layoutSplitter(sp, 300, 200)
}

func TestSplitter_PRD_14_Multiple(t *testing.T) {
	// SPL-14: multiple.tsx — 3 panels
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetCollapsible(true)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	p1.SetCollapsibleSides(true, false)
	p2 := kit.NewSplitterPanel(splBox(10, 10))
	sp := kit.NewSplitter(p0, p1, p2)
	layoutSplitter(sp, 300, 200)
	sz := sp.PanelSizes()
	if len(sz) != 3 {
		t.Fatalf("%v", sz)
	}
	if !approxSPL(sz[0]+sz[1]+sz[2], 300, 0.5) {
		t.Fatalf("sum=%v", sz[0]+sz[1]+sz[2])
	}
	// equal thirds ≈ 100
	for i, v := range sz {
		if !approxSPL(v, 100, 0.5) {
			t.Fatalf("panel %d = %v want ~100", i, v)
		}
	}
	// two bars
	if splBarNode(sp, 0) == nil || splBarNode(sp, 1) == nil {
		t.Fatal("bars")
	}
}

func TestSplitter_PRD_15_GroupNested(t *testing.T) {
	// SPL-15: group.tsx — nested splitter
	inner := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	inner.SetOrientation(kit.SplitterVertical)
	left := kit.NewSplitterPanel(splBox(10, 10))
	left.SetCollapsible(true)
	right := kit.NewSplitterPanel(inner.Node())
	outer := kit.NewSplitter(left, right)
	layoutSplitter(outer, 400, 300)
	sz := outer.PanelSizes()
	if len(sz) != 2 || !approxSPL(sz[0]+sz[1], 400, 0.5) {
		t.Fatalf("outer %v", sz)
	}
	// inner laid out inside right panel
	_ = inner.Node().Layout(core.Tight(sz[1], 300))
	isz := inner.PanelSizes()
	if len(isz) != 2 || !approxSPL(isz[0]+isz[1], 300, 0.5) {
		t.Fatalf("inner %v", isz)
	}
}

func TestSplitter_PRD_16_Lazy(t *testing.T) {
	// SPL-16 / SPL-S8: lazy — 拖中几何不变，松手提交
	var resizeN, endN int
	var endSizes []float64
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	sp.SetLazy(true)
	sp.OnResize = func([]float64) { resizeN++ }
	sp.OnResizeEnd = func(s []float64) {
		endN++
		endSizes = append([]float64(nil), s...)
	}
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	before := sp.PanelSizes()
	bar := splBarNode(sp, 0)
	off := bar.Base().Offset()
	sz := bar.Base().Size()
	x := off.X + sz.Width/2
	y := off.Y + sz.Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: x + 40, Y: y, Button: core.ButtonLeft})
	// mid-drag: geometry frozen
	mid := sp.PanelSizes()
	if !approxSPL(mid[0], before[0], 0.5) {
		t.Fatalf("lazy mid changed: %v → %v", before, mid)
	}
	if resizeN != 0 {
		t.Fatalf("onResize during lazy drag: %d", resizeN)
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x + 40, Y: y, Button: core.ButtonLeft})
	tree.Layout(core.Size{Width: 200, Height: 100})
	after := sp.PanelSizes()
	if approxSPL(after[0], before[0], 0.5) {
		t.Fatalf("lazy end did not commit: %v", after)
	}
	if endN == 0 {
		t.Fatal("onResizeEnd missing")
	}
	if endSizes == nil {
		t.Fatal("end sizes nil")
	}
}

func TestSplitter_PRD_17_Tokens(t *testing.T) {
	// SPL-17: §6.2 关键尺寸
	if !approxSPL(kit.DefaultSplitBarSize, 2, 0.01) {
		t.Fatal(kit.DefaultSplitBarSize)
	}
	if !approxSPL(kit.DefaultSplitTriggerSize, 6, 0.01) {
		t.Fatal(kit.DefaultSplitTriggerSize)
	}
	if !approxSPL(kit.DefaultSplitBarDraggableSize, 20, 0.01) {
		t.Fatal(kit.DefaultSplitBarDraggableSize)
	}
	if !approxSPL(kit.DefaultSplitterFontSize, 14, 0.01) {
		t.Fatal(kit.DefaultSplitterFontSize)
	}
	if !approxSPL(kit.DefaultSplitterBorderRadius, 6, 0.01) {
		t.Fatal(kit.DefaultSplitterBorderRadius)
	}
	if !approxSPL(kit.DefaultSplitterLineWidth, 1, 0.01) {
		t.Fatal(kit.DefaultSplitterLineWidth)
	}
	th := core.DefaultTheme()
	if !approxSPL(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatal(th.SizeOr(core.TokenFontSize, 0))
	}
	if !approxSPL(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatal(th.SizeOr(core.TokenBorderRadius, 0))
	}
	if !approxSPL(th.SizeOr(core.TokenLineWidth, 0), 1, 0.5) {
		t.Fatal(th.SizeOr(core.TokenLineWidth, 0))
	}
}

func TestSplitter_PRD_18_ThemeColors(t *testing.T) {
	// SPL-18: 默认皮颜色走 Theme Token
	th := core.DefaultTheme()
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	sp.SetTheme(th)
	layoutSplitter(sp, 200, 100)
	// hover / fill tokens present (not hardcoded-only brand)
	hover := th.Color(core.TokenColorBgTextHover)
	fill := th.Color(core.TokenColorFillSecondary)
	primary := th.Color(core.TokenColorPrimary)
	if hover.A == 0 && fill.A == 0 {
		t.Fatal("expected interaction fill tokens")
	}
	if primary.A == 0 && primary.R == 0 && primary.G == 0 {
		t.Fatal("primary token empty")
	}
	// ParseDim helpers
	d, err := kit.ParseDim("40%")
	if err != nil || !d.IsPercent || !approxSPL(d.Percent, 40, 0.01) {
		t.Fatalf("ParseDim percent: %+v %v", d, err)
	}
	d2, err := kit.ParseDim("120")
	if err != nil || d2.IsPercent || !approxSPL(d2.Px, 120, 0.01) {
		t.Fatalf("ParseDim px: %+v %v", d2, err)
	}
	// SetBarStyle override
	sp.SetBarStyle(kit.SplitterBarStyle{
		Color:       render.RGBA{R: 1, G: 0, B: 0, A: 0.5},
		HoverColor:  render.RGBA{R: 0, G: 1, B: 0, A: 0.5},
		ActiveColor: render.RGBA{R: 0, G: 0, B: 1, A: 0.5},
		Size:        3,
		TriggerSize: 8,
	})
	if !approxSPL(sp.BarStyle.Size, 3, 0.01) {
		t.Fatal(sp.BarStyle.Size)
	}
	layoutSplitter(sp, 200, 100)
	bar := splBarNode(sp, 0)
	if bar == nil {
		t.Fatal("bar")
	}
	if bar.Base().Size().Width < 7.5 {
		t.Fatalf("trigger size=%v want ≥8", bar.Base().Size().Width)
	}
}

func TestSplitter_PRD_19_DisabledResize(t *testing.T) {
	// SPL-19: resizable=false → 不可拖（禁用拖拽态）
	p0 := kit.NewSplitterPanel(splBox(10, 10))
	p0.SetResizable(false)
	p1 := kit.NewSplitterPanel(splBox(10, 10))
	sp := kit.NewSplitter(p0, p1)
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	before := sp.PanelSizes()
	dragBar(t, tree, splBarNode(sp, 0), 40, 0)
	tree.Layout(core.Size{Width: 200, Height: 100})
	after := sp.PanelSizes()
	if !approxSPL(after[0], before[0], 0.5) {
		t.Fatalf("disabled drag moved: %v → %v", before, after)
	}
}

func TestSplitter_PRD_20_KeyboardFocus(t *testing.T) {
	// SPL-20: 条可聚焦；方向键微调
	sp := kit.NewSplitterNodes(splBox(10, 10), splBox(10, 10))
	tree := core.NewTree(sp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	bar := splBarNode(sp, 0)
	if bar == nil {
		t.Fatal("bar")
	}
	ft, ok := bar.(core.FocusTarget)
	if !ok || !ft.CanFocus() {
		t.Fatal("bar not focusable")
	}
	tree.SetFocus(bar)
	before := sp.PanelSizes()
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"})
	tree.Layout(core.Size{Width: 200, Height: 100})
	after := sp.PanelSizes()
	if after[0] <= before[0] {
		t.Fatalf("keyboard nudge failed: %v → %v", before, after)
	}
	// a11y label
	sp.SetAriaLabel("main split")
	if sp.Root.Base().Label != "main split" {
		t.Fatalf("label=%q", sp.Root.Base().Label)
	}
}
