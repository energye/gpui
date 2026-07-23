package kit

import (
	"math"

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
	TabLeft
)

// Defaults when Tabs fields are 0.
const (
	DefaultTabWidth       = 160.0
	DefaultTabItemHeight  = 40.0
	DefaultTabInkWidth    = 3.0
	DefaultTabPadInline   = 16.0
	DefaultTabPadBlock    = 10.0
	DefaultTabInkDuration = 0.22 // seconds for ink slide
)

// Tabs is Ant Design–style tabs: list + content panel + sliding ink bar.
//
// Left: vertical ink on the trailing edge of the rail content (not under scrollbar).
// Top: horizontal ink under the active tab.
// Ink is configurable (size/color) and animates between tabs on switch.
type Tabs struct {
	Root *primitive.Flex

	barList    *primitive.Flex // tab items only
	barStack   *tabsBarHost
	body       *primitive.Slot
	rail       *primitive.Decorated
	barScroll  *primitive.ScrollViewport
	bodyScroll *primitive.ScrollViewport

	// ink indicator (shared, animated)
	ink     *primitive.Box
	inkPos  *core.NodeBase // positioned wrapper base for SetOffset
	inkNode core.Node      // PositionedAt host

	Items    []MenuItem
	Contents map[string]core.Node
	Active   string

	Position TabPosition

	// TabWidth left rail width (0 → 160).
	TabWidth float64
	// TabItemHeight per left-tab row (0 → 40; <0 → hug content).
	TabItemHeight float64
	// TabInkWidth indicator thickness: left=width, top=height (0 → 3 / 2).
	TabInkWidth float64
	// TabInkColor when A>0 overrides theme primary for the ink bar.
	TabInkColor render.RGBA
	// ShowInk when false hides the indicator (also hidden for Type "card").
	// Zero-value is treated as true (use HideInk to force off).
	HideInk bool
	// InkAnimated slides the indicator between tabs (default true).
	// Set InkAnimated=false for instant jump. inkAnimSet tracks explicit set.
	InkAnimated bool
	inkAnimSet  bool
	// InkDuration seconds for slide (0 → DefaultTabInkDuration).
	InkDuration float64
	// TabPadInline / TabPadBlock padding inside each tab (0 → 16 / 10).
	TabPadInline float64
	TabPadBlock  float64

	// Type: "line" (default) or "card".
	Type string
	// Centered centers the tab list on the bar (top tabs).
	Centered bool

	Face     text.Face
	Theme    *core.Theme
	Nav      *core.KeyboardNav
	OnChange func(key string)

	// ink animation state (main-axis position + span)
	inkAlong, inkSpan        float64
	inkAlongFrom, inkAlongTo float64
	inkSpanFrom, inkSpanTo   float64
	inkT                     float64 // 0..1 while animating; <0 idle
	inkSlots                 []inkSlot
	inkContentMain           float64 // content box size along main axis for layout
	tree                     *core.Tree
}

type inkSlot struct {
	key         string
	along, span float64
}

// tabsBarHost is Stack(barList, ink) that syncs ink slots after layout and
// paints the indicator after children so it always composites on top.
type tabsBarHost struct {
	primitive.Stack
	tabs *Tabs
}

func (h *tabsBarHost) TypeID() string { return "kit.TabsBar" }

func (h *tabsBarHost) Layout(c core.Constraints) core.Size {
	sz := h.Stack.Layout(c)
	if h.tabs != nil {
		h.tabs.syncInkFromLaidOutBar()
	}
	return sz
}

func (h *tabsBarHost) Paint(pc *core.PaintContext) {
	// Paint tab list (and any ink node) first.
	h.Stack.Paint(pc)
	// Draw indicator explicitly on top using current inkAlong/inkSpan (animation-safe).
	if h.tabs != nil {
		h.tabs.paintInk(pc)
	}
}

// NewTabs creates tabs (top by default).
func NewTabs(items ...MenuItem) *Tabs {
	t := &Tabs{
		Items:       append([]MenuItem(nil), items...),
		Contents:    make(map[string]core.Node),
		Position:    TabTop,
		InkAnimated: true,
		inkAnimSet:  true,
		inkT:        -1,
	}
	t.Nav = core.NewKeyboardNav(core.NavHorizontal, len(items))
	t.Active = firstSelectableKey(items)
	t.rebuild()
	return t
}

func firstSelectableKey(items []MenuItem) string {
	for _, it := range items {
		if it.Selectable() {
			return it.Key
		}
	}
	return ""
}

// FirstSelectableKey returns the first non-header/divider item key.
func (t *Tabs) FirstSelectableKey() string {
	if t == nil {
		return ""
	}
	return firstSelectableKey(t.Items)
}

// Node returns the root.
func (t *Tabs) Node() core.Node {
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// AttachTicker enables ink slide animation frames.
func (t *Tabs) AttachTicker(tr *core.Tree) {
	if t == nil {
		return
	}
	t.tree = tr
	if tr != nil && t.inkT >= 0 {
		tr.BindTicker(t, true)
	}
}

// SetPosition sets placement and rebuilds.
func (t *Tabs) SetPosition(pos TabPosition) {
	t.Position = pos
	if pos == TabLeft {
		t.Nav = core.NewKeyboardNav(core.NavVertical, len(t.Items))
	} else {
		t.Nav = core.NewKeyboardNav(core.NavHorizontal, len(t.Items))
	}
	t.inkT = -1 // snap after rebuild
	t.rebuild()
	t.syncBody()
}

// SetTabWidth sets left rail width (0 → default 160).
func (t *Tabs) SetTabWidth(w float64) {
	if t == nil {
		return
	}
	t.TabWidth = w
	t.rebuild()
	t.syncBody()
}

// SetTabItemHeight sets left-tab row height (0 → 40; <0 → hug).
func (t *Tabs) SetTabItemHeight(h float64) {
	t.TabItemHeight = h
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetInkSize sets indicator thickness (left: width, top: height).
func (t *Tabs) SetInkSize(px float64) {
	t.TabInkWidth = px
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetInkColor sets indicator color (A=0 uses theme primary).
func (t *Tabs) SetInkColor(c render.RGBA) {
	t.TabInkColor = c
	t.applyInkChrome()
	t.markDirty()
}

// SetInkAnimated enables/disables sliding ink animation.
func (t *Tabs) SetInkAnimated(v bool) {
	t.InkAnimated = v
	t.inkAnimSet = true
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
// Group headers (Disabled) and Divider items cannot be activated.
func (t *Tabs) SetActive(key string) {
	if key == "" {
		return
	}
	for _, it := range t.Items {
		if it.Key == key && !it.Selectable() {
			return
		}
	}
	changed := t.Active != key
	prev := t.Active
	t.Active = key
	t.syncBody()

	if changed && t.inkAnimated() && t.inkSlots != nil {
		from, to := t.slotOf(prev), t.slotOf(key)
		if from != nil && to != nil {
			t.inkAlongFrom, t.inkSpanFrom = t.inkAlong, t.inkSpan
			if t.inkT < 0 {
				// was snapped: start from previous slot geometry
				t.inkAlongFrom, t.inkSpanFrom = from.along, from.span
			}
			t.inkAlongTo, t.inkSpanTo = to.along, to.span
			t.inkT = 0
			if t.tree != nil {
				t.tree.BindTicker(t, true)
			}
		} else if to != nil {
			t.inkAlong, t.inkSpan = to.along, to.span
			t.inkT = -1
		}
	} else if to := t.slotOf(key); to != nil {
		t.inkAlong, t.inkSpan = to.along, to.span
		t.inkT = -1
	}

	t.rebuildBar()
	t.markLayoutDirty()
	if changed && t.OnChange != nil {
		t.OnChange(key)
	}
}

// SetType sets tab style ("line" or "card").
func (t *Tabs) SetType(typ string) {
	t.Type = typ
	t.rebuild()
	t.syncBody()
}

// Tick advances ink slide animation.
func (t *Tabs) Tick(dt float64) bool {
	if t == nil || t.inkT < 0 {
		return false
	}
	dur := t.InkDuration
	if dur <= 0 {
		dur = DefaultTabInkDuration
	}
	t.inkT += dt / dur
	if t.inkT >= 1 {
		t.inkT = 1
		t.inkAlong, t.inkSpan = t.inkAlongTo, t.inkSpanTo
		t.applyInkGeometry()
		t.inkT = -1
		t.markDirty()
		return false
	}
	// easeOutCubic
	u := t.inkT
	e := 1 - (1-u)*(1-u)*(1-u)
	t.inkAlong = t.inkAlongFrom + (t.inkAlongTo-t.inkAlongFrom)*e
	t.inkSpan = t.inkSpanFrom + (t.inkSpanTo-t.inkSpanFrom)*e
	t.applyInkGeometry()
	if t.barStack != nil {
		t.barStack.MarkNeedsPaint()
	}
	t.markDirty()
	return true
}

func (t *Tabs) inkAnimated() bool {
	if t.inkAnimSet {
		return t.InkAnimated
	}
	return true
}

func (t *Tabs) slotOf(key string) *inkSlot {
	for i := range t.inkSlots {
		if t.inkSlots[i].key == key {
			return &t.inkSlots[i]
		}
	}
	return nil
}

func (t *Tabs) syncBody() {
	if t.body == nil {
		return
	}
	t.body.SetChild(t.Contents[t.Active])
	t.markLayoutDirty()
}

// markDirty schedules paint only. Ink slide and hover must not remeasure the
// rail (remeasure during barScroll drag was a source of thumb height thrash).
func (t *Tabs) markDirty() {
	if t.barStack != nil {
		t.barStack.MarkNeedsPaint()
	} else if t.Root != nil {
		t.Root.MarkNeedsPaint()
	}
	if t.ink != nil {
		t.ink.MarkNeedsPaint()
	}
}

// markLayoutDirty is for structure / size policy changes only.
func (t *Tabs) markLayoutDirty() {
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

func (t *Tabs) inkColor() render.RGBA {
	if t.TabInkColor.A > 0 {
		return t.TabInkColor
	}
	return t.theme().Color(core.TokenColorPrimary)
}

func (t *Tabs) inkVisible() bool {
	if t.HideInk {
		return false
	}
	if t.Type == "card" {
		return false
	}
	return true
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
		t.barList = primitive.Column()
		t.barList.Gap = 0
		t.barList.CrossAlign = core.CrossStretch
	} else {
		t.barList = primitive.Row()
		t.barList.Gap = 0
		t.barList.CrossAlign = core.CrossEnd
		if t.Centered {
			t.barList.MainAlign = core.MainCenter
		}
	}

	// Shared ink indicator
	t.ink = primitive.NewBox()
	t.ink.Hit = core.HitTransparent
	t.applyInkChrome()
	t.inkNode = primitive.PositionedAt(0, 0, t.ink)

	t.barStack = &tabsBarHost{tabs: t}
	t.barStack.Fit = true
	t.barStack.Init(t.barStack)
	t.barStack.Hit = core.HitDefer
	t.barStack.AddChild(t.barList)
	t.barStack.AddChild(t.inkNode)

	t.body = primitive.NewSlot("tab-body", t.Contents[t.Active])
	t.body.ExpandFill = true
	t.rebuildBar()

	// Overflow scroll: Auto + non-overlap gutters.
	t.barScroll = primitive.NewScrollViewport(t.barStack)
	t.bodyScroll = primitive.NewScrollViewport(t.body)
	if t.Position == TabLeft {
		t.barScroll.SetAxis(true, false)
		t.barScroll.Scrollbar().Horizontal = primitive.ScrollbarNever
		t.bodyScroll.SetAxis(true, false)
		t.bodyScroll.Scrollbar().Horizontal = primitive.ScrollbarNever
	} else {
		t.barScroll.SetAxis(false, true)
		t.barScroll.Scrollbar().Vertical = primitive.ScrollbarNever
		t.bodyScroll.SetAxis(true, false)
		t.bodyScroll.Scrollbar().Horizontal = primitive.ScrollbarNever
	}

	if t.Position == TabLeft {
		railW := t.tabWidth()
		t.rail = primitive.NewDecorated(t.barScroll)
		t.rail.Width = railW
		t.rail.MinWidth = railW
		t.rail.Background = th.Color(core.TokenColorBgContainer)
		t.rail.BorderWidth = 0
		t.rail.Padding = primitive.EdgeInsets{} // track fills rail height (no vertical inset gap)
		t.rail.StretchChild = true
		t.rail.Hit = core.HitBlock

		div := primitive.NewDivider()
		div.Vertical = true
		div.Thickness = 1
		div.ColorToken = core.TokenColorBorder

		padBody := primitive.NewDecorated(t.bodyScroll)
		padBody.Padding = primitive.All(16)
		padBody.Background = th.Color(core.TokenColorBgLayout)
		padBody.BorderWidth = 0
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
		barHost := primitive.NewDecorated(t.barScroll)
		barHost.StretchChild = true
		barHost.Background = th.Color(core.TokenColorBgContainer)
		barHost.BorderWidth = 0
		bodyHost := primitive.NewFlexible(1, t.bodyScroll)
		bodyHost.FillChild = true
		t.Root = primitive.Column(barHost, div, bodyHost)
		t.Root.Gap = 0
		t.Root.CrossAlign = core.CrossStretch
	}
}

func (t *Tabs) applyInkChrome() {
	if t.ink == nil {
		return
	}
	// Ink Box is geometry/hit scaffolding only. The visible indicator is drawn
	// exclusively by paintInk (tabsBarHost.Paint). Painting both left a ghost of
	// the previous selection when Offset lagged behind inkAlong during animation.
	t.ink.Color = render.RGBA{}
}

// paintInk draws the selection indicator into the bar host local coords.
// Called from tabsBarHost.Paint so the mark is never clipped under siblings.
// This is the only place the active indicator is painted (single mark).
func (t *Tabs) paintInk(pc *core.PaintContext) {
	if t == nil || pc == nil || !t.inkVisible() {
		return
	}
	inkW := t.tabInkWidth()
	if inkW <= 0 {
		inkW = DefaultTabInkWidth
	}
	col := t.inkColor()
	if col.A <= 0 {
		return
	}
	along, span := t.inkAlong, t.inkSpan
	if span < 4 {
		// Before first layout, derive from item height.
		if t.Position == TabLeft {
			span = t.tabItemHeight()
			if span <= 0 {
				span = DefaultTabItemHeight
			}
		} else {
			span = 48
		}
	}
	if t.Position == TabLeft {
		// Right edge of bar list content.
		main := t.inkContentMain
		if t.barList != nil {
			if w := t.barList.Size().Width; w > 1 {
				main = w
			}
		}
		if main < inkW {
			// fallback: host width
			if t.barStack != nil {
				main = t.barStack.Size().Width
			}
		}
		x := main - inkW
		if x < 0 {
			x = 0
		}
		pc.FillLocalRect(x, along, inkW, span, col)
	} else {
		main := t.inkContentMain
		if t.barList != nil {
			if h := t.barList.Size().Height; h > 1 {
				main = h
			}
		}
		y := main - inkW
		if y < 0 {
			y = 0
		}
		pc.FillLocalRect(along, y, span, inkW, col)
	}
}

// applyInkGeometry places the ink box at the current animated along/span.
func (t *Tabs) applyInkGeometry() {
	if t.ink == nil {
		return
	}
	inkW := t.tabInkWidth()
	if inkW <= 0 {
		inkW = DefaultTabInkWidth
	}
	// Never paint the node itself — paintInk is sole visual source.
	t.ink.Color = render.RGBA{}
	if !t.inkVisible() {
		t.ink.Width, t.ink.Height = 0, 0
		t.setInkOffset(0, 0)
		return
	}

	if t.Position == TabLeft {
		// Vertical ink on the trailing edge of the tab list content box.
		span := t.inkSpan
		if span < 4 {
			span = 4
		}
		t.ink.Width = inkW
		t.ink.Height = span
		// Cross-axis: right edge of laid-out bar list (or estimated content main).
		main := t.inkContentMain
		if t.barList != nil {
			if w := t.barList.Size().Width; w > 0 {
				main = w
			}
		}
		x := main - inkW
		if x < 0 {
			x = 0
		}
		t.setInkOffset(x, t.inkAlong)
	} else {
		span := t.inkSpan
		if span < 8 {
			span = 8
		}
		t.ink.Width = span
		t.ink.Height = inkW
		main := t.inkContentMain
		if t.barList != nil {
			if h := t.barList.Size().Height; h > 0 {
				main = h
			}
		}
		y := main - inkW
		if y < 0 {
			y = 0
		}
		t.setInkOffset(t.inkAlong, y)
	}
}

// syncInkFromLaidOutBar rebuilds inkSlots from real tab host offsets/sizes after layout.
func (t *Tabs) syncInkFromLaidOutBar() {
	if t == nil || t.barList == nil {
		return
	}
	hosts := t.barList.Children()
	t.inkSlots = t.inkSlots[:0]
	hi := 0
	for _, it := range t.Items {
		if hi >= len(hosts) {
			break
		}
		host := hosts[hi]
		hi++
		if !it.Selectable() {
			continue
		}
		off := host.Base().Offset()
		sz := host.Base().Size()
		if t.Position == TabLeft {
			t.inkSlots = append(t.inkSlots, inkSlot{key: it.Key, along: off.Y, span: math.Max(sz.Height, 4)})
		} else {
			t.inkSlots = append(t.inkSlots, inkSlot{key: it.Key, along: off.X, span: math.Max(sz.Width, 8)})
		}
	}
	if bs := t.barList.Size(); bs.Width > 0 || bs.Height > 0 {
		if t.Position == TabLeft && bs.Width > 0 {
			t.inkContentMain = bs.Width
		}
		if t.Position != TabLeft && bs.Height > 0 {
			t.inkContentMain = bs.Height
		}
	}
	if s := t.slotOf(t.Active); s != nil {
		if t.inkT < 0 {
			t.inkAlong, t.inkSpan = s.along, s.span
		} else {
			t.inkAlongTo, t.inkSpanTo = s.along, s.span
		}
	}
	t.applyInkGeometry()
}

func (t *Tabs) setInkOffset(x, y float64) {
	if t.barStack == nil || t.ink == nil {
		return
	}
	type stackOffSet interface {
		SetStackOffset(x, y float64)
	}
	for _, c := range t.barStack.Children() {
		if c == t.barList {
			continue
		}
		if s, ok := c.(stackOffSet); ok {
			s.SetStackOffset(x, y)
			t.ink.Base().SetOffset(core.Point{})
			// Force size into layout cache for paint
			if t.ink.Width > 0 && t.ink.Height > 0 {
				t.ink.Base().SetSize(core.Size{Width: t.ink.Width, Height: t.ink.Height})
			}
			t.inkNode = c
			t.ink.MarkNeedsPaint()
			return
		}
	}
	newHost := primitive.PositionedAt(x, y, t.ink)
	t.inkNode = newHost
	// Keep barList; replace ink host only
	kids := append([]core.Node(nil), t.barStack.Children()...)
	t.barStack.ClearChildren()
	t.barStack.AddChild(t.barList)
	t.barStack.AddChild(newHost)
	_ = kids
	if t.ink.Width > 0 && t.ink.Height > 0 {
		t.ink.Base().SetSize(core.Size{Width: t.ink.Width, Height: t.ink.Height})
	}
}

func (t *Tabs) rebuildBar() {
	if t.barList == nil {
		return
	}
	t.barList.ClearChildren()
	th := t.theme()
	if t.Nav != nil {
		t.Nav.SetCount(len(t.Items))
	}

	itemH := t.tabItemHeight()
	inkW := t.tabInkWidth()
	padI, padB := t.padInline(), t.padBlock()
	railW := t.tabWidth()

	// Content width available for tabs (viewport will also subtract scrollbar gutter).
	gutter := 0.0
	if t.barScroll != nil {
		if b := t.barScroll.Scrollbar(); b != nil {
			gutter = b.GutterThickness()
		}
	} else {
		gutter = 6
	}
	innerW := railW
	if t.Position == TabLeft {
		innerW = railW - gutter
		// Account for rail vertical padding? content is inside scroll only.
		if innerW < 64 {
			innerW = 64
		}
	}

	t.inkSlots = t.inkSlots[:0]
	along := 0.0

	for i, it := range t.Items {
		idx, key := i, it.Key

		if it.Divider {
			line := primitive.NewBox()
			line.Height = 1
			line.Color = th.Color(core.TokenColorBorder)
			host := primitive.NewDecorated(line)
			host.Width = innerW
			host.MinWidth = innerW
			host.Height = 9
			host.MinHeight = 9
			host.BorderWidth = 0
			host.Padding = primitive.EdgeInsets{Top: 4, Bottom: 4, Left: padI, Right: padI}
			host.StretchChild = true
			host.Background = render.RGBA{}
			t.barList.AddChild(host)
			along += 9
			continue
		}

		if it.Disabled || !it.Selectable() {
			lab := primitive.NewText(it.Label)
			lab.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
			lab.Face = t.Face
			lab.Color = th.Color(core.TokenColorTextSecondary)
			if lab.Color.A <= 0 {
				lab.Color = render.RGBA{R: 0.55, G: 0.55, B: 0.58, A: 1}
			}
			box := primitive.NewDecorated(lab)
			box.Width = innerW
			box.MinWidth = innerW
			box.BorderWidth = 0
			box.Background = render.RGBA{}
			box.Padding = primitive.EdgeInsets{Left: padI, Right: padI, Top: 10, Bottom: 4}
			box.StretchChild = true
			// Match host preferred height: top 10 + bottom 4 + ~14 text.
			h := 28.0
			box.Height = h
			box.MinHeight = h
			t.barList.AddChild(box)
			along += h
			continue
		}

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
		tab.ShowFocusRing = false
		tab.Focusable = false
		if t.Position == TabLeft {
			// Leave room on the right for the sliding ink bar.
			tab.Padding = primitive.EdgeInsets{Left: padI, Right: padI + inkW, Top: padB, Bottom: padB}
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
			host := primitive.NewDecorated(tab)
			host.Width = innerW
			host.MinWidth = innerW
			host.BorderWidth = 0
			if active {
				host.Background = antItemSelectedFill(th)
			} else {
				host.Background = render.RGBA{}
			}
			host.StretchChild = true
			span := itemH
			if span <= 0 {
				span = padB*2 + 20
			}
			if itemH > 0 {
				host.Height = itemH
				host.MinHeight = itemH
			}
			t.barList.AddChild(host)
			t.inkSlots = append(t.inkSlots, inkSlot{key: key, along: along, span: span})
			along += span
			continue
		}

		// Top tabs: measure width roughly from label + padding (fixed min).
		// Prefer fixed min width for stable ink.
		minW := 72.0
		// Approximate: pad*2 + 8*len(label) — better after layout; use min for now.
		span := minW + float64(len(it.Label))*7
		if span < minW {
			span = minW
		}
		host := primitive.NewDecorated(tab)
		host.MinWidth = span
		host.Width = span
		host.BorderWidth = 0
		if active {
			host.Background = antItemSelectedFill(th)
		}
		host.StretchChild = true
		if itemH > 0 {
			host.Height = itemH
			host.MinHeight = itemH
		}
		t.barList.AddChild(host)
		t.inkSlots = append(t.inkSlots, inkSlot{key: key, along: along, span: span})
		along += span
	}

	// Content main size for ink placement (cross-axis).
	if t.Position == TabLeft {
		t.inkContentMain = innerW
	} else {
		// top: ink sits at bottom of bar row — height of bar content
		h := itemH
		if h <= 0 {
			h = padB*2 + 22
		}
		t.inkContentMain = h
	}

	// Snap ink if not animating
	if t.inkT < 0 {
		if s := t.slotOf(t.Active); s != nil {
			t.inkAlong, t.inkSpan = s.along, s.span
		}
	}
	t.applyInkGeometry()
}
