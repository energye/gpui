//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTabs() {
	tabsMini := kit.NewTabs(
		kit.MenuItem{Key: "t1", Label: "Tab1"},
		kit.MenuItem{Key: "t2", Label: "Tab2"},
		kit.MenuItem{Key: "t3", Label: "Tab3"},
		kit.MenuItem{Key: "t4", Label: "Tab4"},
		kit.MenuItem{Key: "t5", Label: "Tab5"},
		kit.MenuItem{Key: "t6", Label: "Tab6"},
		kit.MenuItem{Key: "t7", Label: "Tab7"},
		kit.MenuItem{Key: "t8", Label: "Tab8"},
	)
	tabsMini.Face = c.face
	tabsMini.SetPosition(kit.TabLeft)
	tabsMini.TabWidth = 100
	tabsMini.TabItemHeight = 36
	for _, k := range []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8"} {
		tx := kit.NewText("content " + k)
		tx.SetFace(c.face)
		tabsMini.SetContent(k, tx.Node())
	}
	tabsHost := primitive.NewBox(tabsMini.Node())
	tabsHost.Width, tabsHost.Height = 480, 220
	c.add("tabs", "Tabs", "Navigation · Tabs (left rail scroll)", tabsHost)
}
