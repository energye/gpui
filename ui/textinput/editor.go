package textinput

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// TextRange mirrors Flutter's TextRange with affinity.
type TextRange struct {
	Base     int
	Extent   int
	Affinity int // 0 downstream, 1 upstream
}

func (r TextRange) Start() int {
	if r.Base < r.Extent {
		return r.Base
	}
	return r.Extent
}
func (r TextRange) End() int {
	if r.Base > r.Extent {
		return r.Base
	}
	return r.Extent
}
func (r TextRange) Collapsed() bool { return r.Base == r.Extent }
func (r TextRange) Length() int {
	if r.Base > r.Extent {
		return r.Base - r.Extent
	}
	return r.Extent - r.Base
}
func (r TextRange) Contains(off int) bool { return off >= r.Start() && off < r.End() }

// Editor is Flutter TextInputModel four-tuple.
// All offsets are UTF-16 code units (Flutter/Dart). Go string holds UTF-8;
// conversions happen at boundaries.
type Editor struct {
	text           string
	selection      TextRange
	composingRange TextRange
	composing      bool

	batchDepth         int
	lastFrameworkText  string
	lastFrameworkSel   TextRange
	lastFrameworkComp  TextRange
	epoch              uint64
	pendingEpochBump   bool
	caretCol           float64
	caretColValid      bool
	OnChange           func()
}

func New() *Editor { return &Editor{} }

// --- utf16 helpers ---

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r <= 0xFFFF {
			n++
		} else {
			n += 2
		}
	}
	return n
}

func byteOffsetForUtf16(s string, utf16Off int) int {
	if utf16Off <= 0 {
		return 0
	}
	off := 0
	cu := 0
	for _, r := range s {
		sz := utf8.RuneLen(r)
		cuLen := 1
		if r > 0xFFFF {
			cuLen = 2
		}
		if cu+cuLen > utf16Off {
			break
		}
		cu += cuLen
		off += sz
		if cu >= utf16Off {
			break
		}
	}
	if cu < utf16Off {
		return len(s)
	}
	return off
}

func utf16OffsetForByte(s string, byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	if byteOff > len(s) {
		byteOff = len(s)
	}
	cu := 0
	for _, r := range s[:byteOff] {
		if r > 0xFFFF {
			cu += 2
		} else {
			cu++
		}
	}
	// handle split surrogate not needed as we iterate runes
	_ = utf16.Encode
	return cu
}

func clampUtf16(s string, off int) int {
	n := utf16Len(s)
	if off < 0 {
		return 0
	}
	if off > n {
		return n
	}
	return off
}

func (e *Editor) changed() {
	if e == nil {
		return
	}
	if e.batchDepth > 0 {
		e.pendingEpochBump = true
		return
	}
	e.epoch++
	if e.OnChange != nil {
		e.OnChange()
	}
}

// Epoch returns monotonic counter.
func (e *Editor) Epoch() uint64 {
	if e == nil {
		return 0
	}
	return e.epoch
}

func (e *Editor) clamp(off int) int {
	if e == nil {
		return 0
	}
	// legacy byte clamp
	if off < 0 {
		return 0
	}
	if off > len(e.text) {
		return len(e.text)
	}
	return off
}

// --- new API ---

func (e *Editor) GetText() string {
	if e == nil {
		return ""
	}
	return e.text
}

func (e *Editor) GetCursorOffset() int {
	if e == nil {
		return 0
	}
	// byte offset of selection.extent
	return byteOffsetForUtf16(e.text, e.selection.Extent)
}

func (e *Editor) TextRange() TextRange {
	if e == nil {
		return TextRange{}
	}
	return TextRange{Base: 0, Extent: utf16Len(e.text)}
}

func (e *Editor) EditableRange() TextRange {
	if e == nil {
		return TextRange{}
	}
	if e.composing {
		return e.composingRange
	}
	return e.TextRange()
}

func (e *Editor) ShouldSkipFrameworkUpdate(text string, sel, comp TextRange) bool {
	if e == nil {
		return false
	}
	return text == e.lastFrameworkText && sel == e.lastFrameworkSel && comp == e.lastFrameworkComp
}

func (e *Editor) markFrameworkSynced() {
	if e == nil {
		return
	}
	e.lastFrameworkText = e.text
	e.lastFrameworkSel = e.selection
	e.lastFrameworkComp = e.composingRange
}

// SetText mirrors Flutter text_input_model: direct override, used by set_editing_state.
// sel/comp may be -1/-1 sentinel meaning collapsed 0,0 or EndComposing.
func (e *Editor) SetEditingState(text string, sel, comp TextRange, affinity int) bool {
	return e.SetTextWithSelection(text, sel, comp, affinity)
}
func (e *Editor) SetTextWithSelection(text string, sel, comp TextRange, affinity int) bool {
	if e == nil {
		return false
	}
	// -1 sentinel
	if sel.Base == -1 && sel.Extent == -1 {
		sel = TextRange{Base: 0, Extent: 0, Affinity: affinity}
	}
	if comp.Base == -1 && comp.Extent == -1 {
		// EndComposing case handled after assignment
		comp = TextRange{}
		e.composing = false
	} else if comp.Collapsed() && comp.Base == 0 && comp.Extent == 0 && !e.composing {
		// no composing
	} else if !comp.Collapsed() || comp.Base != 0 || comp.Extent != 0 {
		// has composing range
	}
	sel.Affinity = affinity
	// clamp
	_ = utf16Len(text)
	sel.Base = clampUtf16(text, sel.Base)
	sel.Extent = clampUtf16(text, sel.Extent)
	if comp.Base < 0 || comp.Extent < 0 {
		// sentinel already handled
	} else {
		comp.Base = clampUtf16(text, comp.Base)
		comp.Extent = clampUtf16(text, comp.Extent)
	}
	changed := e.text != text || e.selection != sel || e.composingRange != comp || e.composing != (comp.Length() > 0 || e.composing)
	// determine composing flag: collapsed comp means no composing unless explicitly true? spec: composing bool separate
	// For SetText, composing = !comp.Collapsed() (or sentinel)
	hasComp := comp.Length() > 0
	e.text = text
	e.selection = sel
	e.composingRange = comp
	e.composing = hasComp
	if changed {
		e.changed()
	}
	e.ResetCaretColumn()
	return changed
}

func (e *Editor) SetSelectionRange(r TextRange) bool {
	if e == nil {
		return false
	}
	if e.composing && !r.Collapsed() {
		return false
	}
	er := e.EditableRange()
	// clamp to editable range
	r.Base = clampUtf16(e.text, r.Base)
	r.Extent = clampUtf16(e.text, r.Extent)
	if r.Start() < er.Start() || r.End() > er.End() {
		// clamp into editable range
		if r.Base < er.Start() {
			r.Base = er.Start()
		}
		if r.Base > er.End() {
			r.Base = er.End()
		}
		if r.Extent < er.Start() {
			r.Extent = er.Start()
		}
		if r.Extent > er.End() {
			r.Extent = er.End()
		}
	}
	if e.selection == r {
		return true
	}
	e.selection = r
	e.changed()
	e.ResetCaretColumn()
	return true
}

func (e *Editor) SetComposingRange(r TextRange, cursorOffset int) bool {
	if e == nil || !e.composing {
		return false
	}
	// r must be within text
	n := utf16Len(e.text)
	if r.Start() < 0 || r.End() > n {
		return false
	}
	e.composingRange = r
	// selection = r.Start + cursorOffset
	off := r.Start() + cursorOffset
	off = clampUtf16(e.text, off)
	e.selection = TextRange{Base: off, Extent: off}
	e.changed()
	return true
}

func (e *Editor) BeginComposing() {
	if e == nil || e.composing {
		return
	}
	e.composing = true
	e.composingRange = TextRange{Base: e.selection.Extent, Extent: e.selection.Extent}
	e.changed()
}

func (e *Editor) UpdateComposingText(text string, sel TextRange) {
	if e == nil {
		return
	}
	if text == "" && e.composingRange.Collapsed() {
		return // no-op per F-D1
	}
	var replaceRange TextRange
	if e.composingRange.Collapsed() {
		replaceRange = e.selection
	} else {
		replaceRange = e.composingRange
	}
	startByte := byteOffsetForUtf16(e.text, replaceRange.Start())
	endByte := byteOffsetForUtf16(e.text, replaceRange.End())
	newText := e.text[:startByte] + text + e.text[endByte:]
	// new composing range covers inserted text
	newStart := replaceRange.Start()
	newEnd := newStart + utf16Len(text)
	// clamp sel
	sel.Base = clampUtf16(newText, sel.Base)
	sel.Extent = clampUtf16(newText, sel.Extent)
	e.text = newText
	e.composing = true
	e.composingRange = TextRange{Base: newStart, Extent: newEnd}
	e.selection = sel
	e.changed()
	e.ResetCaretColumn()
}

func (e *Editor) CommitComposing() {
	if e == nil || !e.composing {
		return
	}
	e.composing = false
	e.composingRange = TextRange{}
	e.changed()
}

func (e *Editor) EndComposing() {
	if e == nil || !e.composing {
		return
	}
	// delete preedit text on cancel (empty preedit semantics)
	s := byteOffsetForUtf16(e.text, e.composingRange.Start())
	en := byteOffsetForUtf16(e.text, e.composingRange.End())
	if s != en {
		e.text = e.text[:s] + e.text[en:]
		e.selection = TextRange{Base: e.composingRange.Start(), Extent: e.composingRange.Start()}
	}
	e.composing = false
	e.composingRange = TextRange{}
	e.changed()
}

func (e *Editor) BeginBatchEdit() {
	if e == nil {
		return
	}
	e.batchDepth++
}

func (e *Editor) EndBatchEdit() {
	if e == nil || e.batchDepth == 0 {
		return
	}
	e.batchDepth--
	if e.batchDepth == 0 && e.pendingEpochBump {
		e.pendingEpochBump = false
		e.epoch++
		if e.OnChange != nil {
			e.OnChange()
		}
	}
}

func (e *Editor) DeleteSelected() bool {
	if e == nil || e.selection.Collapsed() {
		return false
	}
	if e.composing && !e.selection.Collapsed() {
		return false
	}
	start := byteOffsetForUtf16(e.text, e.selection.Start())
	end := byteOffsetForUtf16(e.text, e.selection.End())
	if start == end {
		return false
	}
	// also clear composing range if collapsed? spec F-S1: DeleteSelected then erase composingRange if composing
	if e.composing {
		// selection is collapsed when composing, so this path not taken; but keep for completeness
		cs := byteOffsetForUtf16(e.text, e.composingRange.Start())
		ce := byteOffsetForUtf16(e.text, e.composingRange.End())
		// erase composing range
		if cs < ce {
			// adjust start/end if overlapping? simplified
			e.text = e.text[:cs] + e.text[ce:]
			// selection already at start
		}
	}
	e.text = e.text[:start] + e.text[end:]
	off := e.selection.Start()
	e.selection = TextRange{Base: off, Extent: off}
	if e.composing {
		// update composingRange to collapsed at new caret
		e.composingRange = TextRange{Base: off, Extent: off}
	}
	e.changed()
	return true
}

func (e *Editor) AddText(text string) {
	if e == nil || text == "" {
		return
	}
	er := e.EditableRange()
	if e.selection.Start() < er.Start() || e.selection.End() > er.End() {
		return
	}
	// per F-C4: read-only not modeled here; input_type NONE handled at session layer
	var replaceRange TextRange
	if e.composing {
		replaceRange = e.composingRange
	} else {
		replaceRange = e.selection
	}
	startByte := byteOffsetForUtf16(e.text, replaceRange.Start())
	endByte := byteOffsetForUtf16(e.text, replaceRange.End())
	e.text = e.text[:startByte] + text + e.text[endByte:]
	newOff := replaceRange.Start() + utf16Len(text)
	e.selection = TextRange{Base: newOff, Extent: newOff}
	if e.composing {
		// Commit after replace? Spec F-S3: atomic replace then composing cleared? Keep composing cleared for AddText commit path
		e.composing = false
		e.composingRange = TextRange{}
	}
	e.changed()
	e.ResetCaretColumn()
}

func (e *Editor) DeleteSurrounding(before, after int) bool {
	if e == nil {
		return false
	}
	er := e.EditableRange()
	caret := e.selection.Extent
	// before/after are byte-legacy but we treat as UTF16 units for now; handle legacy (before>0, after>=0) and new (offset,count)
	var start, end int
	if before >= 0 && after >= 0 && (before > 0 || after > 0) && before <= 10000 {
		// legacy: before = bytes before caret, after = bytes after (both >=0)
		// Interpret as delete before+after around caret; use byte snap later
		// Convert before/after byte counts to UTF16 via rune snaps: approximate by runes
		// We implement snaps at byte level after conversion
		caretByte := byteOffsetForUtf16(e.text, caret)
		sByte := caretByte - before
		if sByte < 0 {
			sByte = 0
		}
		eByte := caretByte + after
		if eByte > len(e.text) {
			eByte = len(e.text)
		}
		// snap outward to rune start
		for sByte > 0 && !utf8.RuneStart(e.text[sByte]) {
			sByte--
		}
		for eByte < len(e.text) && !utf8.RuneStart(e.text[eByte]) {
			eByte++
		}
		if sByte >= eByte {
			return false
		}
		start = utf16OffsetForByte(e.text, sByte)
		end = utf16OffsetForByte(e.text, eByte)
	} else {
		start = caret + before
		end = start + after
	}
	if start < er.Start() {
		start = er.Start()
	}
	if end > er.End() {
		end = er.End()
	}
	if start >= end {
		return false
	}
	startByte := byteOffsetForUtf16(e.text, start)
	endByte := byteOffsetForUtf16(e.text, end)
	if startByte == endByte {
		return false
	}
	// adjust selection and composingRange
	e.text = e.text[:startByte] + e.text[endByte:]
	delta := end - start
	if before > 0 {
		e.selection = TextRange{Base: start, Extent: start}
	} else if before == 0 && after > 0 {
		// caret stays
	} else {
		if before <= 0 {
			e.selection = TextRange{Base: start, Extent: start}
		}
	}
	if e.composing {
		if e.composingRange.End() > start {
			newEnd := e.composingRange.End() - delta
			if newEnd < e.composingRange.Start() {
				newEnd = e.composingRange.Start()
			}
			e.composingRange = TextRange{Base: e.composingRange.Start(), Extent: newEnd}
		}
	}
	e.changed()
	return true
}

func (e *Editor) Backspace() bool {
	if e == nil {
		return false
	}
	if e.composing {
		// Inside composing, delete one code point before caret within composingRange
		if e.selection.Extent <= e.composingRange.Start() {
			return false
		}
		caret := e.selection.Extent
		prev := caret - 1
		// handle surrogate pair: if text at prev is high surrogate, need 2
		// Check rune at byte offset
		b := byteOffsetForUtf16(e.text, prev)
		// peek rune
		r, _ := utf8.DecodeRuneInString(e.text[b:])
		_ = r
		// For surrogate pair (r > 0xFFFF), prev should be 2 units; adjust
		// Detect by checking utf16Len of that rune
		if r > 0xFFFF && prev > e.composingRange.Start() {
			prev--
		}
		return e.DeleteSurrounding(prev-caret, caret-prev)
	}
	if !e.selection.Collapsed() {
		return e.DeleteSelected()
	}
	caret := e.selection.Extent
	if caret == 0 {
		return false
	}
	// move one code point back
	b := byteOffsetForUtf16(e.text, caret)
	_, sz := utf8.DecodeLastRuneInString(e.text[:b])
	r, _ := utf8.DecodeLastRuneInString(e.text[:b])
	cu := 1
	if r > 0xFFFF {
		cu = 2
	}
	_ = sz
	return e.DeleteSurrounding(-cu, cu)
}

func (e *Editor) Delete() bool {
	if e == nil {
		return false
	}
	if e.composing {
		if e.selection.Extent >= e.composingRange.End() {
			return false
		}
		caret := e.selection.Extent
		b := byteOffsetForUtf16(e.text, caret)
		r, _ := utf8.DecodeRuneInString(e.text[b:])
		cu := 1
		if r > 0xFFFF {
			cu = 2
		}
		return e.DeleteSurrounding(0, cu)
	}
	if !e.selection.Collapsed() {
		return e.DeleteSelected()
	}
	caret := e.selection.Extent
	if caret >= utf16Len(e.text) {
		return false
	}
	b := byteOffsetForUtf16(e.text, caret)
	r, _ := utf8.DecodeRuneInString(e.text[b:])
	cu := 1
	if r > 0xFFFF {
		cu = 2
	}
	return e.DeleteSurrounding(0, cu)
}

func (e *Editor) MoveCursorToBeginning() bool {
	if e == nil {
		return false
	}
	er := e.EditableRange()
	return e.SetSelectionRange(TextRange{Base: er.Start(), Extent: er.Start()})
}
func (e *Editor) MoveCursorToEnd() bool {
	if e == nil {
		return false
	}
	er := e.EditableRange()
	return e.SetSelectionRange(TextRange{Base: er.End(), Extent: er.End()})
}
func (e *Editor) MoveCursorBack() bool {
	if e == nil {
		return false
	}
	if e.selection.Extent <= e.EditableRange().Start() {
		return false
	}
	caret := e.selection.Extent
	b := byteOffsetForUtf16(e.text, caret)
	_, sz := utf8.DecodeLastRuneInString(e.text[:b])
	r, _ := utf8.DecodeLastRuneInString(e.text[:b])
	_ = sz
	cu := 1
	if r > 0xFFFF {
		cu = 2
	}
	return e.SetSelectionRange(TextRange{Base: caret - cu, Extent: caret - cu})
}
func (e *Editor) MoveCursorForward() bool {
	if e == nil {
		return false
	}
	if e.selection.Extent >= e.EditableRange().End() {
		return false
	}
	caret := e.selection.Extent
	b := byteOffsetForUtf16(e.text, caret)
	r, _ := utf8.DecodeRuneInString(e.text[b:])
	cu := 1
	if r > 0xFFFF {
		cu = 2
	}
	return e.SetSelectionRange(TextRange{Base: caret + cu, Extent: caret + cu})
}

// --- legacy shims for existing call sites & tests ---

func (e *Editor) Text() string {
	if e == nil {
		return ""
	}
	// legacy: committed buffer (strip composing range)
	if !e.composing || e.composingRange.Collapsed() {
		return e.text
	}
	s := byteOffsetForUtf16(e.text, e.composingRange.Start())
	en := byteOffsetForUtf16(e.text, e.composingRange.End())
	return e.text[:s] + e.text[en:]
}

func (e *Editor) Cursor() int {
	if e == nil {
		return 0
	}
	// legacy byte offset
	if e.composing {
		// map selection inside composing to committed caret
		s := byteOffsetForUtf16(e.text, e.composingRange.Start())
		off := e.selection.Extent - e.composingRange.Start()
		// convert off utf16 to bytes within composing text
		compText := e.text[byteOffsetForUtf16(e.text, e.composingRange.Start()):byteOffsetForUtf16(e.text, e.composingRange.End())]
		b := byteOffsetForUtf16(compText, off)
		return s + b
	}
	return byteOffsetForUtf16(e.text, e.selection.Extent)
}

func (e *Editor) Selection() (int, int) {
	if e == nil {
		return 0, 0
	}
	if e.composing {
		// legacy selection is in committed coords; when composing, selection is collapsed at caret in committed buffer
		c := e.Cursor()
		return c, c
	}
	s := byteOffsetForUtf16(e.text, e.selection.Start())
	en := byteOffsetForUtf16(e.text, e.selection.End())
	return s, en
}

func (e *Editor) SetCaret(off int) {
	if e == nil {
		return
	}
	off = e.clamp(off)
	cu := utf16OffsetForByte(e.text, off)
	e.SetSelectionRange(TextRange{Base: cu, Extent: cu})
}

func (e *Editor) SetSelection(start, end int) {
	if e == nil {
		return
	}
	start = e.clamp(start)
	end = e.clamp(end)
	cuStart := utf16OffsetForByte(e.text, start)
	cuEnd := utf16OffsetForByte(e.text, end)
	e.SetSelectionRange(TextRange{Base: cuStart, Extent: cuEnd})
}

func (e *Editor) SetSelectionLegacy(start, end int) { e.SetSelection(start, end) }

func (e *Editor) SetSelectionBytes(start, end int) { e.SetSelection(start, end) }

// SetSelection with two ints preserved for old tests (byte offsets)
func (e *Editor) SetSelectionInts(start, end int) { e.SetSelection(start, end) }

// New spec alias
func (e *Editor) SetSelectionTR(r TextRange) bool { return e.SetSelectionRange(r) }

// LenRunes legacy
func (e *Editor) LenRunes() int { return utf8.RuneCountInString(e.Text()) }

// SetText legacy: old single-arg API used by tests (committed buffer)
func (e *Editor) SetText(s string) {
	e.SetTextLegacy(s)
}
func (e *Editor) SetTextLegacy(s string) {
	if e == nil {
		return
	}
	e.text = s
	e.selection = TextRange{Base: utf16Len(s), Extent: utf16Len(s)}
	e.composing = false
	e.composingRange = TextRange{}
	e.changed()
	e.ResetCaretColumn()
}

// Insert legacy (committed path)
func (e *Editor) Insert(s string) {
	if e == nil {
		return
	}
	e.AddText(s)
}

func (e *Editor) DeleteBackward() {
	e.Backspace()
}
func (e *Editor) DeleteForward() {
	e.Delete()
}

func (e *Editor) MoveCaretRunes(n int) {
	if n == 0 {
		return
	}
	e.ResetCaretColumn()
	for i := 0; i < n; i++ {
		e.MoveCursorForward()
	}
	for i := 0; i > n; i-- {
		e.MoveCursorBack()
	}
}

func (e *Editor) SelectAll() {
	if e == nil {
		return
	}
	n := utf16Len(e.text)
	er := e.EditableRange()
	_ = er
	e.SetSelectionRange(TextRange{Base: 0, Extent: n})
}

func (e *Editor) Copy() string {
	if e == nil || e.selection.Collapsed() {
		return ""
	}
	s := byteOffsetForUtf16(e.text, e.selection.Start())
	en := byteOffsetForUtf16(e.text, e.selection.End())
	return e.text[s:en]
}
func (e *Editor) Cut() string {
	if e == nil || e.selection.Collapsed() {
		return ""
	}
	s := e.Copy()
	e.DeleteSelected()
	return s
}
func (e *Editor) Paste(s string) bool {
	if e == nil || s == "" {
		return false
	}
	before := e.text
	e.AddText(s)
	return e.text != before
}

func (e *Editor) ComposeActive() bool { return e != nil && e.composing }
func (e *Editor) CompositionText() string {
	if e == nil || !e.composing {
		return ""
	}
	s := byteOffsetForUtf16(e.text, e.composingRange.Start())
	en := byteOffsetForUtf16(e.text, e.composingRange.End())
	return e.text[s:en]
}
func (e *Editor) Snapshot() (string, int) { return e.Text(), e.Cursor() }

// View and mapping for legacy ComposeView tests

type ComposedView struct {
	Display         string
	CompStart, CompEnd int
}

func (e *Editor) View() ComposedView {
	if e == nil {
		return ComposedView{CompStart: -1, CompEnd: -1}
	}
	// For new model, display is full text; composing span is composingRange in byte offsets
	if !e.composing {
		return ComposedView{Display: e.text, CompStart: -1, CompEnd: -1}
	}
	s := byteOffsetForUtf16(e.text, e.composingRange.Start())
	en := byteOffsetForUtf16(e.text, e.composingRange.End())
	return ComposedView{Display: e.text, CompStart: s, CompEnd: en}
}

func (v ComposedView) MapBufToView(off int) int {
	if v.CompStart < 0 || off <= v.CompStart {
		return off
	}
	return off + (v.CompEnd - v.CompStart)
}
func (v ComposedView) MapBufToViewExact(bufOff, compOff int) int {
	if v.CompStart < 0 {
		return bufOff
	}
	if bufOff >= v.CompStart {
		return v.CompStart + compOff
	}
	return bufOff
}
func (v ComposedView) MapViewToBuf(off int) int {
	switch {
	case v.CompStart < 0:
		return max(0, min(off, len(v.Display)))
	case off <= v.CompStart:
		return max(0, off)
	case off >= v.CompEnd:
		return off - (v.CompEnd - v.CompStart)
	default:
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

// ByteOffsetAt for legacy
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

func (e *Editor) ResetCaretColumn() {
	if e != nil {
		e.caretColValid = false
	}
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// MoveCaretVertically retained for compatibility, now delegates to byte offsets
func (e *Editor) MoveCaretVertically(n int, lineCount func() int, penX func(displayOff int) float64) bool {
	if e == nil || n == 0 || lineCount == nil {
		return false
	}
	v := e.View()
	lines := lineCount()
	if lines <= 0 {
		return false
	}
	disp := byteOffsetForUtf16(e.text, e.selection.Extent)
	if e.composing {
		// composing cursor already in selection
	}
	lineIdx := 0
	for i := 0; i < disp; i++ {
		if v.Display[i] == '\n' {
			lineIdx++
		}
	}
	target := lineIdx + n
	if target < 0 {
		target = 0
	}
	if target > lines-1 {
		target = lines - 1
	}
	if target == lineIdx {
		return false
	}
	wantX := e.caretCol
	if !e.caretColValid {
		if penX != nil && disp <= len(v.Display) {
			wantX = penX(disp)
		}
		e.caretCol = wantX
		e.caretColValid = true
	}
	lineStart := 0
	for i := 0; i < target; i++ {
		nl := strings.IndexByte(v.Display[lineStart:], '\n')
		if nl < 0 {
			break
		}
		lineStart += nl + 1
	}
	lineEnd := len(v.Display)
	if nl := strings.IndexByte(v.Display[lineStart:], '\n'); nl >= 0 {
		lineEnd = lineStart + nl
	}
	best, bestX := lineStart, -1.0
	for idx := lineStart; idx <= lineEnd; {
		var x float64
		if idx < lineEnd && penX != nil {
			x = penX(idx)
		} else {
			x = wantX * 2
			if lineEnd >= 0 {
				x = penX(lineEnd)
			}
		}
		if bestX < 0 || absF(x-wantX) < absF(bestX-wantX) {
			best, bestX = idx, x
		}
		if idx >= lineEnd {
			break
		}
		_, sz := utf8.DecodeRuneInString(v.Display[idx:])
		idx += sz
	}
	cu := utf16OffsetForByte(e.text, v.MapViewToBuf(best))
	e.setSelectionNoReset(TextRange{Base: cu, Extent: cu})
	return true
}

func (e *Editor) setSelectionNoReset(r TextRange) {
	if e == nil {
		return
	}
	e.selection = r
	e.changed()
}

// SetSelection override for int pair (legacy calls e.SetSelection(0, len))
// We keep above SetSelection(TextRange) but Go will resolve; legacy callers use ints -> need wrapper
// To avoid conflict, we expose SetSelectionRange for TextRange and keep old name for ints via build tag? Instead rename new to SetSelectionRange and keep old.
// However new spec expects SetSelection(TextRange). Keep both via type switch hack: provide SetSelection2
func (e *Editor) SetSelectionOld(start, end int) { e.SetSelection(start, end) }
