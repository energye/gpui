package kit

import (
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Breadcrumb defaults — components/breadcrumb/style prepareComponentToken
// docs/antd/breadcrumb.md §6.2 / §6.10
// https://ant.design/components/breadcrumb
const (
	DefaultBreadcrumbSeparator        = "/"
	DefaultBreadcrumbFontSize         = 14.0
	DefaultBreadcrumbIconSize         = 14.0
	DefaultBreadcrumbSeparatorMargin  = 8.0 // antd marginXS; kit TokenMarginSM
	DefaultBreadcrumbLinkPadInline    = 4.0 // paddingXXS
	DefaultBreadcrumbLinkRadius       = 4.0 // borderRadiusSM
	DefaultBreadcrumbFocusOutset      = 1.5
	DefaultBreadcrumbDropdownIconName = "chevron-down"
	// BreadcrumbItemSeparatorType is antd SeparatorType.type.
	BreadcrumbItemSeparatorType = "separator"
)

// BreadcrumbItem is one antd ItemType entry (RouteItemType | SeparatorType).
type BreadcrumbItem struct {
	// Type is "" (route item) or "separator" (independent separator).
	Type string
	// Title is the display name (antd title / legacy breadcrumbName).
	Title string
	// TitleNode overrides Title when non-nil (icon+text custom slot).
	TitleNode core.Node
	// Icon is a registry icon name prepended to Title when TitleNode is nil.
	Icon string
	// IconNode preferred over Icon when non-nil.
	IconNode core.Node
	// Href is the link destination. Empty + Link=true maps antd href:''.
	// Cannot be combined with Path in product usage (antd rule).
	Href string
	// Path is joined with previous paths into "#/a/b" when Href is empty.
	Path string
	// Link forces link chrome even when Href/Path are empty (antd href:'').
	Link bool
	// Menu opens a Dropdown on the item (antd menu.items).
	Menu []MenuItem
	// OnClick is the per-item click handler (antd item.onClick).
	OnClick func()
	// Separator is the text for type=separator; empty → root separator or "/".
	Separator string
	// Key is optional identity.
	Key string
	// Disabled blocks interaction (kit extension for BC-18).
	Disabled bool
	// Children is legacy routes[].children → converted to Menu by BreadcrumbFromRoutes.
	Children []BreadcrumbItem
}

// isSeparator reports type == "separator".
func (it BreadcrumbItem) isSeparator() bool {
	return it.Type == BreadcrumbItemSeparatorType
}

// Breadcrumb is Ant Design Breadcrumb (navigation trail).
//
//	nav Flex (role=navigation)  SkinType = kit.Breadcrumb
//	  item | separator | item | … | last
//
// # Lifecycle (#9, Button pattern)
//
// NewBreadcrumb always builds once. After that:
//
//   - structureChange() — items / separator / params / icons / face / theme
//   - chromeChange()    — reserved for pure color refresh (currently rebuilds;
//     colors are applied during structure rebuild from tokens)
//   - ensureBuilt()     — Node/ChromeNode and chrome path
//
// Product contract: docs/antd/breadcrumb.md §6 (P0 DoD).
// Root identity is stable across rebuild (ClearChildren).
type Breadcrumb struct {
	Root *primitive.Flex

	Items []BreadcrumbItem

	// Separator is the root separator (antd separator). Default "/".
	// Use SetSeparator("") to disable automatic separators (separator-component demo).
	Separator string

	// Params substitutes :key in Title/Path (antd params).
	Params map[string]string

	// DropdownIcon is the registry name for overlay chevron (default chevron-down).
	DropdownIcon string
	// DropdownIconNode overrides DropdownIcon when non-nil.
	DropdownIconNode core.Node

	// ItemRender customizes item content (antd itemRender).
	// When non-nil, replaces the default title/icon slot (link chrome still applied).
	ItemRender func(item BreadcrumbItem, params map[string]string, items []BreadcrumbItem, paths []string) core.Node

	Face      text.Face
	Theme     *core.Theme
	AriaLabel string

	// OnClick fires when a link item is activated (index into Items, resolved item).
	OnClick func(index int, item BreadcrumbItem)
	// OnMenuClick fires when a dropdown menu entry is chosen (item index, menu key).
	OnMenuClick func(itemIndex int, key string)

	// separatorSet distinguishes unset (default "/") from explicit empty.
	separatorSet bool

	// Test / chrome hooks (rebuilt each time).
	itemNodes  []core.Node
	itemPress  []*primitive.Pressable // nil for non-link slots; aligned with route items only
	itemDrop   []*Dropdown
	sepNodes   []*primitive.Text
	routeIndex []int // map display slot → Items index for route items
}

// BreadcrumbTitles builds plain title-only items (convenience).
func BreadcrumbTitles(titles ...string) []BreadcrumbItem {
	out := make([]BreadcrumbItem, len(titles))
	for i, t := range titles {
		out[i] = BreadcrumbItem{Title: t}
	}
	return out
}

// BreadcrumbFromRoutes maps legacy antd routes (path/breadcrumbName/children)
// to items (title/menu). children → menu.items.
func BreadcrumbFromRoutes(routes ...BreadcrumbItem) []BreadcrumbItem {
	out := make([]BreadcrumbItem, 0, len(routes))
	for _, r := range routes {
		it := r
		if it.Title == "" && len(r.Children) == 0 {
			// allow Title already set
		}
		// legacy breadcrumbName lives in Title field when callers put it there.
		if len(r.Children) > 0 {
			menu := make([]MenuItem, 0, len(r.Children))
			for i, c := range r.Children {
				key := c.Key
				if key == "" {
					key = c.Path
				}
				if key == "" {
					key = c.Title
				}
				if key == "" {
					key = "child-" + strconv.Itoa(i)
				}
				menu = append(menu, MenuItem{
					Key:   key,
					Label: c.Title,
				})
			}
			it.Menu = menu
			it.Children = nil
		}
		out = append(out, it)
	}
	return out
}

// NewBreadcrumb creates a Breadcrumb with optional items.
// Defaults: separator="/", dropdownIcon=chevron-down.
func NewBreadcrumb(items ...BreadcrumbItem) *Breadcrumb {
	b := &Breadcrumb{
		Items: append([]BreadcrumbItem(nil), items...),
	}
	b.rebuild()
	return b
}

// ensureBuilt materializes the nav Flex if missing.
func (b *Breadcrumb) ensureBuilt() {
	if b == nil {
		return
	}
	if b.Root == nil {
		b.rebuild()
	}
}

// structureChange rebuilds the item/separator tree from product state.
func (b *Breadcrumb) structureChange() {
	if b == nil {
		return
	}
	b.rebuild()
}

// chromeChange refreshes token-driven colors. Breadcrumb colors are applied
// while rebuilding slots, so this currently aliases structureChange; callers
// still use chromeChange for "style only" intent (symmetric with Button).
func (b *Breadcrumb) chromeChange() {
	if b == nil {
		return
	}
	b.ensureBuilt()
	b.rebuild()
}

// Node returns the stable navigation root.
func (b *Breadcrumb) Node() core.Node {
	if b == nil {
		return nil
	}
	b.ensureBuilt()
	return b.Root
}

// ChromeNode returns the visual root (same as Node).
func (b *Breadcrumb) ChromeNode() core.Node { return b.Node() }

// ItemCount returns the number of product items (including separator-type entries).
func (b *Breadcrumb) ItemCount() int {
	if b == nil {
		return 0
	}
	return len(b.Items)
}

// SeparatorCount returns how many separator nodes were rendered.
func (b *Breadcrumb) SeparatorCount() int {
	if b == nil {
		return 0
	}
	return len(b.sepNodes)
}

// ResolvedSeparator returns the effective root separator string.
// When separatorSet and Separator=="", automatic seps are disabled (returns "").
func (b *Breadcrumb) ResolvedSeparator() string {
	if b == nil {
		return DefaultBreadcrumbSeparator
	}
	if b.separatorSet {
		return b.Separator
	}
	if b.Separator != "" {
		return b.Separator
	}
	return DefaultBreadcrumbSeparator
}

// autoSeparatorEnabled is false when SetSeparator("").
func (b *Breadcrumb) autoSeparatorEnabled() bool {
	if b == nil {
		return true
	}
	if b.separatorSet {
		return b.Separator != ""
	}
	return true
}

// ResolvedSeparatorMargin is §6.2 separatorMargin (≈8).
func (b *Breadcrumb) ResolvedSeparatorMargin() float64 {
	th := b.theme()
	// antd marginXS=8; kit TokenMarginXS=4 → prefer SM / default 8.
	if g := th.SizeOr(core.TokenMarginSM, 0); g >= DefaultBreadcrumbSeparatorMargin {
		return g
	}
	return DefaultBreadcrumbSeparatorMargin
}

// ResolvedFontSize is §6.2 fontSize.
func (b *Breadcrumb) ResolvedFontSize() float64 {
	return b.theme().SizeOr(core.TokenFontSize, DefaultBreadcrumbFontSize)
}

// ResolvedIconSize is §6.2 iconFontSize.
func (b *Breadcrumb) ResolvedIconSize() float64 {
	return b.ResolvedFontSize()
}

// ResolvedLinkPadInline is §6.2 link horizontal padding.
func (b *Breadcrumb) ResolvedLinkPadInline() float64 {
	return b.theme().SizeOr(core.TokenPaddingXS, DefaultBreadcrumbLinkPadInline)
}

// ResolvedLinkRadius is §6.2 link borderRadiusSM.
func (b *Breadcrumb) ResolvedLinkRadius() float64 {
	return b.theme().SizeOr(core.TokenBorderRadiusSM, DefaultBreadcrumbLinkRadius)
}

// ItemColor is §6.2 itemColor (description / secondary).
func (b *Breadcrumb) ItemColor() render.RGBA {
	return b.theme().Color(core.TokenColorTextSecondary)
}

// LinkColor is §6.2 linkColor (same as itemColor by default).
func (b *Breadcrumb) LinkColor() render.RGBA {
	return b.theme().Color(core.TokenColorTextSecondary)
}

// LinkHoverColor is §6.2 linkHoverColor.
func (b *Breadcrumb) LinkHoverColor() render.RGBA {
	return b.theme().Color(core.TokenColorText)
}

// LastItemColor is §6.2 lastItemColor.
func (b *Breadcrumb) LastItemColor() render.RGBA {
	return b.theme().Color(core.TokenColorText)
}

// SeparatorColor is §6.2 separatorColor.
func (b *Breadcrumb) SeparatorColor() render.RGBA {
	return b.theme().Color(core.TokenColorTextSecondary)
}

// DisplayTitle returns the params-substituted title for Items[i].
func (b *Breadcrumb) DisplayTitle(i int) string {
	if b == nil || i < 0 || i >= len(b.Items) {
		return ""
	}
	return b.substitute(b.Items[i].Title)
}

// ItemPressable returns the pressable for the i-th product item, or nil.
func (b *Breadcrumb) ItemPressable(i int) *primitive.Pressable {
	if b == nil || i < 0 || i >= len(b.itemPress) {
		return nil
	}
	return b.itemPress[i]
}

// ItemDropdown returns the Dropdown for the i-th product item, or nil.
func (b *Breadcrumb) ItemDropdown(i int) *Dropdown {
	if b == nil || i < 0 || i >= len(b.itemDrop) {
		return nil
	}
	return b.itemDrop[i]
}

// ItemNode returns the root node for the i-th product item slot, or nil.
func (b *Breadcrumb) ItemNode(i int) core.Node {
	if b == nil || i < 0 || i >= len(b.itemNodes) {
		return nil
	}
	return b.itemNodes[i]
}

// IsLinkItem reports whether Items[i] renders as an interactive link
// (href/path/Link, not separator, not disabled-only plain last without link flags).
func (b *Breadcrumb) IsLinkItem(i int) bool {
	if b == nil || i < 0 || i >= len(b.Items) {
		return false
	}
	it := b.Items[i]
	if it.isSeparator() {
		return false
	}
	return b.itemIsLink(it)
}

// IsLastRouteItem reports whether Items[i] is the last non-separator route item.
func (b *Breadcrumb) IsLastRouteItem(i int) bool {
	if b == nil || i < 0 || i >= len(b.Items) {
		return false
	}
	if b.Items[i].isSeparator() {
		return false
	}
	for j := i + 1; j < len(b.Items); j++ {
		if !b.Items[j].isSeparator() {
			return false
		}
	}
	return true
}

// SetItems replaces the trail (structure).
func (b *Breadcrumb) SetItems(items []BreadcrumbItem) {
	if b == nil {
		return
	}
	b.Items = append([]BreadcrumbItem(nil), items...)
	b.structureChange()
}

// SetSeparator sets the root separator. Empty string disables automatic seps.
func (b *Breadcrumb) SetSeparator(sep string) {
	if b == nil {
		return
	}
	b.Separator = sep
	b.separatorSet = true
	b.structureChange()
}

// SetParams sets route params for :key substitution (structure; titles re-resolve).
func (b *Breadcrumb) SetParams(params map[string]string) {
	if b == nil {
		return
	}
	if params == nil {
		b.Params = nil
	} else {
		b.Params = make(map[string]string, len(params))
		for k, v := range params {
			b.Params[k] = v
		}
	}
	b.structureChange()
}

// SetDropdownIcon sets the overlay chevron registry name (structure).
func (b *Breadcrumb) SetDropdownIcon(name string) {
	if b == nil {
		return
	}
	b.DropdownIcon = name
	b.structureChange()
}

// SetDropdownIconNode sets a custom dropdown icon node (structure).
func (b *Breadcrumb) SetDropdownIconNode(n core.Node) {
	if b == nil {
		return
	}
	b.DropdownIconNode = n
	b.structureChange()
}

// SetItemRender sets the custom item content hook (structure).
func (b *Breadcrumb) SetItemRender(fn func(item BreadcrumbItem, params map[string]string, items []BreadcrumbItem, paths []string) core.Node) {
	if b == nil {
		return
	}
	b.ItemRender = fn
	b.structureChange()
}

// SetOnClick sets the root click callback.
func (b *Breadcrumb) SetOnClick(fn func(index int, item BreadcrumbItem)) {
	if b == nil {
		return
	}
	b.OnClick = fn
}

// SetOnMenuClick sets the dropdown menu click callback.
func (b *Breadcrumb) SetOnMenuClick(fn func(itemIndex int, key string)) {
	if b == nil {
		return
	}
	b.OnMenuClick = fn
}

// SetFace sets the font face (structure; labels recreated with face).
func (b *Breadcrumb) SetFace(face text.Face) {
	if b == nil {
		return
	}
	b.Face = face
	b.structureChange()
}

// SetTheme sets an explicit theme override (chrome/structure via rebuild).
func (b *Breadcrumb) SetTheme(th *core.Theme) {
	if b == nil {
		return
	}
	b.Theme = th
	b.chromeChange()
}

// SetAriaLabel sets the accessible name on the nav root.
func (b *Breadcrumb) SetAriaLabel(label string) {
	if b == nil {
		return
	}
	b.AriaLabel = label
	b.ensureBuilt()
	if b.Root != nil {
		b.Root.Base().Label = label
	}
}

func (b *Breadcrumb) theme() *core.Theme {
	var n core.Node
	if b != nil && b.Root != nil {
		n = b.Root
	}
	return themeOf(b.fieldTheme(), n)
}

func (b *Breadcrumb) fieldTheme() *core.Theme {
	if b == nil {
		return nil
	}
	return b.Theme
}

func (b *Breadcrumb) substitute(s string) string {
	if s == "" || b == nil || len(b.Params) == 0 {
		return s
	}
	out := s
	for k, v := range b.Params {
		out = strings.ReplaceAll(out, ":"+k, v)
	}
	return out
}

func (b *Breadcrumb) itemIsLink(it BreadcrumbItem) bool {
	if it.Link {
		return true
	}
	if it.Href != "" {
		return true
	}
	if it.Path != "" {
		return true
	}
	return false
}

func (b *Breadcrumb) rebuild() {
	if b == nil {
		return
	}
	th := b.theme()
	font := b.ResolvedFontSize()
	sepMargin := b.ResolvedSeparatorMargin()
	linkPad := b.ResolvedLinkPadInline()
	linkRadius := b.ResolvedLinkRadius()
	itemCol := b.ItemColor()
	linkCol := b.LinkColor()
	linkHover := b.LinkHoverColor()
	lastCol := b.LastItemColor()
	sepCol := b.SeparatorColor()
	hoverBg := th.Color(core.TokenColorBgTextHover)
	rootSep := b.ResolvedSeparator()
	autoSep := b.autoSeparatorEnabled()

	if b.Root == nil {
		b.Root = primitive.Row()
		b.Root.Base().SetThemeHook(func(th *core.Theme) {
			if b.Theme == nil {
				b.structureChange()
			}
		})
	} else {
		b.Root.ClearChildren()
	}
	// Product skin key so Theme.Skin can override Breadcrumb chrome only (#6).
	b.Root.SkinType = TypeBreadcrumb
	b.Root.Gap = 0
	b.Root.CrossAlign = core.CrossCenter
	b.Root.MainAlign = core.MainStart
	// wrap like antd ol { flex-wrap: wrap }
	b.Root.Wrap = true
	b.Root.Base().Role = "navigation"
	if b.AriaLabel != "" {
		b.Root.Base().Label = b.AriaLabel
	} else {
		b.Root.Base().Label = "Breadcrumb"
	}

	n := len(b.Items)
	b.itemNodes = make([]core.Node, n)
	b.itemPress = make([]*primitive.Pressable, n)
	b.itemDrop = make([]*Dropdown, n)
	b.sepNodes = b.sepNodes[:0]
	b.routeIndex = b.routeIndex[:0]

	// Precompute path join for path-based hrefs (antd getPath + paths[]).
	paths := make([]string, 0, n)
	pathByIndex := make([]string, n)
	for i, it := range b.Items {
		if it.isSeparator() {
			continue
		}
		if it.Path != "" {
			seg := strings.TrimPrefix(b.substitute(it.Path), "/")
			paths = append(paths, seg)
			pathByIndex[i] = "#/" + strings.Join(paths, "/")
		}
	}

	// Find last route item index for aria-current / last color.
	lastRoute := -1
	for i := n - 1; i >= 0; i-- {
		if !b.Items[i].isSeparator() {
			lastRoute = i
			break
		}
	}

	for i, it := range b.Items {
		i, it := i, it

		if it.isSeparator() {
			sepText := it.Separator
			if sepText == "" {
				sepText = rootSep
				if sepText == "" {
					sepText = DefaultBreadcrumbSeparator
				}
			}
			sep := b.makeSeparator(sepText, font, sepCol, sepMargin)
			b.itemNodes[i] = sep
			b.Root.AddChild(sep)
			continue
		}

		b.routeIndex = append(b.routeIndex, i)
		isLast := i == lastRoute
		title := b.substitute(it.Title)

		// Resolved href for this item.
		href := it.Href
		if href == "" && pathByIndex[i] != "" {
			href = pathByIndex[i]
		}
		isLink := b.itemIsLink(it) || pathByIndex[i] != ""

		// Content slot
		var content core.Node
		paramsCopy := b.Params
		if b.ItemRender != nil {
			// paths snapshot for this item (joined so far)
			content = b.ItemRender(it, paramsCopy, b.Items, append([]string(nil), paths...))
		}
		if content == nil {
			content = b.buildTitleContent(it, title, font, itemCol)
		}

		// Color for last vs others
		textCol := itemCol
		if isLast {
			textCol = lastCol
		} else if isLink {
			textCol = linkCol
		}
		b.applyTextColor(content, textCol)

		var slot core.Node
		switch {
		case len(it.Menu) > 0:
			slot = b.buildOverlayItem(i, it, content, title, isLast, textCol, linkHover, hoverBg, linkPad, linkRadius, font)
		case isLink && !it.Disabled:
			pr := b.buildLinkPressable(i, it, content, href, textCol, linkHover, hoverBg, linkPad, linkRadius)
			b.itemPress[i] = pr
			slot = pr
		case it.Disabled:
			// disabled plain / link appearance
			disCol := th.Color(core.TokenColorDisabledText)
			b.applyTextColor(content, disCol)
			box := primitive.NewDecorated(content)
			box.Padding = primitive.Symmetric(linkPad, 0)
			box.Radius = linkRadius
			slot = box
		default:
			// plain span (non-link)
			if isLast {
				box := primitive.NewDecorated(content)
				box.Padding = primitive.Symmetric(0, 0)
				box.Base().Label = title
				box.Base().Role = "text"
				// aria-current=page stand-in: Label + Key
				box.Base().Key = "aria-current=page"
				slot = box
			} else {
				slot = content
			}
		}

		if isLast && slot != nil {
			if bn := slot.Base(); bn != nil {
				if bn.Key == "" {
					bn.Key = "aria-current=page"
				}
				if bn.Label == "" {
					bn.Label = title
				}
			}
		}

		b.itemNodes[i] = slot
		b.Root.AddChild(slot)

		// Automatic separator after non-last route items when enabled.
		if autoSep && !isLast {
			sep := b.makeSeparator(rootSep, font, sepCol, sepMargin)
			b.Root.AddChild(sep)
		}
	}
}

func (b *Breadcrumb) makeSeparator(textStr string, font float64, col render.RGBA, margin float64) core.Node {
	sep := primitive.NewText(textStr)
	sep.FontSize = font
	sep.Face = b.Face
	sep.Color = col
	// marginInline via Decorated padding so hit/layout/paint share the same box.
	box := primitive.NewDecorated(sep)
	box.Padding = primitive.Symmetric(margin, 0)
	b.sepNodes = append(b.sepNodes, sep)
	return box
}

func (b *Breadcrumb) buildTitleContent(it BreadcrumbItem, title string, font float64, col render.RGBA) core.Node {
	if it.TitleNode != nil {
		return it.TitleNode
	}
	var icon core.Node
	if it.IconNode != nil {
		icon = it.IconNode
	} else if it.Icon != "" {
		ic := NewIcon(it.Icon)
		ic.Size = b.ResolvedIconSize()
		ic.Color = col
		if b.Theme != nil {
			ic.Theme = b.Theme
		}
		icon = ic.Node()
	}
	if icon != nil && title != "" {
		lab := primitive.NewText(title)
		lab.FontSize = font
		lab.Face = b.Face
		lab.Color = col
		row := primitive.Row(icon, lab)
		row.Gap = b.theme().SizeOr(core.TokenMarginXS, 4) // icon+span marginInlineStart marginXXS
		row.CrossAlign = core.CrossCenter
		return row
	}
	if icon != nil {
		return icon
	}
	lab := primitive.NewText(title)
	lab.FontSize = font
	lab.Face = b.Face
	lab.Color = col
	return lab
}

func (b *Breadcrumb) applyTextColor(n core.Node, col render.RGBA) {
	if n == nil {
		return
	}
	switch t := n.(type) {
	case *primitive.Text:
		t.Color = col
	case *primitive.Flex:
		for _, c := range t.Children() {
			b.applyTextColor(c, col)
		}
	case *primitive.Decorated:
		for _, c := range t.Children() {
			b.applyTextColor(c, col)
		}
	}
}

func (b *Breadcrumb) buildLinkPressable(
	index int,
	it BreadcrumbItem,
	content core.Node,
	href string,
	textCol, hoverCol, hoverBg render.RGBA,
	pad, radius float64,
) *primitive.Pressable {
	// Re-color content for link.
	b.applyTextColor(content, textCol)
	pr := primitive.NewPressable(content)
	pr.Padding = primitive.Symmetric(pad, 0)
	pr.FocusRingRadius = radius
	pr.FocusRingOutset = DefaultBreadcrumbFocusOutset
	pr.ShowFocusRing = true
	pr.EnableRipple = false
	pr.ColorHovered = hoverBg
	pr.Base().Role = "link"
	if href != "" {
		pr.Base().Label = href
	} else if it.Title != "" {
		pr.Base().Label = b.substitute(it.Title)
	}
	idx, item := index, it
	pr.Click = func() {
		if item.OnClick != nil {
			item.OnClick()
		}
		if b.OnClick != nil {
			b.OnClick(idx, item)
		}
	}
	// Hover text color: update on state change
	pr.OnStateChange = func() {
		if pr.State.Disabled {
			return
		}
		if pr.State.Hovered {
			b.applyTextColor(content, hoverCol)
		} else {
			b.applyTextColor(content, textCol)
		}
		pr.MarkNeedsPaint()
	}
	return pr
}

func (b *Breadcrumb) buildOverlayItem(
	index int,
	it BreadcrumbItem,
	content core.Node,
	title string,
	isLast bool,
	textCol, hoverCol, hoverBg render.RGBA,
	pad, radius, font float64,
) core.Node {
	// Trigger: content + dropdown icon
	iconName := b.DropdownIcon
	if iconName == "" {
		iconName = DefaultBreadcrumbDropdownIconName
	}
	var dropIcon core.Node
	if b.DropdownIconNode != nil {
		dropIcon = b.DropdownIconNode
	} else {
		ic := NewIcon(iconName)
		ic.Size = b.theme().SizeOr(core.TokenFontSize, 12)
		ic.Color = textCol
		if b.Theme != nil {
			ic.Theme = b.Theme
		}
		dropIcon = ic.Node()
	}
	b.applyTextColor(content, textCol)
	row := primitive.Row(content, dropIcon)
	row.Gap = b.theme().SizeOr(core.TokenMarginXS, 4)
	row.CrossAlign = core.CrossCenter

	dd := NewDropdown(title, it.Menu...)
	dd.SetTriggerNode(row)
	dd.SetTriggerModes(DropdownTriggerHover)
	dd.SetPlacement(DropdownBottomLeft)
	if b.Face != nil {
		dd.SetFace(b.Face)
	}
	if b.Theme != nil {
		dd.SetTheme(b.Theme)
	}
	idx := index
	dd.SetOnMenuClick(func(key string) {
		if b.OnMenuClick != nil {
			b.OnMenuClick(idx, key)
		}
	})
	// Also allow clicking the title area to fire OnClick when link-like.
	if shell := dd.TriggerShell(); shell != nil {
		shell.Padding = primitive.Symmetric(pad, 0)
		shell.FocusRingRadius = radius
		shell.FocusRingOutset = DefaultBreadcrumbFocusOutset
		shell.ColorHovered = hoverBg
		shell.EnableRipple = false
		shell.Base().Role = "button"
		shell.Base().Label = title
		if isLast {
			shell.Base().Key = "aria-current=page"
		}
		item := it
		prev := shell.Click
		shell.Click = func() {
			if item.OnClick != nil {
				item.OnClick()
			}
			if b.OnClick != nil {
				b.OnClick(idx, item)
			}
			if prev != nil {
				prev()
			}
		}
	}
	b.itemDrop[index] = dd
	// Expose trigger as pressable for tests.
	b.itemPress[index] = dd.TriggerShell()
	return dd.Node()
}
