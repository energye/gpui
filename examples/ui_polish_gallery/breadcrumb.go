//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerBreadcrumb() {
	bc := kit.NewBreadcrumb("Home", "Nav", "Page")
	bc.SetFace(c.face)
	c.add("breadcrumb", "Breadcrumb", "Navigation · Breadcrumb", bc.Node())
}
