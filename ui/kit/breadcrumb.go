package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Breadcrumb is Ant Design Breadcrumb.
// https://ant.design/components/breadcrumb
type Breadcrumb struct {
	Root      *primitive.Flex
	Items     []string
	Separator string // default "/"
	Face      text.Face
	Theme     *core.Theme
	OnClick   func(index int, item string)
}

// NewBreadcrumb creates a breadcrumb trail.
func NewBreadcrumb(items ...string) *Breadcrumb {
	b := &Breadcrumb{Items: append([]string(nil), items...)}
	b.rebuild()
	return b
}

// Node returns root.
func (b *Breadcrumb) Node() core.Node {
	if b.Root == nil {
		b.rebuild()
	}
	return b.Root
}

// SetFace sets font.
func (b *Breadcrumb) SetFace(face text.Face) {
	b.Face = face
	b.rebuild()
}

// SetItems replaces trail items.
func (b *Breadcrumb) SetItems(items []string) {
	b.Items = append([]string(nil), items...)
	b.rebuild()
}

// SetSeparator sets the separator between items (empty → "/").
func (b *Breadcrumb) SetSeparator(sep string) {
	b.Separator = sep
	b.rebuild()
}

func (b *Breadcrumb) rebuild() {
	th := DefaultTheme()
	if b.Theme != nil {
		th = b.Theme
	}
	sepStr := b.Separator
	if sepStr == "" {
		sepStr = "/"
	}
	if b.Root == nil {
		b.Root = primitive.Row()
	} else {
		b.Root.ClearChildren()
	}
	b.Root.Gap = 4
	b.Root.CrossAlign = core.CrossCenter
	for i, it := range b.Items {
		i, it := i, it
		lab := primitive.NewText(it)
		lab.FontSize = 14
		lab.Face = b.Face
		if i == len(b.Items)-1 {
			lab.Color = th.Color(core.TokenColorText)
			b.Root.AddChild(lab)
		} else {
			lab.Color = th.Color(core.TokenColorPrimary)
			p := primitive.NewPressable(lab)
			p.ShowFocusRing = false
			p.EnableRipple = false
			p.Click = func() {
				if b.OnClick != nil {
					b.OnClick(i, it)
				}
			}
			b.Root.AddChild(p)
			sep := primitive.NewText(sepStr)
			sep.FontSize = 14
			sep.Face = b.Face
			sep.Color = th.Color(core.TokenColorTextSecondary)
			b.Root.AddChild(sep)
		}
	}
}
