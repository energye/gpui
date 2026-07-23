package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Carousel is a simple horizontal page switcher.
// https://ant.design/components/carousel
// Root + slide slot stay stable; only the active slide child is swapped.
type Carousel struct {
	Root   *primitive.Flex
	slot   *primitive.Slot
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
	c.applySlide()
}

// Next advances to the next slide.
func (c *Carousel) Next() {
	c.SetIndex(c.Index + 1)
}

// Prev goes to the previous slide.
func (c *Carousel) Prev() {
	c.SetIndex(c.Index - 1)
}

func (c *Carousel) applySlide() {
	if c.slot == nil {
		c.rebuild()
		return
	}
	if len(c.Slides) > 0 {
		c.slot.SetChild(c.Slides[c.Index])
	} else {
		c.slot.SetChild(nil)
	}
	if c.Root != nil {
		c.Root.MarkNeedsLayout()
		c.Root.MarkNeedsPaint()
	}
}

func (c *Carousel) rebuild() {
	if c.slot == nil {
		c.slot = primitive.NewSlot("slide", nil)
	}
	c.applySlide()
	if c.Root != nil {
		return // shell already built
	}
	prev := NewButton("<")
	prev.SetType(ButtonDefault)
	prev.SetOnClick(func() { c.Prev() })
	next := NewButton(">")
	next.SetType(ButtonDefault)
	next.SetOnClick(func() { c.Next() })
	nav := primitive.Row(prev.Node(), next.Node())
	nav.Gap = 8
	c.Root = primitive.Column(c.slot, nav)
	c.Root.Gap = 8
	c.Root.CrossAlign = core.CrossCenter
}
