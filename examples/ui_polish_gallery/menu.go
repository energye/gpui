//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerMenu() {
	menu := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "Item A"},
		kit.MenuItem{Key: "b", Label: "Item B"},
		kit.MenuItem{Key: "c", Label: "Item C"},
	)
	menu.Face = c.face
	c.add("menu", "Menu", "Navigation · Menu", menu.Node())
}
