//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSpin() {
	spin := kit.NewSpin(nil)
	*c.tickers = append(*c.tickers, spin)
	c.add("spin", "Spin", "Feedback · Spin", spin.Node())
}
