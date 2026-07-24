//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerRate() {
	rate := kit.NewRate(3)
	rate.SetFace(c.face)
	c.add("rate", "Rate", "Data Entry · Rate", rate.Node())
}
