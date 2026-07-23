package kit

import "github.com/energye/gpui/ui/core"

// ConfigProvider is Ant ConfigProvider — ambient Theme for the subtree (F8).
// It is a real layout node so ResolveTheme can find it on the ancestor chain.
// https://ant.design/components/config-provider
type ConfigProvider struct {
	core.NodeBase
	theme *core.Theme
}

// NewConfigProvider wraps child with an ambient theme for descendants.
func NewConfigProvider(theme *core.Theme, child core.Node) *ConfigProvider {
	c := &ConfigProvider{theme: theme}
	c.Init(c)
	c.Hit = core.HitDefer
	if child != nil {
		c.AddChild(child)
	}
	return c
}

// Node returns this provider (mount this, not only the child).
func (c *ConfigProvider) Node() core.Node {
	if c == nil {
		return nil
	}
	return c
}

// TypeID implements core.Node.
func (c *ConfigProvider) TypeID() string { return "kit.ConfigProvider" }

// AmbientTheme implements core.ThemeProvider.
func (c *ConfigProvider) AmbientTheme() *core.Theme {
	if c == nil {
		return nil
	}
	return c.theme
}

// SetTheme replaces the ambient theme, notifies theme hooks under this subtree,
// and dirties layout/paint (theme change broadcast).
func (c *ConfigProvider) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.theme = th
	core.NotifyThemeChanged(c, th)
	c.MarkNeedsLayout()
	c.MarkNeedsPaint()
	if t := c.Tree(); t != nil {
		t.MarkFullPaintRequired()
		t.MarkDirty()
	}
}

// Theme returns the ambient theme (alias for AmbientTheme).
func (c *ConfigProvider) Theme() *core.Theme { return c.AmbientTheme() }

// SetChild replaces the single wrapped child.
func (c *ConfigProvider) SetChild(n core.Node) {
	if c == nil {
		return
	}
	c.ClearChildren()
	if n != nil {
		c.AddChild(n)
	}
	c.MarkNeedsLayout()
}

// Layout passes constraints through to the child(ren).
func (c *ConfigProvider) Layout(cons core.Constraints) core.Size {
	kids := c.Children()
	if len(kids) == 0 {
		out := cons.Tighten(core.Size{})
		c.SetSize(out)
		return out
	}
	sz := kids[0].Layout(cons)
	kids[0].Base().SetOffset(core.Point{})
	for i := 1; i < len(kids); i++ {
		csz := kids[i].Layout(cons)
		kids[i].Base().SetOffset(core.Point{})
		if csz.Width > sz.Width {
			sz.Width = csz.Width
		}
		if csz.Height > sz.Height {
			sz.Height = csz.Height
		}
	}
	out := cons.Tighten(sz)
	c.SetSize(out)
	return out
}

// Paint paints children.
func (c *ConfigProvider) Paint(pc *core.PaintContext) {
	c.DefaultPaintChildren(pc)
}

// HitTest hits children.
func (c *ConfigProvider) HitTest(p core.Point) core.Node {
	return c.DefaultHitTest(p)
}

var (
	_ core.ThemeProvider = (*ConfigProvider)(nil)
	_ core.Node          = (*ConfigProvider)(nil)
)
