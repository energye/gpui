//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerInput() {
	in := kit.NewInput("Input")
	in.SetFace(c.face)
	in.SetFixedSize(240, 32)
	*c.tickers = append(*c.tickers, in)
	c.add("input", "Input", "Data Entry · Input", in.Node())
}
