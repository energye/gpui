//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerAffix() {
	affix := kit.NewAffix(kit.NewText("Affix/Sticky").Node())
	c.add("affix", "Affix", "Other · Affix", affix.Node())
}
