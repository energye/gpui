package textinput

import (
	"fmt"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

func NewViewportInputBox(ed *Editor, w, h, fontSize float64) *ViewportInputBox {
	if fontSize <= 0 {
		fontSize = 12
	}
	outer := rendering.NewRenderBox()
	vb := &ViewportInputBox{
		RenderBox: outer,
		ed:        ed,
		txt:       rendering.NewRenderText(""),
		content:   rendering.NewRenderBox(),
	}
	outer.Init(vb)
	outer.SetRelayoutBoundary(true)
	outer.SetRepaintBoundary(true)
	vb.content.Init(vb.content)
	vb.content.SetRelayoutBoundary(false)
	vb.txt.FontSize = fontSize
	vb.txt.R, vb.txt.G, vb.txt.B, vb.txt.A = 0.05, 0.75, 0.95, 1
	vb.txt.MaxWidth = 0 // single line, no wrap
	outer.FixedWidth = w
	outer.FixedHeight = h

	// Content holds txt+bar; viewport wraps content.
	vb.content.AddChild(vb.txt)
	vb.bar = rendering.NewRenderColorBox(1.5, 22, 1.0, 0.85, 0.2, 1)
	vb.content.AddChild(vb.bar)

	vp := rendering.NewRenderViewport(vb.content)
	vp.FixedWidth = w - 2
	vp.FixedHeight = h - 2
	outer.AddChild(vp)
	vb.Viewport = vp
	vp.SetOffset(rendering.Point{X: 1, Y: 1})

	outer.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc != nil && pc.DC != nil {
			if vb.focused {
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
	}
	vb.caretOn = true
	vb.Node = focus.NewFocusNode(fmt.Sprintf("viewport-input-%p", vb))
	vb.Node.Target = vb
	vb.Node.OnFocusChange = func(on bool) {
		vb.focused = on
		vb.MarkNeedsPaint()
		vb.sync()
	}
	ed.OnChange = func() { vb.sync() }
	vb.sync()
	return vb
}

// ViewportInputBox is the orthodox Flutter RenderEditable+Viewport pattern
// for the 5000 horizontal scroll case (R2). Structure:
//
//	outer (RenderBox 560×36, border paint)
//	 └─ viewport (RenderViewport 558×34, RepaintBoundary, clips)
//	     └─ content (RenderBox, no fixed size, holds txt+bar)
//	         ├─ txt (RenderText, 5000 glyphs SDF)
//	         └─ bar (RenderColorBox caret)
//
// Viewport.scrollX is the single scroll source (Flutter ensureCaretVisible).
// txt stays at X=0 inside content; bar at caretX. Viewport Paint applies
// -scrollX origin, so only the viewport texture is re-blitted on scroll.
// Retained Present therefore keeps damage small (viewport band + HUD only).
type ViewportInputBox struct {
	*rendering.RenderBox
	Viewport  *rendering.RenderViewport
	content   *rendering.RenderBox
	txt       *rendering.RenderText
	bar       *rendering.RenderColorBox
	ed        *Editor
	Node      *focus.FocusNode
	sched     func()
	focused   bool
	caretOn   bool
	clipboard platform.Clipboard
}

func (b *ViewportInputBox) IsFocused() bool                         { return b != nil && b.focused }
func (b *ViewportInputBox) SetClipboard(c platform.Clipboard)       { b.clipboard = c }
func (b *ViewportInputBox) Clipboard() platform.Clipboard           { return b.clipboard }
func (b *ViewportInputBox) SetSchedule(fn func())                   { b.sched = fn }
func (b *ViewportInputBox) FocusNode() *focus.FocusNode             { return b.Node }
func (b *ViewportInputBox) Editor() *Editor                         { return b.ed }
func (b *ViewportInputBox) ContentPurpose() platform.ContentPurpose { return platform.PurposeNormal }
func (b *ViewportInputBox) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: b.ContentPurpose()}
}
func (b *ViewportInputBox) TextLayout() *rendering.TextLayout {
	if b == nil || b.txt == nil {
		return nil
	}
	return b.txt.TextLayout()
}
func (b *ViewportInputBox) SetFace(face text.Face) {
	if b != nil && b.txt != nil {
		b.txt.SetFace(face)
		b.sync()
	}
}
func (b *ViewportInputBox) SetCaretOn(v bool) {
	if b == nil {
		return
	}
	b.caretOn = v
	b.layoutCaret()
	b.MarkNeedsPaint()
}

func (b *ViewportInputBox) caretAnchor() (float64, float64, float64, bool) {
	if b == nil || b.txt == nil || b.ed == nil || b.Viewport == nil {
		return 0, 0, 0, false
	}
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	if lay != nil && len(lay.Lines) > 0 {
		if x, y, h, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			vpOff := b.Viewport.ScrollOffset()
			txtOff := b.txt.Offset()
			cx := x - vpOff.X + 1
			cy := txtOff.Y + y + 1
			return cx, cy, cy + h, true
		}
	}
	return 0, 0, 0, false
}

func (b *ViewportInputBox) IMERect() platform.Rect {
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return platform.Rect{X: 0, Y: 0, W: 2, H: 22}
	}
	abs := absoluteOrigin(b)
	return platform.Rect{X: abs.X + x, Y: abs.Y + top, W: 2, H: bottom - top}
}

func (b *ViewportInputBox) sync() {
	if b == nil || b.txt == nil || b.ed == nil || b.Viewport == nil {
		return
	}
	disp := b.ed.GetText()
	if disp == "" && !b.focused {
		disp = "（5000 横滚）"
	}
	b.txt.SetText(disp)
	lh := b.txt.LineHeight()
	if lh <= 0 {
		lh = 22
	}
	textY := (b.FixedHeight - 2 - lh) / 2
	if textY < 0 {
		textY = 0
	}
	b.txt.SetOffset(rendering.Point{X: 0, Y: textY})
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	var caretX float64
	if lay != nil && len(lay.Lines) > 0 {
		if x, _, _, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			caretX = x
		}
	}
	visW := b.FixedWidth - 2
	scrollX := b.Viewport.ScrollOffset().X
	if caretX-scrollX > visW-4 {
		scrollX = caretX - visW + 4
	}
	if caretX-scrollX < 4 {
		scrollX = caretX - 4
	}
	if scrollX < 0 {
		scrollX = 0
	}
	b.Viewport.SetScrollOffset(scrollX, 0)
	b.txt.SetViewportHint(scrollX, visW)
	b.caretOn = true
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

func (b *ViewportInputBox) layoutCaret() {
	if b == nil || b.bar == nil {
		return
	}
	if b.txt == nil || b.ed == nil {
		return
	}
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	var x, y, h float64
	var ok bool
	if lay != nil && len(lay.Lines) > 0 {
		x, y, h, ok = lay.GetOffsetForCaret(curByte, aff, 1.5)
	}
	if !ok {
		return
	}
	txtOff := b.txt.Offset()
	b.bar.MoveTo(x-b.bar.Width/2, txtOff.Y+y)
	if h > 0 {
		b.bar.Height = h
	}
	if b.caretOn && b.focused {
		b.bar.SetAlpha(1)
	} else {
		b.bar.SetAlpha(0)
	}
}

func (b *ViewportInputBox) Layout(c rendering.Constraints) rendering.Size {
	txtOff := b.txt.Offset()
	barOff := b.bar.Offset()
	sz := b.RenderBox.Layout(c)
	b.txt.SetOffset(txtOff)
	b.bar.SetOffset(barOff)
	return sz
}

func (b *ViewportInputBox) OnPointer(ev input.PointerEvent) {
	if ev.Kind == input.PointerDown && b.Node != nil {
		b.Node.RequestFocus()
		abs := absoluteOrigin(b)
		vpOff := b.Viewport.ScrollOffset()
		txtOff := b.txt.Offset()
		localX := ev.X - abs.X - 1 + vpOff.X
		localY := ev.Y - abs.Y - 1 + vpOff.Y - txtOff.Y
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

func (b *ViewportInputBox) OnKey(ev input.KeyEvent) {
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
		b.ed.MoveVisual(-1, b.txt.TextLayout())
	case input.KeyArrowRight:
		b.ed.MoveVisual(1, b.txt.TextLayout())
	case input.KeyArrowUp:
		b.ed.MoveVisualUp(b.txt.TextLayout())
	case input.KeyArrowDown:
		b.ed.MoveVisualDown(b.txt.TextLayout())
	}
}

func (b *ViewportInputBox) MoveVisual(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	b.ed.MoveVisual(delta, b.txt.TextLayout())
}
func (b *ViewportInputBox) OnText(ev input.TextEvent) {}
func (b *ViewportInputBox) OnIME(ev input.IMEEvent)   {}
