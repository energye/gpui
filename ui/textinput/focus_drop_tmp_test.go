package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/focus"
)

// Temporary diagnosis (delete after): does a MultiLine box drop focus + caret
// when focus moves to another box?
func TestMultiLineFocusDropTmp(t *testing.T) {
	edD := New()
	boxD := NewMultiLineInputBox(edD, 880, 90, 16)
	edB := New()
	boxB := NewViewportInputBox(edB, 880, 40, 16)
	fm := focus.NewManager()
	fm.Register(boxD.FocusNode())
	fm.Register(boxB.FocusNode())
	if !boxD.FocusNode().RequestFocus() {
		t.Fatalf("D focus request failed")
	}
	boxD.TickCaret(0.6)
	boxD.TickCaret(0.6)
	if !boxD.IsFocused() || !boxD.IsCaretOn() {
		t.Fatalf("D should be focused+on, focused=%v caret=%v", boxD.IsFocused(), boxD.IsCaretOn())
	}
	if !boxB.FocusNode().RequestFocus() {
		t.Fatalf("B focus request failed")
	}
	t.Logf("after switch: D focused=%v caret=%v; B focused=%v", boxD.IsFocused(), boxD.IsCaretOn(), boxB.IsFocused())
	if boxD.IsFocused() {
		t.Fatalf("D still focused after switching to B")
	}
	for i := 0; i < 4; i++ {
		boxD.TickCaret(0.6)
		boxB.TickCaret(0.6)
		if boxD.IsCaretOn() {
			t.Fatalf("D caret on while unfocused (tick %d)", i)
		}
	}
}
