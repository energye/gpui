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
		beforeBytes, afterBytes := -ev.Start, ev.End
		if beforeBytes < 0 {
			beforeBytes = 0
		}
		if afterBytes < 0 {
			afterBytes = 0
		}
		// F-D5: 平台 delete_surrounding 的 before/after 为 UTF8 字节数，需按 code point 换算
		caretByte := byteOffsetForUtf16(e.text, e.selection.Extent)
		beforeRunes := 0
		if beforeBytes > 0 && caretByte >= beforeBytes {
			beforeRunes = len([]rune(e.text[caretByte-beforeBytes : caretByte]))
		} else if beforeBytes > 0 {
			beforeRunes = len([]rune(e.text[:caretByte]))
			if beforeRunes > beforeBytes {
				beforeRunes = beforeBytes
			}
		}
		afterRunes := 0
		if afterBytes > 0 && caretByte+afterBytes <= len(e.text) {
			afterRunes = len([]rune(e.text[caretByte : caretByte+afterBytes]))
		} else if afterBytes > 0 {
			afterRunes = len([]rune(e.text[caretByte:]))
			if afterRunes > afterBytes {
				afterRunes = afterBytes
			}
		}
		e.DeleteSurrounding(-beforeRunes, beforeRunes+afterRunes)
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
