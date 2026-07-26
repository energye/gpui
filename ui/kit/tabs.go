package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Tabs defaults — docs/antd/tabs.md §6.2 / §6.10
// Source: components/tabs/style prepareComponentToken
// horizontalItemGutter=32; cardHeight=controlHeightLG=40.
const (
	DefaultTabsHorizontalGutter = 32.0
	DefaultTabsCardHeight       = 40.0 // controlHeightLG
	DefaultTabsCardHeightSM     = 32.0 // controlHeight
	DefaultTabsCardHeightLG     = 48.0 // controlHeightLG+8
	DefaultTabsInkThickness     = 2.0
	DefaultTabsInkThicknessVert = 3.0
	DefaultTabsPadInline        = 0.0 // line: gutter between items, not pad
	DefaultTabsPadBlock         = 12.0
	DefaultTabsRailWidth        = 160.0 // kit left-rail shell
	DefaultTabsInkDuration      = 0.22
	DefaultTabsFocusOutset      = 1.5
	DefaultTabsIconGap          = 8.0
	DefaultTabsCloseSize        = 16.0
	DefaultTabsAddSize          = 32.0
)

// TabsType is antd type: line | card | editable-card.
type TabsType int

const (
	TabsLine TabsType = iota
	TabsCard
	TabsEditableCard
)

// TabsSize is antd size: large | medium | small.
type TabsSize int

const (
	// TabsMiddle is medium (default).
	TabsMiddle TabsSize = iota
	TabsSmall
	TabsLarge
)

// TabsPlacement is antd tabPlacement / tabPosition.
type TabsPlacement int

const (
	TabsTop TabsPlacement = iota
	TabsBottom
	TabsLeft
	TabsRight
)

// Legacy aliases used by gallery shell and older call sites.
const (
	TabTop  = TabsTop
	TabLeft = TabsLeft
)

// TabPosition is a legacy alias for TabsPlacement.
type TabPosition = TabsPlacement

// TabsIndicatorAlign is antd indicator.align.
type TabsIndicatorAlign int

const (
	TabsIndicatorCenter TabsIndicatorAlign = iota
	TabsIndicatorStart
	TabsIndicatorEnd
)

// TabsIndicator customizes the line ink bar (antd indicator).
type TabsIndicator struct {
	// Size is absolute length; 0 → use laid-out tab span (or SizeFn).
	Size float64
	// SizeFn maps origin tab span → ink span (takes precedence when non-nil).
	SizeFn func(origin float64) float64
	Align  TabsIndicatorAlign
}

// TabItem is antd TabItemType (+ kit rail helpers).
type TabItem struct {
	Key      string
	Label    string
	Children core.Node // panel body
	Icon     string
	IconNode core.Node
	Disabled bool
	// Closable: nil → editable-card default true; false hides close.
	Closable *bool
	// ForceRender keeps panel mounted even when inactive (when DestroyOnHidden).
	ForceRender bool
	// Divider: kit extension — thin separator row in the rail (catalog).
	Divider bool
}

// Selectable reports whether the item can become ActiveKey.
func (it TabItem) Selectable() bool {
	if it.Divider || it.Disabled {
		return false
	}
	return it.Key != ""
}

// closable reports whether the close affordance should show.
func (it TabItem) closable(typ TabsType) bool {
	if typ != TabsEditableCard {
		return false
	}
	if it.Closable != nil {
		return *it.Closable
	}
	return true
}

// Tabs is Ant Design Tabs (navigation tab list + panel).
//
//	Flex root (role=tablist host)
//	  bar (scroll) · divider · body (scroll)
//
// Product contract: docs/antd/tabs.md §6 (P0 DoD).
// hit == layout == paint; ink slides via AttachTicker when animated.
type Tabs struct {
	Root *primitive.Flex

	barList    *primitive.Flex
	barStack   *tabsBarHost
	body       *primitive.Slot
	bodyStack  *primitive.Stack // when !DestroyOnHidden
	rail       *primitive.Decorated
	barScroll  *primitive.ScrollViewport
	bodyScroll *primitive.ScrollViewport
	barHost    *primitive.Decorated

	// ink indicator
	ink            *primitive.Box
	inkNode        core.Node
	inkAlong       float64
	inkSpan        float64
	inkAlongFrom   float64
	inkAlongTo     float64
	inkSpanFrom    float64
	inkSpanTo      float64
	inkT           float64 // 0..1 animating; <0 idle
	inkSlots       []inkSlot
	inkContentMain float64

	Items []TabItem
	// ActiveKey is the current panel key (antd activeKey).
	ActiveKey string

	Type      TabsType
	Size      TabsSize
	Placement TabsPlacement

	// Position is a legacy alias field for Placement (gallery writes Position / SetPosition).
	// Prefer Placement / SetPlacement.
	Position TabsPlacement

	Centered        bool
	HideAdd         bool
	DestroyOnHidden bool
	TabBarGutter    float64 // 0 → DefaultTabsHorizontalGutter for horizontal line
	Indicator       TabsIndicator
	ExtraLeft       core.Node
	ExtraRight      core.Node
	HideInk         bool
	InkAnimated     bool
	inkAnimSet      bool
	InkDuration     float64
	TabInkWidth     float64
	TabInkColor     render.RGBA
	TabWidth        float64 // left/right rail (0 → DefaultTabsRailWidth)
	TabItemHeight   float64 // 0 → size ladder; <0 hug
	TabPadInline    float64
	TabPadBlock     float64
	BodyPadding     primitive.EdgeInsets
	bodyPadSet      bool

	Face      text.Face
	Theme     *core.Theme
	AriaLabel string
	Nav       *core.KeyboardNav

	OnChange   func(key string)
	OnEdit     func(targetKey, action string) // action: "add" | "remove"
	OnTabClick func(key string)

	// Controlled / default
	activeControlled bool
	defaultActiveKey string
	appliedDefault   bool

	// Test hooks
	itemPress  map[string]*primitive.Pressable
	itemHost   map[string]*primitive.Decorated
	addPress   *primitive.Pressable
	closePress map[string]*primitive.Pressable

	// side content map (SetContent); merged over Items[].Children
	contents map[string]core.Node
	// force-mounted inactive panels
	mounted map[string]core.Node

	tree *core.Tree
}

type inkSlot struct {
	key         string
	along, span float64
}

// tabsBarHost is Stack(barList, ink) that syncs ink after layout and paints on top.
type tabsBarHost struct {
	primitive.Stack
	tabs *Tabs
}

func (h *tabsBarHost) TypeID() string { return TypeTabs }

func (h *tabsBarHost) Layout(c core.Constraints) core.Size {
	sz := h.Stack.Layout(c)
	if h.tabs != nil {
		h.tabs.syncInkFromLaidOutBar()
	}
	return sz
}

func (h *tabsBarHost) Paint(pc *core.PaintContext) {
	h.Stack.Paint(pc)
	if h.tabs != nil {
		h.tabs.paintInk(pc)
	}
}

// NewTabs creates Tabs (placement top, type line, size middle).
func NewTabs(items ...TabItem) *Tabs {
	t := &Tabs{
		Items:       append([]TabItem(nil), items...),
		contents:    make(map[string]core.Node),
		mounted:     make(map[string]core.Node),
		Placement:   TabsTop,
		Position:    TabsTop,
		Type:        TabsLine,
		Size:        TabsMiddle,
		InkAnimated: true,
		inkAnimSet:  true,
		inkT:        -1,
	}
	t.Nav = core.NewKeyboardNav(core.NavHorizontal, 0)
	t.ActiveKey = firstSelectableTabKey(t.Items)
	t.rebuild()
	return t
}

func firstSelectableTabKey(items []TabItem) string {
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
	return firstSelectableTabKey(t.Items)
}

// Node returns the stable root.

// ensureBuilt materializes the control tree if missing (#9).
func (t *Tabs) ensureBuilt() {
	if t == nil {
		return
	}
	if t.Root == nil {
		t.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (t *Tabs) structureChange() {
	if t == nil {
		return
	}
	t.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (t *Tabs) chromeChange() {
	if t == nil {
		return
	}
	t.ensureBuilt()
	t.rebuild()
}

func (t *Tabs) Node() core.Node {
	if t == nil {
		return nil
	}
	t.ensureBuilt()
	return t.Root
}

// ChromeNode returns the root (a11y host).
func (t *Tabs) ChromeNode() core.Node { return t.Node() }

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

// SetItems replaces tab items.
func (t *Tabs) SetItems(items []TabItem) {
	if t == nil {
		return
	}
	t.Items = append([]TabItem(nil), items...)
	if !t.activeControlled {
		if t.ActiveKey == "" || !t.hasSelectableKey(t.ActiveKey) {
			t.ActiveKey = firstSelectableTabKey(t.Items)
		}
	}
	t.rebuild()
	t.syncBody()
}

// SetContent associates a panel with a tab key (writes side map + item Children when present).
func (t *Tabs) SetContent(key string, n core.Node) {
	if t == nil {
		return
	}
	if t.contents == nil {
		t.contents = make(map[string]core.Node)
	}
	t.contents[key] = n
	for i := range t.Items {
		if t.Items[i].Key == key {
			t.Items[i].Children = n
			break
		}
	}
	if key == t.ActiveKey {
		t.syncBody()
	}
}

// Content returns the resolved panel for key.
func (t *Tabs) Content(key string) core.Node {
	if t == nil {
		return nil
	}
	if n, ok := t.contents[key]; ok && n != nil {
		return n
	}
	for _, it := range t.Items {
		if it.Key == key {
			return it.Children
		}
	}
	return nil
}

// SetActiveKey sets controlled activeKey (antd activeKey).
// While controlled, clicks fire OnChange but do not mutate ActiveKey until
// the parent calls SetActiveKey again.
func (t *Tabs) SetActiveKey(key string) {
	if t == nil {
		return
	}
	t.activeControlled = true
	t.applyActive(key, false)
}

// SetActive programmatically switches the panel without entering controlled mode
// (legacy kit helper; gallery / tests). Prefer SetActiveKey for antd-controlled
// activeKey, or rely on defaultActiveKey + clicks for uncontrolled.
func (t *Tabs) SetActive(key string) {
	if t == nil {
		return
	}
	t.applyActive(key, false)
}

// SetDefaultActiveKey sets non-controlled initial key (antd defaultActiveKey).
func (t *Tabs) SetDefaultActiveKey(key string) {
	if t == nil {
		return
	}
	t.defaultActiveKey = key
	if !t.activeControlled && !t.appliedDefault {
		t.appliedDefault = true
		if key != "" {
			t.ActiveKey = key
		}
		t.rebuildBar()
		t.syncBody()
		t.markLayoutDirty()
	}
}

// Active returns ActiveKey (legacy accessor used by older tests).
func (t *Tabs) Active() string {
	if t == nil {
		return ""
	}
	return t.ActiveKey
}

// SetType sets line | card | editable-card.
func (t *Tabs) SetType(typ TabsType) {
	if t == nil {
		return
	}
	t.Type = typ
	t.rebuild()
	t.syncBody()
}

// SetSize sets small | middle | large.
func (t *Tabs) SetSize(sz TabsSize) {
	if t == nil {
		return
	}
	t.Size = sz
	t.rebuild()
	t.syncBody()
}

// SetPlacement sets tabPlacement (top/bottom/left/right).
func (t *Tabs) SetPlacement(p TabsPlacement) {
	if t == nil {
		return
	}
	t.Placement = p
	t.Position = p
	if t.isVertical() {
		t.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	} else {
		t.Nav = core.NewKeyboardNav(core.NavHorizontal, 0)
	}
	t.inkT = -1
	t.rebuild()
	t.syncBody()
}

// SetPosition is a legacy alias for SetPlacement.
func (t *Tabs) SetPosition(pos TabsPlacement) { t.SetPlacement(pos) }

// SetCentered centers the tab list on horizontal bars.
func (t *Tabs) SetCentered(v bool) {
	if t == nil {
		return
	}
	t.Centered = v
	t.rebuild()
}

// SetIndicator sets line ink size/align.
func (t *Tabs) SetIndicator(ind TabsIndicator) {
	if t == nil {
		return
	}
	t.Indicator = ind
	t.markDirty()
	if t.barStack != nil {
		t.barStack.MarkNeedsLayout()
	}
}

// SetTabBarGutter sets gap between tabs (0 → default 32 for horizontal line).
func (t *Tabs) SetTabBarGutter(px float64) {
	if t == nil {
		return
	}
	t.TabBarGutter = px
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetTabBarExtraContent sets left/right extra nodes on the bar.
func (t *Tabs) SetTabBarExtraContent(left, right core.Node) {
	if t == nil {
		return
	}
	t.ExtraLeft, t.ExtraRight = left, right
	t.rebuild()
	t.syncBody()
}

// SetHideAdd hides the add button on editable-card.
func (t *Tabs) SetHideAdd(v bool) {
	if t == nil {
		return
	}
	t.HideAdd = v
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetDestroyOnHidden toggles unmount of inactive panels.
func (t *Tabs) SetDestroyOnHidden(v bool) {
	if t == nil {
		return
	}
	t.DestroyOnHidden = v
	if v {
		t.mounted = make(map[string]core.Node)
	}
	t.rebuild()
	t.syncBody()
}

// SetOnChange sets onChange(activeKey).
func (t *Tabs) SetOnChange(fn func(key string)) {
	if t != nil {
		t.OnChange = fn
	}
}

// SetOnEdit sets onEdit for editable-card (action: "add"|"remove").
func (t *Tabs) SetOnEdit(fn func(targetKey, action string)) {
	if t != nil {
		t.OnEdit = fn
	}
}

// SetOnTabClick sets onTabClick.
func (t *Tabs) SetOnTabClick(fn func(key string)) {
	if t != nil {
		t.OnTabClick = fn
	}
}

// SetTheme sets design tokens.
func (t *Tabs) SetTheme(th *core.Theme) {
	if t == nil {
		return
	}
	t.Theme = th
	t.rebuild()
	t.syncBody()
}

// SetFace sets text face.
func (t *Tabs) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetAriaLabel sets accessible name on the tablist root.
func (t *Tabs) SetAriaLabel(name string) {
	if t == nil {
		return
	}
	t.AriaLabel = name
	if t.Root != nil {
		t.Root.Base().Label = name
	}
}

// SetTabWidth sets left/right rail width (0 → default 160).
func (t *Tabs) SetTabWidth(w float64) {
	if t == nil {
		return
	}
	t.TabWidth = w
	t.rebuild()
	t.syncBody()
}

// SetTabItemHeight sets fixed item height (0 → size ladder; <0 hug).
func (t *Tabs) SetTabItemHeight(h float64) {
	if t == nil {
		return
	}
	t.TabItemHeight = h
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetBodyPadding sets body panel insets (explicit, including zero).
func (t *Tabs) SetBodyPadding(p primitive.EdgeInsets) {
	if t == nil {
		return
	}
	t.BodyPadding = p
	t.bodyPadSet = true
	t.rebuild()
	t.syncBody()
}

// SetInkSize sets indicator thickness.
func (t *Tabs) SetInkSize(px float64) {
	if t == nil {
		return
	}
	t.TabInkWidth = px
	t.rebuildBar()
	t.markLayoutDirty()
}

// SetInkColor sets indicator color (A=0 → theme primary).
func (t *Tabs) SetInkColor(c render.RGBA) {
	if t == nil {
		return
	}
	t.TabInkColor = c
	t.applyInkChrome()
	t.markDirty()
}

// SetInkAnimated enables/disables sliding ink.
func (t *Tabs) SetInkAnimated(v bool) {
	if t == nil {
		return
	}
	t.InkAnimated = v
	t.inkAnimSet = true
}

// ItemPressable returns the pressable for a key (tests).
func (t *Tabs) ItemPressable(key string) *primitive.Pressable {
	if t == nil || t.itemPress == nil {
		return nil
	}
	return t.itemPress[key]
}

// ItemHost returns the decorated host for a key (size/layout asserts).
func (t *Tabs) ItemHost(key string) *primitive.Decorated {
	if t == nil || t.itemHost == nil {
		return nil
	}
	return t.itemHost[key]
}

// ClosePressable returns the close pressable for editable-card tests.
func (t *Tabs) ClosePressable(key string) *primitive.Pressable {
	if t == nil || t.closePress == nil {
		return nil
	}
	return t.closePress[key]
}

// AddPressable returns the add button pressable.
func (t *Tabs) AddPressable() *primitive.Pressable {
	if t == nil {
		return nil
	}
	return t.addPress
}

// ItemHeight returns resolved tab item height for current size.
func (t *Tabs) ItemHeight() float64 {
	if t == nil {
		return DefaultTabsCardHeight
	}
	return t.tabItemHeight()
}

// HorizontalGutter returns resolved gutter between horizontal tabs.
func (t *Tabs) HorizontalGutter() float64 {
	if t == nil {
		return DefaultTabsHorizontalGutter
	}
	return t.gutter()
}

// IsCard reports card or editable-card chrome.
func (t *Tabs) IsCard() bool {
	return t != nil && (t.Type == TabsCard || t.Type == TabsEditableCard)
}

// InkVisible reports whether the line indicator is shown (tests / chrome).
func (t *Tabs) InkVisible() bool {
	if t == nil {
		return false
	}
	return t.inkVisible()
}

// BarCentered reports horizontal bar MainCenter (centered prop).
func (t *Tabs) BarCentered() bool {
	if t == nil || t.barList == nil {
		return false
	}
	return t.barList.MainAlign == core.MainCenter
}

// IsMounted reports whether a panel node is currently under the body (destroyOnHidden tests).
func (t *Tabs) IsMounted(key string) bool {
	if t == nil {
		return false
	}
	if t.DestroyOnHidden {
		return key == t.ActiveKey && t.Content(key) != nil
	}
	if _, ok := t.mounted[key]; ok {
		return true
	}
	return key == t.ActiveKey
}

// HandleKey routes arrow keys among selectable tabs (host may call after focus).
func (t *Tabs) HandleKey(ev *core.KeyEvent) bool {
	if t == nil || ev == nil || ev.Type != core.KeyDown {
		return false
	}
	keys := t.selectableKeys()
	if len(keys) == 0 {
		return false
	}
	// Map Nav index to selectable list.
	if t.Nav == nil {
		t.Nav = core.NewKeyboardNav(core.NavHorizontal, len(keys))
	}
	t.Nav.SetCount(len(keys))
	// Sync index to current active
	for i, k := range keys {
		if k == t.ActiveKey {
			t.Nav.Index = i
			break
		}
	}
	prev := t.Nav.Index
	if !t.Nav.HandleKey(ev.Key) {
		// Enter/Space activate current (already active)
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "Space" {
			return true
		}
		return false
	}
	if t.Nav.Index != prev && t.Nav.Index >= 0 && t.Nav.Index < len(keys) {
		t.activate(keys[t.Nav.Index])
		return true
	}
	return true
}

// Tick advances ink slide animation.
func (t *Tabs) Tick(dt float64) bool {
	if t == nil || t.inkT < 0 {
		return false
	}
	dur := t.InkDuration
	if dur <= 0 {
		dur = DefaultTabsInkDuration
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

// --- internals ---

func (t *Tabs) theme() *core.Theme {
	var n core.Node
	if t.Root != nil {
		n = t.Root
	}
	return themeOf(t.Theme, n)
}

func (t *Tabs) isVertical() bool {
	return t.Placement == TabsLeft || t.Placement == TabsRight ||
		t.Position == TabsLeft || t.Position == TabsRight
}

func (t *Tabs) placement() TabsPlacement {
	// Prefer Placement; fall back to Position for legacy field writes.
	if t.Placement != TabsTop {
		return t.Placement
	}
	if t.Position != TabsTop {
		return t.Position
	}
	return TabsTop
}

func (t *Tabs) hasSelectableKey(key string) bool {
	for _, it := range t.Items {
		if it.Key == key && it.Selectable() {
			return true
		}
	}
	return false
}

func (t *Tabs) selectableKeys() []string {
	var out []string
	for _, it := range t.Items {
		if it.Selectable() {
			out = append(out, it.Key)
		}
	}
	return out
}

func (t *Tabs) gutter() float64 {
	if t.TabBarGutter > 0 {
		return t.TabBarGutter
	}
	if t.isVertical() || t.IsCard() {
		return 0
	}
	return DefaultTabsHorizontalGutter
}

func (t *Tabs) tabWidth() float64 {
	if t.TabWidth > 0 {
		return t.TabWidth
	}
	return DefaultTabsRailWidth
}

func (t *Tabs) tabItemHeight() float64 {
	if t.TabItemHeight < 0 {
		return 0
	}
	if t.TabItemHeight > 0 {
		return t.TabItemHeight
	}
	switch t.Size {
	case TabsSmall:
		if t.IsCard() {
			return DefaultTabsCardHeightSM
		}
		return t.theme().SizeOr(core.TokenControlHeightSM, 24)
	case TabsLarge:
		if t.IsCard() {
			return DefaultTabsCardHeightLG
		}
		return t.theme().SizeOr(core.TokenControlHeightLG, 40)
	default:
		if t.IsCard() {
			return DefaultTabsCardHeight
		}
		// line middle ~ controlHeight + some chrome → use cardHeight rhythm
		return DefaultTabsCardHeight
	}
}

func (t *Tabs) padInline() float64 {
	if t.TabPadInline > 0 {
		return t.TabPadInline
	}
	if t.IsCard() {
		switch t.Size {
		case TabsSmall:
			return 8
		case TabsLarge:
			return 16
		default:
			return 16
		}
	}
	// line: horizontal spacing via gutter; keep modest inner pad
	return 0
}

func (t *Tabs) padBlock() float64 {
	if t.TabPadBlock > 0 {
		return t.TabPadBlock
	}
	h := t.tabItemHeight()
	fs := t.fontSize()
	// vertical pad so total height ≈ item height
	pad := (h - fs*1.2) / 2
	if pad < 4 {
		pad = 4
	}
	if pad > 16 {
		pad = 16
	}
	return pad
}

func (t *Tabs) fontSize() float64 {
	th := t.theme()
	switch t.Size {
	case TabsSmall:
		return th.SizeOr(core.TokenFontSize, 14)
	case TabsLarge:
		return th.SizeOr(core.TokenFontSizeLG, 16)
	default:
		return th.SizeOr(core.TokenFontSize, 14)
	}
}

func (t *Tabs) tabInkWidth() float64 {
	if t.TabInkWidth > 0 {
		return t.TabInkWidth
	}
	if t.isVertical() {
		return DefaultTabsInkThicknessVert
	}
	return DefaultTabsInkThickness
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
	if t.IsCard() {
		return false
	}
	return true
}

func (t *Tabs) inkAnimated() bool {
	if t.inkAnimSet {
		return t.InkAnimated
	}
	return true
}

func (t *Tabs) bodyPadding() primitive.EdgeInsets {
	if t != nil && t.bodyPadSet {
		return t.BodyPadding
	}
	return primitive.EdgeInsets{}
}

func (t *Tabs) applyActive(key string, fireChange bool) {
	if key == "" {
		return
	}
	for _, it := range t.Items {
		if it.Key == key && !it.Selectable() {
			return
		}
	}
	changed := t.ActiveKey != key
	prev := t.ActiveKey
	t.ActiveKey = key
	t.syncBody()

	if changed && t.inkAnimated() && t.inkSlots != nil {
		from, to := t.slotOf(prev), t.slotOf(key)
		if from != nil && to != nil {
			t.inkAlongFrom, t.inkSpanFrom = t.inkAlong, t.inkSpan
			if t.inkT < 0 {
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
	if fireChange && changed && t.OnChange != nil {
		t.OnChange(key)
	}
}

// activate is user interaction: respects controlled mode.
func (t *Tabs) activate(key string) {
	if t == nil || key == "" {
		return
	}
	if t.OnTabClick != nil {
		t.OnTabClick(key)
	}
	if t.activeControlled {
		if t.OnChange != nil && key != t.ActiveKey {
			t.OnChange(key)
		}
		return
	}
	t.applyActive(key, true)
}

func (t *Tabs) syncBody() {
	if t == nil {
		return
	}
	active := t.Content(t.ActiveKey)
	if t.DestroyOnHidden {
		if t.body != nil {
			t.body.SetChild(active)
		}
		t.mounted = map[string]core.Node{}
		if active != nil {
			t.mounted[t.ActiveKey] = active
		}
		t.markLayoutDirty()
		return
	}
	// Keep previously seen panels mounted (forceRender-ish + non-destroy).
	if active != nil {
		t.mounted[t.ActiveKey] = active
	}
	for _, it := range t.Items {
		if it.ForceRender {
			if n := t.Content(it.Key); n != nil {
				t.mounted[it.Key] = n
			}
		}
	}
	if t.bodyStack != nil {
		t.bodyStack.ClearChildren()
		// Active on top
		if active != nil {
			t.bodyStack.AddChild(active)
		}
	} else if t.body != nil {
		t.body.SetChild(active)
	}
	t.markLayoutDirty()
}

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

func (t *Tabs) markLayoutDirty() {
	if t.Root != nil {
		t.Root.MarkNeedsLayout()
		t.Root.MarkNeedsPaint()
	}
}

func (t *Tabs) slotOf(key string) *inkSlot {
	for i := range t.inkSlots {
		if t.inkSlots[i].key == key {
			return &t.inkSlots[i]
		}
	}
	return nil
}

func (t *Tabs) rebuild() {
	th := t.theme()
	// Keep Placement/Position in sync
	pl := t.placement()
	t.Placement = pl
	t.Position = pl

	if t.isVertical() {
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

	// Body host: slot (destroy) or stack (keep)
	t.body = primitive.NewSlot("tab-body", t.Content(t.ActiveKey))
	t.body.ExpandFill = true
	if !t.DestroyOnHidden {
		t.bodyStack = primitive.NewStack()
		t.bodyStack.Fit = false
		if n := t.Content(t.ActiveKey); n != nil {
			t.bodyStack.AddChild(n)
		}
	} else {
		t.bodyStack = nil
	}

	t.itemPress = make(map[string]*primitive.Pressable)
	t.itemHost = make(map[string]*primitive.Decorated)
	t.closePress = make(map[string]*primitive.Pressable)
	t.addPress = nil
	t.rebuildBar()

	t.barScroll = primitive.NewScrollViewport(t.barStack)
	var bodyChild core.Node = t.body
	if t.bodyStack != nil {
		bodyChild = t.bodyStack
	}
	t.bodyScroll = primitive.NewScrollViewport(bodyChild)
	if t.isVertical() {
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

	// Bar chrome with optional extra left/right
	barInner := t.wrapBarWithExtra(t.barScroll)
	t.barHost = primitive.NewDecorated(barInner)
	t.barHost.StretchChild = true
	t.barHost.Background = th.Color(core.TokenColorBgContainer)
	t.barHost.BorderWidth = 0

	bodyNode := t.wrapBody()

	switch pl {
	case TabsLeft:
		t.Root = primitive.Row(t.buildRail(th), t.vDivider(th), bodyNode)
		t.Root.Gap = 0
		t.Root.SkinType = TypeTabs
		t.Root.CrossAlign = core.CrossStretch
	case TabsRight:
		t.Root = primitive.Row(bodyNode, t.vDivider(th), t.buildRail(th))
		t.Root.Gap = 0
		t.Root.CrossAlign = core.CrossStretch
	case TabsBottom:
		t.Root = primitive.Column(bodyNode, t.hDivider(th), t.barHost)
		t.Root.Gap = 0
		t.Root.CrossAlign = core.CrossStretch
	default: // top
		t.Root = primitive.Column(t.barHost, t.hDivider(th), bodyNode)
		t.Root.Gap = 0
		t.Root.CrossAlign = core.CrossStretch
	}
	t.Root.Base().Role = "tablist"
	if t.AriaLabel != "" {
		t.Root.Base().Label = t.AriaLabel
	}
	t.syncBody()
}

func (t *Tabs) buildRail(th *core.Theme) core.Node {
	railW := t.tabWidth()
	t.rail = primitive.NewDecorated(t.barScroll)
	t.rail.Width = railW
	t.rail.MinWidth = railW
	t.rail.Background = th.Color(core.TokenColorBgContainer)
	t.rail.BorderWidth = 0
	t.rail.Padding = primitive.EdgeInsets{}
	t.rail.StretchChild = true
	t.rail.Hit = core.HitBlock
	return t.rail
}

func (t *Tabs) wrapBody() core.Node {
	th := t.theme()
	padBody := primitive.NewDecorated(t.bodyScroll)
	padBody.Padding = t.bodyPadding()
	if t.isVertical() {
		padBody.Background = th.Color(core.TokenColorBgLayout)
	} else {
		padBody.Background = th.Color(core.TokenColorBgContainer)
	}
	padBody.BorderWidth = 0
	padBody.StretchChild = true
	padBody.Hit = core.HitBlock
	bodyHost := primitive.NewFlexible(1, padBody)
	bodyHost.FillChild = true
	return bodyHost
}

func (t *Tabs) wrapBarWithExtra(bar core.Node) core.Node {
	if t.ExtraLeft == nil && t.ExtraRight == nil {
		return bar
	}
	row := primitive.Row()
	row.CrossAlign = core.CrossCenter
	row.Gap = 8
	if t.ExtraLeft != nil {
		row.AddChild(t.ExtraLeft)
	}
	flex := primitive.NewFlexible(1, bar)
	flex.FillChild = true
	row.AddChild(flex)
	if t.ExtraRight != nil {
		row.AddChild(t.ExtraRight)
	}
	return row
}

func (t *Tabs) hDivider(th *core.Theme) core.Node {
	div := primitive.NewDivider()
	div.ColorToken = core.TokenColorBorder
	_ = th
	return div
}

func (t *Tabs) vDivider(th *core.Theme) core.Node {
	div := primitive.NewDivider()
	div.Vertical = true
	div.Thickness = 1
	div.ColorToken = core.TokenColorBorder
	_ = th
	return div
}

func (t *Tabs) applyInkChrome() {
	if t.ink == nil {
		return
	}
	t.ink.Color = render.RGBA{}
}

func (t *Tabs) paintInk(pc *core.PaintContext) {
	if t == nil || pc == nil || !t.inkVisible() {
		return
	}
	inkW := t.tabInkWidth()
	if inkW <= 0 {
		inkW = DefaultTabsInkThickness
	}
	col := t.inkColor()
	if col.A <= 0 {
		return
	}
	along, span := t.inkAlong, t.inkSpan
	// Apply indicator size/align
	span = t.indicatorSpan(span)
	along = t.indicatorAlong(along, t.inkSpan, span)
	if span < 4 {
		if t.isVertical() {
			span = t.tabItemHeight()
			if span <= 0 {
				span = DefaultTabsCardHeight
			}
		} else {
			span = 48
		}
	}
	if t.isVertical() {
		main := t.inkContentMain
		if t.barList != nil {
			if w := t.barList.Size().Width; w > 1 {
				main = w
			}
		}
		if main < inkW {
			if t.barStack != nil {
				main = t.barStack.Size().Width
			}
		}
		x := main - inkW
		if t.placement() == TabsRight {
			x = 0
		}
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
		if t.placement() == TabsBottom {
			y = 0
		}
		if y < 0 {
			y = 0
		}
		pc.FillLocalRect(along, y, span, inkW, col)
	}
}

func (t *Tabs) indicatorSpan(origin float64) float64 {
	if t.Indicator.SizeFn != nil {
		return t.Indicator.SizeFn(origin)
	}
	if t.Indicator.Size > 0 {
		return t.Indicator.Size
	}
	return origin
}

func (t *Tabs) indicatorAlong(originAlong, originSpan, inkSpan float64) float64 {
	switch t.Indicator.Align {
	case TabsIndicatorStart:
		return originAlong
	case TabsIndicatorEnd:
		return originAlong + (originSpan - inkSpan)
	default:
		return originAlong + (originSpan-inkSpan)/2
	}
}

func (t *Tabs) applyInkGeometry() {
	if t.ink == nil {
		return
	}
	inkW := t.tabInkWidth()
	if inkW <= 0 {
		inkW = DefaultTabsInkThickness
	}
	t.ink.Color = render.RGBA{}
	if !t.inkVisible() {
		t.ink.Width, t.ink.Height = 0, 0
		t.setInkOffset(0, 0)
		return
	}
	span := t.indicatorSpan(t.inkSpan)
	along := t.indicatorAlong(t.inkAlong, t.inkSpan, span)
	if t.isVertical() {
		if span < 4 {
			span = 4
		}
		t.ink.Width = inkW
		t.ink.Height = span
		main := t.inkContentMain
		if t.barList != nil {
			if w := t.barList.Size().Width; w > 0 {
				main = w
			}
		}
		x := main - inkW
		if t.placement() == TabsRight {
			x = 0
		}
		if x < 0 {
			x = 0
		}
		t.setInkOffset(x, along)
	} else {
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
		if t.placement() == TabsBottom {
			y = 0
		}
		if y < 0 {
			y = 0
		}
		t.setInkOffset(along, y)
	}
}

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
		if t.isVertical() {
			t.inkSlots = append(t.inkSlots, inkSlot{key: it.Key, along: off.Y, span: math.Max(sz.Height, 4)})
		} else {
			t.inkSlots = append(t.inkSlots, inkSlot{key: it.Key, along: off.X, span: math.Max(sz.Width, 8)})
		}
	}
	// skip add button host if present
	if bs := t.barList.Size(); bs.Width > 0 || bs.Height > 0 {
		if t.isVertical() && bs.Width > 0 {
			t.inkContentMain = bs.Width
		}
		if !t.isVertical() && bs.Height > 0 {
			t.inkContentMain = bs.Height
		}
	}
	if s := t.slotOf(t.ActiveKey); s != nil {
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
	t.barStack.ClearChildren()
	t.barStack.AddChild(t.barList)
	t.barStack.AddChild(newHost)
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
	keys := t.selectableKeys()
	if t.Nav != nil {
		t.Nav.SetCount(len(keys))
	}

	itemH := t.tabItemHeight()
	inkW := t.tabInkWidth()
	padI, padB := t.padInline(), t.padBlock()
	gutter := t.gutter()
	railW := t.tabWidth()
	fs := t.fontSize()

	gutterScroll := 0.0
	if t.barScroll != nil {
		if b := t.barScroll.Scrollbar(); b != nil {
			gutterScroll = b.GutterThickness()
		}
	} else {
		gutterScroll = 6
	}
	innerW := railW
	if t.isVertical() {
		innerW = railW - gutterScroll
		if innerW < 64 {
			innerW = 64
		}
	}

	t.inkSlots = t.inkSlots[:0]
	along := 0.0
	firstSelectable := true

	for _, it := range t.Items {
		key := it.Key

		if it.Divider {
			line := primitive.NewBox()
			line.Height = 1
			line.Color = th.Color(core.TokenColorBorder)
			host := primitive.NewDecorated(line)
			if t.isVertical() {
				host.Width = innerW
				host.MinWidth = innerW
			}
			host.Height = 9
			host.MinHeight = 9
			host.BorderWidth = 0
			host.Padding = primitive.EdgeInsets{Top: 4, Bottom: 4, Left: 16, Right: 16}
			host.StretchChild = true
			host.Background = render.RGBA{}
			t.barList.AddChild(host)
			along += 9
			continue
		}

		if it.Disabled || !it.Selectable() {
			// Category header (disabled, non-selectable)
			lab := primitive.NewText(it.Label)
			lab.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
			lab.Face = t.Face
			lab.Color = th.Color(core.TokenColorTextSecondary)
			if lab.Color.A <= 0 {
				lab.Color = render.RGBA{R: 0.55, G: 0.55, B: 0.58, A: 1}
			}
			box := primitive.NewDecorated(lab)
			if t.isVertical() {
				box.Width = innerW
				box.MinWidth = innerW
			}
			box.BorderWidth = 0
			box.Background = render.RGBA{}
			box.Padding = primitive.EdgeInsets{Left: 16, Right: 16, Top: 10, Bottom: 4}
			box.StretchChild = true
			h := 28.0
			box.Height = h
			box.MinHeight = h
			t.barList.AddChild(box)
			along += h
			continue
		}

		active := key == t.ActiveKey

		// Label: [icon?] label. Close is a sibling Pressable (Pressable.HitTest does not walk children).
		var labelKids []core.Node
		if it.IconNode != nil {
			labelKids = append(labelKids, it.IconNode)
		} else if it.Icon != "" {
			ic := NewIcon(it.Icon)
			ic.SetSize(fs)
			if active {
				ic.SetColor(th.Color(core.TokenColorPrimary))
			} else {
				ic.SetColor(th.Color(core.TokenColorText))
			}
			labelKids = append(labelKids, ic.Node())
		}
		lab := primitive.NewText(it.Label)
		lab.FontSize = fs
		lab.Face = t.Face
		if active {
			lab.Color = th.Color(core.TokenColorPrimary)
		} else {
			lab.Color = th.Color(core.TokenColorText)
		}
		labelKids = append(labelKids, lab)

		var labelNode core.Node
		if len(labelKids) == 1 {
			labelNode = labelKids[0]
		} else {
			row := primitive.Row(labelKids...)
			row.Gap = DefaultTabsIconGap
			row.CrossAlign = core.CrossCenter
			labelNode = row
		}

		tab := primitive.NewPressable(labelNode)
		tab.Base().Cursor = core.CursorPointer
		tab.EnableRipple = false
		tab.ShowFocusRing = true
		tab.FocusRingOutset = DefaultTabsFocusOutset
		tab.Focusable = true
		tab.Base().Role = "tab"
		tab.Base().Label = it.Label
		leftExtra := 0.0
		if !t.isVertical() && !t.IsCard() {
			if firstSelectable {
				firstSelectable = false
			} else {
				leftExtra = gutter
			}
		}
		if t.isVertical() {
			tab.Padding = primitive.EdgeInsets{Left: 16, Right: 8 + inkW, Top: padB, Bottom: padB}
		} else if t.IsCard() {
			tab.Padding = primitive.EdgeInsets{Left: padI + leftExtra, Right: padI, Top: padB, Bottom: padB}
		} else {
			tab.Padding = primitive.EdgeInsets{Left: leftExtra, Right: 0, Top: padB, Bottom: padB}
		}
		tab.ColorHovered = antItemHoverFill(th)
		if active {
			if t.IsCard() {
				tab.Color = th.Color(core.TokenColorBgContainer)
			} else {
				tab.Color = antItemSelectedFill(th)
			}
		}
		k := key
		tab.Click = func() {
			t.activate(k)
		}
		t.itemPress[key] = tab

		var tabChrome core.Node = tab
		if it.closable(t.Type) {
			closeLab := primitive.NewText("×")
			closeLab.FontSize = fs
			closeLab.Face = t.Face
			closeLab.Color = th.Color(core.TokenColorTextSecondary)
			closePr := primitive.NewPressable(closeLab)
			closePr.Base().Cursor = core.CursorPointer
			closePr.Padding = primitive.All(4)
			closePr.ShowFocusRing = false
			closePr.Focusable = false
			closePr.Base().Role = "button"
			closePr.Base().Label = "Remove tab"
			ck := key
			closePr.Click = func() {
				if t.OnEdit != nil {
					t.OnEdit(ck, "remove")
				}
			}
			t.closePress[key] = closePr
			row := primitive.Row(tab, closePr)
			row.Gap = 4
			row.CrossAlign = core.CrossCenter
			tabChrome = row
		}

		host := primitive.NewDecorated(tabChrome)
		host.BorderWidth = 0
		host.StretchChild = true
		if t.IsCard() {
			host.BorderWidth = 1
			host.BorderColor = th.Color(core.TokenColorBorder)
			host.Radius = th.SizeOr(core.TokenBorderRadius, 6)
			if active {
				host.Background = th.Color(core.TokenColorBgContainer)
				host.BorderColor = th.Color(core.TokenColorBorder)
			} else {
				host.Background = th.Color(core.TokenColorFillSecondary)
				if host.Background.A <= 0 {
					host.Background = render.RGBA{R: 0.97, G: 0.97, B: 0.98, A: 1}
				}
			}
		} else if active {
			host.Background = antItemSelectedFill(th)
		} else {
			host.Background = render.RGBA{}
		}

		span := itemH
		if span <= 0 {
			span = padB*2 + 20
		}
		if t.isVertical() {
			host.Width = innerW
			host.MinWidth = innerW
			if itemH > 0 {
				host.Height = itemH
				host.MinHeight = itemH
			}
		} else {
			minW := 48.0 + leftExtra
			approx := minW + float64(len(it.Label))*fs*0.55
			if it.Icon != "" || it.IconNode != nil {
				approx += fs + DefaultTabsIconGap
			}
			if it.closable(t.Type) {
				approx += fs + DefaultTabsIconGap + 8
			}
			if approx < minW {
				approx = minW
			}
			host.MinWidth = approx
			host.Width = approx
			if itemH > 0 {
				host.Height = itemH
				host.MinHeight = itemH
			}
		}
		t.itemHost[key] = host
		t.barList.AddChild(host)
		t.inkSlots = append(t.inkSlots, inkSlot{key: key, along: along, span: span})
		if t.isVertical() {
			along += span
		} else {
			along += host.Width
		}
	}

	// editable-card add button
	if t.Type == TabsEditableCard && !t.HideAdd {
		addLab := primitive.NewText("+")
		addLab.FontSize = fs + 2
		addLab.Face = t.Face
		addLab.Color = th.Color(core.TokenColorText)
		add := primitive.NewPressable(addLab)
		add.Base().Cursor = core.CursorPointer
		add.Padding = primitive.Symmetric(10, padB)
		add.ShowFocusRing = true
		add.Focusable = true
		add.Base().Role = "button"
		add.Base().Label = "Add tab"
		add.Click = func() {
			if t.OnEdit != nil {
				t.OnEdit("", "add")
			}
		}
		t.addPress = add
		addHost := primitive.NewDecorated(add)
		addHost.BorderWidth = 1
		addHost.BorderColor = th.Color(core.TokenColorBorder)
		addHost.Radius = th.SizeOr(core.TokenBorderRadius, 6)
		if itemH > 0 {
			addHost.Height = itemH
			addHost.MinHeight = itemH
		}
		t.barList.AddChild(addHost)
	}

	if t.isVertical() {
		t.inkContentMain = innerW
	} else {
		h := itemH
		if h <= 0 {
			h = padB*2 + 22
		}
		t.inkContentMain = h
	}

	if t.inkT < 0 {
		if s := t.slotOf(t.ActiveKey); s != nil {
			t.inkAlong, t.inkSpan = s.along, s.span
		}
		t.applyInkGeometry()
	}
}
