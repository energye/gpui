package textinput

import (
	"unicode/utf16"
	"unicode/utf8"
)

type TextRange struct {
	Base     int
	Extent   int
	Affinity int
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

func (e *Editor) Epoch() uint64 {
	if e == nil {
		return 0
	}
	return e.epoch
}

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

func (e *Editor) SetText(text string, sel, comp TextRange, affinity int) bool {
	if e == nil {
		return false
	}
	if sel.Base == -1 && sel.Extent == -1 {
		sel = TextRange{Base: 0, Extent: 0, Affinity: affinity}
	}
	if comp.Base == -1 && comp.Extent == -1 {
		comp = TextRange{}
		e.composing = false
	}
	sel.Affinity = affinity
	sel.Base = clampUtf16(text, sel.Base)
	sel.Extent = clampUtf16(text, sel.Extent)
	if comp.Base < 0 || comp.Extent < 0 {
	} else {
		comp.Base = clampUtf16(text, comp.Base)
		comp.Extent = clampUtf16(text, comp.Extent)
	}
	hasComp := comp.Length() > 0
	changed := e.text != text || e.selection != sel || e.composingRange != comp || e.composing != hasComp
	e.text = text
	e.selection = sel
	e.composingRange = comp
	e.composing = hasComp
	if changed {
		e.changed()
	}
	e.caretColValid = false
	return changed
}

func (e *Editor) SetSelection(r TextRange) bool {
	if e == nil {
		return false
	}
	if e.composing && !r.Collapsed() {
		return false
	}
	er := e.EditableRange()
	r.Base = clampUtf16(e.text, r.Base)
	r.Extent = clampUtf16(e.text, r.Extent)
	if r.Start() < er.Start() || r.End() > er.End() {
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
	e.caretColValid = false
	return true
}

func (e *Editor) SetComposingRange(r TextRange, cursorOffset int) bool {
	if e == nil || !e.composing {
		return false
	}
	n := utf16Len(e.text)
	if r.Start() < 0 || r.End() > n {
		return false
	}
	e.composingRange = r
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
		return
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
	newStart := replaceRange.Start()
	newEnd := newStart + utf16Len(text)
	sel.Base = clampUtf16(newText, sel.Base)
	sel.Extent = clampUtf16(newText, sel.Extent)
	e.text = newText
	e.composing = true
	e.composingRange = TextRange{Base: newStart, Extent: newEnd}
	e.selection = sel
	e.changed()
	e.caretColValid = false
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
	e.text = e.text[:start] + e.text[end:]
	off := e.selection.Start()
	e.selection = TextRange{Base: off, Extent: off}
	if e.composing {
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
		e.composing = false
		e.composingRange = TextRange{}
	}
	e.changed()
	e.caretColValid = false
}

func (e *Editor) DeleteSurrounding(before, after int) bool {
	if e == nil {
		return false
	}
	er := e.EditableRange()
	caret := e.selection.Extent
	var start, end int
	if before >= 0 && after >= 0 && (before > 0 || after > 0) && before <= 10000 {
		caretByte := byteOffsetForUtf16(e.text, caret)
		sByte := caretByte - before
		if sByte < 0 {
			sByte = 0
		}
		eByte := caretByte + after
		if eByte > len(e.text) {
			eByte = len(e.text)
		}
		for sByte > 0 && !utf8.RuneStart(e.text[sByte]) {
			sByte--
		}
		for eByte < len(e.text) && !utf8.RuneStart(e.text[eByte]) {
			eByte++
		}
		if sByte >= eByte {
			return false
		}
		start = 0
		for _, r := range e.text[:sByte] {
			if r > 0xFFFF {
				start += 2
			} else {
				start++
			}
		}
		end = start
		for _, r := range e.text[sByte:eByte] {
			if r > 0xFFFF {
				end += 2
			} else {
				end++
			}
		}
		_ = utf16.Encode
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
	e.text = e.text[:startByte] + e.text[endByte:]
	delta := end - start
	if before > 0 {
		e.selection = TextRange{Base: start, Extent: start}
	} else if before <= 0 && after > 0 {
	} else {
		if before <= 0 {
			e.selection = TextRange{Base: start, Extent: start}
		}
	}
	if e.composing && e.composingRange.End() > start {
		newEnd := e.composingRange.End() - delta
		if newEnd < e.composingRange.Start() {
			newEnd = e.composingRange.Start()
		}
		e.composingRange = TextRange{Base: e.composingRange.Start(), Extent: newEnd}
	}
	e.changed()
	return true
}

func (e *Editor) Backspace() bool {
	if e == nil {
		return false
	}
	if e.composing {
		if e.selection.Extent <= e.composingRange.Start() {
			return false
		}
		caret := e.selection.Extent
		prev := caret - 1
		b := byteOffsetForUtf16(e.text, prev)
		r, _ := utf8.DecodeRuneInString(e.text[b:])
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
	return e.SetSelection(TextRange{Base: er.Start(), Extent: er.Start()})
}
func (e *Editor) MoveCursorToEnd() bool {
	if e == nil {
		return false
	}
	er := e.EditableRange()
	return e.SetSelection(TextRange{Base: er.End(), Extent: er.End()})
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
	return e.SetSelection(TextRange{Base: caret - cu, Extent: caret - cu})
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
	return e.SetSelection(TextRange{Base: caret + cu, Extent: caret + cu})
}

func (e *Editor) ComposeActive() bool { return e != nil && e.composing }
func (e *Editor) CompositionText() string {
	if e == nil || !e.composing {
		return ""
	}
	return e.text[byteOffsetForUtf16(e.text, e.composingRange.Start()):byteOffsetForUtf16(e.text, e.composingRange.End())]
}

// --- convenience aliases for example/tests (pure, not legacy) ---
func (e *Editor) Text() string { return e.GetText() }
func (e *Editor) Cursor() int  { return e.GetCursorOffset() }
func (e *Editor) SetTextSimple(s string) {
	if e == nil {
		return
	}
	n := utf16Len(s)
	e.SetText(s, TextRange{Base: n, Extent: n}, TextRange{}, 0)
}
func (e *Editor) Insert(s string)       { e.AddText(s) }
func (e *Editor) DeleteBackward()       { e.Backspace() }
func (e *Editor) DeleteForward()        { e.Delete() }
func (e *Editor) MoveCaretRunes(n int) {
	for i := 0; i < n; i++ {
		e.MoveCursorForward()
	}
	for i := 0; i > n; i-- {
		e.MoveCursorBack()
	}
}
func (e *Editor) SetCaret(off int) {
	if e == nil {
		return
	}
	if off < 0 {
		off = 0
	}
	if off > len(e.text) {
		off = len(e.text)
	}
	cu := 0
	for _, r := range e.text[:off] {
		if r > 0xFFFF {
			cu += 2
		} else {
			cu++
		}
	}
	e.SetSelection(TextRange{Base: cu, Extent: cu})
}
func (e *Editor) SetSelectionBytes(s, en int) {
	if e == nil {
		return
	}
	if s < 0 {
		s = 0
	}
	if en < 0 {
		en = 0
	}
	if s > len(e.text) {
		s = len(e.text)
	}
	if en > len(e.text) {
		en = len(e.text)
	}
	cs, ce := 0, 0
	for _, r := range e.text[:s] {
		if r > 0xFFFF {
			cs += 2
		} else {
			cs++
		}
	}
	for _, r := range e.text[:en] {
		if r > 0xFFFF {
			ce += 2
		} else {
			ce++
		}
	}
	e.SetSelection(TextRange{Base: cs, Extent: ce})
}
type ComposedView struct {
	Display         string
	CompStart, CompEnd int
}
func (e *Editor) View() ComposedView {
	if e == nil {
		return ComposedView{CompStart: -1, CompEnd: -1}
	}
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
func (v ComposedView) MapViewToBuf(off int) int {
	if v.CompStart < 0 {
		return off
	}
	if off <= v.CompStart {
		return off
	}
	if off >= v.CompEnd {
		return off - (v.CompEnd - v.CompStart)
	}
	return v.CompStart
}
func (v ComposedView) MapBufToViewExact(a, b int) int {
	if v.CompStart < 0 {
		return a
	}
	return v.CompStart + b
}
func (e *Editor) CompositionCursor() int {
	if e == nil || !e.composing {
		return -1
	}
	v := e.View()
	off := e.selection.Extent - e.composingRange.Start()
	return v.CompStart + off
}
func (e *Editor) ByteOffsetAt(x float64, w func(string) float64) int { return e.View().MapViewToBuf(int(x)) }
func (e *Editor) MoveCaretVertically(n int, lineCount func() int, penX func(int) float64) bool { return false }
func (e *Editor) Snapshot() (string, int) { return e.GetText(), e.GetCursorOffset() }
func (e *Editor) LenRunes() int { return len([]rune(e.text)) }
func (e *Editor) Copy() string {
	if e == nil || e.selection.Collapsed() {
		return ""
	}
	return e.text[byteOffsetForUtf16(e.text, e.selection.Start()):byteOffsetForUtf16(e.text, e.selection.End())]
}
func (e *Editor) Cut() string {
	s := e.Copy()
	e.DeleteSelected()
	return s
}
func (e *Editor) Paste(s string) bool {
	before := e.text
	e.AddText(s)
	return e.text != before
}
func (e *Editor) SelectAll() { e.SetSelection(e.TextRange()) }
