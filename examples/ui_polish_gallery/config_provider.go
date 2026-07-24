//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerConfigProvider() {
	cfg := kit.NewConfigProvider(c.theme, kit.NewText("ConfigProvider child").Node())
	c.add("config_provider", "ConfigProvider", "Other · ConfigProvider", cfg.Node())
}
