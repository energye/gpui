// Package timeline implements the Timeline control (docs/antd/timeline.md §6).
//
// P0 scope (§6.8): items/content/title/color/icon/loading/placement +
// mode/orientation/variant/reverse + §6.2 metrics + list/listitem roles.
// P1 (titleSpan fine percent, semantic classNames/styles, pixel motion,
// debug samples) is out of scope here.
package timeline

import (
	"math"
	"strings"
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Metric fallbacks (§6.2.1) when tokens are missing.
const (
	fallbackDotSize        = 10.0
	fallbackTailWidth      = 2.0
	fallbackItemPad        = 20.0
	fallbackFontSize       = 14.0
	fallbackTitleSpan      = 12.0
	fallbackDotBorderWidth = 2.0
	fallbackCustomHeadPadV = 4.0
	railPad                = 4.0
	contentGap             = 8.0
	titleContentGap        = 4.0
	spinnerPeriodSec       = 1.0
)

// TimelineMode selects which side content sits on (vertical) (§6.4).
type TimelineMode int

const (
	TimelineModeStart TimelineMode = iota
	TimelineModeAlternate
	TimelineModeEnd
)

// TimelineOrientation selects the main axis (§6.4).
type TimelineOrientation int

const (
	TimelineVertical TimelineOrientation = iota
	TimelineHorizontal
)

// TimelineVariant selects dot fill style (§6.4 TL-S9).
type TimelineVariant int

const (
	TimelineOutlined TimelineVariant = iota
	TimelineFilled
)

// TimelinePlacement overrides the mode default side per item (§6.4).
// Auto (0) follows the mode; Start/End pin one side.
type TimelinePlacement int

const (
	TimelinePlacementAuto TimelinePlacement = iota
	TimelinePlacementStart
	TimelinePlacementEnd
)

// TimelineStyle is a P0 placeholder for future semantic style hooks (§6.7 P1).
type TimelineStyle struct{}

// TimelineItem is one timeline node (§6.10).
type TimelineItem struct {
	Content     string
	ContentNode rendering.RenderObject
	Title       string
	TitleNode   rendering.RenderObject
	Color       string
	Icon        string
	IconNode    rendering.RenderObject
	Loading     bool
	Placement   TimelinePlacement
}

// Timeline is the Timeline widget (§6.10).
type Timeline struct {
	items        []TimelineItem
	mode         TimelineMode
	orientation  TimelineOrientation
	variant      TimelineVariant
	reverse      bool
	titleSpan    float64
	hasSpan      bool
	provider     *theme.Provider
	override     *theme.Tokens
	ariaLabel    string
	face         text.Face
	style        TimelineStyle
	reduceMotion bool
	// phase is the spinner phase in [0,1), advanced by Tick (UI) and read
	// by dot paint closures (raster): atomic bits, never a bare float
	// (R2-6). Single UI writer, so load-compute-store in Tick is exact.
	phase atomic.Uint64 // math.Float64bits
	// dotsSnap holds the frozen per-item dot inputs ([]dotSnap, copy on
	// write). Rebuilt on UI (rebuild paths + per-item setters); dot paint
	// closures load it on raster. Never mutated in place.
	dotsSnap atomic.Value

	host     *rendering.AbsoluteBox
	attached *scheduler.TickerRegistry
}

// NewTimeline creates a timeline (default mode=start, vertical, outlined).
func NewTimeline(items ...TimelineItem) *Timeline {
	t := &Timeline{mode: TimelineModeStart, variant: TimelineOutlined}
	t.host = rendering.NewAbsoluteBox(0, 0)
	t.host.SetRepaintBoundary(true)
	t.host.SetRelayoutBoundary(true)
	t.SetItems(items)
	return t
}

func (t *Timeline) themeTokens() theme.Tokens {
	if t != nil && t.override != nil {
		return *t.override
	}
	if t != nil && t.provider != nil {
		return t.provider.Current()
	}
	return theme.Default.Current()
}

func themeRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func clearHost(h *rendering.AbsoluteBox) {
	if h == nil {
		return
	}
	for _, c := range append([]rendering.RenderObject(nil), h.Children()...) {
		h.RemoveChild(c)
	}
}

// SetItems replaces the node list.
func (t *Timeline) SetItems(items []TimelineItem) {
	if t == nil {
		return
	}
	cp := make([]TimelineItem, len(items))
	copy(cp, items)
	t.items = cp
	t.rebuild()
}

// Items returns a copy in logical order.
func (t *Timeline) Items() []TimelineItem {
	if t == nil {
		return nil
	}
	cp := make([]TimelineItem, len(t.items))
	copy(cp, t.items)
	return cp
}

// ItemCount returns the logical item count (TL-S1).
func (t *Timeline) ItemCount() int {
	if t == nil {
		return 0
	}
	return len(t.items)
}

// displayOrder returns logical indices in render order.
func (t *Timeline) displayOrder() []int {
	if t == nil || len(t.items) == 0 {
		return nil
	}
	n := len(t.items)
	out := make([]int, n)
	if !t.reverse {
		for i := range out {
			out[i] = i
		}
		return out
	}
	for i := range out {
		out[i] = n - 1 - i
	}
	return out
}

// DisplayIndex maps a logical index to its display position (reverse aware).
func (t *Timeline) DisplayIndex(logical int) int {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return -1
	}
	if !t.reverse {
		return logical
	}
	return len(t.items) - 1 - logical
}

// DisplayLogical maps a display position back to the logical index.
func (t *Timeline) DisplayLogical(display int) int {
	order := t.displayOrder()
	if display < 0 || display >= len(order) {
		return -1
	}
	return order[display]
}

// DisplayItems returns items in render order.
func (t *Timeline) DisplayItems() []TimelineItem {
	if t == nil {
		return nil
	}
	order := t.displayOrder()
	out := make([]TimelineItem, len(order))
	for i, li := range order {
		out[i] = t.items[li]
	}
	return out
}

// SetMode sets start/alternate/end (default start).
func (t *Timeline) SetMode(m TimelineMode) {
	if t == nil {
		return
	}
	t.mode = m
	t.rebuild()
}

// Mode returns the mode.
func (t *Timeline) Mode() TimelineMode {
	if t == nil {
		return TimelineModeStart
	}
	return t.mode
}

// SetOrientation sets vertical/horizontal.
func (t *Timeline) SetOrientation(o TimelineOrientation) {
	if t == nil {
		return
	}
	t.orientation = o
	t.rebuild()
}

// Orientation returns the orientation.
func (t *Timeline) Orientation() TimelineOrientation {
	if t == nil {
		return TimelineVertical
	}
	return t.orientation
}

// SetVariant sets outlined/filled dots (TL-S9).
func (t *Timeline) SetVariant(v TimelineVariant) {
	if t == nil {
		return
	}
	t.variant = v
	t.rebuild()
}

// Variant returns the variant.
func (t *Timeline) Variant() TimelineVariant {
	if t == nil {
		return TimelineOutlined
	}
	return t.variant
}

// SetReverse toggles display reversal (TL-S4).
func (t *Timeline) SetReverse(b bool) {
	if t == nil || t.reverse == b {
		return
	}
	t.reverse = b
	t.rebuild()
}

// Reverse reports reversal.
func (t *Timeline) Reverse() bool { return t != nil && t.reverse }

// SetTitleSpan stores the title share (P0 default readable; fine layout P1).
func (t *Timeline) SetTitleSpan(v float64) {
	if t == nil {
		return
	}
	t.titleSpan = v
	t.hasSpan = true
	t.rebuild()
}

// TitleSpan returns the effective span (default 12).
func (t *Timeline) TitleSpan() float64 {
	if t != nil && t.hasSpan && t.titleSpan > 0 {
		return t.titleSpan
	}
	return fallbackTitleSpan
}

// SetTheme pins exact tokens (nil clears to provider).
func (t *Timeline) SetTheme(tok *theme.Tokens) {
	if t == nil {
		return
	}
	t.override = tok
	t.rebuild()
}

// SetProvider selects the theme source (nil selects process default).
func (t *Timeline) SetProvider(p *theme.Provider) {
	if t == nil {
		return
	}
	t.provider = p
	t.rebuild()
}

// SetFace sets the text face for title/content runs (nil clears).
func (t *Timeline) SetFace(f text.Face) {
	if t == nil {
		return
	}
	t.face = f
	t.rebuild()
}

// SetStyle stores the semantic style hook (P1 depth lives here later).
func (t *Timeline) SetStyle(s TimelineStyle) {
	if t == nil {
		return
	}
	t.style = s
	t.rebuild()
}

// SetAriaLabel sets the accessible name (empty keeps list unnamed).
func (t *Timeline) SetAriaLabel(s string) {
	if t == nil {
		return
	}
	t.ariaLabel = s
}

// AriaLabel returns the accessible name.
func (t *Timeline) AriaLabel() string {
	if t == nil {
		return ""
	}
	return t.ariaLabel
}

// Role is "list" (time-flow reading, §6.6).
func (t *Timeline) Role() string { return "list" }

// ItemRole is "listitem" per node (§6.6).
func (t *Timeline) ItemRole(int) string { return "listitem" }

// Focusable is always false: the body never steals Tab (§6.6).
func (t *Timeline) Focusable() bool { return false }

// ReadingOrder returns logical indices in render (speech) order.
func (t *Timeline) ReadingOrder() []int { return t.displayOrder() }

// ItemName joins title+content for speech (§6.6).
func (t *Timeline) ItemName(logical int) string {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return ""
	}
	it := t.items[logical]
	switch {
	case it.Title != "" && it.Content != "":
		return it.Title + " " + it.Content
	case it.Title != "":
		return it.Title
	default:
		return it.Content
	}
}

// effectivePlacement resolves mode default + per-item override (TL-S2).
// displayPos is the render position (alternate alternates by display order).
func (t *Timeline) effectivePlacement(logical, displayPos int) TimelinePlacement {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return TimelinePlacementEnd
	}
	if p := t.items[logical].Placement; p == TimelinePlacementStart || p == TimelinePlacementEnd {
		return p
	}
	switch t.mode {
	case TimelineModeEnd:
		return TimelinePlacementStart
	case TimelineModeAlternate:
		if displayPos%2 == 0 {
			return TimelinePlacementStart
		}
		return TimelinePlacementEnd
	default:
		return TimelinePlacementEnd
	}
}

// ItemPlacement returns the effective side for a logical index.
func (t *Timeline) ItemPlacement(logical int) TimelinePlacement {
	return t.effectivePlacement(logical, t.DisplayIndex(logical))
}

// ItemColor maps blue/red/green/gray/#hex via Theme (§6.2.2, TL-S5).
func (t *Timeline) ItemColor(logical int) render.RGBA {
	tok := t.themeTokens()
	name := "blue"
	if t != nil && logical >= 0 && logical < len(t.items) {
		name = strings.TrimSpace(t.items[logical].Color)
	}
	switch strings.ToLower(name) {
	case "", "blue":
		return themeRGBA(tok.ColorPrimary)
	case "red":
		return themeRGBA(tok.ColorError)
	case "green":
		return themeRGBA(tok.ColorSuccess)
	case "gray":
		return themeRGBA(tok.ColorTextDisabled)
	default:
		if strings.HasPrefix(name, "#") {
			if c, err := render.ParseHex(name); err == nil {
				return c
			}
		}
		return themeRGBA(tok.ColorPrimary)
	}
}

// ItemLoading reports item loading (TL-S4 pending semantic).
func (t *Timeline) ItemLoading(logical int) bool {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return false
	}
	return t.items[logical].Loading
}

// ItemHasIcon reports custom point coverage (TL-S6).
func (t *Timeline) ItemHasIcon(logical int) bool {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return false
	}
	it := t.items[logical]
	return it.IconNode != nil || it.Icon != "" || it.Loading
}

// HasLoadingSpinner reports whether any loading ticker is needed.
func (t *Timeline) HasLoadingSpinner() bool {
	if t == nil {
		return false
	}
	for i := range t.items {
		if t.items[i].Loading || t.items[i].Icon == "loading" {
			return true
		}
	}
	return false
}

// HasPending is the TL-04 query alias for the migrated pending semantic.
func (t *Timeline) HasPending() bool { return t.HasLoadingSpinner() }

// DotSize is the node diameter (§6.2.1).
func (t *Timeline) DotSize() float64 { return fallbackDotSize }

// DotBorderWidth follows lineWidthBold (§6.2.1).
func (t *Timeline) DotBorderWidth() float64 {
	if w := t.themeTokens().LineWidthBold; w > 0 {
		return w
	}
	return fallbackDotBorderWidth
}

// TailWidth follows lineWidthBold (§6.2.1).
func (t *Timeline) TailWidth() float64 {
	if w := t.themeTokens().LineWidthBold; w > 0 {
		return w
	}
	return fallbackTailWidth
}

// TailColor follows the split track (§6.2.1).
func (t *Timeline) TailColor() render.RGBA {
	return themeRGBA(t.themeTokens().ColorBorderSecondary)
}

// ItemPaddingBottom is padding*1.25 (§6.2.1).
func (t *Timeline) ItemPaddingBottom() float64 {
	if p := t.themeTokens().Padding; p > 0 {
		return p * 1.25
	}
	return fallbackItemPad
}

// FontSize follows the seed (§6.2.1).
func (t *Timeline) FontSize() float64 {
	if s := t.themeTokens().FontSize; s > 0 {
		return s
	}
	return fallbackFontSize
}

// CustomHeadPaddingVertical follows paddingXXS (§6.2.1).
func (t *Timeline) CustomHeadPaddingVertical() float64 {
	if p := t.themeTokens().PaddingXXS; p > 0 {
		return p
	}
	return fallbackCustomHeadPadV
}

// SetReduceMotion freezes spinner phase (a11y).
func (t *Timeline) SetReduceMotion(b bool) {
	if t == nil {
		return
	}
	t.reduceMotion = b
}

// Phase returns the spinner phase in [0,1).
func (t *Timeline) Phase() float64 {
	if t == nil {
		return 0
	}
	return math.Float64frombits(t.phase.Load())
}

// AttachTicker registers the loading ticker.
func (t *Timeline) AttachTicker(reg *scheduler.TickerRegistry) {
	if t == nil || reg == nil {
		return
	}
	if t.attached != nil && t.attached != reg {
		t.attached.Remove(t)
	}
	t.attached = reg
	reg.Add(t)
}

// Detach unregisters the ticker.
func (t *Timeline) Detach() {
	if t == nil || t.attached == nil {
		return
	}
	t.attached.Remove(t)
	t.attached = nil
}

// Tick advances spinner phase; stays registered.
func (t *Timeline) Tick(dt float64) bool {
	if t == nil {
		return false
	}
	if !t.HasLoadingSpinner() || t.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	p := math.Float64frombits(t.phase.Load())
	p += dt / spinnerPeriodSec
	p -= float64(int(p))
	if p < 0 {
		p++
	}
	t.phase.Store(math.Float64bits(p))
	if t.host != nil {
		t.host.MarkNeedsPaint()
	}
	return true
}

// WantsFrame reports spinner frame demand.
func (t *Timeline) WantsFrame() bool {
	return t != nil && t.HasLoadingSpinner() && !t.reduceMotion
}

// Node returns the tree node (layout/paint/hit through it).
func (t *Timeline) Node() rendering.RenderObject {
	if t == nil {
		return nil
	}
	return t.host
}

// ChromeNode mirrors Node (no overlay of its own).
func (t *Timeline) ChromeNode() rendering.RenderObject { return t.Node() }

// Layout sizes the host under constraints.
func (t *Timeline) Layout(c rendering.Constraints) rendering.Size {
	if t == nil || t.host == nil {
		return rendering.Size{}
	}
	if len(t.items) == 0 {
		return rendering.Size{}
	}
	t.rebuild()
	return t.host.Layout(c)
}

// SetItemColor updates one dot color with paint-only dirty (no layout).
// Dot paint reads the live item color, so no rebuild is needed.
func (t *Timeline) SetItemColor(logical int, color string) {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return
	}
	if t.items[logical].Color == color {
		return
	}
	t.items[logical].Color = color
	// R2-6: dot paint closures read the frozen dots snapshot, not live
	// items — refresh it (paint-only dirty preserved) instead of rebuilding
	// the whole tree. Toggles are user-driven (low frequency).
	t.refreshDots()
	if t.host != nil {
		t.host.MarkNeedsPaint()
	}
}

// SetItemLoading toggles one loading flag with paint-only dirty (see
// SetItemColor: frozen snapshot refreshed for this index).
func (t *Timeline) SetItemLoading(logical int, loading bool) {
	if t == nil || logical < 0 || logical >= len(t.items) {
		return
	}
	if t.items[logical].Loading == loading {
		return
	}
	t.items[logical].Loading = loading
	t.refreshDots()
	if t.host != nil {
		t.host.MarkNeedsPaint()
	}
}

// dotSnap is one item's frozen dot paint inputs (R2-6). Stored wholesale in
// dotsSnap (copy-on-write); dot closures load it per paint instead of
// reading the live items slice, colors or variant on raster.
type dotSnap struct {
	col      render.RGBA
	loading  bool
	hasIcon  bool
	iconNode rendering.RenderObject
	variant  TimelineVariant
	border   float64
}

// refreshDots rebuilds the frozen dot snapshot from current items (UI only:
// rebuild paths plus the two per-item setters above).
func (t *Timeline) refreshDots() {
	if t == nil {
		return
	}
	snap := make([]dotSnap, len(t.items))
	for li := range t.items {
		it := t.items[li]
		snap[li] = dotSnap{
			col:      t.ItemColor(li),
			loading:  it.Loading || it.Icon == "loading",
			hasIcon:  it.IconNode != nil || it.Icon != "",
			iconNode: it.IconNode,
			variant:  t.variant,
			border:   t.DotBorderWidth(),
		}
	}
	t.dotsSnap.Store(snap)
}

func textWH(s string, fontSize float64) (w, h float64) {
	if s == "" {
		return 0, 0
	}
	return rendering.EstimateTextSize(s, fontSize, 0.55)
}

func nodeWH(n rendering.RenderObject) (w, h float64) {
	if n == nil {
		return 0, 0
	}
	sz := n.Layout(rendering.Loose(1e9, 1e9))
	return sz.Width, sz.Height
}

func blockWH(title, content string, titleNode, contentNode rendering.RenderObject, fontSize float64) (w, h float64) {
	tw, th := textWH(title, fontSize)
	cw, ch := textWH(content, fontSize)
	if titleNode != nil {
		tw, th = nodeWH(titleNode)
	}
	if contentNode != nil {
		cw, ch = nodeWH(contentNode)
	}
	w = tw
	if cw > w {
		w = cw
	}
	h = th
	if ch > 0 {
		if h > 0 {
			h += titleContentGap
		}
		h += ch
	}
	return w, h
}

func (t *Timeline) rebuild() {
	if t == nil || t.host == nil {
		return
	}
	// Frozen dot inputs refresh with every rebuild (items final here).
	t.refreshDots()
	n := len(t.items)
	if n == 0 {
		clearHost(t.host)
		t.host.FixedWidth, t.host.FixedHeight = 0, 0
		return
	}
	if t.orientation == TimelineHorizontal {
		t.rebuildHorizontal()
		return
	}
	t.rebuildVertical()
}

func (t *Timeline) rebuildVertical() {
	tok := t.themeTokens()
	dot := t.DotSize()
	tailW := t.TailWidth()
	pad := t.ItemPaddingBottom()
	fs := t.FontSize()
	railW := dot + railPad*2

	order := t.displayOrder()
	sides := make([]TimelinePlacement, len(order))
	blockW := make([]float64, len(order))
	blockH := make([]float64, len(order))
	for d, li := range order {
		sides[d] = t.effectivePlacement(li, d)
		it := t.items[li]
		w, h := blockWH(it.Title, it.Content, it.TitleNode, it.ContentNode, fs)
		if h < dot {
			h = dot
		}
		blockW[d], blockH[d] = w, h
	}
	single := true
	for _, s := range sides {
		if s != sides[0] {
			single = false
			break
		}
	}
	leftW, rightW := 0.0, 0.0
	if single {
		m := 0.0
		for _, w := range blockW {
			if w > m {
				m = w
			}
		}
		if sides[0] == TimelinePlacementStart {
			leftW = m
		} else {
			rightW = m
		}
	} else {
		for d, w := range blockW {
			if sides[d] == TimelinePlacementStart {
				if w > leftW {
					leftW = w
				}
			} else if w > rightW {
				rightW = w
			}
		}
	}
	var hostW float64
	if single {
		hostW = railW + contentGap + leftW + rightW
	} else {
		hostW = leftW + contentGap + railW + contentGap + rightW
	}
	hostH := 0.0
	for _, h := range blockH {
		hostH += h
	}
	if len(blockH) > 1 {
		hostH += pad * float64(len(blockH)-1)
	}
	var railX float64
	if single {
		if sides[0] == TimelinePlacementStart {
			railX = leftW + contentGap
		} else {
			railX = 0
		}
	} else {
		railX = leftW + contentGap
	}
	dotX := railX + (railW-dot)/2
	tailX := railX + railW/2 - tailW/2

	clearHost(t.host)
	t.host.FixedWidth, t.host.FixedHeight = hostW, hostH
	tailCol := themeRGBA(tok.ColorBorderSecondary)
	titleCol := themeRGBA(tok.ColorTextSecondary)
	contentCol := themeRGBA(tok.ColorText)

	y := 0.0
	for d, li := range order {
		it := t.items[li]
		h := blockH[d]
		side := sides[d]
		var contentX float64
		if single {
			if side == TimelinePlacementStart {
				contentX = 0
			} else {
				contentX = railW + contentGap
			}
		} else {
			if side == TimelinePlacementStart {
				contentX = 0
			} else {
				contentX = leftW + contentGap + railW + contentGap
			}
		}
		// R2-6: dot inputs come from the frozen dots snapshot (UI-stored
		// on rebuild/setters, loaded here on raster) — never live widget
		// state. Only the animation phase stays live, via atomic load.
		liLive := li
		dotBox := rendering.NewRenderBox()
		dotBox.FixedWidth, dotBox.FixedHeight = dot, dot
		dotBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || t == nil {
				return
			}
			var ds dotSnap
			if snap, ok := t.dotsSnap.Load().([]dotSnap); ok && liLive >= 0 && liLive < len(snap) {
				ds = snap[liLive]
			}
			cx, cy := dot/2, dot/2
			if ds.loading {
				rendering.StrokeCircle(pc, cx, cy, dot/2-1, 1.5, ds.col.R, ds.col.G, ds.col.B, 1)
				ang := math.Float64frombits(t.phase.Load()) * 6.283185307179586
				dx := (dot/2 - 2) * 0.7 * math.Cos(ang)
				dy := (dot/2 - 2) * 0.7 * math.Sin(ang)
				rendering.FillCircle(pc, cx+dx, cy+dy, 1.8, ds.col.R, ds.col.G, ds.col.B, 1)
				return
			}
			if ds.hasIcon {
				rendering.FillCircle(pc, cx, cy, dot/2, ds.col.R, ds.col.G, ds.col.B, 1)
				rendering.FillCircle(pc, cx, cy, dot/4, 1, 1, 1, 1)
				return
			}
			if ds.variant == TimelineFilled {
				rendering.FillCircle(pc, cx, cy, dot/2, ds.col.R, ds.col.G, ds.col.B, 1)
				return
			}
			rendering.FillCircle(pc, cx, cy, dot/2, 1, 1, 1, 1)
			rendering.StrokeCircle(pc, cx, cy, dot/2-1, ds.border, ds.col.R, ds.col.G, ds.col.B, 1)
		}
		t.host.Place(dotBox, dotX, y+2)
		if d < len(order)-1 {
			tailTop := y + 2 + dot
			tailBottom := y + h + pad
			if tailBottom > tailTop {
				tail := rendering.NewRenderColorBox(tailW, tailBottom-tailTop, tailCol.R, tailCol.G, tailCol.B, tailCol.A)
				t.host.Place(tail, tailX, tailTop)
			}
		}
		cy := y
		if it.TitleNode != nil {
			t.host.Place(it.TitleNode, contentX, cy)
			_, th := nodeWH(it.TitleNode)
			cy += th + titleContentGap
		} else if it.Title != "" {
			rt := rendering.NewRenderText(it.Title)
			rt.FontSize = fs
			if t.face != nil {
				rt.SetFace(t.face)
			}
			rt.SetColor(titleCol.R, titleCol.G, titleCol.B, titleCol.A)
			t.host.Place(rt, contentX, cy)
			_, th := textWH(it.Title, fs)
			cy += th + titleContentGap
		}
		if it.ContentNode != nil {
			t.host.Place(it.ContentNode, contentX, cy)
		} else if it.Content != "" {
			rc := rendering.NewRenderText(it.Content)
			rc.FontSize = fs
			if t.face != nil {
				rc.SetFace(t.face)
			}
			rc.SetColor(contentCol.R, contentCol.G, contentCol.B, contentCol.A)
			t.host.Place(rc, contentX, cy)
		}
		y += h + pad
	}
}

func (t *Timeline) rebuildHorizontal() {
	tok := t.themeTokens()
	dot := t.DotSize()
	tailW := t.TailWidth()
	gap := t.ItemPaddingBottom()
	fs := t.FontSize()
	railH := dot + railPad*2

	order := t.displayOrder()
	sides := make([]TimelinePlacement, len(order))
	blockW := make([]float64, len(order))
	blockH := make([]float64, len(order))
	topH, bottomH := 0.0, 0.0
	for d, li := range order {
		sides[d] = t.effectivePlacement(li, d)
		it := t.items[li]
		w, h := blockWH(it.Title, it.Content, it.TitleNode, it.ContentNode, fs)
		if w < dot {
			w = dot
		}
		blockW[d], blockH[d] = w, h
		if sides[d] == TimelinePlacementStart {
			if h > topH {
				topH = h
			}
		} else if h > bottomH {
			bottomH = h
		}
	}
	mixed := topH > 0 && bottomH > 0
	var railY, hostH float64
	if mixed {
		railY = topH + contentGap
		hostH = topH + contentGap + railH + contentGap + bottomH
	} else if bottomH > 0 {
		railY = 0
		hostH = railH + contentGap + bottomH
	} else {
		railY = topH + contentGap
		hostH = topH + contentGap + railH
	}
	hostW := 0.0
	for _, w := range blockW {
		hostW += w
	}
	if len(blockW) > 1 {
		hostW += gap * float64(len(blockW)-1)
	}

	clearHost(t.host)
	t.host.FixedWidth, t.host.FixedHeight = hostW, hostH
	tailCol := themeRGBA(tok.ColorBorderSecondary)
	titleCol := themeRGBA(tok.ColorTextSecondary)
	contentCol := themeRGBA(tok.ColorText)

	x := 0.0
	for d, li := range order {
		it := t.items[li]
		w := blockW[d]
		side := sides[d]
		// R2-6: dot inputs come from the frozen dots snapshot (UI-stored
		// on rebuild/setters, loaded here on raster) — never live widget
		// state. Only the animation phase stays live, via atomic load.
		liLive := li
		dotBox := rendering.NewRenderBox()
		dotBox.FixedWidth, dotBox.FixedHeight = dot, dot
		dotBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || t == nil {
				return
			}
			var ds dotSnap
			if snap, ok := t.dotsSnap.Load().([]dotSnap); ok && liLive >= 0 && liLive < len(snap) {
				ds = snap[liLive]
			}
			cx, cy := dot/2, dot/2
			if ds.loading {
				rendering.StrokeCircle(pc, cx, cy, dot/2-1, 1.5, ds.col.R, ds.col.G, ds.col.B, 1)
				ang := math.Float64frombits(t.phase.Load()) * 6.283185307179586
				dx := (dot/2 - 2) * 0.7 * math.Cos(ang)
				dy := (dot/2 - 2) * 0.7 * math.Sin(ang)
				rendering.FillCircle(pc, cx+dx, cy+dy, 1.8, ds.col.R, ds.col.G, ds.col.B, 1)
				return
			}
			if ds.hasIcon {
				rendering.FillCircle(pc, cx, cy, dot/2, ds.col.R, ds.col.G, ds.col.B, 1)
				rendering.FillCircle(pc, cx, cy, dot/4, 1, 1, 1, 1)
				return
			}
			if ds.variant == TimelineFilled {
				rendering.FillCircle(pc, cx, cy, dot/2, ds.col.R, ds.col.G, ds.col.B, 1)
				return
			}
			rendering.FillCircle(pc, cx, cy, dot/2, 1, 1, 1, 1)
			rendering.StrokeCircle(pc, cx, cy, dot/2-1, ds.border, ds.col.R, ds.col.G, ds.col.B, 1)
		}
		dotX := x + (w-dot)/2
		t.host.Place(dotBox, dotX, railY+(railH-dot)/2)
		if d < len(order)-1 {
			tailLeft := dotX + dot
			tailRight := x + w + gap
			if tailRight > tailLeft {
				tailY := railY + railH/2 - tailW/2
				tail := rendering.NewRenderColorBox(tailRight-tailLeft, tailW, tailCol.R, tailCol.G, tailCol.B, tailCol.A)
				t.host.Place(tail, tailLeft, tailY)
			}
		}
		var cy float64
		if mixed {
			if side == TimelinePlacementStart {
				cy = 0
			} else {
				cy = railY + railH + contentGap
			}
		} else if bottomH > 0 {
			cy = railY + railH + contentGap
		} else {
			cy = 0
		}
		if it.TitleNode != nil {
			t.host.Place(it.TitleNode, x, cy)
			_, th := nodeWH(it.TitleNode)
			cy += th + titleContentGap
		} else if it.Title != "" {
			rt := rendering.NewRenderText(it.Title)
			rt.FontSize = fs
			if t.face != nil {
				rt.SetFace(t.face)
			}
			rt.SetColor(titleCol.R, titleCol.G, titleCol.B, titleCol.A)
			t.host.Place(rt, x, cy)
			_, th := textWH(it.Title, fs)
			cy += th + titleContentGap
		}
		if it.ContentNode != nil {
			t.host.Place(it.ContentNode, x, cy)
		} else if it.Content != "" {
			rc := rendering.NewRenderText(it.Content)
			rc.FontSize = fs
			if t.face != nil {
				rc.SetFace(t.face)
			}
			rc.SetColor(contentCol.R, contentCol.G, contentCol.B, contentCol.A)
			t.host.Place(rc, x, cy)
		}
		x += w + gap
	}
}
