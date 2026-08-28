package textinput

import (
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
	if e == nil || lay == nil || len(lay.Lines) == 0 {
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
	if lineIdx >= len(lay.Lines) {
		lineIdx = len(lay.Lines) - 1
	}
	ln := lay.Lines[lineIdx]
	// 在该行内找当前缝的下标（精确 ByteOff 匹配，找不到就最近）
	idx := -1
	for j, c := range ln.Carets {
		if c.ByteOff == curByte {
			idx = j
			break
		}
	}
	if idx < 0 {
		best, bestDist := -1, 1<<30
		for j, c := range ln.Carets {
			d := c.ByteOff - curByte
			if d < 0 {
				d = -d
			}
			if d < bestDist {
				bestDist = d
				best = j
			}
		}
		idx = best
	}
	if idx < 0 {
		if delta < 0 {
			return e.MoveCursorBack()
		}
		return e.MoveCursorForward()
	}
	// 行内移动 — Flutter visual seam, affinity downstream except cross-line trailing.
	if delta > 0 {
		if idx+1 < len(ln.Carets) {
			off := ln.Carets[idx+1].ByteOff
			e.SetCaretWithAffinity(off, rendering.AffinityDownstream)
			return true
		}
		// 已在行尾，跳到下一行行首 (downstream at new line).
		if lineIdx+1 < len(lay.Lines) {
			off := lay.Lines[lineIdx+1].Carets[0].ByteOff
			e.SetCaretWithAffinity(off, rendering.AffinityDownstream)
			return true
		}
		return false
	}
	// delta < 0
	if idx-1 >= 0 {
		off := ln.Carets[idx-1].ByteOff
		e.SetCaretWithAffinity(off, rendering.AffinityDownstream)
		return true
	}
	// 已在行首，跳到上一行行尾 (upstream → trailing of prev line).
	if lineIdx-1 >= 0 {
		prev := lay.Lines[lineIdx-1]
		off := prev.Carets[len(prev.Carets)-1].ByteOff
		e.SetCaretWithAffinity(off, rendering.AffinityUpstream)
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
	if e == nil || lay == nil || len(lay.Lines) == 0 {
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
	if target < 0 || target >= len(lay.Lines) {
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
	tln := lay.Lines[target]
	bestIdx := 0
	bestDist := 1e12
	for i, c := range tln.Carets {
		d := c.X - sticky
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	off := tln.Carets[bestIdx].ByteOff
	cu := 0
	for _, r := range e.text[:off] {
		if r > 0xFFFF {
			cu += 2
		} else {
			cu++
		}
	}
	// Directly set selection without clearing sticky semantics — bypass SetSelection's reset.
	e.selection = TextRange{Base: cu, Extent: cu}
	e.caretCol = sticky
	e.caretColValid = stickyValid
	// Keep sticky valid for continued vertical travel.
	if !e.caretColValid {
		e.caretColValid = true
	}
	e.changed()
	return true
}
