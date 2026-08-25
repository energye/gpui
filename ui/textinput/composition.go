package textinput

import (
	"fmt"

	"github.com/energye/gpui/ui/input"
)

// --- composition application (design §4.1: inbound IME events) ---
//
// These are the ONLY writers of e.comp. They implement the state machine's
// composing leg: preedit updates mutate the overlay (never the buffer),
// commit replaces the overlay atomically via Insert, empty text terminates
// (R2: an empty preedit can only END a session, never start one).

// ApplyIME applies one normalized IME event. Returns true when visible
// state changed.
func (e *Editor) ApplyIME(ev input.IMEEvent) bool {
	if e == nil {
		return false
	}
	before := e.View().Display
	switch ev.Kind {
	case input.IMECompose:
		if ev.Text == "" {
			e.comp = nil // R2: clear semantics; buffer untouched
			if e.View().Display != before {
				e.changed()
			}
			return true
		}
		if e.comp == nil {
			e.comp = &Composition{}
		}
		e.comp.Text = ev.Text
		e.comp.Cursor = ev.Start
		e.comp.Segs = nil
		e.changed()
	case input.IMECommit:
		// Insert covers the caret where the overlay sat; atomic swap.
		e.Insert(ev.Text)
	case input.IMEDeleteSurrounding:
		before_, after := -ev.Start, ev.End
		if before_ < 0 {
			before_ = 0
		}
		if after < 0 {
			after = 0
		}
		e.DeleteSurrounding(before_, after)
	}
	return e.View().Display != before
}

// ComposeActive reports whether a composition overlay is live.
func (e *Editor) ComposeActive() bool { return e != nil && e.comp != nil }

// CompositionText returns the overlay text ("" when inactive).
func (e *Editor) CompositionText() string {
	if e == nil || e.comp == nil {
		return ""
	}
	return e.comp.Text
}

// CompositionCursor returns the IME-reported caret within the overlay,
// mapped to DISPLAY coordinates for rendering (clamped into the span;
// <0 = end). Buffer coordinates are meaningless inside the span.
func (e *Editor) CompositionCursor() int {
	if e == nil || e.comp == nil {
		return -1
	}
	v := e.View()
	off := e.comp.Cursor
	if off < 0 || off > len(e.comp.Text) {
		off = len(e.comp.Text)
	}
	return v.MapBufToViewExact(e.sel[1], off)
}

// Snapshot returns (text, cursor) for surrounding-text reporting: committed
// buffer + caret. The composition is EXCLUDED by construction (protocol
// reports it separately); cursor needs no adjustment because buf never
// contains the overlay (design D1 makes the old shift-arithmetic class of
// bugs impossible).
func (e *Editor) Snapshot() (text string, cursor int) {
	return e.Text(), e.Cursor()
}

// ApplyText inserts committed text (keyboard chars, paste). Returns true on
// change.
func (e *Editor) ApplyText(ev input.TextEvent) bool {
	if e == nil || ev.Text == "" {
		return false
	}
	before := e.buf
	e.Insert(ev.Text)
	return e.buf != before
}

var _ = fmt.Sprintf // keep fmt for future debug helpers
