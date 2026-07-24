//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerColorPicker() {
	cp := kit.NewColorPicker()
	c.add("color_picker", "ColorPicker", "Data Entry · ColorPicker", cp.Node())
}
