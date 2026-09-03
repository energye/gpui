package textinput

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// MoveVisual moves the cursor by one visual seam (gap) using the single-source
// TextLayout. It is the shipped path used by the true window's manualInputBox
// and is the only honest way to test visual movement: callers must not
// re-implement caret stepping via SetCaret.
//
// delta is +1 (right) or -1 (left). Returns true if the cursor moved.
// 多行时按可视行盒走缝，跨行时自动跳到相邻行的行首/行尾（Flutter TextPainter 可视缝语义）。
func (e *Editor) MoveVisual(delta int, lay *rendering.TextLayout) bool {
	if e == nil || lay == nil || lay.LineCount() == 0 {
		if delta < 0 {
			return e.MoveCursorBack()
		}
		return e.MoveCursorForward()
	}
	curByte := e.GetCursorOffset()
	lineIdx, _, ok := lay.CaretForOffset(curByte)
	if !ok {
		if delta < 0 {
			return e.MoveCursorBack()
		}
		return e.MoveCursorForward()
	}
	if lineIdx < 0 {
		lineIdx = 0
	}
	if lineIdx >= lay.LineCount() {
		lineIdx = lay.LineCount() - 1
	}
	lineStart, lineEnd, _, _, ok := lay.Line(lineIdx)
	if !ok {
		if delta < 0 {
			return e.MoveCursorBack()
		}
		return e.MoveCursorForward()
	}
	// M1-c:按字素簇 stepping,不再逐rune走缝.簇起点=行内相对簇表+行基.
	rel := text.ClusterStarts(lay.Text[lineStart:lineEnd])
	if delta > 0 {
		for _, r := range rel {
			if lineStart+r > curByte {
				e.SetCaretWithAffinity(lineStart+r, rendering.AffinityDownstream)
				return true
			}
		}
		// 已在行尾，跳到下一行行首 (downstream at new line).
		if nextStart, _, _, _, ok := lay.Line(lineIdx + 1); ok {
			e.SetCaretWithAffinity(nextStart, rendering.AffinityDownstream)
			return true
		}
		return false
	}
	for i := len(rel) - 1; i >= 0; i-- {
		if lineStart+rel[i] < curByte {
			e.SetCaretWithAffinity(lineStart+rel[i], rendering.AffinityDownstream)
			return true
		}
	}
	// 已在行首，跳到上一行行尾 (upstream → trailing of prev line).
	if _, prevEnd, _, _, ok := lay.Line(lineIdx - 1); ok {
		e.SetCaretWithAffinity(prevEnd, rendering.AffinityUpstream)
		return true
	}
	return false
}

// MoveVisualUp / MoveVisualDown implement Flutter sticky-column caret movement.
// caretCol is captured from the current penX on first up/down and reused until
// a horizontal move, click, SetCaret, Home/End clears it (mirrors F-A5).
func (e *Editor) MoveVisualUp(lay *rendering.TextLayout) bool { return e.moveVisualVertical(lay, -1) }
func (e *Editor) MoveVisualDown(lay *rendering.TextLayout) bool { return e.moveVisualVertical(lay, 1) }

func (e *Editor) moveVisualVertical(lay *rendering.TextLayout, dir int) bool {
	if e == nil || lay == nil || lay.LineCount() == 0 {
		if dir < 0 {
			return e.MoveCursorUp()
		}
		return e.MoveCursorDown()
	}
	curByte := e.GetCursorOffset()
	lineIdx, _, ok := lay.CaretForOffset(curByte)
	if !ok {
		if dir < 0 {
			return e.MoveCursorUp()
		}
		return e.MoveCursorDown()
	}
	target := lineIdx + dir
	if target < 0 || target >= lay.LineCount() {
		return false
	}
	// Capture sticky column from current caret X on first vertical move.
	sticky := e.caretCol
	stickyValid := e.caretColValid
	if !stickyValid {
		if x, _, _, ok := lay.GetOffsetForCaret(curByte, rendering.AffinityDownstream, 1.5); ok {
			sticky = x
			stickyValid = true
		}
	}
	// Find caret in target line whose X is nearest to sticky column.
	tStart, tEnd, _, _, ok := lay.Line(target)
	if !ok {
		return false
	}
	tCarets := lay.LineCarets(target)
	bestIdx := 0
	bestDist := 1e12
	for i, c := range tCarets {
		d := c.X - sticky
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	off := tCarets[bestIdx].ByteOff
	// M1-c:粘滞列命中的caret可能在簇内,按簇吸附(downstream).
	off = tStart + text.SnapCluster(lay.Text[tStart:tEnd], off-tStart, true)
	cu := 0
	for _, r := range e.text[:off] {
		if r > 0xFFFF {
			cu += 2
		} else {
			cu++
		}
	}
	// F-S2: must go through SetSelection to enforce composing && !collapsed and EditableRange clamp.
	if !e.SetSelection(TextRange{Base: cu, Extent: cu}) {
		return false
	}
	// Restore sticky semantics cleared by SetSelection.
	e.caretCol = sticky
	e.caretColValid = stickyValid
	if !e.caretColValid {
		e.caretColValid = true
	}
	return true
}
