//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerCheckbox() {
	cb := kit.NewCheckbox("Checkbox")
	cb.SetFace(c.face)
	c.add("checkbox", "Checkbox", "Data Entry · Checkbox", cb.Node())
}
