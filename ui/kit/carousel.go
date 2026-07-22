package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Carousel is a simple horizontal page switcher.
// https://ant.design/components/carousel
type Carousel struct {
	Root   *primitive.Flex
	Slides []core.Node
	Index  int
	Face   text.Face
	Theme  *core.Theme
}

// NewCarousel creates a carousel from slides.
func NewCarousel(slides ...core.Node) *Carousel {
	c := &Carousel{Slides: append([]core.Node(nil), slides...)}
	c.rebuild()
	return c
}

// Node returns root.
func (c *Carousel) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// SetIndex shows slide i.
func (c *Carousel) SetIndex(i int) {
	if len(c.Slides) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(c.Slides) {
		i = len(c.Slides) - 1
	}
	c.Index = i
	c.rebuild()
}

// Next advances to the next slide.
func (c *Carousel) Next() {
	c.SetIndex(c.Index + 1)
}

// Prev goes to the previous slide.
func (c *Carousel) Prev() {
	c.SetIndex(c.Index - 1)
}

func (c *Carousel) rebuild() {
	slot := primitive.NewSlot("slide", nil)
	if len(c.Slides) > 0 {
		slot.SetChild(c.Slides[c.Index])
	}
	prev := NewButton("<")
	prev.SetType(ButtonDefault)
	prev.SetOnClick(func() { c.Prev() })
	next := NewButton(">")
	next.SetType(ButtonDefault)
	next.SetOnClick(func() { c.Next() })
	nav := primitive.Row(prev.Node(), next.Node())
	nav.Gap = 8
	c.Root = primitive.Column(slot, nav)
	c.Root.Gap = 8
	c.Root.CrossAlign = core.CrossCenter
}
