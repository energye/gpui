package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Alert is Ant Design Alert banner.
// https://ant.design/components/alert
type Alert struct {
	Root    *primitive.Decorated
	message *primitive.Text

	Message     string
	Description string
	// Type: info (default), success, warning, error
	Type     string
	ShowIcon bool
	Closable bool
	Face     text.Face
	Theme    *core.Theme
	OnClose  func()
}

// NewAlert creates an info alert.
func NewAlert(message string) *Alert {
	a := &Alert{Message: message, Type: "info", ShowIcon: true}
	a.rebuild()
	return a
}

// Node returns root.
func (a *Alert) Node() core.Node {
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// SetMessage updates text.
func (a *Alert) SetMessage(s string) {
	a.Message = s
	if a.message != nil {
		a.message.Value = s
		a.message.MarkNeedsPaint()
	}
}

// SetType sets semantic type.
func (a *Alert) SetType(typ string) {
	a.Type = typ
	a.rebuild()
}

// SetDescription sets secondary description text.
func (a *Alert) SetDescription(s string) {
	a.Description = s
	a.rebuild()
}

// SetClosable toggles the close button.
func (a *Alert) SetClosable(v bool) {
	a.Closable = v
	a.rebuild()
}

// SetFace sets font.
func (a *Alert) SetFace(face text.Face) {
	a.Face = face
	if a.message != nil {
		a.message.Face = face
	}
}

func (a *Alert) theme() *core.Theme {
	var n core.Node
	if a.Root != nil {
		n = a.Root
	}
	return themeOf(a.Theme, n)
}

func (a *Alert) rebuild() {
	th := a.theme()
	a.message = primitive.NewText(a.Message)
	a.message.FontSize = th.SizeOr(core.TokenFontSize, 14)
	a.message.Face = a.Face
	a.message.Color = th.Color(core.TokenColorText)

	row := primitive.Row()
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	if a.ShowIcon {
		icon := primitive.NewText(alertIcon(a.Type))
		icon.FontSize = 14
		icon.Face = a.Face
		icon.Color = alertColor(th, a.Type)
		row.AddChild(icon)
	}
	if a.Description != "" {
		col := primitive.Column(a.message)
		col.Gap = 4
		desc := primitive.NewText(a.Description)
		desc.FontSize = 12
		desc.Face = a.Face
		desc.Color = th.Color(core.TokenColorTextSecondary)
		col.AddChild(desc)
		row.AddChild(col)
	} else {
		row.AddChild(a.message)
	}
	if a.Closable {
		cl := NewButton("×")
		cl.SetType(ButtonText)
		cl.SetFace(a.Face)
		cl.SetOnClick(func() {
			if a.OnClose != nil {
				a.OnClose()
			}
		})
		row.AddChild(primitive.Spacer())
		row.AddChild(cl.Node())
	}

	a.Root = primitive.NewDecorated(row)
	a.Root.Padding = primitive.Symmetric(12, 8)
	a.Root.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	a.Root.BorderWidth = 1
	col := alertColor(th, a.Type)
	a.Root.BorderColor = col
	// Light wash background
	a.Root.Background = render.RGBA{R: col.R, G: col.G, B: col.B, A: 0.08}
	a.Root.Hit = core.HitBlock
}
