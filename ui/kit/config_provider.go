package kit

import (
	"github.com/energye/gpui/ui/core"
)

// ConfigProvider is a thin theme holder (Ant ConfigProvider simplified).
// https://ant.design/components/config-provider
type ConfigProvider struct {
	theme *core.Theme
	Child core.Node
}

// NewConfigProvider wraps child with a theme reference for descendants that read Theme fields.
func NewConfigProvider(theme *core.Theme, child core.Node) *ConfigProvider {
	return &ConfigProvider{theme: theme, Child: child}
}

// Node returns child (theme is ambient for kit widgets that take Theme field).
func (c *ConfigProvider) Node() core.Node {
	if c == nil {
		return nil
	}
	return c.Child
}

// SetTheme replaces the ambient theme.
func (c *ConfigProvider) SetTheme(th *core.Theme) {
	if c != nil {
		c.theme = th
	}
}

// Theme returns the ambient theme.
func (c *ConfigProvider) Theme() *core.Theme {
	if c == nil {
		return nil
	}
	return c.theme
}

// SetChild replaces the wrapped child node.
func (c *ConfigProvider) SetChild(n core.Node) {
	if c != nil {
		c.Child = n
	}
}
