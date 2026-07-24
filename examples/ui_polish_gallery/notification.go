//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerNotification() {
	// Notification uses the same app-level MessageHost (Ant notification queue).
	nBtn := kit.NewButton("Notify")
	nBtn.SetFace(c.face)
	nBtn.SetOnClick(func() { c.msgHost.Notification("Title", "body"); *c.status = "notify" })
	*c.buttons = append(*c.buttons, nBtn)
	c.add("notification", "Notification", "Feedback · Notification", nBtn.Node())
}
