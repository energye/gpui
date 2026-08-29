package textinput

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

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
	if ed != nil {
		ed.SetSingleLine(true)
	}
	outer := rendering.NewRenderBox()
	vb := &ViewportInputBox{
		RenderBox:    outer,
		BaseEditable: NewBaseEditable(ed),
		ed:           ed,
		txt:          rendering.NewRenderText(""),
		content:      rendering.NewRenderBox(),
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
		if vb.Disabled() && on {
			return
		}
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
	*BaseEditable
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
	blinkElapsed float64
	dragging    bool
	dragStart   int
	highlights  []*rendering.RenderColorBox
	lastClickAt time.Time
	lastClickX  float64
	lastClickY  float64
	clickCount  int
	selHas                bool
	selR, selG, selB, selA float64
	padHas                bool
	pad                   float64
	dragLastX             float64
	dragLastY             float64
	autoScrollRunning     bool
}

func (b *ViewportInputBox) SetPlaceholder(s string) {
	if b == nil || b.BaseEditable == nil {
		return
	}
	b.BaseEditable.SetPlaceholder(s)
	if b.txt != nil && b.ed != nil && b.ed.GetText() == "" {
		b.sync()
	}
}
func (b *ViewportInputBox) Placeholder() string {
	if b == nil || b.BaseEditable == nil {
		return ""
	}
	return b.BaseEditable.Placeholder()
}
func (b *ViewportInputBox) SetDisabled(v bool) {
	if b == nil || b.BaseEditable == nil {
		return
	}
	b.BaseEditable.SetDisabled(v)
	b.MarkNeedsPaint()
}
func (b *ViewportInputBox) Disabled() bool {
	if b == nil || b.BaseEditable == nil {
		return false
	}
	return b.BaseEditable.Disabled()
}
func (b *ViewportInputBox) SetSelectionColor(r, g, b2, a float64) {
	if b == nil {
		return
	}
	b.selHas = true
	b.selR, b.selG, b.selB, b.selA = r, g, b2, a
	b.MarkNeedsPaint()
}
func (b *ViewportInputBox) ClearSelectionColor() {
	if b == nil {
		return
	}
	b.selHas = false
	b.MarkNeedsPaint()
}
func (b *ViewportInputBox) SelectionColor() (r, g, b2, a float64) {
	return resolveSelColor(b.selHas, b.selR, b.selG, b.selB, b.selA)
}
func (b *ViewportInputBox) SetPadding(pad float64) {
	if b == nil {
		return
	}
	if pad < 0 {
		pad = 0
	}
	b.padHas = true
	b.pad = pad
	b.sync()
}
func (b *ViewportInputBox) ClearPadding() {
	if b == nil {
		return
	}
	b.padHas = false
	b.sync()
}
func (b *ViewportInputBox) Padding() float64 { return resolvePad(b.padHas, b.pad) }

func (b *ViewportInputBox) IsFocused() bool                         { return b != nil && b.focused }
func (b *ViewportInputBox) ScrollX() float64 {
	if b == nil || b.Viewport == nil {
		return 0
	}
	return b.Viewport.ScrollOffset().X
}
func (b *ViewportInputBox) ScrollY() float64 {
	if b == nil || b.Viewport == nil {
		return 0
	}
	return b.Viewport.ScrollOffset().Y
}
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
	b.blinkElapsed = 0
	b.layoutCaret()
	b.MarkNeedsPaint()
}

func (b *ViewportInputBox) TickCaret(dt float64) {
	if b == nil || !b.focused {
		if b != nil && b.caretOn {
			b.caretOn = false
			b.layoutCaret()
		}
		return
	}
	b.blinkElapsed += dt
	if b.blinkElapsed >= 0.5 {
		b.blinkElapsed = 0
		b.caretOn = !b.caretOn
		b.layoutCaret()
		b.MarkNeedsPaint()
	}
}

func (b *ViewportInputBox) caretAnchor() (float64, float64, float64, bool) {
	if b == nil || b.txt == nil || b.ed == nil || b.Viewport == nil {
		return 0, 0, 0, false
	}
	txtOff := b.txt.Offset()
	vpOff := b.Viewport.ScrollOffset()
	if b.txt.Text == "" {
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		cx := txtOff.X - vpOff.X + 1
		cy := txtOff.Y + 1
		return cx, cy, cy + lh, true
	}
	// Password: map runeIdx to masked byte like InputBox.
	if b.ed.IsPassword() {
		ch := b.ed.ObscuringCharacter()
		chBytes := len(string(ch))
		if chBytes <= 0 {
			chBytes = 3
		}
		runeIdx := utf16ToRuneIndex(b.ed.GetText(), b.ed.SelectionRange().Extent)
		maskedByte := runeIdx * chBytes
		aff := b.ed.TextRange().Affinity
		lay := b.txt.TextLayout()
		if lay != nil && len(lay.Lines) > 0 {
			if x, y, h, ok := lay.GetOffsetForCaret(maskedByte, aff, 1.5); ok {
				cx := txtOff.X + x - vpOff.X + 1
				cy := txtOff.Y + y + 1
				return cx, cy, cy + h, true
			}
		}
		off := txtOff
		charW := b.txt.MeasureWidth(string(ch))
		if charW <= 0 {
			charW = 12
		}
		x := off.X + float64(runeIdx)*charW - vpOff.X + 1
		top := off.Y + 1
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		return x, top, top + lh, true
	}
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	if lay != nil && len(lay.Lines) > 0 {
		if x, y, h, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			cx := txtOff.X + x - vpOff.X + 1
			cy := txtOff.Y + y + 1
			return cx, cy, cy + h, true
		}
	}
	return 0, 0, 0, false
}

func (b *ViewportInputBox) IMERect() platform.Rect {
	if b != nil && b.ed != nil && b.ed.IsComposing() {
		if lay := b.txt.TextLayout(); lay != nil && len(lay.Lines) > 0 {
			cr := b.ed.ComposingRange()
			s := byteOffsetForUtf16(b.ed.GetText(), cr.Start())
			e := byteOffsetForUtf16(b.ed.GetText(), cr.End())
			if s < e {
				if boxes := lay.BoxesForRange(s, e); len(boxes) > 0 {
					minX, minY := boxes[0].Min.X, boxes[0].Min.Y
					maxX, maxY := boxes[0].Max.X, boxes[0].Max.Y
					for _, r := range boxes[1:] {
						if r.Min.X < minX {
							minX = r.Min.X
						}
						if r.Min.Y < minY {
							minY = r.Min.Y
						}
						if r.Max.X > maxX {
							maxX = r.Max.X
						}
						if r.Max.Y > maxY {
							maxY = r.Max.Y
						}
					}
					off := b.txt.Offset()
					vpOff := b.Viewport.ScrollOffset()
					abs := absoluteOrigin(b)
					return platform.Rect{X: abs.X + off.X + minX - vpOff.X + 1, Y: abs.Y + off.Y + minY + 1, W: maxX - minX, H: maxY - minY}
				}
			}
		}
	}
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return platform.Rect{X: 0, Y: 0, W: 2, H: 22}
	}
	abs := absoluteOrigin(b)
	return platform.Rect{X: abs.X + x, Y: abs.Y + top, W: 2, H: bottom - top}
}

func (b *ViewportInputBox) Sync() { b.sync() }

func (b *ViewportInputBox) sync() {
	if b == nil || b.txt == nil || b.ed == nil || b.Viewport == nil {
		return
	}
	isPassword := b.ed.IsPassword()
	disp := b.ed.GetText()
	if isPassword && disp != "" {
		ch := b.ed.ObscuringCharacter()
		disp = strings.Repeat(string(ch), len([]rune(disp)))
	} else if disp == "" && !b.focused && b.placeholder != "" {
		disp = b.placeholder
	}
	b.txt.SetText(disp)
	if disp == "" {
		b.Viewport.SetScrollOffset(0, 0)
	}
	lh := b.txt.LineHeight()
	if lh <= 0 {
		lh = 22
	}
	textY := (b.FixedHeight - 2 - lh) / 2
	if textY < 0 {
		textY = 0
	}
	pad := b.Padding()
	b.txt.SetOffset(rendering.Point{X: pad, Y: textY})
	curByte := b.ed.GetCursorOffset()
	if isPassword {
		runes := []rune(b.ed.GetText())
		runeIdx := 0
		for i := range b.ed.GetText()[:viewportMin(curByte, len(b.ed.GetText()))] {
			if (b.ed.GetText()[i]&0xC0) != 0x80 {
				runeIdx++
			}
		}
		if runeIdx > len(runes) {
			runeIdx = len(runes)
		}
		maskedRunes := []rune(disp)
		if runeIdx > len(maskedRunes) {
			runeIdx = len(maskedRunes)
		}
		curByte = len(string(maskedRunes[:runeIdx]))
		_ = utf8.RuneCountInString
	}
	aff := b.ed.TextRange().Affinity
	lay := b.txt.TextLayout()
	var caretX float64
	if lay != nil && len(lay.Lines) > 0 {
		if x, _, _, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			caretX = x
		}
	}
	visW := b.FixedWidth - 2 - 2*pad
	if visW < 0 {
		visW = 0
	}
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
	// 同 InputBox：按行宽算 maxScroll，删除后自动回移，适配任意字号
	maxW := 0.0
	if lay != nil && len(lay.Lines) > 0 {
		maxW = lay.Lines[0].Width
		if maxW < caretX {
			maxW = caretX
		}
	} else {
		maxW = b.txt.MeasureWidth(b.txt.Text)
	}
	maxScroll := maxW - visW + 4
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollX > maxScroll {
		scrollX = maxScroll
	}
	b.Viewport.SetScrollOffset(scrollX, 0)
	b.txt.SetViewportHint(scrollX, visW)
	b.syncHighlight()
	b.caretOn = true
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

func (b *ViewportInputBox) syncHighlight() {
	if b == nil || b.content == nil || b.txt == nil || b.ed == nil {
		return
	}
	for _, h := range b.highlights {
		if h != nil {
			b.content.RemoveChild(h)
		}
	}
	b.highlights = nil
	if b.ed.IsPassword() {
		return
	}
	sel := b.ed.SelectionRange()
	if sel.Collapsed() {
		return
	}
	sByte := byteOffsetForUtf16(b.ed.GetText(), sel.Start())
	eByte := byteOffsetForUtf16(b.ed.GetText(), sel.End())
	lay := b.txt.TextLayout()
	if lay == nil || len(lay.Lines) == 0 {
		return
	}
	boxes := lay.BoxesForRange(sByte, eByte)
	if len(boxes) == 0 {
		return
	}
	off := b.txt.Offset()
	for idx, r := range boxes {
		h := r.Size().Height
		pad := 0.0
		if idx == 0 {
			if m, ok := b.txt.Metrics(); ok {
				pad = (b.txt.LineHeight() - (m.Ascent + m.Descent)) / 2
				if pad < 1 {
					pad = 1
				} else if pad > 4 {
					pad = 4
				}
			} else {
				pad = h * 0.08
				if pad < 1 {
					pad = 1
				} else if pad > 3 {
					pad = 3
				}
			}
		}
		sr, sg, sb, sa := b.SelectionColor()
		hb := rendering.NewRenderColorBox(r.Size().Width, h+pad, sr, sg, sb, sa)
		hb.MoveTo(off.X+r.Min.X, off.Y+r.Min.Y-pad)
		b.content.AddChild(hb)
		b.highlights = append(b.highlights, hb)
	}
	b.content.RemoveChild(b.txt)
	b.content.RemoveChild(b.bar)
	b.content.AddChild(b.txt)
	b.content.AddChild(b.bar)
}

func (b *ViewportInputBox) layoutCaret() {
	if b == nil || b.bar == nil {
		return
	}
	if b.txt == nil || b.ed == nil {
		return
	}
	txtOff := b.txt.Offset()
	// 空文本也要显示光标：走 caretAnchor 的空分支，避免 GetOffsetForCaret 在 Lines==0 时直接 return
	if b.txt.Text == "" {
		_, top, bottom, ok := b.caretAnchor()
		if !ok {
			return
		}
		// bar 在 content 内，x 已含 pad（txtOff.X），直接用 content 坐标
		b.bar.MoveTo(txtOff.X-b.bar.Width/2, txtOff.Y)
		if h := bottom - top; h > 0 {
			b.bar.Height = h
		}
		if b.caretOn && b.focused {
			b.bar.SetAlpha(1)
		} else {
			b.bar.SetAlpha(0)
		}
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
		if _, top, bottom, ok2 := b.caretAnchor(); ok2 {
			b.bar.MoveTo(txtOff.X-b.bar.Width/2, txtOff.Y)
			if hh := bottom - top; hh > 0 {
				b.bar.Height = hh
			}
			if b.caretOn && b.focused {
				b.bar.SetAlpha(1)
			} else {
				b.bar.SetAlpha(0)
			}
		}
		return
	}
	b.bar.MoveTo(txtOff.X+x-b.bar.Width/2, txtOff.Y+y)
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
	if b == nil || b.ed == nil || b.Disabled() {
		return
	}
	abs := absoluteOrigin(b)
	vpOff := b.Viewport.ScrollOffset()
	txtOff := b.txt.Offset()
	pad := b.Padding()
	localX := ev.X - abs.X - 1 - pad + vpOff.X
	localY := ev.Y - abs.Y - 1 + vpOff.Y - txtOff.Y
	_ = pad
	switch ev.Kind {
	case input.PointerDown:
		if b.Node != nil {
			b.Node.RequestFocus()
		}
		lay := b.txt.TextLayout()
		var byteOff, aff int
		if lay != nil {
			byteOff, aff = lay.GetPositionForOffset(localX, localY)
		} else {
			byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
			aff = rendering.AffinityDownstream
		}
		now := time.Now()
		dx := localX - b.lastClickX
		if dx < 0 {
			dx = -dx
		}
		dy := localY - b.lastClickY
		if dy < 0 {
			dy = -dy
		}
		if now.Sub(b.lastClickAt) < 500*time.Millisecond && dx < 4 && dy < 4 {
			b.clickCount++
		} else {
			b.clickCount = 1
		}
		b.lastClickAt = now
		b.lastClickX = localX
		b.lastClickY = localY
		if b.clickCount == 2 {
			if !b.ed.SelectWordAt(byteOff) {
				b.ed.SetCaretWithAffinity(byteOff, aff)
			}
			b.dragging = false
		} else if b.clickCount >= 3 {
			if !b.ed.SelectLineAt(byteOff) {
				b.ed.SelectAll()
			}
			b.dragging = false
			b.clickCount = 3
		} else {
			b.ed.SetCaretWithAffinity(byteOff, aff)
			b.dragStart = b.ed.utf16ForByte(byteOff)
			b.dragging = true
			b.dragLastX = ev.X
			b.dragLastY = ev.Y
			b.startViewportAutoScroll()
		}
	case input.PointerMove:
		b.dragLastX = ev.X
		b.dragLastY = ev.Y
		if b.dragging {
			pad := b.Padding()
			abs2 := absoluteOrigin(b)
			w2 := b.FixedWidth
			visW := w2 - 2 - 2*pad
			if visW < 0 {
				visW = 0
			}
			scrollX := b.Viewport.ScrollOffset().X
			layTmp := b.txt.TextLayout()
			maxX := 0.0
			if layTmp != nil && len(layTmp.Lines) > 0 {
				maxX = layTmp.Lines[0].Width
			}
			maxScroll := maxX - visW + 4
			if maxScroll < 0 {
				maxScroll = 0
			}
			if ev.X < abs2.X+pad && scrollX > 0 {
				scrollX -= 28
				if scrollX < 0 {
					scrollX = 0
				}
				b.Viewport.SetScrollOffset(scrollX, 0)
				b.txt.SetViewportHint(scrollX, visW)
				localX = ev.X - abs2.X - 1 - pad + scrollX
			} else if ev.X > abs2.X+w2-pad && scrollX < maxScroll {
				scrollX += 28
				if scrollX > maxScroll {
					scrollX = maxScroll
				}
				b.Viewport.SetScrollOffset(scrollX, 0)
				b.txt.SetViewportHint(scrollX, visW)
				localX = ev.X - abs2.X - 1 - pad + scrollX
				if localX > maxX {
					localX = maxX
				}
			}
			lay := b.txt.TextLayout()
			var byteOff int
			if lay != nil {
				byteOff, _ = lay.GetPositionForOffset(localX, localY)
			} else {
				byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
			}
			cur := b.ed.utf16ForByte(byteOff)
			b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: cur})
		}
	case input.PointerUp:
		b.dragging = false
	}
}

func (b *ViewportInputBox) OnKey(ev input.KeyEvent) {
	if !ev.Pressed {
		return
	}
	if b.Disabled() {
		return
	}
	if b.ed != nil && b.ed.IsComposing() && isComposingFilterKey(ev.Key) {
		if ev.Key == input.KeyEnter {
			return
		}
		if ev.Key != input.KeyEnter {
			return
		}
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
			if b.clipboard != nil {
				if s, err := b.clipboard.Get("text/plain"); err == nil && s != "" {
					s = decodeUnicodeEscapes(s)
					b.ed.Paste(s)
				}
			}
			return
		case input.KeyZ:
			if ev.Mods.Shift {
				b.ed.Redo()
			} else {
				b.ed.Undo()
			}
			return
		case input.KeyY:
			b.ed.Redo()
			return
		case input.KeyArrowLeft:
			if ev.Mods.Shift {
				b.extendVisual(-1)
			} else {
				b.ed.MoveVisual(-1, b.txt.TextLayout())
			}
			return
		case input.KeyArrowRight:
			if ev.Mods.Shift {
				b.extendVisual(1)
			} else {
				b.ed.MoveVisual(1, b.txt.TextLayout())
			}
			return
		}
	}
	if ev.Mods.Shift {
		switch ev.Key {
		case input.KeyArrowLeft:
			b.extendVisual(-1)
			return
		case input.KeyArrowRight:
			b.extendVisual(1)
			return
		case input.KeyHome:
			b.extendToBoundary(false)
			return
		case input.KeyEnd:
			b.extendToBoundary(true)
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
	case input.KeyHome:
		b.ed.MoveCursorToBeginning()
	case input.KeyEnd:
		b.ed.MoveCursorToEnd()
	case input.KeyEscape:
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

func (b *ViewportInputBox) extendVisual(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	sel := b.ed.SelectionRange()
	base := sel.Base
	if !sel.Collapsed() {
		base = sel.Base
	} else {
		base = sel.Start()
	}
	cur := sel.Extent
	lay := b.txt.TextLayout()
	newOff := cur
	if lay != nil && len(lay.Lines) > 0 {
		curByte := byteOffsetForUtf16(b.ed.GetText(), cur)
		lineIdx, _, ok := lay.CaretForOffset(curByte)
		if ok {
			ln := lay.Lines[lineIdx]
			idx := -1
			for j, c := range ln.Carets {
				if c.ByteOff == curByte {
					idx = j
					break
				}
			}
			if idx >= 0 {
				if delta > 0 && idx+1 < len(ln.Carets) {
					newOff = b.ed.utf16ForByte(ln.Carets[idx+1].ByteOff)
				} else if delta < 0 && idx-1 >= 0 {
					newOff = b.ed.utf16ForByte(ln.Carets[idx-1].ByteOff)
				}
			}
		}
	} else {
		if delta > 0 {
			newOff = cur + 1
		} else {
			newOff = cur - 1
		}
	}
	b.ed.SetSelection(TextRange{Base: base, Extent: newOff})
}

func (b *ViewportInputBox) extendToBoundary(toEnd bool) {
	if b == nil || b.ed == nil {
		return
	}
	base := b.ed.SelectionRange().Base
	var off int
	if toEnd {
		off = b.ed.EditableRange().End()
	} else {
		off = b.ed.EditableRange().Start()
	}
	b.ed.SetSelection(TextRange{Base: base, Extent: off})
}

func (b *ViewportInputBox) MoveVisual(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	b.ed.MoveVisual(delta, b.txt.TextLayout())
}

func (b *ViewportInputBox) doViewportAutoScroll() {
	if b == nil || !b.dragging {
		b.autoScrollRunning = false
		return
	}
	pad := b.Padding()
	abs := absoluteOrigin(b)
	w := b.FixedWidth
	visW := w - 2 - 2*pad
	if visW < 0 {
		visW = 0
	}
	scrollX := b.Viewport.ScrollOffset().X
	layTmp := b.txt.TextLayout()
	maxX := 0.0
	if layTmp != nil && len(layTmp.Lines) > 0 {
		maxX = layTmp.Lines[0].Width
	}
	maxScroll := maxX - visW + 4
	if maxScroll < 0 {
		maxScroll = 0
	}
	did := false
	if b.dragLastX < abs.X+pad && scrollX > 0 {
		scrollX -= 28
		if scrollX < 0 {
			scrollX = 0
		}
		b.Viewport.SetScrollOffset(scrollX, 0)
		b.txt.SetViewportHint(scrollX, visW)
		did = true
	} else if b.dragLastX > abs.X+w-pad && scrollX < maxScroll {
		scrollX += 28
		if scrollX > maxScroll {
			scrollX = maxScroll
		}
		b.Viewport.SetScrollOffset(scrollX, 0)
		b.txt.SetViewportHint(scrollX, visW)
		did = true
	}
	if did {
		localX := b.dragLastX - abs.X - 1 - pad + scrollX
		localY := b.dragLastY - abs.Y - 1 + b.Viewport.ScrollOffset().Y - b.txt.Offset().Y
		lay := b.txt.TextLayout()
		var byteOff int
		if lay != nil {
			byteOff, _ = lay.GetPositionForOffset(localX, localY)
		} else {
			byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
		}
		cur := b.ed.utf16ForByte(byteOff)
		b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: cur})
	}
	if b.dragging {
		time.AfterFunc(50*time.Millisecond, func() { b.doViewportAutoScroll() })
	} else {
		b.autoScrollRunning = false
	}
}

func (b *ViewportInputBox) startViewportAutoScroll() {
	if b.autoScrollRunning {
		return
	}
	b.autoScrollRunning = true
	time.AfterFunc(50*time.Millisecond, func() { b.doViewportAutoScroll() })
}

func (b *ViewportInputBox) OnText(ev input.TextEvent) {}
func (b *ViewportInputBox) OnIME(ev input.IMEEvent)   {}

func viewportMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
