//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSteps() {
	steps := kit.NewSteps("Login", "Order", "Done")
	steps.SetFace(c.face)
	steps.SetCurrent(1)
	c.add("steps", "Steps", "Navigation · Steps", steps.Node())
}
