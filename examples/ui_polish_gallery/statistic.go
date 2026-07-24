//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerStatistic() {
	stat := kit.NewStatistic("Users", "1,024")
	stat.SetFace(c.face)
	c.add("statistic", "Statistic", "Data Display · Statistic", stat.Node())
}
