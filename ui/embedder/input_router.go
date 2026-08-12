package embedder

import (
	"sync"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/textinput"
)

// HitTestFunc returns the top-most render object at logical (x, y) plus the
// overlay band / entry, matching PipelineApp.HitTestPointer's shape.
type HitTestFunc func(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry)

// InputRouter is the framework's unified event binding layer (plan §4).
//
// When attached to a PipelineApp (PipelineOptions.Input), platform events are
// normalized via input.FromPlatform into the cross-platform input.Event and
// routed automatically:
//
//	Pointer/Touch/Scroll → hit test → every control implementing
//	                        input.PointerHandler on the hit path receives
//	                        OnPointer; plus the optional OnPointer callback.
//	Key                    → focus manager (when provided) and the optional
//	                        OnKey callback; logical modifiers tracked.
//	Text/IME               → optional OnText / OnIME callbacks (textinput
//	                        milestone consumes these).
//
// Any RenderObject implementing the input.EventTarget sub-interfaces
// (PointerHandler/KeyHandler/…) is auto-wired — no per-control platform code.
// Attaching an InputRouter is opt-in: windows without it keep the existing
// OnEvent path byte-for-byte, so current examples are unaffected.
type InputRouter struct {
	hit   HitTestFunc
	focus *focus.FocusManager

	// OnPointer receives every pointer/touch/scroll sample plus the
	// hit-tested target (nil when nothing hit). Handy for gesture arenas.
	OnPointer func(ev input.PointerEvent, target rendering.RenderObject)
	// OnKey receives every logical key event (before focus consumes it).
	OnKey func(ev input.KeyEvent)
	// OnText receives committed text (keyboard chars, paste, IME commits).
	OnText func(ev input.TextEvent)
	// OnIME receives in-progress IME events.
	OnIME func(ev input.IMEEvent)

	// TextEditor is the focused editable control (if any). When set, KindText
	// and KindIME events are routed to it automatically (plan §6), in addition
	// to the OnText/OnIME callbacks.
	TextEditor *textinput.Editor

	mu   sync.Mutex
	mods input.Modifiers
}

// NewInputRouter creates a router. Pass a hit-test function (typically
// PipelineApp.HitTestPointer) and optionally the focus manager to auto-route
// Tab/Enter/activation keys.
func NewInputRouter(hit HitTestFunc, f *focus.FocusManager) *InputRouter {
	return &InputRouter{hit: hit, focus: f}
}

// SetHitTest overrides the hit-test function (e.g. after the app is bound).
func (r *InputRouter) SetHitTest(h HitTestFunc) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.hit = h
	r.mu.Unlock()
}

// SetFocus attaches the focus manager for key routing.
func (r *InputRouter) SetFocus(f *focus.FocusManager) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.focus = f
	r.mu.Unlock()
}

// Modifiers returns the tracked modifier state (for FromPlatform).
func (r *InputRouter) Modifiers() input.Modifiers {
	if r == nil {
		return input.Modifiers{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mods
}

// RoutePlatform normalizes a platform event and routes it. The router tracks
// modifier state across events (shift/ctrl/alt/meta press/release).
func (r *InputRouter) RoutePlatform(ev platform.Event) {
	if r == nil {
		return
	}
	in := input.FromPlatform(ev, r.Modifiers())
	r.Route(in)
}

// Route dispatches one normalized input event.
func (r *InputRouter) Route(ev input.Event) {
	if r == nil {
		return
	}
	switch ev.Kind {
	case input.KindPointer, input.KindTouch, input.KindScroll:
		r.routePointer(ev)
	case input.KindKey:
		r.routeKey(ev)
	case input.KindText:
		if r.TextEditor != nil {
			r.TextEditor.ApplyText(ev.Text)
		}
		if r.OnText != nil {
			r.OnText(ev.Text)
		}
	case input.KindIME:
		if r.TextEditor != nil {
			r.TextEditor.ApplyIME(ev.IME)
		}
		if r.OnIME != nil {
			r.OnIME(ev.IME)
		}
	}
}

func (r *InputRouter) routePointer(ev input.Event) {
	var target rendering.RenderObject
	var band overlay.Band
	var entry *overlay.Entry
	if r.hit != nil {
		band, target, entry = r.hit(ev.Pointer.X, ev.Pointer.Y)
	}
	_ = band
	_ = entry
	// Deliver to every control on the hit path implementing PointerHandler.
	if target != nil {
		if ph, ok := target.(input.PointerHandler); ok {
			ph.OnPointer(ev.Pointer)
		}
	}
	if r.OnPointer != nil {
		r.OnPointer(ev.Pointer, target)
	}
}

func (r *InputRouter) routeKey(ev input.Event) {
	ke := ev.Key
	// Printable character without a modifier (or with shift) → committed
	// text into the focused editor (plain keyboard path; IME compose goes
	// through KindText/KindIME separately). Control keys (Backspace/arrows)
	// are handled by the editor's OnKey consumer instead.
	if ke.Pressed && r.TextEditor != nil && ke.Rune != 0 && ke.Rune != '\r' && ke.Rune != '\n' {
		switch ke.Key {
		case input.KeyBackspace, input.KeyDelete,
			input.KeyArrowLeft, input.KeyArrowRight,
			input.KeyArrowUp, input.KeyArrowDown,
			input.KeyHome, input.KeyEnd,
			input.KeyPageUp, input.KeyPageDown,
			input.KeyEnter, input.KeyTab, input.KeyEscape:
			// editing/control keys: leave to OnKey/focus
		default:
			// When an IME composition is active the raw keys feed the
			// pre-edit (handled by the IME); do not double-insert.
			if !r.mods.Control && !r.mods.Alt && !r.mods.Meta && !r.TextEditor.ComposeActive() {
				r.TextEditor.Insert(string(ke.Rune))
			}
		}
	}
	// OnKey receives the event-time modifier state (ev.Key.Mods), which does
	// not include the modifier key itself if it is the key being pressed.
	if r.OnKey != nil {
		r.OnKey(ke)
	}
	// Update tracked modifiers for the NEXT event (a modifier press applies
	// from its own event onward; its press event still reports pre-press
	// state per FromPlatform semantics).
	if ke.Pressed {
		switch ke.Key {
		case input.KeyShift:
			r.mods.Shift = true
		case input.KeyControl:
			r.mods.Control = true
		case input.KeyAlt:
			r.mods.Alt = true
		case input.KeyMeta:
			r.mods.Meta = true
		}
	} else {
		switch ke.Key {
		case input.KeyShift:
			r.mods.Shift = false
		case input.KeyControl:
			r.mods.Control = false
		case input.KeyAlt:
			r.mods.Alt = false
		case input.KeyMeta:
			r.mods.Meta = false
		}
	}
	// Focus routing (Tab / Enter / activation / primary.OnKey).
	if r.focus != nil {
		fk := focus.KeyEvent{
			KeyCode: mapFocusKeyCode(ke.Key, ke.Rune),
			Rune:    ke.Rune,
			Pressed: ke.Pressed,
			Shift:   ke.Mods.Shift,
		}
		_ = r.focus.HandleKey(fk)
	}
}

// mapFocusKeyCode maps logical keys into the legacy focus key code space
// (X11 keysym values used by ui/focus). The focus manager migrates to input
// logical keys in the gestures milestone; this bridge keeps it compatible.
func mapFocusKeyCode(k input.Key, r rune) int {
	switch k {
	case input.KeyTab:
		return focus.KeyTab
	case input.KeyEnter:
		return focus.KeyEnter
	case input.KeySpace:
		return focus.KeySpace
	case input.KeyShift:
		return focus.KeyShiftL
	case input.KeyControl:
		return 0xffe3 // Control_L keysym
	case input.KeyAlt:
		return 0xffe9 // Alt_L keysym
	case input.KeyMeta:
		return 0xffeb // Super_L keysym
	}
	if r != 0 {
		return int(r)
	}
	return 0
}
