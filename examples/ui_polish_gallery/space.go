//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSpace() {
	// Space — size / vertical / wrap
	spH := kit.NewSpace(
		kit.NewTag("Item 1").Node(),
		kit.NewTag("Item 2").Node(),
		kit.NewTag("Item 3").Node(),
	)
	spH.SetSize(16)

	spV := kit.NewSpace(
		kit.NewTag("Top").Node(),
		kit.NewTag("Middle").Node(),
		kit.NewTag("Bottom").Node(),
	)
	spV.SetSize(8)
	spV.SetVertical(true)

	spWrap := kit.NewSpace()
	spWrap.SetSize(8)
	spWrap.SetWrap(true)
	for _, lab := range []string{"Tag", "Tag", "Tag", "Tag", "Tag", "Tag", "Tag"} {
		spWrap.Add(kit.NewTag(lab).Node())
	}
	spWrapHost := primitive.NewDecorated(spWrap.Node())
	spWrapHost.Width = 220
	spWrapHost.Padding = primitive.All(8)
	spWrapHost.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	spWrapHost.Radius = 6

	c.items = append(c.items, ctlTab("space", "Space"))
	c.contents["space"] = demoPage(c.face, "Space",
		"Ant Design Space: uniform gap, direction, wrap.",
		demoSection(c.face, c.theme, "Horizontal", "size=16 gap between children.", spH.Node()),
		demoSection(c.face, c.theme, "Vertical", "SetVertical(true).", spV.Node()),
		demoSection(c.face, c.theme, "Wrap", "SetWrap(true) in a narrow host (width 220).", spWrapHost),
	)
}
