//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerPopconfirm() {
	pcTrig := kit.NewButton("Popconfirm")
	pcTrig.SetFace(c.face)
	pc := kit.NewPopconfirm(pcTrig.Node(), "Are you sure?")
	pc.SetFace(c.face)
	*c.buttons = append(*c.buttons, pcTrig)
	c.add("popconfirm", "Popconfirm", "Feedback · Popconfirm", pc.Node())
}
