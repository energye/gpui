package core_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Theme change broadcast increments epoch and invokes theme hooks.
func TestThemeChangeBroadcast(t *testing.T) {
	box := primitive.NewBox()
	box.Width, box.Height = 40, 40
	var hooks int
	box.SetThemeHook(func(*core.Theme) { hooks++ })
	tree := core.NewTree(box)
	tree.Layout(core.Size{Width: 40, Height: 40})

	th1 := core.DefaultTheme()
	tree.SetTheme(th1)
	if tree.ThemeEpoch() != 1 {
		t.Fatalf("epoch=%d want 1", tree.ThemeEpoch())
	}
	if hooks != 1 {
		t.Fatalf("hooks=%d want 1", hooks)
	}
	th2 := core.DefaultTheme()
	th2.ColorPrimary = render.Hex("#FF0000")
	tree.SetTheme(th2)
	if tree.ThemeEpoch() != 2 || hooks != 2 {
		t.Fatalf("epoch=%d hooks=%d", tree.ThemeEpoch(), hooks)
	}
	if !tree.FullPaintRequired() {
		t.Fatal("theme change should MarkFullPaintRequired")
	}
}

// Open geometry refresher runs after Layout (anchor follow).
func TestLayoutRefreshesOpenAnchoredPopup(t *testing.T) {
	trigger := primitive.NewBox()
	trigger.Width, trigger.Height = 40, 20
	panel := primitive.NewBox()
	panel.Width, panel.Height = 80, 40
	pop := primitive.NewAnchoredPopup(panel)
	pop.Placement = primitive.PlaceBottom
	pop.UpdateAnchorFromNode(trigger)

	col := primitive.Column(trigger, pop)
	root := primitive.NewBox(col)
	root.Width, root.Height = 200, 200
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 200, Height: 200})
	pop.SetOpen(true)
	tree.Layout(core.Size{Width: 200, Height: 200})

	// Move trigger by changing column padding / offset via layout of larger first child area
	// Simpler: set trigger offset manually then Refresh via Layout.
	abs1 := core.AbsoluteBounds(trigger)
	// Push trigger down by inserting spacer
	spacer := primitive.NewBox()
	spacer.Width, spacer.Height = 10, 50
	col.ClearChildren()
	col.AddChild(spacer)
	col.AddChild(trigger)
	col.AddChild(pop)
	tree.Layout(core.Size{Width: 200, Height: 200})
	abs2 := core.AbsoluteBounds(trigger)
	if abs2.Min.Y <= abs1.Min.Y {
		t.Fatalf("trigger should move down: %v → %v", abs1.Min, abs2.Min)
	}
	// Popup content offset should track (panel absolute Y roughly below trigger)
	off := panel.Base().Offset()
	if off.Y < abs2.Max.Y-1 {
		// PlaceBottom: y = anchor.Max.Y + gap
		t.Fatalf("popup Y=%v should be below trigger maxY=%v after layout refresh", off.Y, abs2.Max.Y)
	}
}
