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

// InputBox is a single-line text field backed by Editor.
// It owns a RenderText + caret and handles focus, scroll, IME rect and
// pointer-to-caret mapping via the single-source TextLayout (Flutter-aligned).
type InputBox struct {
	*rendering.RenderBox
	*BaseEditable
	ed                     *Editor
	txt                    *rendering.RenderText
	bar                    *rendering.RenderColorBox
	clip                   *rendering.RenderClipRRect
	Node                   *focus.FocusNode
	sched                  func()
	focused                bool
	caretOn                bool
	scrollX                float64
	clipboard              platform.Clipboard
	selHas                 bool
	selR, selG, selB, selA float64
	padHas                 bool
	pad                    float64
	// R4: double/triple click and drag
	lastClickAt time.Time
	lastClickX  float64
	lastClickY  float64
	clickCount  int
	dragging    bool
	dragStart   int // utf16 offset at drag start
	highlights  []*rendering.RenderColorBox
	// 严格对齐 Flutter 闪烁：~500ms 周期，编辑/获焦后重置为常亮
	blinkElapsed      float64
	dragLastX         float64
	dragLastY         float64
	autoScrollRunning bool
}

func (b *InputBox) SetPlaceholder(s string) {
	if b == nil {
		return
	}
	if b.BaseEditable != nil {
		b.BaseEditable.SetPlaceholder(s)
	}
	if b.txt != nil && b.ed != nil && b.ed.GetText() == "" {
		b.sync()
	}
}
func (b *InputBox) Placeholder() string {
	if b == nil || b.BaseEditable == nil {
		return ""
	}
	return b.BaseEditable.Placeholder()
}

func (b *InputBox) SetDisabled(v bool) {
	if b == nil {
		return
	}
	if b.BaseEditable != nil {
		b.BaseEditable.SetDisabled(v)
	}
	if b.Node != nil {
		b.Node.Enabled = !v
		if v && b.focused {
			b.focused = false
			b.caretOn = false
			b.layoutCaret()
			if b.Node.HasFocus() {
				b.Node.Unfocus()
			}
		}
	}
	b.MarkNeedsPaint()
}
func (b *InputBox) Disabled() bool {
	if b == nil || b.BaseEditable == nil {
		return false
	}
	return b.BaseEditable.Disabled()
}

// SetSelectionColor 自定义选中高亮色（per-box），未设时走全局 SetDefaultSelectionColor / 引擎默认
func (b *InputBox) SetSelectionColor(r, g, b2, a float64) {
	if b == nil {
		return
	}
	b.selHas = true
	b.selR, b.selG, b.selB, b.selA = r, g, b2, a
	b.MarkNeedsPaint()
}
func (b *InputBox) ClearSelectionColor() {
	if b == nil {
		return
	}
	b.selHas = false
	b.MarkNeedsPaint()
}
func (b *InputBox) SelectionColor() (r, g, b2, a float64) {
	return resolveSelColor(b.selHas, b.selR, b.selG, b.selB, b.selA)
}
func (b *InputBox) SetPadding(pad float64) {
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
func (b *InputBox) ClearPadding() {
	if b == nil {
		return
	}
	b.padHas = false
	b.sync()
}
func (b *InputBox) Padding() float64 { return resolvePad(b.padHas, b.pad) }

// SetMaxLines 仅多行语义：单行忽略但保证 Ellipsis 不进 editable_range。
func (b *InputBox) SetMaxLines(n int) {
	// F-C5 单行显式忽略：单行不接受 MaxLines，保留 editable_range 完整
	return
}
func (b *InputBox) SetOverflow(o rendering.TextOverflow) {
	if b == nil || b.txt == nil {
		return
	}
	b.txt.SetOverflow(o)
}

func (b *InputBox) IsFocused() bool { return b != nil && b.focused }
func (b *InputBox) IsCaretOn() bool { return b != nil && b.caretOn }
func (b *InputBox) ScrollX() float64 {
	if b == nil {
		return 0
	}
	return b.scrollX
}
func (b *InputBox) ScrollY() float64 {
	if b == nil {
		return 0
	}
	return 0
}
func (b *InputBox) SetCaretOn(v bool) {
	if b == nil {
		return
	}
	b.caretOn = v
	b.blinkElapsed = 0
	b.layoutCaret()
	b.MarkNeedsPaint()
}

// TickCaret 按 Flutter 500ms 周期推进闪烁，获焦时编辑后已重置为常亮，需每帧调用
func (b *InputBox) TickCaret(dt float64) {
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

// SetObscuringCharacter 自定义密码掩码字符，对齐 Flutter TextField.obscuringCharacter（默认 '•'）
func (b *InputBox) SetObscuringCharacter(r rune) {
	if b == nil || b.ed == nil {
		return
	}
	b.ed.SetObscuringCharacter(r)
}

// ObscuringCharacter 返回当前掩码字符
func (b *InputBox) ObscuringCharacter() rune {
	if b == nil || b.ed == nil {
		return '•'
	}
	return b.ed.ObscuringCharacter()
}
func (b *InputBox) Sync() { b.sync() }

// NewInputBox creates a single-line box. fontSize <=0 defaults to 16.
func NewInputBox(ed *Editor, w, h, fontSize float64) *InputBox {
	if fontSize <= 0 {
		fontSize = 16
	}
	if ed != nil {
		ed.SetSingleLine(true)
	}
	inner := rendering.NewRenderBox()
	b := &InputBox{
		RenderBox:    inner,
		BaseEditable: NewBaseEditable(ed),
		ed:           ed,
		txt:          rendering.NewRenderText(""),
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
	b.clip = clip
	b.AddChild(clip)
	clip.AddChild(b.txt)
	b.bar = rendering.NewRenderColorBox(1.5, 22, 1.0, 0.85, 0.2, 1)
	clip.AddChild(b.bar)
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc != nil && pc.DC != nil {
			if b.disabled {
				pc.DC.SetRGBA(0.28, 0.30, 0.34, 1)
			} else if b.focused {
				pc.DC.SetRGBA(0.30, 0.58, 0.95, 1)
			} else {
				pc.DC.SetRGBA(0.38, 0.46, 0.56, 1)
			}
			pc.DC.SetLineWidth(1.4)
			pc.DC.DrawRectangle(pc.OriginX+0.7, pc.OriginY+0.7, size.Width-1.4, size.Height-1.4)
			_ = pc.DC.Stroke()
			if b.disabled {
				pc.DC.SetRGBA(0.18, 0.19, 0.21, 1)
			} else {
				pc.DC.SetRGBA(0.13, 0.15, 0.18, 1)
			}
			pc.DC.DrawRectangle(pc.OriginX+1, pc.OriginY+1, size.Width-2, size.Height-2)
			_ = pc.DC.Fill()
		}
		b.layoutCaret()
	}
	b.caretOn = true
	b.Node = focus.NewFocusNode(fmt.Sprintf("input-%.0f-%p", fontSize, b))
	b.Node.Target = b
	b.Node.OnFocusChange = func(on bool) {
		if b.disabled {
			return
		}
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
func (b *InputBox) Clipboard() platform.Clipboard     { return b.clipboard }

// SetSchedule sets the frame scheduler (PipelineApp.ScheduleFrame).
func (b *InputBox) SetSchedule(fn func()) { b.sched = fn }

// FocusNode returns the focus node for registration.
func (b *InputBox) FocusNode() *focus.FocusNode { return b.Node }

// Editor returns the backing editor.
func (b *InputBox) Editor() *Editor { return b.ed }
func (b *InputBox) ContentPurpose() platform.ContentPurpose {
	if b != nil && b.BaseEditable != nil {
		return b.BaseEditable.ContentType().Purpose
	}
	return platform.PurposeNormal
}
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
	// 空串获焦：TextLayout 可能 0 行，仍需在 padding 8 处给 caret
	if b.txt.Text == "" {
		off := b.txt.Offset()
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		// 单行居中时 off.Y 已是 textY，空串也用它
		return off.X, off.Y, off.Y + lh, true
	}
	// 密码框：光标按掩码串的缝表，不走原文 byte（掩码字符可自定义，对齐 Flutter obscuringCharacter）
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
				off := b.txt.Offset()
				return off.X + x, off.Y + y, off.Y + y + h, true
			}
		}
		// fallback: estimate by runeIdx * charW
		off := b.txt.Offset()
		charW := b.txt.MeasureWidth(string(ch))
		if charW <= 0 {
			charW = 12
		}
		x := off.X + float64(runeIdx)*charW
		top := off.Y
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
	// F-D3: composing 时必报 composing_rect（Flutter firstRectForCharacterRange），
	// 非 composing 仅预热 caret 矩形。composing_rect 取 composingRange 的 BoxesForRange 并集。
	if b != nil && b.ed != nil && b.ed.IsComposing() {
		if lay := b.txt.TextLayout(); lay != nil && len(lay.Lines) > 0 {
			cr := b.ed.ComposingRange()
			// ComposingRange 是 UTF16，需转 byte 再取盒
			s := byteOffsetForUtf16(b.ed.GetText(), cr.Start())
			e := byteOffsetForUtf16(b.ed.GetText(), cr.End())
			if s < e {
				if boxes := lay.BoxesForRange(s, e); len(boxes) > 0 {
					// 并集
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
					abs := absoluteOrigin(b)
					return platform.Rect{X: abs.X + off.X + minX, Y: abs.Y + off.Y + minY, W: maxX - minX, H: maxY - minY}
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

func (b *InputBox) sync() {
	if b == nil || b.txt == nil || b.ed == nil {
		return
	}
	// F-E0c 密码掩码：显示 obscuringCharacter（可自定义，对齐 Flutter TextField.obscuringCharacter，默认 '•'），保持与 rune 数一致
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
		b.scrollX = 0
	}
	// 密码模式下光标按 rune 索引映射到掩码串
	curByte := b.ed.GetCursorOffset()
	if isPassword {
		// ed 的 byte 转 rune 索引，再转掩码 byte
		runes := []rune(b.ed.GetText())
		runeIdx := 0
		for i := range b.ed.GetText()[:min(curByte, len(b.ed.GetText()))] {
			if (b.ed.GetText()[i] & 0xC0) != 0x80 {
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
	lh := b.txt.LineHeight()
	if lh <= 0 {
		lh = 22
	}
	textY := (b.FixedHeight - lh) / 2
	if textY < 0 {
		textY = 0
	}
	pad := b.Padding()
	visW := b.FixedWidth - 2*pad
	if visW < 0 {
		visW = 0
	}
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
		// 对齐 Flutter RenderEditable ensureCaretVisible + 溢出回滚：
		// 删除后总宽变小，scrollX 若仍停在旧 max 会在右侧留白、前面字不回移。
		// 按实际行宽计算 maxScroll 并夹紧，自动适配任意字号/字体。
		maxW := lay.Lines[0].Width
		if maxW < caretX {
			maxW = caretX
		}
		maxScroll := maxW - visW + 4
		if maxScroll < 0 {
			maxScroll = 0
		}
		if b.scrollX > maxScroll {
			b.scrollX = maxScroll
		}
		b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: textY})
	} else {
		b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: textY})
	}
	b.txt.SetViewportHint(b.scrollX, visW)
	b.syncSelectionHighlight()
	// 严格对齐 Flutter：编辑/同步后光标立即可见且闪烁计时重置为 0
	b.caretOn = true
	b.blinkElapsed = 0
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

// syncSelectionHighlight draws F-B4 selection using BoxesForRange (TextLayout single source).
func (b *InputBox) syncSelectionHighlight() {
	if b == nil || b.clip == nil || b.txt == nil || b.ed == nil {
		return
	}
	// clear old
	for _, h := range b.highlights {
		if h != nil {
			b.clip.RemoveChild(h)
		}
	}
	b.highlights = nil
	if b.ed.IsPassword() {
		return
	}
	selRange := b.ed.SelectionRange()
	if selRange.Collapsed() {
		return
	}
	// Convert utf16 range to byte offsets
	sByte := byteOffsetForUtf16(b.ed.GetText(), selRange.Start())
	eByte := byteOffsetForUtf16(b.ed.GetText(), selRange.End())
	// For password we already returned; for normal, map to display bytes (same as text)
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
		// 只给首行上面多留一点，下面不动；pad 按字体度量自动算，避免小字过大、大字过小
		// 多行时只有第一段加 pad，其余段保持行盒，避免上下两段重叠变深（对齐 Flutter 选区）
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
		b.clip.AddChild(hb)
		b.highlights = append(b.highlights, hb)
	}
	b.clip.RemoveChild(b.txt)
	b.clip.RemoveChild(b.bar)
	b.clip.AddChild(b.txt)
	b.clip.AddChild(b.bar)
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
	if b == nil || b.ed == nil || b.disabled {
		return
	}
	abs := absoluteOrigin(b)
	off := b.txt.Offset()
	localX := ev.X - abs.X - off.X
	localY := ev.Y - abs.Y - off.Y
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
		// 密码框：掩码 byte( runeIdx*chBytes ) 转原文 utf16（掩码字符可自定义）
		if b.ed.IsPassword() {
			ch := b.ed.ObscuringCharacter()
			chBytes := len(string(ch))
			if chBytes <= 0 {
				chBytes = 3
			}
			runeIdx := byteOff / chBytes
			if runeIdx < 0 {
				runeIdx = 0
			}
			if runeIdx > len([]rune(b.ed.GetText())) {
				runeIdx = len([]rune(b.ed.GetText()))
			}
			byteOff = runeIdx // dummy, will map via runeIdx
			// 单击直接按 runeIdx 设光标
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
			if b.clickCount >= 2 {
				b.ed.SelectAll()
				b.dragging = false
				if b.clickCount > 3 {
					b.clickCount = 3
				}
			} else {
				utf16Off := 0
				for _, r := range []rune(b.ed.GetText())[:runeIdx] {
					if r > 0xFFFF {
						utf16Off += 2
					} else {
						utf16Off++
					}
				}
				b.ed.SetSelection(TextRange{Base: utf16Off, Extent: utf16Off})
				b.dragStart = utf16Off
				b.dragging = true
				b.dragLastX = ev.X
				b.dragLastY = ev.Y
				b.startInputBoxAutoScroll()
			}
			return
		}
		now := time.Now()
		dx := localX - b.lastClickX
		dy := localY - b.lastClickY
		if dx < 0 {
			dx = -dx
		}
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
			// F-B3 三击选段：整行或全选
			if !b.ed.SelectLineAt(byteOff) {
				b.ed.SelectAll()
			}
			b.dragging = false
			b.clickCount = 3 // clamp
		} else {
			b.ed.SetCaretWithAffinity(byteOff, aff)
			b.dragStart = b.ed.utf16ForByte(byteOff)
			b.dragging = true
			b.dragLastX = ev.X
			b.dragLastY = ev.Y
			b.startInputBoxAutoScroll()
		}
	case input.PointerMove:
		b.dragLastX = ev.X
		b.dragLastY = ev.Y
		if b.dragging {
			pad := b.Padding()
			abs2 := absoluteOrigin(b)
			w2 := b.FixedWidth
			visW := w2 - 2*pad
			if visW < 0 {
				visW = 0
			}
			layTmp := b.txt.TextLayout()
			maxX := 0.0
			if layTmp != nil && len(layTmp.Lines) > 0 {
				maxX = layTmp.Lines[0].Width
			}
			maxScroll := maxX - visW + 4
			if maxScroll < 0 {
				maxScroll = 0
			}
			if ev.X < abs2.X+pad && b.scrollX > 0 {
				b.scrollX -= 28
				if b.scrollX < 0 {
					b.scrollX = 0
				}
				b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: b.txt.Offset().Y})
				b.txt.SetViewportHint(b.scrollX, visW)
				localX = ev.X - abs2.X - b.txt.Offset().X
			} else if ev.X > abs2.X+w2-pad && b.scrollX < maxScroll {
				b.scrollX += 28
				if b.scrollX > maxScroll {
					b.scrollX = maxScroll
				}
				b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: b.txt.Offset().Y})
				b.txt.SetViewportHint(b.scrollX, visW)
				localX = ev.X - abs2.X - b.txt.Offset().X
				if localX > maxX {
					localX = maxX
				}
			}
			if b.ed.IsPassword() {
				ch := b.ed.ObscuringCharacter()
				chBytes := len(string(ch))
				if chBytes <= 0 {
					chBytes = 3
				}
				lay := b.txt.TextLayout()
				var byteOff int
				if lay != nil {
					byteOff, _ = lay.GetPositionForOffset(localX, localY)
				} else {
					byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
				}
				runeIdx := byteOff / chBytes
				if runeIdx < 0 {
					runeIdx = 0
				}
				runes := []rune(b.ed.GetText())
				if runeIdx > len(runes) {
					runeIdx = len(runes)
				}
				cur := 0
				for _, r := range runes[:runeIdx] {
					if r > 0xFFFF {
						cur += 2
					} else {
						cur++
					}
				}
				b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: cur})
			} else {
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
		}
	case input.PointerUp:
		b.dragging = false
	}
}

func isComposingFilterKey(k input.Key) bool {
	switch k {
	case input.KeyHome, input.KeyEnd, input.KeyPageUp, input.KeyPageDown,
		input.KeyArrowLeft, input.KeyArrowRight, input.KeyArrowUp, input.KeyArrowDown,
		input.KeyEnter:
		return true
	}
	return false
}
func (b *InputBox) OnKey(ev input.KeyEvent) {
	if !ev.Pressed {
		return
	}
	if b.Disabled() {
		return
	}
	// F-D3/filter_keypress: composing 时 Home/End/Page/Arrow/Enter 由 IME 优先消费，避免光标在 composingRange 外
	if b.ed != nil && b.ed.IsComposing() && isComposingFilterKey(ev.Key) {
		if ev.Key == input.KeyEnter && b.ed.IsComposing() {
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
		case input.KeyC:
			s := b.ed.Copy()
			if s != "" {
				_ = effectiveClipboard(b.clipboard).Set("text/plain", s)
			}
			return
		case input.KeyX:
			s := b.ed.Cut()
			if s != "" {
				_ = effectiveClipboard(b.clipboard).Set("text/plain", s)
			}
			return
		case input.KeyV:
			if s, err := effectiveClipboard(b.clipboard).Get("text/plain"); err == nil && s != "" {
				s = decodeUnicodeEscapes(s)
				b.ed.Paste(s)
			}
			return
		case input.KeyBackspace:
			// F-C3 词删除 Ctrl+Backspace 限 editable_range
			b.deleteWord(false)
			return
		case input.KeyDelete:
			b.deleteWord(true)
			return
		case input.KeyArrowLeft:
			b.moveWord(false, ev.Mods.Shift)
			return
		case input.KeyArrowRight:
			b.moveWord(true, ev.Mods.Shift)
			return
		}
	}
	// Shift+方向扩展选区
	if ev.Mods.Shift {
		switch ev.Key {
		case input.KeyArrowLeft:
			b.extendVisual(-1)
			return
		case input.KeyArrowRight:
			b.extendVisual(1)
			return
		case input.KeyArrowUp:
			b.extendVertical(-1)
			return
		case input.KeyArrowDown:
			b.extendVertical(1)
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
		b.moveVisual(-1)
	case input.KeyArrowRight:
		b.moveVisual(1)
	case input.KeyArrowUp:
		b.ed.MoveCursorUp()
	case input.KeyArrowDown:
		b.ed.MoveCursorDown()
	case input.KeyHome:
		b.ed.MoveCursorToBeginning()
	case input.KeyEnd:
		b.ed.MoveCursorToEnd()
	case input.KeyEscape:
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

func (b *InputBox) extendVisual(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	// 密码框：按 rune 单步，不走缝表
	if b.ed.IsPassword() {
		sel := b.ed.SelectionRange()
		base := sel.Base
		if sel.Collapsed() {
			base = sel.Start()
		}
		cur := sel.Extent
		if delta > 0 {
			if cur < utf16Len(b.ed.GetText()) {
				// 按簇：surrogate 2
				bRunes := []rune(b.ed.GetText())
				runeIdx := utf16ToRuneIndex(b.ed.GetText(), cur)
				if runeIdx < len(bRunes) && bRunes[runeIdx] > 0xFFFF {
					cur += 2
				} else {
					cur += 1
				}
			}
		} else {
			if cur > 0 {
				bRunes := []rune(b.ed.GetText())
				runeIdx := utf16ToRuneIndex(b.ed.GetText(), cur)
				if runeIdx > 0 && bRunes[runeIdx-1] > 0xFFFF {
					cur -= 2
				} else {
					cur -= 1
				}
			}
		}
		b.ed.SetSelection(TextRange{Base: base, Extent: cur})
		return
	}
	sel := b.ed.SelectionRange()
	var base int
	if sel.Collapsed() {
		base = sel.Start()
	} else {
		// 已经有选区，锚点取 Base 侧
		base = sel.Base
	}
	// 计算新 Extent
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
				} else if delta > 0 && lineIdx+1 < len(lay.Lines) {
					newOff = b.ed.utf16ForByte(lay.Lines[lineIdx+1].Carets[0].ByteOff)
				} else if delta < 0 && lineIdx-1 >= 0 {
					prev := lay.Lines[lineIdx-1]
					newOff = b.ed.utf16ForByte(prev.Carets[len(prev.Carets)-1].ByteOff)
				}
			} else {
				// fallback
				if delta > 0 {
					newOff = cur + 1
				} else {
					newOff = cur - 1
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

func (b *InputBox) extendVertical(dir int) {
	if b == nil || b.ed == nil {
		return
	}
	sel := b.ed.SelectionRange()
	base := sel.Base
	lay := b.txt.TextLayout()
	curByte := byteOffsetForUtf16(b.ed.GetText(), sel.Extent)
	lineIdx, _, ok := lay.CaretForOffset(curByte)
	if !ok {
		if dir < 0 {
			b.ed.MoveCursorUp()
		} else {
			b.ed.MoveCursorDown()
		}
		return
	}
	target := lineIdx + dir
	if target < 0 || target >= len(lay.Lines) {
		return
	}
	// 粘滞列
	curX, _, _, _ := lay.GetOffsetForCaret(curByte, rendering.AffinityDownstream, 1.5)
	tln := lay.Lines[target]
	best := 0
	bestDist := 1e12
	for i, c := range tln.Carets {
		d := c.X - curX
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	newOff := b.ed.utf16ForByte(tln.Carets[best].ByteOff)
	b.ed.SetSelection(TextRange{Base: base, Extent: newOff})
}

func (b *InputBox) extendToBoundary(toEnd bool) {
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

func (b *InputBox) moveWord(forward, extend bool) {
	if b == nil || b.ed == nil {
		return
	}
	if extend {
		base := b.ed.SelectionRange().Base
		cur := b.ed.SelectionRange().Extent
		// 临时移动再取新 Extent
		_ = cur
		b.ed.MoveCursorByWord(forward)
		newOff := b.ed.SelectionRange().Start()
		// 恢复 anchor
		b.ed.SetSelection(TextRange{Base: base, Extent: newOff})
	} else {
		b.ed.MoveCursorByWord(forward)
	}
}

func (b *InputBox) deleteWord(forward bool) {
	if b == nil || b.ed == nil {
		return
	}
	if !b.ed.SelectionRange().Collapsed() {
		b.ed.DeleteSelected()
		return
	}
	cur := b.ed.SelectionRange().Start()
	er := b.ed.EditableRange()
	// 找词边界：沿用 Editor 的词划分
	start := cur
	end := cur
	if forward {
		// 删后词
		dup := textinputDup(b.ed)
		dup.MoveCursorByWord(true)
		end = dup.SelectionRange().Start()
		if end > er.End() {
			end = er.End()
		}
		if end > start {
			b.ed.SetSelection(TextRange{Base: start, Extent: end})
			b.ed.DeleteSelected()
		}
	} else {
		dup := textinputDup(b.ed)
		dup.MoveCursorByWord(false)
		start = dup.SelectionRange().Start()
		if start < er.Start() {
			start = er.Start()
		}
		if start < cur {
			b.ed.SetSelection(TextRange{Base: start, Extent: cur})
			b.ed.DeleteSelected()
		}
	}
}

// textinputDup creates a shallow copy of Editor for word boundary calculation without mutating original.
func textinputDup(e *Editor) *Editor {
	if e == nil {
		return nil
	}
	c := *e
	return &c
}

func (b *InputBox) moveVisual(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	if b.ed.IsPassword() {
		if delta > 0 {
			b.ed.MoveCursorForward()
		} else {
			b.ed.MoveCursorBack()
		}
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

func (b *InputBox) doInputBoxAutoScroll() {
	if b == nil || !b.dragging {
		b.autoScrollRunning = false
		return
	}
	pad := b.Padding()
	abs := absoluteOrigin(b)
	w := b.FixedWidth
	visW := w - 2*pad
	if visW < 0 {
		visW = 0
	}
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
	if b.dragLastX < abs.X+pad && b.scrollX > 0 {
		b.scrollX -= 28
		if b.scrollX < 0 {
			b.scrollX = 0
		}
		b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: b.txt.Offset().Y})
		b.txt.SetViewportHint(b.scrollX, visW)
		did = true
	} else if b.dragLastX > abs.X+w-pad && b.scrollX < maxScroll {
		b.scrollX += 28
		if b.scrollX > maxScroll {
			b.scrollX = maxScroll
		}
		b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: b.txt.Offset().Y})
		b.txt.SetViewportHint(b.scrollX, visW)
		did = true
	}
	if did {
		localX := b.dragLastX - abs.X - b.txt.Offset().X
		localY := b.dragLastY - abs.Y - b.txt.Offset().Y
		var byteOff int
		if layTmp != nil {
			byteOff, _ = layTmp.GetPositionForOffset(localX, localY)
		} else {
			byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
		}
		if b.ed.IsPassword() {
			ch := b.ed.ObscuringCharacter()
			chBytes := len(string(ch))
			if chBytes <= 0 {
				chBytes = 3
			}
			runeIdx := byteOff / chBytes
			runes := []rune(b.ed.GetText())
			if runeIdx < 0 {
				runeIdx = 0
			}
			if runeIdx > len(runes) {
				runeIdx = len(runes)
			}
			cur := 0
			for _, r := range runes[:runeIdx] {
				if r > 0xFFFF {
					cur += 2
				} else {
					cur++
				}
			}
			b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: cur})
		} else {
			cur := b.ed.utf16ForByte(byteOff)
			b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: cur})
		}
	}
	if b.dragging {
		time.AfterFunc(50*time.Millisecond, func() { b.doInputBoxAutoScroll() })
	} else {
		b.autoScrollRunning = false
	}
}

func (b *InputBox) startInputBoxAutoScroll() {
	if b.autoScrollRunning {
		return
	}
	b.autoScrollRunning = true
	time.AfterFunc(50*time.Millisecond, func() { b.doInputBoxAutoScroll() })
}

func (b *InputBox) OnText(ev input.TextEvent) {
	if b == nil || b.Disabled() {
		return
	}
}
func (b *InputBox) OnIME(ev input.IMEEvent) {
	if b == nil || b.Disabled() {
		return
	}
}

// MultiLineInputBox is a wrapping editor. It scrolls both axes.
type MultiLineInputBox struct {
	*rendering.RenderBox
	*BaseEditable
	ed                     *Editor
	txt                    *rendering.RenderText
	bar                    *rendering.RenderColorBox
	clip                   *rendering.RenderClipRRect
	Node                   *focus.FocusNode
	sched                  func()
	focused                bool
	caretOn                bool
	scrollX                float64
	scrollY                float64
	clipboard              platform.Clipboard
	selHas                 bool
	selR, selG, selB, selA float64
	padHas                 bool
	pad                    float64
	lastClickAt            time.Time
	lastClickX             float64
	lastClickY             float64
	clickCount             int
	dragging               bool
	dragStart              int
	highlights             []*rendering.RenderColorBox
	blinkElapsed           float64
	dragLastX              float64
	dragLastY              float64
	autoScrollRunning      bool
	wrap                   bool
	wrapMode               text.WrapMode
}

func (b *MultiLineInputBox) SetPadding(pad float64) {
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
func (b *MultiLineInputBox) ClearPadding() {
	if b == nil {
		return
	}
	b.padHas = false
	b.sync()
}
func (b *MultiLineInputBox) Padding() float64 { return resolvePad(b.padHas, b.pad) }

func (b *MultiLineInputBox) SetPlaceholder(s string) {
	if b == nil || b.BaseEditable == nil {
		return
	}
	b.BaseEditable.SetPlaceholder(s)
	if b.txt != nil && b.ed != nil && b.ed.GetText() == "" {
		b.sync()
	}
}
func (b *MultiLineInputBox) Placeholder() string {
	if b == nil || b.BaseEditable == nil {
		return ""
	}
	return b.BaseEditable.Placeholder()
}

func (b *MultiLineInputBox) IsFocused() bool { return b != nil && b.focused }
func (b *MultiLineInputBox) IsCaretOn() bool { return b != nil && b.caretOn }
func (b *MultiLineInputBox) SetDisabled(v bool) {
	if b == nil || b.BaseEditable == nil {
		return
	}
	b.BaseEditable.SetDisabled(v)
	if b.Node != nil {
		b.Node.Enabled = !v
		if v && b.focused {
			b.focused = false
			b.caretOn = false
			b.layoutCaret()
			if b.Node.HasFocus() {
				b.Node.Unfocus()
			}
		}
	}
	b.MarkNeedsPaint()
}
func (b *MultiLineInputBox) Disabled() bool {
	if b == nil || b.BaseEditable == nil {
		return false
	}
	return b.BaseEditable.Disabled()
}
func (b *MultiLineInputBox) SetSelectionColor(r, g, b2, a float64) {
	if b == nil {
		return
	}
	b.selHas = true
	b.selR, b.selG, b.selB, b.selA = r, g, b2, a
	b.MarkNeedsPaint()
}
func (b *MultiLineInputBox) ClearSelectionColor() {
	if b == nil {
		return
	}
	b.selHas = false
	b.MarkNeedsPaint()
}
func (b *MultiLineInputBox) SelectionColor() (r, g, b2, a float64) {
	return resolveSelColor(b.selHas, b.selR, b.selG, b.selB, b.selA)
}
func (b *MultiLineInputBox) SetMaxLines(n int) {
	if b == nil || b.txt == nil {
		return
	}
	b.txt.SetMaxLines(n)
	b.sync()
}
func (b *MultiLineInputBox) SetOverflow(o rendering.TextOverflow) {
	if b == nil || b.txt == nil {
		return
	}
	b.txt.SetOverflow(o)
	b.sync()
}

// SetWrap 控制多行是否自动换行，默认 false（不换行，超出宽度水平滚动）。
// 对齐 Flutter softWrap：true 时按 MaxWidth 软换行，false 时仅硬换行 \n。
func (b *MultiLineInputBox) SetWrap(v bool) {
	if b == nil {
		return
	}
	b.wrap = v
	b.sync()
}
func (b *MultiLineInputBox) Wrap() bool {
	if b == nil {
		return false
	}
	return b.wrap
}

// SetWrapMode 控制换行时的断行策略，默认 WrapWordChar（词优先+字符兜底，对齐 Flutter）。
func (b *MultiLineInputBox) SetWrapMode(m text.WrapMode) {
	if b == nil {
		return
	}
	b.wrapMode = m
	b.sync()
}
func (b *MultiLineInputBox) WrapMode() text.WrapMode {
	if b == nil {
		return text.WrapWordChar
	}
	if b.wrapMode == text.WrapNone && !b.wrap {
		return text.WrapNone
	}
	if b.wrapMode != 0 {
		return b.wrapMode
	}
	return text.WrapWordChar
}
func (b *MultiLineInputBox) ScrollX() float64 {
	if b == nil {
		return 0
	}
	return b.scrollX
}
func (b *MultiLineInputBox) ScrollY() float64 {
	if b == nil {
		return 0
	}
	return b.scrollY
}
func (b *MultiLineInputBox) SetCaretOn(v bool) {
	if b == nil {
		return
	}
	b.caretOn = v
	b.blinkElapsed = 0
	b.layoutCaret()
	b.MarkNeedsPaint()
}

func (b *MultiLineInputBox) TickCaret(dt float64) {
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
func (b *MultiLineInputBox) Sync() { b.sync() }

func NewMultiLineInputBox(ed *Editor, w, h, fontSize float64) *MultiLineInputBox {
	if fontSize <= 0 {
		fontSize = 14
	}
	if ed != nil {
		ed.SetSingleLine(false)
	}
	inner := rendering.NewRenderBox()
	b := &MultiLineInputBox{
		RenderBox:    inner,
		BaseEditable: NewBaseEditable(ed),
		ed:           ed,
		txt:          rendering.NewRenderText(""),
	}
	inner.Init(b)
	inner.SetRelayoutBoundary(true)
	inner.SetRepaintBoundary(true)
	b.txt.FontSize = fontSize
	b.txt.R, b.txt.G, b.txt.B, b.txt.A = 0.06, 0.85, 0.60, 1
	b.FixedWidth = w
	b.FixedHeight = h
	// 默认不自动换行（满足 R4 测试要求），需换行时显式 SetWrap(true)
	b.wrap = false
	b.wrapMode = text.WrapWordChar
	b.txt.MaxWidth = 0
	clip := rendering.NewRenderClipRRect()
	clip.FixedWidth = w
	clip.FixedHeight = h
	clip.SetRadius(3)
	b.clip = clip
	b.AddChild(clip)
	clip.AddChild(b.txt)
	b.bar = rendering.NewRenderColorBox(1.5, 22, 1.0, 0.85, 0.2, 1)
	clip.AddChild(b.bar)
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc != nil && pc.DC != nil {
			if b.Disabled() {
				pc.DC.SetRGBA(0.28, 0.30, 0.34, 1)
			} else if b.focused {
				pc.DC.SetRGBA(0.30, 0.58, 0.95, 1)
			} else {
				pc.DC.SetRGBA(0.38, 0.46, 0.56, 1)
			}
			pc.DC.SetLineWidth(1.4)
			pc.DC.DrawRectangle(pc.OriginX+0.7, pc.OriginY+0.7, size.Width-1.4, size.Height-1.4)
			_ = pc.DC.Stroke()
			if b.Disabled() {
				pc.DC.SetRGBA(0.18, 0.19, 0.21, 1)
			} else {
				pc.DC.SetRGBA(0.13, 0.15, 0.18, 1)
			}
			pc.DC.DrawRectangle(pc.OriginX+1, pc.OriginY+1, size.Width-2, size.Height-2)
			_ = pc.DC.Fill()
		}
		b.layoutCaret()
	}
	b.caretOn = true
	b.Node = focus.NewFocusNode(fmt.Sprintf("multi-%p", b))
	b.Node.Target = b
	b.Node.OnFocusChange = func(on bool) {
		if b.Disabled() && on {
			return
		}
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
	if b != nil && b.BaseEditable != nil {
		return b.BaseEditable.ContentType().Purpose
	}
	return platform.PurposeNormal
}
func (b *MultiLineInputBox) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: b.ContentPurpose()}
}

func (b *MultiLineInputBox) caretAnchor() (float64, float64, float64, bool) {
	if b == nil || b.txt == nil || b.ed == nil {
		return 0, 0, 0, false
	}
	if b.txt.Text == "" {
		off := b.txt.Offset()
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		return off.X, off.Y, off.Y + lh, true
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
					abs := absoluteOrigin(b)
					return platform.Rect{X: abs.X + off.X + minX, Y: abs.Y + off.Y + minY, W: maxX - minX, H: maxY - minY}
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

func (b *MultiLineInputBox) sync() {
	if b == nil || b.txt == nil || b.ed == nil {
		return
	}
	disp := b.ed.GetText()
	if disp == "" && !b.focused && b.placeholder != "" {
		disp = b.placeholder
	}
	pad := b.Padding()
	if b.wrap {
		b.txt.MaxWidth = b.FixedWidth - 2*pad
		if b.txt.MaxWidth < 0 {
			b.txt.MaxWidth = 0
		}
	} else {
		b.txt.MaxWidth = 0
	}
	b.txt.SetText(disp)
	if disp == "" {
		b.scrollX = 0
		b.scrollY = 0
	}
	// delayed highlight handled after layout
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
	visW := b.FixedWidth - 2*pad
	visH := b.FixedHeight - 2*pad
	if visW < 0 {
		visW = 0
	}
	if visH < 0 {
		visH = 0
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
	// 横向 maxScroll 夹紧：删除后总宽变小，前面文本自动回移
	if lay != nil && len(lay.Lines) > 0 {
		maxW := 0.0
		for _, ln := range lay.Lines {
			if ln.Width > maxW {
				maxW = ln.Width
			}
		}
		if maxW < caretX {
			maxW = caretX
		}
		maxScrollX := maxW - visW + 4
		if maxScrollX < 0 {
			maxScrollX = 0
		}
		if b.scrollX > maxScrollX {
			b.scrollX = maxScrollX
		}
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
	b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: pad - b.scrollY})
	b.syncMultiHighlight()
	b.caretOn = true
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

func (b *MultiLineInputBox) syncMultiHighlight() {
	if b == nil || b.clip == nil || b.txt == nil || b.ed == nil {
		return
	}
	for _, h := range b.highlights {
		if h != nil {
			b.clip.RemoveChild(h)
		}
	}
	b.highlights = nil
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
		b.clip.AddChild(hb)
		b.highlights = append(b.highlights, hb)
	}
	b.clip.RemoveChild(b.txt)
	b.clip.RemoveChild(b.bar)
	b.clip.AddChild(b.txt)
	b.clip.AddChild(b.bar)
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

func (b *MultiLineInputBox) doMultiAutoScroll() {
	if b == nil || !b.dragging {
		b.autoScrollRunning = false
		return
	}
	pad := b.Padding()
	abs := absoluteOrigin(b)
	w, h := b.FixedWidth, b.FixedHeight
	visW := w - 2*pad
	visH := h - 2*pad
	if visW < 0 {
		visW = 0
	}
	if visH < 0 {
		visH = 0
	}
	lay := b.txt.TextLayout()
	maxX, maxY := 0.0, 0.0
	if lay != nil && len(lay.Lines) > 0 {
		for _, ln := range lay.Lines {
			if ln.Width > maxX {
				maxX = ln.Width
			}
		}
		totalH := lay.LineTop(len(lay.Lines)-1) + lay.LineHeight(len(lay.Lines)-1)
		maxY = totalH - visH
		if maxY < 0 {
			maxY = 0
		}
	}
	maxScrollX := maxX - visW + 4
	if maxScrollX < 0 {
		maxScrollX = 0
	}
	did := false
	if b.dragLastX < abs.X+pad && b.scrollX > 0 {
		b.scrollX -= 28
		if b.scrollX < 0 {
			b.scrollX = 0
		}
		did = true
	} else if b.dragLastX > abs.X+w-pad && b.scrollX < maxScrollX {
		b.scrollX += 28
		if b.scrollX > maxScrollX {
			b.scrollX = maxScrollX
		}
		did = true
	}
	if b.dragLastY < abs.Y+pad && b.scrollY > 0 {
		b.scrollY -= 14
		if b.scrollY < 0 {
			b.scrollY = 0
		}
		did = true
	} else if b.dragLastY > abs.Y+h-pad && b.scrollY < maxY {
		b.scrollY += 14
		if b.scrollY > maxY {
			b.scrollY = maxY
		}
		did = true
	}
	if did {
		b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: pad - b.scrollY})
		localX := b.dragLastX - abs.X - b.txt.Offset().X
		localY := b.dragLastY - abs.Y - b.txt.Offset().Y
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
		time.AfterFunc(50*time.Millisecond, func() { b.doMultiAutoScroll() })
	} else {
		b.autoScrollRunning = false
	}
}

func (b *MultiLineInputBox) startMultiAutoScroll() {
	if b.autoScrollRunning {
		return
	}
	b.autoScrollRunning = true
	time.AfterFunc(50*time.Millisecond, func() { b.doMultiAutoScroll() })
}

func (b *MultiLineInputBox) OnPointer(ev input.PointerEvent) {
	if b == nil || b.ed == nil || b.Disabled() {
		return
	}
	abs := absoluteOrigin(b)
	off := b.txt.Offset()
	localX := ev.X - abs.X - off.X
	localY := ev.Y - abs.Y - off.Y
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
		dy := localY - b.lastClickY
		if dx < 0 {
			dx = -dx
		}
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
			b.startMultiAutoScroll()
		}
	case input.PointerMove:
		b.dragLastX = ev.X
		b.dragLastY = ev.Y
		if b.dragging {
			pad := b.Padding()
			abs2 := absoluteOrigin(b)
			w2, h2 := b.FixedWidth, b.FixedHeight
			visW := w2 - 2*pad
			visH := h2 - 2*pad
			if visW < 0 {
				visW = 0
			}
			if visH < 0 {
				visH = 0
			}
			layTmp := b.txt.TextLayout()
			maxX, maxY := 0.0, 0.0
			if layTmp != nil && len(layTmp.Lines) > 0 {
				for _, ln := range layTmp.Lines {
					if ln.Width > maxX {
						maxX = ln.Width
					}
				}
				totalH := layTmp.LineTop(len(layTmp.Lines)-1) + layTmp.LineHeight(len(layTmp.Lines)-1)
				maxY = totalH - visH
				if maxY < 0 {
					maxY = 0
				}
			}
			maxScrollX := maxX - visW + 4
			if maxScrollX < 0 {
				maxScrollX = 0
			}
			if ev.X < abs2.X+pad && b.scrollX > 0 {
				b.scrollX -= 28
				if b.scrollX < 0 {
					b.scrollX = 0
				}
				b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: pad - b.scrollY})
				localX = ev.X - abs2.X - b.txt.Offset().X
			} else if ev.X > abs2.X+w2-pad && b.scrollX < maxScrollX {
				b.scrollX += 28
				if b.scrollX > maxScrollX {
					b.scrollX = maxScrollX
				}
				b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: pad - b.scrollY})
				localX = ev.X - abs2.X - b.txt.Offset().X
				if localX > maxX {
					localX = maxX
				}
			}
			if ev.Y < abs2.Y+pad && b.scrollY > 0 {
				b.scrollY -= 14
				if b.scrollY < 0 {
					b.scrollY = 0
				}
				b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: pad - b.scrollY})
				localY = ev.Y - abs2.Y - b.txt.Offset().Y
			} else if ev.Y > abs2.Y+h2-pad && b.scrollY < maxY {
				b.scrollY += 14
				if b.scrollY > maxY {
					b.scrollY = maxY
				}
				b.txt.SetOffset(rendering.Point{X: pad - b.scrollX, Y: pad - b.scrollY})
				localY = ev.Y - abs2.Y - b.txt.Offset().Y
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

func (b *MultiLineInputBox) OnKey(ev input.KeyEvent) {
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
		return
	}
	if ev.Mods.Control || ev.Mods.Meta {
		switch ev.Key {
		case input.KeyA:
			b.ed.SelectAll()
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
		case input.KeyC:
			s := b.ed.Copy()
			if s != "" {
				_ = effectiveClipboard(b.clipboard).Set("text/plain", s)
			}
			return
		case input.KeyX:
			s := b.ed.Cut()
			if s != "" {
				_ = effectiveClipboard(b.clipboard).Set("text/plain", s)
			}
			return
		case input.KeyV:
			if s, err := effectiveClipboard(b.clipboard).Get("text/plain"); err == nil && s != "" {
				s = decodeUnicodeEscapes(s)
				b.ed.Paste(s)
			}
			return
		case input.KeyBackspace:
			b.deleteWordMulti(false)
			return
		case input.KeyDelete:
			b.deleteWordMulti(true)
			return
		case input.KeyArrowLeft:
			b.moveWordMulti(false, ev.Mods.Shift)
			return
		case input.KeyArrowRight:
			b.moveWordMulti(true, ev.Mods.Shift)
			return
		}
	}
	if ev.Mods.Shift {
		switch ev.Key {
		case input.KeyArrowLeft:
			b.extendVisualMulti(-1)
			return
		case input.KeyArrowRight:
			b.extendVisualMulti(1)
			return
		case input.KeyArrowUp:
			b.extendVerticalMulti(-1)
			return
		case input.KeyArrowDown:
			b.extendVerticalMulti(1)
			return
		case input.KeyHome:
			b.extendToBoundaryMulti(false)
			return
		case input.KeyEnd:
			b.extendToBoundaryMulti(true)
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
	case input.KeyHome:
		b.ed.MoveCursorToBeginning()
	case input.KeyEnd:
		b.ed.MoveCursorToEnd()
	case input.KeyEnter:
		b.ed.Insert("\n")
	case input.KeyEscape:
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

func (b *MultiLineInputBox) extendVisualMulti(delta int) {
	if b == nil || b.ed == nil {
		return
	}
	sel := b.ed.SelectionRange()
	base := sel.Base
	cur := sel.Extent
	lay := b.txt.TextLayout()
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
					cur = b.ed.utf16ForByte(ln.Carets[idx+1].ByteOff)
				} else if delta < 0 && idx-1 >= 0 {
					cur = b.ed.utf16ForByte(ln.Carets[idx-1].ByteOff)
				} else if delta > 0 && lineIdx+1 < len(lay.Lines) {
					cur = b.ed.utf16ForByte(lay.Lines[lineIdx+1].Carets[0].ByteOff)
				} else if delta < 0 && lineIdx-1 >= 0 {
					prev := lay.Lines[lineIdx-1]
					cur = b.ed.utf16ForByte(prev.Carets[len(prev.Carets)-1].ByteOff)
				}
			}
		}
	} else {
		if delta > 0 {
			cur++
		} else {
			cur--
		}
	}
	b.ed.SetSelection(TextRange{Base: base, Extent: cur})
}

func (b *MultiLineInputBox) extendVerticalMulti(dir int) {
	if b == nil || b.ed == nil {
		return
	}
	base := b.ed.SelectionRange().Base
	lay := b.txt.TextLayout()
	if lay == nil || len(lay.Lines) == 0 {
		if dir < 0 {
			b.ed.MoveCursorUp()
		} else {
			b.ed.MoveCursorDown()
		}
		return
	}
	curByte := byteOffsetForUtf16(b.ed.GetText(), b.ed.SelectionRange().Extent)
	lineIdx, _, ok := lay.CaretForOffset(curByte)
	if !ok {
		return
	}
	target := lineIdx + dir
	if target < 0 || target >= len(lay.Lines) {
		return
	}
	curX, _, _, _ := lay.GetOffsetForCaret(curByte, rendering.AffinityDownstream, 1.5)
	tln := lay.Lines[target]
	best := 0
	bestDist := 1e12
	for i, c := range tln.Carets {
		d := c.X - curX
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	newOff := b.ed.utf16ForByte(tln.Carets[best].ByteOff)
	b.ed.SetSelection(TextRange{Base: base, Extent: newOff})
}

func (b *MultiLineInputBox) extendToBoundaryMulti(toEnd bool) {
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

func (b *MultiLineInputBox) moveWordMulti(forward, extend bool) {
	if b == nil || b.ed == nil {
		return
	}
	if extend {
		base := b.ed.SelectionRange().Base
		b.ed.MoveCursorByWord(forward)
		newOff := b.ed.SelectionRange().Start()
		b.ed.SetSelection(TextRange{Base: base, Extent: newOff})
	} else {
		b.ed.MoveCursorByWord(forward)
	}
}

func (b *MultiLineInputBox) deleteWordMulti(forward bool) {
	if b == nil || b.ed == nil {
		return
	}
	if !b.ed.SelectionRange().Collapsed() {
		b.ed.DeleteSelected()
		return
	}
	cur := b.ed.SelectionRange().Start()
	er := b.ed.EditableRange()
	if forward {
		dup := textinputDup(b.ed)
		dup.MoveCursorByWord(true)
		end := dup.SelectionRange().Start()
		if end > er.End() {
			end = er.End()
		}
		if end > cur {
			b.ed.SetSelection(TextRange{Base: cur, Extent: end})
			b.ed.DeleteSelected()
		}
	} else {
		dup := textinputDup(b.ed)
		dup.MoveCursorByWord(false)
		start := dup.SelectionRange().Start()
		if start < er.Start() {
			start = er.Start()
		}
		if start < cur {
			b.ed.SetSelection(TextRange{Base: start, Extent: cur})
			b.ed.DeleteSelected()
		}
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
func (b *MultiLineInputBox) OnText(ev input.TextEvent) {
	if b == nil || b.Disabled() {
		return
	}
}
func (b *MultiLineInputBox) OnIME(ev input.IMEEvent) {
	if b == nil || b.Disabled() {
		return
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Strings helper for paste decode is handled in embedder/router, not here.
func init() { _ = strings.Contains }
