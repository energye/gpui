//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerBadge() {
	badge := kit.NewBadge(kit.NewButton("msg").Node(), 8)
	c.add("badge", "Badge", "Data Display · Badge", badge.Node())
}
