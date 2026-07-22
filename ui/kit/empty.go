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
	Image       core.Node
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

// SetDescription updates the description text.
func (e *Empty) SetDescription(s string) {
	e.Description = s
	e.rebuild()
}

// SetImage sets a custom illustration node.
func (e *Empty) SetImage(n core.Node) {
	e.Image = n
	e.rebuild()
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
	var icon core.Node
	if e.Image != nil {
		icon = e.Image
	} else {
		t := primitive.NewText("∅")
		t.FontSize = 32
		t.Face = e.Face
		t.Color = th.Color(core.TokenColorTextSecondary)
		if t.Color.A < 0.1 {
			t.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		icon = t
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
