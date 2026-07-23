package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Concurrent empty-ID portals must not share one "portal" id (clobber).
func TestOverlayPortal_UniqueAutoIDs(t *testing.T) {
	a := primitive.NewOverlayPortal(primitive.NewText("a"))
	b := primitive.NewOverlayPortal(primitive.NewText("b"))
	c := kit.NewMessageHost()
	d := kit.NewMessageHost()

	root := primitive.Column(a, b, c.Node(), d.Node())
	tree := core.NewTree(root)
	a.SetOpen(true)
	b.SetOpen(true)
	c.Info("msg-a")
	d.Info("msg-b")
	tree.Layout(core.Size{Width: 800, Height: 600})

	if a.ID == "" || b.ID == "" {
		t.Fatal("auto-id not assigned")
	}
	if a.ID == b.ID {
		t.Fatalf("portals clobbered: both id=%q", a.ID)
	}
	if c.Portal.ID == "" || d.Portal.ID == "" {
		t.Fatal("message host auto-id missing")
	}
	if c.Portal.ID == d.Portal.ID {
		t.Fatalf("message hosts clobbered: both id=%q", c.Portal.ID)
	}
	// Four distinct overlay entries
	if n := tree.Overlays().Len(); n < 4 {
		t.Fatalf("expected ≥4 overlays, got %d", n)
	}
	seen := map[string]bool{}
	for _, e := range tree.Overlays().Entries() {
		if seen[e.ID] {
			t.Fatalf("duplicate overlay id %q", e.ID)
		}
		seen[e.ID] = true
	}
}

// Tour step change must keep the same portal node (no replace while open).
func TestTour_NextKeepsPortalIdentity(t *testing.T) {
	tour := kit.NewTour(
		kit.TourStep{Title: "One", Body: "first"},
		kit.TourStep{Title: "Two", Body: "second"},
	)
	tree := core.NewTree(tour.Node())
	tour.Viewport = core.Size{Width: 800, Height: 600}
	tour.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	p0 := tour.Portal
	id0 := p0.ID
	if !p0.Open {
		t.Fatal("tour portal should be open")
	}
	tour.Next()
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tour.Portal != p0 {
		t.Fatal("Next must not replace Portal while open")
	}
	if tour.Portal.ID != id0 {
		t.Fatalf("portal id changed: %q → %q", id0, tour.Portal.ID)
	}
	if tour.Index != 1 {
		t.Fatalf("index want 1 got %d", tour.Index)
	}
	if tree.Overlays().Len() < 1 {
		t.Fatal("overlay missing after Next")
	}
}

// Modal SetFooterVisible while open must keep portal mounted in tree.
func TestModal_RebuildWhileOpenKeepsPortal(t *testing.T) {
	m := kit.NewModal("T")
	m.Viewport = core.Size{Width: 800, Height: 600}
	tree := core.NewTree(m.Node())
	m.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	p0 := m.Portal
	if tree.Overlays().Len() < 1 {
		t.Fatal("modal not in overlays")
	}
	m.SetFooterVisible(false)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if m.Portal != p0 {
		t.Fatal("SetFooterVisible replaced Portal")
	}
	if !m.Portal.Open {
		t.Fatal("portal should stay open")
	}
	if tree.Overlays().Len() < 1 {
		t.Fatal("overlay dropped after rebuild")
	}
}

// Popover programmatic SetOpen applies Viewport like click path.
func TestPopover_SetOpenAppliesViewport(t *testing.T) {
	trig := kit.NewButton("pop").Node()
	body := primitive.NewText("panel body")
	p := kit.NewPopover(trig, body)
	p.Viewport = core.Size{Width: 640, Height: 480}
	tree := core.NewTree(p.Node())
	tree.Layout(core.Size{Width: 640, Height: 480})
	p.SetOpen(true)
	if !p.Open {
		t.Fatal("Open flag")
	}
	if p.Popup == nil || !p.Popup.Open {
		t.Fatal("popup not open")
	}
	if p.Popup.Viewport.Width != 640 {
		t.Fatalf("Viewport not applied: %+v", p.Popup.Viewport)
	}
	tree.Layout(core.Size{Width: 640, Height: 480})
	if tree.Overlays().Len() < 1 {
		t.Fatal("no overlay after SetOpen")
	}
}
