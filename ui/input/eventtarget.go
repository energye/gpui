package input

// PointerHandler is implemented by controls that receive unified pointer
// samples (mouse + touch, ID-based). The InputRouter delivers hit-test
// results to any RenderObject in the hit path that implements it.
type PointerHandler interface {
	OnPointer(ev PointerEvent)
}

// KeyHandler is implemented by controls that receive logical key events.
type KeyHandler interface {
	OnKey(ev KeyEvent)
}

// TextHandler is implemented by editable controls that receive committed
// text (keyboard chars, paste, IME commits).
type TextHandler interface {
	OnText(ev TextEvent)
}

// IMEHandler is implemented by editable controls that receive in-progress
// input-method events (compose/caret).
type IMEHandler interface {
	OnIME(ev IMEEvent)
}

// EventTarget is the combined control event interface. A control that
// implements the specific sub-interfaces (PointerHandler/KeyHandler/
// TextHandler/IMEHandler) is auto-wired by the framework's InputRouter —
// no per-control native code, no platform identifiers.
type EventTarget interface {
	PointerHandler
	KeyHandler
	TextHandler
	IMEHandler
}
