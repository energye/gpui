package textinput

import "github.com/energye/gpui/ui/input"

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
