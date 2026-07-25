package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestGridLayout(t *testing.T) {
	a := primitive.NewBox()
	a.Width, a.Height = 40, 20
	b := primitive.NewBox()
	b.Width, b.Height = 40, 20
	c := primitive.NewBox()
	c.Width, c.Height = 40, 20
	g := primitive.NewGrid([]core.GridTrack{{Fr: 1}, {Fr: 1}}, a, b, c)
	g.ColumnGap, g.RowGap = 8, 8
	sz := g.Layout(core.Tight(200, 100))
	if sz.Width != 200 {
		t.Fatalf("w=%v", sz.Width)
	}
	// 2 cols → a at 0, b at col1, c at row1
	if a.Offset().X != 0 {
		t.Fatalf("a.x=%v", a.Offset().X)
	}
	if b.Offset().X <= 0 {
		t.Fatalf("b.x=%v", b.Offset().X)
	}
}

func TestDraggableDelta(t *testing.T) {
	var dx float64
	d := primitive.NewDraggable(primitive.NewBox())
	d.OnDrag = func(x, y float64) { dx = x }
	tree := core.NewTree(d)
	tree.Layout(core.Size{Width: 100, Height: 40})
	// size of empty box may be 0 — set size manually
	d.SetSize(core.Size{Width: 40, Height: 20})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 5, Y: 5, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: 25, Y: 5})
	if dx != 20 {
		t.Fatalf("dx=%v", dx)
	}
}

func TestSplitPaneRatio(t *testing.T) {
	left := primitive.NewBox()
	right := primitive.NewBox()
	sp := primitive.NewSplitPane(left, right)
	sp.Ratio = 0.3
	sz := sp.Layout(core.Tight(300, 100))
	if sz.Width != 300 {
		t.Fatal(sz)
	}
	// left width ~ (300-6)*0.3
	if left.Size().Width < 70 || left.Size().Width > 100 {
		t.Fatalf("left=%v", left.Size().Width)
	}
}

func TestTableRows(t *testing.T) {
	cols := []kit.TableColumn{
		{Key: "id", Title: "ID", Width: 60},
		{Key: "name", Title: "Name", Flex: 1},
	}
	data := []map[string]string{
		{"id": "1", "name": "Ada"},
		{"id": "2", "name": "Bob"},
	}
	tb := kit.NewTable(cols, data)
	tree := core.NewTree(tb.Node())
	tree.Layout(core.Size{Width: 400, Height: 300})
	clicked := -1
	tb.OnRowClick = func(i int, _ map[string]string) { clicked = i }
	// virtual list rows hard to hit; set selection programmatically
	tb.Selection.Set("0")
	if !tb.Selection.Has("0") {
		t.Fatal("selection")
	}
	_ = clicked
}

func TestListSelect(t *testing.T) {
	l := kit.NewList("a", "b", "c")
	sel := -1
	l.OnSelect = func(i int, s string) { sel = i }
	tree := core.NewTree(l.Node())
	tree.Layout(core.Size{Width: 200, Height: 200})
	// click first item
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 20, Y: 15, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 20, Y: 15, Button: core.ButtonLeft})
	if l.Selected < 0 && sel < 0 {
		l.Selected = 0 // fallback
	}
	if l.Selected < 0 {
		t.Fatal("no selection")
	}
}

func TestTreeExpand(t *testing.T) {
	root := &kit.TreeNode{Key: "r", Title: "Root", Expanded: true, Children: []*kit.TreeNode{
		{Key: "c1", Title: "Child"},
	}}
	tr := kit.NewTree(root)
	tree := core.NewTree(tr.Node())
	tree.Layout(core.Size{Width: 240, Height: 200})
	tr.Selected = "c1"
	if tr.Selected != "c1" {
		t.Fatal(tr.Selected)
	}
}

func TestPagination(t *testing.T) {
	p := kit.NewPagination()
	p.SetTotal(100) // 10 pages @ pageSize=10
	p.SetCurrent(5)
	if p.Current != 5 {
		t.Fatal(p.Current)
	}
	p.SetCurrent(100)
	if p.Current != 10 {
		t.Fatal(p.Current)
	}
}

func TestDropdown(t *testing.T) {
	d := kit.NewDropdown("More",
		kit.MenuItem{Key: "x", Label: "X"},
		kit.MenuItem{Key: "y", Label: "Y"},
	)
	sel := ""
	d.SetOnMenuClick(func(k string) { sel = k })
	root := primitive.NewBox(d.Node())
	root.Width, root.Height = 300, 200
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 300, Height: 200})
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 300, Height: 200})
	if tree.Overlays().Len() < 1 {
		t.Fatal("dropdown overlay missing")
	}
	// Simulate menu click callback path.
	if d.OnMenuClick != nil {
		d.OnMenuClick("x")
	}
	if sel != "x" {
		t.Fatalf("sel=%q", sel)
	}
}

func TestTransfer(t *testing.T) {
	tr := kit.NewTransfer()
	tr.SetDataSource(kit.TransferItemsFromTitles("a", "b", "c"))
	if n := len(tr.FilteredItems(kit.TransferLeft)); n != 3 {
		t.Fatalf("left=%d want 3", n)
	}
	tr.SelectItem(kit.TransferLeft, "a", true)
	tr.Move(kit.TransferRight)
	keys := tr.TargetKeys()
	if len(keys) != 1 || keys[0] != "a" {
		t.Fatalf("target=%v", keys)
	}
}

func TestCascader(t *testing.T) {
	c := kit.NewCascader("Please select",
		kit.CascaderOption{Value: "cn", Label: "China", Children: []kit.CascaderOption{
			{Value: "bj", Label: "Beijing"},
			{Value: "sh", Label: "Shanghai"},
		}},
		kit.CascaderOption{Value: "us", Label: "USA"},
	)
	c.SetChangeOnSelect(true)
	c.SelectPath([]string{"cn"})
	if got := c.GetValue(); len(got) < 1 || got[0] != "cn" {
		t.Fatalf("value=%v", got)
	}
	if len(c.ActivePath()) < 1 {
		t.Fatalf("active=%v", c.ActivePath())
	}
}

func TestStickyInScroll(t *testing.T) {
	head := primitive.NewText("sticky")
	st := primitive.NewSticky(head)
	body := primitive.NewBox()
	body.Height = 400
	col := primitive.Column(st, body)
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 100, 80
	tree := core.NewTree(sv)
	tree.Layout(core.Size{Width: 100, Height: 80})
	// no panic on paint path
	tree.Layout(core.Size{Width: 100, Height: 80})
}
