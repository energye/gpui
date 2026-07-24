//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerCascader() {
	casc := kit.NewCascader(&kit.TreeNode{
		Key: "z", Title: "Zhejiang",
		Children: []*kit.TreeNode{{Key: "hz", Title: "Hangzhou"}},
	})
	c.add("cascader", "Cascader", "Data Entry · Cascader", casc.Node())
}
