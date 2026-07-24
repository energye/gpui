//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTimeline() {
	tl := kit.NewTimeline(kit.TimelineItem{Label: "Create"}, kit.TimelineItem{Label: "Ship"})
	tl.SetFace(c.face)
	c.add("timeline", "Timeline", "Data Display · Timeline", tl.Node())
}
