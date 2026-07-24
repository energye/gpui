package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Anchor defaults — components/anchor/style/index.ts
// docs/antd/anchor.md §6.2 / §6.10
const (
	DefaultAnchorFontSize               = 14.0
	DefaultAnchorLinkPaddingBlock       = 4.0  // paddingXXS
	DefaultAnchorLinkPaddingInlineStart = 16.0 // padding
	DefaultAnchorHolderOffset           = 4.0  // paddingXXS
	DefaultAnchorPadBlockSecondary      = 2.0  // paddingXXS/2
	DefaultAnchorTitleBlock             = 3.0  // fontSize/14*3
	DefaultAnchorInkWidth               = 2.0  // lineWidthBold
	DefaultAnchorBounds                 = 5.0
	DefaultAnchorBorderRadius           = 6.0
	DefaultAnchorLineWidth              = 1.0
	DefaultAnchorFocusRingOutset        = 1.5
)

// AnchorDirection is antd direction: vertical | horizontal.
type AnchorDirection int

const (
	// AnchorVertical is the default (antd direction="vertical").
	AnchorVertical AnchorDirection = iota
	// AnchorHorizontal lays links in a row; children nesting is ignored.
	AnchorHorizontal
)

// AnchorItem is antd AnchorItem / items[] entry.
type AnchorItem struct {
	Key      string
	Href     string
	Title    string
	Target   string
	Children []AnchorItem
	// Replace overrides Anchor.Replace for this item when true (antd item.replace).
	Replace bool
}

// AnchorLinkInfo is the onClick payload (antd: { title, href } + key).
type AnchorLinkInfo struct {
	Title string
	Href  string
	Key   string
}

// flatLink is a laid-out link slot used for ink / keyboard / scroll-spy.
type flatLink struct {
	item  AnchorItem
	depth int
	node  *primitive.Pressable
}

// Anchor is Ant Design Anchor: in-page section links + scroll-spy + ink.
//
//	[Affix/Sticky?]
//	  └─ wrapper (role=navigation)
//	       └─ Stack
//	            ├─ rail (split line)
//	            ├─ links Flex
//	            └─ ink  (primary indicator)
//
// Product contract: docs/antd/anchor.md §6 (P0 DoD).
// Desktop scroll host: ScrollTarget + SectionOffsets (maps getContainer + #id tops).
// https://ant.design/components/anchor
type Anchor struct {
	// Root is the stable mount node (Affix wrapper when Affix=true, else chrome).
	Root core.Node

	// Product fields
	Items           []AnchorItem
	Direction       AnchorDirection
	Affix           bool
	ShowInkInFixed  bool
	Bounds          float64
	OffsetTop       float64
	TargetOffset    float64
	targetOffsetSet bool
	Replace         bool
	// ActiveLink is the highlighted href (after getCurrentAnchor).
	ActiveLink string
	// ComputedLink is the scroll/click-derived href before getCurrentAnchor (OnChange arg).
	ComputedLink string

	GetCurrentAnchor func(activeLink string) string
	OnChange         func(currentActiveLink string)
	OnClick          func(link AnchorLinkInfo)

	// Scroll host (desktop mapping of getContainer + section tops).
	ScrollTarget   *primitive.ScrollViewport
	SectionOffsets map[string]float64
	// scrollHooked is true when SetScrollTarget installed OnScroll spy.
	scrollHooked bool

	// History is a desktop stand-in for browser history push/replace (P0 replace demo).
	History     []string
	CurrentHref string

	Face      text.Face
	Theme     *core.Theme
	AriaLabel string

	// Internals
	chrome  *anchorChrome
	wrapper *primitive.Decorated
	links   *primitive.Flex
	ink     *primitive.Box
	rail    *primitive.Box
	affix   *Affix
	flat    []flatLink
	inkVis  bool
	Nav     *core.KeyboardNav
}

// anchorChrome is Stack + post-layout ink sync (same pattern as tabsBarHost).
type anchorChrome struct {
	primitive.Stack
	a *Anchor
}

func (h *anchorChrome) TypeID() string { return "kit.AnchorChrome" }

func (h *anchorChrome) Layout(c core.Constraints) core.Size {
	sz := h.Stack.Layout(c)
	if h.a != nil {
		h.a.syncInkFromLayout()
	}
	return sz
}

// NewAnchor creates an Anchor with optional items. Defaults match antd:
// direction=vertical, affix=true, bounds=5, showInkInFixed=false.
func NewAnchor(items ...AnchorItem) *Anchor {
	a := &Anchor{
		Items:  append([]AnchorItem(nil), items...),
		Affix:  true,
		Bounds: DefaultAnchorBounds,
	}
	a.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	a.rebuild()
	return a
}

// Node returns the root core.Node for tree attachment.
func (a *Anchor) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// ChromeNode returns the visual chrome (wrapper), skipping Affix when present.
func (a *Anchor) ChromeNode() core.Node {
	if a == nil {
		return nil
	}
	if a.wrapper != nil {
		return a.wrapper
	}
	return a.Node()
}

// InkVisible reports whether the ink indicator is drawn (tests / ANC-05).
func (a *Anchor) InkVisible() bool {
	if a == nil {
		return false
	}
	return a.inkVis
}

// EffectiveTargetOffset returns targetOffset if set, else offsetTop (antd).
func (a *Anchor) EffectiveTargetOffset() float64 {
	if a == nil {
		return 0
	}
	if a.targetOffsetSet {
		return a.TargetOffset
	}
	return a.OffsetTop
}

// ResolvedInkWidth returns indicator thickness from theme / defaults.
func (a *Anchor) ResolvedInkWidth() float64 {
	th := a.theme()
	// antd lineWidthBold ≈ 2; kit has TokenLineWidth=1 → use max(2*lineWidth, default).
	lw := th.SizeOr(core.TokenLineWidth, DefaultAnchorLineWidth)
	bold := lw * 2
	if bold < DefaultAnchorInkWidth {
		bold = DefaultAnchorInkWidth
	}
	return bold
}

// ResolvedLinkPaddingBlock is §6.2 linkPaddingBlock.
func (a *Anchor) ResolvedLinkPaddingBlock() float64 {
	return a.theme().SizeOr(core.TokenPaddingXS, DefaultAnchorLinkPaddingBlock)
}

// ResolvedLinkPaddingInlineStart is §6.2 linkPaddingInlineStart.
func (a *Anchor) ResolvedLinkPaddingInlineStart() float64 {
	return a.theme().SizeOr(core.TokenPadding, DefaultAnchorLinkPaddingInlineStart)
}

// ResolvedFontSize is §6.2 fontSize.
func (a *Anchor) ResolvedFontSize() float64 {
	return a.theme().SizeOr(core.TokenFontSize, DefaultAnchorFontSize)
}

// SetItems replaces the link tree.
func (a *Anchor) SetItems(items []AnchorItem) {
	if a == nil {
		return
	}
	a.Items = append([]AnchorItem(nil), items...)
	a.rebuild()
}

// SetDirection sets vertical/horizontal layout.
func (a *Anchor) SetDirection(d AnchorDirection) {
	if a == nil {
		return
	}
	a.Direction = d
	if d == AnchorHorizontal {
		a.Nav = core.NewKeyboardNav(core.NavHorizontal, 0)
	} else {
		a.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	}
	a.rebuild()
}

// SetHorizontal is sugar for direction=horizontal when v is true.
func (a *Anchor) SetHorizontal(v bool) {
	if v {
		a.SetDirection(AnchorHorizontal)
	} else {
		a.SetDirection(AnchorVertical)
	}
}

// SetAffix toggles fixed mode (antd affix, default true).
func (a *Anchor) SetAffix(v bool) {
	if a == nil {
		return
	}
	a.Affix = v
	a.rebuild()
}

// SetShowInkInFixed sets showInkInFixed (ink when affix=false).
func (a *Anchor) SetShowInkInFixed(v bool) {
	if a == nil {
		return
	}
	a.ShowInkInFixed = v
	a.rebuild()
}

// SetBounds sets scroll-spy bounds (default 5).
func (a *Anchor) SetBounds(b float64) {
	if a == nil {
		return
	}
	a.Bounds = b
}

// SetOffsetTop sets affix / default target offset.
func (a *Anchor) SetOffsetTop(top float64) {
	if a == nil {
		return
	}
	a.OffsetTop = top
	if a.affix != nil {
		a.affix.SetOffsetTop(top)
	}
}

// SetTargetOffset sets scroll stop offset (overrides offsetTop when set).
func (a *Anchor) SetTargetOffset(off float64) {
	if a == nil {
		return
	}
	a.TargetOffset = off
	a.targetOffsetSet = true
}

// SetReplace sets history replace mode.
func (a *Anchor) SetReplace(v bool) {
	if a == nil {
		return
	}
	a.Replace = v
}

// SetGetCurrentAnchor sets custom highlight resolver.
func (a *Anchor) SetGetCurrentAnchor(fn func(string) string) {
	if a == nil {
		return
	}
	a.GetCurrentAnchor = fn
	// antd re-resolves when getCurrentAnchor identity changes.
	if a.ComputedLink != "" || a.ActiveLink != "" {
		src := a.ComputedLink
		if src == "" {
			src = a.ActiveLink
		}
		a.applyActive(src)
	} else if fn != nil {
		a.applyActive(fn(""))
	}
	a.rebuild()
}

// SetScrollTarget sets the scroll viewport used for spy + click-to-scroll.
// Installs OnScroll so wheel/thumb drag auto-run SyncFromScroll (antd scroll spy).
func (a *Anchor) SetScrollTarget(sv *primitive.ScrollViewport) {
	if a == nil {
		return
	}
	if a.ScrollTarget == sv && a.scrollHooked {
		return
	}
	// Detach previous listener if we owned it.
	if a.ScrollTarget != nil && a.ScrollTarget != sv && a.scrollHooked {
		a.ScrollTarget.OnScroll = nil
		a.scrollHooked = false
	}
	a.ScrollTarget = sv
	if sv == nil {
		a.scrollHooked = false
		return
	}
	prev := sv.OnScroll
	sv.OnScroll = func(x, y float64) {
		if prev != nil {
			prev(x, y)
		}
		a.SyncFromScroll()
	}
	a.scrollHooked = true
}

// SetSectionOffsets maps href → content Y inside ScrollTarget.
func (a *Anchor) SetSectionOffsets(m map[string]float64) {
	if a == nil {
		return
	}
	if m == nil {
		a.SectionOffsets = nil
		return
	}
	cp := make(map[string]float64, len(m))
	for k, v := range m {
		cp[k] = v
	}
	a.SectionOffsets = cp
}

// SetOnChange sets the active-link change callback.
func (a *Anchor) SetOnChange(fn func(string)) {
	if a == nil {
		return
	}
	a.OnChange = fn
}

// SetOnClick sets the link click callback.
func (a *Anchor) SetOnClick(fn func(AnchorLinkInfo)) {
	if a == nil {
		return
	}
	a.OnClick = fn
}

// SetActiveLink forces the highlighted href (display only; does not scroll).
func (a *Anchor) SetActiveLink(href string) {
	if a == nil {
		return
	}
	a.applyActive(href)
	a.rebuild()
}

// SetFace sets the font face for titles.
func (a *Anchor) SetFace(face text.Face) {
	if a == nil {
		return
	}
	a.Face = face
	a.rebuild()
}

// SetTheme sets an explicit theme override.
func (a *Anchor) SetTheme(th *core.Theme) {
	if a == nil {
		return
	}
	a.Theme = th
	a.rebuild()
}

// SetAriaLabel sets the accessible name on the navigation root.
func (a *Anchor) SetAriaLabel(name string) {
	if a == nil {
		return
	}
	a.AriaLabel = name
	if a.wrapper != nil {
		a.wrapper.Base().Label = name
	}
}

func (a *Anchor) theme() *core.Theme {
	var n core.Node
	if a.wrapper != nil {
		n = a.wrapper
	} else if a.Root != nil {
		n = a.Root
	}
	return themeOf(a.Theme, n)
}

// shouldShowInk mirrors antd: hidden when affix=false && !showInkInFixed (class fixed).
func (a *Anchor) shouldShowInk() bool {
	if a == nil {
		return false
	}
	if !a.Affix && !a.ShowInkInFixed {
		return false
	}
	return a.ActiveLink != ""
}

// SyncFromScroll updates ActiveLink from ScrollTarget.ScrollY + SectionOffsets.
// antd: pick max section where top <= targetOffset + bounds.
// Desktop: SectionOffset - ScrollY ≈ top → SectionOffset <= ScrollY + targetOffset + bounds.
func (a *Anchor) SyncFromScroll() {
	if a == nil || a.ScrollTarget == nil || len(a.SectionOffsets) == 0 {
		return
	}
	y := a.ScrollTarget.ScrollY
	limit := y + a.EffectiveTargetOffset() + a.Bounds
	best, bestY := "", -1.0
	hrefs := a.flatHrefs()
	if len(hrefs) == 0 {
		for href := range a.SectionOffsets {
			hrefs = append(hrefs, href)
		}
	}
	for _, href := range hrefs {
		off, ok := a.SectionOffsets[href]
		if !ok {
			continue
		}
		if off <= limit && off >= bestY {
			best, bestY = href, off
		}
	}
	if best == "" {
		return
	}
	prev := a.ActiveLink
	prevComputed := a.ComputedLink
	a.applyActive(best)
	// Rebuild only when highlight changes (scroll ticks are frequent).
	if a.ActiveLink != prev || a.ComputedLink != prevComputed {
		a.rebuild()
	}
}

// ScrollTo scrolls ScrollTarget so href section sits at EffectiveTargetOffset.
func (a *Anchor) ScrollTo(href string) {
	if a == nil || a.ScrollTarget == nil || a.SectionOffsets == nil {
		return
	}
	off, ok := a.SectionOffsets[href]
	if !ok {
		return
	}
	y := off - a.EffectiveTargetOffset()
	if y < 0 {
		y = 0
	}
	a.ScrollTarget.SetScroll(a.ScrollTarget.ScrollX, y)
}

// flatHrefs returns hrefs in document order (DFS). Horizontal ignores children.
func (a *Anchor) flatHrefs() []string {
	var out []string
	var walk func([]AnchorItem)
	walk = func(items []AnchorItem) {
		for _, it := range items {
			if it.Href != "" {
				out = append(out, it.Href)
			}
			if a.Direction == AnchorVertical && len(it.Children) > 0 {
				walk(it.Children)
			}
		}
	}
	walk(a.Items)
	return out
}

// applyActive sets ComputedLink + ActiveLink (via getCurrentAnchor) and fires OnChange
// when the *computed* link changes (antd onChange gets original link).
func (a *Anchor) applyActive(link string) {
	if a.ComputedLink == link {
		// Re-apply getCurrentAnchor for display even if computed unchanged.
		display := link
		if a.GetCurrentAnchor != nil {
			display = a.GetCurrentAnchor(link)
		}
		if a.ActiveLink != display {
			a.ActiveLink = display
		}
		return
	}
	prevComputed := a.ComputedLink
	a.ComputedLink = link
	display := link
	if a.GetCurrentAnchor != nil {
		display = a.GetCurrentAnchor(link)
	}
	a.ActiveLink = display
	if prevComputed != link && a.OnChange != nil {
		a.OnChange(link)
	}
}

func (a *Anchor) recordHistory(href string, itemReplace bool) {
	rep := a.Replace || itemReplace
	if rep {
		if len(a.History) == 0 {
			a.History = []string{href}
		} else {
			a.History[len(a.History)-1] = href
		}
	} else {
		a.History = append(a.History, href)
	}
	a.CurrentHref = href
}

func (a *Anchor) activateLink(it AnchorItem) {
	info := AnchorLinkInfo{Title: it.Title, Href: it.Href, Key: it.Key}
	if a.OnClick != nil {
		a.OnClick(info)
	}
	a.recordHistory(it.Href, it.Replace)
	a.applyActive(it.Href)
	a.ScrollTo(it.Href)
	a.rebuild()
}

func (a *Anchor) rebuild() {
	if a == nil {
		return
	}
	th := a.theme()
	fontSize := a.ResolvedFontSize()
	padBlock := a.ResolvedLinkPaddingBlock()
	padInline := a.ResolvedLinkPaddingInlineStart()
	padSec := padBlock / 2
	if padSec < 1 {
		padSec = DefaultAnchorPadBlockSecondary
	}
	inkW := a.ResolvedInkWidth()
	holder := th.SizeOr(core.TokenPaddingXS, DefaultAnchorHolderOffset)

	// Links flex
	if a.Direction == AnchorHorizontal {
		a.links = primitive.Row()
		a.links.CrossAlign = core.CrossCenter
	} else {
		a.links = primitive.Column()
		a.links.CrossAlign = core.CrossStart
	}
	a.links.Gap = 0
	a.links.Hit = core.HitDefer

	a.flat = a.flat[:0]
	a.buildTree(a.Items, 0, fontSize, padBlock, padInline, padSec, th, a.links)

	if a.Nav != nil {
		a.Nav.SetCount(len(a.flat))
	}

	// Ink box
	if a.ink == nil {
		a.ink = primitive.NewBox()
	}
	a.ink.Hit = core.HitDefer
	a.ink.Color = th.Color(core.TokenColorPrimary)
	a.inkVis = a.shouldShowInk()
	if !a.inkVis {
		a.ink.Width, a.ink.Height = 0, 0
		a.ink.Color = render.RGBA{}
	} else if a.Direction == AnchorHorizontal {
		a.ink.Height = inkW
		a.ink.Width = 24 // provisional; syncInkFromLayout sets real span
	} else {
		a.ink.Width = inkW
		a.ink.Height = fontSize + padBlock*2
	}

	// Rail (split line) — visual only
	if a.rail == nil {
		a.rail = primitive.NewBox()
	}
	a.rail.Hit = core.HitDefer
	split := th.Color(core.TokenColorSplit)
	if a.Direction == AnchorHorizontal {
		a.rail.Width, a.rail.Height = 0, 0
		a.rail.Color = render.RGBA{}
	} else {
		a.rail.Width = inkW
		a.rail.Height = 200 // provisional; stretched in syncInkFromLayout
		a.rail.Color = split
	}

	inkPos := primitive.PositionedAt(0, 0, a.ink)
	railPos := primitive.PositionedAt(0, 0, a.rail)

	if a.chrome == nil {
		a.chrome = &anchorChrome{a: a}
		a.chrome.Fit = true
		a.chrome.Init(a.chrome)
		a.chrome.Hit = core.HitDefer
	} else {
		a.chrome.ClearChildren()
		a.chrome.a = a
	}
	// Paint order: rail, links, ink on top
	a.chrome.AddChild(railPos)
	a.chrome.AddChild(a.links)
	a.chrome.AddChild(inkPos)

	// Wrapper
	if a.wrapper == nil {
		a.wrapper = primitive.NewDecorated(a.chrome)
	} else {
		a.wrapper.ClearChildren()
		a.wrapper.AddChild(a.chrome)
	}
	a.wrapper.Hit = core.HitDefer
	a.wrapper.Base().Role = "navigation"
	if a.AriaLabel != "" {
		a.wrapper.Base().Label = a.AriaLabel
	} else {
		a.wrapper.Base().Label = "Anchor"
	}
	// holderOffsetBlock on top (antd wrapper). Vertical rail/ink sit at stack x=0;
	// link titles already use linkPaddingInlineStart — no extra left gutter on wrapper.
	a.wrapper.Padding = primitive.EdgeInsets{Top: holder}
	a.wrapper.BorderWidth = 0

	// Affix wrap
	if a.Affix {
		if a.affix == nil {
			a.affix = NewAffix(a.wrapper)
		} else if a.affix.sticky != nil {
			a.affix.sticky.ClearChildren()
			a.affix.sticky.AddChild(a.wrapper)
			a.affix.Content = a.wrapper
			a.affix.Root = a.affix.sticky
		} else {
			a.affix = NewAffix(a.wrapper)
		}
		a.affix.SetOffsetTop(a.OffsetTop)
		a.Root = a.affix.Node()
	} else {
		a.affix = nil
		a.Root = a.wrapper
	}

	if a.wrapper != nil {
		a.wrapper.MarkNeedsLayout()
		a.wrapper.MarkNeedsPaint()
	}
}

// buildTree appends link nodes under parent flex and records flat slots.
// Horizontal direction ignores children (antd).
func (a *Anchor) buildTree(items []AnchorItem, depth int, fontSize, padBlock, padInline, padSec float64, th *core.Theme, parent *primitive.Flex) {
	for i, it := range items {
		it := it
		lab := primitive.NewText(it.Title)
		lab.FontSize = fontSize
		lab.Face = a.Face
		active := it.Href != "" && it.Href == a.ActiveLink
		if active {
			lab.Color = th.Color(core.TokenColorPrimary)
		} else {
			lab.Color = th.Color(core.TokenColorText)
		}

		row := primitive.NewPressable(lab)
		row.ShowFocusRing = true
		row.FocusRingOutset = DefaultAnchorFocusRingOutset
		pb := padBlock
		if depth > 0 {
			pb = padSec
		}
		if a.Direction == AnchorHorizontal {
			pl := padInline / 2
			if i == 0 && depth == 0 && parent == a.links {
				pl = 0
			}
			row.Padding = primitive.EdgeInsets{Left: pl, Right: padInline / 2, Top: pb, Bottom: pb}
		} else {
			row.Padding = primitive.EdgeInsets{Left: padInline, Right: 0, Top: pb, Bottom: pb}
		}
		row.Base().Role = "link"
		row.Base().Label = it.Title
		row.ColorHovered = th.Color(core.TokenColorBgTextHover)
		itemCopy := it
		row.Click = func() {
			a.activateLink(itemCopy)
		}

		a.flat = append(a.flat, flatLink{item: it, depth: depth, node: row})

		nest := a.Direction == AnchorVertical && len(it.Children) > 0
		if nest {
			col := primitive.Column()
			col.Hit = core.HitDefer
			col.CrossAlign = core.CrossStart
			col.Gap = 0
			col.AddChild(row)
			a.buildTree(it.Children, depth+1, fontSize, padBlock, padInline, padSec, th, col)
			parent.AddChild(col)
		} else {
			parent.AddChild(row)
		}
	}
}

// syncInkFromLayout positions the ink bar over the active title after layout.
func (a *Anchor) syncInkFromLayout() {
	if a == nil || a.ink == nil || a.chrome == nil {
		return
	}
	a.inkVis = a.shouldShowInk()
	if !a.inkVis {
		a.ink.Width, a.ink.Height = 0, 0
		a.ink.Color = render.RGBA{}
		a.setInkOffset(0, 0)
		return
	}
	th := a.theme()
	a.ink.Color = th.Color(core.TokenColorPrimary)
	inkW := a.ResolvedInkWidth()

	var active *primitive.Pressable
	for _, fl := range a.flat {
		if fl.item.Href == a.ActiveLink && fl.node != nil {
			active = fl.node
			break
		}
	}
	if active == nil {
		a.ink.Width, a.ink.Height = 0, 0
		return
	}

	cAbs := core.AbsoluteBounds(a.chrome)
	tAbs := core.AbsoluteBounds(active)
	relX := tAbs.Min.X - cAbs.Min.X
	relY := tAbs.Min.Y - cAbs.Min.Y
	tw := tAbs.Width()
	thh := tAbs.Height()
	if tw <= 0 || thh <= 0 {
		fontSize := a.ResolvedFontSize()
		padBlock := a.ResolvedLinkPaddingBlock()
		thh = fontSize + padBlock*2
		tw = 48
		relX, relY = 0, 0
		y := 0.0
		for _, fl := range a.flat {
			h := fontSize + padBlock*2
			if fl.depth > 0 {
				h = fontSize + padBlock
			}
			if fl.item.Href == a.ActiveLink {
				relY = y
				thh = h
				break
			}
			y += h
		}
	}

	if a.Direction == AnchorHorizontal {
		a.ink.Width = tw
		a.ink.Height = inkW
		ch := a.chrome.Size().Height
		if ch <= 0 {
			ch = thh
		}
		a.setInkOffset(relX, ch-inkW)
	} else {
		a.ink.Width = inkW
		a.ink.Height = thh
		// Rail/ink sit at the left of the stack (wrapper provides left gutter).
		a.setInkOffset(0, relY)
		if a.rail != nil {
			a.rail.Width = inkW
			ch := a.chrome.Size().Height
			if ch < thh {
				ch = thh
			}
			a.rail.Height = ch
			a.rail.Color = th.Color(core.TokenColorSplit)
		}
	}
	a.ink.MarkNeedsLayout()
	a.ink.MarkNeedsPaint()
}

// setInkOffset updates the PositionedAt host wrapping ink (3rd stack child).
func (a *Anchor) setInkOffset(x, y float64) {
	if a.chrome == nil {
		return
	}
	kids := a.chrome.Children()
	if len(kids) < 3 {
		return
	}
	if so, ok := kids[2].(interface {
		SetStackOffset(x, y float64)
	}); ok {
		so.SetStackOffset(x, y)
		return
	}
	kids[2].Base().SetOffset(core.Point{X: x, Y: y})
}

// FocusIndex returns the keyboard nav index (tests).
func (a *Anchor) FocusIndex() int {
	if a == nil || a.Nav == nil {
		return -1
	}
	return a.Nav.Index
}

// ActivateFocused activates the keyboard-focused link (Enter/Space path for tests).
func (a *Anchor) ActivateFocused() {
	if a == nil || a.Nav == nil {
		return
	}
	i := a.Nav.Index
	if i < 0 || i >= len(a.flat) {
		return
	}
	a.activateLink(a.flat[i].item)
}

// MoveFocus moves keyboard selection by delta.
func (a *Anchor) MoveFocus(delta int) {
	if a == nil || a.Nav == nil || len(a.flat) == 0 {
		return
	}
	a.Nav.SetCount(len(a.flat))
	a.Nav.Move(delta)
	i := a.Nav.Index
	if i >= 0 && i < len(a.flat) && a.flat[i].node != nil {
		a.flat[i].node.State.Focused = true
		a.flat[i].node.State.FocusVisible = true
		a.flat[i].node.MarkNeedsPaint()
	}
}

// FlatCount returns the number of flattened links (including nested).
func (a *Anchor) FlatCount() int {
	if a == nil {
		return 0
	}
	return len(a.flat)
}

// FlatHref returns href at flat index.
func (a *Anchor) FlatHref(i int) string {
	if a == nil || i < 0 || i >= len(a.flat) {
		return ""
	}
	return a.flat[i].item.Href
}

// LinkPressable returns the pressable for href (tests / click simulation).
func (a *Anchor) LinkPressable(href string) *primitive.Pressable {
	if a == nil {
		return nil
	}
	for _, fl := range a.flat {
		if fl.item.Href == href {
			return fl.node
		}
	}
	return nil
}

// IsAffixed reports whether the root is wrapped in Affix (tests / ANC-04).
func (a *Anchor) IsAffixed() bool {
	return a != nil && a.Affix && a.affix != nil
}
