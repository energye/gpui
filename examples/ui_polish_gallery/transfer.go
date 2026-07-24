//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTransfer() {
	tr := kit.NewTransfer([]string{"A", "B", "C"})
	c.add("transfer", "Transfer", "Data Entry · Transfer", tr.Node())
}
