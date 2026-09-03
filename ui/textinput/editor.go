package textinput

import (
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/platform"
)

const (
	AffinityDownstream = 0
	AffinityUpstream   = 1
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

type editOp struct {
	pos      int
	deleted  string
	inserted string
}

type editGroup struct {
	ops             []editOp
	selBefore       TextRange
	selAfter        TextRange
	compBefore      TextRange
	compAfter       TextRange
	composingBefore bool
	composingAfter  bool
}

type Editor struct {
	text           string
	selection      TextRange
	composingRange TextRange
	composing      bool
	enableDeltaModel bool
	deltaModelLocked bool
	isPassword     bool
	obscuringChar  rune // 0 means default '•' (Flutter TextField.obscuringCharacter)
	readOnly       bool
	singleLine   bool
	contentType    platform.ContentType
	inputType      string
	inputAction    string
	autofillHints  []string
	batchDepth         int
	lastFrameworkText  string
	lastFrameworkSel   TextRange
	lastFrameworkComp  TextRange
	epoch              uint64
	pendingEpochBump   bool
	caretCol           float64
	caretColValid      bool
	OnChange           func()
	// lastEdit是最近一次文本变更区间(M1-d增量排版用):旧串[oldA,oldB)→
	// 新串[newA,newB).sync消费前又发生变更则失效(回退diff,保正确).
	// 由noteEdit维护,ConsumeEditSpan读取并清除.
	editOldA, editOldB, editNewA, editNewB int
	editSpanValid                            bool
	OnAnchor           func() // R4 F-D10: programmatic SetText → RefreshIMEAnchor
	history            []editGroup
	redoStack          []editGroup
	composingSnapshot  bool // true when a group is open for current composition
}

// noteEdit记录文本变更区间;已有未消费区间则失效(多变更回退diff).
func (e *Editor) noteEdit(oldA, oldB, newA, newB int) {
	if e == nil {
		return
	}
	if e.editSpanValid {
		e.editSpanValid = false
		return
	}
	e.editOldA, e.editOldB, e.editNewA, e.editNewB = oldA, oldB, newA, newB
	e.editSpanValid = true
}

// ConsumeEditSpan取走最近变更区间并清除,供InputBox.sync传RenderText.
func (e *Editor) ConsumeEditSpan() (oldA, oldB, newA, newB int, ok bool) {
	if e == nil || !e.editSpanValid {
		return 0, 0, 0, 0, false
	}
	e.editSpanValid = false
	return e.editOldA, e.editOldB, e.editNewA, e.editNewB, true
}

func New() *Editor { return &Editor{singleLine: true} }

func (e *Editor) SetSingleLine(v bool) {
	if e == nil {
		return
	}
	e.singleLine = v
}

func (e *Editor) IsSingleLine() bool {
	if e == nil {
		return true
	}
	return e.singleLine
}

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

// maxHistoryGroups caps undo depth (I4: history stays O(edit count), never O(document)).
const maxHistoryGroups = 100

// cloneDelta copies a stored delta so history never aliases the live document
// backing array (Go substrings would pin the whole pre-edit string, I4).
func cloneDelta(s string) string {
	if s == "" {
		return ""
	}
	return strings.Clone(s)
}

// appendHistory records one group, evicting past the cap. The evicted slot is
// zeroed: reslicing alone would keep its strings reachable (I4).
func (e *Editor) appendHistory(g editGroup) {
	e.history = append(e.history, g)
	if len(e.history) > maxHistoryGroups {
		e.history[0] = editGroup{}
		e.history = e.history[1:]
	}
}

// popHistory takes the newest group for Undo; popRedo mirrors it for Redo.
// The popped slot is zeroed so its strings drop off the backing array (I4).
func (e *Editor) popHistory() (editGroup, bool) {
	if e == nil || len(e.history) == 0 {
		return editGroup{}, false
	}
	g := e.history[len(e.history)-1]
	e.history[len(e.history)-1] = editGroup{}
	e.history = e.history[:len(e.history)-1]
	return g, true
}

func (e *Editor) popRedo() (editGroup, bool) {
	if e == nil || len(e.redoStack) == 0 {
		return editGroup{}, false
	}
	g := e.redoStack[len(e.redoStack)-1]
	e.redoStack[len(e.redoStack)-1] = editGroup{}
	e.redoStack = e.redoStack[:len(e.redoStack)-1]
	return g, true
}

func groupsBytes(gs []editGroup) int {
	n := 0
	for _, g := range gs {
		for _, op := range g.ops {
			n += len(op.deleted) + len(op.inserted)
		}
	}
	return n
}

// dropTrailingEmptyGroup removes a trailing zero-op group opened for a
// composition that produced no edits, so phantom steps never consume undo
// depth or evict real history through the cap (I4).
func (e *Editor) dropTrailingEmptyGroup() {
	if e == nil || len(e.history) == 0 {
		return
	}
	last := len(e.history) - 1
	if len(e.history[last].ops) == 0 {
		e.history[last] = editGroup{}
		e.history = e.history[:last]
	}
}

func (e *Editor) openGroup(selB, compB TextRange, cB bool) {
	if e == nil {
		return
	}
	g := editGroup{selBefore: selB, selAfter: selB, compBefore: compB, compAfter: compB, composingBefore: cB, composingAfter: cB}
	e.appendHistory(g)
	e.redoStack = nil
}

func (e *Editor) pushDelta(pos int, deleted, inserted string, selB, compB TextRange, cB bool, selA, compA TextRange, cA bool) {
	if e == nil {
		return
	}
	if e.composingSnapshot && len(e.history) > 0 {
		g := &e.history[len(e.history)-1]
		g.ops = append(g.ops, editOp{pos: pos, deleted: cloneDelta(deleted), inserted: cloneDelta(inserted)})
		g.selAfter = selA
		g.compAfter = compA
		g.composingAfter = cA
		return
	}
	g := editGroup{ops: []editOp{{pos: pos, deleted: cloneDelta(deleted), inserted: cloneDelta(inserted)}}, selBefore: selB, selAfter: selA, compBefore: compB, compAfter: compA, composingBefore: cB, composingAfter: cA}
	e.appendHistory(g)
	e.redoStack = nil
}

func diffStrings(old, new string) (pos int, deleted, inserted string) {
	maxPre := len(old)
	if len(new) < maxPre {
		maxPre = len(new)
	}
	pre := 0
	for pre < maxPre && old[pre] == new[pre] {
		pre++
	}
	for pre > 0 && pre < len(old) && pre < len(new) && (old[pre]&0xC0) == 0x80 {
		pre--
	}
	maxSuf := len(old) - pre
	if len(new)-pre < maxSuf {
		maxSuf = len(new) - pre
	}
	suf := 0
	for suf < maxSuf && old[len(old)-1-suf] == new[len(new)-1-suf] {
		suf++
	}
	for suf > 0 && (old[len(old)-suf]&0xC0) == 0x80 {
		suf--
	}
	return pre, old[pre : len(old)-suf], new[pre : len(new)-suf]
}

func (e *Editor) HistoryBytes() int {
	if e == nil {
		return 0
	}
	return groupsBytes(e.history) + groupsBytes(e.redoStack)
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
	if e.OnAnchor != nil {
		e.OnAnchor()
	}
}

// Undo reverts the last edit group. One IME composition (Begin→Update*→Commit) = one step (F-C2).
func (e *Editor) Undo() bool {
	if e == nil {
		return false
	}
	// Drop phantom trailing empty groups left by compositions that produced
	// no edits. The open composition group is never skipped: undoing it must
	// still cancel the composition and restore selBefore (I4).
	for len(e.history) > 0 && len(e.history[len(e.history)-1].ops) == 0 {
		if e.composingSnapshot {
			break
		}
		last := len(e.history) - 1
		e.history[last] = editGroup{}
		e.history = e.history[:last]
	}
	g, ok := e.popHistory()
	if !ok {
		return false
	}
	oldLen := len(e.text)
	for i := len(g.ops) - 1; i >= 0; i-- {
		op := g.ops[i]
		if op.pos < 0 {
			op.pos = 0
		}
		if op.pos > len(e.text) {
			op.pos = len(e.text)
		}
		end := op.pos + len(op.inserted)
		if end > len(e.text) {
			end = len(e.text)
		}
		e.text = e.text[:op.pos] + op.deleted + e.text[end:]
	}
	e.noteEdit(0, oldLen, 0, len(e.text))
	e.selection = g.selBefore
	e.composingRange = g.compBefore
	e.composing = g.composingBefore
	e.composingSnapshot = false
	if len(g.ops) > 0 {
		e.redoStack = append(e.redoStack, g)
	}
	e.caretColValid = false
	e.changed()
	return true
}

// Redo reapplies the last undone edit.
func (e *Editor) Redo() bool {
	g, ok := e.popRedo()
	if !ok {
		return false
	}
	oldLen := len(e.text)
	for _, op := range g.ops {
		if op.pos < 0 {
			op.pos = 0
		}
		if op.pos > len(e.text) {
			op.pos = len(e.text)
		}
		end := op.pos + len(op.deleted)
		if end > len(e.text) {
			end = len(e.text)
		}
		e.text = e.text[:op.pos] + op.inserted + e.text[end:]
	}
	e.noteEdit(0, oldLen, 0, len(e.text))
	e.selection = g.selAfter
	e.composingRange = g.compAfter
	e.composing = g.composingAfter
	e.composingSnapshot = false
	e.appendHistory(g)
	e.caretColValid = false
	e.changed()
	return true
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
func (e *Editor) SetClient(cfg TextInputConfiguration) {
	if e == nil {
		return
	}
	// §6.3 定值不可变：首次 SetClient 锁定 enableDeltaModel，后续翻转忽略（对齐 Flutter TextInputModel）
	if !e.deltaModelLocked {
		e.enableDeltaModel = cfg.EnableDeltaModel
		e.deltaModelLocked = true
	}
	e.inputType = cfg.InputType
	e.inputAction = cfg.InputAction
	if len(cfg.AutofillHints) > 0 {
		e.autofillHints = append([]string(nil), cfg.AutofillHints...)
	} else {
		e.autofillHints = nil
	}
	// §5 F-D6 透传：InputType/InputAction/autofillHints → ContentPurpose/Hints
	if purpose := purposeFromInputType(cfg.InputType); purpose != platform.PurposeNormal || cfg.InputType == "text" || cfg.InputType == "multiline" {
		e.contentType.Purpose = purpose
	}
	// password 类型联动 isPassword（对齐 Windows TYPE_TEXT_VARIATION_PASSWORD）
	if cfg.InputType == "visiblePassword" || cfg.InputType == "password" {
		e.isPassword = true
	}
}
func purposeFromInputType(t string) platform.ContentPurpose {
	switch t {
	case "emailAddress":
		return platform.PurposeEmail
	case "number":
		return platform.PurposeNumber
	case "phone":
		return platform.PurposePhone
	case "url":
		return platform.PurposeURL
	case "name":
		return platform.PurposeName
	case "datetime", "date", "time":
		return platform.PurposeDatetime
	case "visiblePassword", "password":
		return platform.PurposePassword
	case "multiline":
		return platform.PurposeNormal
	default:
		return platform.PurposeNormal
	}
}
func (e *Editor) SetConfiguration(cfg TextInputConfiguration) {
	e.SetClient(cfg)
}
func (e *Editor) ContentType() platform.ContentType {
	if e == nil {
		return platform.ContentType{}
	}
	return e.contentType
}
func (e *Editor) SetContentType(ct platform.ContentType) {
	if e == nil {
		return
	}
	e.contentType = ct
}
func (e *Editor) EnableDeltaModel() bool {
	if e == nil {
		return false
	}
	return e.enableDeltaModel
}
func (e *Editor) SetPassword(v bool) {
	if e == nil {
		return
	}
	e.isPassword = v
	if v && e.composing {
		e.EndComposing()
	}
}
func (e *Editor) IsPassword() bool {
	if e == nil {
		return false
	}
	return e.isPassword
}

// ObscuringCharacter returns the password masking rune, default '•' (U+2022) per Flutter TextField.obscuringCharacter.
func (e *Editor) ObscuringCharacter() rune {
	if e == nil {
		return '•'
	}
	if e.obscuringChar == 0 {
		return '•'
	}
	return e.obscuringChar
}

// SetObscuringCharacter customizes the password mask; pass 0 to reset to default '•'.
// Aligns Flutter TextField.obscuringCharacter (customizable, default '•').
func (e *Editor) SetObscuringCharacter(r rune) {
	if e == nil {
		return
	}
	if r == 0 {
		e.obscuringChar = 0
	} else {
		e.obscuringChar = r
	}
	if e.isPassword && e.OnChange != nil {
		e.OnChange()
	}
}
func (e *Editor) SetReadOnly(v bool) {
	if e == nil {
		return
	}
	e.readOnly = v
}
func (e *Editor) IsReadOnly() bool {
	if e == nil {
		return false
	}
	return e.readOnly
}
func (e *Editor) IsInBatch() bool {
	if e == nil {
		return false
	}
	return e.batchDepth > 0
}
func (e *Editor) IsNone() bool {
	if e == nil {
		return false
	}
	return e.inputType == "none"
}

func (e *Editor) ApplyFrameworkState(text string, sel, comp TextRange, affinity int) bool {
	// F-D8 二次覆盖：框架侧 set_editing_state 按 -1 哨兵→显式→Sel→Composing 四步原子
	// 这里合并为一次 SetText，但保留哨兵语义与 lastFramework 去重
	if e == nil {
		return false
	}
	return e.SetText(text, sel, comp, affinity)
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
	selB, compB, cB := e.selection, e.composingRange, e.composing
	var dPos int
	var dDel, dIns string
	if changed {
		dPos, dDel, dIns = diffStrings(e.text, text)
	}
	if e.text != text {
		e.noteEdit(0, len(e.text), 0, len(text))
	}
	e.text = text
	e.selection = sel
	e.composingRange = comp
	e.composing = hasComp
	if changed {
		e.pushDelta(dPos, dDel, dIns, selB, compB, cB, e.selection, e.composingRange, e.composing)
	}
	if changed {
		e.changed()
	}
	// §8 C4 自动维护：框架侧 SetText 即视为 lastFramework 已同步，便于 ShouldSkip 去重
	e.lastFrameworkText = text
	e.lastFrameworkSel = sel
	e.lastFrameworkComp = comp
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
	if e == nil || e.composing || e.isPassword || e.readOnly {
		return
	}
	if !e.composingSnapshot {
		e.openGroup(e.selection, e.composingRange, e.composing)
		e.composingSnapshot = true
	}
	// F-S1 有选区时先删选中段再起 preedit，批内合并为一次 epoch/OnChange
	wasBatch := e.batchDepth > 0
	if !wasBatch {
		e.BeginBatchEdit()
	}
	if !e.selection.Collapsed() {
		e.DeleteSelected()
	}
	e.composing = true
	e.composingRange = TextRange{Base: e.selection.Extent, Extent: e.selection.Extent}
	if !wasBatch {
		e.EndBatchEdit()
	} else {
		e.changed()
	}
}

func (e *Editor) UpdateComposingText(text string, sel TextRange) bool {
	if e == nil || e.isPassword || e.readOnly {
		return false
	}
	if e.singleLine && strings.Contains(text, "\n") {
		text = strings.ReplaceAll(text, "\n", "")
	}
	if text == "" && sel.Collapsed() && e.composingRange.Collapsed() && e.selection.Collapsed() {
		return false
	}
	if text == "" && e.composingRange.Collapsed() {
		return false
	}
	if !e.composingSnapshot {
		e.openGroup(e.selection, e.composingRange, e.composing)
		e.composingSnapshot = true
	}
	var replaceRange TextRange
	if e.composingRange.Collapsed() {
		replaceRange = e.selection
	} else {
		replaceRange = e.composingRange
	}
	startByte := byteOffsetForUtf16(e.text, replaceRange.Start())
	endByte := byteOffsetForUtf16(e.text, replaceRange.End())
	deleted := e.text[startByte:endByte]
	selB, compB, cB := e.selection, e.composingRange, e.composing
	newText := e.text[:startByte] + text + e.text[endByte:]
	newStart := replaceRange.Start()
	newEnd := newStart + utf16Len(text)
	sel.Base = clampUtf16(newText, sel.Base)
	sel.Extent = clampUtf16(newText, sel.Extent)
	e.noteEdit(startByte, endByte, startByte, startByte+len(text))
	e.text = newText
	e.composing = true
	e.composingRange = TextRange{Base: newStart, Extent: newEnd}
	e.selection = sel
	e.pushDelta(startByte, deleted, text, selB, compB, cB, e.selection, e.composingRange, e.composing)
	e.changed()
	e.caretColValid = false
	return true
}

func (e *Editor) CommitComposing() {
	if e == nil || !e.composing {
		return
	}
	wasOpen := e.composingSnapshot
	e.composing = false
	e.composingRange = TextRange{}
	e.composingSnapshot = false
	if wasOpen && len(e.history) > 0 {
		g := &e.history[len(e.history)-1]
		g.selAfter = e.selection
		g.compAfter = e.composingRange
		g.composingAfter = false
	}
	e.dropTrailingEmptyGroup()
	e.changed()
}

func (e *Editor) EndComposing() {
	if e == nil || !e.composing {
		return
	}
	s := byteOffsetForUtf16(e.text, e.composingRange.Start())
	en := byteOffsetForUtf16(e.text, e.composingRange.End())
	selB, compB := e.selection, e.composingRange
	wasOpen := e.composingSnapshot
	if s != en {
		deleted := e.text[s:en]
		e.noteEdit(s, en, s, s)
		e.text = e.text[:s] + e.text[en:]
		e.selection = TextRange{Base: e.composingRange.Start(), Extent: e.composingRange.Start()}
		e.pushDelta(s, deleted, "", selB, compB, true, e.selection, e.composingRange, true)
	}
	e.composing = false
	e.composingRange = TextRange{}
	e.composingSnapshot = false
	if (wasOpen || s != en) && len(e.history) > 0 {
		g := &e.history[len(e.history)-1]
		g.selAfter = e.selection
		g.compAfter = e.composingRange
		g.composingAfter = false
	}
	if wasOpen {
		e.dropTrailingEmptyGroup()
	}
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
		if e.OnAnchor != nil {
			e.OnAnchor()
		}
	}
}

func (e *Editor) DeleteSelected() bool {
	if e == nil || e.selection.Collapsed() || e.readOnly {
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
	deleted := e.text[start:end]
	selB, compB, cB := e.selection, e.composingRange, e.composing
	e.noteEdit(start, end, start, start)
	e.text = e.text[:start] + e.text[end:]
	off := e.selection.Start()
	e.selection = TextRange{Base: off, Extent: off}
	if e.composing {
		e.composingRange = TextRange{Base: off, Extent: off}
	}
	e.pushDelta(start, deleted, "", selB, compB, cB, e.selection, e.composingRange, e.composing)
	e.changed()
	return true
}

func (e *Editor) AddText(text string) bool {
	if e == nil || text == "" || e.readOnly {
		return false
	}
	// F-C5/F-F3: single-line must reject '\n' (Flutter single-line behavior).
	if e.singleLine && strings.Contains(text, "\n") {
		text = strings.ReplaceAll(text, "\n", "")
		if text == "" {
			return false
		}
	}
	er := e.EditableRange()
	if e.selection.Start() < er.Start() || e.selection.End() > er.End() {
		return false
	}
	var replaceRange TextRange
	if e.composing {
		replaceRange = e.composingRange
	} else {
		replaceRange = e.selection
	}
	startByte := byteOffsetForUtf16(e.text, replaceRange.Start())
	endByte := byteOffsetForUtf16(e.text, replaceRange.End())
	deleted := e.text[startByte:endByte]
	selB, compB, cB := e.selection, e.composingRange, e.composing
	e.noteEdit(startByte, endByte, startByte, startByte+len(text))
	e.text = e.text[:startByte] + text + e.text[endByte:]
	newOff := replaceRange.Start() + utf16Len(text)
	e.selection = TextRange{Base: newOff, Extent: newOff}
	e.pushDelta(startByte, deleted, text, selB, compB, cB, e.selection, TextRange{}, false)
	if e.composing {
		e.composing = false
		e.composingRange = TextRange{}
		e.composingSnapshot = false
	}
	e.changed()
	e.caretColValid = false
	return true
}

func (e *Editor) AddCodePoint(r rune) bool {
	if e == nil || e.readOnly {
		return false
	}
	return e.AddText(string(r))
}

func (e *Editor) DeleteSurrounding(offset, count int) bool {
	if e == nil || e.readOnly || count <= 0 {
		return false
	}
	// F-D5/Spec: offset/count 按 code point（rune）计，surrogate 计 1。
	// caret 为 UTF16，需经 rune 索引换算，避免劈半 surrogate（Flutter RuneCount 校准）。
	caret := e.selection.Extent
	runes := []rune(e.text)
	runeCaret := utf16ToRuneIndex(e.text, caret)
	startRune := runeCaret + offset
	endRune := startRune + count
	if startRune < 0 {
		startRune = 0
	}
	if endRune > len(runes) {
		endRune = len(runes)
	}
	start := runeIndexToUtf16(runes, startRune)
	end := runeIndexToUtf16(runes, endRune)
	er := e.EditableRange()
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
	deleted := e.text[startByte:endByte]
	selB, compB, cB := e.selection, e.composingRange, e.composing
	e.noteEdit(startByte, endByte, startByte, startByte)
	e.text = e.text[:startByte] + e.text[endByte:]
	delta := end - start
	if offset < 0 {
		e.selection = TextRange{Base: start, Extent: start}
	}
	if e.composing && e.composingRange.End() > start {
		newEnd := e.composingRange.End() - delta
		if newEnd < e.composingRange.Start() {
			newEnd = e.composingRange.Start()
		}
		e.composingRange = TextRange{Base: e.composingRange.Start(), Extent: newEnd}
	}
	e.pushDelta(startByte, deleted, "", selB, compB, cB, e.selection, e.composingRange, e.composing)
	e.changed()
	e.caretColValid = false
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
		// 按 code point 删 1 个，DeleteSurrounding 内部会按 rune→utf16 换算避免劈 surrogate
		return e.DeleteSurrounding(-1, 1)
	}
	if !e.selection.Collapsed() {
		return e.DeleteSelected()
	}
	caret := e.selection.Extent
	if caret == 0 {
		return false
	}
	return e.DeleteSurrounding(-1, 1)
}

func (e *Editor) Delete() bool {
	if e == nil {
		return false
	}
	if e.composing {
		if e.selection.Extent >= e.composingRange.End() {
			return false
		}
		return e.DeleteSurrounding(0, 1)
	}
	if !e.selection.Collapsed() {
		return e.DeleteSelected()
	}
	caret := e.selection.Extent
	if caret >= utf16Len(e.text) {
		return false
	}
	return e.DeleteSurrounding(0, 1)
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

func (e *Editor) MoveCursorUp() bool {
	if e == nil {
		return false
	}
	lines := splitLines(e.text)
	caret := e.selection.Extent
	idx, col := lineColForOffset(lines, e.text, caret)
	if idx <= 0 {
		return false
	}
	target := idx - 1
	off := offsetForLineCol(lines, e.text, target, col)
	return e.SetSelection(TextRange{Base: off, Extent: off})
}

func (e *Editor) MoveCursorDown() bool {
	if e == nil {
		return false
	}
	lines := splitLines(e.text)
	caret := e.selection.Extent
	idx, col := lineColForOffset(lines, e.text, caret)
	if idx >= len(lines)-1 {
		return false
	}
	target := idx + 1
	off := offsetForLineCol(lines, e.text, target, col)
	return e.SetSelection(TextRange{Base: off, Extent: off})
}

func (e *Editor) MoveCursorByWord(forward bool) bool {
	if e == nil {
		return false
	}
	caret := e.selection.Extent
	runes := []rune(e.text)
	// map utf16 caret to rune index
	runeIdx := utf16ToRuneIndex(e.text, caret)
	if forward {
		if runeIdx >= len(runes) {
			return false
		}
		// skip current word if inside
		if isWordChar(runes[runeIdx]) {
			for runeIdx < len(runes) && isWordChar(runes[runeIdx]) {
				runeIdx++
			}
		}
		for runeIdx < len(runes) && !isWordChar(runes[runeIdx]) {
			runeIdx++
		}
		for runeIdx < len(runes) && isWordChar(runes[runeIdx]) {
			runeIdx++
		}
		// stop at word end; if we skipped spaces, we are at start of next word, back to start
		// Actually for Ctrl+Right, move to end of next word; our loop does that.
	} else {
		if runeIdx <= 0 {
			return false
		}
		runeIdx--
		for runeIdx > 0 && !isWordChar(runes[runeIdx]) {
			runeIdx--
		}
		for runeIdx > 0 && isWordChar(runes[runeIdx-1]) {
			runeIdx--
		}
	}
	off := runeIndexToUtf16(runes, runeIdx)
	return e.SetSelection(TextRange{Base: off, Extent: off})
}

func (e *Editor) IsComposing() bool { return e != nil && e.composing }
func (e *Editor) ComposeActive() bool { return e.IsComposing() }
func (e *Editor) ComposingRange() TextRange {
	if e == nil {
		return TextRange{}
	}
	return e.composingRange
}
func (e *Editor) ComposingStartByte() int {
	if e == nil || !e.composing {
		return -1
	}
	return byteOffsetForUtf16(e.text, e.composingRange.Start())
}
func (e *Editor) ComposingEndByte() int {
	if e == nil || !e.composing {
		return -1
	}
	return byteOffsetForUtf16(e.text, e.composingRange.End())
}
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
	e.SetCaretWithAffinity(off, AffinityDownstream)
}
func (e *Editor) SetCaretWithAffinity(off int, affinity int) {
	if e == nil {
		return
	}
	if off < 0 {
		off = 0
	}
	if off > len(e.text) {
		off = len(e.text)
	}
	// Snap to UAX#29 cluster boundary per affinity (M1-c), scoped to the
	// current line so the cost is O(L), not O(n). Clusters never span \n.
	lineStart := off
	for lineStart > 0 && e.text[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := off
	for lineEnd < len(e.text) && e.text[lineEnd] != '\n' {
		lineEnd++
	}
	rel := text.SnapCluster(e.text[lineStart:lineEnd], off-lineStart, affinity == AffinityDownstream)
	off = lineStart + rel
	cu := 0
	for _, r := range e.text[:off] {
		if r > 0xFFFF {
			cu += 2
		} else {
			cu++
		}
	}
	e.SetSelection(TextRange{Base: cu, Extent: cu, Affinity: affinity})
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
	// F-B5：PurposePassword 时只给 obscuringCharacter，且限 editable_range（Flutter obscureText 语义）
	if e.isPassword {
		ch := e.ObscuringCharacter()
		selLen := e.selection.Length()
		raw := e.text[byteOffsetForUtf16(e.text, e.selection.Start()):byteOffsetForUtf16(e.text, e.selection.End())]
		n := len([]rune(raw))
		if n == 0 {
			n = selLen
			if n == 0 {
				n = 1
			}
		}
		return strings.Repeat(string(ch), n)
	}
	// 限 editable_range：选区若完全在可编辑区外则不给
	er := e.EditableRange()
	if e.selection.End() <= er.Start() || e.selection.Start() >= er.End() {
		return ""
	}
	// 夹到可编辑区内再取
	s := e.selection.Start()
	en := e.selection.End()
	if s < er.Start() {
		s = er.Start()
	}
	if en > er.End() {
		en = er.End()
	}
	return e.text[byteOffsetForUtf16(e.text, s):byteOffsetForUtf16(e.text, en)]
}
func (e *Editor) Cut() string {
	if e == nil || e.readOnly {
		return ""
	}
	s := e.Copy()
	// Cut 受同一钳制：DeleteSelected 已限 editable_range / composing
	e.DeleteSelected()
	return s
}
func (e *Editor) Paste(s string) bool {
	if e == nil || e.readOnly || s == "" {
		return false
	}
	before := e.text
	e.AddText(s)
	return e.text != before
}
func (e *Editor) SelectAll() { e.SetSelection(e.TextRange()) }
func (e *Editor) SelectionRange() TextRange {
	if e == nil {
		return TextRange{}
	}
	return e.selection
}

// SelectLineAt selects the line containing byteOff (for triple-click).
func (e *Editor) SelectLineAt(byteOff int) bool {
	if e == nil || e.text == "" {
		return false
	}
	if byteOff < 0 {
		byteOff = 0
	}
	if byteOff > len(e.text) {
		byteOff = len(e.text)
	}
	for byteOff > 0 && byteOff < len(e.text) && (e.text[byteOff]&0xC0) == 0x80 {
		byteOff--
	}
	// find line start/end in utf16
	accBytes := 0
	accUnits := 0
	lines := splitLines(e.text)
	for _, ln := range lines {
		lnBytes := len(ln)
		lineStartByte := accBytes
		lineEndByte := accBytes + lnBytes
		// include '\n' break after line except last
		if byteOff >= lineStartByte && byteOff <= lineEndByte {
			cs := accUnits
			ce := accUnits + utf16Len(ln)
			er := e.EditableRange()
			if cs < er.Start() {
				cs = er.Start()
			}
			if ce > er.End() {
				ce = er.End()
			}
			return e.SetSelection(TextRange{Base: cs, Extent: ce})
		}
		accBytes += lnBytes + 1 // '\n'
		accUnits += utf16Len(ln) + 1
	}
	e.SelectAll()
	return true
}

// utf16ForByte returns utf16 offset for a byte offset (clamped to rune boundary).
func (e *Editor) utf16ForByte(byteOff int) int {
	if e == nil {
		return 0
	}
	if byteOff <= 0 {
		return 0
	}
	if byteOff >= len(e.text) {
		return utf16Len(e.text)
	}
	for byteOff > 0 && byteOff < len(e.text) && (e.text[byteOff]&0xC0) == 0x80 {
		byteOff--
	}
	n := 0
	for _, r := range e.text[:byteOff] {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func lineColForOffset(lines []string, text string, off int) (int, int) {
	if len(lines) == 0 {
		return 0, 0
	}
	acc := 0
	for i, ln := range lines {
		lnUnits := utf16Len(ln)
		if off <= acc+lnUnits {
			return i, off - acc
		}
		acc += lnUnits + 1 // '\n' is 1 unit
		if i == len(lines)-1 {
			return i, lnUnits
		}
	}
	return len(lines) - 1, utf16Len(lines[len(lines)-1])
}

func offsetForLineCol(lines []string, text string, line, col int) int {
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}
	acc := 0
	for i := 0; i < line; i++ {
		acc += utf16Len(lines[i]) + 1
	}
	lnUnits := utf16Len(lines[line])
	if col < 0 {
		col = 0
	}
	if col > lnUnits {
		col = lnUnits
	}
	return acc + col
}

func utf16ToRuneIndex(s string, utf16Off int) int {
	off := 0
	idx := 0
	for _, r := range s {
		if off >= utf16Off {
			break
		}
		sz := 1
		if r > 0xFFFF {
			sz = 2
		}
		off += sz
		idx++
	}
	return idx
}

func runeIndexToUtf16(runes []rune, idx int) int {
	if idx < 0 {
		idx = 0
	}
	if idx > len(runes) {
		idx = len(runes)
	}
	n := 0
	for _, r := range runes[:idx] {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func isWordChar(r rune) bool {
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
}
func (e *Editor) SelectWordAt(byteOff int) bool {
	if e == nil || e.text == "" {
		return false
	}
	if byteOff < 0 {
		byteOff = 0
	}
	if byteOff > len(e.text) {
		byteOff = len(e.text)
	}
	// Snap to rune start
	for byteOff > 0 && byteOff < len(e.text) && (e.text[byteOff]&0xC0) == 0x80 {
		byteOff--
	}
	// Find rune index
	runeIdx := 0
	for i := range e.text[:byteOff] {
		if (e.text[i]&0xC0) != 0x80 {
			runeIdx++
		}
	}
	runes := []rune(e.text)
	if runeIdx >= len(runes) {
		runeIdx = len(runes) - 1
	}
	if !isWordChar(runes[runeIdx]) {
		return false
	}
	start, end := runeIdx, runeIdx
	for start > 0 && isWordChar(runes[start-1]) {
		start--
	}
	for end < len(runes) && isWordChar(runes[end]) {
		end++
	}
	// Convert to utf16
	cs, ce := 0, 0
	for _, r := range runes[:start] {
		if r > 0xFFFF {
			cs += 2
		} else {
			cs++
		}
	}
	for _, r := range runes[:end] {
		if r > 0xFFFF {
			ce += 2
		} else {
			ce++
		}
	}
	er := e.EditableRange()
	if cs < er.Start() {
		cs = er.Start()
	}
	if ce > er.End() {
		ce = er.End()
	}
	return e.SetSelection(TextRange{Base: cs, Extent: ce})
}
