//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerDatePicker() {
	dp := kit.NewDatePicker()
	dp.SetFace(c.face)
	c.add("date_picker", "DatePicker", "Data Entry · DatePicker", dp.Node())
}
