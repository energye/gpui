package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// MenuItem is one menu entry.
type MenuItem struct {
	Key   string
	Label string
}

// Menu is a vertical list of selectable items with keyboard nav (B3 base).
type Menu struct {
	Root     *primitive.Decorated
	list     *primitive.Flex
	Items    []MenuItem
	Selected string
	// OpenKeys are expanded submenu keys (submenu chrome deferred; field for Ant API).
	OpenKeys []string
	Face     text.Face
	Theme    *core.Theme
	Nav      *core.KeyboardNav
	OnSelect func(key string)
}

// NewMenu creates a menu.
func NewMenu(items ...MenuItem) *Menu {
	m := &Menu{Items: items}
	m.Nav = core.NewKeyboardNav(core.NavVertical, len(items))
	m.rebuild()
	return m
}

// Node returns the root.
func (m *Menu) Node() core.Node {
	if m.Root == nil {
		m.rebuild()
	}
	return m.Root
}

// SetSelected highlights a key.
func (m *Menu) SetSelected(key string) {
	m.Selected = key
	m.rebuild()
}

// SetOpenKeys sets expanded submenu keys.
func (m *Menu) SetOpenKeys(keys []string) {
	m.OpenKeys = append([]string(nil), keys...)
	m.rebuild()
}

func (m *Menu) theme() *core.Theme {
	if m.Theme != nil {
		return m.Theme
	}
	return DefaultTheme()
}

func (m *Menu) rebuild() {
	th := m.theme()
	m.list = primitive.Column()
	m.list.Gap = 2
	m.Nav.SetCount(len(m.Items))
	for i, it := range m.Items {
		i, it := i, it
		lab := primitive.NewText(it.Label)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = m.Face
		lab.Color = th.Color(core.TokenColorText)
		row := primitive.NewPressable(lab)
		// Ant Menu item: padding 5×12
		row.Padding = primitive.Symmetric(12, 5)
		if it.Key == m.Selected {
			row.Color = antItemSelectedFill(th)
			lab.Color = antItemSelectedText(th)
		}
		row.ColorHovered = antItemHoverFill(th)
		row.Click = func() {
			m.Selected = it.Key
			m.Nav.Index = i
			if m.OnSelect != nil {
				m.OnSelect(it.Key)
			}
			m.rebuild()
		}
		m.list.AddChild(row)
	}
	m.Root = primitive.NewDecorated(m.list)
	m.Root.Padding = primitive.All(4)
	m.Root.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	m.Root.Background = th.Color(core.TokenColorBgContainer)
	m.Root.BorderWidth = 1
	m.Root.BorderColor = th.Color(core.TokenColorBorder)
	m.Root.MinWidth = 160
}
