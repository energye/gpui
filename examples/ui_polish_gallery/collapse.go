//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerCollapse() {
	collapse := kit.NewCollapse(
		kit.CollapsePanel{Key: "1", Header: "Panel 1", Content: kit.NewText("body 1").Node()},
		kit.CollapsePanel{Key: "2", Header: "Panel 2", Content: kit.NewText("body 2").Node()},
	)
	collapse.SetFace(c.face)
	collapse.SetActive("1")
	c.add("collapse", "Collapse", "Data Display · Collapse", collapse.Node())
}
