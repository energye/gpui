//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerTable() {
	table := kit.NewTable(
		[]kit.TableColumn{{Key: "n", Title: "Name"}, {Key: "a", Title: "Age"}},
		[]map[string]string{{"n": "Ada", "a": "36"}, {"n": "Lin", "a": "28"}},
	)
	c.add("table", "Table", "Data Display · Table", table.Node())
}
