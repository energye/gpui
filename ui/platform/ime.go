package platform

// Rect is a rectangle in logical pixels (Y-down), used for IME cursor rects.
type Rect struct {
	X, Y, W, H float64
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
