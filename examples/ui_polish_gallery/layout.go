//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerLayout() {
	lay := kit.NewLayout(
		kit.NewText("Header").Node(),
		kit.NewText("Sider").Node(),
		kit.NewText("Content").Node(),
		kit.NewText("Footer").Node(),
	)
	c.add("layout", "Layout", "Layout · Layout shell", lay.Node())
}
