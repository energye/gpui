//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerGrid() {
	g := kit.NewGridCols(3,
		kit.NewText("1").Node(), kit.NewText("2").Node(), kit.NewText("3").Node(),
		kit.NewText("4").Node(), kit.NewText("5").Node(), kit.NewText("6").Node(),
	)
	c.add("grid", "Grid", "Layout · Grid 3-col", g.Node())
}
