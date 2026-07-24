//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerResult() {
	res := kit.NewResult("success", "Success", "All good")
	res.SetFace(c.face)
	c.add("result", "Result", "Feedback · Result", res.Node())
}
