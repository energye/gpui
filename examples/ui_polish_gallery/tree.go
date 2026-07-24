//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTree() {
	tree := kit.NewTree(&kit.TreeNode{Key: "r", Title: "root", Children: []*kit.TreeNode{{Key: "c", Title: "child"}}})
	c.add("tree", "Tree", "Data Display · Tree", tree.Node())
}
