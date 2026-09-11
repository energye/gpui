package textinput

import (
	"sort"

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
// visualStepByte computes one visual seam step from a byte offset without
// touching the selection: the shared core behind collapsed moves
// (MoveVisual) and extending moves (Box.extendVisual). delta is +1 (right)
// or -1 (left); cross-line hops land on the neighbor line start/end with
// the same affinities MoveVisual always used. ok=false when no layout is
// available; callers apply their own rune fallback then.
func visualStepByte(e *Editor, fromByte, delta int, lay *rendering.TextLayout) (newByte, affinity int, ok bool) {
	if e == nil || lay == nil || lay.LineCount() == 0 {
		return 0, 0, false
	}
	lineIdx, _, ok := lay.CaretForOffset(fromByte)
	if !ok {
		return 0, 0, false
	}
	if lineIdx < 0 {
		lineIdx = 0
	}
	if lineIdx >= lay.LineCount() {
		lineIdx = lay.LineCount() - 1
	}
	lineStart, _, _, _, ok := lay.Line(lineIdx)
	if !ok {
		return 0, 0, false
	}
	// M1-c:按字素簇 stepping,不再逐rune走缝.簇起点=行内相对簇表+行基.
	// 簇表随排版缓存,逐键不再整行重切;表内二分,行程与行位置无关.
	rel := lay.LineClusterStarts(lineIdx)
	if delta > 0 {
		if j := sort.Search(len(rel), func(i int) bool { return lineStart+rel[i] > fromByte }); j < len(rel) {
			return lineStart + rel[j], rendering.AffinityDownstream, true
		}
		// 已在行尾，跳到下一行行首 (downstream at new line).
		if nextStart, _, _, _, ok := lay.Line(lineIdx + 1); ok {
			return nextStart, rendering.AffinityDownstream, true
		}
		return 0, 0, false
	}
	if j := sort.Search(len(rel), func(i int) bool { return lineStart+rel[i] >= fromByte }) - 1; j >= 0 {
		return lineStart + rel[j], rendering.AffinityDownstream, true
	}
	// 已在行首，跳到上一行行尾 (upstream → trailing of prev line).
	if _, prevEnd, _, _, ok := lay.Line(lineIdx - 1); ok {
		return prevEnd, rendering.AffinityUpstream, true
	}
	return 0, 0, false
}

// nearestLineByte maps a sticky X column onto the nearest caret of the
// target line, snapped to a grapheme boundary. Shared by collapsed
// vertical moves and extending ones.
func nearestLineByte(lay *rendering.TextLayout, target int, x float64) (int, bool) {
	tStart, _, _, _, ok := lay.Line(target)
	if !ok {
		return 0, false
	}
	tCarets := lay.LineCarets(target)
	if len(tCarets) == 0 {
		return tStart, true
	}
	bestIdx := 0
	bestDist := 1e12
	for i, c := range tCarets {
		d := c.X - x
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	off := tCarets[bestIdx].ByteOff
	// M1-c:粘滞列命中的caret可能在簇内,按簇吸附(downstream).簇表复用排版缓存.
	return tStart + text.SnapInStarts(lay.LineClusterStarts(target), off-tStart, true), true
}

func (e *Editor) MoveVisual(delta int, lay *rendering.TextLayout) bool {
	if e == nil || lay == nil || lay.LineCount() == 0 {
		if delta < 0 {
			return e.MoveCursorBack()
		}
		return e.MoveCursorForward()
	}
	curByte := e.GetCursorOffset()
	if newByte, affinity, ok := visualStepByte(e, curByte, delta, lay); ok {
		e.stepCaretBytes(curByte, newByte, affinity)
		return true
	}
	if delta < 0 {
		return e.MoveCursorBack()
	}
	return e.MoveCursorForward()
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
	off, ok := nearestLineByte(lay, target, sticky)
	if !ok {
		return false
	}
	// F-S2: must go through SetSelection to enforce composing && !collapsed and EditableRange clamp.
	cu := e.extentForBytes(curByte, off)
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
