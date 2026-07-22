package primitive

import (
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// EditableText is a single/multi-line text editor kernel (C-Edit).
// Supports caret, basic selection collapse, backspace/delete, arrows, and IME preedit.
type EditableText struct {
	core.NodeBase

	Value       string
	Placeholder string
	// Cursor is a rune index into Value.
	Cursor int
	// SelAnchor is the other end of selection; equal to Cursor means no selection.
	SelAnchor int

	Multiline bool
	ReadOnly  bool
	Disabled  bool
	MaxLength int
	// Width/Height preferred; 0 → intrinsic / expand.
	Width, Height    float64
	FontSize         float64
	Face             text.Face
	Color            render.RGBA
	PlaceholderColor render.RGBA
	CaretColor       render.RGBA

	// IME preedit (not yet committed).
	preedit string

	focused bool

	OnChange      func(value string)
	OnSubmit      func(value string) // Enter when !Multiline
	OnFocusChange func(focused bool)
	// OnHoverChange is invoked when pointer hover enters/leaves (for kit Input border).
	OnHoverChange func(hovered bool)
	hovered       bool

	// Caret blink state (demand-frame ticker).
	caretPhase   float64
	caretVisible bool
	boundTree    *core.Tree

	// ShowFocusRing draws an inner focus ring (default true).
	// Kit Input sets this false and uses Decorated border as focus chrome.
	ShowFocusRing bool
}

// NewEditableText creates an empty editor.
func NewEditableText() *EditableText {
	e := &EditableText{
		FontSize:         14,
		Color:            render.RGBA{R: 0, G: 0, B: 0, A: 0.88},
		PlaceholderColor: render.RGBA{R: 0, G: 0, B: 0, A: 0.25},
		CaretColor:       render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 1},
		ShowFocusRing:    true,
		caretVisible:     true,
	}
	e.Init(e)
	e.Hit = core.HitTarget
	e.Base().Cursor = core.CursorText
	return e
}

// TypeID implements core.Node.
func (e *EditableText) TypeID() string { return TypeEditableText }

// SetValue replaces content and moves the caret to the end.
func (e *EditableText) SetValue(v string) {
	if e.Value == v {
		return
	}
	e.Value = v
	e.Cursor = utf8.RuneCountInString(e.Value)
	e.SelAnchor = e.Cursor
	e.MarkNeedsPaint()
	if e.OnChange != nil {
		e.OnChange(e.Value)
	}
}

// Layout implements core.Node.
func (e *EditableText) Layout(c core.Constraints) core.Size {
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	lineH := fs * 1.3
	w, h := e.Width, e.Height
	if w <= 0 {
		// Prefer expanding when constrained.
		if c.HasBoundedWidth() {
			w = c.MaxWidth
		} else {
			w = measureTextWidth(e.Face, e.displayText(), fs)
			if w < 40 {
				w = 40
			}
		}
	}
	if h <= 0 {
		if e.Multiline {
			lines := 1 + strings.Count(e.Value, "\n")
			h = lineH * float64(lines)
			if h < lineH*2 {
				h = lineH * 2
			}
		} else {
			h = lineH
		}
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	e.SetSize(out)
	return out
}

func (e *EditableText) displayText() string {
	if e.Value == "" && e.preedit == "" {
		return e.Placeholder
	}
	if e.preedit != "" {
		// Insert preedit at caret for display.
		return insertAtRune(e.Value, e.Cursor, e.preedit)
	}
	return e.Value
}

// Paint implements core.Node.
func (e *EditableText) Paint(pc *core.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	dc := pc.DC
	if e.Face != nil {
		dc.SetFont(faceAtSize(e.Face, e.FontSize))
	}
	show := e.Value
	col := e.Color
	if show == "" && e.preedit == "" {
		show = e.Placeholder
		col = e.PlaceholderColor
	} else if e.preedit != "" {
		show = insertAtRune(e.Value, e.Cursor, e.preedit)
	}
	if pc.Theme != nil && col.A == 0 {
		col = pc.Theme.Color(core.TokenColorText)
	}
	dc.SetRGBA(col.R, col.G, col.B, col.A)
	ascent := fs * 0.8
	descent := fs * 0.2
	if e.Face != nil {
		m := faceAtSize(e.Face, fs).Metrics()
		ascent = m.Ascent
		if m.Descent > 0 {
			descent = m.Descent
		}
	}
	// Single-line: vertically center in allocated height (Input placeholder).
	lineH := ascent + descent
	if lineH <= 0 {
		lineH = fs * 1.2
	}
	y0 := pc.Origin.Y + ascent
	if !e.Multiline {
		sz := e.Size()
		if sz.Height > lineH {
			y0 = pc.Origin.Y + (sz.Height-lineH)/2 + ascent
		}
	}
	y := y0
	for i, line := range strings.Split(show, "\n") {
		if i > 0 {
			y += fs * 1.3
		}
		dc.DrawString(line, pc.Origin.X, y)
	}
	// Preedit underline so composition is visible (C2).
	if e.preedit != "" && e.focused {
		preX := measureTextWidth(e.Face, runePrefix(e.Value, e.Cursor), fs)
		preW := measureTextWidth(e.Face, e.preedit, fs)
		if preW < 2 {
			preW = 2
		}
		ul := e.CaretColor
		if pc.Theme != nil {
			if c := pc.Theme.Color(core.TokenColorPrimary); c.A > 0 {
				ul = c
			}
		}
		pc.FillLocalRect(preX, fs*1.05, preW, 1.5, ul)
	}
	// Caret
	if e.focused && !e.Disabled && !e.ReadOnly && e.caretVisible {
		cx := measureTextWidth(e.Face, runePrefix(e.Value, e.Cursor)+e.preedit, fs)
		// account for newlines before caret roughly on first line only for M2
		if !e.Multiline || !strings.Contains(e.Value[:byteIndex(e.Value, e.Cursor)], "\n") {
			cc := e.CaretColor
			if pc.Theme != nil {
				if c := pc.Theme.Color(core.TokenColorPrimary); c.A > 0 {
					cc = c
				}
			}
			// Local Y matches vertically-centered text (same as DrawString baseline path).
			caretTop := (y0 - pc.Origin.Y) - ascent
			if caretTop < 0 {
				caretTop = 0
			}
			pc.FillLocalRect(cx, caretTop, 1.5, lineH, cc)
		}
	}
	// Optional inner focus ring (kit Input uses outer Decorated border instead).
	if e.focused && e.ShowFocusRing {
		sz := e.Size()
		PaintFocusRing(pc, sz.Width, sz.Height, 2, 1, 1.5)
	}
}

// SetHovered implements tree hoverable — used for kit Input hover border.
func (e *EditableText) SetHovered(h bool) {
	if e.hovered == h {
		return
	}
	e.hovered = h
	if e.OnHoverChange != nil {
		e.OnHoverChange(h)
	}
}

// IsHovered reports hover state.
func (e *EditableText) IsHovered() bool { return e != nil && e.hovered }

// HitTest implements core.Node.
func (e *EditableText) HitTest(p core.Point) core.Node {
	if e.Disabled {
		return nil
	}
	if e.LocalBounds().Contains(p) {
		return e
	}
	return nil
}

// CanFocus implements core.FocusTarget.
func (e *EditableText) CanFocus() bool { return !e.Disabled }

// SetFocused implements core.FocusTarget.
func (e *EditableText) SetFocused(f bool) {
	if e.focused == f {
		return
	}
	e.focused = f
	if !f {
		e.preedit = ""
		if e.boundTree != nil {
			e.boundTree.RemoveTicker(e)
		}
	} else {
		e.caretPhase = 0
		e.caretVisible = true
		if e.boundTree != nil {
			e.boundTree.AddTicker(e)
		}
	}
	e.MarkNeedsPaint()
	if e.OnFocusChange != nil {
		e.OnFocusChange(f)
	}
}

// AttachTicker registers caret blink on the demand-frame loop.
func (e *EditableText) AttachTicker(tr *core.Tree) {
	if e == nil || tr == nil {
		return
	}
	e.boundTree = tr
	if e.focused {
		tr.AddTicker(e)
	}
}

// Tick advances caret blink. Implements core.Ticker.
func (e *EditableText) Tick(dt float64) bool {
	if e == nil || !e.focused {
		return false
	}
	const period = 1.06
	e.caretPhase += dt / period
	if e.caretPhase >= 1 {
		e.caretPhase -= 1
	}
	vis := e.caretPhase < 0.5
	if vis != e.caretVisible {
		e.caretVisible = vis
		e.MarkNeedsPaint()
	}
	return true
}

// IsFocused reports focus.
func (e *EditableText) IsFocused() bool { return e.focused }

// HandlePointer focuses and places caret (end for M2 simplicity on click).
func (e *EditableText) HandlePointer(ev *core.PointerEvent) {
	if e.Disabled || ev == nil {
		return
	}
	if ev.Type == core.PointerDown {
		// Place caret by approximate x
		fs := e.FontSize
		if fs <= 0 {
			fs = 14
		}
		// local x
		// AbsoluteOffset not needed — event is absolute but HitTest already local
		// Pointer events are tree-absolute; approximate using size only → end
		e.Cursor = utf8.RuneCountInString(e.Value)
		e.SelAnchor = e.Cursor
		ev.Handled = true
		e.MarkNeedsPaint()
	}
}

// HandleKey implements core.KeyHandler.
func (e *EditableText) HandleKey(ev *core.KeyEvent) {
	if e.Disabled || e.ReadOnly || ev == nil || ev.Type != core.KeyDown {
		return
	}
	switch ev.Key {
	case "BackSpace", "Backspace":
		e.deleteBackward()
		ev.Handled = true
	case "Delete":
		e.deleteForward()
		ev.Handled = true
	case "Left", "ArrowLeft":
		if e.Cursor > 0 {
			e.Cursor--
			e.SelAnchor = e.Cursor
			e.MarkNeedsPaint()
		}
		ev.Handled = true
	case "Right", "ArrowRight":
		if e.Cursor < utf8.RuneCountInString(e.Value) {
			e.Cursor++
			e.SelAnchor = e.Cursor
			e.MarkNeedsPaint()
		}
		ev.Handled = true
	case "Home":
		e.Cursor = 0
		e.SelAnchor = 0
		e.MarkNeedsPaint()
		ev.Handled = true
	case "End":
		e.Cursor = utf8.RuneCountInString(e.Value)
		e.SelAnchor = e.Cursor
		e.MarkNeedsPaint()
		ev.Handled = true
	case "Enter", "Return":
		if e.Multiline {
			e.insertText("\n")
		} else if e.OnSubmit != nil {
			e.OnSubmit(e.Value)
		}
		ev.Handled = true
	case "Tab", "Shift+Tab", "ISO_Left_Tab":
		// let tree handle focus traversal
		return
	default:
		// printable via Text field if present
		if ev.Text != "" && ev.Key != "Text" {
			e.insertText(ev.Text)
			ev.Handled = true
		}
	}
}

// HandleTextInput implements core.TextInputHandler.
func (e *EditableText) HandleTextInput(ev *core.TextInputEvent) {
	if e.Disabled || e.ReadOnly || ev == nil || ev.Text == "" {
		return
	}
	e.preedit = ""
	e.insertText(ev.Text)
	ev.Handled = true
}

// HandleIME implements core.IMEHandler.
//
// Sequence (C2):
//   - preedit update: End=false, Text=current composition
//   - end without commit: End=true, Text="" → clear preedit
//   - end with commit text: End=true, Text=final → insert then clear
//   - separate EventText / TextInput may also commit (host-dependent)
func (e *EditableText) HandleIME(ev *core.IMECompositionEvent) {
	if e.Disabled || e.ReadOnly || ev == nil {
		return
	}
	if ev.End {
		final := ev.Text
		e.preedit = ""
		if final != "" {
			e.insertText(final)
		} else {
			e.MarkNeedsPaint()
		}
	} else {
		e.preedit = ev.Text
		e.MarkNeedsPaint()
	}
	ev.Handled = true
}

// Preedit returns the current IME preedit string (empty when none).
func (e *EditableText) Preedit() string { return e.preedit }

// CaretLocalPos returns the caret position in local coordinates (top of caret).
// Apps combine with AbsoluteOffset for host SetIMEPosition when CapIME is set.
func (e *EditableText) CaretLocalPos() (x, y float64) {
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	prefix := runePrefix(e.Value, e.Cursor) + e.preedit
	x = measureTextWidth(e.Face, prefix, fs)
	y = 0
	return x, y
}

func (e *EditableText) insertText(s string) {
	if s == "" {
		return
	}
	if e.MaxLength > 0 {
		cur := utf8.RuneCountInString(e.Value)
		add := utf8.RuneCountInString(s)
		if cur+add > e.MaxLength {
			// truncate
			r := []rune(s)
			remain := e.MaxLength - cur
			if remain <= 0 {
				return
			}
			s = string(r[:remain])
		}
	}
	e.Value = insertAtRune(e.Value, e.Cursor, s)
	e.Cursor += utf8.RuneCountInString(s)
	e.SelAnchor = e.Cursor
	e.MarkNeedsLayout()
	e.MarkNeedsPaint()
	if e.OnChange != nil {
		e.OnChange(e.Value)
	}
}

func (e *EditableText) deleteBackward() {
	if e.Cursor <= 0 {
		return
	}
	r := []rune(e.Value)
	r = append(r[:e.Cursor-1], r[e.Cursor:]...)
	e.Value = string(r)
	e.Cursor--
	e.SelAnchor = e.Cursor
	e.MarkNeedsLayout()
	e.MarkNeedsPaint()
	if e.OnChange != nil {
		e.OnChange(e.Value)
	}
}

func (e *EditableText) deleteForward() {
	r := []rune(e.Value)
	if e.Cursor >= len(r) {
		return
	}
	r = append(r[:e.Cursor], r[e.Cursor+1:]...)
	e.Value = string(r)
	e.SelAnchor = e.Cursor
	e.MarkNeedsLayout()
	e.MarkNeedsPaint()
	if e.OnChange != nil {
		e.OnChange(e.Value)
	}
}

func (e *EditableText) clampCursor() {
	n := utf8.RuneCountInString(e.Value)
	if e.Cursor < 0 {
		e.Cursor = 0
	}
	if e.Cursor > n {
		e.Cursor = n
	}
	if e.SelAnchor < 0 {
		e.SelAnchor = 0
	}
	if e.SelAnchor > n {
		e.SelAnchor = n
	}
}

func measureTextWidth(face text.Face, s string, fontSize float64) float64 {
	if fontSize <= 0 {
		fontSize = 14
	}
	if face != nil {
		// Re-derive face at fontSize so FontSize is not ignored when Face is set.
		return faceAtSize(face, fontSize).Advance(s)
	}
	return float64(utf8.RuneCountInString(s)) * fontSize * 0.5
}

func insertAtRune(s string, at int, ins string) string {
	r := []rune(s)
	if at < 0 {
		at = 0
	}
	if at > len(r) {
		at = len(r)
	}
	out := make([]rune, 0, len(r)+len([]rune(ins)))
	out = append(out, r[:at]...)
	out = append(out, []rune(ins)...)
	out = append(out, r[at:]...)
	return string(out)
}

func runePrefix(s string, n int) string {
	r := []rune(s)
	if n < 0 {
		n = 0
	}
	if n > len(r) {
		n = len(r)
	}
	return string(r[:n])
}

func byteIndex(s string, runeIdx int) int {
	if runeIdx <= 0 {
		return 0
	}
	i := 0
	for ri := 0; ri < runeIdx && i < len(s); ri++ {
		_, sz := utf8.DecodeRuneInString(s[i:])
		i += sz
	}
	return i
}
