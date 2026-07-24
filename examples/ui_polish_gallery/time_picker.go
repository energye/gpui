//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTimePicker() {
	tp := kit.NewTimePicker()
	tp.SetFace(c.face)
	c.add("time_picker", "TimePicker", "Data Entry · TimePicker", tp.Node())
}
