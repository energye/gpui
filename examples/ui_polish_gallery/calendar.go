//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerCalendar() {
	cal := kit.NewCalendar(2026, 7)
	cal.SetFace(c.face)
	c.add("calendar", "Calendar", "Data Display · Calendar", cal.Node())
}
