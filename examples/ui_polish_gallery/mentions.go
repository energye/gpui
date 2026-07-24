//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerMentions() {
	ment := kit.NewMentions("@user", "alice", "bob")
	ment.SetFace(c.face)
	c.add("mentions", "Mentions", "Data Entry · Mentions", ment.Node())
}
