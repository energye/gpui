// Package textinput implements the engine-level editable-text controller
// (plan §6): the buffer, cursor/selection, and IME compose/commit handling
// that editable controls (kit Input/TextArea) consume.
//
// The buffer works in UTF-8 byte offsets — the same coordinate system the
// Wayland text-input protocol (zwp_text_input_v3) and the platform IME
// capability use — with rune helpers for UI navigation (arrows/home/end).
//
// Controls consume normalized input.TextEvent (committed text) and
// input.IMEEvent (compose/caret) via ApplyText/ApplyIME; this package keeps
// the editing state and fires OnChange for repaint/dirty propagation.
package textinput

import (
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/ui/input"
)

// Editor is a single-line/multi-line text editing state: buffer + cursor +
// selection + IME pre-edit region.
//
// Byte offsets (start/end) follow the Wayland text-input convention; the
// zero-offset helpers (LenRunes, RuneAt) convert for UI navigation.
type Editor struct {
	text string
	// selStart/selEnd are byte offsets. selStart == selEnd is a caret.
	selStart, selEnd int
	// composeStart/composeEnd delimit the IME pre-edit region in the buffer
	// (composeStart == -1 when no composition is active).
	composeStart, composeEnd int

	// OnChange fires after every mutation (for MarkNeedsPaint / dirty).
	OnChange func()
}

// New creates an empty editor.
func New() *Editor {
	return &Editor{composeStart: -1, composeEnd: -1}
}

// clampByte bounds an offset into [0, len(text)].
func (e *Editor) clampByte(off int) int {
	if off < 0 {
		return 0
	}
	if off > len(e.text) {
		return len(e.text)
	}
	return off
}

// Text returns the full buffer (including any active pre-edit region).
func (e *Editor) Text() string {
	if e == nil {
		return ""
	}
	return e.text
}

// LenRunes returns the buffer length in runes.
func (e *Editor) LenRunes() int {
	return utf8.RuneCountInString(e.Text())
}

// Selection returns the selection byte range (start ≤ end).
func (e *Editor) Selection() (start, end int) {
	if e == nil {
		return 0, 0
	}
	return e.selStart, e.selEnd
}

// Cursor returns the caret byte offset (= selection end).
func (e *Editor) Cursor() int {
	if e == nil {
		return 0
	}
	return e.selEnd
}

// SetText replaces the whole buffer and collapses the selection to the end.
func (e *Editor) SetText(s string) {
	if e == nil {
		return
	}
	e.text = s
	e.selStart, e.selEnd = len(s), len(s)
	e.cancelComposeLocked()
	e.fireChange()
}

// SetSelection sets the selection byte range (clamped, normalized).
func (e *Editor) SetSelection(start, end int) {
	if e == nil {
		return
	}
	start, end = e.clampByte(start), e.clampByte(end)
	if start > end {
		start, end = end, start
	}
	e.selStart, e.selEnd = start, end
	e.fireChange()
}

// SetCaret places the caret at a byte offset.
func (e *Editor) SetCaret(off int) {
	e.SetSelection(off, off)
}

// MoveCaretRunes moves the caret by n runes (negative = left).
func (e *Editor) MoveCaretRunes(n int) {
	if e == nil {
		return
	}
	pos := e.selEnd
	if n < 0 {
		pos = e.runeOffset(-n, true /*backward*/)
	} else {
		pos = e.runeOffset(n, false)
	}
	e.SetSelection(pos, pos)
}

// runeOffset returns the byte offset n runes away from selEnd.
func (e *Editor) runeOffset(n int, backward bool) int {
	pos := e.selEnd
	for i := 0; i < n; i++ {
		if backward {
			if pos <= 0 {
				break
			}
			_, sz := utf8.DecodeLastRuneInString(e.text[:pos])
			pos -= sz
		} else {
			if pos >= len(e.text) {
				break
			}
			_, sz := utf8.DecodeRuneInString(e.text[pos:])
			pos += sz
		}
	}
	return pos
}

// Insert replaces the selection with s and places the caret after it.
// This is the text-input path for committed text (keyboard chars, paste,
// IME commits via ApplyText). When an IME pre-edit is active, Insert
// replaces the whole pre-edit region (the committed replacement).
func (e *Editor) Insert(s string) {
	if e == nil {
		return
	}
	start, end := e.selStart, e.selEnd
	if e.composeStart >= 0 {
		// Replace the whole pre-edit region (compose → commit replacement).
		start, end = e.composeStart, e.composeEnd
	}
	e.cancelComposeLocked()
	e.text = e.text[:start] + s + e.text[end:]
	pos := start + len(s)
	e.selStart, e.selEnd = pos, pos
	e.fireChange()
}

// DeleteBackward removes one rune before the caret (or the selection).
// When an IME pre-edit is active it cancels the whole pre-edit region first.
func (e *Editor) DeleteBackward() {
	if e == nil {
		return
	}
	if e.composeStart >= 0 {
		e.CancelCompose()
		return
	}
	start, end := e.selStart, e.selEnd
	if start == end && start > 0 {
		_, sz := utf8.DecodeLastRuneInString(e.text[:start])
		start -= sz
	}
	if start == end {
		return
	}
	e.text = e.text[:start] + e.text[end:]
	e.selStart, e.selEnd = start, start
	e.fireChange()
}

// DeleteForward removes one rune after the caret (or the selection).
// When an IME pre-edit is active it cancels the whole pre-edit region first.
func (e *Editor) DeleteForward() {
	if e == nil {
		return
	}
	if e.composeStart >= 0 {
		e.CancelCompose()
		return
	}
	start, end := e.selStart, e.selEnd
	if start == end && end < len(e.text) {
		_, sz := utf8.DecodeRuneInString(e.text[end:])
		end += sz
	}
	if start == end {
		return
	}
	e.text = e.text[:start] + e.text[end:]
	e.selStart, e.selEnd = start, start
	e.fireChange()
}

// --- IME pre-edit handling (zwp_text_input_v3 / platform.IME) ---

// ComposeActive reports whether an IME pre-edit region is in the buffer.
func (e *Editor) ComposeActive() bool {
	return e != nil && e.composeStart >= 0
}

// ComposeRange returns the pre-edit byte range, or (0,0,false) when inactive.
func (e *Editor) ComposeRange() (start, end int, ok bool) {
	if e == nil || e.composeStart < 0 {
		return 0, 0, false
	}
	return e.composeStart, e.composeEnd, true
}

// BeginCompose starts a pre-edit at the caret (replacing the selection).
// The pre-edit text is inserted into the buffer immediately so the UI paints
// it live; CommitCompose/CancelCompose finalize or roll it back.
func (e *Editor) BeginCompose(preedit string) {
	if e == nil {
		return
	}
	e.composeStart, e.composeEnd = e.selStart, e.selEnd
	e.replaceCompose(preedit)
	e.fireChange()
}

// UpdateCompose replaces the current pre-edit text. cursor is the caret byte
// offset within the pre-edit (clamped into the region); negative = end of
// the pre-edit.
func (e *Editor) UpdateCompose(preedit string, cursor int) {
	if e == nil || e.composeStart < 0 {
		return
	}
	e.replaceCompose(preedit)
	pos := e.composeEnd // negative cursor = end of the pre-edit region
	if cursor >= 0 {
		pos = e.composeStart + cursor
		if pos < e.composeStart {
			pos = e.composeStart
		}
		if pos > e.composeEnd {
			pos = e.composeEnd
		}
	}
	e.selStart, e.selEnd = pos, pos
	e.fireChange()
}

// CommitCompose finalizes the pre-edit as committed text.
func (e *Editor) CommitCompose() {
	if e == nil || e.composeStart < 0 {
		return
	}
	e.selStart, e.selEnd = e.composeEnd, e.composeEnd
	e.cancelComposeLocked()
	e.fireChange()
}

// CancelCompose rolls back the pre-edit region.
func (e *Editor) CancelCompose() {
	if e == nil {
		return
	}
	if e.composeStart < 0 {
		return
	}
	start, end := e.composeStart, e.composeEnd
	e.text = e.text[:start] + e.text[end:]
	e.cancelComposeLocked()
	e.selStart, e.selEnd = start, start
	e.fireChange()
}

// DeleteSurrounding removes `before` bytes preceding the caret and `after`
// bytes following it (zwp_text_input_v3 delete_surrounding_text semantics:
// byte counts relative to the caret). Boundaries snap outward to UTF-8 rune
// starts so a multi-byte character is never split. A deletion crossing the
// active pre-edit region cancels the composition; otherwise the compose
// offsets shift with the removed span. Returns true when the buffer changed.
func (e *Editor) DeleteSurrounding(before, after int) bool {
	if e == nil || (before <= 0 && after <= 0) {
		return false
	}
	caret := e.selEnd
	start := caret - before
	if start < 0 {
		start = 0
	}
	end := caret + after
	if end > len(e.text) {
		end = len(e.text)
	}
	for start > 0 && !utf8.RuneStart(e.text[start]) {
		start--
	}
	for end < len(e.text) && !utf8.RuneStart(e.text[end]) {
		end++
	}
	if start >= end {
		return false
	}
	composeActive := e.composeStart >= 0
	overlap := composeActive && start < e.composeEnd && e.composeStart < end
	removed := end - start
	e.text = e.text[:start] + e.text[end:]
	if composeActive {
		if overlap {
			e.cancelComposeLocked()
		} else {
			if e.composeStart >= end {
				e.composeStart -= removed
			}
			if e.composeEnd >= end {
				e.composeEnd -= removed
			}
			if e.composeEnd < e.composeStart {
				e.cancelComposeLocked()
			}
		}
	}
	e.selStart, e.selEnd = start, start
	e.fireChange()
	return true
}

// replaceCompose swaps the current pre-edit region for preedit, keeping the
// region's start and extending its end to the inserted text.
func (e *Editor) replaceCompose(preedit string) {
	start, end := e.composeStart, e.composeEnd
	if start < 0 || end < start {
		start = e.selStart
		end = e.selEnd
	}
	e.text = e.text[:start] + preedit + e.text[end:]
	e.composeStart, e.composeEnd = start, start+len(preedit)
}

func (e *Editor) cancelComposeLocked() {
	e.composeStart, e.composeEnd = -1, -1
}

func (e *Editor) fireChange() {
	if e.OnChange != nil {
		e.OnChange()
	}
}

// ApplyText handles a committed-text event (keyboard chars, paste, IME
// commit surfaced as input.KindText). Returns true when the buffer changed.
func (e *Editor) ApplyText(ev input.TextEvent) bool {
	if e == nil {
		return false
	}
	if ev.Text == "" {
		return false
	}
	before := e.text
	e.Insert(ev.Text)
	return e.text != before
}

// ApplyIME handles a normalized IME event (compose/commit/caret).
// Returns true when the buffer changed.
func (e *Editor) ApplyIME(ev input.IMEEvent) bool {
	if e == nil {
		return false
	}
	before := e.text
	switch ev.Kind {
	case input.IMECompose:
		// Empty pre-edit text means "reset the pre-edit" (zwp_text_input_v3
		// preedit_string semantics): end the active session instead of
		// starting/updating an empty one — otherwise a stray empty preedit
		// after a commit latches compose state on and blocks plain typing.
		// A commit_string may follow in the same round and inserts fresh.
		if ev.Text == "" {
			e.CancelCompose()
			break
		}
		if !e.ComposeActive() {
			e.BeginCompose(ev.Text)
		}
		e.UpdateCompose(ev.Text, caretFromIME(ev))
	case input.IMECommit:
		if ev.Text != "" {
			// Commit replaces the whole pre-edit region with the committed text.
			if e.ComposeActive() {
				e.selStart, e.selEnd = e.composeStart, e.composeEnd
			}
			e.Insert(ev.Text)
		} else {
			e.CommitCompose()
		}
	case input.IMECaretMove:
		if ev.Start >= 0 && ev.End >= 0 {
			e.SetSelection(ev.Start, ev.End)
		}
	case input.IMEDeleteSurrounding:
		before, after := -ev.Start, ev.End
		if before < 0 {
			before = 0
		}
		if after < 0 {
			after = 0
		}
		e.DeleteSurrounding(before, after)
	}
	return e.text != before
}

// caretFromIME extracts the pre-edit caret byte offset carried by a compose
// event (protocol index; negative = end of the pre-edit string).
func caretFromIME(ev input.IMEEvent) int {
	return ev.Start
}

// --- clipboard integration (optional; platform.Clipboard capability) ---

// Cut removes the selection and returns the removed text.
func (e *Editor) Cut() string {
	if e == nil {
		return ""
	}
	if e.selStart == e.selEnd {
		return ""
	}
	s := e.text[e.selStart:e.selEnd]
	e.DeleteBackward()
	return s
}

// Copy returns the selected text without removing it.
func (e *Editor) Copy() string {
	if e == nil || e.selStart == e.selEnd {
		return ""
	}
	return e.text[e.selStart:e.selEnd]
}

// Paste inserts s at the caret (no clipboard dependency; callers wire
// platform.Clipboard). Returns true when the buffer changed.
func (e *Editor) Paste(s string) bool {
	if e == nil || s == "" {
		return false
	}
	before := e.text
	e.Insert(s)
	return e.text != before
}

// SelectAll selects the whole buffer.
func (e *Editor) SelectAll() {
	if e == nil {
		return
	}
	e.SetSelection(0, len(e.text))
}

// String implements fmt.Stringer.
func (e *Editor) String() string {
	if e == nil {
		return ""
	}
	return e.text
}

// IsEmpty reports an empty buffer.
func (e *Editor) IsEmpty() bool {
	return e == nil || e.text == ""
}

// Trimmed returns strings.TrimSpace(e.text) (used for value validation).
func (e *Editor) Trimmed() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.text)
}
