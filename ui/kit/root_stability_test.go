package kit_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Click after mount must not replace Root (parent tree would freeze).
func TestList_SelectKeepsRootIdentity(t *testing.T) {
	l := kit.NewList("a", "b", "c")
	tree := core.NewTree(l.Node())
	tree.Layout(core.Size{Width: 300, Height: 300})
	r0 := l.Root
	// simulate selection rebuild
	l.SetSelected(1)
	tree.Layout(core.Size{Width: 300, Height: 300})
	if l.Root != r0 {
		t.Fatal("List.SetSelected replaced Root")
	}
	if l.Selected != 1 {
		t.Fatalf("selected=%d", l.Selected)
	}
}

func TestCollapse_ToggleKeepsRootIdentity(t *testing.T) {
	c := kit.NewCollapse(kit.CollapsePanel{Key: "1", Header: "H", Content: primitive.NewText("body")})
	tree := core.NewTree(c.Node())
	tree.Layout(core.Size{Width: 400, Height: 400})
	r0 := c.Root
	c.SetActive("1")
	if c.Root != r0 {
		t.Fatal("Collapse.SetActive replaced Root")
	}
}

func TestRate_SetValueKeepsRootIdentity(t *testing.T) {
	r := kit.NewRate(2)
	tree := core.NewTree(r.Node())
	tree.Layout(core.Size{Width: 200, Height: 40})
	r0 := r.Root
	r.SetValue(4)
	if r.Root != r0 {
		t.Fatal("Rate.SetValue replaced Root")
	}
	if r.Value != 4 {
		t.Fatal(r.Value)
	}
}

func TestSegmented_SetValueKeepsRootIdentity(t *testing.T) {
	s := kit.NewSegmented("A", "B", "C")
	tree := core.NewTree(s.Node())
	tree.Layout(core.Size{Width: 300, Height: 40})
	r0 := s.Root
	s.SetValue("B")
	if s.Root != r0 {
		t.Fatal("Segmented.SetValue replaced Root")
	}
}

func TestPagination_SetPageKeepsRootIdentity(t *testing.T) {
	p := kit.NewPagination(10)
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 400, Height: 40})
	r0 := p.Root
	p.SetPage(3)
	if p.Root != r0 {
		t.Fatal("Pagination.SetPage replaced Root")
	}
}

func TestCarousel_NextKeepsRootIdentity(t *testing.T) {
	c := kit.NewCarousel(primitive.NewText("1"), primitive.NewText("2"))
	tree := core.NewTree(c.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})
	r0 := c.Root
	c.Next()
	if c.Root != r0 {
		t.Fatal("Carousel.Next replaced Root")
	}
	if c.Index != 1 {
		t.Fatal(c.Index)
	}
}

func TestCascader_SelectKeepsRootIdentity(t *testing.T) {
	c := kit.NewCascader(
		&kit.TreeNode{Key: "a", Title: "A", Children: []*kit.TreeNode{{Key: "a1", Title: "A1"}}},
		&kit.TreeNode{Key: "b", Title: "B"},
	)
	tree := core.NewTree(c.Node())
	tree.Layout(core.Size{Width: 600, Height: 300})
	r0 := c.Root
	// select first column via list API path
	if len(c.Columns) < 1 {
		t.Fatal("no columns")
	}
	c.Columns[0].SetSelected(0)
	// OnSelect of cascader is only wired via List.OnSelect — call showLevel path via SetSelected click:
	// SetSelected rebuilds list but doesn't fire OnSelect. Fire manually as click would:
	if c.Columns[0].OnSelect != nil {
		c.Columns[0].OnSelect(0, "A")
	}
	tree.Layout(core.Size{Width: 600, Height: 300})
	if c.Root != r0 {
		t.Fatal("Cascader column select replaced Root")
	}
	if len(c.Columns) < 2 {
		t.Fatalf("expected 2 columns, got %d", len(c.Columns))
	}
}

func TestButton_SetLoadingKeepsRootIdentity(t *testing.T) {
	b := kit.NewButton("Go")
	tree := core.NewTree(b.Node())
	tree.Layout(core.Size{Width: 100, Height: 40})
	r0 := b.Root
	b.SetLoading(true)
	if b.Root != r0 {
		t.Fatal("Button.SetLoading replaced Root")
	}
	b.SetLoading(false)
	if b.Root != r0 {
		t.Fatal("Button.SetLoading(false) replaced Root")
	}
}

func TestNotifyQueue_Expire(t *testing.T) {
	q := core.NewNotifyQueue(5)
	now := time.Now().UnixMilli()
	q.Push(core.NotifyItem{Content: "x", DurationMs: 100, CreatedAtMs: now - 200})
	q.Push(core.NotifyItem{Content: "y", DurationMs: 5000, CreatedAtMs: now})
	if !q.Expire(now) {
		t.Fatal("expected expire")
	}
	if q.Len() != 1 {
		t.Fatalf("len=%d", q.Len())
	}
	if q.Items()[0].Content != "y" {
		t.Fatal(q.Items()[0].Content)
	}
}
