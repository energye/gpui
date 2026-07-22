package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// TabPosition places the tab list (Ant Design tabPosition).
type TabPosition int

const (
	// TabTop is horizontal tabs above content (default).
	TabTop TabPosition = iota
	// TabLeft is a vertical category rail on the left + content on the right.
	// https://ant.design/components/tabs tabPosition="left"
	TabLeft
)

// Defaults when Tabs fields are 0.
const (
	DefaultTabWidth      = 160.0
	DefaultTabItemHeight = 40.0
	DefaultTabInkWidth   = 3.0
	DefaultTabPadInline  = 16.0
	DefaultTabPadBlock   = 10.0
)

// Tabs is Ant Design–style tabs: list + content panel.
//
// Left tabs: each item is a fixed-size row (default 160×40). Items never stretch
// to fill the rail height. Width/height/padding/ink are configurable.
type Tabs struct {
	Root *primitive.Flex

	bar  *primitive.Flex
	body *primitive.Slot
	rail *primitive.Decorated

	Items    []MenuItem
	Contents map[string]core.Node
	Active   string

	Position TabPosition

	// TabWidth left rail width (0 → 160).
	TabWidth float64
	// TabItemHeight per left-tab row (0 → 40; <0 → hug content).
	TabItemHeight float64
	// TabInkWidth active indicator (0 → 3 left / 2 top).
	TabInkWidth float64
	// TabPadInline / TabPadBlock padding inside each tab (0 → 16 / 10).
	TabPadInline float64
	TabPadBlock  float64

	Face     text.Face
	Theme    *core.Theme
	Nav      *core.KeyboardNav
	OnChange func(key string)
}

// NewTabs creates tabs (top by default).
func NewTabs(items ...MenuItem) *Tabs {
	t := &Tabs{
		Items:    append([]MenuItem(nil), items...),
		Contents: make(map[string]core.Node),
		Position: TabTop,
	}
	t.Nav = core.NewKeyboardNav(core.NavHorizontal, len(items))
	if len(items) > 0 {
		t.Active = items[0].Key
	}
	t.rebuild()
	return t
}

// Node returns the root.
func (t *Tabs) Node() core.Node {
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// SetPosition sets placement and rebuilds.
func (t *Tabs) SetPosition(pos TabPosition) {
	t.Position = pos
	if pos == TabLeft {
		t.Nav = core.NewKeyboardNav(core.NavVertical, len(t.Items))
	} else {
		t.Nav = core.NewKeyboardNav(core.NavHorizontal, len(t.Items))
	}
	t.rebuild()
	t.syncBody()
}

// SetTabWidth sets left rail width (0 → default 160).
func (t *Tabs) SetTabWidth(w float64) {
	t.TabWidth = w
	t.rebuild()
	t.syncBody()
}

// SetTabItemHeight sets left-tab row height (0 → 40; <0 → hug).
func (t *Tabs) SetTabItemHeight(h float64) {
	t.TabItemHeight = h
	t.rebuildBar()
	t.markDirty()
}

// SetContent associates a panel with a tab key.
func (t *Tabs) SetContent(key string, n core.Node) {
	if t.Contents == nil {
		t.Contents = make(map[string]core.Node)
	}
	t.Contents[key] = n
	if key == t.Active {
		t.syncBody()
	}
}

// SetActive switches the panel and tab chrome.
func (t *Tabs) SetActive(key string) {
	if key == "" {
		return
	}
	changed := t.Active != key
	t.Active = key
	t.syncBody()
	t.rebuildBar()
	t.markDirty()
	if changed && t.OnChange != nil {
		t.OnChange(key)
	}
}

func (t *Tabs) syncBody() {
	if t.body == nil {
		return
	}
	t.body.SetChild(t.Contents[t.Active])
	t.markDirty()
}

func (t *Tabs) markDirty() {
	if t.Root != nil {
		t.Root.MarkNeedsLayout()
		t.Root.MarkNeedsPaint()
	}
}

func (t *Tabs) theme() *core.Theme {
	if t.Theme != nil {
		return t.Theme
	}
	return DefaultTheme()
}

func (t *Tabs) tabWidth() float64 {
	if t.TabWidth > 0 {
		return t.TabWidth
	}
	return DefaultTabWidth
}

func (t *Tabs) tabItemHeight() float64 {
	if t.TabItemHeight < 0 {
		return 0
	}
	if t.TabItemHeight > 0 {
		return t.TabItemHeight
	}
	return DefaultTabItemHeight
}

func (t *Tabs) tabInkWidth() float64 {
	if t.TabInkWidth > 0 {
		return t.TabInkWidth
	}
	if t.Position == TabLeft {
		return DefaultTabInkWidth
	}
	return 2
}

func (t *Tabs) padInline() float64 {
	if t.TabPadInline > 0 {
		return t.TabPadInline
	}
	return DefaultTabPadInline
}

func (t *Tabs) padBlock() float64 {
	if t.TabPadBlock > 0 {
		return t.TabPadBlock
	}
	return DefaultTabPadBlock
}

func (t *Tabs) rebuild() {
	th := t.theme()
	if t.Position == TabLeft {
		t.bar = primitive.Column()
		t.bar.Gap = 0
		// CrossStretch: stretch each item's WIDTH to rail, not its height.
		t.bar.CrossAlign = core.CrossStretch
	} else {
		t.bar = primitive.Row()
		t.bar.Gap = 0
		t.bar.CrossAlign = core.CrossEnd
	}

	t.body = primitive.NewSlot("tab-body", t.Contents[t.Active])
	t.body.ExpandFill = true // panel fills right side
	t.rebuildBar()

	if t.Position == TabLeft {
		railW := t.tabWidth()
		t.rail = primitive.NewDecorated(t.bar)
		t.rail.Width = railW
		t.rail.MinWidth = railW
		t.rail.Background = th.Color(core.TokenColorBgContainer)
		t.rail.BorderWidth = 0
		t.rail.Padding = primitive.EdgeInsets{Top: 8, Bottom: 8}
		// Tabs stick to the top of the rail — never vertically center the list.
		// Ensure children of rail are width-capped (Decorated.Layout clamps MaxWidth).

		div := primitive.NewDivider()
		div.Vertical = true
		div.Thickness = 1
		div.ColorToken = core.TokenColorBorder

		padBody := primitive.NewDecorated(t.body)
		padBody.Padding = primitive.All(16)
		padBody.Background = th.Color(core.TokenColorBgLayout)
		padBody.BorderWidth = 0
		// Fill Flexible: panel background + hit cover full body (top-left content).
		padBody.StretchChild = true
		padBody.Hit = core.HitBlock
		bodyHost := primitive.NewFlexible(1, padBody)
		bodyHost.FillChild = true

		t.Root = primitive.Row(t.rail, div, bodyHost)
		t.Root.Gap = 0
		t.Root.CrossAlign = core.CrossStretch
	} else {
		div := primitive.NewDivider()
		div.ColorToken = core.TokenColorBorder
		t.Root = primitive.Column(t.bar, div, t.body)
		t.Root.Gap = 0
		t.Root.CrossAlign = core.CrossStretch
	}
}

func (t *Tabs) rebuildBar() {
	if t.bar == nil {
		return
	}
	t.bar.ClearChildren()
	th := t.theme()
	if t.Nav != nil {
		t.Nav.SetCount(len(t.Items))
	}

	itemH := t.tabItemHeight()
	inkW := t.tabInkWidth()
	padI, padB := t.padInline(), t.padBlock()
	railW := t.tabWidth()
	// Inner width available for tab chrome (rail has no horizontal padding).
	innerW := railW

	for i, it := range t.Items {
		idx, key := i, it.Key
		active := key == t.Active

		lab := primitive.NewText(it.Label)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = t.Face
		if active {
			lab.Color = th.Color(core.TokenColorPrimary)
		} else {
			lab.Color = th.Color(core.TokenColorText)
		}

		tab := primitive.NewPressable(lab)
		tab.Base().Cursor = core.CursorPointer
		tab.EnableRipple = true
		if t.Position == TabLeft {
			// Leave room on the right for ink when active.
			rightPad := padI
			if active {
				rightPad = padI + inkW
			}
			tab.Padding = primitive.EdgeInsets{Left: padI, Right: rightPad, Top: padB, Bottom: padB}
		} else {
			tab.Padding = primitive.Symmetric(padI, padB)
		}
		tab.ColorHovered = antItemHoverFill(th)
		if active {
			tab.Color = antItemSelectedFill(th)
		}
		tab.Click = func() {
			if t.Nav != nil {
				t.Nav.Index = idx
			}
			t.SetActive(key)
		}

		if t.Position == TabLeft {
			// One fixed-size Decorated per tab: full rail width × item height.
			// Ink is painted as a right-edge box sibling inside a non-flex Row
			// with explicit widths only (never Flexible in the vertical list).
			if active {
				labelW := innerW - inkW
				if labelW < 48 {
					labelW = 48
				}
				labelHost := primitive.NewDecorated(tab)
				labelHost.Width = labelW
				labelHost.MinWidth = labelW
				labelHost.BorderWidth = 0
				labelHost.Background = antItemSelectedFill(th)
				labelHost.StretchChild = true // Pressable fills full tab hit area
				if itemH > 0 {
					labelHost.Height = itemH
					labelHost.MinHeight = itemH
				}

				ink := primitive.NewBox()
				ink.Width = inkW
				ink.Color = th.Color(core.TokenColorPrimary)
				if itemH > 0 {
					ink.Height = itemH
				}

				row := primitive.Row(labelHost, ink)
				row.Gap = 0
				row.CrossAlign = core.CrossStretch
				// Pin the whole row height so Column cannot re-stretch it.
				if itemH > 0 {
					pin := primitive.NewDecorated(row)
					pin.Width = innerW
					pin.MinWidth = innerW
					pin.Height = itemH
					pin.MinHeight = itemH
					pin.BorderWidth = 0
					pin.Background = render.RGBA{}
					pin.StretchChild = true
					t.bar.AddChild(pin)
				} else {
					t.bar.AddChild(row)
				}
			} else {
				host := primitive.NewDecorated(tab)
				host.Width = innerW
				host.MinWidth = innerW
				host.BorderWidth = 0
				host.Background = render.RGBA{}
				host.StretchChild = true
				if itemH > 0 {
					host.Height = itemH
					host.MinHeight = itemH
				}
				t.bar.AddChild(host)
			}
			continue
		}

		// Top tabs
		if active {
			ink := primitive.NewBox()
			ink.Height = inkW
			if ink.Height <= 0 {
				ink.Height = 2
			}
			ink.Color = th.Color(core.TokenColorPrimary)
			col := primitive.Column(tab, ink)
			col.CrossAlign = core.CrossStretch
			col.Gap = 0
			t.bar.AddChild(col)
		} else {
			t.bar.AddChild(tab)
		}
	}
}
