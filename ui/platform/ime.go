package platform

// Rect is a rectangle in logical pixels (Y-down), used for IME cursor rects.
type Rect struct {
	X, Y, W, H float64
}

// ContentPurpose classifies an editing field so the input method can adapt
// (layout, prediction, masking). Values mirror the zwp_text_input_v3
// content_purpose enum (which itself follows Android InputType); backends
// without purpose support ignore it.
type ContentPurpose uint32

const (
	// PurposeNormal default text entry.
	PurposeNormal ContentPurpose = 0
	// PurposeAlpha alphabetic input.
	PurposeAlpha ContentPurpose = 1
	// PurposeDigits digits without numeric formatting (e.g. codes).
	PurposeDigits ContentPurpose = 2
	// PurposeNumber a number (with numeric formatting).
	PurposeNumber ContentPurpose = 3
	// PurposePhone a phone number.
	PurposePhone ContentPurpose = 4
	// PurposeURL a URL / file path.
	PurposeURL ContentPurpose = 5
	// PurposeEmail an email address.
	PurposeEmail ContentPurpose = 6
	// PurposeName a person name.
	PurposeName ContentPurpose = 7
	// PurposePassword a password (masking + no prediction).
	PurposePassword ContentPurpose = 8
	// PurposePin a PIN (numeric, masked).
	PurposePin ContentPurpose = 9
	// PurposeDate a date.
	PurposeDate ContentPurpose = 10
	// PurposeTime a time.
	PurposeTime ContentPurpose = 11
	// PurposeDatetime date and time.
	PurposeDatetime ContentPurpose = 12
	// PurposeTerminal shell input.
	PurposeTerminal ContentPurpose = 13
)

// ContentType bundles the purpose declaration with editing hints so every
// backend receives one complete snapshot (design D8).
type ContentType struct {
	Purpose ContentPurpose
	// Hints are advisory (autocap/spellcheck/private); Wayland maps the
	// subset it supports, IMM32/TSF use InputScope, mac uses keyboardType.
	Hints uint32
}

// FieldSnapshot is everything Enable needs about the focused field.
type FieldSnapshot struct {
	Rect Rect
	Type ContentType
}

// IMEWantsSurrounding is an optional IME extension: when implemented and
// true, the embedder pushes surrounding text on every edit (X11 D-Bus
// SetSurroundingText). Wayland's retrieve-surrounding is on-demand, so
// its implementation returns false and the push stays opt-in.
type IMEWantsSurrounding interface {
	WantsSurrounding() bool
}

// IME is the optional cross-platform input-method capability. A backend that
// does not support IME leaves Window.IME() nil; the UI layer silently degrades
// to plain keyboard text (same pattern as the optional VSyncWaiter).
//
// Lifecycle: when a text field gains focus the app calls EnableIME with the
// cursor/field rect (logical px, window-relative); on blur it calls
// DisableIME. During composition the system drives SetComposing (pre-edit)
// and Commit (accepted text); the backend forwards these as platform events
// that ui/input normalizes into input.Event{Kind: IME}.
type IME interface {
	// EnableIME opens an input session for the focused field at rect
	// (logical px, Y-down).
	EnableIME(rect Rect)
	// UpdateCursorRect moves the IME anchor to the current caret position
	// (logical px, window-relative). Candidate windows anchor here; call it
	// whenever the caret moves or the field scrolls.
	UpdateCursorRect(rect Rect)
	// SetContentType declares the editing purpose for subsequent state
	// commits (digits/email/password…). Backends without purpose support
	// ignore it.
	SetContentType(purpose ContentPurpose)
	// SetComposing updates the pre-edit text (e.g. pinyin romanization) and
	// the caret position within it.
	SetComposing(text string, cursor int)
	// Commit accepts the current composition as committed text.
	Commit(text string)
	// DisableIME ends the input session (focus left the field).
	DisableIME()
}

// Clipboard is the optional cross-platform clipboard capability. nil means
// unsupported; UI must degrade (internal clipboard fallback).
type Clipboard interface {
	// Get returns clipboard content of the given kind ("text/plain" etc.).
	Get(kind string) (string, error)
	// Set writes content of the given kind.
	Set(kind, data string) error
}
