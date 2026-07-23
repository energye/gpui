package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Text is a product text node (typography baseline for B0).
// Supports F6 overflow: MaxWidth / MaxLines / Ellipsis via primitive.Text.
type Text struct {
	Root  *primitive.Text
	Value string
	// Secondary uses colorTextSecondary.
	Secondary bool
	Face      text.Face
	FontSize  float64
	Theme     *core.Theme
	// MaxWidth caps layout width (0 = unconstrained preferred).
	MaxWidth float64
	// MaxLines wrap budget (≤1 single line). 0 = single line.
	MaxLines int
	// Ellipsis truncates overflow with "…" (needs MaxWidth or tight parent).
	Ellipsis bool
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

// SetEllipsis enables overflow ellipsis.
func (t *Text) SetEllipsis(on bool) {
	t.Ellipsis = on
	if t.Root != nil {
		t.Root.SetEllipsis(on)
	}
}

// SetMaxWidth sets the preferred max width for ellipsis/wrap.
func (t *Text) SetMaxWidth(w float64) {
	t.MaxWidth = w
	if t.Root != nil {
		t.Root.MaxWidth = w
		t.Root.MarkNeedsLayout()
	}
}

// SetMaxLines sets wrap line budget.
func (t *Text) SetMaxLines(n int) {
	t.MaxLines = n
	if t.Root != nil {
		t.Root.SetMaxLines(n)
	}
}

func (t *Text) theme() *core.Theme {
	var n core.Node
	if t.Root != nil {
		n = t.Root
	}
	return themeOf(t.Theme, n)
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
	t.Root.MaxWidth = t.MaxWidth
	t.Root.MaxLines = t.MaxLines
	t.Root.Ellipsis = t.Ellipsis
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
