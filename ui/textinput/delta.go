package textinput

import "unicode/utf8"

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
		e.selection = d.Selection
		e.composingRange = d.Composing
		e.composing = d.Composing.Length() > 0
		e.changed()
		return true
	}
	// last-write-wins with OldText base
	if d.OldText != e.text && d.OldText != "" {
		// For simplicity, if OldText mismatches, treat as SetText
	}
	startByte := byteOffsetForUtf16(d.OldText, d.DeltaStart)
	endByte := byteOffsetForUtf16(d.OldText, d.DeltaEnd)
	// If OldText is current text, use it, else use e.text as base for mismatch
	base := e.text
	if d.OldText == e.text {
		base = d.OldText
		startByte = byteOffsetForUtf16(base, d.DeltaStart)
		endByte = byteOffsetForUtf16(base, d.DeltaEnd)
	} else {
		// fallback: replace based on e.text range
		startByte = byteOffsetForUtf16(e.text, d.DeltaStart)
		endByte = byteOffsetForUtf16(e.text, d.DeltaEnd)
		if startByte > len(e.text) {
			startByte = len(e.text)
		}
		if endByte > len(e.text) {
			endByte = len(e.text)
		}
	}
	newText := base[:startByte] + d.DeltaText + base[endByte:]
	e.text = newText
	e.selection = d.Selection
	e.composingRange = d.Composing
	e.composing = d.Composing.Length() > 0
	e.changed()
	return true
}

// TruncateSurrounding implements 4000 bytes including NUL, centered at cursor.
func TruncateSurrounding(text string, cursorByte int) (string, int) {
	const max = 4000
	if len(text)+1 <= max {
		return text, cursorByte
	}
	// Need to keep window of max-1 bytes (reserve 1 for NUL) centered at cursor
	budget := max - 1
	half := budget / 2
	start := cursorByte - half
	if start < 0 {
		start = 0
	}
	end := start + budget
	if end > len(text) {
		end = len(text)
		start = end - budget
		if start < 0 {
			start = 0
		}
	}
	// Snap to rune boundaries
	for start > 0 && start < len(text) && !utf8.RuneStart(text[start]) {
		start--
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
		if end-start > budget {
			break
		}
	}
	if end-start > budget {
		end = start + budget
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end++
		}
	}
	newCursor := cursorByte - start
	for newCursor > 0 && newCursor < len(text[start:end]) && !utf8.RuneStart(text[start+newCursor]) {
		newCursor--
	}
	return text[start:end], newCursor
}
