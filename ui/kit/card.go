package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Card defaults (https://ant.design/components/card).
const (
	DefaultCardPadding   = 24.0 // body padding (token paddingLG)
	DefaultCardTitleFont = 16.0
	DefaultCardHeaderGap = 8.0
	DefaultCardBodyGap   = 12.0 // header · divider · body stack
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

	// Padding outer chrome inset (0 → DefaultCardPadding).
	// Use SetPadding for EdgeInsets; this float is the uniform default ladder.
	Padding float64
	// TitleFontSize (0 → DefaultCardTitleFont).
	TitleFontSize float64
	// HeaderGap between title and extra (0 → DefaultCardHeaderGap).
	HeaderGap float64
	// BodyGap between header, divider, and body (0 → DefaultCardBodyGap).
	BodyGap float64
	// pad explicit EdgeInsets when padSet (overrides uniform Padding).
	pad    primitive.EdgeInsets
	padSet bool
}

// NewCard creates a card with title and Ant defaults.
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

// SetPadding sets outer chrome inset (uniform). 0 uses DefaultCardPadding.
func (c *Card) SetPadding(px float64) {
	if c == nil {
		return
	}
	c.Padding = px
	c.padSet = false
	c.rebuild()
}

// SetPaddingInsets sets per-side outer inset (explicit, including all-zero).
func (c *Card) SetPaddingInsets(p primitive.EdgeInsets) {
	if c == nil {
		return
	}
	c.pad = p
	c.padSet = true
	c.rebuild()
}

// SetTitleFontSize sets title size (0 → DefaultCardTitleFont).
func (c *Card) SetTitleFontSize(px float64) {
	if c == nil {
		return
	}
	c.TitleFontSize = px
	if c.title != nil && px > 0 {
		c.title.FontSize = px
		c.title.MarkNeedsLayout()
		return
	}
	c.rebuild()
}

// SetHeaderGap sets title/extra row gap (0 → DefaultCardHeaderGap).
func (c *Card) SetHeaderGap(px float64) {
	if c == nil {
		return
	}
	c.HeaderGap = px
	c.rebuild()
}

// SetBodyGap sets vertical stack gap (0 → DefaultCardBodyGap).
func (c *Card) SetBodyGap(px float64) {
	if c == nil {
		return
	}
	c.BodyGap = px
	c.rebuild()
}

func (c *Card) padding() primitive.EdgeInsets {
	if c != nil && c.padSet {
		return c.pad
	}
	px := DefaultCardPadding
	if c != nil && c.Padding > 0 {
		px = c.Padding
	}
	return primitive.All(px)
}

func (c *Card) titleFont() float64 {
	if c != nil && c.TitleFontSize > 0 {
		return c.TitleFontSize
	}
	return DefaultCardTitleFont
}

func (c *Card) headerGap() float64 {
	if c != nil && c.HeaderGap > 0 {
		return c.HeaderGap
	}
	return DefaultCardHeaderGap
}

func (c *Card) bodyGap() float64 {
	if c != nil && c.BodyGap > 0 {
		return c.BodyGap
	}
	return DefaultCardBodyGap
}

func (c *Card) rebuild() {
	th := DefaultTheme()
	if c.Theme != nil {
		th = c.Theme
	}
	c.title = primitive.NewText(c.Title)
	c.title.FontSize = c.titleFont()
	c.title.Face = c.Face
	c.title.Color = th.Color(core.TokenColorText)
	c.extra = primitive.NewSlot("card-extra", c.Extra)
	header := primitive.Row(c.title, primitive.Spacer(), c.extra)
	header.CrossAlign = core.CrossCenter
	header.Gap = c.headerGap()
	c.body = primitive.NewSlot("card-body", c.content)
	div := primitive.NewDivider()
	div.ColorToken = core.TokenColorBorder
	col := primitive.Column(header, div, c.body)
	col.Gap = c.bodyGap()
	col.CrossAlign = core.CrossStart
	c.Root = primitive.NewDecorated(col)
	c.Root.Padding = c.padding()
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
