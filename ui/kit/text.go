package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Text is a product text node (typography baseline for B0).
type Text struct {
	Root  *primitive.Text
	Value string
	// Secondary uses colorTextSecondary.
	Secondary bool
	Face      text.Face
	FontSize  float64
	Theme     *core.Theme
}

// NewText creates kit text with the given value.
func NewText(value string) *Text {
	t := &Text{Value: value}
	t.rebuild()
	return t
}

// Node returns the root node.
func (t *Text) Node() core.Node {
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// SetValue updates the string.
func (t *Text) SetValue(v string) {
	t.Value = v
	if t.Root != nil {
		t.Root.SetValue(v)
	}
}

// SetSecondary toggles secondary text color.
func (t *Text) SetSecondary(v bool) {
	t.Secondary = v
	t.applyColor()
}

// SetFace sets the font face.
func (t *Text) SetFace(face text.Face) {
	t.Face = face
	if t.Root != nil {
		t.Root.Face = face
	}
}

func (t *Text) theme() *core.Theme {
	if t.Theme != nil {
		return t.Theme
	}
	return DefaultTheme()
}

func (t *Text) rebuild() {
	th := t.theme()
	t.Root = primitive.NewText(t.Value)
	fs := t.FontSize
	if fs <= 0 {
		fs = th.SizeOr(core.TokenFontSize, 14)
	}
	t.Root.FontSize = fs
	t.Root.Face = t.Face
	t.applyColor()
}

func (t *Text) applyColor() {
	if t.Root == nil {
		return
	}
	th := t.theme()
	if t.Secondary {
		t.Root.Color = th.Color(core.TokenColorTextSecondary)
	} else {
		t.Root.Color = th.Color(core.TokenColorText)
	}
}
