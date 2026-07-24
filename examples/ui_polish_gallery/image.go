//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerImage() {
	img := kit.NewImage("Image", 120, 72)
	c.add("image", "Image", "Data Display · Image", img.Node())
}
