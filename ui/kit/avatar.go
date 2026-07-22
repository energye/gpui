package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Avatar is a circular initials/icon chip.
// https://ant.design/components/avatar
type Avatar struct {
	Root  *primitive.Decorated
	label *primitive.Text
	Text  string
	Size  float64
	Face  text.Face
	Theme *core.Theme
}

// NewAvatar creates an avatar with text (initials).
func NewAvatar(text string) *Avatar {
	a := &Avatar{Text: text, Size: 32}
	a.rebuild()
	return a
}

// Node returns root.
func (a *Avatar) Node() core.Node {
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// SetFace sets font.
func (a *Avatar) SetFace(face text.Face) {
	a.Face = face
	if a.label != nil {
		a.label.Face = face
	}
}

func (a *Avatar) rebuild() {
	th := DefaultTheme()
	if a.Theme != nil {
		th = a.Theme
	}
	sz := a.Size
	if sz <= 0 {
		sz = 32
	}
	a.label = primitive.NewText(a.Text)
	a.label.FontSize = sz * 0.4
	if a.label.FontSize < 10 {
		a.label.FontSize = 10
	}
	a.label.Face = a.Face
	a.label.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	a.Root = primitive.NewDecorated(a.label)
	a.Root.Width, a.Root.Height = sz, sz
	a.Root.MinWidth, a.Root.MinHeight = sz, sz
	a.Root.Radius = sz / 2
	a.Root.Background = th.Color(core.TokenColorPrimary)
	if a.Root.Background.A < 0.5 {
		a.Root.Background = render.Hex("#1677FF")
	}
	a.Root.SetCenterContent(true)
	a.Root.StretchChild = true
	a.Root.Hit = core.HitDefer
}
