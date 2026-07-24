//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSplitter() {
	split := kit.NewSplitter(
		kit.NewText("Left pane").Node(),
		kit.NewText("Right pane").Node(),
	)
	c.add("splitter", "Splitter", "Layout · Splitter", split.Node())
}
