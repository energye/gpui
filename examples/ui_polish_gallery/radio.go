//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerRadio() {
	ra := kit.NewRadio("x", "Radio A")
	rb := kit.NewRadio("y", "Radio B")
	ra.SetFace(c.face)
	rb.SetFace(c.face)
	rg := kit.NewRadioGroup(ra, rb)
	rg.Select("x")
	c.add("radio", "Radio", "Data Entry · Radio / RadioGroup", rg.Node())
}
