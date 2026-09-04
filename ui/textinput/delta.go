package textinput

import "github.com/energye/gpui/ui/imeutil"

type TextEditingDelta struct {
	OldText     string
	DeltaText   string
	DeltaStart  int // -1 for NonTextUpdate
	DeltaEnd    int
	Selection   TextRange
	Composing   TextRange
	Affinity    int
}

func (d TextEditingDelta) IsNonTextUpdate() bool { return d.DeltaStart == -1 }

type TextInputConfiguration struct {
	InputType         string
	InputAction       string
	EnableDeltaModel  bool
	AutofillHints     []string
}

// ToDelta computes delta from old state to current Editor state.
func (e *Editor) ToDelta(oldText string, oldSel, oldComp TextRange) TextEditingDelta {
	if e == nil {
		return TextEditingDelta{DeltaStart: -1}
	}
	if e.text == oldText {
		if e.selection != oldSel || e.composingRange != oldComp {
			return TextEditingDelta{OldText: oldText, DeltaStart: -1, Selection: e.selection, Composing: e.composingRange}
		}
		return TextEditingDelta{OldText: oldText, DeltaStart: -1, Selection: e.selection, Composing: e.composingRange}
	}
	// Find first differing index
	oldRunes := []rune(oldText)
	newRunes := []rune(e.text)
	i := 0
	for i < len(oldRunes) && i < len(newRunes) && oldRunes[i] == newRunes[i] {
		i++
	}
	jOld := len(oldRunes)
	jNew := len(newRunes)
	for jOld > i && jNew > i && oldRunes[jOld-1] == newRunes[jNew-1] {
		jOld--
		jNew--
	}
	// Convert rune indices to utf16 offsets
	toU16 := func(s string, runeIdx int) int {
		n := 0
		for _, r := range []rune(s)[:runeIdx] {
			if r > 0xFFFF {
				n += 2
			} else {
				n++
			}
		}
		return n
	}
	deltaStart := toU16(oldText, i)
	deltaEnd := toU16(oldText, jOld)
	deltaText := string(newRunes[i:jNew])
	return TextEditingDelta{
		OldText:   oldText,
		DeltaText: deltaText,
		DeltaStart: deltaStart,
		DeltaEnd:   deltaEnd,
		Selection: e.selection,
		Composing: e.composingRange,
		Affinity:  e.selection.Affinity,
	}
}

func (e *Editor) ApplyDelta(d TextEditingDelta) bool {
	if e == nil {
		return false
	}
	if d.IsNonTextUpdate() {
		// §6.3 IsNonTextUpdate 要求 OldText==text 且 deltaText==""
		if d.OldText != e.text {
			return false
		}
		e.selection = d.Selection
		e.composingRange = d.Composing
		e.composing = d.Composing.Length() > 0
		e.changed()
		e.lastFrameworkText = e.text
		e.lastFrameworkSel = e.selection
		e.lastFrameworkComp = e.composingRange
		return true
	}
	// §6.3 last-write-wins：以 OldText 为基底计算，不发明 e.text 范围
	startByte := byteOffsetForUtf16(d.OldText, d.DeltaStart)
	endByte := byteOffsetForUtf16(d.OldText, d.DeltaEnd)
	if startByte < 0 {
		startByte = 0
	}
	if endByte < startByte {
		endByte = startByte
	}
	if startByte > len(d.OldText) {
		startByte = len(d.OldText)
	}
	if endByte > len(d.OldText) {
		endByte = len(d.OldText)
	}
	// Rune 边界吸附（对齐 F-D5 换算）
	for startByte > 0 && startByte < len(d.OldText) && (d.OldText[startByte]&0xC0) == 0x80 {
		startByte--
	}
	for endByte > 0 && endByte < len(d.OldText) && (d.OldText[endByte]&0xC0) == 0x80 {
		endByte--
	}
	newText := d.OldText[:startByte] + d.DeltaText + d.OldText[endByte:]
	e.text = newText
	e.mirrorRebuild()
	e.selection = clampRangeForText(newText, d.Selection)
	e.composingRange = clampRangeForText(newText, d.Composing)
	e.composing = e.composingRange.Length() > 0
	e.changed()
	e.lastFrameworkText = e.text
	e.lastFrameworkSel = e.selection
	e.lastFrameworkComp = e.composingRange
	return true
}
func clampRangeForText(text string, r TextRange) TextRange {
	n := utf16Len(text)
	if r.Base < 0 {
		r.Base = 0
	}
	if r.Extent < 0 {
		r.Extent = 0
	}
	if r.Base > n {
		r.Base = n
	}
	if r.Extent > n {
		r.Extent = n
	}
	return r
}

// TruncateSurrounding delegates to imeutil single source (B10 convergence).
func TruncateSurrounding(text string, cursorByte int) (string, int) {
	return imeutil.TruncateSurrounding(text, cursorByte)
}
