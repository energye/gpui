package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestPressable_FocusVisible_PointerNoRingFlag(t *testing.T) {
	btn := primitive.NewPressable(primitive.NewBox())
	tree := core.NewTree(btn)
	tree.Layout(core.Size{Width: 100, Height: 40})
	// Pointer focus
	tree.SetFocusFromPointer(btn)
	if !btn.State.Focused {
		t.Fatal("expected focused")
	}
	if btn.State.FocusVisible {
		t.Fatal("pointer focus must not set FocusVisible (Ant :focus-visible)")
	}
	// Keyboard focus
	tree.SetFocus(btn)
	if !btn.State.FocusVisible {
		t.Fatal("SetFocus (keyboard) should set FocusVisible")
	}
}
