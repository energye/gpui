package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/timeline.md §6.9 — P0 PRD cases (TL-01 … TL-19 L1/L2).
// TL-20 L3 / TL-21 L4 / TL-22 P1 deferred.

func approxTL(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxTLColor(a, b render.RGBA, tol float64) bool {
	return approxTL(float64(a.R), float64(b.R), tol) &&
		approxTL(float64(a.G), float64(b.G), tol) &&
		approxTL(float64(a.B), float64(b.B), tol) &&
		approxTL(float64(a.A), float64(b.A), tol)
}

func TestTimeline_PRD_01_Defaults(t *testing.T) {
	// TL-01: NewTimeline 默认创建
	tl := kit.NewTimeline()
	if tl.Mode() != kit.TimelineModeStart {
		t.Fatalf("mode=%v want start", tl.Mode())
	}
	if tl.Orientation() != kit.TimelineVertical {
		t.Fatalf("orient=%v want vertical", tl.Orientation())
	}
	if tl.Variant() != kit.TimelineOutlined {
		t.Fatalf("variant=%v want outlined", tl.Variant())
	}
	if tl.Reverse() {
		t.Fatal("reverse want false")
	}
	if tl.ItemCount() != 0 {
		t.Fatalf("items=%d", tl.ItemCount())
	}
	if !approxTL(tl.TitleSpan(), kit.DefaultTimelineTitleSpan, 0.5) {
		t.Fatalf("titleSpan=%v", tl.TitleSpan())
	}
	if tl.Node() == nil || tl.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	sz := tl.Node().Layout(core.Loose(400, 200))
	if sz.Width < 0 || sz.Height < 0 {
		t.Fatalf("size=%v", sz)
	}
}

func TestTimeline_PRD_02_ThreeItems(t *testing.T) {
	// TL-02 / TL-S1: 3 items → 3 节点
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "a"},
		kit.TimelineItem{Content: "b"},
		kit.TimelineItem{Content: "c"},
	)
	if tl.ItemCount() != 3 {
		t.Fatalf("count=%d", tl.ItemCount())
	}
	_ = tl.Node().Layout(core.Loose(400, 300))
	for i := 0; i < 3; i++ {
		if tl.ItemNode(i) == nil {
			t.Fatalf("ItemNode(%d) nil", i)
		}
	}
}

func TestTimeline_PRD_03_Alternate(t *testing.T) {
	// TL-03 / TL-S2: alternate → 左右交错
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "0"},
		kit.TimelineItem{Content: "1"},
		kit.TimelineItem{Content: "2"},
		kit.TimelineItem{Content: "3"},
	)
	tl.SetMode(kit.TimelineModeAlternate)
	if tl.Mode() != kit.TimelineModeAlternate {
		t.Fatal("mode")
	}
	if !tl.LayoutAlternate() {
		t.Fatal("LayoutAlternate false")
	}
	_ = tl.Node().Layout(core.Loose(500, 400))
	if tl.ItemPlacement(0) != kit.TimelinePlacementStart {
		t.Fatalf("p0=%v", tl.ItemPlacement(0))
	}
	if tl.ItemPlacement(1) != kit.TimelinePlacementEnd {
		t.Fatalf("p1=%v", tl.ItemPlacement(1))
	}
	if tl.ItemPlacement(2) != kit.TimelinePlacementStart {
		t.Fatalf("p2=%v", tl.ItemPlacement(2))
	}
	if tl.ItemPlacement(3) != kit.TimelinePlacementEnd {
		t.Fatalf("p3=%v", tl.ItemPlacement(3))
	}
}

func TestTimeline_PRD_04_PendingLoading(t *testing.T) {
	// TL-04 / TL-S3: 末项 loading
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Recording...", Loading: true},
	)
	if !tl.HasPending() {
		t.Fatal("HasPending false")
	}
	if !tl.ItemLoading(3) {
		t.Fatal("ItemLoading(3) false")
	}
	if !tl.HasLoadingSpinner() {
		t.Fatal("HasLoadingSpinner false")
	}
	_ = tl.Node().Layout(core.Loose(400, 300))
	tree := core.NewTree(tl.Node())
	tl.AttachTicker(tree)
	if !tl.Tick(0.016) {
		t.Fatal("Tick should continue while loading")
	}
}

func TestTimeline_PRD_05_Reverse(t *testing.T) {
	// TL-05 / TL-S4: reverse 倒序
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "first"},
		kit.TimelineItem{Content: "second"},
		kit.TimelineItem{Content: "third"},
	)
	tl.SetReverse(true)
	if !tl.Reverse() {
		t.Fatal("reverse false")
	}
	if tl.DisplayIndex(0) != 2 {
		t.Fatalf("display0→origin=%d want 2", tl.DisplayIndex(0))
	}
	if tl.DisplayIndex(2) != 0 {
		t.Fatalf("display2→origin=%d want 0", tl.DisplayIndex(2))
	}
	_ = tl.Node().Layout(core.Loose(400, 300))
}

func TestTimeline_PRD_06_ColorDots(t *testing.T) {
	// TL-06 / TL-S5: color 点
	th := kit.DefaultTheme()
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "blue", Color: "blue"},
		kit.TimelineItem{Content: "green", Color: "green"},
		kit.TimelineItem{Content: "red", Color: "red"},
		kit.TimelineItem{Content: "gray", Color: "gray"},
		kit.TimelineItem{Content: "custom", Color: "#722ED1"},
	)
	tl.SetTheme(th)
	_ = tl.Node().Layout(core.Loose(400, 400))
	if !approxTLColor(tl.ItemColor(0), th.Color(core.TokenColorPrimary), 0.02) {
		t.Fatalf("blue=%v want primary", tl.ItemColor(0))
	}
	if !approxTLColor(tl.ItemColor(1), th.Color(core.TokenColorSuccess), 0.02) {
		t.Fatalf("green=%v", tl.ItemColor(1))
	}
	if !approxTLColor(tl.ItemColor(2), th.Color(core.TokenColorError), 0.02) {
		t.Fatalf("red=%v", tl.ItemColor(2))
	}
	if !approxTLColor(tl.ItemColor(3), th.Color(core.TokenColorDisabledText), 0.02) {
		t.Fatalf("gray=%v", tl.ItemColor(3))
	}
	want := render.Hex("#722ED1")
	if !approxTLColor(tl.ItemColor(4), want, 0.02) {
		t.Fatalf("custom=%v want %v", tl.ItemColor(4), want)
	}
}

func TestTimeline_PRD_07_CustomIcon(t *testing.T) {
	// TL-07 / TL-S6: 自定义 icon
	ic := kit.NewIcon("sync")
	ic.SetSize(16)
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "a"},
		kit.TimelineItem{Content: "b", Icon: "sync"},
		kit.TimelineItem{Content: "c", IconNode: ic.Node()},
	)
	if !tl.ItemHasIcon(1) || !tl.ItemHasIcon(2) {
		t.Fatal("ItemHasIcon")
	}
	if tl.ItemHasIcon(0) {
		t.Fatal("item0 should be default dot")
	}
	_ = tl.Node().Layout(core.Loose(400, 300))
	if tl.DotDecorated(1) == nil || tl.DotDecorated(2) == nil {
		t.Fatal("dot chrome nil")
	}
}

func TestTimeline_PRD_08_BasicDemo(t *testing.T) {
	// TL-08: basic.tsx
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	)
	if tl.ItemCount() != 4 {
		t.Fatalf("count=%d", tl.ItemCount())
	}
	if tl.Mode() != kit.TimelineModeStart || tl.Variant() != kit.TimelineOutlined {
		t.Fatal("defaults")
	}
	sz := tl.Node().Layout(core.Loose(400, 400))
	if sz.Height < 40 {
		t.Fatalf("height=%v", sz.Height)
	}
}

func TestTimeline_PRD_09_VariantFilled(t *testing.T) {
	// TL-09: variant.tsx
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	)
	tl.SetVariant(kit.TimelineFilled)
	if tl.Variant() != kit.TimelineFilled {
		t.Fatal("variant")
	}
	_ = tl.Node().Layout(core.Loose(400, 400))
	dot := tl.DotDecorated(0)
	if dot == nil {
		t.Fatal("dot nil")
	}
	// filled → solid bg, no border
	if dot.BorderWidth > 0.5 {
		t.Fatalf("filled border=%v want 0", dot.BorderWidth)
	}
	if dot.Background.A < 0.5 {
		t.Fatalf("filled bg alpha=%v", dot.Background.A)
	}
}

func TestTimeline_PRD_10_PendingDemo(t *testing.T) {
	// TL-10: pending.tsx + reverse toggle
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Recording...", Loading: true},
	)
	_ = tl.Node().Layout(core.Loose(400, 300))
	if !tl.HasPending() || !tl.HasLoadingSpinner() {
		t.Fatal("pending loading")
	}
	tl.SetReverse(true)
	if tl.DisplayIndex(0) != 3 {
		t.Fatalf("reverse display0 origin=%d", tl.DisplayIndex(0))
	}
	// last origin still loading after reverse
	if !tl.ItemLoading(3) {
		t.Fatal("origin 3 still loading")
	}
	_ = tl.Node().Layout(core.Loose(400, 300))
}

func TestTimeline_PRD_11_AlternateDemo(t *testing.T) {
	// TL-11: alternate.tsx
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01", Color: "green"},
		kit.TimelineItem{Content: "long text", Icon: "sync"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01", Color: "red"},
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01", Icon: "sync"},
	)
	tl.SetMode(kit.TimelineModeAlternate)
	_ = tl.Node().Layout(core.Loose(600, 500))
	if tl.ItemPlacement(0) != kit.TimelinePlacementStart || tl.ItemPlacement(1) != kit.TimelinePlacementEnd {
		t.Fatal("alternate placements")
	}
	th := kit.DefaultTheme()
	if !approxTLColor(tl.ItemColor(1), th.Color(core.TokenColorSuccess), 0.02) {
		t.Fatal("green")
	}
	if !approxTLColor(tl.ItemColor(3), th.Color(core.TokenColorError), 0.02) {
		t.Fatal("red")
	}
}

func TestTimeline_PRD_12_Horizontal(t *testing.T) {
	// TL-12: horizontal.tsx
	items := []kit.TimelineItem{
		{Content: "Init"},
		{Content: "Start"},
		{Content: "Pending"},
		{Content: "Complete"},
	}
	for _, mode := range []kit.TimelineMode{kit.TimelineModeStart, kit.TimelineModeEnd, kit.TimelineModeAlternate} {
		tl := kit.NewTimeline(items...)
		tl.SetOrientation(kit.TimelineHorizontal)
		tl.SetMode(mode)
		if tl.Orientation() != kit.TimelineHorizontal {
			t.Fatal("orient")
		}
		sz := tl.Node().Layout(core.Loose(800, 200))
		if sz.Width < 10 {
			t.Fatalf("mode=%v size=%v", mode, sz)
		}
		if mode == kit.TimelineModeEnd {
			if tl.ItemPlacement(0) != kit.TimelinePlacementEnd {
				t.Fatalf("end placement=%v", tl.ItemPlacement(0))
			}
		}
		if mode == kit.TimelineModeAlternate {
			if tl.ItemPlacement(1) != kit.TimelinePlacementEnd {
				t.Fatal("alt odd")
			}
		}
	}
}

func TestTimeline_PRD_13_CustomDotDemo(t *testing.T) {
	// TL-13: custom.tsx
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01", Icon: "sync", Color: "red"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	)
	_ = tl.Node().Layout(core.Loose(400, 300))
	if !tl.ItemHasIcon(2) {
		t.Fatal("custom icon missing")
	}
	th := kit.DefaultTheme()
	if !approxTLColor(tl.ItemColor(2), th.Color(core.TokenColorError), 0.02) {
		t.Fatal("red color")
	}
}

func TestTimeline_PRD_14_ModeEnd(t *testing.T) {
	// TL-14: end.tsx
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01", Icon: "sync", Color: "red"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	)
	tl.SetMode(kit.TimelineModeEnd)
	_ = tl.Node().Layout(core.Loose(400, 300))
	for i := 0; i < 4; i++ {
		if tl.ItemPlacement(i) != kit.TimelinePlacementEnd {
			t.Fatalf("item %d placement=%v want end", i, tl.ItemPlacement(i))
		}
	}
}

func TestTimeline_PRD_15_TitleDemo(t *testing.T) {
	// TL-15: title.tsx
	tl := kit.NewTimeline(
		kit.TimelineItem{Title: "2015-09-01", Content: "Create a services"},
		kit.TimelineItem{Title: "2015-09-01 09:12:11", Content: "Solve initial network problems"},
		kit.TimelineItem{Content: "Technical testing"},
		kit.TimelineItem{Title: "2015-09-01 09:12:11", Content: "Network problems being solved"},
	)
	// vertical + title → layout alternate slots
	if !tl.LayoutAlternate() {
		t.Fatal("title should enable layout alternate")
	}
	for _, mode := range []kit.TimelineMode{kit.TimelineModeStart, kit.TimelineModeEnd, kit.TimelineModeAlternate} {
		tl.SetMode(mode)
		_ = tl.Node().Layout(core.Loose(500, 400))
		if tl.ItemCount() != 4 {
			t.Fatal("count")
		}
	}
	// explicit placement override
	tl2 := kit.NewTimeline(
		kit.TimelineItem{Content: "a", Placement: kit.TimelinePlacementEnd},
		kit.TimelineItem{Content: "b"},
	)
	tl2.SetMode(kit.TimelineModeStart)
	_ = tl2.Node().Layout(core.Loose(400, 200))
	if tl2.ItemPlacement(0) != kit.TimelinePlacementEnd {
		t.Fatal("placement override")
	}
	if tl2.ItemPlacement(1) != kit.TimelinePlacementStart {
		t.Fatal("default start")
	}
}

func TestTimeline_PRD_16_Metrics(t *testing.T) {
	// TL-16 / §6.2
	tl := kit.NewTimeline(kit.TimelineItem{Content: "x"})
	_ = tl.Node().Layout(core.Loose(300, 100))
	if !approxTL(tl.DotSize(), kit.DefaultTimelineDotSize, 0.5) {
		t.Fatalf("dot=%v", tl.DotSize())
	}
	if !approxTL(tl.TailWidth(), kit.DefaultTimelineTailWidth, 0.5) {
		t.Fatalf("tail=%v", tl.TailWidth())
	}
	if !approxTL(tl.ItemPaddingBottom(), kit.DefaultTimelineItemPadBottom, 0.5) {
		t.Fatalf("padBot=%v", tl.ItemPaddingBottom())
	}
	if !approxTL(tl.FontSize(), kit.DefaultTimelineFontSize, 0.5) {
		t.Fatalf("font=%v", tl.FontSize())
	}
	if !approxTL(tl.DotBorderWidth(), kit.DefaultTimelineDotBorder, 0.5) {
		t.Fatalf("dotBorder=%v", tl.DotBorderWidth())
	}
}

func TestTimeline_PRD_17_TokenColors(t *testing.T) {
	// TL-17: 默认皮走 Theme Token，无硬编码唯一品牌色
	th := kit.DefaultTheme()
	tl := kit.NewTimeline(kit.TimelineItem{Content: "x"})
	tl.SetTheme(th)
	_ = tl.Node().Layout(core.Loose(300, 100))
	got := tl.ItemColor(0)
	want := th.Color(core.TokenColorPrimary)
	if !approxTLColor(got, want, 0.02) {
		t.Fatalf("color=%v want token primary %v", got, want)
	}
	// custom theme primary
	custom := kit.DefaultTheme()
	if custom.Tokens != nil {
		custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#112233")
	}
	custom.ColorPrimary = render.Hex("#112233")
	tl2 := kit.NewTimeline(kit.TimelineItem{Content: "y"})
	tl2.SetTheme(custom)
	_ = tl2.Node().Layout(core.Loose(300, 100))
	if !approxTLColor(tl2.ItemColor(0), render.Hex("#112233"), 0.02) {
		t.Fatalf("custom primary not applied: %v", tl2.ItemColor(0))
	}
}

func TestTimeline_PRD_18_DisabledNA(t *testing.T) {
	// TL-18: Timeline 无 disabled API（antd items 亦无）— N/A smoke
	tl := kit.NewTimeline(kit.TimelineItem{Content: "x"})
	_ = tl.Node().Layout(core.Loose(300, 100))
	if tl.ItemCount() != 1 {
		t.Fatal("smoke")
	}
}

func TestTimeline_PRD_19_A11yList(t *testing.T) {
	// TL-19: list / listitem 结构（非焦点控件；无 keyboard 主路径）
	tl := kit.NewTimeline(
		kit.TimelineItem{Content: "a"},
		kit.TimelineItem{Content: "b"},
	)
	tl.SetAriaLabel("project timeline")
	root := tl.Node()
	_ = root.Layout(core.Loose(400, 200))
	// host → list
	if tl.ChromeNode() == nil {
		t.Fatal("chrome")
	}
	// walk: root children include list with role=list
	foundList := false
	var walk func(n core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if n.Base() != nil && n.Base().Role == "list" {
			foundList = true
			if n.Base().Label != "project timeline" {
				t.Fatalf("list label=%q", n.Base().Label)
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	if !foundList {
		t.Fatal("role=list not found")
	}
	// item roles
	if n := tl.ItemNode(0); n == nil || n.Base() == nil || n.Base().Role != "listitem" {
		t.Fatalf("item role=%v", n)
	}
}

func TestTimeline_PRD_OutlinedBorder(t *testing.T) {
	// outlined default: hollow stroke
	tl := kit.NewTimeline(kit.TimelineItem{Content: "x"})
	_ = tl.Node().Layout(core.Loose(300, 100))
	dot := tl.DotDecorated(0)
	if dot == nil {
		t.Fatal("nil")
	}
	if !approxTL(dot.BorderWidth, kit.DefaultTimelineDotBorder, 0.5) {
		t.Fatalf("outlined border=%v", dot.BorderWidth)
	}
}

func TestTimeline_PRD_ContentNode(t *testing.T) {
	box := primitive.NewBox()
	box.Width, box.Height = 40, 20
	box.Color = render.Hex("#E6F4FF")
	tl := kit.NewTimeline(kit.TimelineItem{ContentNode: box})
	_ = tl.Node().Layout(core.Loose(300, 100))
	if tl.ItemNode(0) == nil {
		t.Fatal("nil item")
	}
}
