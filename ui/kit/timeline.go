package kit

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Timeline tokens — components/timeline/style/index.ts
// docs/antd/timeline.md §6.2
const (
	// DefaultTimelineDotSize is itemHeadSize (dot diameter).
	DefaultTimelineDotSize = 10.0
	// DefaultTimelineDotBorder is dotBorderWidth ← lineWidthBold.
	DefaultTimelineDotBorder = 2.0
	// DefaultTimelineTailWidth is tailWidth ← lineWidthBold.
	DefaultTimelineTailWidth = 2.0
	// DefaultTimelineItemPadBottom is itemPaddingBottom = padding×1.25.
	DefaultTimelineItemPadBottom = 20.0
	// DefaultTimelineFontSize is content/title fontSize.
	DefaultTimelineFontSize = 14.0
	// DefaultTimelineRadius is borderRadius fallback.
	DefaultTimelineRadius = 6.0
	// DefaultTimelineLineWidth is lineWidth.
	DefaultTimelineLineWidth = 1.0
	// DefaultTimelineTitleSpan is titleSpan (12/24 → 50% slot).
	DefaultTimelineTitleSpan = 12.0
	// DefaultTimelineCustomIconPad is customHeadPaddingVertical (paddingXXS).
	DefaultTimelineCustomIconPad = 4.0
	// DefaultTimelineFocusOutset approximates focus-visible outset (N/A for non-focus list).
	DefaultTimelineFocusOutset = 1.5
	// DefaultTimelineIconSize is the custom icon box edge.
	DefaultTimelineIconSize = 16.0
	// timelineSpinRPS is loading ring revolutions per second.
	timelineSpinRPS = 0.9
	// timelineContentGap is gap between rail column and body.
	timelineContentGap = 12.0
	// timelineMinTail is minimum rail length between dots.
	timelineMinTail = 12.0
)

// TimelineMode is antd mode (start | alternate | end).
type TimelineMode int

const (
	// TimelineModeStart places content on the logical end side (LTR right). Default.
	TimelineModeStart TimelineMode = iota
	// TimelineModeAlternate alternates placement per index.
	TimelineModeAlternate
	// TimelineModeEnd places content on the logical start side (LTR left).
	TimelineModeEnd
)

// TimelineOrientation is antd orientation.
type TimelineOrientation int

const (
	// TimelineVertical stacks items top→bottom. Default.
	TimelineVertical TimelineOrientation = iota
	// TimelineHorizontal lays items left→right.
	TimelineHorizontal
)

// TimelineVariant is antd variant (outlined | filled).
type TimelineVariant int

const (
	// TimelineOutlined is hollow stroke dots. Default.
	TimelineOutlined TimelineVariant = iota
	// TimelineFilled is solid filled dots.
	TimelineFilled
)

// TimelinePlacement is item placement (start | end); 0 = auto from mode.
type TimelinePlacement int

const (
	// TimelinePlacementAuto derives side from mode / index.
	TimelinePlacementAuto TimelinePlacement = iota
	// TimelinePlacementStart is logical start side.
	TimelinePlacementStart
	// TimelinePlacementEnd is logical end side.
	TimelinePlacementEnd
)

// TimelineItem is one antd Timeline items[] entry (P0 fields).
type TimelineItem struct {
	// Content is the main body text (antd content). Prefer over ContentNode when both set for tests.
	Content string
	// ContentNode overrides Content when non-nil.
	ContentNode core.Node
	// Title is the side label / time (antd title; replaces deprecated label).
	Title string
	// TitleNode overrides Title when non-nil.
	TitleNode core.Node
	// Color is blue|red|green|gray or #hex; empty → blue.
	Color string
	// Icon is a registry icon name; "loading" forces spinner.
	Icon string
	// IconNode overrides Icon when non-nil (custom dot).
	IconNode core.Node
	// Loading marks process status + default LoadingOutlined spinner.
	Loading bool
	// Placement overrides mode-derived side when not Auto.
	Placement TimelinePlacement
}

// TimelineClassNames holds shallow semantic class tags (antd classNames; P1 depth).
type TimelineClassNames struct {
	Root string
	Item string
}

// Timeline is Ant Design Timeline — vertical/horizontal time-flow list.
//
//	timelineHost (RepaintBoundary; OnMount binds Ticker when any item.loading)
//	  └─ Flex root (Column|Row)  role=list
//	       item × N  role=listitem
//	         railCol (dot|icon + tail) · title? · content
//
// Product contract: docs/antd/timeline.md §6 (P0 DoD).
// Root identity is stable across rebuild when axis unchanged (ClearChildren).
type Timeline struct {
	Root *timelineHost
	list *primitive.Flex

	Items []TimelineItem

	mode        TimelineMode
	orientation TimelineOrientation
	variant     TimelineVariant
	reverse     bool
	titleSpan   float64 // 0 → DefaultTimelineTitleSpan

	Style      Style
	ClassNames TimelineClassNames
	AriaLabel  string
	Face       text.Face
	Theme      *core.Theme

	// chrome / test hooks
	itemNodes  []core.Node
	dotBoxes   []*primitive.Decorated
	placements []TimelinePlacement
	dotColors  []render.RGBA

	// metrics cache (L2)
	dotSize    float64
	dotBorder  float64
	tailWidth  float64
	itemPadBot float64
	fontSize   float64
	titleSpanV float64

	// loading spinner
	spinPhase   float64
	needSpinner bool
	life        tickerLifecycle
}

type timelineHost struct {
	primitive.RepaintBoundary
	tl *Timeline
}

func (h *timelineHost) TypeID() string { return "kit.Timeline" }

func (h *timelineHost) OnMount() {
	if h == nil || h.tl == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.tl.life.attach(t, h.tl, h.tl.needSpinner)
	}
}

func (h *timelineHost) OnUnmount() {
	if h != nil && h.tl != nil {
		h.tl.life.unmount()
	}
}

// NewTimeline creates a Timeline with optional items (antd defaults).
func NewTimeline(items ...TimelineItem) *Timeline {
	tl := &Timeline{
		Items:       append([]TimelineItem(nil), items...),
		mode:        TimelineModeStart,
		orientation: TimelineVertical,
		variant:     TimelineOutlined,
	}
	tl.rebuild()
	return tl
}

// Node returns the mount root.

// ensureBuilt materializes the control tree if missing (#9).
func (tl *Timeline) ensureBuilt() {
	if tl == nil {
		return
	}
	if tl.Root == nil {
		tl.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (tl *Timeline) structureChange() {
	if tl == nil {
		return
	}
	tl.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (tl *Timeline) chromeChange() {
	if tl == nil {
		return
	}
	tl.ensureBuilt()
	tl.rebuild()
}

func (tl *Timeline) Node() core.Node {
	if tl == nil {
		return nil
	}
	tl.ensureBuilt()
	return tl.Root
}

// ChromeNode returns the visual shell (same as Node).
func (tl *Timeline) ChromeNode() core.Node { return tl.Node() }

// --- setters ---

// SetItems replaces items and rebuilds.
func (tl *Timeline) SetItems(items []TimelineItem) {
	if tl == nil {
		return
	}
	tl.Items = append([]TimelineItem(nil), items...)
	tl.rebuild()
}

// SetMode sets start|alternate|end.
func (tl *Timeline) SetMode(m TimelineMode) {
	if tl == nil {
		return
	}
	tl.mode = m
	tl.rebuild()
}

// SetOrientation sets vertical|horizontal.
func (tl *Timeline) SetOrientation(o TimelineOrientation) {
	if tl == nil {
		return
	}
	tl.orientation = o
	tl.rebuild()
}

// SetVariant sets outlined|filled.
func (tl *Timeline) SetVariant(v TimelineVariant) {
	if tl == nil {
		return
	}
	tl.variant = v
	tl.rebuild()
}

// SetReverse toggles display order.
func (tl *Timeline) SetReverse(on bool) {
	if tl == nil {
		return
	}
	tl.reverse = on
	tl.rebuild()
}

// SetTitleSpan sets title slot weight (antd titleSpan; default 12).
func (tl *Timeline) SetTitleSpan(span float64) {
	if tl == nil {
		return
	}
	if span < 0 {
		span = 0
	}
	tl.titleSpan = span
	tl.rebuild()
}

// SetTheme sets an explicit theme override.
func (tl *Timeline) SetTheme(th *core.Theme) {
	if tl == nil {
		return
	}
	tl.Theme = th
	tl.rebuild()
}

// SetFace sets the text face.
func (tl *Timeline) SetFace(face text.Face) {
	if tl == nil {
		return
	}
	tl.Face = face
	tl.rebuild()
}

// SetStyle sets optional root style overrides.
func (tl *Timeline) SetStyle(st Style) {
	if tl == nil {
		return
	}
	tl.Style = st
	tl.rebuild()
}

// SetAriaLabel sets the list accessible name.
func (tl *Timeline) SetAriaLabel(s string) {
	if tl == nil {
		return
	}
	tl.AriaLabel = s
	if tl.list != nil {
		if s != "" {
			tl.list.Base().Label = s
		} else {
			tl.list.Base().Label = "timeline"
		}
	}
}

// --- getters ---

// Mode returns the current mode.
func (tl *Timeline) Mode() TimelineMode {
	if tl == nil {
		return TimelineModeStart
	}
	return tl.mode
}

// Orientation returns the current orientation.
func (tl *Timeline) Orientation() TimelineOrientation {
	if tl == nil {
		return TimelineVertical
	}
	return tl.orientation
}

// Variant returns the current variant.
func (tl *Timeline) Variant() TimelineVariant {
	if tl == nil {
		return TimelineOutlined
	}
	return tl.variant
}

// Reverse reports reverse flag.
func (tl *Timeline) Reverse() bool {
	return tl != nil && tl.reverse
}

// TitleSpan returns the effective titleSpan.
func (tl *Timeline) TitleSpan() float64 {
	if tl == nil {
		return DefaultTimelineTitleSpan
	}
	if tl.titleSpan > 0 {
		return tl.titleSpan
	}
	return DefaultTimelineTitleSpan
}

// ItemCount returns len(Items).
func (tl *Timeline) ItemCount() int {
	if tl == nil {
		return 0
	}
	return len(tl.Items)
}

// DisplayIndex maps display slot → origin index (accounts for reverse).
func (tl *Timeline) DisplayIndex(display int) int {
	n := tl.ItemCount()
	if display < 0 || display >= n {
		return -1
	}
	if tl.reverse {
		return n - 1 - display
	}
	return display
}

// OriginDisplayIndex maps origin → display slot.
func (tl *Timeline) OriginDisplayIndex(origin int) int {
	n := tl.ItemCount()
	if origin < 0 || origin >= n {
		return -1
	}
	if tl.reverse {
		return n - 1 - origin
	}
	return origin
}

// ItemPlacement returns resolved placement for origin index.
func (tl *Timeline) ItemPlacement(origin int) TimelinePlacement {
	if tl == nil || origin < 0 || origin >= len(tl.Items) {
		return TimelinePlacementStart
	}
	if len(tl.placements) == len(tl.Items) {
		// placements stored by display order — convert
		di := tl.OriginDisplayIndex(origin)
		if di >= 0 && di < len(tl.placements) {
			return tl.placements[di]
		}
	}
	return tl.resolvePlacement(origin, tl.Items[origin])
}

// ItemColor returns resolved dot color for origin index.
func (tl *Timeline) ItemColor(origin int) render.RGBA {
	if tl == nil || origin < 0 || origin >= len(tl.Items) {
		return render.RGBA{}
	}
	di := tl.OriginDisplayIndex(origin)
	if di >= 0 && di < len(tl.dotColors) {
		return tl.dotColors[di]
	}
	return tl.resolveDotColor(tl.Items[origin], tl.theme())
}

// ItemLoading reports items[origin].Loading.
func (tl *Timeline) ItemLoading(origin int) bool {
	if tl == nil || origin < 0 || origin >= len(tl.Items) {
		return false
	}
	return tl.Items[origin].Loading
}

// ItemHasIcon reports custom icon (name or node) or loading spinner.
func (tl *Timeline) ItemHasIcon(origin int) bool {
	if tl == nil || origin < 0 || origin >= len(tl.Items) {
		return false
	}
	it := tl.Items[origin]
	return it.IconNode != nil || it.Icon != "" || it.Loading
}

// ItemNode returns the listitem node for origin index (nil if missing).
func (tl *Timeline) ItemNode(origin int) core.Node {
	if tl == nil {
		return nil
	}
	di := tl.OriginDisplayIndex(origin)
	if di < 0 || di >= len(tl.itemNodes) {
		return nil
	}
	return tl.itemNodes[di]
}

// DotDecorated returns the default circle chrome for origin (nil when pure custom icon without box).
func (tl *Timeline) DotDecorated(origin int) *primitive.Decorated {
	if tl == nil {
		return nil
	}
	di := tl.OriginDisplayIndex(origin)
	if di < 0 || di >= len(tl.dotBoxes) {
		return nil
	}
	return tl.dotBoxes[di]
}

// DotSize returns resolved dot diameter.
func (tl *Timeline) DotSize() float64 {
	if tl == nil || tl.dotSize <= 0 {
		return DefaultTimelineDotSize
	}
	return tl.dotSize
}

// TailWidth returns resolved tail line width.
func (tl *Timeline) TailWidth() float64 {
	if tl == nil || tl.tailWidth <= 0 {
		return DefaultTimelineTailWidth
	}
	return tl.tailWidth
}

// ItemPaddingBottom returns resolved item bottom padding.
func (tl *Timeline) ItemPaddingBottom() float64 {
	if tl == nil || tl.itemPadBot <= 0 {
		return DefaultTimelineItemPadBottom
	}
	return tl.itemPadBot
}

// FontSize returns resolved content font size.
func (tl *Timeline) FontSize() float64 {
	if tl == nil || tl.fontSize <= 0 {
		return DefaultTimelineFontSize
	}
	return tl.fontSize
}

// DotBorderWidth returns resolved dot stroke width.
func (tl *Timeline) DotBorderWidth() float64 {
	if tl == nil || tl.dotBorder <= 0 {
		return DefaultTimelineDotBorder
	}
	return tl.dotBorder
}

// HasLoadingSpinner reports whether any item needs a spinner.
func (tl *Timeline) HasLoadingSpinner() bool {
	return tl != nil && tl.needSpinner
}

// HasPending is true when the last origin item is loading (pending semantics).
func (tl *Timeline) HasPending() bool {
	n := tl.ItemCount()
	if n == 0 {
		return false
	}
	return tl.Items[n-1].Loading
}

// LayoutAlternate reports whether bilateral title/content slots are active.
func (tl *Timeline) LayoutAlternate() bool {
	if tl == nil {
		return false
	}
	if tl.mode == TimelineModeAlternate {
		return true
	}
	if tl.orientation == TimelineVertical {
		for _, it := range tl.Items {
			if it.Title != "" || it.TitleNode != nil {
				return true
			}
		}
	}
	return false
}

// AttachTicker registers loading spinner animation.
func (tl *Timeline) AttachTicker(t *core.Tree) {
	if tl == nil || t == nil {
		return
	}
	tl.life.attach(t, tl, tl.needSpinner)
}

// Tick advances loading spinner phase.
func (tl *Timeline) Tick(dt float64) bool {
	if tl == nil || !tl.needSpinner {
		return false
	}
	var nt *core.Tree
	if tl.Root != nil {
		nt = tl.Root.Tree()
	}
	if !tl.life.stillMounted(nt) {
		return false
	}
	tl.spinPhase += dt * timelineSpinRPS
	if tl.spinPhase > 1 {
		tl.spinPhase -= math.Floor(tl.spinPhase)
	}
	if tl.Root != nil {
		tl.Root.MarkNeedsPaint()
	}
	return true
}

// --- internals ---

func (tl *Timeline) theme() *core.Theme {
	var n core.Node
	if tl.Root != nil {
		n = tl.Root
	}
	return themeOf(tl.Theme, n)
}

func (tl *Timeline) resolveMetrics() {
	th := tl.theme()
	tl.fontSize = th.SizeOr(core.TokenFontSize, DefaultTimelineFontSize)
	if tl.Style.FontSize > 0 {
		tl.fontSize = tl.Style.FontSize
	}
	tl.dotSize = DefaultTimelineDotSize
	// lineWidthBold ≈ max(lineWidth*2, 2)
	lw := th.SizeOr(core.TokenLineWidth, DefaultTimelineLineWidth)
	bold := lw * 2
	if bold < DefaultTimelineDotBorder {
		bold = DefaultTimelineDotBorder
	}
	tl.dotBorder = bold
	tl.tailWidth = bold
	pad := th.SizeOr(core.TokenPadding, 16)
	tl.itemPadBot = pad * 1.25
	if tl.itemPadBot <= 0 {
		tl.itemPadBot = DefaultTimelineItemPadBottom
	}
	tl.titleSpanV = tl.TitleSpan()
}

func (tl *Timeline) resolvePlacement(origin int, it TimelineItem) TimelinePlacement {
	if it.Placement == TimelinePlacementStart || it.Placement == TimelinePlacementEnd {
		return it.Placement
	}
	switch tl.mode {
	case TimelineModeAlternate:
		if origin%2 == 0 {
			return TimelinePlacementStart
		}
		return TimelinePlacementEnd
	case TimelineModeEnd:
		return TimelinePlacementEnd
	default:
		return TimelinePlacementStart
	}
}

func (tl *Timeline) resolveDotColor(it TimelineItem, th *core.Theme) render.RGBA {
	c := strings.TrimSpace(strings.ToLower(it.Color))
	switch c {
	case "", "blue":
		return th.Color(core.TokenColorPrimary)
	case "red":
		return th.Color(core.TokenColorError)
	case "green":
		return th.Color(core.TokenColorSuccess)
	case "gray", "grey":
		return th.Color(core.TokenColorDisabledText)
	default:
		if strings.HasPrefix(c, "#") || looksLikeHexColor(c) {
			col := render.Hex(it.Color)
			if col.A > 0 {
				return col
			}
		}
		// unknown name → primary
		return th.Color(core.TokenColorPrimary)
	}
}

func looksLikeHexColor(s string) bool {
	if len(s) != 3 && len(s) != 6 && len(s) != 8 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func (tl *Timeline) anyLoading() bool {
	for _, it := range tl.Items {
		if it.Loading || strings.EqualFold(it.Icon, "loading") {
			return true
		}
	}
	return false
}

func (tl *Timeline) rebuild() {
	if tl == nil {
		return
	}
	tl.resolveMetrics()
	th := tl.theme()
	vertical := tl.orientation != TimelineHorizontal
	tl.needSpinner = tl.anyLoading()

	// Host
	if tl.Root == nil {
		tl.Root = &timelineHost{tl: tl}
		tl.Root.Init(tl.Root)
		tl.Root.Hit = core.HitDefer
	} else {
		tl.Root.tl = tl
		tl.Root.ClearChildren()
	}

	// List flex
	wantAxis := core.AxisVertical
	if !vertical {
		wantAxis = core.AxisHorizontal
	}
	if tl.list == nil || tl.list.Axis != wantAxis {
		tl.list = primitive.NewFlex(wantAxis)
	} else {
		tl.list.ClearChildren()
	}
	tl.list.Gap = 0
	tl.list.CrossAlign = core.CrossStretch
	if !vertical {
		tl.list.CrossAlign = core.CrossStart
		tl.list.MainAlign = core.MainStart
	}
	tl.list.Base().Role = "list"
	if tl.AriaLabel != "" {
		tl.list.Base().Label = tl.AriaLabel
	} else {
		tl.list.Base().Label = "timeline"
	}
	tl.list.SetThemeHook(func(*core.Theme) { tl.rebuild() })

	if tl.Style.hasBG() {
		// wrap later if needed; keep list transparent for hit==layout
	}

	n := len(tl.Items)
	tl.itemNodes = make([]core.Node, 0, n)
	tl.dotBoxes = make([]*primitive.Decorated, 0, n)
	tl.placements = make([]TimelinePlacement, 0, n)
	tl.dotColors = make([]render.RGBA, 0, n)

	// display order
	order := make([]int, n)
	for i := 0; i < n; i++ {
		if tl.reverse {
			order[i] = n - 1 - i
		} else {
			order[i] = i
		}
	}

	alternate := tl.LayoutAlternate()

	for di, origin := range order {
		it := tl.Items[origin]
		// placement uses origin index (antd useItems maps before reverse; reverse only reorders)
		// After reverse, alternate parity follows display index in antd? Looking at useItems:
		// placement is computed on parseItems before reverse, then reverse reorders.
		// So parity is based on original index. We use origin.
		place := tl.resolvePlacement(origin, it)
		// When reverse + alternate, antd still uses original index parity from pre-reverse map.
		// Our resolvePlacement(origin) matches that.
		col := tl.resolveDotColor(it, th)
		last := di == n-1
		node, dot := tl.buildItem(it, origin, place, col, last, vertical, alternate, th)
		tl.itemNodes = append(tl.itemNodes, node)
		tl.dotBoxes = append(tl.dotBoxes, dot)
		tl.placements = append(tl.placements, place)
		tl.dotColors = append(tl.dotColors, col)
		if vertical {
			tl.list.AddChild(node)
		} else {
			// equal flex share for horizontal items
			flex := primitive.NewFlexible(1, node)
			flex.FillChild = true
			tl.list.AddChild(flex)
		}
	}

	tl.Root.AddChild(tl.list)
	tl.life.setActive(tl.needSpinner)
	tl.Root.MarkNeedsLayout()
	tl.Root.MarkNeedsPaint()
}

func (tl *Timeline) buildItem(
	it TimelineItem,
	origin int,
	place TimelinePlacement,
	col render.RGBA,
	last, vertical, alternate bool,
	th *core.Theme,
) (core.Node, *primitive.Decorated) {
	dot, dec := tl.buildDot(it, col, th)
	tail := tl.buildTail(last, vertical, it.Loading, th)

	var body core.Node
	if vertical {
		body = tl.buildVerticalItem(it, place, dot, tail, last, alternate, th)
	} else {
		body = tl.buildHorizontalItem(it, place, dot, tail, last, th)
	}

	// listitem wrapper — hit == layout == paint
	wrap := primitive.NewDecorated(body)
	wrap.BorderWidth = 0
	wrap.Hit = core.HitDefer
	wrap.Base().Role = "listitem"
	label := it.Content
	if label == "" && it.Title != "" {
		label = it.Title
	}
	if label == "" {
		label = "timeline item"
	}
	wrap.Base().Label = label
	if tl.ClassNames.Item != "" {
		wrap.Base().Label = tl.ClassNames.Item + " " + wrap.Base().Label
	}
	_ = origin
	return wrap, dec
}

func (tl *Timeline) buildDot(it TimelineItem, col render.RGBA, th *core.Theme) (core.Node, *primitive.Decorated) {
	custom := it.IconNode != nil || (it.Icon != "" && !strings.EqualFold(it.Icon, "loading"))
	loading := it.Loading || strings.EqualFold(it.Icon, "loading")

	if it.IconNode != nil {
		// Custom node: light container for stable hit box
		sz := DefaultTimelineIconSize
		dec := primitive.NewDecorated(it.IconNode)
		dec.Width, dec.Height = sz, sz
		dec.Radius = sz / 2
		dec.SetCenterContent(true)
		dec.Background = th.Color(core.TokenColorBgContainer)
		dec.BorderWidth = 0
		dec.Hit = core.HitDefer
		return dec, dec
	}

	if loading {
		tl.needSpinner = true
		sz := DefaultTimelineIconSize
		spin := primitive.NewCanvas(sz, sz, tl.paintSpinner)
		dec := primitive.NewDecorated(spin)
		dec.Width, dec.Height = sz, sz
		dec.Radius = sz / 2
		dec.SetCenterContent(true)
		dec.Background = th.Color(core.TokenColorBgContainer)
		dec.BorderWidth = 0
		dec.Hit = core.HitDefer
		return dec, dec
	}

	if custom {
		name := it.Icon
		ic := primitive.NewIcon(name)
		ic.Size = DefaultTimelineIconSize
		ic.Color = col
		sz := DefaultTimelineIconSize + DefaultTimelineCustomIconPad
		dec := primitive.NewDecorated(ic)
		dec.Width, dec.Height = sz, sz
		dec.Radius = sz / 2
		dec.SetCenterContent(true)
		dec.Background = th.Color(core.TokenColorBgContainer)
		dec.BorderWidth = 0
		dec.Hit = core.HitDefer
		return dec, dec
	}

	// Default circle
	sz := tl.dotSize
	if sz <= 0 {
		sz = DefaultTimelineDotSize
	}
	dec := primitive.NewDecorated()
	dec.Width, dec.Height = sz, sz
	dec.Radius = sz / 2
	dec.Hit = core.HitDefer
	switch tl.variant {
	case TimelineFilled:
		dec.Background = col
		dec.BorderWidth = 0
	default: // outlined
		dec.Background = th.Color(core.TokenColorBgContainer)
		if dec.Background.A == 0 {
			dec.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		dec.BorderWidth = tl.dotBorder
		if dec.BorderWidth <= 0 {
			dec.BorderWidth = DefaultTimelineDotBorder
		}
		dec.BorderColor = col
	}
	return dec, dec
}

func (tl *Timeline) buildTail(last, vertical, loading bool, th *core.Theme) core.Node {
	if last {
		return nil
	}
	tw := tl.tailWidth
	if tw <= 0 {
		tw = DefaultTimelineTailWidth
	}
	tailCol := th.Color(core.TokenColorSplit)
	if tailCol.A == 0 {
		tailCol = th.Color(core.TokenColorBorder)
	}
	box := primitive.NewBox()
	box.Color = tailCol
	if vertical {
		box.Width = tw
		// height grows via flexible parent; min length
		box.Height = timelineMinTail + tl.itemPadBot
		if loading {
			// process rail: lighter / dashed approximation (solid with lower alpha)
			box.Color = render.RGBA{R: tailCol.R, G: tailCol.G, B: tailCol.B, A: tailCol.A * 0.55}
		}
	} else {
		box.Height = tw
		box.Width = timelineMinTail + 8
		if loading {
			box.Color = render.RGBA{R: tailCol.R, G: tailCol.G, B: tailCol.B, A: tailCol.A * 0.55}
		}
	}
	return box
}

func (tl *Timeline) buildContent(it TimelineItem, th *core.Theme, alignEnd bool) core.Node {
	col := primitive.Column()
	col.Gap = 2
	col.CrossAlign = core.CrossStart
	if alignEnd {
		col.CrossAlign = core.CrossEnd
	}
	has := false
	if it.ContentNode != nil {
		col.AddChild(it.ContentNode)
		has = true
	} else if it.Content != "" {
		tx := primitive.NewText(it.Content)
		tx.FontSize = tl.fontSize
		tx.Face = tl.Face
		tx.Color = th.Color(core.TokenColorText)
		col.AddChild(tx)
		has = true
	}
	if !has {
		// empty content placeholder for layout stability
		sp := primitive.NewBox()
		sp.Width, sp.Height = 1, tl.fontSize
		sp.Color = render.RGBA{}
		col.AddChild(sp)
	}
	return col
}

func (tl *Timeline) buildTitle(it TimelineItem, th *core.Theme, alignEnd bool) core.Node {
	if it.TitleNode == nil && it.Title == "" {
		return nil
	}
	col := primitive.Column()
	col.Gap = 0
	col.CrossAlign = core.CrossStart
	if alignEnd {
		col.CrossAlign = core.CrossEnd
	}
	if it.TitleNode != nil {
		col.AddChild(it.TitleNode)
		return col
	}
	tx := primitive.NewText(it.Title)
	tx.FontSize = tl.fontSize
	tx.Face = tl.Face
	tx.Color = th.Color(core.TokenColorTextSecondary)
	col.AddChild(tx)
	return col
}

func (tl *Timeline) railColumn(dot, tail core.Node, vertical bool) core.Node {
	if vertical {
		col := primitive.Column(dot)
		col.Gap = 0
		col.CrossAlign = core.CrossCenter
		col.MainAlign = core.MainStart
		if tail != nil {
			// grow tail under dot
			flex := primitive.NewFlexible(1, tail)
			flex.FillChild = true
			col.AddChild(flex)
		}
		return col
	}
	// horizontal: icon only; tails are siblings
	return dot
}

func (tl *Timeline) buildVerticalItem(
	it TimelineItem,
	place TimelinePlacement,
	dot, tail core.Node,
	last, alternate bool,
	th *core.Theme,
) core.Node {
	rail := tl.railColumn(dot, tail, true)
	padBot := tl.itemPadBot
	if last {
		padBot = 0
	}

	// Same-side layout (no bilateral title slots)
	if !alternate {
		content := tl.buildContent(it, th, place == TimelinePlacementEnd)
		// title stacks above content on same side when present
		if t := tl.buildTitle(it, th, place == TimelinePlacementEnd); t != nil {
			stack := primitive.Column(t, content)
			stack.Gap = 2
			stack.CrossAlign = core.CrossStart
			if place == TimelinePlacementEnd {
				stack.CrossAlign = core.CrossEnd
			}
			content = stack
		}
		var row *primitive.Flex
		if place == TimelinePlacementEnd {
			// content | rail  (content left, dots right)
			row = primitive.Row()
			row.Gap = timelineContentGap
			row.CrossAlign = core.CrossStart
			row.MainAlign = core.MainEnd
			flex := primitive.NewFlexible(1, content)
			row.AddChild(flex)
			row.AddChild(rail)
		} else {
			// rail | content
			row = primitive.Row(rail, content)
			row.Gap = timelineContentGap
			row.CrossAlign = core.CrossStart
		}
		if padBot > 0 {
			dec := primitive.NewDecorated(row)
			dec.Padding = primitive.EdgeInsets{Bottom: padBot}
			dec.BorderWidth = 0
			dec.Hit = core.HitDefer
			return dec
		}
		return row
	}

	// Alternate / title bilateral slots
	// placement start: title left (end-align) | rail | content right
	// placement end:   content left (end-align) | rail | title right
	titleAlignEnd := place == TimelinePlacementStart
	contentAlignEnd := place == TimelinePlacementEnd
	title := tl.buildTitle(it, th, titleAlignEnd)
	content := tl.buildContent(it, th, contentAlignEnd)
	if title == nil {
		// empty title slot still takes space in alternate mode for balance
		sp := primitive.NewBox()
		sp.Width, sp.Height = 1, 1
		sp.Color = render.RGBA{}
		title = sp
	}

	// Weight: titleSpan/24 vs remainder
	span := tl.titleSpanV
	if span <= 0 {
		span = DefaultTimelineTitleSpan
	}
	titleGrow := span
	contentGrow := 24 - span
	if contentGrow < 1 {
		contentGrow = 1
	}

	row := primitive.Row()
	row.Gap = timelineContentGap
	row.CrossAlign = core.CrossStart

	if place == TimelinePlacementEnd {
		// content | rail | title
		cf := primitive.NewFlexible(contentGrow, content)
		tf := primitive.NewFlexible(titleGrow, title)
		row.AddChild(cf)
		row.AddChild(rail)
		row.AddChild(tf)
	} else {
		// title | rail | content
		tf := primitive.NewFlexible(titleGrow, title)
		cf := primitive.NewFlexible(contentGrow, content)
		row.AddChild(tf)
		row.AddChild(rail)
		row.AddChild(cf)
	}

	if padBot > 0 {
		dec := primitive.NewDecorated(row)
		dec.Padding = primitive.EdgeInsets{Bottom: padBot}
		dec.BorderWidth = 0
		dec.Hit = core.HitDefer
		return dec
	}
	return row
}

func (tl *Timeline) buildHorizontalItem(
	it TimelineItem,
	place TimelinePlacement,
	dot, tail core.Node,
	last bool,
	th *core.Theme,
) core.Node {
	// Content above (end) or below (start) the axis
	content := tl.buildContent(it, th, false)
	title := tl.buildTitle(it, th, false)

	// Axis row: [optional leading tail share is on previous item] icon [tail to next]
	axis := primitive.Row()
	axis.Gap = 0
	axis.CrossAlign = core.CrossCenter
	axis.MainAlign = core.MainStart
	// leading flexible spacer so icon can sit mid-slot when stretched
	axis.AddChild(primitive.NewFlexible(1, nil))
	axis.AddChild(dot)
	if tail != nil {
		tf := primitive.NewFlexible(1, tail)
		tf.FillChild = true
		axis.AddChild(tf)
	} else {
		axis.AddChild(primitive.NewFlexible(1, nil))
	}

	col := primitive.Column()
	col.Gap = 8
	col.CrossAlign = core.CrossCenter
	col.MainAlign = core.MainStart

	above := place == TimelinePlacementEnd
	// mode start → content below; mode end → content above; alternate flips per place
	if above {
		if title != nil {
			col.AddChild(title)
		}
		col.AddChild(content)
		col.AddChild(axis)
	} else {
		col.AddChild(axis)
		if title != nil {
			col.AddChild(title)
		}
		col.AddChild(content)
	}
	_ = last
	return col
}

func (tl *Timeline) paintSpinner(pc *core.PaintContext, sz core.Size) {
	if pc == nil {
		return
	}
	th := tl.theme()
	col := th.Color(core.TokenColorPrimary)
	track := render.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * 0.35}
	stroke := 2.0
	if sz.Width < 14 {
		stroke = 1.5
	}
	cx, cy := sz.Width/2, sz.Height/2
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + tl.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	steps := 40
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		a := start + (end-start)*t
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	if len(pts) >= 4 {
		pc.StrokeLocalPolyline(pts, stroke, col)
	}
}
