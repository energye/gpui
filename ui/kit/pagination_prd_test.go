package kit_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/pagination.md §6.9 — P0 PRD cases (PG-01 … PG-23 L1/L2).
// L3/L4 (PG-24/25) and P1 (PG-26) deferred.

func approxPag(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func approxPagColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(float64(a.R-b.R)) <= tol &&
		math.Abs(float64(a.G-b.G)) <= tol &&
		math.Abs(float64(a.B-b.B)) <= tol &&
		math.Abs(float64(a.A-b.A)) <= tol
}

func clickPagItem(t *testing.T, tree *core.Tree, p *kit.Pagination, kind kit.PaginationItemKind, page int) {
	t.Helper()
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr := p.ItemPressable(kind, page)
	if pr == nil {
		t.Fatalf("no pressable kind=%s page=%d", kind, page)
	}
	abs := core.AbsoluteBounds(pr)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	if abs.Width() <= 0 || abs.Height() <= 0 {
		t.Fatalf("empty bounds kind=%s page=%d", kind, page)
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestPagination_PRD_01_Defaults(t *testing.T) {
	// PG-01
	p := kit.NewPagination()
	if p.Node() == nil {
		t.Fatal("nil node")
	}
	if p.Current != 1 {
		t.Fatalf("Current=%d want 1", p.Current)
	}
	if p.PageSize != 10 {
		t.Fatalf("PageSize=%d want 10", p.PageSize)
	}
	if p.Total != 0 {
		t.Fatalf("Total=%d want 0", p.Total)
	}
	if p.PageCount() != 1 {
		t.Fatalf("PageCount=%d want 1", p.PageCount())
	}
	if p.Disabled {
		t.Fatal("Disabled default false")
	}
	if p.Size != kit.PaginationMiddle {
		t.Fatalf("Size=%v want middle", p.Size)
	}
	if p.Align != kit.PaginationAlignStart {
		t.Fatalf("Align=%v want start", p.Align)
	}
	if p.ShowQuickJumper || p.Simple || p.HideOnSinglePage {
		t.Fatal("flags should be false")
	}
	if p.ChromeNode().Base().Role != "navigation" {
		t.Fatalf("role=%q want navigation", p.ChromeNode().Base().Role)
	}
}

func TestPagination_PRD_02_ClickPage2(t *testing.T) {
	// PG-02 / PG-S1
	p := kit.NewPagination()
	p.SetTotal(50)
	var gotPage, gotSize, n int
	p.SetOnChange(func(page, pageSize int) {
		n++
		gotPage, gotSize = page, pageSize
	})
	tree := core.NewTree(p.Node())
	clickPagItem(t, tree, p, kit.PaginationItemPage, 2)
	if p.Current != 2 {
		t.Fatalf("Current=%d want 2", p.Current)
	}
	if n != 1 || gotPage != 2 || gotSize != 10 {
		t.Fatalf("onChange n=%d page=%d size=%d", n, gotPage, gotSize)
	}
}

func TestPagination_PRD_03_PrevOnFirst(t *testing.T) {
	// PG-03 / PG-S2
	p := kit.NewPagination()
	p.SetTotal(50)
	n := 0
	p.SetOnChange(func(page, pageSize int) { n++ })
	tree := core.NewTree(p.Node())
	pr := p.ItemPressable(kit.PaginationItemPrev, 0)
	if pr == nil {
		t.Fatal("nil prev")
	}
	if !pr.State.Disabled {
		t.Fatal("prev should be disabled on first page")
	}
	tree.Layout(core.Size{Width: 900, Height: 80})
	abs := core.AbsoluteBounds(pr)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if p.Current != 1 || n != 0 {
		t.Fatalf("Current=%d n=%d", p.Current, n)
	}
}

func TestPagination_PRD_04_NextOnLast(t *testing.T) {
	// PG-04 / PG-S3
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetDefaultCurrent(5)
	n := 0
	p.SetOnChange(func(page, pageSize int) { n++ })
	tree := core.NewTree(p.Node())
	pr := p.ItemPressable(kit.PaginationItemNext, 0)
	if pr == nil || !pr.State.Disabled {
		t.Fatal("next should be disabled on last page")
	}
	tree.Layout(core.Size{Width: 900, Height: 80})
	abs := core.AbsoluteBounds(pr)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if p.Current != 5 || n != 0 {
		t.Fatalf("Current=%d n=%d", p.Current, n)
	}
}

func TestPagination_PRD_05_PageSizeChange(t *testing.T) {
	// PG-05 / PG-S4
	p := kit.NewPagination()
	p.SetTotal(500)
	p.SetShowSizeChanger(true)
	var sizeN, changeN, lastSize int
	p.SetOnShowSizeChange(func(current, size int) {
		sizeN++
		lastSize = size
	})
	p.SetOnChange(func(page, pageSize int) {
		changeN++
		lastSize = pageSize
	})
	_ = p.Node()
	sel := p.SizeSelect()
	if sel == nil {
		t.Fatal("SizeSelect nil")
	}
	sel.SetValue("20")
	if sizeN < 1 {
		t.Fatalf("onShowSizeChange n=%d", sizeN)
	}
	if changeN < 1 {
		t.Fatalf("onChange n=%d", changeN)
	}
	if lastSize != 20 {
		t.Fatalf("lastSize=%d", lastSize)
	}
	if p.PageSize != 20 {
		t.Fatalf("PageSize=%d want 20", p.PageSize)
	}
	if p.PageCount() != 25 {
		t.Fatalf("PageCount=%d want 25", p.PageCount())
	}
}

func TestPagination_PRD_06_QuickJumper(t *testing.T) {
	// PG-06 / PG-S5
	p := kit.NewPagination()
	p.SetTotal(500)
	p.SetShowQuickJumper(true)
	var got int
	p.SetOnChange(func(page, pageSize int) { got = page })
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 900, Height: 80})
	in := p.JumperInput()
	if in == nil {
		t.Fatal("JumperInput nil")
	}
	in.SetValue("7")
	if in.OnPressEnter != nil {
		in.OnPressEnter("7")
	} else {
		t.Fatal("OnPressEnter nil")
	}
	if got != 7 || p.Current != 7 {
		t.Fatalf("Current=%d got=%d want 7", p.Current, got)
	}
	_ = tree
}

func TestPagination_PRD_07_Disabled(t *testing.T) {
	// PG-07 / PG-S6
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetDisabled(true)
	n := 0
	p.SetOnChange(func(page, pageSize int) { n++ })
	tree := core.NewTree(p.Node())
	pr := p.ItemPressable(kit.PaginationItemPage, 2)
	if pr == nil {
		t.Fatal("nil page 2")
	}
	if !pr.State.Disabled {
		t.Fatal("page item should be disabled")
	}
	tree.Layout(core.Size{Width: 900, Height: 80})
	abs := core.AbsoluteBounds(pr)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if p.Current != 1 || n != 0 {
		t.Fatalf("Current=%d n=%d", p.Current, n)
	}
}

func TestPagination_PRD_08_Simple(t *testing.T) {
	// PG-08 / PG-S7
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetSimple(true)
	p.SetDefaultCurrent(2)
	if !p.Simple {
		t.Fatal("Simple")
	}
	_ = p.Node()
	if p.ItemPressable(kit.PaginationItemPage, 3) != nil {
		t.Fatal("simple should not build numbered page items")
	}
	if p.ItemPressable(kit.PaginationItemPrev, 0) == nil || p.ItemPressable(kit.PaginationItemNext, 0) == nil {
		t.Fatal("simple should have prev/next")
	}
	tree := core.NewTree(p.Node())
	clickPagItem(t, tree, p, kit.PaginationItemNext, 0)
	if p.Current != 3 {
		t.Fatalf("Current=%d want 3", p.Current)
	}
}

func TestPagination_PRD_09_MiddleHeight(t *testing.T) {
	// PG-09 / PG-S8
	p := kit.NewPagination()
	p.SetTotal(50)
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr := p.ItemPressable(kit.PaginationItemPage, 1)
	if pr == nil {
		t.Fatal("nil page1")
	}
	if !approxPag(pr.Size().Height, 32, 0.5) {
		t.Fatalf("item height=%v want 32±0.5", pr.Size().Height)
	}
}

func TestPagination_PRD_10_TotalZero(t *testing.T) {
	// PG-10 / PG-S9
	p := kit.NewPagination()
	if p.PageCount() != 1 {
		t.Fatalf("PageCount=%d", p.PageCount())
	}
	if p.Current != 1 {
		t.Fatalf("Current=%d", p.Current)
	}
	start, end := p.Range()
	if start != 0 || end != 0 {
		t.Fatalf("Range=%d-%d want 0-0", start, end)
	}
	if p.Node() == nil {
		t.Fatal("nil")
	}
}

func TestPagination_PRD_11_ShowTotal(t *testing.T) {
	// PG-11 / PG-S10
	p := kit.NewPagination()
	p.SetTotal(85)
	p.SetDefaultPageSize(20)
	var seen string
	p.SetShowTotal(func(total, start, end int) string {
		seen = strconv.Itoa(total)
		return "Total " + seen + " items"
	})
	root := p.Node()
	if len(root.Children()) < 1 {
		t.Fatal("no children")
	}
	tx, ok := root.Children()[0].(*primitive.Text)
	if !ok {
		t.Fatalf("first child %T want Text", root.Children()[0])
	}
	if !strings.Contains(tx.Value, "85") {
		t.Fatalf("showTotal text=%q", tx.Value)
	}
	if seen != "85" {
		t.Fatalf("seen=%q", seen)
	}
}

func TestPagination_PRD_12_BasicDemo(t *testing.T) {
	// PG-12 basic.tsx
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetDefaultCurrent(1)
	if p.PageCount() != 5 {
		t.Fatalf("pages=%d", p.PageCount())
	}
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 600, Height: 60})
	if p.ItemPressable(kit.PaginationItemPage, 1) == nil {
		t.Fatal("missing page 1")
	}
}

func TestPagination_PRD_13_AlignDemo(t *testing.T) {
	// PG-13 align.tsx
	for _, a := range []kit.PaginationAlign{kit.PaginationAlignStart, kit.PaginationAlignCenter, kit.PaginationAlignEnd} {
		p := kit.NewPagination()
		p.SetTotal(50)
		p.SetAlign(a)
		_ = p.Node()
		switch a {
		case kit.PaginationAlignCenter:
			if p.Root.MainAlign != core.MainCenter {
				t.Fatalf("center MainAlign=%v", p.Root.MainAlign)
			}
		case kit.PaginationAlignEnd:
			if p.Root.MainAlign != core.MainEnd {
				t.Fatalf("end MainAlign=%v", p.Root.MainAlign)
			}
		default:
			if p.Root.MainAlign != core.MainStart {
				t.Fatalf("start MainAlign=%v", p.Root.MainAlign)
			}
		}
	}
}

func TestPagination_PRD_14_MoreDemo(t *testing.T) {
	// PG-14 more.tsx
	p := kit.NewPagination()
	p.SetTotal(500)
	p.SetDefaultCurrent(6)
	_ = p.Node()
	if p.ItemPressable(kit.PaginationItemJumpPrev, 0) == nil && p.ItemPressable(kit.PaginationItemJumpNext, 0) == nil {
		t.Fatal("expected ellipsis jump controls")
	}
	tree := core.NewTree(p.Node())
	before := p.Current
	if p.ItemPressable(kit.PaginationItemJumpNext, 0) != nil {
		clickPagItem(t, tree, p, kit.PaginationItemJumpNext, 0)
		if p.Current <= before {
			t.Fatalf("jump next Current=%d before=%d", p.Current, before)
		}
	}
}

func TestPagination_PRD_15_ChangerDemo(t *testing.T) {
	// PG-15 changer.tsx
	p := kit.NewPagination()
	p.SetTotal(500)
	p.SetDefaultCurrent(3)
	p.SetShowSizeChanger(true)
	if !p.ShowSizeChanger() {
		t.Fatal("showSizeChanger")
	}
	p2 := kit.NewPagination()
	p2.SetTotal(51)
	if !p2.ShowSizeChanger() {
		t.Fatal("auto showSizeChanger when total>50")
	}
	p3 := kit.NewPagination()
	p3.SetTotal(50)
	if p3.ShowSizeChanger() {
		t.Fatal("auto hide when total<=50")
	}
}

func TestPagination_PRD_16_JumpDemo(t *testing.T) {
	// PG-16 jump.tsx
	p := kit.NewPagination()
	p.SetTotal(500)
	p.SetDefaultCurrent(2)
	p.SetShowQuickJumper(true)
	if !p.ShowQuickJumper {
		t.Fatal()
	}
	if p.JumperInput() == nil {
		t.Fatal("jumper missing")
	}
}

func TestPagination_PRD_17_SizeDemo(t *testing.T) {
	// PG-17 mini.tsx
	for _, tc := range []struct {
		sz   kit.PaginationSize
		want float64
	}{
		{kit.PaginationSmall, 24},
		{kit.PaginationMiddle, 32},
		{kit.PaginationLarge, 40},
	} {
		p := kit.NewPagination()
		p.SetTotal(50)
		p.SetSize(tc.sz)
		tree := core.NewTree(p.Node())
		tree.Layout(core.Size{Width: 900, Height: 100})
		pr := p.ItemPressable(kit.PaginationItemPage, 1)
		if pr == nil {
			t.Fatalf("size %v nil item", tc.sz)
		}
		if !approxPag(pr.Size().Height, tc.want, 0.5) {
			t.Fatalf("size %v h=%v want %v", tc.sz, pr.Size().Height, tc.want)
		}
	}
}

func TestPagination_PRD_18_SimpleDemo(t *testing.T) {
	// PG-18 simple.tsx
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetDefaultCurrent(2)
	p.SetSimple(true)
	p.SetSimpleReadOnly(true)
	_ = p.Node()
	if p.ItemPressable(kit.PaginationItemPrev, 0) == nil {
		t.Fatal("prev")
	}
	tree := core.NewTree(p.Node())
	clickPagItem(t, tree, p, kit.PaginationItemNext, 0)
	if p.Current != 3 {
		t.Fatalf("Current=%d", p.Current)
	}
}

func TestPagination_PRD_19_Controlled(t *testing.T) {
	// PG-19 controlled.tsx
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetCurrent(3)
	var got int
	p.SetOnChange(func(page, pageSize int) {
		got = page
		p.SetCurrent(page)
	})
	tree := core.NewTree(p.Node())
	clickPagItem(t, tree, p, kit.PaginationItemPage, 4)
	if got != 4 {
		t.Fatalf("got=%d", got)
	}
	if p.Current != 4 {
		t.Fatalf("Current=%d", p.Current)
	}
	// without parent SetCurrent, controlled stays put
	p2 := kit.NewPagination()
	p2.SetTotal(50)
	p2.SetCurrent(3)
	tree2 := core.NewTree(p2.Node())
	clickPagItem(t, tree2, p2, kit.PaginationItemPage, 2)
	if p2.Current != 3 {
		t.Fatalf("controlled without parent update Current=%d want 3", p2.Current)
	}
}

func TestPagination_PRD_20_TokenSizes(t *testing.T) {
	// PG-20
	th := kit.DefaultTheme()
	if !approxPag(th.SizeOr(core.TokenControlHeight, 0), 32, 0.01) {
		t.Fatal("controlHeight")
	}
	if !approxPag(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.01) {
		t.Fatal("controlHeightSM")
	}
	if !approxPag(th.SizeOr(core.TokenControlHeightLG, 0), 40, 0.01) {
		t.Fatal("controlHeightLG")
	}
	if !approxPag(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.01) {
		t.Fatal("radius")
	}
	p := kit.NewPagination()
	p.SetTotal(50)
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr := p.ItemPressable(kit.PaginationItemPage, 1)
	if !approxPag(pr.Size().Height, 32, 0.5) {
		t.Fatalf("h=%v", pr.Size().Height)
	}
}

func TestPagination_PRD_21_ThemeColors(t *testing.T) {
	// PG-21
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetDefaultCurrent(2)
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr := p.ItemPressable(kit.PaginationItemPage, 2)
	if pr == nil {
		t.Fatal("nil")
	}
	kids := pr.Children()
	if len(kids) == 0 {
		t.Fatal("no chrome")
	}
	dec, ok := kids[0].(*primitive.Decorated)
	if !ok {
		t.Fatalf("%T", kids[0])
	}
	if !approxPagColor(dec.Background, primary, 0.05) {
		t.Fatalf("active bg=%v want primary %v", dec.Background, primary)
	}
	if primary.A < 0.5 {
		t.Fatal("primary token missing")
	}
}

func TestPagination_PRD_22_DisabledChrome(t *testing.T) {
	// PG-22
	th := kit.DefaultTheme()
	dis := th.Color(core.TokenColorDisabledText)
	p := kit.NewPagination()
	p.SetTotal(50)
	p.SetDisabled(true)
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr := p.ItemPressable(kit.PaginationItemPage, 1)
	kids := pr.Children()
	dec := kids[0].(*primitive.Decorated)
	var lab *primitive.Text
	for _, c := range dec.Children() {
		if tx, ok := c.(*primitive.Text); ok {
			lab = tx
			break
		}
	}
	if lab == nil {
		t.Fatal("no label")
	}
	if !approxPagColor(lab.Color, dis, 0.08) {
		t.Fatalf("disabled text=%v want %v", lab.Color, dis)
	}
}

func TestPagination_PRD_23_KeyboardFocus(t *testing.T) {
	// PG-23
	p := kit.NewPagination()
	p.SetTotal(50)
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr := p.ItemPressable(kit.PaginationItemPage, 2)
	if pr == nil {
		t.Fatal("nil")
	}
	if !pr.Focusable || !pr.ShowFocusRing {
		t.Fatalf("focusable=%v ring=%v", pr.Focusable, pr.ShowFocusRing)
	}
	var n int
	p.SetOnChange(func(page, pageSize int) { n++ })
	clickPagItem(t, tree, p, kit.PaginationItemPage, 2)
	if n != 1 {
		t.Fatalf("click n=%d", n)
	}
	// keyboard activate focused control
	tree.Layout(core.Size{Width: 900, Height: 80})
	pr3 := p.ItemPressable(kit.PaginationItemPage, 3)
	if pr3 == nil {
		t.Fatal("page 3 missing")
	}
	abs := core.AbsoluteBounds(pr3)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if p.Current < 2 {
		t.Fatalf("Current=%d", p.Current)
	}
}
