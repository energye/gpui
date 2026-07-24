//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerEmpty() {
	empty := kit.NewEmpty("No data")
	empty.SetFace(c.face)
	c.add("empty", "Empty", "Data Display · Empty", empty.Node())
}
