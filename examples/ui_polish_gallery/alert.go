//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerAlert() {
	al := kit.NewAlert("Alert message")
	al.SetFace(c.face)
	al.SetType("warning")
	c.add("alert", "Alert", "Feedback · Alert", al.Node())
}
