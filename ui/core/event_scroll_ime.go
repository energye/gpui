package core

// ScrollEvent is a wheel/trackpad scroll delivered near a ScrollViewport.
type ScrollEvent struct {
	X, Y    float64 // pointer position in tree coords
	DX, DY  float64 // deltas (positive = right/down)
	Handled bool
}

// ScrollHandler receives scroll events.
type ScrollHandler interface {
	HandleScroll(ev *ScrollEvent)
}

// TextInputEvent carries committed text (post-IME) for the focused editor.
type TextInputEvent struct {
	Text    string
	Handled bool
}

// TextInputHandler receives committed text.
type TextInputHandler interface {
	HandleTextInput(ev *TextInputEvent)
}

// IMECompositionEvent is a composition update for CJK etc.
type IMECompositionEvent struct {
	// Text is the current preedit string (empty on end without commit).
	Text string
	// Cursor is the caret offset within Text (-1 if unknown).
	Cursor int
	// End is true when composition finished (commit may arrive as TextInput).
	End     bool
	Handled bool
}

// IMEHandler receives composition events.
type IMEHandler interface {
	HandleIME(ev *IMECompositionEvent)
}
