//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerInputNumber() {
	num := kit.NewInputNumber(1)
	num.SetFace(c.face)
	c.add("input_number", "InputNumber", "Data Entry · InputNumber", num.Node())
}
