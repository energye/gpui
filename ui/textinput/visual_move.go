package textinput

import "github.com/energye/gpui/ui/rendering"

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
	// 行内移动
	if delta > 0 {
		if idx+1 < len(ln.Carets) {
			off := ln.Carets[idx+1].ByteOff
			e.SetCaret(off)
			return true
		}
		// 已在行尾，跳到下一行行首
		if lineIdx+1 < len(lay.Lines) {
			off := lay.Lines[lineIdx+1].Carets[0].ByteOff
			e.SetCaret(off)
			return true
		}
		return false
	}
	// delta < 0
	if idx-1 >= 0 {
		off := ln.Carets[idx-1].ByteOff
		e.SetCaret(off)
		return true
	}
	// 已在行首，跳到上一行行尾
	if lineIdx-1 >= 0 {
		prev := lay.Lines[lineIdx-1]
		off := prev.Carets[len(prev.Carets)-1].ByteOff
		e.SetCaret(off)
		return true
	}
	return false
}
