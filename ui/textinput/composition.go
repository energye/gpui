package textinput

import (
	"github.com/energye/gpui/ui/input"
)

// ApplyIME applies one normalized IME event. Returns true when visible state changed.
// Now delegates to Flutter-aligned four-tuple: preedit stays in text via UpdateComposingText.
func (e *Editor) ApplyIME(ev input.IMEEvent) bool {
	if e == nil {
		return false
	}
	before := e.text
	switch ev.Kind {
	case input.IMECompose:
		if ev.Text == "" {
			if e.composing {
				e.EndComposing()
			}
			return e.text != before
		}
		// batch Begin+Update into single epoch (old test expects one fire)
		was := e.composing
		if !was {
			e.BeginBatchEdit()
			e.BeginComposing()
		}
		cursor := ev.Start
		if cursor < 0 || cursor > len(ev.Text) {
			cursor = len(ev.Text)
		}
		cuOff := utf16Len(ev.Text[:cursor])
		e.UpdateComposingText(ev.Text, TextRange{Base: e.composingRange.Start() + cuOff, Extent: e.composingRange.Start() + cuOff})
		if !was {
			e.EndBatchEdit()
		}
	case input.IMECommit:
		e.AddText(ev.Text)
	case input.IMEDeleteSurrounding:
		before_, after := -ev.Start, ev.End
		if before_ < 0 {
			before_ = 0
		}
		if after < 0 {
			after = 0
		}
		e.DeleteSurrounding(-before_, after)
	}
	return e.text != before
}

func (e *Editor) ApplyText(ev input.TextEvent) bool {
	if e == nil || ev.Text == "" {
		return false
	}
	before := e.text
	e.AddText(ev.Text)
	return e.text != before
}

// CompositionCursor legacy: caret within composing span in display coords
func (e *Editor) CompositionCursor() int {
	if e == nil || !e.composing {
		return -1
	}
	v := e.View()
	off := e.selection.Extent - e.composingRange.Start()
	compText := e.CompositionText()
	b := byteOffsetForUtf16(compText, off)
	return v.CompStart + b
}
