//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTextArea() {
	ta := kit.NewTextArea("TextArea", 3)
	c.add("textarea", "TextArea", "Data Entry · TextArea", ta.Node())
}
