package textinput

import "github.com/energye/gpui/ui/rendering"

// MoveVisual moves the cursor by one visual seam (gap) using the single-source
// TextLayout. It is the shipped path used by the true window's manualInputBox
// and is the only honest way to test visual movement: callers must not
// re-implement caret stepping via SetCaret.
//
// delta is +1 (right) or -1 (left). Returns true if the cursor moved.
func (e *Editor) MoveVisual(delta int, lay *rendering.TextLayout) bool {
	if e == nil || lay == nil || len(lay.Lines) == 0 {
		if delta < 0 {
			return e.MoveCursorBack()
		}
		return e.MoveCursorForward()
	}
	curByte := e.GetCursorOffset()
	ln := lay.Lines[0]
	// Find nearest seam via HitTest (robust for inside multi-byte)
	_, penX, _ := lay.CaretForOffset(curByte)
	hit := lay.HitTest(penX, 0, lay.FontSize*1.25)
	idx := -1
	for j, c := range ln.Carets {
		if c.ByteOff == hit {
			idx = j
			break
		}
	}
	if idx < 0 {
		// fallback: nearest
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
	ni := idx + delta
	if ni < 0 {
		ni = 0
	}
	if ni >= len(ln.Carets) {
		ni = len(ln.Carets) - 1
	}
	if ni == idx {
		return false
	}
	off := ln.Carets[ni].ByteOff
	e.SetCaret(off)
	return true
}
