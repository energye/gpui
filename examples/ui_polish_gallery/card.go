//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerCard() {
	card := kit.NewCard("Card")
	card.SetFace(c.face)
	card.SetContent(kit.NewText("body").Node())
	c.add("card", "Card", "Data Display · Card", card.Node())
}
