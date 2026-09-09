package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
)

// Focused boxes self-register for framework-owned blink dt; unfocused or
// disabled boxes cost nothing. The embedder pump drives registered boxes via
// owner.TickBlink (UI thread); frames follow dirtiness, never ticker
// aliveness.
func TestBlinkFocusRegistration(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Place(box, 296, 312)
	owner := rendering.NewPipelineOwner(root)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if owner.BlinkActive() {
		t.Fatal("unfocused box must not register for blink dt")
	}
	if !box.FocusNode().RequestFocus() {
		t.Fatal("focus request failed")
	}
	if !owner.BlinkActive() {
		t.Fatal("focused box must register for blink dt")
	}
	// Fresh focus starts caret-on; one half period toggles it off.
	if !box.IsCaretOn() {
		t.Fatal("fresh focus must start caret-on")
	}
	if !owner.TickBlink(0.6) {
		t.Fatal("TickBlink must report the toggle")
	}
	if box.IsCaretOn() {
		t.Fatal("caret must be off after one blink half-period")
	}
	if !owner.TickBlink(0.6) {
		t.Fatal("TickBlink must report the toggle back on")
	}
	if !box.IsCaretOn() {
		t.Fatal("caret must be on after two half-periods")
	}
	// Disabling a focused box drops it: no dt, no frames for it.
	box.SetDisabled(true)
	if owner.BlinkActive() {
		t.Fatal("disabled box must drop blink registration")
	}
}
