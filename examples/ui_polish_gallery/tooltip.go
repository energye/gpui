//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTooltip() {
	tooltip := kit.NewTooltip(kit.NewButton("Hover me").Node(), "Tooltip")
	c.add("tooltip", "Tooltip", "Data Display · Tooltip", tooltip.Node())
}
