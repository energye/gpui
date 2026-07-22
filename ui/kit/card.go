package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Card is Ant Design Card shell (title + body).
// https://ant.design/components/card
type Card struct {
	Root     *primitive.Decorated
	title    *primitive.Text
	body     *primitive.Slot
	extra    *primitive.Slot
	Title    string
	Bordered bool
	Extra    core.Node
	Face     text.Face
	Theme    *core.Theme
	content  core.Node
}

// NewCard creates a card with title.
func NewCard(title string) *Card {
	c := &Card{Title: title, Bordered: true}
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

// SetTitle updates the card title and rebuilds header.
func (c *Card) SetTitle(s string) {
	c.Title = s
	if c.title != nil {
		c.title.SetValue(s)
		c.title.MarkNeedsPaint()
	} else {
		c.rebuild()
	}
}

// SetExtra sets the header extra node (actions area).
func (c *Card) SetExtra(n core.Node) {
	c.Extra = n
	if c.extra != nil {
		c.extra.SetChild(n)
	} else {
		c.rebuild()
	}
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
	c.extra = primitive.NewSlot("card-extra", c.Extra)
	header := primitive.Row(c.title, primitive.Spacer(), c.extra)
	header.CrossAlign = core.CrossCenter
	header.Gap = 8
	c.body = primitive.NewSlot("card-body", c.content)
	div := primitive.NewDivider()
	div.ColorToken = core.TokenColorBorder
	col := primitive.Column(header, div, c.body)
	col.Gap = 12
	col.CrossAlign = core.CrossStart
	c.Root = primitive.NewDecorated(col)
	c.Root.Padding = primitive.All(16)
	c.Root.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	c.Root.Background = th.Color(core.TokenColorBgContainer)
	if c.Bordered {
		c.Root.BorderWidth = 1
		c.Root.BorderColor = th.Color(core.TokenColorBorder)
	} else {
		c.Root.BorderWidth = 0
	}
	c.Root.Hit = core.HitBlock
}
