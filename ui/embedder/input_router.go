package embedder

import (
	"fmt"
	"os"
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

	// TextEditor is the FALLBACK editable control used when no focused
	// TextEditTarget resolves (single-editor windows / tests). A focused
	// target takes precedence (plan I5).
	TextEditor *textinput.Editor

	// ime + session implement automatic IME session management (I4): the
	// session belongs to the currently focused TextEditTarget.
	ime     platform.IME
	session TextEditTarget
	// SurroundingUpdates enables periodic set_surrounding_text reporting
	// (context for IME reconversion). OFF by default: each push is a
	// commit_state round-trip and chatty reporting starved the input
	// method (observed: engine switching stopped responding). Enable
	// deliberately when a target needs context-aware IME features.
	SurroundingUpdates bool
	lastSurr           string // dedupe key of the last surrounding push ("text\x00cursor")
	hasAnchor          bool   // lastAnchor valid?
	lastAnchor         platform.Rect

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

// SetFocus attaches the focus manager for key routing. When the IME
// capability is already attached, focus observation starts here.
func (r *InputRouter) SetFocus(f *focus.FocusManager) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.focus = f
	ime := r.ime
	r.mu.Unlock()
	if ime != nil && f != nil {
		f.AddFocusObserver(r.onFocusChange)
		r.onFocusChange(nil, f.Primary())
	}
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

// TextEditTarget is implemented by editable controls (kit Input/TextArea,
// demo boxes) so the framework drives their IME session automatically
// (plan §10.2 I4): focus-in opens the session, blur closes it (rolling back
// any live pre-edit), edits keep the candidate anchor and surrounding text
// fresh.
type TextEditTarget interface {
	// Editor returns the editing state this target edits (non-nil).
	Editor() *textinput.Editor
	// IMERect returns the current caret anchor rectangle in logical px,
	// window-relative — candidate windows attach here. Compute it from the
	// caret position (e.g. prefix width), not just the field bounds.
	IMERect() platform.Rect
	// ContentPurpose declares the field type for the input method.
	ContentPurpose() platform.ContentPurpose
}

// AttachIME wires the optional IME capability for automatic session
// management: focus transitions of registered TextEditTargets open/close
// sessions; edits refresh the anchor and surrounding text. Re-evaluates the
// current primary immediately; safe before or after SetFocus.
func (r *InputRouter) AttachIME(ime platform.IME) {
	if r == nil || ime == nil {
		return
	}
	r.mu.Lock()
	r.ime = ime
	fm := r.focus
	r.mu.Unlock()
	if fm != nil {
		fm.AddFocusObserver(r.onFocusChange)
		r.onFocusChange(nil, fm.Primary())
	}
}

// onFocusChange runs on UI-thread focus transitions and re-syncs the IME
// session with the newly focused target.
func (r *InputRouter) onFocusChange(from, to *focus.FocusNode) {
	r.mu.Lock()
	prev := r.session
	var next TextEditTarget
	if to != nil {
		next, _ = to.Target.(TextEditTarget)
	}
	if ime := r.ime; ime == nil || prev == next {
		r.session = prev // unchanged (or no capability yet)
		r.mu.Unlock()
		return
	} else {
		r.session = next
		r.lastSurr = "" // new session must push fresh surrounding state
		r.hasAnchor = false
		r.mu.Unlock()
		r.syncSession(ime, prev, next)
	}
}

// debugIME logs protocol-relevant session transitions when
// GPUI_IME_DEBUG=1 — the evidence trail for compositor/IME misbehavior.
func (r *InputRouter) debugIME(format string, args ...any) {
	if os.Getenv("GPUI_IME_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[ime-router] "+format+"\n", args...)
}

// syncSession closes the outgoing session (drop live overlay → disable)
// then opens the incoming one (purpose → enable at its anchor).
func (r *InputRouter) syncSession(ime platform.IME, prev, next TextEditTarget) {
	if prev != nil {
		r.debugIME("session close: cancel+disable")
		if ed := prev.Editor(); ed != nil {
			ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""}) // R2 clear
		}
		ime.DisableIME()
	}
	if next != nil {
		r.debugIME("session open: purpose=%v rect=%v", next.ContentPurpose(), next.IMERect())
		ime.SetContentType(next.ContentPurpose())
		ime.EnableIME(next.IMERect())
	}
}

// currentTarget resolves the focused control when it is a text edit target.
func (r *InputRouter) currentTarget() TextEditTarget {
	if r.focus != nil {
		n := r.focus.Primary()
		if n != nil {
			if tt, ok := n.Target.(TextEditTarget); ok && tt.Editor() != nil {
				return tt
			}
		}
	}
	return nil
}

// editorFor returns the editor events apply to: the focused target's editor
// when one is focused, else the static fallback (I5).
func (r *InputRouter) editorFor() *textinput.Editor {
	if t := r.currentTarget(); t != nil {
		return t.Editor()
	}
	return r.TextEditor
}

// afterEdit refreshes the open session's candidate anchor and surrounding
// text after an edit or caret move reached an editor (typing, IME commit,
// arrows/backspace routed through the router). Both pushes are deduped:
// compositor preedit ECHOES (mutter re-sends the current preedit in reply to
// our commit_state) must not turn into another commit_state or they form a
// protocol feedback loop — observed as dozens of identical preedit events
// per keystroke.
func (r *InputRouter) afterEdit() {
	r.mu.Lock()
	t, ime := r.session, r.ime
	r.mu.Unlock()
	if t == nil || ime == nil {
		return
	}
	rect := t.IMERect()
	r.mu.Lock()
	same := r.hasAnchor && rect == r.lastAnchor
	r.hasAnchor, r.lastAnchor = true, rect
	r.mu.Unlock()
	if !same {
		ime.UpdateCursorRect(rect)
	}
	r.pushSurrounding(ime, t)
}

// RefreshIMEAnchor re-sends the open session's cursor rect and surrounding
// text. Widgets call it after programmatic buffer/caret mutations that did
// not flow through the router (SetText, click-to-place-caret).
func (r *InputRouter) RefreshIMEAnchor() { r.afterEdit() }

// pushSurrounding reports buffer+caret as surrounding text. Truncates to 4000 bytes centered at cursor (R3).
func (r *InputRouter) pushSurrounding(ime platform.IME, t TextEditTarget) {
	if !r.SurroundingUpdates {
		return
	}
	ed := t.Editor()
	if ed == nil {
		return
	}
	text, cursor := ed.Snapshot()
	trText, trCur := textinput.TruncateSurrounding(text, cursor)
	key := fmt.Sprintf("%s\x00%d", trText, trCur)
	r.mu.Lock()
	same := key == r.lastSurr
	r.lastSurr = key
	r.mu.Unlock()
	if same {
		return
	}
	ime.SetComposing(trText, trCur)
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
		if ed := r.editorFor(); ed != nil {
			ed.ApplyText(ev.Text)
		}
		if r.OnText != nil {
			r.OnText(ev.Text)
		}
		r.afterEdit()
	case input.KindIME:
		if ed := r.editorFor(); ed != nil {
			ed.ApplyIME(ev.IME)
		}
		if r.OnIME != nil {
			r.OnIME(ev.IME)
		}
		r.afterEdit()
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
	// Deliver to every control ON THE HIT PATH implementing PointerHandler:
	// the deepest hit node first, then its ancestors (Flutter dispatches
	// pointer events along the whole hit path). Without the ancestor walk a
	// container-wrapped field (handler on the box, click landing on its text
	// child) never sees the click — observed as click-to-caret doing nothing.
	for n := target; n != nil; {
		if ph, ok := n.(input.PointerHandler); ok {
			ph.OnPointer(ev.Pointer)
		}
		n = n.Parent()
	}
	if r.OnPointer != nil {
		r.OnPointer(ev.Pointer, target)
	}
	// Click-to-place-caret edits the buffer inside the handler — refresh the
	// session anchor/surrounding like keyboard-driven edits do. ONLY on
	// down events: motion/up never edit, and refreshing per mouse-move
	// spams commit_state hundreds of times a second, starving the input
	// method (observed: engine switching stopped responding).
	if ev.Kind == input.KindPointer && ev.Pointer.Kind == input.PointerDown {
		r.afterEdit()
	}
}

func (r *InputRouter) routeKey(ev input.Event) {
	ke := ev.Key
	ed := r.editorFor()
	// Printable character without a modifier (or with shift) → committed
	// text into the focused editor (plain keyboard path; IME compose goes
	// through KindText/KindIME separately). Control keys (Backspace/arrows)
	// are handled by the editor's OnKey consumer instead.
	if ke.Pressed && ed != nil && ke.Rune != 0 && ke.Rune != '\r' && ke.Rune != '\n' {
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
			if !r.mods.Control && !r.mods.Alt && !r.mods.Meta && !ed.ComposeActive() {
				ed.Insert(string(ke.Rune))
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
	// Editing keys (arrows/backspace via the OnKey consumer above) move the
	// caret too — refresh anchor + surrounding like text edits do.
	r.afterEdit()
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
