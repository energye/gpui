package textinput

import (
	"unicode/utf8"
)

// --- mutation (committed-text path) ---

// Insert replaces the selection with s and places the caret after it. When
// a composition is active the committed replacement covers it atomically
// (design D1: commit replaces the composing span, buffer never held it).
func (e *Editor) Insert(s string) {
	if e == nil {
		return
	}
	start, end := e.sel[0], e.sel[1]
	if e.comp != nil {
		start = e.sel[1] // composition sat at the caret; it vanishes
		e.comp = nil
	}
	e.buf = e.buf[:start] + s + e.buf[end:]
	pos := start + len(s)
	e.sel = [2]int{pos, pos}
	e.changed()
}

// DeleteBackward removes one rune before the caret, or the selection.
func (e *Editor) DeleteBackward() {
	if e == nil || e.comp != nil {
		return // composition active: IME owns backspace; never touch buf
	}
	start, end := e.sel[0], e.sel[1]
	if start == end {
		if start == 0 {
			return
		}
		_, sz := utf8.DecodeLastRuneInString(e.buf[:start])
		start -= sz
	}
	if start == end {
		return
	}
	e.buf = e.buf[:start] + e.buf[end:]
	e.sel = [2]int{start, start}
	e.changed()
}

// DeleteForward removes one rune after the caret, or the selection.
func (e *Editor) DeleteForward() {
	if e == nil || e.comp != nil {
		return
	}
	start, end := e.sel[0], e.sel[1]
	if start == end {
		if end >= len(e.buf) {
			return
		}
		_, sz := utf8.DecodeRuneInString(e.buf[end:])
		end += sz
	}
	if start == end {
		return
	}
	e.buf = e.buf[:start] + e.buf[end:]
	e.sel = [2]int{start, start}
	e.changed()
}

// DeleteSurrounding removes before bytes preceding the caret and after
// bytes following it (zwp delete_surrounding_text / TSF semantics: byte
// counts relative to the caret). Boundaries snap outward to rune starts so
// a multi-byte character is never split. Grapheme-cluster refinement is a
// declared non-goal (design §0). Returns true when the buffer changed.
func (e *Editor) DeleteSurrounding(before, after int) bool {
	if e == nil || (before <= 0 && after <= 0) {
		return false
	}
	caret := e.sel[1]
	start := caret - before
	if start < 0 {
		start = 0
	}
	end := caret + after
	if end > len(e.buf) {
		end = len(e.buf)
	}
	for start > 0 && !utf8.RuneStart(e.buf[start]) {
		start--
	}
	for end < len(e.buf) && !utf8.RuneStart(e.buf[end]) {
		end++
	}
	if start >= end {
		return false
	}
	e.buf = e.buf[:start] + e.buf[end:]
	e.sel = [2]int{start, start}
	e.changed()
	return true
}

// MoveCaretRunes moves the caret by n runes (negative = left).
func (e *Editor) MoveCaretRunes(n int) {
	if e == nil {
		return
	}
	pos := e.sel[1]
	for i := 0; i < n; i++ {
		if pos >= len(e.buf) {
			break
		}
		_, sz := utf8.DecodeRuneInString(e.buf[pos:])
		pos += sz
	}
	for i := 0; i > n; i-- {
		if pos <= 0 {
			break
		}
		_, sz := utf8.DecodeLastRuneInString(e.buf[:pos])
		pos -= sz
	}
	e.SetSelection(pos, pos)
}

// SelectAll selects the whole buffer.
func (e *Editor) SelectAll() { e.SetSelection(0, len(e.Text())) }

// ByteOffsetAt converts an x offset (logical px from text origin) into the
// nearest UTF-8 byte boundary of the DISPLAY string — click-to-caret.
// Display offsets inside the composition snap out to its boundaries via
// MapViewToBuf. Heuristic measure when no Face is set (same as MeasureWidth).
func (e *Editor) ByteOffsetAt(x float64, widthOf func(string) float64) int {
	v := e.View()
	if x <= 0 || v.Display == "" {
		return v.MapViewToBuf(0)
	}
	prev := 0.0
	for idx, r := range v.Display {
		right := widthOf(v.Display[:idx+utf8.RuneLen(r)])
		if x < (prev+right)/2 {
			return v.MapViewToBuf(idx)
		}
		prev = right
	}
	return v.MapViewToBuf(len(v.Display))
}

// --- clipboard ---

// Cut removes the selection and returns the removed text.
func (e *Editor) Cut() string {
	if e == nil || e.sel[0] == e.sel[1] {
		return ""
	}
	s := e.buf[e.sel[0]:e.sel[1]]
	e.DeleteBackward()
	return s
}

// Copy returns the selected text without removing it.
func (e *Editor) Copy() string {
	if e == nil || e.sel[0] == e.sel[1] {
		return ""
	}
	return e.buf[e.sel[0]:e.sel[1]]
}

// Paste inserts s at the caret. Returns true when the buffer changed.
func (e *Editor) Paste(s string) bool {
	if e == nil || s == "" {
		return false
	}
	before := e.buf
	e.Insert(s)
	return e.buf != before
}
