//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerPagination() {
	pag := kit.NewPagination(5)
	c.add("pagination", "Pagination", "Navigation · Pagination", pag.Node())
}
