//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSegmented() {
	seg := kit.NewSegmented("Daily", "Weekly")
	seg.SetFace(c.face)
	c.add("segmented", "Segmented", "Data Display · Segmented", seg.Node())
}
