package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestClickCanceledWhenReleaseOutside(t *testing.T) {
	clicks := 0
	child := primitive.NewDecorated()
	child.Width, child.Height = 40, 20
	btn := primitive.NewPressable(child)
	btn.Click = func() { clicks++ }

	// Button sizes to 40×20; column is 200×100 — coords outside button cancel click.
	col := primitive.Column(btn)
	col.CrossAlign = core.CrossStart
	tree := core.NewTree(col)
	tree.Layout(core.Size{Width: 200, Height: 100})
	if btn.Size().Width != 40 || btn.Size().Height != 20 {
		t.Fatalf("btn size=%+v want 40x20", btn.Size())
	}

	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 150, Y: 80, Button: core.ButtonLeft})
	if clicks != 0 {
		t.Fatalf("outside release clicked=%d want 0", clicks)
	}

	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 12, Y: 12, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("inside release clicked=%d want 1", clicks)
	}
}
