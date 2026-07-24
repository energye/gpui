//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerMessage() {
	if c.msgHost == nil {
		c.msgHost = kit.NewMessageHost()
	}
	msgBtn := kit.NewButton("Info message")
	msgBtn.SetFace(c.face)
	msgBtn.SetOnClick(func() { c.msgHost.Info("hello"); *c.status = "message" })
	*c.buttons = append(*c.buttons, msgBtn)
	// Portal is mounted at app root (main.go); only the trigger lives in this tab.
	c.add("message", "Message", "Feedback · Message", msgBtn.Node())
}
