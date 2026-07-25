package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Menu defaults — components/menu/style + docs/antd/menu.md §6.2.
const (
	DefaultMenuItemHeight     = 40.0 // controlHeightLG / itemHeight
	DefaultMenuInlineIndent   = 24.0 // inlineIndent
	DefaultMenuCollapsedWidth = 80.0 // controlHeightLG * 2
	DefaultMenuDropdownWidth  = 160.0
	DefaultMenuItemPadInline  = 16.0 // horizontal padding for expanded items
	DefaultMenuItemPadBlock   = 0.0  // height is fixed via MinHeight
	DefaultMenuFontSize       = 14.0
	DefaultMenuPanelPad       = 4.0
	DefaultMenuIconGap        = 10.0
	DefaultMenuDarkBg         = "#001529"
	DefaultMenuDarkPopupBg    = "#001529"
)

// MenuMode is antd Menu.mode.
type MenuMode int

const (
	// MenuModeVertical is the antd default.
	MenuModeVertical MenuMode = iota
	MenuModeHorizontal
	MenuModeInline
)

// MenuColorTheme is antd Menu.theme / SubMenu.theme (light|dark).
// Distinct from Theme *core.Theme (design tokens).
type MenuColorTheme int

const (
	MenuColorLight MenuColorTheme = iota
	MenuColorDark
)

// MenuItem is one menu entry (also used by Tabs / Dropdown).
//
// Item kinds (antd ItemType):
//   - plain item: Key + Label, optional Icon/Disabled/Danger/Extra/Title
//   - SubMenu: Children non-empty
//   - group: Group==true or Type=="group"
//   - divider: Divider==true or Type=="divider"
type MenuItem struct {
	Key   string
	Label string
	// Type is antd item type: "" | "group" | "divider". Optional; Divider/Group flags also work.
	Type string
	// Disabled: non-interactive (e.g. Tabs category header — gray, no content).
	Disabled bool
	// Divider: thin separator row; Key/Label ignored (e.g. Tabs rail "-").
	Divider bool
	// Group: MenuItemGroupType header + children.
	Group bool
	// Danger: error-colored label (Dropdown / Menu item.danger).
	Danger bool
	// Extra: trailing shortcut / hint text (Dropdown extra demo).
	Extra string
	// Icon: optional leading icon name (primitive.Icon registry).
	Icon string
	// Title: collapsed tooltip title (antd MenuItemType.title); empty → Label.
	Title string
	// ColorTheme: SubMenu theme override (antd SubMenuType.theme); 0=inherit parent.
	// Use MenuColorLight/Dark; zero value means inherit (light is also 0 — set
	// ColorThemeSet when explicitly light under dark parent is required).
	ColorTheme    MenuColorTheme
	ColorThemeSet bool
	// Children: submenu / group items.
	Children []MenuItem
}

// Selectable reports whether the item can become the active Tabs key / menu selection.
func (it MenuItem) Selectable() bool {
	return !it.Disabled && !it.isDivider() && !it.isGroup() && it.Key != "" && len(it.Children) == 0
}

func (it MenuItem) isDivider() bool {
	return it.Divider || it.Type == "divider"
}

func (it MenuItem) isGroup() bool {
	return it.Group || it.Type == "group"
}

func (it MenuItem) isSubMenu() bool {
	return len(it.Children) > 0 && !it.isGroup() && !it.isDivider()
}

func (it MenuItem) tipTitle() string {
	if it.Title != "" {
		return it.Title
	}
	return it.Label
}

// MenuInfo is the payload for Menu onClick / onSelect / onDeselect (antd info).
type MenuInfo struct {
	Key          string
	KeyPath      []string
	SelectedKeys []string
}

// Menu is Ant Design Menu: navigation list with modes, select, open SubMenus.
//
//	Decorated (root chrome)
//	  └─ Flex (row|column by mode)
//	       ├─ item Pressable rows (height itemHeight)
//	       ├─ inline SubMenu: title + nested children
//	       └─ popup SubMenu: title + AnchoredPopup panel
//
// Product contract: docs/antd/menu.md §6 (P0 DoD).
// hit == layout == paint via primitive Pressable / Decorated / Flex.
type Menu struct {
	Root  *primitive.Decorated
	list  *primitive.Flex
	Items []MenuItem

	Mode            MenuMode
	ColorTheme      MenuColorTheme
	InlineCollapsed bool
	// InlineIndent 0 → DefaultMenuInlineIndent.
	InlineIndent float64
	Multiple     bool
	// Selectable default true (antd).
	Selectable bool
	// selectableSet distinguishes unset (default true) from explicit false.
	selectableSet bool
	// TooltipEnabled default true; false disables collapsed tooltips (antd tooltip={false}).
	TooltipEnabled bool
	tooltipSet     bool

	// selectedKeys is the live selection (uncontrolled) or last applied controlled value.
	selectedKeys []string
	// openKeys is the live open SubMenu set.
	openKeys []string

	defaultSelectedKeys []string
	defaultOpenKeys     []string
	selectedControlled  bool
	openControlled      bool
	appliedDefaultSel   bool
	appliedDefaultOpen  bool

	Face      text.Face
	Theme     *core.Theme
	AriaLabel string
	Nav       *core.KeyboardNav

	OnClick      func(info MenuInfo)
	OnSelect     func(info MenuInfo)
	OnDeselect   func(info MenuInfo)
	OnOpenChange func(openKeys []string)

	// item index map for tests / keyboard.
	itemRows map[string]*primitive.Pressable
	// flat selectable keys in visual order (keyboard).
	flatKeys []string
	// popup hosts kept for tests.
	popups []*primitive.AnchoredPopup
}

// NewMenu creates a Menu with optional items.
// Defaults (§6.10): mode=vertical, theme=light, selectable=true, multiple=false.
func NewMenu(items ...MenuItem) *Menu {
	m := &Menu{
		Items:          append([]MenuItem(nil), items...),
		Mode:           MenuModeVertical,
		ColorTheme:     MenuColorLight,
		TooltipEnabled: true,
		itemRows:       map[string]*primitive.Pressable{},
	}
	m.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	m.rebuild()
	return m
}

// Node returns the root core.Node.
func (m *Menu) Node() core.Node {
	if m == nil {
		return nil
	}
	if m.Root == nil {
		m.rebuild()
	}
	return m.Root
}

// ChromeNode returns the root Decorated chrome (Token / layout tests).
func (m *Menu) ChromeNode() core.Node {
	if m == nil {
		return nil
	}
	if m.Root == nil {
		m.rebuild()
	}
	return m.Root
}

// ItemPressable returns the pressable row for a key (tests).
func (m *Menu) ItemPressable(key string) *primitive.Pressable {
	if m == nil || m.itemRows == nil {
		return nil
	}
	return m.itemRows[key]
}

// SelectedKeys returns a copy of the current selection.
func (m *Menu) SelectedKeys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.selectedKeys...)
}

// OpenKeys returns a copy of expanded SubMenu keys.
func (m *Menu) OpenKeys() []string {
	if m == nil {
		return nil
	}
	return append([]string(nil), m.openKeys...)
}

// IsOpen reports whether key is in openKeys.
func (m *Menu) IsOpen(key string) bool {
	if m == nil {
		return false
	}
	return containsKey(m.openKeys, key)
}

// IsSelected reports whether key is selected.
func (m *Menu) IsSelected(key string) bool {
	if m == nil {
		return false
	}
	return containsKey(m.selectedKeys, key)
}

// SetItems replaces menu items.
func (m *Menu) SetItems(items ...MenuItem) {
	if m == nil {
		return
	}
	m.Items = append([]MenuItem(nil), items...)
	m.rebuild()
}

// SetMode sets vertical | horizontal | inline.
func (m *Menu) SetMode(mode MenuMode) {
	if m == nil || m.Mode == mode {
		return
	}
	m.Mode = mode
	if mode == MenuModeHorizontal {
		m.Nav.Mode = core.NavHorizontal
	} else {
		m.Nav.Mode = core.NavVertical
	}
	m.rebuild()
}

// SetColorTheme sets light | dark (antd theme).
func (m *Menu) SetColorTheme(t MenuColorTheme) {
	if m == nil || m.ColorTheme == t {
		return
	}
	m.ColorTheme = t
	m.rebuild()
}

// SetInlineCollapsed toggles inline collapsed (narrow icon rail).
func (m *Menu) SetInlineCollapsed(v bool) {
	if m == nil || m.InlineCollapsed == v {
		return
	}
	m.InlineCollapsed = v
	m.rebuild()
}

// SetInlineIndent sets inline mode indent width (0 → default 24).
func (m *Menu) SetInlineIndent(px float64) {
	if m == nil {
		return
	}
	m.InlineIndent = px
	m.rebuild()
}

// SetMultiple enables multi-select.
func (m *Menu) SetMultiple(v bool) {
	if m == nil || m.Multiple == v {
		return
	}
	m.Multiple = v
	// Keep selection coherent when leaving multi.
	if !v && len(m.selectedKeys) > 1 {
		m.selectedKeys = m.selectedKeys[:1]
	}
	m.rebuild()
}

// SetSelectable toggles whether items can be selected (antd selectable, default true).
func (m *Menu) SetSelectable(v bool) {
	if m == nil {
		return
	}
	m.Selectable = v
	m.selectableSet = true
	m.rebuild()
}

// SetTooltipEnabled enables/disables collapsed item tooltips (antd tooltip).
func (m *Menu) SetTooltipEnabled(v bool) {
	if m == nil {
		return
	}
	m.TooltipEnabled = v
	m.tooltipSet = true
	m.rebuild()
}

// SetSelectedKeys sets controlled selected keys (external priority).
func (m *Menu) SetSelectedKeys(keys ...string) {
	if m == nil {
		return
	}
	m.selectedControlled = true
	m.selectedKeys = append([]string(nil), keys...)
	m.rebuild()
}

// SetDefaultSelectedKeys sets initial selection when uncontrolled.
func (m *Menu) SetDefaultSelectedKeys(keys ...string) {
	if m == nil {
		return
	}
	m.defaultSelectedKeys = append([]string(nil), keys...)
	if !m.selectedControlled && !m.appliedDefaultSel {
		m.selectedKeys = append([]string(nil), keys...)
		m.appliedDefaultSel = true
		m.rebuild()
	}
}

// SetOpenKeys sets controlled open SubMenu keys.
func (m *Menu) SetOpenKeys(keys ...string) {
	if m == nil {
		return
	}
	m.openControlled = true
	m.openKeys = append([]string(nil), keys...)
	m.rebuild()
}

// SetDefaultOpenKeys sets initial open keys when uncontrolled.
func (m *Menu) SetDefaultOpenKeys(keys ...string) {
	if m == nil {
		return
	}
	m.defaultOpenKeys = append([]string(nil), keys...)
	if !m.openControlled && !m.appliedDefaultOpen {
		m.openKeys = append([]string(nil), keys...)
		m.appliedDefaultOpen = true
		m.rebuild()
	}
}

// SetOnClick registers item click handler.
func (m *Menu) SetOnClick(fn func(MenuInfo)) {
	if m != nil {
		m.OnClick = fn
	}
}

// SetOnSelect registers select handler.
func (m *Menu) SetOnSelect(fn func(MenuInfo)) {
	if m != nil {
		m.OnSelect = fn
	}
}

// SetOnDeselect registers deselect handler (multiple only).
func (m *Menu) SetOnDeselect(fn func(MenuInfo)) {
	if m != nil {
		m.OnDeselect = fn
	}
}

// SetOnOpenChange registers SubMenu open-keys change handler.
func (m *Menu) SetOnOpenChange(fn func(openKeys []string)) {
	if m != nil {
		m.OnOpenChange = fn
	}
}

// SetTheme sets design-token Theme override.
func (m *Menu) SetTheme(th *core.Theme) {
	if m == nil {
		return
	}
	m.Theme = th
	m.rebuild()
}

// SetFace sets text face.
func (m *Menu) SetFace(f text.Face) {
	if m == nil {
		return
	}
	m.Face = f
	m.rebuild()
}

// SetAriaLabel sets accessible name for the menu root.
func (m *Menu) SetAriaLabel(s string) {
	if m == nil {
		return
	}
	m.AriaLabel = s
	if m.Root != nil {
		m.applyA11y()
	}
}

// ItemHeight returns resolved item height (§6.2 / Token).
func (m *Menu) ItemHeight() float64 {
	th := m.theme()
	return th.SizeOr(core.TokenControlHeightLG, DefaultMenuItemHeight)
}

// CollapsedWidth returns resolved collapsed rail width.
func (m *Menu) CollapsedWidth() float64 {
	th := m.theme()
	h := th.SizeOr(core.TokenControlHeightLG, DefaultMenuItemHeight)
	return h * 2
}

// ResolvedInlineIndent returns indent used for inline nesting.
func (m *Menu) ResolvedInlineIndent() float64 {
	if m != nil && m.InlineIndent > 0 {
		return m.InlineIndent
	}
	return DefaultMenuInlineIndent
}

// selectableOn is the effective selectable flag.
func (m *Menu) selectableOn() bool {
	if m == nil {
		return true
	}
	if m.selectableSet {
		return m.Selectable
	}
	return true
}

// tooltipOn is the effective collapsed-tooltip flag.
func (m *Menu) tooltipOn() bool {
	if m == nil {
		return true
	}
	if m.tooltipSet {
		return m.TooltipEnabled
	}
	return true
}

func (m *Menu) theme() *core.Theme {
	var n core.Node
	if m.Root != nil {
		n = m.Root
	}
	return themeOf(m.Theme, n)
}

func (m *Menu) ensureDefaults() {
	if !m.selectedControlled && !m.appliedDefaultSel && len(m.defaultSelectedKeys) > 0 {
		m.selectedKeys = append([]string(nil), m.defaultSelectedKeys...)
		m.appliedDefaultSel = true
	}
	if !m.openControlled && !m.appliedDefaultOpen && len(m.defaultOpenKeys) > 0 {
		m.openKeys = append([]string(nil), m.defaultOpenKeys...)
		m.appliedDefaultOpen = true
	}
}

func (m *Menu) rebuild() {
	if m.itemRows == nil {
		m.itemRows = map[string]*primitive.Pressable{}
	}
	// Clear maps rebuilt each pass.
	for k := range m.itemRows {
		delete(m.itemRows, k)
	}
	m.flatKeys = m.flatKeys[:0]
	m.popups = m.popups[:0]
	m.ensureDefaults()

	th := m.theme()
	itemH := m.ItemHeight()
	dark := m.ColorTheme == MenuColorDark

	// Root list orientation.
	if m.Mode == MenuModeHorizontal {
		m.list = primitive.Row()
		m.list.CrossAlign = core.CrossCenter
	} else {
		m.list = primitive.Column()
		m.list.CrossAlign = core.CrossStretch
	}
	m.list.Gap = 0
	m.list.MainAlign = core.MainStart

	m.buildItems(m.list, m.Items, 0, nil, m.ColorTheme)

	if m.Root == nil {
		m.Root = primitive.NewDecorated(m.list)
	} else {
		m.Root.ClearChildren()
		m.Root.AddChild(m.list)
	}
	m.Root.Padding = primitive.EdgeInsets{}
	m.Root.Radius = 0
	m.Root.BorderWidth = 0
	m.Root.StretchChild = true

	if dark {
		m.Root.Background = render.Hex(DefaultMenuDarkBg)
	} else {
		// Light menu: transparent / container; border only for non-inline chrome optional.
		m.Root.Background = th.Color(core.TokenColorBgContainer)
	}

	// Collapsed rail width.
	if m.Mode == MenuModeInline && m.InlineCollapsed {
		m.Root.Width = m.CollapsedWidth()
		m.Root.MinWidth = m.CollapsedWidth()
	} else {
		m.Root.Width = 0
		if m.Mode == MenuModeHorizontal {
			m.Root.MinWidth = 0
			m.Root.ExpandWidth = true
		} else {
			m.Root.MinWidth = DefaultMenuDropdownWidth
		}
	}

	// Horizontal bar height ≈ itemHeight * 1.15 (menuHorizontalHeight).
	if m.Mode == MenuModeHorizontal {
		m.Root.MinHeight = itemH * 1.15
	}

	m.Nav.SetCount(len(m.flatKeys))
	m.applyA11y()
	m.Root.SetThemeHook(func(*core.Theme) { m.rebuild() })
	m.Root.MarkNeedsLayout()
	m.Root.MarkNeedsPaint()
}

func (m *Menu) applyA11y() {
	if m.Root == nil {
		return
	}
	m.Root.Base().Role = "menu"
	name := m.AriaLabel
	if name == "" {
		name = "menu"
	}
	m.Root.Base().Label = name
}

// buildItems renders items into list at depth with keyPath.
func (m *Menu) buildItems(list *primitive.Flex, items []MenuItem, depth int, keyPath []string, parentTheme MenuColorTheme) {
	for _, it := range items {
		it := it
		if it.isDivider() {
			list.AddChild(m.buildDivider(parentTheme))
			continue
		}
		if it.isGroup() {
			list.AddChild(m.buildGroupHeader(it, depth, parentTheme))
			if len(it.Children) > 0 {
				m.buildItems(list, it.Children, depth+1, keyPath, parentTheme)
			}
			continue
		}
		path := append(append([]string(nil), keyPath...), it.Key)
		if it.isSubMenu() {
			m.buildSubMenu(list, it, depth, path, parentTheme)
			continue
		}
		list.AddChild(m.buildItemRow(it, depth, path, parentTheme, false))
	}
}

func (m *Menu) buildDivider(ct MenuColorTheme) core.Node {
	th := m.theme()
	line := primitive.NewDecorated(nil)
	line.Height = 1
	line.ExpandWidth = true
	if ct == MenuColorDark {
		line.Background = render.RGBA{R: 1, G: 1, B: 1, A: 0.12}
	} else {
		line.Background = th.Color(core.TokenColorSplit)
		if line.Background.A < 0.05 {
			line.Background = th.Color(core.TokenColorBorder)
		}
	}
	line.Base().Role = "separator"
	wrap := primitive.NewDecorated(line)
	wrap.Padding = primitive.Symmetric(0, 4)
	wrap.ExpandWidth = true
	return wrap
}

func (m *Menu) buildGroupHeader(it MenuItem, depth int, ct MenuColorTheme) core.Node {
	th := m.theme()
	fs := th.SizeOr(core.TokenFontSizeSM, 12)
	lab := primitive.NewText(it.Label)
	lab.FontSize = fs
	lab.Face = m.Face
	if ct == MenuColorDark {
		lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.45}
	} else {
		lab.Color = th.Color(core.TokenColorTextSecondary)
	}
	row := primitive.NewDecorated(lab)
	row.MinHeight = m.ItemHeight() * 0.75
	row.Padding = m.itemPadding(depth)
	row.ExpandWidth = true
	row.Base().Role = "group"
	row.Base().Label = it.Label
	return row
}

func (m *Menu) subTheme(it MenuItem, parent MenuColorTheme) MenuColorTheme {
	if it.ColorThemeSet {
		return it.ColorTheme
	}
	return parent
}

func (m *Menu) buildSubMenu(list *primitive.Flex, it MenuItem, depth int, keyPath []string, parentTheme MenuColorTheme) {
	ct := m.subTheme(it, parentTheme)
	inlineExpand := m.Mode == MenuModeInline && !m.InlineCollapsed
	open := containsKey(m.openKeys, it.Key)

	// Title row.
	title := m.buildItemRow(it, depth, keyPath, parentTheme, true)
	list.AddChild(title)

	if inlineExpand {
		if open {
			// Nested children in-flow.
			m.buildItems(list, it.Children, depth+1, keyPath, ct)
		}
		return
	}

	// Popup SubMenu (vertical / horizontal / collapsed inline).
	popList := primitive.Column()
	popList.Gap = 0
	popList.CrossAlign = core.CrossStretch
	// Children rendered into popup panel.
	m.buildPopupChildren(popList, it.Children, keyPath, ct)

	panel := primitive.NewDecorated(popList)
	panel.Padding = primitive.All(DefaultMenuPanelPad)
	panel.Radius = m.theme().SizeOr(core.TokenBorderRadiusLG, 8)
	panel.MinWidth = DefaultMenuDropdownWidth
	panel.BorderWidth = m.theme().SizeOr(core.TokenLineWidth, 1)
	if ct == MenuColorDark {
		panel.Background = render.Hex(DefaultMenuDarkPopupBg)
		panel.BorderColor = render.RGBA{R: 1, G: 1, B: 1, A: 0.12}
	} else {
		panel.Background = m.theme().Color(core.TokenColorBgContainer)
		panel.BorderColor = m.theme().Color(core.TokenColorBorder)
	}
	panel.Base().Role = "menu"

	pop := primitive.NewAnchoredPopup(panel)
	pop.DismissOnOutside = true
	pop.Gap = 4
	if m.Mode == MenuModeHorizontal {
		pop.Placement = primitive.PlaceBottom
	} else {
		pop.Placement = primitive.PlaceRight
	}
	pop.OnDismiss = func() {
		m.requestOpenKey(it.Key, false)
	}
	m.popups = append(m.popups, pop)

	// Host column: title is already in list; attach popup as sibling host under list.
	// Use a zero-size host next to title via wrapping — AnchoredPopup must be in tree.
	// We already added title; add popup after it. Anchor from title pressable.
	list.AddChild(pop)
	if open {
		if row := m.itemRows[it.Key]; row != nil {
			pop.UpdateAnchorFromNode(row)
		}
		pop.SetOpen(true)
	}
}

func (m *Menu) buildPopupChildren(list *primitive.Flex, items []MenuItem, keyPath []string, ct MenuColorTheme) {
	for _, it := range items {
		it := it
		if it.isDivider() {
			list.AddChild(m.buildDivider(ct))
			continue
		}
		if it.isGroup() {
			list.AddChild(m.buildGroupHeader(it, 0, ct))
			if len(it.Children) > 0 {
				m.buildPopupChildren(list, it.Children, keyPath, ct)
			}
			continue
		}
		path := append(append([]string(nil), keyPath...), it.Key)
		if it.isSubMenu() {
			// Nested popup submenu: one-level expand-in-panel for P0 (like Dropdown).
			row := m.buildItemRow(it, 0, path, ct, true)
			list.AddChild(row)
			if containsKey(m.openKeys, it.Key) {
				m.buildPopupChildren(list, it.Children, path, m.subTheme(it, ct))
			}
			continue
		}
		list.AddChild(m.buildItemRow(it, 0, path, ct, false))
	}
}

func (m *Menu) itemPadding(depth int) primitive.EdgeInsets {
	indent := m.ResolvedInlineIndent()
	collapsed := m.Mode == MenuModeInline && m.InlineCollapsed
	if collapsed {
		// Center icon in rail.
		return primitive.Symmetric(0, 0)
	}
	left := DefaultMenuItemPadInline + float64(depth)*indent
	if m.Mode == MenuModeHorizontal {
		return primitive.Symmetric(DefaultMenuItemPadInline, 0)
	}
	return primitive.EdgeInsets{Left: left, Right: DefaultMenuItemPadInline, Top: 0, Bottom: 0}
}

func (m *Menu) buildItemRow(it MenuItem, depth int, keyPath []string, ct MenuColorTheme, isSubTitle bool) core.Node {
	th := m.theme()
	itemH := m.ItemHeight()
	fs := th.SizeOr(core.TokenFontSize, DefaultMenuFontSize)
	collapsed := m.Mode == MenuModeInline && m.InlineCollapsed
	selected := containsKey(m.selectedKeys, it.Key)
	open := isSubTitle && containsKey(m.openKeys, it.Key)
	// SubMenu title selected when any descendant selected.
	if isSubTitle && !selected {
		selected = m.anySelected(it.Children)
	}

	textC, iconC, fill, hover := m.itemColors(ct, selected, it.Disabled, it.Danger)

	var kids []core.Node
	if it.Icon != "" {
		ic := primitive.NewIcon(it.Icon)
		ic.Size = fs
		ic.Color = iconC
		kids = append(kids, ic)
	} else if collapsed {
		// Collapsed without icon: first rune / bullet placeholder.
		ph := primitive.NewText(collapsedGlyph(it.Label))
		ph.FontSize = fs
		ph.Face = m.Face
		ph.Color = textC
		kids = append(kids, ph)
	}

	if !collapsed {
		lab := primitive.NewText(it.Label)
		lab.FontSize = fs
		lab.Face = m.Face
		lab.Color = textC
		kids = append(kids, lab)
		if it.Extra != "" {
			kids = append(kids, primitive.Spacer())
			ex := primitive.NewText(it.Extra)
			ex.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
			ex.Face = m.Face
			ex.Color = th.Color(core.TokenColorTextSecondary)
			if ct == MenuColorDark {
				ex.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.45}
			}
			if it.Disabled {
				ex.Color = th.Color(core.TokenColorDisabledText)
			}
			kids = append(kids, ex)
		}
		if isSubTitle {
			kids = append(kids, primitive.Spacer())
			chevName := "chevron-down"
			if m.Mode != MenuModeInline || m.InlineCollapsed {
				chevName = "chevron-right"
			}
			if open && m.Mode == MenuModeInline && !m.InlineCollapsed {
				chevName = "chevron-down"
			}
			chev := primitive.NewIcon(chevName)
			chev.Size = 12
			chev.Color = iconC
			kids = append(kids, chev)
		}
	}

	var inner core.Node
	if len(kids) == 1 {
		inner = kids[0]
	} else {
		row := primitive.Row(kids...)
		row.CrossAlign = core.CrossCenter
		row.Gap = DefaultMenuIconGap
		if collapsed {
			row.MainAlign = core.MainCenter
		}
		inner = row
	}

	press := primitive.NewPressable(inner)
	press.Padding = m.itemPadding(depth)
	press.Base().Role = "menuitem"
	press.Base().Label = it.Label
	if selected {
		press.Color = fill
	}
	if !it.Disabled {
		press.ColorHovered = hover
	} else {
		press.SetDisabled(true)
	}
	press.FocusRingRadius = th.SizeOr(core.TokenBorderRadius, 6)
	press.FocusRingOutset = 1.5

	key := it.Key
	pathCopy := append([]string(nil), keyPath...)
	sub := isSubTitle
	dis := it.Disabled
	press.Click = func() {
		if dis {
			return
		}
		if sub {
			m.toggleOpen(key)
			return
		}
		m.activateItem(key, pathCopy)
	}

	if key != "" {
		m.itemRows[key] = press
		if !isSubTitle && !it.Disabled && !it.isDivider() && !it.isGroup() {
			m.flatKeys = append(m.flatKeys, key)
		} else if isSubTitle {
			m.flatKeys = append(m.flatKeys, key)
		}
	}

	// Force itemHeight via Decorated shell (Pressable has no MinHeight field).
	shell := primitive.NewDecorated(press)
	shell.MinHeight = itemH
	shell.ExpandWidth = m.Mode != MenuModeHorizontal
	shell.StretchChild = true
	if selected && fill.A > 0.02 {
		// Keep fill visible on shell when pressable padding doesn't fill width.
		shell.Background = fill
	}

	// Collapsed tooltip.
	if collapsed && m.tooltipOn() && it.tipTitle() != "" {
		tt := NewTooltip(it.tipTitle())
		tt.SetTriggerNode(shell)
		tt.SetFace(m.Face)
		tt.Theme = m.Theme
		tt.SetPlacement(TooltipRight)
		tt.SetMouseEnterDelay(0)
		tt.SetMouseLeaveDelay(0)
		return tt.Node()
	}

	// Horizontal selected underline (approx).
	if m.Mode == MenuModeHorizontal && selected && !isSubTitle {
		bar := primitive.NewDecorated(nil)
		bar.Height = 2
		bar.ExpandWidth = true
		bar.Background = th.Color(core.TokenColorPrimary)
		col := primitive.Column(shell, bar)
		col.CrossAlign = core.CrossStretch
		col.Gap = 0
		return col
	}

	return shell
}

func (m *Menu) itemColors(ct MenuColorTheme, selected, disabled, danger bool) (textC, iconC, fill, hover render.RGBA) {
	th := m.theme()
	if ct == MenuColorDark {
		textC = render.RGBA{R: 1, G: 1, B: 1, A: 0.65}
		iconC = textC
		fill = render.RGBA{}
		hover = render.RGBA{R: 1, G: 1, B: 1, A: 0.06}
		if selected {
			fill = th.Color(core.TokenColorPrimary)
			textC = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			iconC = textC
		}
		if disabled {
			textC = render.RGBA{R: 1, G: 1, B: 1, A: 0.25}
			iconC = textC
			hover = render.RGBA{}
		}
		if danger && !disabled {
			textC = th.Color(core.TokenColorError)
			iconC = textC
		}
		return
	}
	// light
	textC = th.Color(core.TokenColorText)
	iconC = textC
	fill = render.RGBA{}
	hover = antItemHoverFill(th)
	if selected {
		fill = antItemSelectedFill(th)
		textC = antItemSelectedText(th)
		iconC = textC
	}
	if disabled {
		textC = th.Color(core.TokenColorDisabledText)
		iconC = textC
		hover = render.RGBA{}
		fill = render.RGBA{}
	}
	if danger && !disabled {
		textC = th.Color(core.TokenColorError)
		iconC = textC
		if selected {
			// Soft error wash (no dedicated ErrorBg token).
			fill = render.RGBA{R: 1, G: 0.94, B: 0.94, A: 1}
		}
	}
	return
}

func (m *Menu) anySelected(items []MenuItem) bool {
	for _, it := range items {
		if containsKey(m.selectedKeys, it.Key) {
			return true
		}
		if m.anySelected(it.Children) {
			return true
		}
	}
	return false
}

func (m *Menu) activateItem(key string, keyPath []string) {
	info := MenuInfo{Key: key, KeyPath: append([]string(nil), keyPath...), SelectedKeys: nil}
	if m.OnClick != nil {
		m.OnClick(info)
	}
	if !m.selectableOn() {
		return
	}
	prev := append([]string(nil), m.selectedKeys...)
	next := prev
	wasSelected := containsKey(prev, key)

	if m.Multiple {
		if wasSelected {
			next = removeKey(prev, key)
			if !m.selectedControlled {
				m.selectedKeys = next
			}
			info.SelectedKeys = append([]string(nil), next...)
			if m.OnDeselect != nil {
				m.OnDeselect(info)
			}
			if !m.selectedControlled {
				m.rebuild()
			}
			return
		}
		next = append(append([]string(nil), prev...), key)
	} else {
		if wasSelected && !m.Multiple {
			// Single-select re-click keeps selected (antd).
			next = []string{key}
		} else {
			next = []string{key}
		}
	}

	if !m.selectedControlled {
		m.selectedKeys = next
	}
	info.SelectedKeys = append([]string(nil), next...)
	if m.OnSelect != nil {
		m.OnSelect(info)
	}
	// Controlled: wait for SetSelectedKeys; still rebuild if uncontrolled.
	if !m.selectedControlled {
		m.rebuild()
	} else {
		// Keep chrome in sync with last click when host is slow; host should SetSelectedKeys.
		// Do not mutate selectedKeys further.
	}
}

func (m *Menu) toggleOpen(key string) {
	open := !containsKey(m.openKeys, key)
	m.requestOpenKey(key, open)
}

func (m *Menu) requestOpenKey(key string, open bool) {
	prev := append([]string(nil), m.openKeys...)
	var next []string
	if open {
		if containsKey(prev, key) {
			next = prev
		} else {
			next = append(append([]string(nil), prev...), key)
		}
	} else {
		next = removeKey(prev, key)
	}
	// Equal?
	if sameKeys(prev, next) {
		return
	}
	if m.openControlled {
		if m.OnOpenChange != nil {
			m.OnOpenChange(next)
		}
		return
	}
	m.openKeys = next
	if m.OnOpenChange != nil {
		m.OnOpenChange(append([]string(nil), next...))
	}
	m.rebuild()
}

// HandleKey routes arrow/enter for keyboard nav (host may call via focus scope).
func (m *Menu) HandleKey(ev *core.KeyEvent) bool {
	if m == nil || ev == nil || ev.Type != core.KeyDown {
		return false
	}
	if m.Nav.HandleKey(ev.Key) {
		// Focus corresponding item if present.
		if m.Nav.Index >= 0 && m.Nav.Index < len(m.flatKeys) {
			key := m.flatKeys[m.Nav.Index]
			if row := m.itemRows[key]; row != nil {
				// Best-effort: rely on tree focus if mounted.
				_ = row
			}
		}
		return true
	}
	if ev.Key == "Enter" || ev.Key == "Return" || ev.Key == " " || ev.Key == "Space" {
		if m.Nav.Index >= 0 && m.Nav.Index < len(m.flatKeys) {
			key := m.flatKeys[m.Nav.Index]
			if row := m.itemRows[key]; row != nil && row.Click != nil {
				row.Click()
				return true
			}
		}
	}
	if ev.Key == "Escape" || ev.Key == "Esc" {
		// Close last open popup key if any.
		if len(m.openKeys) > 0 && m.Mode != MenuModeInline {
			last := m.openKeys[len(m.openKeys)-1]
			m.requestOpenKey(last, false)
			return true
		}
	}
	return false
}

// BackgroundColor returns root background (theme tests).
func (m *Menu) BackgroundColor() render.RGBA {
	if m == nil || m.Root == nil {
		return render.RGBA{}
	}
	return m.Root.Background
}

// --- helpers ---

func containsKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

func removeKey(keys []string, key string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

func sameKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// order-sensitive for openKeys (antd preserves order).
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func collapsedGlyph(label string) string {
	for _, r := range label {
		return string(r)
	}
	return "·"
}
