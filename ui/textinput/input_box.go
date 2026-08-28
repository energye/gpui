package textinput

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// InputBox is a single-line text field backed by Editor.
// It owns a RenderText + caret and handles focus, scroll, IME rect and
// pointer-to-caret mapping via the single-source TextLayout (Flutter-aligned).
type InputBox struct {
	*rendering.RenderBox
	ed        *Editor
	txt       *rendering.RenderText
	bar       *rendering.RenderColorBox
	Node      *focus.FocusNode
	sched     func()
	focused   bool
	caretOn   bool
	scrollX   float64
	clipboard platform.Clipboard
}

func (b *InputBox) IsFocused() bool { return b != nil && b.focused }
func (b *InputBox) IsCaretOn() bool { return b != nil && b.caretOn }
func (b *InputBox) SetCaretOn(v bool) {
	if b == nil {
		return
	}
	b.caretOn = v
	b.layoutCaret()
	b.MarkNeedsPaint()
}
func (b *InputBox) Sync() { b.sync() }

// NewInputBox creates a single-line box. fontSize <=0 defaults to 16.
func NewInputBox(ed *Editor, w, h, fontSize float64) *InputBox {
	if fontSize <= 0 {
		fontSize = 16
	}
	inner := rendering.NewRenderBox()
	b := &InputBox{
		RenderBox: inner,
		ed:        ed,
		txt:       rendering.NewRenderText(""),
	}
	inner.Init(b)
	// Flutter RenderEditable is both RelayoutBoundary and RepaintBoundary:
	// typing 5000 chars must not relayout the whole window nor repaint
	// siblings. Without this, each keystroke bubbles MarkNeedsLayout/Paint
	// to the shell root, collapsing fps and causing input lag.
	inner.SetRelayoutBoundary(true)
	inner.SetRepaintBoundary(true)
	b.txt.FontSize = fontSize
	// Face is optional; caller may set via SetFace or via rendering.LoadMultiFace.
	b.txt.R, b.txt.G, b.txt.B, b.txt.A = 0.05, 0.75, 0.95, 1
	b.FixedWidth = w
	b.FixedHeight = h
	// 内容裁剪：超长/多行文本在框外不可见（R1/R2 手工验证溢出）
	clip := rendering.NewRenderClipRRect()
	clip.FixedWidth = w
	clip.FixedHeight = h
	clip.SetRadius(3)
	b.AddChild(clip)
	clip.AddChild(b.txt)
	b.bar = rendering.NewRenderColorBox(1.5, 22, 1.0, 0.85, 0.2, 1)
	clip.AddChild(b.bar)
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc != nil && pc.DC != nil {
			// 外边框：一眼可辨是输入框；获焦蓝框，未获焦灰框（与 R1 手工可难度对齐）
			if b.focused {
				pc.DC.SetRGBA(0.30, 0.58, 0.95, 1)
			} else {
				pc.DC.SetRGBA(0.38, 0.46, 0.56, 1)
			}
			pc.DC.SetLineWidth(1.4)
			pc.DC.DrawRectangle(pc.OriginX+0.7, pc.OriginY+0.7, size.Width-1.4, size.Height-1.4)
			_ = pc.DC.Stroke()
			// 内底：深灰底让文字可读，边框更突出（在 clip 外绘制，不被裁剪）
			pc.DC.SetRGBA(0.13, 0.15, 0.18, 1)
			pc.DC.DrawRectangle(pc.OriginX+1, pc.OriginY+1, size.Width-2, size.Height-2)
			_ = pc.DC.Fill()
		}
		b.layoutCaret()
	}
	b.caretOn = true
	b.Node = focus.NewFocusNode(fmt.Sprintf("input-%.0f-%p", fontSize, b))
	b.Node.Target = b
	b.Node.OnFocusChange = func(on bool) {
		b.focused = on
		b.MarkNeedsPaint()
		b.sync()
	}
	ed.OnChange = func() { b.sync() }
	b.sync()
	return b
}

// SetFace sets the font face for this box.
func (b *InputBox) SetFace(face text.Face) {
	if b != nil && b.txt != nil {
		b.txt.SetFace(face)
		b.sync()
	}
}

// SetClipboard sets the platform clipboard.
func (b *InputBox) SetClipboard(c platform.Clipboard) { b.clipboard = c }
func (b *InputBox) Clipboard() platform.Clipboard { return b.clipboard }

// SetSchedule sets the frame scheduler (PipelineApp.ScheduleFrame).
func (b *InputBox) SetSchedule(fn func()) { b.sched = fn }

// FocusNode returns the focus node for registration.
func (b *InputBox) FocusNode() *focus.FocusNode { return b.Node }

// Editor returns the backing editor.
func (b *InputBox) Editor() *Editor { return b.ed }
func (b *InputBox) ContentPurpose() platform.ContentPurpose { return platform.PurposeNormal }
func (b *InputBox) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: b.ContentPurpose()}
}

func absoluteOrigin(n rendering.RenderObject) rendering.Point {
	x, y := 0.0, 0.0
	for cur := n; cur != nil; cur = cur.Parent() {
		off := cur.Offset()
		x += off.X
		y += off.Y
	}
	return rendering.Point{X: x, Y: y}
}

func (b *InputBox) caretAnchor() (float64, float64, float64, bool) {
	if b == nil || b.txt == nil || b.ed == nil {
		return 0, 0, 0, false
	}
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	if lay != nil && len(lay.Lines) > 0 {
		if x, y, h, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			off := b.txt.Offset()
			return off.X + x, off.Y + y, off.Y + y + h, true
		}
	}
	lineIdx, penX, ok := b.txt.CaretColumn(min(curByte, len(b.txt.Text)))
	if !ok {
		return 0, 0, 0, false
	}
	off := b.txt.Offset()
	x := off.X + penX
	var top, bottom float64
	if lay != nil && len(lay.Lines) > lineIdx {
		top = off.Y + lay.LineTop(lineIdx)
		bottom = top + lay.LineHeight(lineIdx)
	} else {
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		top = off.Y + float64(lineIdx)*lh
		bottom = top + lh
	}
	return x, top, bottom, true
}

func (b *InputBox) IMERect() platform.Rect {
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return platform.Rect{X: 0, Y: 0, W: 2, H: 22}
	}
	abs := absoluteOrigin(b)
	return platform.Rect{X: abs.X + x, Y: abs.Y + top, W: 2, H: bottom - top}
}

func (b *InputBox) sync() {
	if b == nil || b.txt == nil || b.ed == nil {
		return
	}
	disp := b.ed.GetText()
	if disp == "" && !b.focused {
		disp = "（点此获焦）"
	}
	b.txt.SetText(disp)
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lh := b.txt.LineHeight()
	if lh <= 0 {
		lh = 22
	}
	textY := (b.FixedHeight - lh) / 2
	if textY < 0 {
		textY = 0
	}
	visW := b.FixedWidth - 16
	if lay := b.txt.TextLayout(); lay != nil && len(lay.Lines) > 0 {
		caretX := 0.0
		if x, _, _, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			caretX = x
		} else {
			_, caretX, _ = lay.CaretForOffset(curByte)
		}
		if caretX-b.scrollX > visW-4 {
			b.scrollX = caretX - visW + 4
		}
		if caretX-b.scrollX < 4 {
			b.scrollX = caretX - 4
		}
		if b.scrollX < 0 {
			b.scrollX = 0
		}
		b.txt.SetOffset(rendering.Point{X: 8 - b.scrollX, Y: textY})
	} else {
		b.txt.SetOffset(rendering.Point{X: 8 - b.scrollX, Y: textY})
	}
	b.txt.SetViewportHint(b.scrollX, visW)
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

func (b *InputBox) layoutCaret() {
	if b == nil || b.bar == nil {
		return
	}
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return
	}
	b.bar.MoveTo(x-b.bar.Width/2, top)
	if h := bottom - top; h > 0 {
		b.bar.Height = h
	}
	if b.caretOn && b.focused {
		b.bar.SetAlpha(1)
	} else {
		b.bar.SetAlpha(0)
	}
}

// Layout preserves the text/caret offsets computed by sync().
// RenderBox.Layout would reset children to Pad(0,0).
func (b *InputBox) Layout(c rendering.Constraints) rendering.Size {
	textOff := b.txt.Offset()
	barOff := b.bar.Offset()
	sz := b.RenderBox.Layout(c)
	b.txt.SetOffset(textOff)
	b.bar.SetOffset(barOff)
	return sz
}

func (b *InputBox) OnPointer(ev input.PointerEvent) {
	if ev.Kind == input.PointerDown && b.Node != nil {
		b.Node.RequestFocus()
		abs := absoluteOrigin(b)
		off := b.txt.Offset()
		localX := ev.X - abs.X - off.X
		localY := ev.Y - abs.Y - off.Y
		lay := b.txt.TextLayout()
		var byteOff, aff int
		if lay != nil {
			byteOff, aff = lay.GetPositionForOffset(localX, localY)
		} else {
			byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
			aff = rendering.AffinityDownstream
		}
		b.ed.SetCaretWithAffinity(byteOff, aff)
	}
}

func (b *InputBox) OnKey(ev input.KeyEvent) {
	if !ev.Pressed {
		return
	}
	if ev.Mods.Control || ev.Mods.Meta {
		switch ev.Key {
		case input.KeyA:
			b.ed.SelectAll()
			return
		case input.KeyC:
			s := b.ed.Copy()
			if s != "" && b.clipboard != nil {
				_ = b.clipboard.Set("text/plain", s)
			}
			return
		case input.KeyX:
			s := b.ed.Cut()
			if s != "" && b.clipboard != nil {
				_ = b.clipboard.Set("text/plain", s)
			}
			return
		case input.KeyV:
			return
		}
	}
	switch ev.Key {
	case input.KeyBackspace:
		b.ed.DeleteBackward()
	case input.KeyDelete:
		b.ed.DeleteForward()
	case input.KeyArrowLeft:
		b.moveVisual(-1)
	case input.KeyArrowRight:
		b.moveVisual(1)
	case input.KeyArrowUp:
		b.ed.MoveCursorUp()
	case input.KeyArrowDown:
		b.ed.MoveCursorDown()
	case input.KeyEscape:
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

func (b *InputBox) moveVisual(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	b.ed.MoveVisual(delta, b.txt.TextLayout())
}
func (b *InputBox) MoveVisual(delta int) { b.moveVisual(delta) }
func (b *InputBox) TextLayout() *rendering.TextLayout {
	if b == nil || b.txt == nil {
		return nil
	}
	return b.txt.TextLayout()
}
func (b *InputBox) OnText(ev input.TextEvent) {}
func (b *InputBox) OnIME(ev input.IMEEvent)    {}

// MultiLineInputBox is a wrapping editor. It scrolls both axes.
type MultiLineInputBox struct {
	*rendering.RenderBox
	ed        *Editor
	txt       *rendering.RenderText
	bar       *rendering.RenderColorBox
	Node      *focus.FocusNode
	sched     func()
	focused   bool
	caretOn   bool
	scrollX   float64
	scrollY   float64
	clipboard platform.Clipboard
}

func (b *MultiLineInputBox) IsFocused() bool { return b != nil && b.focused }
func (b *MultiLineInputBox) IsCaretOn() bool { return b != nil && b.caretOn }
func (b *MultiLineInputBox) SetCaretOn(v bool) {
	if b == nil {
		return
	}
	b.caretOn = v
	b.layoutCaret()
	b.MarkNeedsPaint()
}
func (b *MultiLineInputBox) Sync() { b.sync() }

func NewMultiLineInputBox(ed *Editor, w, h, fontSize float64) *MultiLineInputBox {
	if fontSize <= 0 {
		fontSize = 14
	}
	inner := rendering.NewRenderBox()
	b := &MultiLineInputBox{
		RenderBox: inner,
		ed:        ed,
		txt:       rendering.NewRenderText(""),
	}
	inner.Init(b)
	inner.SetRelayoutBoundary(true)
	inner.SetRepaintBoundary(true)
	b.txt.FontSize = fontSize
	b.txt.R, b.txt.G, b.txt.B, b.txt.A = 0.06, 0.85, 0.60, 1
	b.txt.MaxWidth = w - 16
	b.FixedWidth = w
	b.FixedHeight = h
	clip := rendering.NewRenderClipRRect()
	clip.FixedWidth = w
	clip.FixedHeight = h
	clip.SetRadius(3)
	b.AddChild(clip)
	clip.AddChild(b.txt)
	b.bar = rendering.NewRenderColorBox(1.5, 22, 1.0, 0.85, 0.2, 1)
	clip.AddChild(b.bar)
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc != nil && pc.DC != nil {
			if b.focused {
				pc.DC.SetRGBA(0.30, 0.58, 0.95, 1)
			} else {
				pc.DC.SetRGBA(0.38, 0.46, 0.56, 1)
			}
			pc.DC.SetLineWidth(1.4)
			pc.DC.DrawRectangle(pc.OriginX+0.7, pc.OriginY+0.7, size.Width-1.4, size.Height-1.4)
			_ = pc.DC.Stroke()
			pc.DC.SetRGBA(0.13, 0.15, 0.18, 1)
			pc.DC.DrawRectangle(pc.OriginX+1, pc.OriginY+1, size.Width-2, size.Height-2)
			_ = pc.DC.Fill()
		}
		b.layoutCaret()
	}
	b.caretOn = true
	b.Node = focus.NewFocusNode(fmt.Sprintf("multi-%p", b))
	b.Node.Target = b
	b.Node.OnFocusChange = func(on bool) {
		b.focused = on
		b.MarkNeedsPaint()
		b.sync()
	}
	ed.OnChange = func() { b.sync() }
	b.sync()
	return b
}

func (b *MultiLineInputBox) SetFace(face text.Face) {
	if b != nil && b.txt != nil {
		b.txt.SetFace(face)
		b.sync()
	}
}
func (b *MultiLineInputBox) SetClipboard(c platform.Clipboard) { b.clipboard = c }
func (b *MultiLineInputBox) Clipboard() platform.Clipboard     { return b.clipboard }
func (b *MultiLineInputBox) SetSchedule(fn func())             { b.sched = fn }
func (b *MultiLineInputBox) FocusNode() *focus.FocusNode       { return b.Node }
func (b *MultiLineInputBox) Editor() *Editor                   { return b.ed }
func (b *MultiLineInputBox) ContentPurpose() platform.ContentPurpose {
	return platform.PurposeNormal
}
func (b *MultiLineInputBox) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: b.ContentPurpose()}
}

func (b *MultiLineInputBox) caretAnchor() (float64, float64, float64, bool) {
	if b == nil || b.txt == nil || b.ed == nil {
		return 0, 0, 0, false
	}
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	if lay != nil && len(lay.Lines) > 0 {
		if x, y, h, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			off := b.txt.Offset()
			return off.X + x, off.Y + y, off.Y + y + h, true
		}
	}
	lineIdx, penX, ok := b.txt.CaretColumn(min(curByte, len(b.txt.Text)))
	if !ok {
		return 0, 0, 0, false
	}
	off := b.txt.Offset()
	x := off.X + penX
	var top, bottom float64
	if lay != nil && len(lay.Lines) > lineIdx {
		top = off.Y + lay.LineTop(lineIdx)
		bottom = top + lay.LineHeight(lineIdx)
	} else {
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		top = off.Y + float64(lineIdx)*lh
		bottom = top + lh
	}
	return x, top, bottom, true
}

func (b *MultiLineInputBox) IMERect() platform.Rect {
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return platform.Rect{X: 0, Y: 0, W: 2, H: 22}
	}
	abs := absoluteOrigin(b)
	return platform.Rect{X: abs.X + x, Y: abs.Y + top, W: 2, H: bottom - top}
}

func (b *MultiLineInputBox) sync() {
	if b == nil || b.txt == nil || b.ed == nil {
		return
	}
	disp := b.ed.GetText()
	if disp == "" && !b.focused {
		disp = "（多行：点获焦，Enter 换行）"
	}
	b.txt.SetText(disp)
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	var caretX, caretY, caretH float64
	var lineIdx int
	if lay != nil && len(lay.Lines) > 0 {
		if x, y, h, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			caretX, caretY, caretH = x, y, h
			for i := range lay.Lines {
				top := lay.LineTop(i)
				ht := lay.LineHeight(i)
				if y >= top-0.01 && y < top+ht-0.01 {
					lineIdx = i
					break
				}
			}
		} else {
			lineIdx, caretX, _ = lay.CaretForOffset(curByte)
			caretY = lay.LineTop(lineIdx)
			caretH = lay.LineHeight(lineIdx)
		}
	} else {
		lineIdx, caretX, _ = b.txt.CaretColumn(min(curByte, len(b.txt.Text)))
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		caretY = float64(lineIdx) * lh
		caretH = lh
	}
	_ = caretH
	visW := b.FixedWidth - 16
	visH := b.FixedHeight - 16
	if caretX-b.scrollX > visW-4 {
		b.scrollX = caretX - visW + 4
	}
	if caretX-b.scrollX < 4 {
		b.scrollX = caretX - 4
	}
	if b.scrollX < 0 {
		b.scrollX = 0
	}
	var lineH float64
	if lay != nil && len(lay.Lines) > lineIdx {
		lineH = lay.LineHeight(lineIdx)
	} else {
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		lineH = lh
	}
	if caretY-b.scrollY < 4 {
		b.scrollY = caretY - 4
	}
	if caretY+lineH-b.scrollY > visH-4 {
		b.scrollY = caretY + lineH - visH + 4
	}
	if b.scrollY < 0 {
		b.scrollY = 0
	}
	if lay != nil && len(lay.Lines) > 0 {
		totalH := lay.LineTop(len(lay.Lines)-1) + lay.LineHeight(len(lay.Lines)-1)
		maxY := totalH - visH
		if maxY < 0 {
			maxY = 0
		}
		if b.scrollY > maxY {
			b.scrollY = maxY
		}
	}
	b.txt.SetOffset(rendering.Point{X: 8 - b.scrollX, Y: 8 - b.scrollY})
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

func (b *MultiLineInputBox) layoutCaret() {
	if b == nil || b.bar == nil {
		return
	}
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return
	}
	b.bar.MoveTo(x-b.bar.Width/2, top)
	if h := bottom - top; h > 0 {
		b.bar.Height = h
	}
	if b.caretOn && b.focused {
		b.bar.SetAlpha(1)
	} else {
		b.bar.SetAlpha(0)
	}
}

func (b *MultiLineInputBox) Layout(c rendering.Constraints) rendering.Size {
	textOff := b.txt.Offset()
	barOff := b.bar.Offset()
	sz := b.RenderBox.Layout(c)
	b.txt.SetOffset(textOff)
	b.bar.SetOffset(barOff)
	return sz
}

func (b *MultiLineInputBox) OnPointer(ev input.PointerEvent) {
	if ev.Kind == input.PointerDown && b.Node != nil {
		b.Node.RequestFocus()
		abs := absoluteOrigin(b)
		off := b.txt.Offset()
		localX := ev.X - abs.X - off.X
		localY := ev.Y - abs.Y - off.Y
		lay := b.txt.TextLayout()
		var byteOff, aff int
		if lay != nil {
			byteOff, aff = lay.GetPositionForOffset(localX, localY)
		} else {
			byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
			aff = rendering.AffinityDownstream
		}
		b.ed.SetCaretWithAffinity(byteOff, aff)
	}
}

func (b *MultiLineInputBox) OnKey(ev input.KeyEvent) {
	if !ev.Pressed {
		return
	}
	if ev.Mods.Control || ev.Mods.Meta {
		switch ev.Key {
		case input.KeyA:
			b.ed.SelectAll()
			return
		case input.KeyC:
			s := b.ed.Copy()
			if s != "" && b.clipboard != nil {
				_ = b.clipboard.Set("text/plain", s)
			}
			return
		case input.KeyX:
			s := b.ed.Cut()
			if s != "" && b.clipboard != nil {
				_ = b.clipboard.Set("text/plain", s)
			}
			return
		case input.KeyV:
			return
		}
	}
	switch ev.Key {
	case input.KeyBackspace:
		b.ed.DeleteBackward()
	case input.KeyDelete:
		b.ed.DeleteForward()
	case input.KeyArrowLeft:
		b.moveVisual(-1)
	case input.KeyArrowRight:
		b.moveVisual(1)
	case input.KeyArrowUp:
		if lay := b.txt.TextLayout(); lay != nil {
			b.ed.MoveVisualUp(lay)
		} else {
			b.ed.MoveCursorUp()
		}
	case input.KeyArrowDown:
		if lay := b.txt.TextLayout(); lay != nil {
			b.ed.MoveVisualDown(lay)
		} else {
			b.ed.MoveCursorDown()
		}
	case input.KeyEnter:
		b.ed.Insert("\n")
	case input.KeyEscape:
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

func (b *MultiLineInputBox) moveVisual(delta int) {
	lay := b.txt.TextLayout()
	b.ed.MoveVisual(delta, lay)
}
func (b *MultiLineInputBox) MoveVisual(delta int) { b.moveVisual(delta) }
func (b *MultiLineInputBox) TextLayout() *rendering.TextLayout {
	if b == nil || b.txt == nil {
		return nil
	}
	return b.txt.TextLayout()
}
func (b *MultiLineInputBox) OnText(ev input.TextEvent) {}
func (b *MultiLineInputBox) OnIME(ev input.IMEEvent)    {}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Strings helper for paste decode is handled in embedder/router, not here.
func init() { _ = strings.Contains }