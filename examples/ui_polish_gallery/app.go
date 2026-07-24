//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerApp() {
	// App = c.theme density demo
	c.add("app", "App", "Other · App (Theme density)",
		sec(c.face, "DefaultTheme + ApplyDensity"),
		kit.NewText(fmt.Sprintf("density=%s", c.theme.Density)).Node(),
	)
}
