package textinput

import (
	"testing"
	"time"

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

// Oversized steps keep true parity: a deadline-slept loop wakes with the
// real elapsed time, so one TickBlink may cross several half periods.
func TestBlinkParity_OversizedStep(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	root := rendering.NewAbsoluteBox(1200, 800)
	root.Place(box, 296, 312)
	_ = rendering.NewPipelineOwner(root)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatal("focus request failed")
	}
	box.SetCaretOn(true)
	if box.BlinkTick(1.2) {
		t.Fatal("two crossed periods must net no visible change")
	}
	if !box.IsCaretOn() {
		t.Fatal("even toggles must leave caret on")
	}
	if !box.BlinkTick(0.6) {
		t.Fatal("one more period must toggle off")
	}
	if box.IsCaretOn() {
		t.Fatal("caret must be off after odd toggles")
	}
}

// NextBlinkIn tracks the phase remainder; unfocused boxes report none.
func TestBlinkDeadline_TracksPhase(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	if _, ok := box.NextBlinkIn(); ok {
		t.Fatal("unfocused box must report no deadline")
	}
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatal("focus request failed")
	}
	box.SetCaretOn(true)
	if d, ok := box.NextBlinkIn(); !ok || d != 500*time.Millisecond {
		t.Fatalf("NextBlinkIn=%v,%v want 500ms,true", d, ok)
	}
	box.TickCaret(0.2)
	if d, ok := box.NextBlinkIn(); !ok || d < 290*time.Millisecond || d > 310*time.Millisecond {
		t.Fatalf("NextBlinkIn=%v,%v want ~300ms", d, ok)
	}
}
