package primitive

import (
	"math"
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

	// Pointer drag selection (F1).
	dragging bool

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

// LineMetricsForTest exposes line pitch for geometry tests.
func (e *EditableText) LineMetricsForTest() (ascent, descent, lineH float64) {
	if e == nil {
		return 0, 0, 0
	}
	return e.lineMetrics()
}

// lineMetrics returns ascent, descent, and line advance (row height).
func (e *EditableText) lineMetrics() (ascent, descent, lineH float64) {
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	ascent = fs * 0.8
	descent = fs * 0.2
	if e.Face != nil {
		m := faceAtSize(e.Face, fs).Metrics()
		ascent = m.Ascent
		if m.Descent > 0 {
			descent = m.Descent
		}
	}
	lineH = ascent + descent
	if lineH <= 0 {
		lineH = fs * 1.2
	}
	// Multiline uses slightly larger row pitch (matches previous paint fs*1.3).
	if e.Multiline {
		pitch := fs * 1.3
		if pitch > lineH {
			lineH = pitch
		}
	}
	return ascent, descent, lineH
}

// textOriginLocal returns the local top-left of the first line's glyph box
// (y = top of line, not baseline). Single-line vertically centers in height.
func (e *EditableText) textOriginLocal() (ox, oy float64) {
	ascent, _, lineH := e.lineMetrics()
	oy = 0
	if !e.Multiline {
		sz := e.Size()
		if sz.Height > lineH {
			oy = (sz.Height - lineH) / 2
		}
	}
	_ = ascent
	return 0, oy
}

// caretLocalAt returns local (x,yTop,lineH) for a rune cursor index into Value
// (preedit not included in index; painted separately after cursor).
func (e *EditableText) caretLocalAt(cursor int) (x, yTop, lineH float64) {
	ascent, _, lineH := e.lineMetrics()
	_, oy := e.textOriginLocal()
	r := []rune(e.Value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(r) {
		cursor = len(r)
	}
	// Count newlines before cursor → line index; column = runes after last \n.
	lineIdx := 0
	colStart := 0
	for i := 0; i < cursor; i++ {
		if r[i] == '\n' {
			lineIdx++
			colStart = i + 1
		}
	}
	prefix := string(r[colStart:cursor])
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	x = measureTextWidth(e.Face, prefix, fs)
	yTop = oy + float64(lineIdx)*lineH
	_ = ascent
	return x, yTop, lineH
}

// indexAtLocal maps a local point to a rune cursor index (click-to-caret).
func (e *EditableText) indexAtLocal(lx, ly float64) int {
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	_, oy := e.textOriginLocal()
	_, _, lineH := e.lineMetrics()
	if lineH < 1 {
		lineH = 1
	}
	lineIdx := int((ly - oy) / lineH)
	if lineIdx < 0 {
		lineIdx = 0
	}
	lines := strings.Split(e.Value, "\n")
	if len(lines) == 0 {
		return 0
	}
	if lineIdx >= len(lines) {
		lineIdx = len(lines) - 1
	}
	// Rune offset of line start.
	start := 0
	for i := 0; i < lineIdx; i++ {
		start += utf8.RuneCountInString(lines[i]) + 1 // + newline
	}
	line := lines[lineIdx]
	// Walk glyphs to find closest insert position.
	best := 0
	bestDist := math.MaxFloat64
	runes := []rune(line)
	for i := 0; i <= len(runes); i++ {
		w := measureTextWidth(e.Face, string(runes[:i]), fs)
		d := math.Abs(w - lx)
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return start + best
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
	ascent, _, lineH := e.lineMetrics()
	_, oy := e.textOriginLocal()

	// Selection highlight (before glyphs).
	if e.hasSelection() && e.focused {
		sel := render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 0.25}
		if pc.Theme != nil {
			if c := pc.Theme.Color(core.TokenColorPrimaryBg); c.A > 0 {
				sel = c
			}
		}
		lo, hi := e.selRange()
		// Paint per-line segments between lo and hi.
		r := []rune(e.Value)
		lineStart := 0
		lineIdx := 0
		for i := 0; i <= len(r); i++ {
			atEnd := i == len(r)
			atNL := !atEnd && r[i] == '\n'
			if atEnd || atNL {
				lineEnd := i
				// intersection of [lineStart,lineEnd) with [lo,hi)
				a := lineStart
				if a < lo {
					a = lo
				}
				b := lineEnd
				if b > hi {
					b = hi
				}
				if a < b {
					px0 := measureTextWidth(e.Face, string(r[lineStart:a]), fs)
					px1 := measureTextWidth(e.Face, string(r[lineStart:b]), fs)
					if px1 < px0+1 {
						px1 = px0 + 1
					}
					yTop := oy + float64(lineIdx)*lineH
					pc.FillLocalRect(px0, yTop, px1-px0, lineH, sel)
				}
				if atNL {
					lineStart = i + 1
					lineIdx++
				}
			}
		}
	}

	dc.SetRGBA(col.R, col.G, col.B, col.A)
	// Draw lines at baselines: top + ascent within each row.
	for i, line := range strings.Split(show, "\n") {
		yBase := pc.Origin.Y + oy + float64(i)*lineH + ascent
		dc.DrawString(line, pc.Origin.X, yBase)
	}
	// Preedit underline on caret line.
	if e.preedit != "" && e.focused {
		cx, yTop, _ := e.caretLocalAt(e.Cursor)
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
		pc.FillLocalRect(cx, yTop+lineH-2, preW, 1.5, ul)
	}
	// Caret (all lines).
	if e.focused && !e.Disabled && !e.ReadOnly && e.caretVisible {
		cx, yTop, lh := e.caretLocalAt(e.Cursor)
		// preedit shifts caret visually to end of composition
		if e.preedit != "" {
			cx += measureTextWidth(e.Face, e.preedit, fs)
		}
		cc := e.CaretColor
		if pc.Theme != nil {
			if c := pc.Theme.Color(core.TokenColorPrimary); c.A > 0 {
				cc = c
			}
		}
		pc.FillLocalRect(cx, yTop, 1.5, lh, cc)
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
		e.dragging = false
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

// AttachTicker registers caret blink on the demand-frame loop when focused.
func (e *EditableText) AttachTicker(tr *core.Tree) {
	if e == nil || tr == nil {
		return
	}
	e.boundTree = tr
	tr.BindTicker(e, e.focused)
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

// HandlePointer focuses, places caret, supports drag-select and Shift+click extend (F1).
//
//	PointerDown: click → caret (collapse) + start drag; Shift+click (when focused) → extend
//	PointerMove while dragging: update Cursor, keep SelAnchor
//	PointerUp / Cancel: end drag
func (e *EditableText) HandlePointer(ev *core.PointerEvent) {
	if e.Disabled || ev == nil {
		return
	}
	abs := core.AbsoluteOffset(e)
	lx := ev.X - abs.X
	ly := ev.Y - abs.Y
	idx := e.indexAtLocal(lx, ly)

	switch ev.Type {
	case core.PointerDown:
		if ev.Shift && e.focused {
			// Extend: keep SelAnchor, move Cursor only (no drag).
			e.Cursor = idx
			e.dragging = false
		} else {
			// New selection origin; tree capture delivers subsequent Move/Up here.
			e.Cursor = idx
			e.SelAnchor = idx
			e.dragging = true
		}
		ev.Handled = true
		e.MarkNeedsPaint()

	case core.PointerMove:
		if !e.dragging {
			return
		}
		if e.Cursor != idx {
			e.Cursor = idx
			e.MarkNeedsPaint()
		}
		ev.Handled = true

	case core.PointerUp, core.PointerCancel:
		if !e.dragging {
			return
		}
		e.Cursor = idx
		e.dragging = false
		ev.Handled = true
		e.MarkNeedsPaint()
	}
}

// IsDraggingSelection reports an active mouse drag selection (tests / chrome).
func (e *EditableText) IsDraggingSelection() bool {
	return e != nil && e.dragging
}

// CaretLocalPos returns the caret top-left in local coordinates (for IME position).
func (e *EditableText) CaretLocalPos() (x, y float64) {
	cx, yTop, _ := e.caretLocalAt(e.Cursor)
	if e.preedit != "" {
		fs := e.FontSize
		if fs <= 0 {
			fs = 14
		}
		cx += measureTextWidth(e.Face, e.preedit, fs)
	}
	return cx, yTop
}

// HandleKey implements core.KeyHandler.
func (e *EditableText) HandleKey(ev *core.KeyEvent) {
	if e.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	// Clipboard shortcuts (work even when ReadOnly for Copy).
	if e.handleClipboardKey(ev) {
		return
	}
	if e.ReadOnly {
		// navigation still allowed
		switch ev.Key {
		case "Left", "ArrowLeft", "Right", "ArrowRight", "Home", "End", "Up", "ArrowUp", "Down", "ArrowDown":
			// fall through to nav below without mutation when selection-only
		default:
			return
		}
	}
	switch ev.Key {
	case "BackSpace", "Backspace":
		if e.hasSelection() {
			e.deleteSelection()
		} else {
			e.deleteBackward()
		}
		ev.Handled = true
	case "Delete":
		if e.hasSelection() {
			e.deleteSelection()
		} else {
			e.deleteForward()
		}
		ev.Handled = true
	case "Left", "ArrowLeft":
		if e.extendSelection(ev) {
			if e.Cursor > 0 {
				e.Cursor--
			}
		} else if e.hasSelection() {
			lo, _ := e.selRange()
			e.Cursor, e.SelAnchor = lo, lo
		} else if e.Cursor > 0 {
			e.Cursor--
			e.SelAnchor = e.Cursor
		}
		e.MarkNeedsPaint()
		ev.Handled = true
	case "Right", "ArrowRight":
		n := utf8.RuneCountInString(e.Value)
		if e.extendSelection(ev) {
			if e.Cursor < n {
				e.Cursor++
			}
		} else if e.hasSelection() {
			_, hi := e.selRange()
			e.Cursor, e.SelAnchor = hi, hi
		} else if e.Cursor < n {
			e.Cursor++
			e.SelAnchor = e.Cursor
		}
		e.MarkNeedsPaint()
		ev.Handled = true
	case "Up", "ArrowUp":
		e.moveLine(-1, e.extendSelection(ev))
		ev.Handled = true
	case "Down", "ArrowDown":
		e.moveLine(1, e.extendSelection(ev))
		ev.Handled = true
	case "Home":
		// Multiline: go to start of current line; Ctrl+Home → document start (Ctrl handled above for clipboard).
		if e.Multiline && !ev.Ctrl {
			e.Cursor = e.lineStart(e.Cursor)
			if !e.extendSelection(ev) {
				e.SelAnchor = e.Cursor
			}
		} else {
			e.Cursor = 0
			if !e.extendSelection(ev) {
				e.SelAnchor = 0
			}
		}
		e.MarkNeedsPaint()
		ev.Handled = true
	case "End":
		if e.Multiline && !ev.Ctrl {
			e.Cursor = e.lineEnd(e.Cursor)
			if !e.extendSelection(ev) {
				e.SelAnchor = e.Cursor
			}
		} else {
			e.Cursor = utf8.RuneCountInString(e.Value)
			if !e.extendSelection(ev) {
				e.SelAnchor = e.Cursor
			}
		}
		e.MarkNeedsPaint()
		ev.Handled = true
	case "Enter", "Return":
		if e.ReadOnly {
			return
		}
		if e.Multiline {
			e.insertText("\n")
		} else if e.OnSubmit != nil {
			e.OnSubmit(e.Value)
		}
		ev.Handled = true
	case "Tab", "Shift+Tab", "ISO_Left_Tab":
		// let tree handle focus traversal
		return
	case "a", "A":
		// Ctrl/Cmd+A without modifiers field: accept "a" only if already handled clipboard
		return
	default:
		// printable via Text field if present
		if !e.ReadOnly && ev.Text != "" && ev.Key != "Text" {
			e.insertText(ev.Text)
			ev.Handled = true
		}
	}
}

// extendSelection reports whether Shift is held (key name or Modifiers).
func (e *EditableText) extendSelection(ev *core.KeyEvent) bool {
	if ev == nil {
		return false
	}
	// Key may be "Shift+Left" from some hosts; primary path uses Modifiers if present.
	if strings.Contains(ev.Key, "Shift") {
		return true
	}
	return ev.Shift
}

func (e *EditableText) handleClipboardKey(ev *core.KeyEvent) bool {
	if ev == nil {
		return false
	}
	// Accept Ctrl/Cmd via Modifiers, or composite key names.
	ctrl := ev.Ctrl || ev.Meta || strings.HasPrefix(ev.Key, "Ctrl+") || strings.HasPrefix(ev.Key, "Meta+")
	key := ev.Key
	if i := strings.LastIndex(key, "+"); i >= 0 {
		key = key[i+1:]
	}
	key = strings.ToLower(key)
	if !ctrl {
		// Select-all via explicit "Ctrl+A" only when ctrl; plain 'a' inserts.
		return false
	}
	switch key {
	case "a":
		e.selectAll()
		ev.Handled = true
		return true
	case "c":
		e.copySelection()
		ev.Handled = true
		return true
	case "x":
		if !e.ReadOnly {
			e.copySelection()
			e.deleteSelection()
		} else {
			e.copySelection()
		}
		ev.Handled = true
		return true
	case "v":
		if !e.ReadOnly {
			e.pasteClipboard()
		}
		ev.Handled = true
		return true
	}
	return false
}

func (e *EditableText) hasSelection() bool {
	return e != nil && e.SelAnchor != e.Cursor
}

func (e *EditableText) selRange() (lo, hi int) {
	lo, hi = e.SelAnchor, e.Cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo, hi
}

// SelectedText returns the current selection (empty if caret only).
func (e *EditableText) SelectedText() string {
	if e == nil || !e.hasSelection() {
		return ""
	}
	lo, hi := e.selRange()
	r := []rune(e.Value)
	if lo < 0 {
		lo = 0
	}
	if hi > len(r) {
		hi = len(r)
	}
	return string(r[lo:hi])
}

func (e *EditableText) selectAll() {
	e.SelAnchor = 0
	e.Cursor = utf8.RuneCountInString(e.Value)
	e.MarkNeedsPaint()
}

func (e *EditableText) clipboard() core.Clipboard {
	if e == nil {
		return nil
	}
	if t := e.Tree(); t != nil {
		return t.Clipboard()
	}
	if e.boundTree != nil {
		return e.boundTree.Clipboard()
	}
	return nil
}

func (e *EditableText) copySelection() {
	clip := e.clipboard()
	if clip == nil {
		return
	}
	text := e.SelectedText()
	if text == "" {
		// Copy all when no selection (common desktop convenience) — skip; only selection.
		return
	}
	_ = clip.WriteText(text)
}

func (e *EditableText) pasteClipboard() {
	clip := e.clipboard()
	if clip == nil {
		return
	}
	s, ok := clip.ReadText()
	if !ok || s == "" {
		return
	}
	e.insertText(s)
}

func (e *EditableText) deleteSelection() {
	if !e.hasSelection() {
		return
	}
	lo, hi := e.selRange()
	r := []rune(e.Value)
	if lo < 0 {
		lo = 0
	}
	if hi > len(r) {
		hi = len(r)
	}
	e.Value = string(append(r[:lo], r[hi:]...))
	e.Cursor = lo
	e.SelAnchor = lo
	e.MarkNeedsLayout()
	e.MarkNeedsPaint()
	if e.OnChange != nil {
		e.OnChange(e.Value)
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

func (e *EditableText) lineStart(cursor int) int {
	r := []rune(e.Value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(r) {
		cursor = len(r)
	}
	for i := cursor - 1; i >= 0; i-- {
		if r[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

func (e *EditableText) lineEnd(cursor int) int {
	r := []rune(e.Value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(r) {
		cursor = len(r)
	}
	for i := cursor; i < len(r); i++ {
		if r[i] == '\n' {
			return i
		}
	}
	return len(r)
}

// moveLine moves caret by delta lines, preserving column when possible.
func (e *EditableText) moveLine(delta int, extend bool) {
	if e == nil || delta == 0 {
		return
	}
	if !e.Multiline {
		if delta < 0 {
			e.Cursor = 0
		} else {
			e.Cursor = utf8.RuneCountInString(e.Value)
		}
		if !extend {
			e.SelAnchor = e.Cursor
		}
		e.MarkNeedsPaint()
		return
	}
	// Preferred column width in px from line start.
	fs := e.FontSize
	if fs <= 0 {
		fs = 14
	}
	ls := e.lineStart(e.Cursor)
	colRunes := e.Cursor - ls
	r := []rune(e.Value)
	prefW := measureTextWidth(e.Face, string(r[ls:e.Cursor]), fs)
	_ = colRunes

	// Find target line index.
	lineIdx := 0
	for i := 0; i < e.Cursor && i < len(r); i++ {
		if r[i] == '\n' {
			lineIdx++
		}
	}
	lineIdx += delta
	if lineIdx < 0 {
		lineIdx = 0
	}
	// Find start offset of target line.
	start := 0
	curLine := 0
	for i := 0; i < len(r); i++ {
		if curLine == lineIdx {
			start = i
			break
		}
		if r[i] == '\n' {
			curLine++
			if curLine == lineIdx {
				start = i + 1
				break
			}
		}
		if i == len(r)-1 && curLine < lineIdx {
			// past last line → end of document
			e.Cursor = len(r)
			if !extend {
				e.SelAnchor = e.Cursor
			}
			e.MarkNeedsPaint()
			return
		}
	}
	if lineIdx == 0 {
		start = 0
	}
	end := e.lineEnd(start)
	// Place at preferred width within line.
	best := start
	bestDist := math.MaxFloat64
	for i := start; i <= end; i++ {
		w := measureTextWidth(e.Face, string(r[start:i]), fs)
		d := math.Abs(w - prefW)
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	e.Cursor = best
	if !extend {
		e.SelAnchor = e.Cursor
	}
	e.MarkNeedsPaint()
}

func (e *EditableText) insertText(s string) {
	if s == "" {
		return
	}
	if e.hasSelection() {
		e.deleteSelection()
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
	if e.hasSelection() {
		e.deleteSelection()
		return
	}
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
