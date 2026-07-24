//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerModal() {
	c.modal = kit.NewModal("Confirm")
	c.modal.SetFace(c.face)
	c.modal.SetContent(kit.NewText("Modal body").Node())
	c.modal.Viewport = core.Size{Width: 1024, Height: 768}
	c.modal.OnOk = func() { *c.status = "modal ok"; c.modal.SetOpen(false) }
	c.modal.OnCancel = func() { *c.status = "modal cancel"; c.modal.SetOpen(false) }
	openM := kit.NewButton("Open Modal")
	openM.SetFace(c.face)
	openM.SetOnClick(func() { c.modal.SetOpen(true); *c.status = "modal open" })
	*c.buttons = append(*c.buttons, openM)
	c.add("modal", "Modal", "Feedback · Modal", openM.Node(), c.modal.Node())
}
