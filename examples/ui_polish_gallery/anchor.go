//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerAnchor() {
	anchor := kit.NewAnchor("#Intro", "#API", "#FAQ")
	anchor.SetFace(c.face)
	c.add("anchor", "Anchor", "Navigation · Anchor", anchor.Node())
}
