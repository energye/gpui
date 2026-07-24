//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerPopover() {
	pop := kit.NewPopover(kit.NewButton("Popover").Node(), kit.NewText("popover body").Node())
	c.add("popover", "Popover", "Data Display · Popover", pop.Node())
}
