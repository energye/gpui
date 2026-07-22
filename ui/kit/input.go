package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Input is a single-line text field (B1).
//
//	Decorated
//	  └─ Flex(Row)
//	       Prefix? · EditableText · Suffix?
//
// Focus chrome is the Decorated border (primary when focused); the inner
// EditableText focus ring is disabled so there is a single path with Decorated.
//
// Ant Design 5 middle: height 32, paddingInline 11, font 14, radius 6.
type Input struct {
	Root        *primitive.Decorated
	editor      *primitive.EditableText
	row         *primitive.Flex
	prefix      *primitive.Slot
	suffix      *primitive.Slot
	Value       string
	Placeholder string
	Disabled    bool
	ReadOnly    bool
	AllowClear  bool
	MaxLength   int
	Face        text.Face
	Theme       *core.Theme
	Style       Style
	OnChange    func(string)
	OnSubmit    func(string)
	focused     bool
	hovered     bool
}

// NewInput creates an Input with optional placeholder.
func NewInput(placeholder string) *Input {
	in := &Input{Placeholder: placeholder}
	in.rebuild()
	return in
}

// Node returns the root node.
func (in *Input) Node() core.Node {
	if in.Root == nil {
		in.rebuild()
	}
	return in.Root
}

// Editor returns the inner EditableText.
func (in *Input) Editor() *primitive.EditableText { return in.editor }

// SetValue updates the text.
func (in *Input) SetValue(v string) {
	in.Value = v
	if in.editor != nil {
		in.editor.SetValue(v)
	}
}

// SetDisabled toggles disabled.
func (in *Input) SetDisabled(d bool) {
	in.Disabled = d
	if in.editor != nil {
		in.editor.Disabled = d
	}
	in.applyChrome()
}

// SetPrefix sets a leading node (icon/text).
func (in *Input) SetPrefix(n core.Node) {
	if in.prefix != nil {
		in.prefix.SetChild(n)
	}
}

// SetSuffix sets a trailing node.
func (in *Input) SetSuffix(n core.Node) {
	if in.suffix != nil {
		in.suffix.SetChild(n)
	}
}

// SetOnChange sets the change callback.
func (in *Input) SetOnChange(fn func(string)) {
	in.OnChange = fn
	if in.editor != nil {
		in.editor.OnChange = fn
	}
}

// SetFace sets the font face.
func (in *Input) SetFace(face text.Face) {
	in.Face = face
	in.Style.Face = face
	if in.editor != nil {
		in.editor.Face = face
	}
}

// SetStyle applies visual overrides.
func (in *Input) SetStyle(st Style) {
	in.Style = st
	if st.Face != nil {
		in.SetFace(st.Face)
	}
	if st.FontSize > 0 || st.Height > 0 || st.Width > 0 {
		in.rebuild()
		return
	}
	in.applyChrome()
}

// SetBackground overrides fill.
func (in *Input) SetBackground(c render.RGBA) {
	in.Style.Background = c
	in.applyChrome()
}

// SetTextColor overrides editor text color.
func (in *Input) SetTextColor(c render.RGBA) {
	in.Style.Text = c
	in.applyChrome()
}

// SetFontSize overrides editor font size.
func (in *Input) SetFontSize(px float64) {
	in.Style.FontSize = px
	in.rebuild()
}

// SetFixedSize forces outer chrome size (visual scenarios / forms).
func (in *Input) SetFixedSize(w, h float64) {
	if in.Root == nil {
		in.rebuild()
	}
	in.Root.Width = w
	in.Root.Height = h
	if h > 0 {
		in.Root.MinHeight = h
	}
	if w > 0 {
		in.Root.MinWidth = w
	}
	in.Root.MarkNeedsLayout()
}

// IsFocused reports whether the inner editor is focused.
func (in *Input) IsFocused() bool { return in.focused }

func (in *Input) theme() *core.Theme {
	if in.Theme != nil {
		return in.Theme
	}
	return DefaultTheme()
}

func (in *Input) rebuild() {
	th := in.theme()
	h := th.SizeOr(core.TokenControlHeight, 32)
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	padV := 0.0 // fixed height + content centering
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	lineW := th.SizeOr(core.TokenLineWidth, 1)
	fontSize := th.SizeOr(core.TokenFontSize, 14)
	if in.Style.FontSize > 0 {
		fontSize = in.Style.FontSize
	}
	if in.Style.Height > 0 {
		h = in.Style.Height
	}

	in.editor = primitive.NewEditableText()
	in.editor.Placeholder = in.Placeholder
	in.editor.Value = in.Value
	in.editor.Disabled = in.Disabled
	in.editor.ReadOnly = in.ReadOnly
	in.editor.MaxLength = in.MaxLength
	in.editor.Face = in.Face
	in.editor.FontSize = fontSize
	in.editor.ShowFocusRing = false // outer Decorated border is focus chrome
	in.editor.Color = th.Color(core.TokenColorText)
	in.editor.PlaceholderColor = th.Color(core.TokenColorTextSecondary)
	if in.editor.PlaceholderColor.A < 0.15 {
		in.editor.PlaceholderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	in.editor.OnChange = func(v string) {
		in.Value = v
		if in.OnChange != nil {
			in.OnChange(v)
		}
	}
	in.editor.OnSubmit = in.OnSubmit
	in.editor.OnFocusChange = func(f bool) {
		in.focused = f
		in.applyChrome()
	}
	in.editor.OnHoverChange = func(h bool) {
		in.hovered = h
		in.applyChrome()
	}

	// Expand editor in flex
	flexEd := primitive.NewFlexible(1, in.editor)

	in.prefix = primitive.NewSlot("prefix", nil)
	in.suffix = primitive.NewSlot("suffix", nil)
	in.row = primitive.Row(in.prefix, flexEd, in.suffix)
	in.row.Gap = th.SizeOr(core.TokenMarginXS, 4) + 2 // ~6
	in.row.CrossAlign = core.CrossCenter

	in.Root = primitive.NewDecorated(in.row)
	in.Root.Padding = primitive.Symmetric(padH, padV)
	in.Root.Radius = radius
	in.Root.MinHeight = h
	in.Root.Height = h
	in.Root.BorderWidth = lineW
	in.applyChrome()
}

func (in *Input) applyChrome() {
	if in.Root == nil {
		return
	}
	th := in.theme()
	in.Root.Background = th.Color(core.TokenColorBgContainer)
	if in.Style.hasBG() {
		in.Root.Background = in.Style.Background
	}
	in.Root.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	if in.Style.hasBorder() {
		in.Root.BorderColor = in.Style.Border
	}
	if in.Style.hasRadius() {
		in.Root.Radius = in.Style.Radius
	}
	if in.editor != nil && in.Style.hasText() {
		in.editor.Color = in.Style.Text
	}
	switch {
	case in.Disabled:
		in.Root.Background = th.Color(core.TokenColorDisabledBg)
		in.Root.BorderColor = th.Color(core.TokenColorBorder)
		if in.editor != nil {
			in.editor.Color = th.Color(core.TokenColorDisabledText)
		}
	case in.focused:
		// Primary border = focus state (Ant Input).
		in.Root.BorderColor = th.Color(core.TokenColorPrimary)
		if in.editor != nil {
			in.editor.Color = th.Color(core.TokenColorText)
		}
	case in.hovered:
		// Hover border (Ant colorPrimaryHover / colorBorderHover).
		bd := th.Color(core.TokenColorBorderHover)
		if bd.A < 0.5 {
			bd = th.Color(core.TokenColorPrimaryHover)
		}
		in.Root.BorderColor = bd
		if in.editor != nil {
			in.editor.Color = th.Color(core.TokenColorText)
		}
	default:
		in.Root.BorderColor = th.Color(core.TokenColorBorder)
		if in.editor != nil {
			in.editor.Color = th.Color(core.TokenColorText)
		}
	}
	in.Root.MarkNeedsPaint()
}

// TextArea is a multi-line Input.
type TextArea struct {
	*Input
	Rows int
}

// NewTextArea creates a multi-line field.
func NewTextArea(placeholder string, rows int) *TextArea {
	if rows < 2 {
		rows = 3
	}
	ta := &TextArea{Rows: rows}
	ta.Input = NewInput(placeholder)
	if ta.editor != nil {
		ta.editor.Multiline = true
		fs := ta.theme().SizeOr(core.TokenFontSize, 14)
		// Ant line-height ≈ 1.5714285714 (22/14)
		ta.editor.Height = fs * 1.5714285714 * float64(rows)
		if ta.Root != nil {
			ta.Root.Height = 0
			ta.Root.MinHeight = ta.editor.Height + 8
			ta.Root.SetCenterContent(false)
			ta.Root.Padding = primitive.Symmetric(
				ta.theme().SizeOr(core.TokenControlPaddingInline, 11),
				ta.theme().SizeOr(core.TokenPaddingXS, 4),
			)
		}
	}
	return ta
}
