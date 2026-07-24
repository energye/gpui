//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSlider() {
	slider := kit.NewSlider(40)
	c.add("slider", "Slider", "Data Entry · Slider", slider.Node())
}
