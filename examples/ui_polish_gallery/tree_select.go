//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTreeSelect() {
	ts := kit.NewTreeSelect("TreeSelect", "a/b", "a/c")
	c.add("tree_select", "TreeSelect", "Data Entry · TreeSelect", ts.Node())
}
