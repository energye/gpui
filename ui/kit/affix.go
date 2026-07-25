package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Affix defaults — docs/antd/affix.md §6.2 / §6.3
// components/affix + utils.ts (getFixedTop / getFixedBottom).
const (
	// DefaultAffixOffsetTop is antd offsetTop default when neither offset is set.
	DefaultAffixOffsetTop = 0.0
	// DefaultAffixFontSize is §6.2 fontSize middle.
	DefaultAffixFontSize = 14.0
	// DefaultAffixBorderRadius is §6.2 borderRadius.
	DefaultAffixBorderRadius = 6.0
	// DefaultAffixLineWidth is §6.2 lineWidth.
	DefaultAffixLineWidth = 1.0
	// DefaultAffixFocusRingOutset is §6.2 focus ring outset (≈1.5px visible).
	DefaultAffixFocusRingOutset = 1.5
)

// Affix pins children in the visible range of a scroll target (antd Affix).
//
//	affixHost (placeholder · layout size always = content)
//	  └─ Content
//
// Desktop mapping of antd target(): SetScrollTarget(*ScrollViewport).
// UpdatePosition / Evaluate implement getFixedTop + getFixedBottom + onChange.
// When affixed, paint origin shifts so content sticks (placeholder keeps flow size — AFX-S3).
//
// Product contract: docs/antd/affix.md §6 (P0).
// https://ant.design/components/affix
type Affix struct {
	// Root is the stable mount node (affixHost).
	Root *affixHost

	// Content is the wrapped child (antd children).
	Content core.Node

	// OffsetTop is distance from target top that triggers affix (antd offsetTop).
	// Default 0 when neither offset is set. Explicit 0 via SetOffsetTop is distinct
	// from "only offsetBottom" (see offsetTopSet).
	OffsetTop float64
	// OffsetBottom is distance from target bottom that triggers affix (antd offsetBottom).
	OffsetBottom float64

	offsetTopSet    bool
	offsetBottomSet bool

	// Affixed is true when currently fixed (antd lastAffix). Public for hosts/tests;
	// prefer IsAffixed() after UpdatePosition / Evaluate.
	Affixed bool

	// OnChange fires when affixed state flips (antd onChange).
	OnChange func(affixed bool)

	// ScrollTarget is the desktop mapping of antd target() scroll container.
	// nil → no auto scroll hook; call Evaluate / UpdatePosition from the host.
	ScrollTarget *primitive.ScrollViewport
	scrollHooked bool

	// ContentTop is the Y of this affix in scroll-content coordinates.
	// When contentTopSet=false, measured from layout offsets under ScrollTarget.
	ContentTop    float64
	contentTopSet bool

	// stickDY is the paint/hit shift applied while affixed (host-local).
	stickDY float64
	// mode: 0 none, 1 top, 2 bottom
	affixMode int

	// ContentWidth / ContentHeight last measured placeholder size (AFX-S3).
	ContentWidth  float64
	ContentHeight float64

	Theme     *core.Theme
	Style     Style
	AriaLabel string
}

// affixHost is the placeholder root: layout size always matches content;
// paint/hit apply stickDY while affixed so the pin tracks the viewport edge.
type affixHost struct {
	core.NodeBase
	owner *Affix
}

func (h *affixHost) TypeID() string { return "kit.Affix" }

func (h *affixHost) Layout(c core.Constraints) core.Size {
	kids := h.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		h.SetSize(out)
		if h.owner != nil {
			h.owner.ContentWidth, h.owner.ContentHeight = out.Width, out.Height
		}
		return out
	}
	sz := kids[0].Layout(c.Expand())
	// Content stays at local (0,0); stick is paint/hit only so placeholder size
	// does not jump when affixed (AFX-S3 / antd placeholderStyle).
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	h.SetSize(out)
	if h.owner != nil {
		h.owner.ContentWidth, h.owner.ContentHeight = out.Width, out.Height
	}
	return out
}

func (h *affixHost) Paint(pc *core.PaintContext) {
	if h.owner != nil && h.owner.Affixed && h.owner.stickDY != 0 && pc != nil {
		adj := pc.WithOrigin(pc.Origin.Add(core.Point{Y: h.owner.stickDY}))
		h.DefaultPaintChildren(adj)
		return
	}
	h.DefaultPaintChildren(pc)
}

func (h *affixHost) HitTest(p core.Point) core.Node {
	if h.owner != nil && h.owner.Affixed && h.owner.stickDY != 0 {
		// Inverse of paint shift so hit tracks the pinned visual (hit≈paint).
		return h.DefaultHitTest(core.Point{X: p.X, Y: p.Y - h.owner.stickDY})
	}
	return h.DefaultHitTest(p)
}

// NewAffix wraps content. Defaults: offsetTop=0, not affixed, no scroll target.
func NewAffix(content core.Node) *Affix {
	a := &Affix{
		Content:   content,
		OffsetTop: DefaultAffixOffsetTop,
	}
	h := &affixHost{owner: a}
	h.Init(h)
	h.Hit = core.HitDefer
	a.Root = h
	a.rebuild()
	return a
}

// Node returns the mount root (stable identity across rebuild).
func (a *Affix) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// IsAffixed reports the current fixed state.
func (a *Affix) IsAffixed() bool {
	return a != nil && a.Affixed
}

// AffixMode returns "top", "bottom", or "" when not affixed.
func (a *Affix) AffixMode() string {
	if a == nil || !a.Affixed {
		return ""
	}
	switch a.affixMode {
	case 1:
		return "top"
	case 2:
		return "bottom"
	default:
		return ""
	}
}

// SetContent replaces children (Anchor / hosts rewiring).
func (a *Affix) SetContent(content core.Node) {
	if a == nil {
		return
	}
	a.Content = content
	a.rebuild()
}

// SetOffsetTop sets antd offsetTop (explicit, including 0).
func (a *Affix) SetOffsetTop(top float64) {
	if a == nil {
		return
	}
	a.OffsetTop = top
	a.offsetTopSet = true
	a.UpdatePosition()
}

// SetOffsetBottom sets antd offsetBottom (explicit, including 0).
func (a *Affix) SetOffsetBottom(bottom float64) {
	if a == nil {
		return
	}
	a.OffsetBottom = bottom
	a.offsetBottomSet = true
	a.UpdatePosition()
}

// ClearOffsetBottom unsets offsetBottom (antd undefined).
func (a *Affix) ClearOffsetBottom() {
	if a == nil {
		return
	}
	a.OffsetBottom = 0
	a.offsetBottomSet = false
	a.UpdatePosition()
}

// SetContentTop sets the Y of this affix in scroll-content coordinates.
// Prefer this for tests and gallery when the host does not rely on layout walk.
func (a *Affix) SetContentTop(y float64) {
	if a == nil {
		return
	}
	a.ContentTop = y
	a.contentTopSet = true
	a.UpdatePosition()
}

// SetOnChange sets the affixed-state callback (antd onChange).
func (a *Affix) SetOnChange(fn func(affixed bool)) {
	if a == nil {
		return
	}
	a.OnChange = fn
}

// SetScrollTarget sets the scroll container (desktop target()).
// Installs OnScroll → UpdatePosition (antd scroll / resize listeners).
func (a *Affix) SetScrollTarget(sv *primitive.ScrollViewport) {
	if a == nil {
		return
	}
	if a.ScrollTarget == sv && a.scrollHooked {
		return
	}
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
		a.UpdatePosition()
	}
	a.scrollHooked = true
	a.UpdatePosition()
}

// SetTheme applies a theme override (Token reads for §6.2 metrics).
func (a *Affix) SetTheme(th *core.Theme) {
	if a == nil {
		return
	}
	a.Theme = th
	a.rebuild()
}

// SetStyle applies optional shallow overrides.
func (a *Affix) SetStyle(st Style) {
	if a == nil {
		return
	}
	a.Style = st
	a.rebuild()
}

// SetAriaLabel sets an accessible name on the root (decorative by default).
func (a *Affix) SetAriaLabel(label string) {
	if a == nil {
		return
	}
	a.AriaLabel = label
	a.rebuild()
}

// FontSize returns §6.2 middle font size from Theme (fallback DefaultAffixFontSize).
func (a *Affix) FontSize() float64 {
	th := a.theme()
	if a != nil && a.Style.FontSize > 0 {
		return a.Style.FontSize
	}
	return th.SizeOr(core.TokenFontSize, DefaultAffixFontSize)
}

// BorderRadius returns §6.2 border radius from Theme.
func (a *Affix) BorderRadius() float64 {
	if a != nil && a.Style.hasRadius() {
		return a.Style.Radius
	}
	return a.theme().SizeOr(core.TokenBorderRadius, DefaultAffixBorderRadius)
}

// LineWidth returns §6.2 line width from Theme.
func (a *Affix) LineWidth() float64 {
	return a.theme().SizeOr(core.TokenLineWidth, DefaultAffixLineWidth)
}

// FocusRingOutset returns §6.2 focus ring outset baseline.
func (a *Affix) FocusRingOutset() float64 {
	return DefaultAffixFocusRingOutset
}

// ResolvedOffsetTop returns (value, enabled) for the top pin rule.
// enabled is false when only offsetBottom is set (antd internalOffsetTop undefined).
func (a *Affix) ResolvedOffsetTop() (float64, bool) {
	if a == nil {
		return DefaultAffixOffsetTop, true
	}
	if a.offsetTopSet {
		return a.OffsetTop, true
	}
	if a.offsetBottomSet {
		return 0, false
	}
	return DefaultAffixOffsetTop, true
}

// ResolvedOffsetBottom returns (value, enabled).
func (a *Affix) ResolvedOffsetBottom() (float64, bool) {
	if a == nil || !a.offsetBottomSet {
		return 0, false
	}
	return a.OffsetBottom, true
}

// PlaceholderSize returns the layout box kept while affixed (AFX-S3).
func (a *Affix) PlaceholderSize() core.Size {
	if a == nil {
		return core.Size{}
	}
	return core.Size{Width: a.ContentWidth, Height: a.ContentHeight}
}

// MeasureContentTop returns ContentTop or walks layout offsets under ScrollTarget.
func (a *Affix) MeasureContentTop() float64 {
	if a == nil {
		return 0
	}
	if a.contentTopSet {
		return a.ContentTop
	}
	if a.Root == nil {
		return 0
	}
	y := 0.0
	for n := core.Node(a.Root); n != nil; n = n.Parent() {
		if a.ScrollTarget != nil && n == a.ScrollTarget {
			break
		}
		y += n.Base().Offset().Y
	}
	return y
}

// UpdatePosition recomputes affixed from ScrollTarget + geometry (antd updatePosition).
// No-op when ScrollTarget is nil — use Evaluate for pure unit tests.
func (a *Affix) UpdatePosition() {
	if a == nil {
		return
	}
	if a.ScrollTarget == nil {
		return
	}
	vh := a.ScrollTarget.Size().Height
	if vh <= 0 {
		vh = a.ScrollTarget.Height
	}
	ch := a.ContentHeight
	if ch <= 0 && a.Root != nil {
		ch = a.Root.Size().Height
	}
	a.Evaluate(a.ScrollTarget.ScrollY, vh, a.MeasureContentTop(), ch)
}

// Evaluate applies antd getFixedTop / getFixedBottom against synthetic geometry.
// scrollY / viewportH are target scroll metrics; contentTop / contentH are the
// placeholder box in scroll-content coordinates.
//
//	scroll < 阈 → static
//	top rule   → affixed top + onChange(true)
//	bottom rule → affixed bottom + onChange(true)
//	release    → onChange(false); placeholder size unchanged
func (a *Affix) Evaluate(scrollY, viewportH, contentTop, contentH float64) {
	if a == nil {
		return
	}
	if contentH < 0 {
		contentH = 0
	}
	a.ContentTop = contentTop
	a.ContentHeight = contentH
	if a.ContentWidth <= 0 && a.Root != nil {
		a.ContentWidth = a.Root.Size().Width
	}

	// Screen (viewport-local) edges of the placeholder, matching getTargetRect math:
	// placeholder.top = contentTop - scrollY ; target.top = 0 ; target.bottom = viewportH.
	phTop := contentTop - scrollY
	phBottom := phTop + contentH

	topVal, useTop := a.ResolvedOffsetTop()
	botVal, useBot := a.ResolvedOffsetBottom()

	newAffixed := false
	mode := 0
	stick := 0.0

	// getFixedTop: target.top > placeholder.top - offsetTop  →  0 > phTop - offsetTop
	if useTop {
		if roundAffix(0) > roundAffix(phTop)-roundAffix(topVal) {
			newAffixed = true
			mode = 1
			// stick so painted top == offsetTop: phTop + stick = topVal
			stick = topVal - phTop
		}
	}
	// getFixedBottom only when top did not pin (antd checks top first, else bottom).
	if !newAffixed && useBot && viewportH > 0 {
		// target.bottom < placeholder.bottom + offsetBottom
		if roundAffix(viewportH) < roundAffix(phBottom)+roundAffix(botVal) {
			newAffixed = true
			mode = 2
			// stick so painted bottom == viewportH - offsetBottom
			// phBottom + stick = viewportH - botVal
			stick = viewportH - botVal - phBottom
		}
	}

	prev := a.Affixed
	a.Affixed = newAffixed
	a.affixMode = mode
	a.stickDY = stick
	if a.Root != nil {
		a.Root.MarkNeedsPaint()
	}
	if prev != newAffixed && a.OnChange != nil {
		a.OnChange(newAffixed)
	}
}

// SyncFromScroll is an alias of UpdatePosition (Anchor-style naming).
func (a *Affix) SyncFromScroll() { a.UpdatePosition() }

func (a *Affix) theme() *core.Theme {
	var n core.Node
	if a != nil && a.Root != nil {
		n = a.Root
	}
	return themeOf(a.fieldTheme(), n)
}

func (a *Affix) fieldTheme() *core.Theme {
	if a == nil {
		return nil
	}
	return a.Theme
}

func (a *Affix) rebuild() {
	if a == nil {
		return
	}
	if a.Root == nil {
		h := &affixHost{owner: a}
		h.Init(h)
		h.Hit = core.HitDefer
		a.Root = h
	}
	a.Root.owner = a
	a.Root.ClearChildren()
	if a.Content != nil {
		a.Root.AddChild(a.Content)
	}
	// Affix itself is a layout host, not a focusable control. Children keep a11y.
	a.Root.Role = "presentation"
	if a.AriaLabel != "" {
		a.Root.Label = a.AriaLabel
	} else {
		a.Root.Label = "Affix"
	}
	a.Root.SetThemeHook(func(*core.Theme) {
		// Token consumers (FontSize/BorderRadius) re-read on next query; chrome is content-driven.
		a.Root.MarkNeedsPaint()
	})
	a.Root.MarkNeedsLayout()
	a.Root.MarkNeedsPaint()
}

func roundAffix(v float64) float64 {
	// Match Math.round used by antd utils.ts thresholds.
	if v >= 0 {
		return float64(int(v + 0.5))
	}
	return float64(int(v - 0.5))
}
