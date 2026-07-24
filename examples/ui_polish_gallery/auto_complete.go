//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerAutoComplete() {
	ac := kit.NewAutoComplete("AutoComplete", "Apple", "Banana", "Cherry")
	ac.SetFace(c.face)
	c.add("auto_complete", "AutoComplete", "Data Entry · AutoComplete", ac.Node())
}
