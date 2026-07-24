//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerAvatar() {
	av := kit.NewAvatar("UI")
	av.SetFace(c.face)
	c.add("avatar", "Avatar", "Data Display · Avatar", av.Node())
}
