package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Tag is Ant Design Tag (filled/outlined label chip).
// https://ant.design/components/tag
type Tag struct {
	Root  *primitive.Decorated
	label *primitive.Text

	Value    string
	Color    render.RGBA // fill when A>0
	Bordered bool
	Closable bool
	Face     text.Face
	Theme    *core.Theme
	OnClose  func()
}

// NewTag creates a tag.
func NewTag(value string) *Tag {
	t := &Tag{Value: value, Bordered: true}
	t.rebuild()
	return t
}

// Node returns the root.
func (t *Tag) Node() core.Node {
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// SetValue updates label text.
func (t *Tag) SetValue(v string) {
	t.Value = v
	if t.label != nil {
		t.label.Value = v
		t.label.MarkNeedsPaint()
	}
}

// SetFace sets font face.
func (t *Tag) SetFace(face text.Face) {
	t.Face = face
	if t.label != nil {
		t.label.Face = face
	}
}

// SetClosable toggles the close button and rebuilds.
func (t *Tag) SetClosable(on bool) {
	t.Closable = on
	t.rebuild()
}

// SetColor sets fill color and rebuilds.
func (t *Tag) SetColor(c render.RGBA) {
	t.Color = c
	t.rebuild()
}

func (t *Tag) theme() *core.Theme {
	if t.Theme != nil {
		return t.Theme
	}
	return DefaultTheme()
}

func (t *Tag) rebuild() {
	th := t.theme()
	t.label = primitive.NewText(t.Value)
	t.label.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
	t.label.Face = t.Face
	t.label.Color = th.Color(core.TokenColorText)

	row := primitive.Row(t.label)
	row.CrossAlign = core.CrossCenter
	row.Gap = 4
	if t.Closable {
		x := NewButton("×")
		x.SetType(ButtonText)
		x.SetFace(t.Face)
		x.SetOnClick(func() {
			if t.OnClose != nil {
				t.OnClose()
			}
		})
		row.AddChild(x.Node())
	}

	if t.Root == nil {
		t.Root = primitive.NewDecorated(row)
	} else {
		t.Root.ClearChildren()
		t.Root.AddChild(row)
	}
	t.Root.Padding = primitive.Symmetric(7, 1)
	t.Root.Radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
	if t.Bordered {
		t.Root.BorderWidth = 1
		t.Root.BorderColor = th.Color(core.TokenColorBorder)
	}
	if t.Color.A > 0 {
		t.Root.Background = t.Color
	} else {
		t.Root.Background = th.Color(core.TokenColorBgContainer)
	}
	t.Root.Hit = core.HitDefer
	t.Root.MarkNeedsLayout()
	t.Root.MarkNeedsPaint()
}
