package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// CollapsePanel is one panel in Collapse.
type CollapsePanel struct {
	Key     string
	Header  string
	Content core.Node
}

// Collapse is Ant Design Collapse (accordion).
// https://ant.design/components/collapse
// Root stays stable across expand/collapse so mounted trees update in place.
type Collapse struct {
	Root      *primitive.Flex
	Panels    []CollapsePanel
	Active    map[string]bool
	Accordion bool
	Face      text.Face
	Theme     *core.Theme
	OnChange  func(activeKeys []string)
}

// NewCollapse creates collapse from panels.
func NewCollapse(panels ...CollapsePanel) *Collapse {
	c := &Collapse{
		Panels: append([]CollapsePanel(nil), panels...),
		Active: make(map[string]bool),
	}
	c.rebuild()
	return c
}

// Node returns root.
func (c *Collapse) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// SetActive opens keys.
func (c *Collapse) SetActive(keys ...string) {
	c.Active = make(map[string]bool)
	for _, k := range keys {
		c.Active[k] = true
	}
	c.rebuild()
}

// SetFace sets font.
func (c *Collapse) SetFace(face text.Face) {
	c.Face = face
	c.rebuild()
}

func (c *Collapse) rebuild() {
	th := DefaultTheme()
	if c.Theme != nil {
		th = c.Theme
	}
	if c.Root == nil {
		c.Root = primitive.Column()
	} else {
		c.Root.ClearChildren()
	}
	c.Root.Gap = 8
	c.Root.CrossAlign = core.CrossStretch
	for _, p := range c.Panels {
		p := p
		open := c.Active[p.Key]
		arrow := "▸"
		if open {
			arrow = "▾"
		}
		hLab := primitive.NewText(arrow + "  " + p.Header)
		hLab.FontSize = 14
		hLab.Face = c.Face
		hLab.Color = th.Color(core.TokenColorText)
		head := primitive.NewPressable(hLab)
		head.Padding = primitive.Symmetric(12, 8)
		head.ShowFocusRing = false
		key := p.Key
		head.Click = func() {
			if c.Accordion {
				c.Active = map[string]bool{key: !c.Active[key]}
			} else {
				c.Active[key] = !c.Active[key]
			}
			c.rebuild()
			if c.OnChange != nil {
				var keys []string
				for k, v := range c.Active {
					if v {
						keys = append(keys, k)
					}
				}
				c.OnChange(keys)
			}
		}
		shell := primitive.NewDecorated(head)
		shell.BorderWidth = 1
		shell.BorderColor = th.Color(core.TokenColorBorder)
		shell.Radius = 6
		shell.Background = th.Color(core.TokenColorBgContainer)
		col := primitive.Column(shell)
		if open && p.Content != nil {
			body := primitive.NewDecorated(p.Content)
			body.Padding = primitive.All(12)
			body.BorderWidth = 0
			col.AddChild(body)
		}
		col.Gap = 0
		c.Root.AddChild(col)
	}
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
}
