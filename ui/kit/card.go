package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Card is Ant Design Card shell (title + body).
// https://ant.design/components/card
type Card struct {
	Root    *primitive.Decorated
	title   *primitive.Text
	body    *primitive.Slot
	Title   string
	Face    text.Face
	Theme   *core.Theme
	content core.Node
}

// NewCard creates a card with title.
func NewCard(title string) *Card {
	c := &Card{Title: title}
	c.rebuild()
	return c
}

// Node returns root.
func (c *Card) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// SetContent sets body child.
func (c *Card) SetContent(n core.Node) {
	c.content = n
	if c.body != nil {
		c.body.SetChild(n)
	} else {
		c.rebuild()
	}
}

// SetFace sets font.
func (c *Card) SetFace(face text.Face) {
	c.Face = face
	if c.title != nil {
		c.title.Face = face
	}
}

func (c *Card) rebuild() {
	th := DefaultTheme()
	if c.Theme != nil {
		th = c.Theme
	}
	c.title = primitive.NewText(c.Title)
	c.title.FontSize = 16
	c.title.Face = c.Face
	c.title.Color = th.Color(core.TokenColorText)
	c.body = primitive.NewSlot("card-body", c.content)
	div := primitive.NewDivider()
	div.ColorToken = core.TokenColorBorder
	col := primitive.Column(c.title, div, c.body)
	col.Gap = 12
	col.CrossAlign = core.CrossStart
	c.Root = primitive.NewDecorated(col)
	c.Root.Padding = primitive.All(16)
	c.Root.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	c.Root.Background = th.Color(core.TokenColorBgContainer)
	c.Root.BorderWidth = 1
	c.Root.BorderColor = th.Color(core.TokenColorBorder)
	c.Root.Hit = core.HitBlock
}
