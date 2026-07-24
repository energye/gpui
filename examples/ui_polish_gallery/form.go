//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerForm() {
	fm := core.NewFormModel()
	form := kit.NewForm(fm)
	form.AddItem(kit.NewFormItem("name", "Name", kit.NewInput("name").Node()))
	c.add("form", "Form", "Data Entry · Form", form.Node())
}
