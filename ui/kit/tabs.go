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
	// Matches https://ant.design/components/tabs tabPosition="left".
	TabLeft
)

// Default left-tabs metrics (overridable; 0 on Tabs fields → these defaults).
const (
	DefaultTabWidth      = 160.0 // rail width
	DefaultTabItemHeight = 40.0  // each left-tab row height
	DefaultTabInkWidth   = 3.0   // left active ink bar width
	DefaultTabPadInline  = 16.0  // horizontal padding inside a tab
	DefaultTabPadBlock   = 10.0  // vertical padding when height hugs content
)

// Tabs is Ant Design–style tabs: tab list + content panel.
//
//	TabLeft layout (default for gallery):
//	  ┌──────────┬────────────────┐
//	  │ Button   │  content       │
//	  │ Input    │                │
//	  │ …        │                │
//	  └──────────┴────────────────┘
//
// Left tab items keep a fixed/content height — they do NOT stretch to fill the
// rail. Configure TabWidth / TabItemHeight / TabPad* / TabInkWidth; zero uses defaults.
type Tabs struct {
	Root *primitive.Flex

	bar  *primitive.Flex
	body *primitive.Slot
	rail *primitive.Decorated

	Items    []MenuItem
	Contents map[string]core.Node
	Active   string

	Position TabPosition

	// TabWidth: left rail width (0 → DefaultTabWidth=160).
	TabWidth float64
	// TabItemHeight: left-tab row height (0 → DefaultTabItemHeight=40; <0 → hug content).
	TabItemHeight float64
	// TabInkWidth: active indicator size (0 → 3 left / 2 top).
	TabInkWidth float64
	// TabPadInline / TabPadBlock: padding inside each tab (0 → defaults).
	TabPadInline float64
	TabPadBlock  float64

	Face     text.Face
	Theme    *core.Theme
	Nav      *core.KeyboardNav
	OnChange func(key string)
}

// NewTabs creates tabs (top position by default).
func NewTabs(items ...MenuItem) *Tabs {
	t := &Tabs{
		Items:    items,
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

// Node returns the root for tree attachment.
func (t *Tabs) Node() core.Node {
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// SetPosition sets tab list placement and rebuilds.
func (t *Tabs) SetPosition(pos TabPosition) {
	t.Position = pos
	if pos == TabLeft {
		t.Nav = core.NewKeyboardNav(core.NavVertical, len(t.Items))
	} else {
		t.Nav = core.NewKeyboardNav(core.NavHorizontal, len(t.Items))
	}
	t.rebuild()
	if t.body != nil && t.Active != "" {
		t.body.SetChild(t.Contents[t.Active])
	}
}

// SetTabWidth sets left rail width (0 restores DefaultTabWidth).
func (t *Tabs) SetTabWidth(w float64) {
	t.TabWidth = w
	t.rebuild()
	if t.body != nil && t.Active != "" {
		t.body.SetChild(t.Contents[t.Active])
	}
}

// SetTabItemHeight sets each left-tab row height.
// 0 → DefaultTabItemHeight (40); negative → hug content height.
func (t *Tabs) SetTabItemHeight(h float64) {
	t.TabItemHeight = h
	t.rebuildBar()
	if t.Root != nil {
		t.Root.MarkNeedsLayout()
		t.Root.MarkNeedsPaint()
	}
}

// SetContent associates a panel with a tab key.
func (t *Tabs) SetContent(key string, n core.Node) {
	if t.Contents == nil {
		t.Contents = make(map[string]core.Node)
	}
	t.Contents[key] = n
	if key == t.Active && t.body != nil {
		t.body.SetChild(n)
		if t.Root != nil {
			t.Root.MarkNeedsLayout()
			t.Root.MarkNeedsPaint()
		}
	}
}

// SetActive switches the visible panel and tab chrome.
func (t *Tabs) SetActive(key string) {
	t.Active = key
	if t.body != nil {
		t.body.SetChild(t.Contents[key])
	}
	t.rebuildBar()
	if t.Root != nil {
		t.Root.MarkNeedsLayout()
		t.Root.MarkNeedsPaint()
	}
	if t.OnChange != nil {
		t.OnChange(key)
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

// tabItemHeight: 0 default 40; negative → 0 (hug).
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
		// Column of fixed-height rows. CrossStretch only stretches width of each
		// row to the rail width — NOT the height of a single tab to fill the rail.
		t.bar = primitive.Column()
		t.bar.Gap = 0
		t.bar.CrossAlign = core.CrossStretch
	} else {
		t.bar = primitive.Row()
		t.bar.Gap = 0
		t.bar.CrossAlign = core.CrossEnd
	}

	var prev core.Node
	if t.Active != "" {
		prev = t.Contents[t.Active]
	}
	t.body = primitive.NewSlot("tab-body", prev)
	t.rebuildBar()

	if t.Position == TabLeft {
		railW := t.tabWidth()
		t.rail = primitive.NewDecorated(t.bar)
		t.rail.Width = railW
		t.rail.MinWidth = railW
		t.rail.Background = th.Color(core.TokenColorBgContainer)
		t.rail.BorderWidth = 0
		t.rail.Padding = primitive.EdgeInsets{Top: 8, Bottom: 8}

		div := primitive.NewDivider()
		div.Vertical = true
		div.Thickness = 1
		div.ColorToken = core.TokenColorBorder

		padBody := primitive.NewDecorated(t.body)
		padBody.Padding = primitive.All(16)
		padBody.Background = th.Color(core.TokenColorBgLayout)
		padBody.BorderWidth = 0
		bodyHost := primitive.NewFlexible(1, padBody)

		// rail is non-flex (fixed width); bodyHost grows horizontally.
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

	for i, it := range t.Items {
		i, it := i, it
		lab := primitive.NewText(it.Label)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = t.Face
		active := it.Key == t.Active
		if active {
			lab.Color = th.Color(core.TokenColorPrimary)
		} else {
			lab.Color = th.Color(core.TokenColorText)
		}

		tab := primitive.NewPressable(lab)
		tab.Base().Cursor = core.CursorPointer
		if t.Position == TabLeft {
			tab.Padding = primitive.EdgeInsets{Left: padI, Right: padI, Top: padB, Bottom: padB}
		} else {
			tab.Padding = primitive.Symmetric(padI, padB)
		}
		tab.ColorHovered = antItemHoverFill(th)
		tab.Click = func() {
			if t.Nav != nil {
				t.Nav.Index = i
			}
			t.SetActive(it.Key)
		}

		if t.Position == TabLeft {
			// One row per tab, fixed height — never Flexible(grow) in the vertical bar.
			var item core.Node
			if active {
				fill := primitive.NewDecorated(tab)
				fill.Background = antItemSelectedFill(th)
				fill.BorderWidth = 0
				if itemH > 0 {
					fill.Height = itemH
					fill.MinHeight = itemH
				}
				ink := primitive.NewBox()
				ink.Width = inkW
				ink.Color = th.Color(core.TokenColorPrimary)
				if itemH > 0 {
					ink.Height = itemH
				}
				// Horizontal flex only: fill takes remaining rail width, ink fixed.
				row := primitive.Row(primitive.NewFlexible(1, fill), ink)
				row.Gap = 0
				row.CrossAlign = core.CrossStretch
				if itemH > 0 {
					// Cap row height so Column parent cannot stretch this child.
					wrap := primitive.NewDecorated(row)
					wrap.Height = itemH
					wrap.MinHeight = itemH
					wrap.BorderWidth = 0
					wrap.Background = render.RGBA{}
					item = wrap
				} else {
					item = row
				}
			} else {
				if itemH > 0 {
					wrap := primitive.NewDecorated(tab)
					wrap.Height = itemH
					wrap.MinHeight = itemH
					wrap.BorderWidth = 0
					wrap.Background = render.RGBA{}
					item = wrap
				} else {
					item = tab
				}
			}
			t.bar.AddChild(item)
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
