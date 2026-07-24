//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSelect() {
	sel := kit.NewSelect("Select", kit.SelectOption{Value: "1", Label: "One"}, kit.SelectOption{Value: "2", Label: "Two"})
	sel.SetFace(c.face)
	c.add("select", "Select", "Data Entry · Select", sel.Node())
}
