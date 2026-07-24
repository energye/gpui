//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerDrawer() {
	drawer := kit.NewDrawer("Drawer")
	drawer.Face = c.face
	drawer.SetContent(kit.NewText("drawer body").Node())
	openD := kit.NewButton("Open Drawer")
	openD.SetFace(c.face)
	openD.SetOnClick(func() { drawer.SetOpen(true); *c.status = "drawer open" })
	*c.buttons = append(*c.buttons, openD)
	c.add("drawer", "Drawer", "Feedback · Drawer", openD.Node(), drawer.Node())
}
