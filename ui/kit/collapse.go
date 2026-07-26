package kit

import (
	"sort"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Collapse component tokens — prepareComponentToken / genBaseStyle.
// docs/antd/collapse.md §6.2 · components/collapse/style/index.ts
//
// antd seed uses paddingSM=12 / marginSM=12; kit TokenPaddingSM=8 maps to antd paddingXS.
// Component defaults below match antd Collapse tokens, not raw kit seed SM.
const (
	// DefaultCollapseHeaderPadV is header padding-block (medium, antd paddingSM=12).
	DefaultCollapseHeaderPadV = 12.0
	// DefaultCollapseHeaderPadH is header padding-inline (medium, antd padding=16).
	DefaultCollapseHeaderPadH = 16.0
	// DefaultCollapseHeaderPadVSM is small header pad-block (antd paddingXS=8).
	DefaultCollapseHeaderPadVSM = 8.0
	// DefaultCollapseHeaderPadHSM is small header pad-inline (antd paddingSM=12).
	DefaultCollapseHeaderPadHSM = 12.0
	// DefaultCollapseHeaderPadVLG is large header pad-block (antd padding=16).
	DefaultCollapseHeaderPadVLG = 16.0
	// DefaultCollapseHeaderPadHLG is large header pad-inline (antd paddingLG=24).
	DefaultCollapseHeaderPadHLG = 24.0

	// DefaultCollapseContentPad is content padding medium (16).
	DefaultCollapseContentPad = 16.0
	// DefaultCollapseContentPadSM is content padding small (antd paddingSM=12).
	DefaultCollapseContentPadSM = 12.0
	// DefaultCollapseContentPadLG is content padding large (paddingLG=24).
	DefaultCollapseContentPadLG = 24.0

	// DefaultCollapseBorderlessPadT is borderless body padding-block-start (paddingXXS=4).
	DefaultCollapseBorderlessPadT = 4.0
	// DefaultCollapseBorderlessPadH is borderless body padding-inline (16).
	DefaultCollapseBorderlessPadH = 16.0
	// DefaultCollapseBorderlessPadB is borderless body padding-block-end (padding=16).
	DefaultCollapseBorderlessPadB = 16.0

	// DefaultCollapseGhostBodyPadV is ghost body padding-block (paddingSM=12).
	DefaultCollapseGhostBodyPadV = 12.0

	// DefaultCollapseRadius is collapsePanelBorderRadius (borderRadiusLG).
	DefaultCollapseRadius = 8.0
	// DefaultCollapseLineWidth is lineWidth.
	DefaultCollapseLineWidth = 1.0
	// DefaultCollapseFontSize is fontSize (medium).
	DefaultCollapseFontSize = 14.0
	// DefaultCollapseFontSizeLG is fontSizeLG (large panels).
	DefaultCollapseFontSizeLG = 16.0
	// DefaultCollapseIconSize is fontSizeIcon.
	DefaultCollapseIconSize = 12.0
	// DefaultCollapseIconGap is margin between expand icon and title (antd marginSM=12).
	DefaultCollapseIconGap = 12.0
	// DefaultCollapseIconBoxH approximates fontHeight for expand-icon align.
	DefaultCollapseIconBoxH = 22.0
	// DefaultCollapseIconBoxHLG approximates fontHeightLG.
	DefaultCollapseIconBoxHLG = 24.0
	// DefaultCollapseFocusRingOutset approximates Ant focus-visible outset.
	DefaultCollapseFocusRingOutset = 1.5
)

// CollapseSize is antd size: large | medium | small (kit Middle = medium).
type CollapseSize int

const (
	// CollapseMiddle is antd medium (default).
	CollapseMiddle CollapseSize = iota
	// CollapseSmall is compact.
	CollapseSmall
	// CollapseLarge is large type/font/padding.
	CollapseLarge
)

// CollapseCollapsible is antd collapsible trigger region.
type CollapseCollapsible int

const (
	// CollapseCollapsibleInherit means use Collapse.Collapsible (item-level unset).
	CollapseCollapsibleInherit CollapseCollapsible = iota
	// CollapseCollapsibleHeader: title + icon toggle (default).
	CollapseCollapsibleHeader
	// CollapseCollapsibleIcon: only expand icon toggles.
	CollapseCollapsibleIcon
	// CollapseCollapsibleDisabled: cannot toggle.
	CollapseCollapsibleDisabled
)

// CollapseIconPlacement is expandIconPlacement: start | end.
type CollapseIconPlacement int

const (
	// CollapseIconStart places the expand icon at the logical start (default).
	CollapseIconStart CollapseIconPlacement = iota
	// CollapseIconEnd places the expand icon at the logical end.
	CollapseIconEnd
)

// CollapseItem is antd ItemType (items API).
//
// Deprecated field aliases: Header≡Label, Content≡Children (old CollapsePanel).
type CollapseItem struct {
	Key         string
	Label       string
	Header      string // alias for Label when Label empty
	LabelNode   core.Node
	Children    core.Node
	Content     core.Node // alias for Children when Children nil
	Extra       core.Node
	ExpandIcon  core.Node // per-item static icon override (optional)
	ShowArrow   *bool     // nil → true
	Collapsible CollapseCollapsible
	ForceRender bool
	// Style optional per-item chrome (custom.tsx panelStyle).
	Style Style
}

// labelText resolves Label or Header alias.
func (it CollapseItem) labelText() string {
	if it.Label != "" {
		return it.Label
	}
	return it.Header
}

// bodyNode resolves Children or Content alias.
func (it CollapseItem) bodyNode() core.Node {
	if it.Children != nil {
		return it.Children
	}
	return it.Content
}

// showArrowResolved returns whether the arrow should render.
func (it CollapseItem) showArrowResolved() bool {
	if it.ShowArrow == nil {
		return true
	}
	return *it.ShowArrow
}

// CollapsePanel is a deprecated alias of CollapseItem (antd Collapse.Panel era).
// Prefer CollapseItem with Label/Children.
type CollapsePanel = CollapseItem

// Collapse is Ant Design Collapse (折叠面板).
//
//	Decorated root
//	  └─ Column
//	       └─ item Column
//	            header (Pressable regions) · body?
//
// Product contract: docs/antd/collapse.md §6 (P0 DoD).
// Root pointer stays stable across expand/collapse (ClearChildren).
type Collapse struct {
	Root *primitive.Decorated
	col  *primitive.Flex

	Items []CollapseItem

	// activeKeys is the current open set (ordered for onChange stability by item order).
	activeKeys []string
	// activeControlled: SetActiveKey was used (antd controlled activeKey).
	activeControlled bool
	defaultActiveKey []string
	appliedDefault   bool

	// Active mirrors open keys for legacy map access (c.Active["k"]).
	// Prefer IsActive / ActiveKeys. Do not mutate in place — use SetActive*.
	Active map[string]bool

	Accordion           bool
	Bordered            bool
	Ghost               bool
	Size                CollapseSize
	Collapsible         CollapseCollapsible
	ExpandIconPlacement CollapseIconPlacement
	DestroyOnHidden     bool
	// ExpandIcon builds a custom arrow; nil → default ▸/▾ text.
	ExpandIcon func(isActive bool, item CollapseItem) core.Node

	OnChange func(keys []string)

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// headerPress caches last-built header pressables for tests (key → press).
	headerPress map[string]*primitive.Pressable
	iconPress   map[string]*primitive.Pressable
}

// NewCollapse creates a Collapse with antd defaults (bordered, medium, collapsible header).
// Accepts CollapseItem or legacy CollapsePanel values.
func NewCollapse(items ...CollapseItem) *Collapse {
	c := &Collapse{
		Items:       append([]CollapseItem(nil), items...),
		Bordered:    true,
		Size:        CollapseMiddle,
		Collapsible: CollapseCollapsibleHeader,
	}
	c.rebuild()
	return c
}

// Node returns the mount root (stable Decorated).

// ensureBuilt materializes the control tree if missing (#9).
func (c *Collapse) ensureBuilt() {
	if c == nil {
		return
	}
	if c.Root == nil {
		c.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (c *Collapse) structureChange() {
	if c == nil {
		return
	}
	c.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (c *Collapse) chromeChange() {
	if c == nil {
		return
	}
	c.ensureBuilt()
	c.rebuild()
}

func (c *Collapse) Node() core.Node {
	if c == nil {
		return nil
	}
	c.ensureBuilt()
	return c.Root
}

// ItemsList returns a copy of items.
func (c *Collapse) ItemsList() []CollapseItem {
	if c == nil {
		return nil
	}
	return append([]CollapseItem(nil), c.Items...)
}

// SetItems replaces panels and rebuilds.
func (c *Collapse) SetItems(items ...CollapseItem) {
	if c == nil {
		return
	}
	c.Items = append([]CollapseItem(nil), items...)
	// Drop active keys that no longer exist.
	c.activeKeys = filterKeysInItems(c.activeKeys, c.Items)
	c.rebuild()
}

// ActiveKeys returns currently open keys in item order.
func (c *Collapse) ActiveKeys() []string {
	if c == nil {
		return nil
	}
	return c.orderedActive()
}

// IsActive reports whether key is expanded.
func (c *Collapse) IsActive(key string) bool {
	if c == nil || key == "" {
		return false
	}
	for _, k := range c.activeKeys {
		if k == key {
			return true
		}
	}
	return false
}

// syncActiveMap refreshes the public Active map from activeKeys.
func (c *Collapse) syncActiveMap() {
	if c == nil {
		return
	}
	m := make(map[string]bool, len(c.activeKeys))
	for _, k := range c.activeKeys {
		m[k] = true
	}
	c.Active = m
}

// SetActiveKey sets controlled activeKey (antd activeKey).
// While controlled, clicks fire OnChange but do not mutate until SetActiveKey again.
func (c *Collapse) SetActiveKey(keys ...string) {
	if c == nil {
		return
	}
	c.activeControlled = true
	c.activeKeys = collapseUniqueKeys(keys)
	if c.Accordion && len(c.activeKeys) > 1 {
		c.activeKeys = c.activeKeys[:1]
	}
	c.rebuild()
}

// SetActive programmatically opens keys without entering controlled mode
// (legacy helper used by gallery / root-stability tests).
func (c *Collapse) SetActive(keys ...string) {
	if c == nil {
		return
	}
	c.activeKeys = collapseUniqueKeys(keys)
	if c.Accordion && len(c.activeKeys) > 1 {
		c.activeKeys = c.activeKeys[:1]
	}
	c.rebuild()
}

// SetDefaultActiveKey sets non-controlled initial keys (antd defaultActiveKey).
func (c *Collapse) SetDefaultActiveKey(keys ...string) {
	if c == nil {
		return
	}
	c.defaultActiveKey = collapseUniqueKeys(keys)
	if !c.activeControlled && !c.appliedDefault {
		c.appliedDefault = true
		c.activeKeys = append([]string(nil), c.defaultActiveKey...)
		if c.Accordion && len(c.activeKeys) > 1 {
			c.activeKeys = c.activeKeys[:1]
		}
		c.rebuild()
	}
}

// SetAccordion enables hand-accordion mode (at most one open panel).
func (c *Collapse) SetAccordion(v bool) {
	if c == nil {
		return
	}
	c.Accordion = v
	if v && len(c.activeKeys) > 1 {
		c.activeKeys = c.activeKeys[:1]
	}
	c.rebuild()
}

// SetBordered toggles root border chrome (default true).
func (c *Collapse) SetBordered(v bool) {
	if c == nil {
		return
	}
	c.Bordered = v
	c.rebuild()
}

// SetGhost enables transparent borderless chrome.
func (c *Collapse) SetGhost(v bool) {
	if c == nil {
		return
	}
	c.Ghost = v
	c.rebuild()
}

// SetSize sets small | middle | large.
func (c *Collapse) SetSize(sz CollapseSize) {
	if c == nil {
		return
	}
	c.Size = sz
	c.rebuild()
}

// SetCollapsible sets the default trigger region for all items.
func (c *Collapse) SetCollapsible(mode CollapseCollapsible) {
	if c == nil {
		return
	}
	if mode == CollapseCollapsibleInherit {
		mode = CollapseCollapsibleHeader
	}
	c.Collapsible = mode
	c.rebuild()
}

// SetExpandIconPlacement sets start | end.
func (c *Collapse) SetExpandIconPlacement(p CollapseIconPlacement) {
	if c == nil {
		return
	}
	c.ExpandIconPlacement = p
	c.rebuild()
}

// SetExpandIcon sets a custom expand icon factory.
func (c *Collapse) SetExpandIcon(fn func(isActive bool, item CollapseItem) core.Node) {
	if c == nil {
		return
	}
	c.ExpandIcon = fn
	c.rebuild()
}

// SetDestroyOnHidden destroys body nodes when a panel collapses.
func (c *Collapse) SetDestroyOnHidden(v bool) {
	if c == nil {
		return
	}
	c.DestroyOnHidden = v
	c.rebuild()
}

// SetOnChange sets the activeKey change callback.
func (c *Collapse) SetOnChange(fn func(keys []string)) {
	if c == nil {
		return
	}
	c.OnChange = fn
}

// SetFace sets the font face for text chrome.
func (c *Collapse) SetFace(face text.Face) {
	if c == nil {
		return
	}
	c.Face = face
	c.rebuild()
}

// SetTheme sets an explicit theme override.
func (c *Collapse) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.rebuild()
}

// SetStyle sets optional visual overrides on the root.
func (c *Collapse) SetStyle(st Style) {
	if c == nil {
		return
	}
	c.Style = st
	c.applyChrome()
}

// SetAriaLabel sets the accessible name on the root group.
func (c *Collapse) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.AriaLabel = s
	c.applyA11y()
}

// HeaderPress returns the header (or icon) Pressable for key — test helper.
func (c *Collapse) HeaderPress(key string) *primitive.Pressable {
	if c == nil {
		return nil
	}
	if c.headerPress != nil {
		if p := c.headerPress[key]; p != nil {
			return p
		}
	}
	if c.iconPress != nil {
		return c.iconPress[key]
	}
	return nil
}

// IconPress returns the expand-icon Pressable when collapsible=icon.
func (c *Collapse) IconPress(key string) *primitive.Pressable {
	if c == nil || c.iconPress == nil {
		return nil
	}
	return c.iconPress[key]
}

// HeaderPadding returns resolved header EdgeInsets vertical/horizontal for assertions.
func (c *Collapse) HeaderPadding() (v, h float64) {
	if c == nil {
		return DefaultCollapseHeaderPadV, DefaultCollapseHeaderPadH
	}
	return c.headerPadResolved()
}

// ContentPadding returns resolved body padding (uniform for medium/small/large non-borderless).
func (c *Collapse) ContentPadding() float64 {
	if c == nil {
		return DefaultCollapseContentPad
	}
	return c.contentPadResolved()
}

// Radius returns resolved corner radius.
func (c *Collapse) Radius() float64 {
	if c == nil {
		return DefaultCollapseRadius
	}
	return c.radiusResolved()
}

// LineWidth returns resolved border width (0 when ghost or !bordered root stroke).
func (c *Collapse) LineWidth() float64 {
	if c == nil {
		return DefaultCollapseLineWidth
	}
	if c.Ghost || !c.Bordered {
		return 0
	}
	return c.lineWResolved()
}

// FontSize returns resolved panel font size.
func (c *Collapse) FontSize() float64 {
	if c == nil {
		return DefaultCollapseFontSize
	}
	return c.fontResolved()
}

// --- resolve helpers ---

func (c *Collapse) theme() *core.Theme {
	var n core.Node
	if c.Root != nil {
		n = c.Root
	}
	return themeOf(c.Theme, n)
}

func (c *Collapse) headerPadResolved() (v, h float64) {
	switch c.Size {
	case CollapseSmall:
		return DefaultCollapseHeaderPadVSM, DefaultCollapseHeaderPadHSM
	case CollapseLarge:
		return DefaultCollapseHeaderPadVLG, DefaultCollapseHeaderPadHLG
	default:
		return DefaultCollapseHeaderPadV, DefaultCollapseHeaderPadH
	}
}

func (c *Collapse) contentPadResolved() float64 {
	switch c.Size {
	case CollapseSmall:
		return DefaultCollapseContentPadSM
	case CollapseLarge:
		return DefaultCollapseContentPadLG
	default:
		return DefaultCollapseContentPad
	}
}

func (c *Collapse) radiusResolved() float64 {
	if c.Style.hasRadius() {
		return c.Style.Radius
	}
	return c.theme().SizeOr(core.TokenBorderRadiusLG, DefaultCollapseRadius)
}

func (c *Collapse) lineWResolved() float64 {
	return c.theme().SizeOr(core.TokenLineWidth, DefaultCollapseLineWidth)
}

func (c *Collapse) fontResolved() float64 {
	if c.Style.FontSize > 0 {
		return c.Style.FontSize
	}
	th := c.theme()
	if c.Size == CollapseLarge {
		return th.SizeOr(core.TokenFontSizeLG, DefaultCollapseFontSizeLG)
	}
	return th.SizeOr(core.TokenFontSize, DefaultCollapseFontSize)
}

func (c *Collapse) iconBoxHResolved() float64 {
	if c.Size == CollapseLarge {
		return DefaultCollapseIconBoxHLG
	}
	return DefaultCollapseIconBoxH
}

func (c *Collapse) itemCollapsible(it CollapseItem) CollapseCollapsible {
	mode := it.Collapsible
	if mode == CollapseCollapsibleInherit {
		mode = c.Collapsible
	}
	if mode == CollapseCollapsibleInherit {
		mode = CollapseCollapsibleHeader
	}
	// showArrow=false forbids collapsible=icon (antd).
	if mode == CollapseCollapsibleIcon && !it.showArrowResolved() {
		mode = CollapseCollapsibleHeader
	}
	return mode
}

func (c *Collapse) orderedActive() []string {
	if len(c.activeKeys) == 0 {
		return nil
	}
	set := make(map[string]bool, len(c.activeKeys))
	for _, k := range c.activeKeys {
		set[k] = true
	}
	out := make([]string, 0, len(c.activeKeys))
	for _, it := range c.Items {
		if set[it.Key] {
			out = append(out, it.Key)
			delete(set, it.Key)
		}
	}
	// Orphans (keys not in items) — stable sort leftover.
	if len(set) > 0 {
		rest := make([]string, 0, len(set))
		for k := range set {
			rest = append(rest, k)
		}
		sort.Strings(rest)
		out = append(out, rest...)
	}
	return out
}

func (c *Collapse) ensureDefaultApplied() {
	if c.activeControlled || c.appliedDefault {
		return
	}
	if len(c.defaultActiveKey) == 0 {
		return
	}
	c.appliedDefault = true
	c.activeKeys = append([]string(nil), c.defaultActiveKey...)
	if c.Accordion && len(c.activeKeys) > 1 {
		c.activeKeys = c.activeKeys[:1]
	}
}

// toggleKey flips key open/closed. Controlled mode only fires OnChange.
func (c *Collapse) toggleKey(key string) {
	if c == nil || key == "" {
		return
	}
	open := c.IsActive(key)
	var next []string
	if c.Accordion {
		if open {
			next = nil
		} else {
			next = []string{key}
		}
	} else if open {
		for _, k := range c.activeKeys {
			if k != key {
				next = append(next, k)
			}
		}
	} else {
		next = append(append([]string(nil), c.activeKeys...), key)
	}
	if c.activeControlled {
		if c.OnChange != nil {
			c.OnChange(orderKeysByItems(next, c.Items))
		}
		return
	}
	c.activeKeys = next
	c.rebuild()
	if c.OnChange != nil {
		c.OnChange(c.orderedActive())
	}
}

// --- build ---

func (c *Collapse) rebuild() {
	if c == nil {
		return
	}
	c.ensureDefaultApplied()
	c.syncActiveMap()
	th := c.theme()
	radius := c.radiusResolved()
	lineW := c.lineWResolved()
	font := c.fontResolved()
	headV, headH := c.headerPadResolved()
	contentPad := c.contentPadResolved()
	iconH := c.iconBoxHResolved()
	iconGap := DefaultCollapseIconGap

	c.headerPress = make(map[string]*primitive.Pressable)
	c.iconPress = make(map[string]*primitive.Pressable)

	if c.col == nil {
		c.col = primitive.Column()
	} else {
		c.col.ClearChildren()
	}
	c.col.Gap = 0
	c.col.CrossAlign = core.CrossStretch
	c.col.ExpandMax = true
	c.col.MainAlign = core.MainStart

	nItems := len(c.Items)
	for i, raw := range c.Items {
		it := raw
		key := it.Key
		open := c.IsActive(key)
		mode := c.itemCollapsible(it)
		disabled := mode == CollapseCollapsibleDisabled
		showArrow := it.showArrowResolved()
		isFirst := i == 0
		isLast := i == nItems-1

		// --- expand icon ---
		var iconNode core.Node
		if showArrow {
			if it.ExpandIcon != nil {
				iconNode = it.ExpandIcon
			} else if c.ExpandIcon != nil {
				iconNode = c.ExpandIcon(open, it)
			} else {
				arrow := "▸"
				if open {
					arrow = "▾"
				}
				lab := primitive.NewText(arrow)
				lab.FontSize = DefaultCollapseIconSize
				lab.Face = c.Face
				if disabled {
					lab.Color = th.Color(core.TokenColorDisabledText)
				} else {
					lab.Color = th.Color(core.TokenColorText)
				}
				iconNode = lab
			}
			iconBox := primitive.NewDecorated(iconNode)
			iconBox.MinHeight = iconH
			iconBox.Hit = core.HitDefer
			iconBox.Padding = primitive.EdgeInsets{}
			iconNode = iconBox
		}

		// --- title ---
		var titleNode core.Node
		if it.LabelNode != nil {
			titleNode = it.LabelNode
		} else {
			t := primitive.NewText(it.labelText())
			t.FontSize = font
			t.Face = c.Face
			if disabled {
				t.Color = th.Color(core.TokenColorDisabledText)
			} else if c.Style.hasText() {
				t.Color = c.Style.Text
			} else {
				t.Color = th.Color(core.TokenColorText)
			}
			titleNode = t
		}
		// Title flexes.
		titleWrap := primitive.NewFlexible(1, titleNode)
		titleWrap.FillChild = true

		// --- extra (outside toggle pressable so clicks don't collapse) ---
		var extraNode core.Node
		if it.Extra != nil {
			extraNode = primitive.NewSlot("collapse-extra-"+key, it.Extra)
		}

		// Assemble header row children by placement.
		buildRowKids := func(icon, title, extra core.Node) []core.Node {
			kids := make([]core.Node, 0, 4)
			if c.ExpandIconPlacement == CollapseIconEnd {
				if title != nil {
					kids = append(kids, title)
				}
				if extra != nil {
					kids = append(kids, extra)
				}
				if icon != nil {
					kids = append(kids, icon)
				}
			} else {
				if icon != nil {
					kids = append(kids, icon)
				}
				if title != nil {
					kids = append(kids, title)
				}
				if extra != nil {
					kids = append(kids, extra)
				}
			}
			return kids
		}

		var headerInner core.Node
		switch mode {
		case CollapseCollapsibleIcon:
			// Only icon is pressable; title + extra static.
			var iconPart core.Node
			if iconNode != nil {
				ip := primitive.NewPressable(iconNode)
				ip.Focusable = true
				ip.ShowFocusRing = true
				ip.FocusRingOutset = DefaultCollapseFocusRingOutset
				ip.EnableRipple = false
				ip.SetDisabled(false)
				k := key
				ip.Click = func() { c.toggleKey(k) }
				ip.Base().Role = "button"
				ip.Base().Label = it.labelText()
				c.iconPress[key] = ip
				iconPart = ip
			}
			row := primitive.Row(buildRowKids(iconPart, titleWrap, extraNode)...)
			row.CrossAlign = core.CrossStart
			row.Gap = iconGap
			row.ExpandMax = true
			headerInner = row

		case CollapseCollapsibleDisabled:
			row := primitive.Row(buildRowKids(iconNode, titleWrap, extraNode)...)
			row.CrossAlign = core.CrossStart
			row.Gap = iconGap
			row.ExpandMax = true
			// Non-interactive shell — still a decorated header for chrome.
			headerInner = row

		default: // header — title+icon toggle; extra never toggles
			k := key
			mkPress := func(child core.Node) *primitive.Pressable {
				pr := primitive.NewPressable(child)
				pr.Focusable = true
				pr.ShowFocusRing = true
				pr.FocusRingOutset = DefaultCollapseFocusRingOutset
				pr.FocusRingRadius = 2
				pr.EnableRipple = false
				pr.Click = func() { c.toggleKey(k) }
				pr.Base().Role = "button"
				pr.Base().Label = it.labelText()
				return pr
			}
			// Order start: [icon?][title][extra?] ; end: [title][extra?][icon?]
			var rowKids []core.Node
			if c.ExpandIconPlacement == CollapseIconEnd {
				titlePr := mkPress(titleWrap)
				c.headerPress[key] = titlePr
				rowKids = append(rowKids, titlePr)
				if extraNode != nil {
					rowKids = append(rowKids, extraNode)
				}
				if iconNode != nil {
					ip := mkPress(iconNode)
					c.iconPress[key] = ip
					rowKids = append(rowKids, ip)
				}
			} else {
				var toggle []core.Node
				if iconNode != nil {
					toggle = append(toggle, iconNode)
				}
				toggle = append(toggle, titleWrap)
				tr := primitive.Row(toggle...)
				tr.CrossAlign = core.CrossStart
				tr.Gap = iconGap
				tr.ExpandMax = true
				pr := mkPress(tr)
				c.headerPress[key] = pr
				rowKids = append(rowKids, pr)
				if extraNode != nil {
					rowKids = append(rowKids, extraNode)
				}
			}
			row := primitive.Row(rowKids...)
			row.CrossAlign = core.CrossStart
			row.Gap = iconGap
			row.ExpandMax = true
			headerInner = row
		}

		headDec := primitive.NewDecorated(headerInner)
		headDec.Padding = primitive.EdgeInsets{
			Top: headV, Bottom: headV, Left: headH, Right: headH,
		}
		headDec.ExpandWidth = true
		headDec.Hit = core.HitDefer
		if disabled {
			// cursor not-allowed equivalent: no press
		}

		// Per-item / root radius on first/last header corners when bordered.
		itemCol := primitive.Column(headDec)
		itemCol.Gap = 0
		itemCol.CrossAlign = core.CrossStretch
		itemCol.ExpandMax = true

		// --- body ---
		keepBody := open || it.ForceRender
		if c.DestroyOnHidden && !open && !it.ForceRender {
			keepBody = false
		}
		// Default without forceRender: closed panels omit body (same as destroy for tree).
		if !open && !it.ForceRender {
			keepBody = false
		}
		if keepBody {
			bodyChild := it.bodyNode()
			if bodyChild == nil {
				bodyChild = primitive.NewSlot("collapse-body-empty-"+key, nil)
			} else {
				bodyChild = primitive.NewSlot("collapse-body-"+key, bodyChild)
			}
			bodyDec := primitive.NewDecorated(bodyChild)
			bodyDec.ExpandWidth = true
			bodyDec.Hit = core.HitDefer
			if c.Ghost {
				bodyDec.Background = render.RGBA{}
				bodyDec.BorderWidth = 0
				bodyDec.Padding = primitive.EdgeInsets{
					Top: DefaultCollapseGhostBodyPadV, Bottom: DefaultCollapseGhostBodyPadV,
					Left: headH, Right: headH,
				}
			} else if !c.Bordered {
				bodyDec.Background = render.RGBA{} // borderlessContentBg transparent
				bodyDec.BorderWidth = 0
				bodyDec.Padding = primitive.EdgeInsets{
					Top:    DefaultCollapseBorderlessPadT,
					Bottom: DefaultCollapseBorderlessPadB,
					Left:   DefaultCollapseBorderlessPadH,
					Right:  DefaultCollapseBorderlessPadH,
				}
			} else {
				bodyDec.Background = th.Color(core.TokenColorBgContainer)
				bodyDec.BorderWidth = 0
				// Top divider simulated via a thin sibling line.
				bodyDec.Padding = primitive.All(contentPad)
			}
			if !open && it.ForceRender {
				// Keep mounted but collapse layout contribution.
				bodyDec.Height = 0
				bodyDec.MinHeight = 0
			}
			if c.Bordered && !c.Ghost && open {
				div := primitive.NewDecorated()
				div.Height = lineW
				div.ExpandWidth = true
				div.Background = th.Color(core.TokenColorBorder)
				div.Hit = core.HitDefer
				itemCol.AddChild(div)
			}
			itemCol.AddChild(bodyDec)
		}

		// Item chrome: bottom border between items (bordered & borderless); ghost none.
		itemShell := primitive.NewDecorated(itemCol)
		itemShell.ExpandWidth = true
		itemShell.Hit = core.HitDefer
		if it.Style.hasBG() {
			itemShell.Background = it.Style.Background
		}
		if it.Style.hasRadius() {
			itemShell.Radius = it.Style.Radius
		}
		if it.Style.hasBorder() {
			itemShell.BorderWidth = lineW
			itemShell.BorderColor = it.Style.Border
		}
		// Segment borders.
		if !c.Ghost {
			if c.Bordered {
				// item bottom border except last (antd last has borderBottom 0; root supplies outer).
				if !isLast {
					// Approximate bottom edge with a trailing hairline child.
					hair := primitive.NewDecorated()
					hair.Height = lineW
					hair.ExpandWidth = true
					hair.Background = th.Color(core.TokenColorBorder)
					hair.Hit = core.HitDefer
					itemCol.AddChild(hair)
				}
				// First/last corner radius on shell when single continuous root.
				if isFirst && isLast {
					itemShell.Radius = radius
				} else if isFirst {
					// top radii only — Decorated has uniform radius; root carries outer radius.
				} else if isLast {
					// bottom radii on root
				}
			} else {
				// borderless: item bottom border except last
				if !isLast {
					hair := primitive.NewDecorated()
					hair.Height = lineW
					hair.ExpandWidth = true
					hair.Background = th.Color(core.TokenColorBorder)
					hair.Hit = core.HitDefer
					itemCol.AddChild(hair)
				}
			}
		}

		c.col.AddChild(itemShell)
		_ = isFirst
	}

	if c.Root == nil {
		c.Root = primitive.NewDecorated(c.col)
		c.Root.SkinType = TypeCollapse
	} else {
		// Keep Root identity: replace single child.
		c.Root.ClearChildren()
		c.Root.AddChild(c.col)
	}
	c.Root.ExpandWidth = true
	c.Root.Hit = core.HitBlock
	c.Root.Radius = radius
	c.applyChrome()
	c.applyA11y()
	c.Root.SetThemeHook(func(*core.Theme) { c.rebuild() })
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
}

func (c *Collapse) applyChrome() {
	if c == nil || c.Root == nil {
		return
	}
	th := c.theme()
	radius := c.radiusResolved()
	lineW := c.lineWResolved()
	c.Root.Radius = radius

	switch {
	case c.Ghost:
		c.Root.Background = render.RGBA{}
		c.Root.BorderWidth = 0
		c.Root.BorderColor = render.RGBA{}
	case !c.Bordered:
		// borderless: headerBg fill, no outer border
		bg := th.Color(core.TokenColorFillSecondary)
		if c.Style.hasBG() {
			bg = c.Style.Background
		}
		c.Root.Background = bg
		c.Root.BorderWidth = 0
		c.Root.BorderColor = render.RGBA{}
	default:
		bg := th.Color(core.TokenColorFillSecondary) // headerBg
		if c.Style.hasBG() {
			bg = c.Style.Background
		}
		border := th.Color(core.TokenColorBorder)
		if c.Style.hasBorder() {
			border = c.Style.Border
		}
		c.Root.Background = bg
		c.Root.BorderWidth = lineW
		c.Root.BorderColor = border
	}
}

func (c *Collapse) applyA11y() {
	if c == nil || c.Root == nil {
		return
	}
	c.Root.Base().Role = "group"
	c.Root.Base().Label = c.AriaLabel
}

// --- helpers ---

func collapseUniqueKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func filterKeysInItems(keys []string, items []CollapseItem) []string {
	if len(keys) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, it := range items {
		if it.Key != "" {
			set[it.Key] = true
		}
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if set[k] {
			out = append(out, k)
		}
	}
	return out
}

func orderKeysByItems(keys []string, items []CollapseItem) []string {
	if len(keys) == 0 {
		return nil
	}
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	out := make([]string, 0, len(keys))
	for _, it := range items {
		if set[it.Key] {
			out = append(out, it.Key)
			delete(set, it.Key)
		}
	}
	if len(set) > 0 {
		rest := make([]string, 0, len(set))
		for k := range set {
			rest = append(rest, k)
		}
		sort.Strings(rest)
		out = append(out, rest...)
	}
	return out
}
