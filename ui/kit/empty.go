package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Empty is Ant Design Empty placeholder.
// https://ant.design/components/empty
type Empty struct {
	Root        *primitive.Flex
	Description string
	Face        text.Face
	Theme       *core.Theme
}

// NewEmpty creates an empty state with description.
func NewEmpty(description string) *Empty {
	if description == "" {
		description = "No data"
	}
	e := &Empty{Description: description}
	e.rebuild()
	return e
}

// Node returns root.
func (e *Empty) Node() core.Node {
	if e.Root == nil {
		e.rebuild()
	}
	return e.Root
}

// SetFace sets font.
func (e *Empty) SetFace(face text.Face) {
	e.Face = face
	e.rebuild()
}

func (e *Empty) rebuild() {
	th := DefaultTheme()
	if e.Theme != nil {
		th = e.Theme
	}
	icon := primitive.NewText("∅")
	icon.FontSize = 32
	icon.Face = e.Face
	icon.Color = th.Color(core.TokenColorTextSecondary)
	if icon.Color.A < 0.1 {
		icon.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	desc := primitive.NewText(e.Description)
	desc.FontSize = 14
	desc.Face = e.Face
	desc.Color = th.Color(core.TokenColorTextSecondary)
	if desc.Color.A < 0.1 {
		desc.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	e.Root = primitive.Column(icon, desc)
	e.Root.Gap = 8
	e.Root.CrossAlign = core.CrossCenter
	e.Root.MainAlign = core.MainCenter
	e.Root.Padding = primitive.All(24)
}
