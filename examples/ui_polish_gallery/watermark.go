//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerWatermark() {
	wm := kit.NewWatermark(kit.NewText("content").Node(), "WM")
	c.add("watermark", "Watermark", "Feedback · Watermark", wm.Node())
}
