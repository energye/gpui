// Package textinput implements the engine-level editable-text core
// (ENGINE_IME_MODERN_STANDARD.md §4.4/§4.5): the committed-text buffer is
// the single source of truth and NEVER contains IME pre-edit text — the
// composition lives in an overlay (comp) rendered between buffer segments.
// All buffer↔display offset conversion goes through ComposedView.
package textinput

import (
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/ui/input"
)

// SegmentAttr classifies composition styling (maps IME attribute sets).
// Alias of the normalized vocabulary in ui/input (design §4.1: one
// vocabulary, no parallel types).
type SegmentAttr = uint8

const (
	// AttrUnderline default composition highlight.
	AttrUnderline SegmentAttr = iota
	// AttrClauseActive the clause being converted (thicker underline).
	AttrClauseActive
	// AttrConverted already-converted text inside the composition.
	AttrConverted
)

// Composition is the live pre-edit overlay. nil (on Editor) = no session.
// Segments use the input.Segment type (single vocabulary rule).
type Composition struct {
	Text   string
	Cursor int            // caret byte offset within Text (<0 = end)
	Segs   []input.Segment
}

// Editor is the editing state: committed buffer + selection + optional
// composition overlay. NOT concurrency-safe; all mutations must run on the
// event-loop thread (design §4.0 C1).
type Editor struct {
	buf   string
	sel   [2]int // byte offsets into buf; sel[0]==sel[1] is a caret
	comp  *Composition
	epoch uint64 // increments on every real change; monotonic forever

	// Sticky column for vertical caret movement (Flutter's desired-X):
	// adopted on the first Up/Down of a run, reset by horizontal moves,
	// clicks and edits. See MoveCaretVertically / ResetCaretColumn.
	caretCol       float64
	caretColValid  bool

	// OnChange fires after every mutation (repaint / anchor refresh).
	OnChange func()
}

// New creates an empty editor.
func New() *Editor { return &Editor{} }

func (e *Editor) changed() {
	e.epoch++
	if e != nil && e.OnChange != nil {
		e.OnChange()
	}
}

// Epoch returns the monotonically increasing change counter (echo-suppression
// evidence; design D2/P1-5). Never resets, not even on SetText.
func (e *Editor) Epoch() uint64 {
	if e == nil {
		return 0
	}
	return e.epoch
}

// clamp bounds off into [0, len(buf)].
func (e *Editor) clamp(off int) int {
	if off < 0 {
		return 0
	}
	if off > len(e.buf) {
		return len(e.buf)
	}
	return off
}

// --- ComposedView (design §4.5): THE only offset-conversion exit ---

// ComposedView is the display-form snapshot of the editor state plus the
// buffer↔display mapping for the one composition span.
type ComposedView struct {
	Display string // buf[:split] + comp.Text + buf[split:]
	// CompStart/CompEnd delimit the composition span in DISPLAY offsets.
	// CompStart == -1 when no composition is active.
	CompStart, CompEnd int
}

// View returns the current display form.
func (e *Editor) View() ComposedView {
	if e == nil {
		return ComposedView{}
	}
	if e.comp == nil {
		return ComposedView{Display: e.buf, CompStart: -1, CompEnd: -1}
	}
	at := e.sel[1] // composition always sits at the caret
	v := ComposedView{
		Display:   e.buf[:at] + e.comp.Text + e.buf[at:],
		CompStart: at,
	}
	v.CompEnd = at + len(e.comp.Text)
	return v
}

// MapBufToView converts a buffer offset to display coordinates. Offsets at
// or before the composition start pass through; offsets after it shift by
// the composition length (the span occupies display space the buffer does
// not have). There is no "inside" case in buffer coordinates — the span is
// pure overlay, invisible to buf indices.
func (v ComposedView) MapBufToView(off int) int {
	if v.CompStart < 0 || off <= v.CompStart {
		return off
	}
	return off + (v.CompEnd - v.CompStart)
}

// MapBufToViewExact converts precisely: buffer offsets inside the
// composition map onto their position within comp.Text (used by IME-driven
// caret positioning), unlike MapBufToView which snaps into the span edge.
func (v ComposedView) MapBufToViewExact(bufOff, compOff int) int {
	if v.CompStart < 0 {
		return bufOff
	}
	if bufOff >= v.CompStart {
		return v.CompStart + compOff
	}
	return bufOff
}

// MapViewToBuf converts a display offset back to buffer coordinates.
// Display offsets inside the composition snap OUT to its boundaries:
// before/at start → start; after end → end. Used by click-to-caret so the
// caret never sits mid-composition from a mouse action.
//
// NOTE the clamp uses len(Display) only as an outer guard; the >=CompEnd
// branch must be tested BEFORE any clamping to it, otherwise offsets just
// past the span collapse into the in-span case (found by mapping tests).
func (v ComposedView) MapViewToBuf(off int) int {
	switch {
	case v.CompStart < 0:
		return max(0, min(off, len(v.Display)))
	case off <= v.CompStart:
		return max(0, off)
	case off >= v.CompEnd:
		return off - (v.CompEnd - v.CompStart)
	default: // strictly inside the span
		return v.CompStart
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- basic accessors ---

// Text returns the committed buffer (never contains pre-edit text).
func (e *Editor) Text() string {
	if e == nil {
		return ""
	}
	return e.buf
}

// Cursor returns the caret as a buffer offset (= selection end).
func (e *Editor) Cursor() int {
	if e == nil {
		return 0
	}
	return e.sel[1]
}

// Selection returns the selection byte range (start ≤ end).
func (e *Editor) Selection() (start, end int) {
	if e == nil {
		return 0, 0
	}
	return e.sel[0], e.sel[1]
}

// SetSelection sets the selection (clamped, normalized).
func (e *Editor) SetSelection(start, end int) {
	if e == nil {
		return
	}
	start, end = e.clamp(start), e.clamp(end)
	if start > end {
		start, end = end, start
	}
	e.sel = [2]int{start, end}
	e.changed()
}

// SetCaret places the caret at a buffer offset.
func (e *Editor) SetCaret(off int) {
	e.ResetCaretColumn() // any explicit placement starts a fresh vertical run
	e.SetSelection(off, off)
}

// LenRunes returns the buffer length in runes.
func (e *Editor) LenRunes() int { return utf8.RuneCountInString(e.Text()) }

// IsEmpty reports an empty buffer.
func (e *Editor) IsEmpty() bool { return e == nil || e.buf == "" }

// Trimmed returns strings.TrimSpace(buf).
func (e *Editor) Trimmed() string { return strings.TrimSpace(e.Text()) }

// SetText replaces the whole buffer, collapsing selection to the end and
// dropping any live composition.
func (e *Editor) SetText(s string) {
	if e == nil {
		return
	}
	e.buf = s
	e.sel = [2]int{len(s), len(s)}
	e.comp = nil
	e.changed()
}

// String implements fmt.Stringer (committed text).
func (e *Editor) String() string { return e.Text() }
