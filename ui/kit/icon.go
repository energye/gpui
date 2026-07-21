package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Icon is a product icon node wrapping primitive.Icon with theme color.
type Icon struct {
	Root *primitive.Icon
	Name string
	Size float64
	// Color overrides theme text color when A>0.
	Color render.RGBA
	Theme *core.Theme
}

// NewIcon creates a kit icon by registry name.
func NewIcon(name string) *Icon {
	ic := &Icon{Name: name, Size: 16}
	ic.rebuild()
	return ic
}

// Node returns the root node.
func (ic *Icon) Node() core.Node {
	if ic.Root == nil {
		ic.rebuild()
	}
	return ic.Root
}

// SetName changes the icon.
func (ic *Icon) SetName(name string) {
	ic.Name = name
	if ic.Root != nil {
		ic.Root.Name = name
		ic.Root.MarkNeedsPaint()
	}
}

// SetSize changes icon size.
func (ic *Icon) SetSize(s float64) {
	ic.Size = s
	if ic.Root != nil {
		ic.Root.Size = s
		ic.Root.MarkNeedsLayout()
	}
}

func (ic *Icon) theme() *core.Theme {
	if ic.Theme != nil {
		return ic.Theme
	}
	return DefaultTheme()
}

func (ic *Icon) rebuild() {
	ic.Root = primitive.NewIcon(ic.Name)
	if ic.Size > 0 {
		ic.Root.Size = ic.Size
	}
	if ic.Color.A > 0 {
		ic.Root.Color = ic.Color
	} else {
		ic.Root.Color = ic.theme().Color(core.TokenColorText)
	}
}
