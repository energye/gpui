//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerDropdown() {
	dd := kit.NewDropdown("Dropdown", kit.MenuItem{Key: "1", Label: "One"}, kit.MenuItem{Key: "2", Label: "Two"})
	c.add("dropdown", "Dropdown", "Navigation · Dropdown", dd.Node())
}
