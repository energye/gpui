//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerDescriptions() {
	desc := kit.NewDescriptions([2]string{"Name", "Ada"}, [2]string{"City", "London"})
	desc.SetFace(c.face)
	c.add("descriptions", "Descriptions", "Data Display · Descriptions", desc.Node())
}
