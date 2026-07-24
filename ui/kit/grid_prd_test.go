package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/grid.md §6.9 — P0 PRD cases (GRD-01 … GRD-19 L1/L2).
// L3/L4 (GRD-20/21) and P1 (GRD-22) deferred; GRD-18/19 N/A for layout-only.

func approxGrid(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func gridBox(w, h float64) core.Node {
	b := primitive.NewBox()
	b.Width, b.Height = w, h
	return b
}

func colNode(c *kit.Col) core.Node { return c.Node() }

func TestGrid_PRD_01_Defaults(t *testing.T) {
	// GRD-01: NewGrid 默认创建
	r := kit.NewGrid()
	if r.Node() == nil {
		t.Fatal("nil node")
	}
	if r.Align != kit.RowAlignTop {
		t.Fatalf("align=%v want top", r.Align)
	}
	if r.Justify != kit.RowJustifyStart {
		t.Fatalf("justify=%v want start", r.Justify)
	}
	if !r.Wrap {
		t.Fatal("wrap default true")
	}
	if r.GutterH != 0 || r.GutterV != 0 {
		t.Fatalf("gutter=%v,%v want 0", r.GutterH, r.GutterV)
	}
	// NewRow same defaults
	r2 := kit.NewRow()
	if !r2.Wrap || r2.Align != kit.RowAlignTop {
		t.Fatal("NewRow defaults")
	}
	if kit.GridColumns != 24 {
		t.Fatalf("GridColumns=%d", kit.GridColumns)
	}
}

func TestGrid_PRD_02_Span12_12(t *testing.T) {
	// GRD-02 / GRD-S1: span=12+12 → 各 50%
	c1, c2 := kit.NewCol(gridBox(10, 20)), kit.NewCol(gridBox(10, 20))
	c1.SetSpan(12)
	c2.SetSpan(12)
	r := kit.NewRow(colNode(c1), colNode(c2))
	_ = r.Node().Layout(core.Loose(400, 100))
	if !approxGrid(c1.Node().Base().Size().Width, 200, 0.5) {
		t.Fatalf("c1 w=%v want 200", c1.Node().Base().Size().Width)
	}
	if !approxGrid(c2.Node().Base().Size().Width, 200, 0.5) {
		t.Fatalf("c2 w=%v want 200", c2.Node().Base().Size().Width)
	}
	if !approxGrid(c2.Node().Base().Offset().X, 200, 0.5) {
		t.Fatalf("c2 x=%v want 200", c2.Node().Base().Offset().X)
	}
}

func TestGrid_PRD_03_Span8x3(t *testing.T) {
	// GRD-03 / GRD-S2: span=8×3 → 各约 33%
	const W = 300.0
	cols := make([]*kit.Col, 3)
	nodes := make([]core.Node, 3)
	for i := range cols {
		cols[i] = kit.NewCol(gridBox(4, 16))
		cols[i].SetSpan(8)
		nodes[i] = colNode(cols[i])
	}
	r := kit.NewRow(nodes...)
	_ = r.Node().Layout(core.Loose(W, 80))
	want := W / 3
	for i, c := range cols {
		if !approxGrid(c.Node().Base().Size().Width, want, 0.5) {
			t.Fatalf("col%d w=%v want %v", i, c.Node().Base().Size().Width, want)
		}
		if !approxGrid(c.Node().Base().Offset().X, want*float64(i), 0.5) {
			t.Fatalf("col%d x=%v want %v", i, c.Node().Base().Offset().X, want*float64(i))
		}
	}
}

func TestGrid_PRD_04_Offset6(t *testing.T) {
	// GRD-04 / GRD-S3: offset=6 → 左空 6 格
	const W = 480.0
	unit := W / 24
	c := kit.NewCol(gridBox(8, 20))
	c.SetSpan(12)
	c.SetOffset(6)
	r := kit.NewRow(colNode(c))
	_ = r.Node().Layout(core.Loose(W, 60))
	wantX := 6 * unit
	wantW := 12 * unit
	if !approxGrid(c.Node().Base().Offset().X, wantX, 0.5) {
		t.Fatalf("x=%v want %v", c.Node().Base().Offset().X, wantX)
	}
	if !approxGrid(c.Node().Base().Size().Width, wantW, 0.5) {
		t.Fatalf("w=%v want %v", c.Node().Base().Size().Width, wantW)
	}
}

func TestGrid_PRD_05_Gutter16(t *testing.T) {
	// GRD-05 / GRD-S4: gutter=16 → 列间隙（内容区间距）
	// two span-12 cols; outer adjacent, content inset padH=8 each → gap 16
	const W = 400.0
	mk := func() (*kit.Col, *primitive.Box) {
		b := primitive.NewBox()
		b.Width, b.Height = 10, 20
		c := kit.NewCol(b)
		c.SetSpan(12)
		return c, b
	}
	c1, b1 := mk()
	c2, b2 := mk()
	r := kit.NewRow(colNode(c1), colNode(c2))
	r.SetGutter(16)
	_ = r.Node().Layout(core.Loose(W, 80))
	x1 := c1.Node().Base().Offset().X + b1.Base().Offset().X
	x2 := c2.Node().Base().Offset().X + b2.Base().Offset().X
	cw1 := c1.Node().Base().Size().Width - 2*b1.Base().Offset().X
	gap := x2 - (x1 + cw1)
	if !approxGrid(gap, 16, 1.0) {
		t.Fatalf("content gap=%v want 16 (x1=%v x2=%v cw1=%v pad=%v)", gap, x1, x2, cw1, b1.Base().Offset().X)
	}
	if !approxGrid(b1.Base().Offset().X, 8, 0.5) {
		t.Fatalf("padH=%v want 8", b1.Base().Offset().X)
	}
}

func TestGrid_PRD_06_ResponsiveMdXs(t *testing.T) {
	// GRD-06 / GRD-S5: 响应 md=12 xs=24 断点切换
	c1 := kit.NewCol(gridBox(8, 16))
	c2 := kit.NewCol(gridBox(8, 16))
	c1.SetXs(24)
	c1.SetMd(12)
	c2.SetXs(24)
	c2.SetMd(12)
	r := kit.NewRow(colNode(c1), colNode(c2))

	// xs viewport
	r.SetViewportWidth(400)
	_ = r.Node().Layout(core.Loose(400, 100))
	if !approxGrid(c1.Node().Base().Size().Width, 400, 0.5) {
		t.Fatalf("xs c1 w=%v want 400", c1.Node().Base().Size().Width)
	}
	// second wraps below when wrap (default) and each takes full width
	if c2.Node().Base().Offset().Y < 1 {
		// if same row would be wrong for 24+24
		t.Fatalf("xs expected wrap, c2.y=%v", c2.Node().Base().Offset().Y)
	}

	// md viewport
	r.SetViewportWidth(800)
	_ = r.Node().Layout(core.Loose(800, 100))
	if !approxGrid(c1.Node().Base().Size().Width, 400, 0.5) {
		t.Fatalf("md c1 w=%v want 400", c1.Node().Base().Size().Width)
	}
	if !approxGrid(c2.Node().Base().Offset().X, 400, 0.5) {
		t.Fatalf("md c2 x=%v want 400", c2.Node().Base().Offset().X)
	}
	if c2.Node().Base().Offset().Y > 0.5 {
		t.Fatalf("md expected same row, c2.y=%v", c2.Node().Base().Offset().Y)
	}
}

func TestGrid_PRD_07_Wrap(t *testing.T) {
	// GRD-07 / GRD-S6: wrap
	const W = 240.0 // 10 units per col of span 12 → 2 per line
	cols := make([]*kit.Col, 3)
	nodes := make([]core.Node, 3)
	for i := range cols {
		cols[i] = kit.NewCol(gridBox(4, 20))
		cols[i].SetSpan(12)
		nodes[i] = colNode(cols[i])
	}
	r := kit.NewRow(nodes...)
	r.SetWrap(true)
	sz := r.Node().Layout(core.Loose(W, 400))
	if cols[2].Node().Base().Offset().Y < 10 {
		t.Fatalf("third should wrap, y=%v", cols[2].Node().Base().Offset().Y)
	}
	if sz.Height < 39 {
		t.Fatalf("height=%v want >=40", sz.Height)
	}
	// wrap=false: stay one line (overflow ok)
	r.SetWrap(false)
	_ = r.Node().Layout(core.Loose(W, 400))
	if cols[2].Node().Base().Offset().Y > 0.5 {
		t.Fatalf("nowrap should stay row0, y=%v", cols[2].Node().Base().Offset().Y)
	}
}

func TestGrid_PRD_08_BasicDemo(t *testing.T) {
	// GRD-08: basic.tsx — 24 / 12+12 / 8×3 / 6×4
	const W = 480.0
	row := func(spans ...int) *kit.Row {
		nodes := make([]core.Node, len(spans))
		for i, s := range spans {
			c := kit.NewCol(gridBox(4, 16))
			c.SetSpan(s)
			nodes[i] = colNode(c)
		}
		return kit.NewRow(nodes...)
	}
	for _, tc := range []struct {
		spans []int
		n     int
	}{
		{[]int{24}, 1},
		{[]int{12, 12}, 2},
		{[]int{8, 8, 8}, 3},
		{[]int{6, 6, 6, 6}, 4},
	} {
		r := row(tc.spans...)
		_ = r.Node().Layout(core.Loose(W, 80))
		kids := r.Node().Base().Children()
		if len(kids) != tc.n {
			t.Fatalf("spans=%v kids=%d", tc.spans, len(kids))
		}
		unit := W / 24
		x := 0.0
		for i, s := range tc.spans {
			w := float64(s) * unit
			if !approxGrid(kids[i].Base().Size().Width, w, 0.5) {
				t.Fatalf("spans=%v i=%d w=%v want %v", tc.spans, i, kids[i].Base().Size().Width, w)
			}
			if !approxGrid(kids[i].Base().Offset().X, x, 0.5) {
				t.Fatalf("spans=%v i=%d x=%v want %v", tc.spans, i, kids[i].Base().Offset().X, x)
			}
			x += w
		}
	}
}

func TestGrid_PRD_09_GutterDemo(t *testing.T) {
	// GRD-09: gutter.tsx — horizontal 16 + vertical [16,24]
	const W = 400.0
	mk := func() *kit.Col {
		c := kit.NewCol(gridBox(8, 24))
		c.SetSpan(6)
		return c
	}
	r := kit.NewRow(colNode(mk()), colNode(mk()), colNode(mk()), colNode(mk()))
	r.SetGutter(16)
	_ = r.Node().Layout(core.Loose(W, 100))
	// 4×span6 fill row
	if !approxGrid(r.Node().Base().Children()[3].Base().Offset().X, W*0.75, 1) {
		t.Fatalf("4th col x=%v", r.Node().Base().Children()[3].Base().Offset().X)
	}

	// vertical gutter: 8 cols span6 → 2 lines, gap 24
	nodes := make([]core.Node, 8)
	for i := range nodes {
		nodes[i] = colNode(mk())
	}
	r2 := kit.NewRow(nodes...)
	r2.SetGutterHV(16, 24)
	_ = r2.Node().Layout(core.Loose(W, 400))
	// line0 y=0 height~24; line1 y=24+24=48
	y1 := r2.Node().Base().Children()[4].Base().Offset().Y
	if !approxGrid(y1, 24+24, 2) {
		t.Fatalf("line1 y=%v want 48", y1)
	}
}

func TestGrid_PRD_10_OffsetDemo(t *testing.T) {
	// GRD-10: offset.tsx
	const W = 480.0
	unit := W / 24
	// Row: span8 + span8 offset8
	a, b := kit.NewCol(gridBox(4, 16)), kit.NewCol(gridBox(4, 16))
	a.SetSpan(8)
	b.SetSpan(8)
	b.SetOffset(8)
	r := kit.NewRow(colNode(a), colNode(b))
	_ = r.Node().Layout(core.Loose(W, 60))
	if !approxGrid(b.Node().Base().Offset().X, 8*unit+8*unit, 0.5) {
		// flow: a takes 8, then offset 8 + span 8 → x = 16 units
		t.Fatalf("b.x=%v want %v", b.Node().Base().Offset().X, 16*unit)
	}
}

func TestGrid_PRD_11_SortPushPull(t *testing.T) {
	// GRD-11: sort.tsx — span18 push6 + span6 pull18 → visual swap
	const W = 480.0
	unit := W / 24
	a, b := kit.NewCol(gridBox(4, 16)), kit.NewCol(gridBox(4, 16))
	a.SetSpan(18)
	a.SetPush(6)
	b.SetSpan(6)
	b.SetPull(18)
	r := kit.NewRow(colNode(a), colNode(b))
	_ = r.Node().Layout(core.Loose(W, 60))
	// a flow at 0, visual +6u; b flow at 18u, visual -18u → 0
	if !approxGrid(a.Node().Base().Offset().X, 6*unit, 0.5) {
		t.Fatalf("push a.x=%v want %v", a.Node().Base().Offset().X, 6*unit)
	}
	if !approxGrid(b.Node().Base().Offset().X, 0, 0.5) {
		t.Fatalf("pull b.x=%v want 0", b.Node().Base().Offset().X)
	}
}

func TestGrid_PRD_12_JustifyDemo(t *testing.T) {
	// GRD-12: flex.tsx justify variants with span4×4
	const W = 480.0
	unit := W / 24
	mk := func() []core.Node {
		ns := make([]core.Node, 4)
		for i := range ns {
			c := kit.NewCol(gridBox(4, 20))
			c.SetSpan(4)
			ns[i] = colNode(c)
		}
		return ns
	}
	// start: first at 0
	r := kit.NewRow(mk()...)
	r.SetJustify(kit.RowJustifyStart)
	_ = r.Node().Layout(core.Loose(W, 40))
	if !approxGrid(r.Node().Base().Children()[0].Base().Offset().X, 0, 0.5) {
		t.Fatal("start")
	}
	// end: last ends at W
	r = kit.NewRow(mk()...)
	r.SetJustify(kit.RowJustifyEnd)
	_ = r.Node().Layout(core.Loose(W, 40))
	last := r.Node().Base().Children()[3]
	if !approxGrid(last.Base().Offset().X+last.Base().Size().Width, W, 1) {
		t.Fatalf("end last right=%v", last.Base().Offset().X+last.Base().Size().Width)
	}
	// center
	r = kit.NewRow(mk()...)
	r.SetJustify(kit.RowJustifyCenter)
	_ = r.Node().Layout(core.Loose(W, 40))
	// total content 16 units; free 8 units → leading 4 units
	if !approxGrid(r.Node().Base().Children()[0].Base().Offset().X, 4*unit, 1) {
		t.Fatalf("center x0=%v want %v", r.Node().Base().Children()[0].Base().Offset().X, 4*unit)
	}
	// space-between: first 0 last end
	r = kit.NewRow(mk()...)
	r.SetJustify(kit.RowJustifySpaceBetween)
	_ = r.Node().Layout(core.Loose(W, 40))
	if !approxGrid(r.Node().Base().Children()[0].Base().Offset().X, 0, 0.5) {
		t.Fatal("sb first")
	}
	last = r.Node().Base().Children()[3]
	if !approxGrid(last.Base().Offset().X+last.Base().Size().Width, W, 1) {
		t.Fatal("sb last")
	}
}

func TestGrid_PRD_13_AlignDemo(t *testing.T) {
	// GRD-13: flex-align.tsx — align middle
	const W = 400.0
	tall := kit.NewCol(gridBox(10, 80))
	short := kit.NewCol(gridBox(10, 20))
	tall.SetSpan(8)
	short.SetSpan(8)
	r := kit.NewRow(colNode(tall), colNode(short))
	r.SetAlign(kit.RowAlignMiddle)
	r.SetJustify(kit.RowJustifyStart)
	_ = r.Node().Layout(core.Loose(W, 200))
	// short centered against 80: y ≈ 30
	if !approxGrid(short.Node().Base().Offset().Y, 30, 1.5) {
		t.Fatalf("middle y=%v want ~30", short.Node().Base().Offset().Y)
	}
	r.SetAlign(kit.RowAlignBottom)
	_ = r.Node().Layout(core.Loose(W, 200))
	if !approxGrid(short.Node().Base().Offset().Y, 60, 1.5) {
		t.Fatalf("bottom y=%v want ~60", short.Node().Base().Offset().Y)
	}
}

func TestGrid_PRD_14_OrderDemo(t *testing.T) {
	// GRD-14: flex-order.tsx — order 4,3,2,1 reverses visual
	const W = 400.0
	cols := make([]*kit.Col, 4)
	nodes := make([]core.Node, 4)
	for i := 0; i < 4; i++ {
		cols[i] = kit.NewCol(gridBox(4, 16))
		cols[i].SetSpan(6)
		cols[i].SetOrder(4 - i) // 4,3,2,1
		nodes[i] = colNode(cols[i])
	}
	r := kit.NewRow(nodes...)
	_ = r.Node().Layout(core.Loose(W, 40))
	// visual left-to-right: order1 (cols[3]), order2 (cols[2]), ...
	if cols[3].Node().Base().Offset().X > cols[0].Node().Base().Offset().X {
		// cols[3] order=1 should be leftmost
		t.Fatalf("order: col3(order1) x=%v col0(order4) x=%v", cols[3].Node().Base().Offset().X, cols[0].Node().Base().Offset().X)
	}
	if !approxGrid(cols[3].Node().Base().Offset().X, 0, 0.5) {
		t.Fatalf("order1 x=%v", cols[3].Node().Base().Offset().X)
	}
	if !approxGrid(cols[0].Node().Base().Offset().X, W*0.75, 1) {
		t.Fatalf("order4 x=%v want %v", cols[0].Node().Base().Offset().X, W*0.75)
	}
}

func TestGrid_PRD_15_FlexStretchDemo(t *testing.T) {
	// GRD-15: flex-stretch.tsx — flex 2/3 and auto
	const W = 500.0
	a, b := kit.NewCol(gridBox(10, 20)), kit.NewCol(gridBox(10, 20))
	a.SetFlexNumber(2)
	b.SetFlexNumber(3)
	r := kit.NewRow(colNode(a), colNode(b))
	_ = r.Node().Layout(core.Loose(W, 40))
	// flex: n n auto → basis=content then grow free space. Equal content → ~2:3 of free+basis.
	// With tiny equal bases, ratio is near 2/5 : 3/5 of row.
	aw, bw := a.Node().Base().Size().Width, b.Node().Base().Size().Width
	if aw+bw < W-2 || aw >= bw {
		t.Fatalf("flex 2/3 widths a=%v b=%v sum=%v want a<b ~2:3 of %v", aw, bw, aw+bw, W)
	}
	if !approxGrid(aw/bw, 2.0/3.0, 0.15) {
		t.Fatalf("flex ratio a/b=%v want ~2/3 (a=%v b=%v)", aw/bw, aw, bw)
	}

	// 100px + auto
	c1, c2 := kit.NewCol(gridBox(10, 20)), kit.NewCol(gridBox(10, 20))
	c1.SetFlexString("100px")
	c2.SetFlexAuto()
	r2 := kit.NewRow(colNode(c1), colNode(c2))
	_ = r2.Node().Layout(core.Loose(W, 40))
	if !approxGrid(c1.Node().Base().Size().Width, 100, 1) {
		t.Fatalf("100px w=%v", c1.Node().Base().Size().Width)
	}
	if !approxGrid(c2.Node().Base().Size().Width, W-100, 1.5) {
		t.Fatalf("auto w=%v want %v", c2.Node().Base().Size().Width, W-100)
	}
}

func TestGrid_PRD_16_MetricsToken(t *testing.T) {
	// GRD-16: §6.2 关键数字
	if kit.GridColumns != 24 {
		t.Fatal(kit.GridColumns)
	}
	if !approxGrid(kit.DefaultGridFontSize, 14, 0.01) {
		t.Fatal(kit.DefaultGridFontSize)
	}
	if !approxGrid(kit.DefaultGridBorderRadius, 6, 0.01) {
		t.Fatal(kit.DefaultGridBorderRadius)
	}
	if !approxGrid(kit.DefaultGridLineWidth, 1, 0.01) {
		t.Fatal(kit.DefaultGridLineWidth)
	}
	// breakpoints
	if kit.ScreenSM != 576 || kit.ScreenMD != 768 || kit.ScreenLG != 992 {
		t.Fatalf("breakpoints sm=%v md=%v lg=%v", kit.ScreenSM, kit.ScreenMD, kit.ScreenLG)
	}
}

func TestGrid_PRD_17_NoHardcodedBrandSkin(t *testing.T) {
	// GRD-17: 默认皮无硬编码品牌色
	r := kit.NewRow(colNode(kit.NewCol(gridBox(20, 20))))
	_ = r.Node()
	primary := kit.DefaultTheme().Color(core.TokenColorPrimary)
	if primary.A == 0 {
		t.Fatal("theme primary missing")
	}
	// Row root is layout-only (kit.Row type id)
	if r.Node().TypeID() != "kit.Row" {
		t.Fatalf("type=%s", r.Node().TypeID())
	}
}

func TestGrid_PRD_18_DisabledN_A(t *testing.T) {
	// GRD-18: disabled — 布局容器不适用
	r := kit.NewGrid(colNode(kit.NewCol(gridBox(20, 20))))
	_ = r.Node().Layout(core.Loose(100, 40))
	if r.Node() == nil {
		t.Fatal("nil")
	}
}

func TestGrid_PRD_19_KeyboardN_A(t *testing.T) {
	// GRD-19: 键盘/焦点 — 布局容器不聚焦
	r := kit.NewRow()
	r.SetAriaLabel("grid region")
	_ = r.Node()
	if r.Node().Base().Label != "grid region" {
		t.Fatalf("label=%q", r.Node().Base().Label)
	}
	if r.Node().Base().Role != "" {
		t.Fatalf("role=%q want empty", r.Node().Base().Role)
	}
}

func TestGrid_PRD_HitEqualsLayout(t *testing.T) {
	c1, c2 := kit.NewCol(gridBox(10, 20)), kit.NewCol(gridBox(10, 20))
	c1.SetSpan(12)
	c2.SetSpan(12)
	r := kit.NewRow(colNode(c1), colNode(c2))
	n := r.Node()
	sz := n.Layout(core.Loose(400, 100))
	if !approxGrid(sz.Width, 400, 0.5) {
		t.Fatalf("w=%v want 400", sz.Width)
	}
	tree := core.NewTree(n)
	tree.Layout(core.Size{Width: 400, Height: 100})
	if tree.HitTest(core.Point{X: 10, Y: 10}) == nil {
		t.Fatal("expected hit")
	}
}

func TestGrid_PRD_Span0Hidden(t *testing.T) {
	c1, c2 := kit.NewCol(gridBox(10, 20)), kit.NewCol(gridBox(10, 20))
	c1.SetSpan(0)
	c2.SetSpan(12)
	r := kit.NewRow(colNode(c1), colNode(c2))
	_ = r.Node().Layout(core.Loose(400, 40))
	if c1.Node().Base().Size().Width > 0.5 {
		t.Fatalf("hidden col w=%v", c1.Node().Base().Size().Width)
	}
}
