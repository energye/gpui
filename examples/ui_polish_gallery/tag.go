//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTag() {
	tag := kit.NewTag("Tag")
	tag.SetFace(c.face)
	c.add("tag", "Tag", "Data Display · Tag", tag.Node())
}
