//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerUpload() {
	up := kit.NewUpload("Upload")
	up.SetFace(c.face)
	c.add("upload", "Upload", "Data Entry · Upload", up.Node())
}
