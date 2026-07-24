//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerProgress() {
	prog := kit.NewProgress(60)
	prog.Width = 280
	prog.ShowInfo = true
	c.add("progress", "Progress", "Feedback · Progress", prog.Node())
}
