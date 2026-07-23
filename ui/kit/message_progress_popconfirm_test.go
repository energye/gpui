package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestMessageHost_InfoOpensPortal(t *testing.T) {
	h := kit.NewMessageHost()
	h.Viewport = core.Size{Width: 800, Height: 600}
	tree := core.NewTree(h.Node())
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Overlays().Len() != 0 {
		t.Fatal("portal should be closed when empty")
	}
	h.Info("hello toast")
	// refresh must open portal without requiring host Sync
	if h.Portal == nil || !h.Portal.Open {
		t.Fatal("Info must open MessageHost portal")
	}
	// After layout, overlay entry present
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Overlays().Len() < 1 {
		t.Fatal("expected overlay entry after Info")
	}
	// Content includes the message text somewhere under overlay
	found := false
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || found {
			return
		}
		if tx, ok := n.(*primitive.Text); ok && strings.Contains(tx.Value, "hello toast") {
			found = true
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	for _, e := range tree.Overlays().Entries() {
		walk(e.Node)
	}
	if !found {
		t.Fatal("toast text not found in overlay")
	}
}

func TestMessageHost_NotificationOpensPortal(t *testing.T) {
	h := kit.NewMessageHost()
	h.Viewport = core.Size{Width: 640, Height: 480}
	tree := core.NewTree(h.Node())
	tree.Layout(core.Size{Width: 640, Height: 480})
	h.Notification("Title", "body")
	if !h.Portal.Open {
		t.Fatal("Notification must open portal")
	}
	tree.Layout(core.Size{Width: 640, Height: 480})
	found := false
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || found {
			return
		}
		if tx, ok := n.(*primitive.Text); ok && strings.Contains(tx.Value, "Title") {
			found = true
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	for _, e := range tree.Overlays().Entries() {
		walk(e.Node)
	}
	if !found {
		t.Fatal("notification title not in overlay")
	}
}

func TestPopconfirm_TitleInContent(t *testing.T) {
	trig := kit.NewButton("Ask").Node()
	pc := kit.NewPopconfirm(trig, "Are you sure?")
	// Walk popover content for title text
	found := false
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || found {
			return
		}
		if tx, ok := n.(*primitive.Text); ok && tx.Value == "Are you sure?" {
			found = true
			if tx.Color.A < 0.1 {
				t.Fatal("title text alpha too low (invisible)")
			}
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	if pc.Popover != nil && pc.Popover.Content != nil {
		walk(pc.Popover.Content)
	}
	if !found {
		t.Fatal("popconfirm title text missing from content")
	}
}

func TestProgress_NotRepaintBoundary(t *testing.T) {
	p := kit.NewProgress(60)
	n := p.Node()
	if n == nil {
		t.Fatal("nil node")
	}
	if n.Base().IsRepaintBoundary() {
		t.Fatal("Progress must not be RepaintBoundary under ScrollViewport holes")
	}
	// Layout produces non-zero size
	sz := n.Layout(core.Loose(400, 100))
	if sz.Width < 50 || sz.Height < 8 {
		t.Fatalf("progress size too small: %v", sz)
	}
}
