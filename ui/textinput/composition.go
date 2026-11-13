//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package textinput

import "github.com/energye/gpui/ui/input"

func (e *Editor) ApplyIME(ev input.IMEEvent) bool {
	if e == nil {
		return false
	}
	before := e.docVersion
	switch ev.Kind {
	case input.IMECompose:
		if ev.Text == "" {
			if e.composing {
				e.EndComposing()
			}
			return e.docVersion != before
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
		// F-D5: 平台 delete_surrounding 的 before/after 为 UTF8 字节数，需按 code point 换算，RuneStart 吸附避免劈半
		b := e.ensureBuf()
		caretByte := b.ByteOffsetForUTF16(e.selection.Extent)
		beforeRunes := 0
		if beforeBytes > 0 {
			start := caretByte - beforeBytes
			if start < 0 {
				start = 0
			}
			for start > 0 && start < b.Len() && (b.ByteAt(start)&0xC0) == 0x80 {
				start--
			}
			if start < caretByte {
				beforeRunes = len([]rune(b.Slice(start, caretByte)))
			}
		}
		afterRunes := 0
		if afterBytes > 0 {
			end := caretByte + afterBytes
			if end > b.Len() {
				end = b.Len()
			}
			for end > caretByte && end < b.Len() && (b.ByteAt(end)&0xC0) == 0x80 {
				end--
			}
			if end > caretByte {
				afterRunes = len([]rune(b.Slice(caretByte, end)))
			}
		}
		e.DeleteSurrounding(-beforeRunes, beforeRunes+afterRunes)
	}
	return e.docVersion != before
}

func (e *Editor) ApplyText(ev input.TextEvent) bool {
	if e == nil || ev.Text == "" {
		return false
	}
	before := e.docVersion
	e.AddText(ev.Text)
	return e.docVersion != before
}
