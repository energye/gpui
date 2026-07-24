//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerCarousel() {
	carousel := kit.NewCarousel(kit.NewText("Slide A").Node(), kit.NewText("Slide B").Node())
	c.add("carousel", "Carousel", "Data Display · Carousel", carousel.Node())
}
