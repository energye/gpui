package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Input is a single-line text field (B1).
//
//	Decorated
//	  └─ Flex(Row)
//	       Prefix? · EditableText · Suffix?
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
	OnChange    func(string)
	OnSubmit    func(string)
	focused     bool
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
	if in.editor != nil {
		in.editor.Face = face
	}
}

func (in *Input) theme() *core.Theme {
	if in.Theme != nil {
		return in.Theme
	}
	return DefaultTheme()
}

func (in *Input) rebuild() {
	th := in.theme()
	h := th.SizeOr(core.TokenControlHeight, 32)
	pad := th.SizeOr(core.TokenPaddingSM, 8)
	radius := th.SizeOr(core.TokenBorderRadius, 6)

	in.editor = primitive.NewEditableText()
	in.editor.Placeholder = in.Placeholder
	in.editor.Value = in.Value
	in.editor.Disabled = in.Disabled
	in.editor.ReadOnly = in.ReadOnly
	in.editor.MaxLength = in.MaxLength
	in.editor.Face = in.Face
	in.editor.FontSize = th.SizeOr(core.TokenFontSize, 14)
	in.editor.OnChange = func(v string) {
		in.Value = v
		if in.OnChange != nil {
			in.OnChange(v)
		}
	}
	in.editor.OnSubmit = in.OnSubmit
	// Expand editor in flex
	flexEd := primitive.NewFlexible(1, in.editor)

	in.prefix = primitive.NewSlot("prefix", nil)
	in.suffix = primitive.NewSlot("suffix", nil)
	in.row = primitive.Row(in.prefix, flexEd, in.suffix)
	in.row.Gap = 6
	in.row.CrossAlign = core.CrossCenter

	in.Root = primitive.NewDecorated(in.row)
	in.Root.Padding = primitive.Symmetric(pad, 4)
	in.Root.Radius = radius
	in.Root.MinHeight = h
	in.Root.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	in.applyChrome()
}

func (in *Input) applyChrome() {
	if in.Root == nil {
		return
	}
	th := in.theme()
	in.Root.Background = th.Color(core.TokenColorBgContainer)
	in.Root.BorderColor = th.Color(core.TokenColorBorder)
	if in.Disabled {
		in.Root.Background = th.Color(core.TokenColorDisabledBg)
		in.Root.BorderColor = th.Color(core.TokenColorBorder)
	}
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
		ta.editor.Height = fs * 1.3 * float64(rows)
		ta.Root.MinHeight = ta.editor.Height + 8
	}
	return ta
}
