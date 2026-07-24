//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerList() {
	list := kit.NewList("Alpha", "Beta", "Gamma")
	c.add("list", "List", "Data Display · List", list.Node())
}
