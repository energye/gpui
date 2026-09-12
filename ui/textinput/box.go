package textinput

import (
	"fmt"
	"math"
	"time"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// defaultCursorWidth is the caret bar width (Flutter RenderEditable
// cursorWidth default 2.0).
const defaultCursorWidth = 2.0

// Box is the single edit-box implementation for every text field
// (Flutter EditableText / RenderEditable, one class): single-line vs
// multi-line, wrap and the retained viewport are properties, not types.
// All scrolling flows through the shared editScroll funnel; all caret
// stepping through Editor.MoveVisual; text sync through syncBoxText.
// What differs per mode lives in small branches (keyed by multi /
// useViewport), the way RenderEditable branches on _isMultiline.
type Box struct {
	*rendering.RenderBox
	*BaseEditable
	editScroll
	ed      *Editor
	txt     *rendering.RenderText
	bar     *rendering.RenderColorBox
	clip    *rendering.RenderClipRRect // direct mode (nil in viewport mode)
	content *rendering.RenderBox       // viewport mode content holder
	// Viewport is the retained-viewport node in viewport mode, nil in
	// direct mode. Exported for read probes (scroll offset); scroll
	// writes always go through the editScroll funnel.
	Viewport *rendering.RenderViewport
	Node     *focus.FocusNode
	sched    func()
	focused  bool
	caretOn  bool
	// Cursor prototype (Flutter RenderEditable cursorWidth/cursorHeight/
	// cursorOffset): cursorW is the bar width, cursorH the bar height
	// (<=0 follows the row height), cursorOff shifts the bar from the seam.
	cursorW     float64
	cursorH     float64
	cursorOffX  float64
	cursorOffY  float64
	multi       bool
	useViewport bool
	wrap        bool
	wrapMode    text.WrapMode
	clipboard   platform.Clipboard
	selHas      bool
	selR        float64
	selG        float64
	selB        float64
	selA        float64
	padHas      bool
	pad         float64
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

// Old shell names are aliases of the single Box: one implementation,
// configured by properties (multi-line, wrap, viewport).
type (
	InputBox          = Box
	ViewportInputBox  = Box
	MultiLineInputBox = Box
)

// highlightHost abstracts the highlight parent: clip in direct mode,
// viewport content in viewport mode.
type highlightHost interface {
	AddChild(rendering.RenderObject)
	RemoveChild(rendering.RenderObject)
}

func (b *Box) highlightParent() highlightHost {
	if b == nil {
		return nil
	}
	if b.useViewport {
		return b.content
	}
	return b.clip
}

// newBoxShell builds the direct-mode shell (clip tree). Callers set ink,
// mode properties and sync once.
func newBoxShell(ed *Editor, w, h, fontSize float64, name string) *Box {
	inner := rendering.NewRenderBox()
	b := &Box{
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
	b.editScroll.txt = b.txt
	b.txt.FontSize = fontSize
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
	b.bar = rendering.NewRenderColorBox(defaultCursorWidth, 22, 1.0, 0.85, 0.2, 1)
	clip.AddChild(b.bar)
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		b.paintBorder(pc, size)
	}
	b.caretOn = true
	b.cursorW = defaultCursorWidth
	b.Node = focus.NewFocusNode(name)
	b.Node.Target = b
	b.Node.OnFocusChange = func(on bool) {
		b.onFocusChange(on)
	}
	if ed != nil {
		ed.OnChange = func() { b.sync() }
	}
	return b
}

// NewBox creates a single-line box (= old InputBox: box A).
// It shares the retained-viewport tree with NewBoxWithViewport: scrolling
// re-blits the viewport texture instead of re-recording vector text per
// step, so pasting a long line (ex B content) and arrowing through it
// cannot raster-storm (each scroll step re-recorded + re-phased every
// glyph). Short-text behavior is unchanged (same caret, border, colors).
func NewBox(ed *Editor, w, h, fontSize float64) *Box {
	if fontSize <= 0 {
		fontSize = 16
	}
	return NewBoxWithViewport(ed, w, h, fontSize)
}

// NewInputBox is the single-line preset (= NewBox, kept for call sites).
func NewInputBox(ed *Editor, w, h, fontSize float64) *Box {
	return NewBox(ed, w, h, fontSize)
}

// NewBoxWithViewport creates a single-line retained-viewport box
// (= old ViewportInputBox: boxes B/C). Scroll re-blits the viewport
// texture instead of shifting the text offset.
func NewBoxWithViewport(ed *Editor, w, h, fontSize float64) *Box {
	if fontSize <= 0 {
		fontSize = 12
	}
	if ed != nil {
		ed.SetSingleLine(true)
	}
	outer := rendering.NewRenderBox()
	b := &Box{
		RenderBox:    outer,
		BaseEditable: NewBaseEditable(ed),
		ed:           ed,
		txt:          rendering.NewRenderText(""),
		useViewport:  true,
		content:      rendering.NewRenderBox(),
	}
	outer.Init(b)
	outer.SetRelayoutBoundary(true)
	outer.SetRepaintBoundary(true)
	b.editScroll.txt = b.txt
	b.txt.FontSize = fontSize
	b.txt.R, b.txt.G, b.txt.B, b.txt.A = 0.05, 0.75, 0.95, 1
	b.txt.MaxWidth = 0 // single line, no wrap
	outer.FixedWidth = w
	outer.FixedHeight = h
	// Content holds txt+bar; viewport wraps content.
	b.content.AddChild(b.txt)
	b.bar = rendering.NewRenderColorBox(defaultCursorWidth, 22, 1.0, 0.85, 0.2, 1)
	b.content.AddChild(b.bar)
	vp := rendering.NewRenderViewport(b.content)
	vp.FixedWidth = w - 2
	vp.FixedHeight = h - 2
	outer.AddChild(vp)
	b.Viewport = vp
	vp.SetOffset(rendering.Point{X: 1, Y: 1})
	b.editScroll.viewport = vp
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		b.paintBorder(pc, size)
	}
	b.caretOn = true
	b.cursorW = defaultCursorWidth
	b.Node = focus.NewFocusNode(fmt.Sprintf("viewport-input-%p", b))
	b.Node.Target = b
	b.Node.OnFocusChange = func(on bool) {
		b.onFocusChange(on)
	}
	if ed != nil {
		ed.OnChange = func() { b.sync() }
	}
	b.sync()
	return b
}

// NewViewportInputBox is the single-line viewport preset
// (= NewBoxWithViewport, kept for call sites).
func NewViewportInputBox(ed *Editor, w, h, fontSize float64) *Box {
	return NewBoxWithViewport(ed, w, h, fontSize)
}

// NewMultiLineInputBox creates a wrapping-capable multi-line box
// (= old MultiLineInputBox: boxes D/E/F): direct-offset tree with the
// multi-line property set. Wrap stays off by default (R4 tests).
func NewMultiLineInputBox(ed *Editor, w, h, fontSize float64) *Box {
	if fontSize <= 0 {
		fontSize = 14
	}
	b := newBoxShell(ed, w, h, fontSize, fmt.Sprintf("multi-%p", ed))
	b.txt.R, b.txt.G, b.txt.B, b.txt.A = 0.06, 0.85, 0.60, 1
	// 默认不自动换行（满足 R4 测试要求），需换行时显式 SetWrap(true)
	b.wrap = false
	b.wrapMode = text.WrapWordChar
	b.txt.MaxWidth = 0
	b.multi = true
	if ed != nil {
		ed.SetSingleLine(false)
	}
	b.sync()
	return b
}

// paintBorder draws the shared box border + background.
func (b *Box) paintBorder(pc *rendering.PaintContext, size rendering.Size) {
	if b == nil || pc == nil || pc.DC == nil {
		return
	}
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

// onFocusChange is the single focus handler (viewport/multi form: blur is
// always honored, even while disabled, so focus can never strand).
func (b *Box) onFocusChange(on bool) {
	if b == nil || b.Disabled() && on {
		return
	}
	b.focused = on
	syncBlinkRegistration(b.RenderBox, on, b)
	b.MarkNeedsPaint()
	b.sync()
}

// SetMultiLine selects multi-line behavior (Enter inserts newline, 2D
// mapping, vertical scroll), mirroring EditableText(maxLines: null).
// Password masking only applies while single-line (Flutter: obscureText
// requires maxLines == 1).
func (b *Box) SetMultiLine(v bool) {
	if b == nil || b.multi == v {
		return
	}
	b.multi = v
	if b.ed != nil {
		b.ed.SetSingleLine(!v)
	}
	b.sync()
}

// MultiLine reports the multi-line property.
func (b *Box) MultiLine() bool { return b != nil && b.multi }

// SetWrap controls auto-wrap for multi-line (Flutter softWrap).
func (b *Box) SetWrap(v bool) {
	if b == nil {
		return
	}
	b.wrap = v
	b.sync()
}

// Wrap reports the wrap property.
func (b *Box) Wrap() bool {
	if b == nil {
		return false
	}
	return b.wrap
}

// SetWrapMode controls the line-break policy, default WrapWordChar.
func (b *Box) SetWrapMode(m text.WrapMode) {
	if b == nil {
		return
	}
	b.wrapMode = m
	b.sync()
}

// WrapMode reports the wrap policy.
func (b *Box) WrapMode() text.WrapMode {
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

// SetMaxLines forwards to the text node in multi-line mode; single-line
// ignores it but keeps editable_range intact (F-C5).
func (b *Box) SetMaxLines(n int) {
	if b == nil || b.txt == nil {
		return
	}
	if !b.multi {
		return
	}
	b.txt.SetMaxLines(n)
	b.sync()
}

// SetOverflow forwards to the text node.
func (b *Box) SetOverflow(o rendering.TextOverflow) {
	if b == nil || b.txt == nil {
		return
	}
	b.txt.SetOverflow(o)
	b.sync()
}

// SetTextColor sets the text ink (per-box property).
func (b *Box) SetTextColor(r, g, bl, a float64) {
	if b == nil || b.txt == nil {
		return
	}
	b.txt.R, b.txt.G, b.txt.B, b.txt.A = r, g, bl, a
	b.MarkNeedsPaint()
}

func (b *Box) SetPlaceholder(s string) {
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

func (b *Box) Placeholder() string {
	if b == nil || b.BaseEditable == nil {
		return ""
	}
	return b.BaseEditable.Placeholder()
}

func (b *Box) SetDisabled(v bool) {
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
			syncBlinkRegistration(b.RenderBox, false, b)
			b.caretOn = false
			b.layoutCaret()
			if b.Node.HasFocus() {
				b.Node.Unfocus()
			}
		}
	}
	b.MarkNeedsPaint()
}

func (b *Box) Disabled() bool {
	if b == nil || b.BaseEditable == nil {
		return false
	}
	return b.BaseEditable.Disabled()
}

// SetSelectionColor customizes the selection ink (per-box).
func (b *Box) SetSelectionColor(r, g, b2, a float64) {
	if b == nil {
		return
	}
	b.selHas = true
	b.selR, b.selG, b.selB, b.selA = r, g, b2, a
	b.MarkNeedsPaint()
}

func (b *Box) ClearSelectionColor() {
	if b == nil {
		return
	}
	b.selHas = false
	b.MarkNeedsPaint()
}

func (b *Box) SelectionColor() (r, g, b2, a float64) {
	return resolveSelColor(b.selHas, b.selR, b.selG, b.selB, b.selA)
}

func (b *Box) SetPadding(pad float64) {
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

func (b *Box) ClearPadding() {
	if b == nil {
		return
	}
	b.padHas = false
	b.sync()
}

func (b *Box) Padding() float64 { return resolvePad(b.padHas, b.pad) }

// SetObscuringCharacter customizes the password mask (Flutter
// TextField.obscuringCharacter, default '•').
func (b *Box) SetObscuringCharacter(r rune) {
	if b == nil || b.ed == nil {
		return
	}
	b.ed.SetObscuringCharacter(r)
}

// ObscuringCharacter returns the current mask character.
func (b *Box) ObscuringCharacter() rune {
	if b == nil || b.ed == nil {
		return '•'
	}
	return b.ed.ObscuringCharacter()
}

// SetCursorWidth sets the caret bar width (Flutter cursorWidth).
func (b *Box) SetCursorWidth(w float64) {
	if b == nil {
		return
	}
	if w <= 0 {
		w = defaultCursorWidth
	}
	b.cursorW = w
	b.sync()
}

// CursorWidth returns the caret bar width.
func (b *Box) CursorWidth() float64 {
	if b == nil || b.cursorW <= 0 {
		return defaultCursorWidth
	}
	return b.cursorW
}

// SetCursorHeight sets the caret bar height (Flutter cursorHeight);
// <=0 follows the row height.
func (b *Box) SetCursorHeight(h float64) {
	if b == nil {
		return
	}
	if h < 0 {
		h = 0
	}
	b.cursorH = h
	b.sync()
}

// CursorHeight returns the caret bar height setting (<=0 means row height).
func (b *Box) CursorHeight() float64 {
	if b == nil {
		return 0
	}
	return b.cursorH
}

// SetCursorOffset shifts the caret bar from the seam
// (Flutter cursorOffset, desktop default 0,0: bar left edge on the seam).
func (b *Box) SetCursorOffset(dx, dy float64) {
	if b == nil {
		return
	}
	b.cursorOffX, b.cursorOffY = dx, dy
	b.sync()
}

// CursorOffset returns the caret seam offset.
func (b *Box) CursorOffset() (float64, float64) {
	if b == nil {
		return 0, 0
	}
	return b.cursorOffX, b.cursorOffY
}

// caretMargin is the reveal/clamp gap around the caret
// (Flutter RenderEditable _caretMargin = _kCaretGap + cursorWidth).
func (b *Box) caretMargin() float64 {
	return 1 + b.CursorWidth()
}

func (b *Box) IsFocused() bool { return b != nil && b.focused }
func (b *Box) IsCaretOn() bool { return b != nil && b.caretOn }

func (b *Box) ScrollX() float64 {
	if b == nil {
		return 0
	}
	if b.useViewport && b.Viewport != nil {
		return b.Viewport.ScrollOffset().X
	}
	return b.scrollX
}

func (b *Box) ScrollY() float64 {
	if b == nil {
		return 0
	}
	if b.useViewport && b.Viewport != nil {
		return b.Viewport.ScrollOffset().Y
	}
	return b.scrollY
}

func (b *Box) SetCaretOn(v bool) {
	if b == nil {
		return
	}
	b.caretOn = v
	b.blinkElapsed = 0
	b.layoutCaret()
	b.MarkNeedsPaint()
}

// TickCaret advances the caret blink phase on UI-thread dt.
func (b *Box) TickCaret(dt float64) {
	if b == nil || !b.focused {
		if b != nil && b.caretOn {
			b.caretOn = false
			b.layoutCaret()
			b.MarkNeedsPaint()
		}
		return
	}
	var toggled bool
	b.caretOn, b.blinkElapsed, toggled = stepBlink(b.caretOn, b.blinkElapsed, dt)
	if toggled {
		b.layoutCaret()
		b.MarkNeedsPaint()
	}
}

// BlinkTick implements rendering.Blinkable.
func (b *Box) BlinkTick(dt float64) bool {
	if b == nil {
		return false
	}
	before := b.caretOn
	b.TickCaret(dt)
	return b.caretOn != before
}

// NextBlinkIn implements rendering.BlinkDeadliner.
func (b *Box) NextBlinkIn() (time.Duration, bool) {
	if b == nil || !b.focused {
		return 0, false
	}
	return nextBlinkIn(b.blinkElapsed)
}

// SetFace sets the font face for this box.
func (b *Box) SetFace(face text.Face) {
	if b != nil && b.txt != nil {
		b.txt.SetFace(face)
		b.sync()
	}
}

// SetClipboard sets the platform clipboard.
func (b *Box) SetClipboard(c platform.Clipboard) { b.clipboard = c }
func (b *Box) Clipboard() platform.Clipboard {
	if b == nil {
		return nil
	}
	return b.clipboard
}

// SetSchedule sets the frame scheduler (PipelineApp.ScheduleFrame).
func (b *Box) SetSchedule(fn func()) {
	if b != nil {
		b.sched = fn
	}
}

// FocusNode returns the focus node for registration.
func (b *Box) FocusNode() *focus.FocusNode {
	if b == nil {
		return nil
	}
	return b.Node
}

// Editor returns the backing editor.
func (b *Box) Editor() *Editor {
	if b == nil {
		return nil
	}
	return b.ed
}

func (b *Box) ContentPurpose() platform.ContentPurpose {
	if b != nil && b.BaseEditable != nil {
		return b.BaseEditable.ContentType().Purpose
	}
	return platform.PurposeNormal
}

func (b *Box) ContentType() platform.ContentType {
	return platform.ContentType{Purpose: b.ContentPurpose()}
}

func (b *Box) TextLayout() *rendering.TextLayout {
	if b == nil || b.txt == nil {
		return nil
	}
	return b.txt.TextLayout()
}

// caretAnchor reports the caret rect in owner (box-frame) coordinates:
// clip coords in direct mode, box coords (content minus viewport scroll,
// plus the 1px border) in viewport mode. Single source for layoutCaret
// and IMERect.
func (b *Box) caretAnchor() (x, top, bottom float64, ok bool) {
	if b == nil || b.txt == nil || b.ed == nil {
		return 0, 0, 0, false
	}
	if b.useViewport && b.Viewport == nil {
		return 0, 0, 0, false
	}
	off := b.txt.Offset()
	lh := b.txt.LineHeight()
	if lh <= 0 {
		lh = 22
	}
	// toOwner converts a content-frame rect to owner-frame coords.
	toOwner := func(rx, rtop, rbottom float64) (float64, float64, float64, bool) {
		if b.useViewport {
			vpOff := b.Viewport.ScrollOffset()
			return rx - vpOff.X + 1, rtop + 1, rbottom + 1, true
		}
		return rx, rtop, rbottom, true
	}
	if b.txt.Text == "" {
		return toOwner(off.X, off.Y, off.Y+lh)
	}
	// Password: caret follows the mask seams, not source bytes.
	if b.ed.IsPassword() {
		ch := b.ed.ObscuringCharacter()
		chBytes := len(string(ch))
		if chBytes <= 0 {
			chBytes = 3
		}
		runeIdx := utf16ToRuneIndex(b.ed.GetText(), b.ed.SelectionRange().Extent)
		maskedByte := runeIdx * chBytes
		aff := b.ed.TextRange().Affinity
		if lay := b.txt.TextLayout(); lay != nil && lay.LineCount() > 0 {
			if cx, cy, chh, ok := lay.GetOffsetForCaret(maskedByte, aff, 1.5); ok {
				return toOwner(off.X+cx, off.Y+cy, off.Y+cy+chh)
			}
		}
		charW := b.txt.MeasureWidth(string(ch))
		if charW <= 0 {
			charW = 12
		}
		return toOwner(off.X+float64(runeIdx)*charW, off.Y, off.Y+lh)
	}
	curByte := b.ed.GetCursorOffset()
	aff := b.ed.TextRange().Affinity
	if lay := b.txt.TextLayout(); lay != nil && lay.LineCount() > 0 {
		if cx, cy, chh, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
			return toOwner(off.X+cx, off.Y+cy, off.Y+cy+chh)
		}
	}
	lineIdx, penX, ok := b.txt.CaretColumn(min(curByte, len(b.txt.Text)))
	if !ok {
		return 0, 0, 0, false
	}
	rx := off.X + penX
	var rtop, rbottom float64
	if lay := b.txt.TextLayout(); lay != nil && lay.LineCount() > lineIdx {
		rtop = off.Y + lay.LineTop(lineIdx)
		rbottom = rtop + lay.LineHeight(lineIdx)
	} else {
		rtop = off.Y + float64(lineIdx)*lh
		rbottom = rtop + lh
	}
	return toOwner(rx, rtop, rbottom)
}

func (b *Box) IMERect() platform.Rect {
	if b != nil && b.ed != nil && b.ed.IsComposing() {
		if lay := b.txt.TextLayout(); lay != nil && lay.LineCount() > 0 {
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
					if b.useViewport && b.Viewport != nil {
						vpOff := b.Viewport.ScrollOffset()
						return platform.Rect{X: abs.X + off.X + minX - vpOff.X + 1, Y: abs.Y + off.Y + minY + 1, W: maxX - minX, H: maxY - minY}
					}
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

func (b *Box) Sync() { b.sync() }

func (b *Box) sync() {
	if b == nil || b.txt == nil || b.ed == nil {
		return
	}
	if b.useViewport && (b.Viewport == nil || b.content == nil) {
		return
	}
	rawEmpty := b.ed.GetText() == ""
	disp := syncBoxText(b.txt, b.ed, b.placeholder, rawEmpty && !b.focused && b.placeholder != "")
	if rawEmpty {
		b.clear()
	}
	pad := b.Padding()
	if b.multi {
		if b.wrap {
			wantW := b.FixedWidth - 2*pad
			if wantW < 0 {
				wantW = 0
			}
			// Setter invalidates the layout cache; a bare field write
			// would keep serving the old wrap after a width change.
			b.txt.SetMaxWidth(wantW)
		} else {
			b.txt.SetMaxWidth(0)
		}
		curByte := b.ed.GetCursorOffset()
		aff := b.ed.TextRange().Affinity
		lay := b.txt.TextLayout()
		m := scrollMetrics{pad: pad, multi: true, margin: b.caretMargin()}
		m.visW = b.FixedWidth - 2*pad
		m.visH = b.FixedHeight - 2*pad
		if m.visW < 0 {
			m.visW = 0
		}
		if m.visH < 0 {
			m.visH = 0
		}
		if lay != nil && lay.LineCount() > 0 {
			if x, y, h, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
				m.caretX, m.caretY = x, y
				m.lineH = h
				if lineIdx := lay.RowForY(y); lay.LineCount() > lineIdx {
					m.lineH = lay.LineHeight(lineIdx)
				}
			} else {
				var lineIdx int
				lineIdx, m.caretX, _ = lay.CaretForOffset(curByte)
				m.caretY = lay.LineTop(lineIdx)
				m.lineH = lay.LineHeight(lineIdx)
			}
			maxW := 0.0
			for i := 0; i < lay.LineCount(); i++ {
				if _, _, w, _, ok := lay.Line(i); ok && w > maxW {
					maxW = w
				}
			}
			m.maxW = maxW
			m.totalH = lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
		} else {
			var lineIdx int
			lineIdx, m.caretX, _ = b.txt.CaretColumn(min(curByte, len(b.txt.Text)))
			lh := b.txt.LineHeight()
			if lh <= 0 {
				lh = 22
			}
			m.caretY = float64(lineIdx) * lh
			m.lineH = lh
		}
		b.ensureVisible(m)
	} else {
		curByte := b.ed.GetCursorOffset()
		if b.ed.IsPassword() {
			curByte = maskedCaretByte(b.ed, disp, curByte)
		}
		aff := b.ed.TextRange().Affinity
		lh := b.txt.LineHeight()
		if lh <= 0 {
			lh = 22
		}
		textY := (b.FixedHeight - lh) / 2
		if b.useViewport {
			textY = (b.FixedHeight - 2 - lh) / 2
		}
		if textY < 0 {
			textY = 0
		}
		m := scrollMetrics{pad: pad, textY: textY, margin: b.caretMargin()}
		m.visW = b.FixedWidth - 2*pad
		if b.useViewport {
			m.visW = b.FixedWidth - 2 - 2*pad
			b.txt.SetOffset(rendering.Point{X: pad, Y: textY})
			// Seed the funnel from the viewport (single scroll source).
			b.scrollX = b.Viewport.ScrollOffset().X
		}
		if m.visW < 0 {
			m.visW = 0
		}
		if lay := b.txt.TextLayout(); lay != nil && lay.LineCount() > 0 {
			if x, _, _, ok := lay.GetOffsetForCaret(curByte, aff, 1.5); ok {
				m.caretX = x
			} else {
				_, m.caretX, _ = lay.CaretForOffset(curByte)
			}
			_, _, m.maxW, _, _ = lay.Line(0)
		} else {
			m.maxW = b.txt.MeasureWidth(b.txt.Text)
		}
		b.ensureVisible(m)
	}
	b.syncHighlight()
	// 严格对齐 Flutter：编辑/同步后光标立即可见且闪烁计时重置为 0
	b.caretOn = true
	b.blinkElapsed = 0
	b.layoutCaret()
	if b.sched != nil {
		b.sched()
	}
}

// syncHighlight draws the selection via BoxesForRange (TextLayout single
// source). Parent and visible window branch on the viewport property.
func (b *Box) syncHighlight() {
	host := b.highlightParent()
	if b == nil || host == nil || b.txt == nil || b.ed == nil {
		return
	}
	for _, h := range b.highlights {
		if h != nil {
			host.RemoveChild(h)
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
	text := b.ed.GetText()
	sByte := byteOffsetForUtf16(text, sel.Start())
	eByte := byteOffsetForUtf16(text, sel.End())
	lay := b.txt.TextLayout()
	if lay == nil || lay.LineCount() == 0 {
		return
	}
	boxes := lay.BoxesForRange(sByte, eByte)
	if len(boxes) == 0 {
		return
	}
	off := b.txt.Offset()
	var vis rendering.Rect
	if b.useViewport && b.Viewport != nil {
		vis = rendering.NewRect(b.Viewport.ScrollOffset().X, 0, b.FixedWidth-2, b.FixedHeight)
	} else {
		vis = rendering.NewRect(0, 0, b.FixedWidth, b.FixedHeight)
	}
	sr, sg, sb, sa := b.SelectionColor()
	b.highlights = make([]*rendering.RenderColorBox, 0, len(boxes))
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
		hl := rendering.NewRect(off.X+r.Min.X, off.Y+r.Min.Y-pad, r.Size().Width, h+pad)
		hb := newClippedHighlight(hl, vis, sr, sg, sb, sa)
		if hb == nil {
			continue
		}
		host.AddChild(hb)
		b.highlights = append(b.highlights, hb)
	}
	host.RemoveChild(b.txt)
	host.RemoveChild(b.bar)
	host.AddChild(b.txt)
	host.AddChild(b.bar)
}

func (b *Box) layoutCaret() {
	if b == nil || b.bar == nil || b.txt == nil || b.ed == nil {
		return
	}
	x, top, bottom, ok := b.caretAnchor()
	if !ok {
		return
	}
	// Flutter getLocalRectForCaret (desktop path): the bar height is the
	// cursor height (row height when unset), centered on the full row
	// height; the bar sits at the seam plus cursorOffset, snapped to whole
	// pixels so it stays texel-aligned with the retained blits.
	full := bottom - top
	caretH := b.cursorH
	if caretH <= 0 {
		caretH = full
	}
	bx := x + b.cursorOffX
	by := top + (full-caretH)/2 + b.cursorOffY
	// Bar lives in content coords in viewport mode: undo the owner-frame
	// conversion (minus viewport scroll, plus the 1px border).
	if b.useViewport && b.Viewport != nil {
		vpOff := b.Viewport.ScrollOffset()
		bx += vpOff.X - 1
		by -= 1
	}
	bx = math.Round(bx)
	by = math.Round(by)
	b.bar.MoveTo(bx, by)
	if w := b.CursorWidth(); w > 0 && w != b.bar.Width {
		b.bar.Width = w
		b.bar.MarkNeedsPaint()
	}
	if caretH > 0 && caretH != b.bar.Height {
		b.bar.Height = caretH
		b.bar.MarkNeedsPaint()
	}
	if b.caretOn && b.focused {
		b.bar.SetAlpha(1)
	} else {
		b.bar.SetAlpha(0)
	}
}

// Layout preserves the text/caret offsets computed by sync().
// RenderBox.Layout would reset children to Pad(0,0).
// Wrap width tracks the box width here (not only in sync): a width change
// with no edit in between must still rewrap and re-clamp scroll.
func (b *Box) Layout(c rendering.Constraints) rendering.Size {
	if b == nil || b.txt == nil || b.bar == nil {
		return rendering.Size{}
	}
	// Size follow-through (window resize): the clip/viewport shells were
	// sized once at construction; when the outer box size changes they must
	// track it here, otherwise content stays clipped to the old rect while
	// the border paints the new one. sync re-clamps scroll and refreshes
	// the cull hints for the new visible window.
	resized := false
	if b.clip != nil && (b.clip.FixedWidth != b.FixedWidth || b.clip.FixedHeight != b.FixedHeight) {
		b.clip.FixedWidth, b.clip.FixedHeight = b.FixedWidth, b.FixedHeight
		resized = true
	}
	if b.Viewport != nil {
		vw, vh := b.FixedWidth-2, b.FixedHeight-2
		if vw < 0 {
			vw = 0
		}
		if vh < 0 {
			vh = 0
		}
		if b.Viewport.FixedWidth != vw || b.Viewport.FixedHeight != vh {
			b.Viewport.FixedWidth, b.Viewport.FixedHeight = vw, vh
			resized = true
		}
	}
	if b.multi && b.wrap {
		wantW := b.FixedWidth - 2*b.Padding()
		if wantW < 0 {
			wantW = 0
		}
		if b.txt.MaxWidth != wantW {
			b.sync()
		} else if resized {
			b.sync()
		}
	} else if resized {
		b.sync()
	}
	if resized {
		// A size change always repaints the border/background shell: layout
		// alone leaves the retained picture (recorded at the old rect)
		// blitting stale. The text child re-marks itself through band
		// expiry (visW/visH inequality) inside sync above.
		b.MarkNeedsPaint()
	}
	textOff := b.txt.Offset()
	barOff := b.bar.Offset()
	sz := b.RenderBox.Layout(c)
	b.txt.SetOffset(textOff)
	b.bar.SetOffset(barOff)
	return sz
}

// pointerLocal maps an event point into text coordinates. Direct mode text
// moves inside the offset; viewport mode text is fixed and the viewport
// scrolls (plus the 1px border).
func (b *Box) pointerLocal(evX, evY float64) (float64, float64) {
	abs := absoluteOrigin(b)
	if b.useViewport && b.Viewport != nil {
		vpOff := b.Viewport.ScrollOffset()
		txtOff := b.txt.Offset()
		pad := b.Padding()
		return evX - abs.X - 1 - pad + vpOff.X, evY - abs.Y - 1 + vpOff.Y - txtOff.Y
	}
	off := b.txt.Offset()
	return evX - abs.X - off.X, evY - abs.Y - off.Y
}

func (b *Box) OnPointer(ev input.PointerEvent) {
	if b == nil || b.ed == nil || b.Disabled() {
		return
	}
	localX, localY := b.pointerLocal(ev.X, ev.Y)
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
		// Password: seams map onto mask runes; double-click selects all.
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
			b.countClick(localX, localY)
			if b.clickCount >= 2 {
				b.ed.SelectAll()
				b.dragging = false
				if b.clickCount > 3 {
					b.clickCount = 3
				}
			} else {
				utf16Off := utf16ForRuneIndex(b.ed.GetText(), runeIdx)
				b.ed.SetSelection(TextRange{Base: utf16Off, Extent: utf16Off})
				b.dragStart = utf16Off
				b.dragging = true
				b.dragLastX = ev.X
				b.dragLastY = ev.Y
				b.startAutoScroll()
			}
			return
		}
		b.countClick(localX, localY)
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
			b.clickCount = 3 // clamp
		} else {
			b.ed.SetCaretWithAffinity(byteOff, aff)
			b.dragStart = b.ed.utf16ForByte(byteOff)
			b.dragging = true
			b.dragLastX = ev.X
			b.dragLastY = ev.Y
			b.startAutoScroll()
		}
	case input.PointerMove:
		b.dragLastX = ev.X
		b.dragLastY = ev.Y
		if b.dragging {
			b.dragMove(ev.X, ev.Y, localX, localY)
		}
	case input.PointerUp:
		b.dragging = false
	}
}

func (b *Box) countClick(localX, localY float64) {
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
}

// dragMove scrolls at the edges and extends the selection. Axis handling
// branches on the multi/viewport properties; selection mapping is shared.
func (b *Box) dragMove(evX, evY, localX, localY float64) {
	pad := b.Padding()
	abs := absoluteOrigin(b)
	txtOff := b.txt.Offset()
	if b.multi {
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
		if layTmp != nil && layTmp.LineCount() > 0 {
			for i := 0; i < layTmp.LineCount(); i++ {
				if _, _, w, _, ok := layTmp.Line(i); ok && w > maxX {
					maxX = w
				}
			}
			totalH := layTmp.LineTop(layTmp.LineCount()-1) + layTmp.LineHeight(layTmp.LineCount()-1)
			maxY = totalH - visH
			if maxY < 0 {
				maxY = 0
			}
		}
		maxScrollX := maxX - visW + b.caretMargin()
		if maxScrollX < 0 {
			maxScrollX = 0
		}
		m := scrollMetrics{maxW: maxX, totalH: maxY + visH, visW: visW, visH: visH, pad: pad, multi: true, margin: b.caretMargin()}
		if evX < abs.X+pad && b.scrollX > 0 {
			b.scrollBy(-28, 0, m)
			localX = evX - abs.X - b.txt.Offset().X
		} else if evX > abs.X+w2-pad && b.scrollX < maxScrollX {
			b.scrollBy(28, 0, m)
			localX = evX - abs.X - b.txt.Offset().X
			if localX > maxX {
				localX = maxX
			}
		}
		if evY < abs.Y+pad && b.scrollY > 0 {
			b.scrollBy(0, -14, m)
			localY = evY - abs.Y - b.txt.Offset().Y
		} else if evY > abs.Y+h2-pad && b.scrollY < maxY {
			b.scrollBy(0, 14, m)
			localY = evY - abs.Y - b.txt.Offset().Y
		}
	} else if b.useViewport {
		visW := b.FixedWidth - 2 - 2*pad
		if visW < 0 {
			visW = 0
		}
		layTmp := b.txt.TextLayout()
		maxX := 0.0
		if layTmp != nil && layTmp.LineCount() > 0 {
			_, _, maxX, _, _ = layTmp.Line(0)
		}
		maxScroll := maxX - visW + b.caretMargin()
		if maxScroll < 0 {
			maxScroll = 0
		}
		m := scrollMetrics{maxW: maxX, visW: visW, pad: pad, margin: b.caretMargin()}
		if evX < abs.X+pad && b.scrollX > 0 {
			b.scrollBy(-28, 0, m)
			localX = evX - abs.X - 1 - pad + b.scrollX
		} else if evX > abs.X+b.FixedWidth-pad && b.scrollX < maxScroll {
			b.scrollBy(28, 0, m)
			localX = evX - abs.X - 1 - pad + b.scrollX
			if localX > maxX {
				localX = maxX
			}
		}
	} else {
		visW := b.FixedWidth - 2*pad
		if visW < 0 {
			visW = 0
		}
		layTmp := b.txt.TextLayout()
		maxX := 0.0
		if layTmp != nil && layTmp.LineCount() > 0 {
			_, _, maxX, _, _ = layTmp.Line(0)
		}
		maxScroll := maxX - visW + b.caretMargin()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if evX < abs.X+pad && b.scrollX > 0 {
			b.scrollBy(-28, 0, scrollMetrics{maxW: maxX, visW: visW, pad: pad, textY: txtOff.Y, margin: b.caretMargin()})
			localX = evX - abs.X - b.txt.Offset().X
		} else if evX > abs.X+b.FixedWidth-pad && b.scrollX < maxScroll {
			b.scrollBy(28, 0, scrollMetrics{maxW: maxX, visW: visW, pad: pad, textY: txtOff.Y, margin: b.caretMargin()})
			localX = evX - abs.X - b.txt.Offset().X
			if localX > maxX {
				localX = maxX
			}
		}
	}
	lay := b.txt.TextLayout()
	var byteOff int
	if lay != nil {
		byteOff, _ = lay.GetPositionForOffset(localX, localY)
	} else {
		byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
	}
	b.setDragSelection(byteOff)
}

// setDragSelection extends the drag selection to a layout byte offset,
// mapping through the password mask when masked.
func (b *Box) setDragSelection(byteOff int) {
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
		if runes := []rune(b.ed.GetText()); runeIdx > len(runes) {
			runeIdx = len(runes)
		}
		b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: utf16ForRuneIndex(b.ed.GetText(), runeIdx)})
		return
	}
	cur := b.ed.utf16ForByte(byteOff)
	b.ed.SetSelection(TextRange{Base: b.dragStart, Extent: cur})
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

func (b *Box) OnKey(ev input.KeyEvent) {
	if b == nil || b.ed == nil {
		return
	}
	if !ev.Pressed {
		return
	}
	if b.Disabled() {
		return
	}
	// Composing 时 Home/End/Page/Arrow/Enter 由 IME 优先消费
	if b.ed.IsComposing() && isComposingFilterKey(ev.Key) {
		return
	}
	if ev.Mods.Control || ev.Mods.Meta {
		switch ev.Key {
		case input.KeyA:
			b.ed.SelectAll()
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
		case input.KeyBackspace:
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
		// MoveVisualUp nil-guards into MoveCursorUp; single-line text has
		// one visual row, so both old single-line forms land here.
		b.ed.MoveVisualUp(b.txt.TextLayout())
	case input.KeyArrowDown:
		b.ed.MoveVisualDown(b.txt.TextLayout())
	case input.KeyHome:
		b.ed.MoveCursorToBeginning()
	case input.KeyEnd:
		b.ed.MoveCursorToEnd()
	case input.KeyEnter:
		if b.multi {
			b.ed.Insert("\n")
		}
	case input.KeyEscape:
		b.ed.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	}
}

func (b *Box) extendVisual(delta int) {
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
		b.ed.SetSelection(TextRange{Base: base, Extent: stepRuneUTF16(b.ed.GetText(), sel.Extent, delta)})
		return
	}
	sel := b.ed.SelectionRange()
	base := sel.Base
	if sel.Collapsed() {
		base = sel.Start()
	}
	cur := sel.Extent
	lay := b.txt.TextLayout()
	curByte := byteOffsetForUtf16(b.ed.GetText(), cur)
	if newByte, _, ok := visualStepByte(b.ed, curByte, delta, lay); ok {
		b.ed.extendExtentBytes(curByte, newByte)
		return
	}
	b.ed.SetSelection(TextRange{Base: base, Extent: stepRuneUTF16(b.ed.GetText(), cur, delta)})
}

func (b *Box) extendVertical(dir int) {
	if b == nil || b.ed == nil {
		return
	}
	lay := b.txt.TextLayout()
	if lay == nil || lay.LineCount() == 0 {
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
	if target < 0 || target >= lay.LineCount() {
		return
	}
	curX, _, _, _ := lay.GetOffsetForCaret(curByte, rendering.AffinityDownstream, 1.5)
	newByte, ok := nearestLineByte(lay, target, curX)
	if !ok {
		return
	}
	b.ed.extendExtentBytes(curByte, newByte)
}

func (b *Box) extendToBoundary(toEnd bool) {
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

func (b *Box) moveWord(forward, extend bool) {
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

func (b *Box) deleteWord(forward bool) {
	if b == nil || b.ed == nil {
		return
	}
	if !b.ed.SelectionRange().Collapsed() {
		b.ed.DeleteSelected()
		return
	}
	cur := b.ed.SelectionRange().Start()
	er := b.ed.EditableRange()
	start := cur
	end := cur
	if forward {
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

func (b *Box) moveVisual(delta int) {
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

// MoveVisual steps the caret visually (shared by all modes).
func (b *Box) MoveVisual(delta int) { b.moveVisual(delta) }

// doAutoScroll is the single drag auto-scroll (X for single-line either
// tree, XY for multi-line); selection remap is shared.
func (b *Box) doAutoScroll() {
	if b == nil || !b.dragging {
		b.autoScrollRunning = false
		return
	}
	pad := b.Padding()
	abs := absoluteOrigin(b)
	if b.multi {
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
		if lay != nil && lay.LineCount() > 0 {
			for i := 0; i < lay.LineCount(); i++ {
				if _, _, w, _, ok := lay.Line(i); ok && w > maxX {
					maxX = w
				}
			}
			totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
			maxY = totalH - visH
			if maxY < 0 {
				maxY = 0
			}
		}
		maxScrollX := maxX - visW + b.caretMargin()
		if maxScrollX < 0 {
			maxScrollX = 0
		}
		dx, dy := 0.0, 0.0
		if b.dragLastX < abs.X+pad && b.scrollX > 0 {
			dx = -28
		} else if b.dragLastX > abs.X+w-pad && b.scrollX < maxScrollX {
			dx = 28
		}
		if b.dragLastY < abs.Y+pad && b.scrollY > 0 {
			dy = -14
		} else if b.dragLastY > abs.Y+h-pad && b.scrollY < maxY {
			dy = 14
		}
		if dx != 0 || dy != 0 {
			b.scrollBy(dx, dy, scrollMetrics{maxW: maxX, totalH: maxY + visH, visW: visW, visH: visH, pad: pad, multi: true, margin: b.caretMargin()})
			localX := b.dragLastX - abs.X - b.txt.Offset().X
			localY := b.dragLastY - abs.Y - b.txt.Offset().Y
			var byteOff int
			if lay != nil {
				byteOff, _ = lay.GetPositionForOffset(localX, localY)
			} else {
				byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
			}
			b.setDragSelection(byteOff)
		}
	} else if b.useViewport {
		visW := b.FixedWidth - 2 - 2*pad
		if visW < 0 {
			visW = 0
		}
		layTmp := b.txt.TextLayout()
		maxX := 0.0
		if layTmp != nil && layTmp.LineCount() > 0 {
			_, _, maxX, _, _ = layTmp.Line(0)
		}
		maxScroll := maxX - visW + b.caretMargin()
		if maxScroll < 0 {
			maxScroll = 0
		}
		did := false
		m := scrollMetrics{maxW: maxX, visW: visW, pad: pad, margin: b.caretMargin()}
		if b.dragLastX < abs.X+pad && b.scrollX > 0 {
			b.scrollBy(-28, 0, m)
			did = true
		} else if b.dragLastX > abs.X+b.FixedWidth-pad && b.scrollX < maxScroll {
			b.scrollBy(28, 0, m)
			did = true
		}
		if did {
			localX := b.dragLastX - abs.X - 1 - pad + b.scrollX
			localY := b.dragLastY - abs.Y - 1 + b.Viewport.ScrollOffset().Y - b.txt.Offset().Y
			lay := b.txt.TextLayout()
			var byteOff int
			if lay != nil {
				byteOff, _ = lay.GetPositionForOffset(localX, localY)
			} else {
				byteOff = b.txt.ByteOffsetAtPoint(localX, localY)
			}
			b.setDragSelection(byteOff)
		}
	} else {
		visW := b.FixedWidth - 2*pad
		if visW < 0 {
			visW = 0
		}
		layTmp := b.txt.TextLayout()
		maxX := 0.0
		if layTmp != nil && layTmp.LineCount() > 0 {
			_, _, maxX, _, _ = layTmp.Line(0)
		}
		maxScroll := maxX - visW + b.caretMargin()
		if maxScroll < 0 {
			maxScroll = 0
		}
		did := false
		if b.dragLastX < abs.X+pad && b.scrollX > 0 {
			b.scrollBy(-28, 0, scrollMetrics{maxW: maxX, visW: visW, pad: pad, textY: b.txt.Offset().Y, margin: b.caretMargin()})
			did = true
		} else if b.dragLastX > abs.X+b.FixedWidth-pad && b.scrollX < maxScroll {
			b.scrollBy(28, 0, scrollMetrics{maxW: maxX, visW: visW, pad: pad, textY: b.txt.Offset().Y, margin: b.caretMargin()})
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
			b.setDragSelection(byteOff)
		}
	}
	if b.dragging {
		time.AfterFunc(50*time.Millisecond, func() { b.doAutoScroll() })
	} else {
		b.autoScrollRunning = false
	}
}

func (b *Box) startAutoScroll() {
	if b == nil || b.autoScrollRunning {
		return
	}
	b.autoScrollRunning = true
	time.AfterFunc(50*time.Millisecond, func() { b.doAutoScroll() })
}

func (b *Box) OnText(ev input.TextEvent) {
	if b == nil || b.Disabled() {
		return
	}
}

func (b *Box) OnIME(ev input.IMEEvent) {
	if b == nil || b.Disabled() {
		return
	}
}

// syncBlinkRegistration syncs framework-owned blink registration with focus
// transitions: focused boxes receive dt from the embedder blink pump (which
// frames only on toggles via dirtiness); unfocused boxes cost nothing.
func syncBlinkRegistration(box *rendering.RenderBox, focused bool, bl rendering.Blinkable) {
	if box == nil || bl == nil {
		return
	}
	o := box.Owner()
	if o == nil {
		return
	}
	if focused {
		o.NoteBlinkable(bl)
	} else {
		o.DropBlinkable(bl)
	}
}

// blinkHalfPeriod is the caret on/off dwell, matching Flutter's 500ms blink
// half period. A full blink cycle is twice this.
const blinkHalfPeriod = 0.5

// stepBlink advances caret blink by dt seconds and reports whether the
// visible state changed. Oversized steps count every crossed half period,
// so the state keeps true parity and the remainder preserves the cadence.
func stepBlink(caretOn bool, elapsed, dt float64) (bool, float64, bool) {
	if dt < 0 {
		dt = 0
	}
	elapsed += dt
	n := int(elapsed / blinkHalfPeriod)
	if n <= 0 {
		return caretOn, elapsed, false
	}
	elapsed -= float64(n) * blinkHalfPeriod
	if n%2 == 0 {
		return caretOn, elapsed, false
	}
	return !caretOn, elapsed, true
}

// nextBlinkIn reports how long until the current blink phase ends.
func nextBlinkIn(elapsed float64) (time.Duration, bool) {
	d := blinkHalfPeriod - elapsed
	if d < 0 {
		d = 0
	}
	return time.Duration(d * float64(time.Second)), true
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// utf16ForRuneIndex maps a rune index in s to a utf16 offset.
func utf16ForRuneIndex(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	off := 0
	i := 0
	for _, r := range s {
		if i >= runeIdx {
			break
		}
		if r > 0xFFFF {
			off += 2
		} else {
			off++
		}
		i++
	}
	return off
}
