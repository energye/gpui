// Package splitter implements the Splitter control (docs/antd/splitter.md §6).
//
// Composition over new frameworks: the widget owns a rendering RenderObject
// node (Node) and reuses ui/rendering for draw, ui/theme for color.
// No new event or frame system.
package splitter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Geometry baselines (docs/antd/splitter.md §6.2.1, scale=1).
// Widgets read live values from theme tokens; these are fallbacks.
const (
	DefaultSplitBarSize          = 2.0
	DefaultSplitTriggerSize      = 6.0
	DefaultSplitBarDraggableSize = 20.0
	DefaultSplitFocusOutset      = 1.5
	DefaultSplitContainerMain    = 600.0
	DefaultSplitContainerCross   = 200.0
	DefaultSplitKeyboardStep     = 4.0
	DefaultSplitCollapseButtonW  = 12.0
	DefaultSplitCollapseButtonH  = 24.0
)

// Orientation selects horizontal or vertical layout.
type Orientation int

const (
	Horizontal Orientation = iota
	Vertical
)

// CollapseSide selects which panel of a bar collapses.
type CollapseSide string

const (
	CollapseStart CollapseSide = "start"
	CollapseEnd   CollapseSide = "end"
)

// CollapsibleIconMode selects collapse icon visibility.
type CollapsibleIconMode string

const (
	CollapsibleIconAuto   CollapsibleIconMode = "auto"
	CollapsibleIconAlways CollapsibleIconMode = "always"
	CollapsibleIconNever  CollapsibleIconMode = "never"
)

// SemanticKey names the classNames/styles hooks (P1 semantic DOM,
// docs/antd/splitter.md §6.7-§6.8: root/bar/panel/handle).
type SemanticKey string

const (
	SemanticRoot   SemanticKey = "root"
	SemanticBar    SemanticKey = "bar"
	SemanticPanel  SemanticKey = "panel"
	SemanticHandle SemanticKey = "handle"
)

// Style is an optional business override (P1 customize.tsx hook).
// Zero value disables every field; set a Use flag to enable one.
type Style struct {
	Bg, Text          render.RGBA
	UseBg, UseText bool
}

// SplitterDim is px or percent (antd number | 'xx%'). Unset means auto.
type SplitterDim struct {
	kind  int // 0 auto, 1 px, 2 percent
	value float64
}

// DimPx builds a px dim.
func DimPx(px float64) SplitterDim {
	if px < 0 {
		px = 0
	}
	return SplitterDim{kind: 1, value: px}
}

// DimPercent builds a percent dim (0..100, e.g. 40 -> "40%").
func DimPercent(pct float64) SplitterDim {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return SplitterDim{kind: 2, value: pct}
}

// ParseDim parses px ("120") or percent ("40%"). Empty returns auto.
func ParseDim(s string) (SplitterDim, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return SplitterDim{}, nil
	}
	if strings.HasSuffix(t, "%") {
		num := strings.TrimSpace(strings.TrimSuffix(t, "%"))
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return SplitterDim{}, fmt.Errorf("splitter: bad percent %q", s)
		}
		return DimPercent(v), nil
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return SplitterDim{}, fmt.Errorf("splitter: bad px %q", s)
	}
	return DimPx(v), nil
}

// IsSet reports whether the dim was explicitly set.
func (d SplitterDim) IsSet() bool { return d.kind != 0 }

// Resolve converts the dim to px under container length L.
func (d SplitterDim) Resolve(L float64) float64 {
	if L < 0 {
		L = 0
	}
	switch d.kind {
	case 1:
		if d.value < 0 {
			return 0
		}
		return d.value
	case 2:
		return L * d.value / 100
	default:
		return -1
	}
}

// String renders px or percent (auto is "").
func (d SplitterDim) String() string {
	switch d.kind {
	case 1:
		return strconv.FormatFloat(d.value, 'f', -1, 64)
	case 2:
		return strconv.FormatFloat(d.value, 'f', -1, 64) + "%"
	default:
		return ""
	}
}

// SplitterPanel is one panel (docs/antd/splitter.md §6.10 Panel).
type SplitterPanel struct {
	child rendering.RenderObject

	min         SplitterDim
	max         SplitterDim
	size        SplitterDim
	defaultSize SplitterDim
	sizeSet     bool

	resizable          bool
	collapsible        bool
	collapsibleStart   bool
	collapsibleEnd     bool
	showIcon           CollapsibleIconMode
	showIconSet        bool
	destroyOnHidden    bool
	destroyOnHiddenSet bool
}

// NewSplitterPanel creates a panel hosting child (nil child allowed).
func NewSplitterPanel(child rendering.RenderObject) *SplitterPanel {
	return &SplitterPanel{child: child, resizable: true, showIcon: CollapsibleIconAuto}
}

// SetChild replaces the hosted content.
func (p *SplitterPanel) SetChild(child rendering.RenderObject) {
	if p == nil {
		return
	}
	p.child = child
}

// Child returns the hosted content.
func (p *SplitterPanel) Child() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.child
}

// SetMin sets the min threshold dim.
func (p *SplitterPanel) SetMin(d SplitterDim) {
	if p == nil {
		return
	}
	p.min = d
}

// SetMax sets the max threshold dim.
func (p *SplitterPanel) SetMax(d SplitterDim) {
	if p == nil {
		return
	}
	p.max = d
}

// SetSize sets the controlled size (marks controlled path).
func (p *SplitterPanel) SetSize(d SplitterDim) {
	if p == nil {
		return
	}
	p.size = d
	p.sizeSet = true
}

// SetDefaultSize sets the initial size (uncontrolled only).
func (p *SplitterPanel) SetDefaultSize(d SplitterDim) {
	if p == nil {
		return
	}
	p.defaultSize = d
}

// SetMinPx sets min in px.
func (p *SplitterPanel) SetMinPx(v float64) { p.SetMin(DimPx(v)) }

// SetMaxPx sets max in px.
func (p *SplitterPanel) SetMaxPx(v float64) { p.SetMax(DimPx(v)) }

// SetSizePx sets controlled size in px.
func (p *SplitterPanel) SetSizePx(v float64) { p.SetSize(DimPx(v)) }

// SetDefaultSizePx sets initial size in px.
func (p *SplitterPanel) SetDefaultSizePx(v float64) { p.SetDefaultSize(DimPx(v)) }

// SetMinPercent sets min in percent.
func (p *SplitterPanel) SetMinPercent(v float64) { p.SetMin(DimPercent(v)) }

// SetMaxPercent sets max in percent.
func (p *SplitterPanel) SetMaxPercent(v float64) { p.SetMax(DimPercent(v)) }

// SetSizePercent sets controlled size in percent.
func (p *SplitterPanel) SetSizePercent(v float64) { p.SetSize(DimPercent(v)) }

// SetDefaultSizePercent sets initial size in percent.
func (p *SplitterPanel) SetDefaultSizePercent(v float64) { p.SetDefaultSize(DimPercent(v)) }

// ClearSize clears the controlled flag (back to uncontrolled).
func (p *SplitterPanel) ClearSize() {
	if p == nil {
		return
	}
	p.size = SplitterDim{}
	p.sizeSet = false
}

// HasSize reports whether controlled size was set.
func (p *SplitterPanel) HasSize() bool { return p != nil && p.sizeSet }

// SetResizable sets whether the panel can be dragged (default true).
func (p *SplitterPanel) SetResizable(b bool) {
	if p == nil {
		return
	}
	p.resizable = b
}

// Resizable reports the resizable flag.
func (p *SplitterPanel) Resizable() bool { return p == nil || p.resizable }

// SetCollapsible sets both sides collapsible.
func (p *SplitterPanel) SetCollapsible(b bool) {
	if p == nil {
		return
	}
	p.collapsible = b
	p.collapsibleStart = b
	p.collapsibleEnd = b
}

// SetCollapsibleSides sets start/end collapsible separately.
func (p *SplitterPanel) SetCollapsibleSides(start, end bool) {
	if p == nil {
		return
	}
	p.collapsibleStart = start
	p.collapsibleEnd = end
	p.collapsible = start || end
}

// Collapsible reports whether any side can collapse.
func (p *SplitterPanel) Collapsible() bool { return p != nil && p.collapsible }

// CollapsibleStart reports start-side collapsible.
func (p *SplitterPanel) CollapsibleStart() bool { return p != nil && p.collapsibleStart }

// CollapsibleEnd reports end-side collapsible.
func (p *SplitterPanel) CollapsibleEnd() bool { return p != nil && p.collapsibleEnd }

// SetShowCollapsibleIcon sets icon visibility mode.
func (p *SplitterPanel) SetShowCollapsibleIcon(mode CollapsibleIconMode) {
	if p == nil {
		return
	}
	switch mode {
	case CollapsibleIconAlways, CollapsibleIconNever:
		p.showIcon = mode
	default:
		p.showIcon = CollapsibleIconAuto
	}
	p.showIconSet = true
}

// ShowCollapsibleIcon returns the icon mode.
func (p *SplitterPanel) ShowCollapsibleIcon() CollapsibleIconMode {
	if p == nil || !p.showIconSet {
		return CollapsibleIconAuto
	}
	return p.showIcon
}

// SetDestroyOnHidden overrides the global flag for this panel.
func (p *SplitterPanel) SetDestroyOnHidden(b bool) {
	if p == nil {
		return
	}
	p.destroyOnHidden = b
	p.destroyOnHiddenSet = true
}

func (p *SplitterPanel) effectiveDestroy(global bool) bool {
	if p != nil && p.destroyOnHiddenSet {
		return p.destroyOnHidden
	}
	return global
}

// Splitter is the splitter widget (docs/antd/splitter.md §6.10).
type Splitter struct {
	panels []*SplitterPanel

	orientation    Orientation
	orientationSet bool
	vertical       bool

	lazy              bool
	destroyOnHidden   bool
	collapsibleMotion bool

	width  float64
	height float64

	provider *theme.Provider
	override *theme.Tokens

	ariaLabel string

	innerSizes    []float64
	explicitSizes []float64
	hasExplicit   bool
	lastSizes     []float64
	lastW, lastH  float64

	collapsed    []bool
	savedSizes   map[int][]float64
	previewDelta float64
	dragging     int

	hovered    []bool
	focusedBar int
	barHovered bool
	barActive  int

	onResize      func([]float64)
	onResizeStart func([]float64)
	onResizeEnd   func([]float64)
	onCollapse    func([]bool, []float64)
	onDoubleClick func(int)

	classNames     map[SemanticKey]string
	semanticStyles map[SemanticKey]Style

	draggerIcon       rendering.RenderObject
	collapseStartIcon rendering.RenderObject
	collapseEndIcon   rendering.RenderObject

	resetOnDoubleClick bool

	root    *rendering.AbsoluteBox
	hosts   []*rendering.RenderBox
	bars    []*rendering.RenderBox
	preview *rendering.RenderColorBox
}

// NewSplitter creates a splitter hosting panels.
func NewSplitter(panels ...*SplitterPanel) *Splitter {
	s := &Splitter{dragging: -1, focusedBar: -1, barActive: -1, savedSizes: map[int][]float64{}}
	s.root = rendering.NewAbsoluteBox(0, 0)
	s.root.SetRepaintBoundary(true)
	s.root.SetRelayoutBoundary(true)
	s.preview = rendering.NewRenderColorBox(2, 2, 0, 0, 0, 0)
	if len(panels) > 0 {
		s.SetPanels(panels...)
	} else {
		s.ensureNodes()
	}
	return s
}

// SetPanels replaces panels.
func (s *Splitter) SetPanels(panels ...*SplitterPanel) {
	if s == nil {
		return
	}
	s.panels = append([]*SplitterPanel(nil), panels...)
	s.innerSizes = nil
	s.explicitSizes = nil
	s.hasExplicit = false
	s.lastSizes = nil
	s.collapsed = make([]bool, len(panels))
	s.hovered = make([]bool, max(0, len(panels)-1))
	s.dragging = -1
	s.focusedBar = -1
	s.barActive = -1
	s.previewDelta = 0
	// Drop stale hosts/bars so indices rebuild cleanly.
	for _, h := range s.hosts {
		if h != nil && s.root != nil {
			s.root.RemoveChild(h)
		}
	}
	for _, b := range s.bars {
		if b != nil && s.root != nil {
			s.root.RemoveChild(b)
		}
	}
	s.hosts = nil
	s.bars = nil
	s.rebuild()
}

// Panels returns the panel list.
func (s *Splitter) Panels() []*SplitterPanel {
	if s == nil {
		return nil
	}
	return s.panels
}

// SetOrientation sets layout direction (wins over SetVertical).
func (s *Splitter) SetOrientation(o Orientation) {
	if s == nil {
		return
	}
	s.orientation = o
	s.orientationSet = true
	s.markLayout()
}

// SetVertical sets legacy vertical flag (ignored when orientation set).
func (s *Splitter) SetVertical(b bool) {
	if s == nil {
		return
	}
	s.vertical = b
	s.markLayout()
}

// EffectiveOrientation resolves orientation (orientation wins over vertical).
func (s *Splitter) EffectiveOrientation() Orientation {
	if s != nil && s.orientationSet {
		return s.orientation
	}
	if s != nil && s.vertical {
		return Vertical
	}
	return Horizontal
}

// IsVertical reports vertical layout.
func (s *Splitter) IsVertical() bool { return s.EffectiveOrientation() == Vertical }

// SetLazy sets lazy preview mode.
func (s *Splitter) SetLazy(b bool) {
	if s == nil {
		return
	}
	s.lazy = b
	s.markPaint()
}

// Lazy reports lazy mode.
func (s *Splitter) Lazy() bool { return s != nil && s.lazy }

// SetDestroyOnHidden sets the global destroy flag.
func (s *Splitter) SetDestroyOnHidden(b bool) {
	if s == nil {
		return
	}
	s.destroyOnHidden = b
	s.markLayout()
}

// DestroyOnHidden reports the global flag.
func (s *Splitter) DestroyOnHidden() bool { return s != nil && s.destroyOnHidden }

// SetCollapsibleMotion sets fold animation intent (P0 instant).
func (s *Splitter) SetCollapsibleMotion(b bool) {
	if s == nil {
		return
	}
	s.collapsibleMotion = b
}

// CollapsibleMotion reports the motion flag.
func (s *Splitter) CollapsibleMotion() bool { return s != nil && s.collapsibleMotion }

// SetWidth sets fixed width (0 fills parent).
func (s *Splitter) SetWidth(w float64) {
	if s == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	s.width = w
	s.markLayout()
}

// SetHeight sets fixed height (0 fills parent).
func (s *Splitter) SetHeight(h float64) {
	if s == nil {
		return
	}
	if h < 0 {
		h = 0
	}
	s.height = h
	s.markLayout()
}

// Width returns the fixed width.
func (s *Splitter) Width() float64 {
	if s == nil {
		return 0
	}
	return s.width
}

// Height returns the fixed height.
func (s *Splitter) Height() float64 {
	if s == nil {
		return 0
	}
	return s.height
}

// SetProvider selects the theme source (nil selects process default).
func (s *Splitter) SetProvider(p *theme.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.markPaint()
}

// SetTheme pins exact tokens (nil clears to provider).
func (s *Splitter) SetTheme(t *theme.Tokens) {
	if s == nil {
		return
	}
	if t == nil {
		s.override = nil
	} else {
		cp := *t
		s.override = &cp
	}
	s.markPaint()
}

func (s *Splitter) themeTokens() theme.Tokens {
	if s != nil && s.override != nil {
		return *s.override
	}
	if s != nil && s.provider != nil {
		return s.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// SplitBarSize returns the visual bar width (token fallback 2).
func (s *Splitter) SplitBarSize() float64 { return DefaultSplitBarSize }

// SplitTriggerSize returns the hit/layout box (token fallback 6).
func (s *Splitter) SplitTriggerSize() float64 { return DefaultSplitTriggerSize }

// SplitBarDraggableSize returns the handle length (token fallback 20).
func (s *Splitter) SplitBarDraggableSize() float64 { return DefaultSplitBarDraggableSize }

// EffectiveBarColor returns the default track color (theme border).
// P1 semantic override wins when SetSemanticStyle(SemanticBar, UseBg) set.
func (s *Splitter) EffectiveBarColor() render.RGBA {
	if s != nil && s.semanticStyles != nil {
		if st, ok := s.semanticStyles[SemanticBar]; ok && st.UseBg {
			return st.Bg
		}
	}
	tok := s.themeTokens()
	if tok.ColorBorderSecondary.A > 0 {
		return themeToRGBA(tok.ColorBorderSecondary)
	}
	return themeToRGBA(tok.ColorBorder)
}

// EffectiveBarHoverColor returns hover fill (theme fill secondary).
func (s *Splitter) EffectiveBarHoverColor() render.RGBA {
	tok := s.themeTokens()
	return themeToRGBA(tok.ColorFillSecondary)
}

// EffectiveBarActiveColor returns active fill (theme fill).
func (s *Splitter) EffectiveBarActiveColor() render.RGBA {
	tok := s.themeTokens()
	return themeToRGBA(tok.ColorFill)
}

// EffectiveHandleColor returns the grip color (theme tertiary text).
// P1 semantic override wins when SetSemanticStyle(SemanticHandle, UseBg) set.
func (s *Splitter) EffectiveHandleColor() render.RGBA {
	if s != nil && s.semanticStyles != nil {
		if st, ok := s.semanticStyles[SemanticHandle]; ok && st.UseBg {
			return st.Bg
		}
	}
	tok := s.themeTokens()
	return themeToRGBA(tok.ColorTextTertiary)
}

// EffectivePreviewColor returns the lazy preview line (theme primary).
func (s *Splitter) EffectivePreviewColor() render.RGBA {
	tok := s.themeTokens()
	c := themeToRGBA(tok.ColorPrimary)
	return c
}

// EffectiveFocusColor returns the focus ring (theme primary).
func (s *Splitter) EffectiveFocusColor() render.RGBA {
	tok := s.themeTokens()
	return themeToRGBA(tok.ColorPrimary)
}

// SetAriaLabel sets the accessible name.
func (s *Splitter) SetAriaLabel(v string) {
	if s == nil {
		return
	}
	s.ariaLabel = v
}

// AriaLabel returns the accessible name.
func (s *Splitter) AriaLabel() string {
	if s == nil {
		return ""
	}
	return s.ariaLabel
}

// Role returns the accessible role.
func (s *Splitter) Role() string { return "separator" }

// BarRole returns the bar role (separator per bar).
func (s *Splitter) BarRole(_ int) string { return "separator" }

// BarAriaLabel returns the bar accessible name.
func (s *Splitter) BarAriaLabel(i int) string {
	if s != nil && s.ariaLabel != "" {
		return s.ariaLabel
	}
	return "splitter bar"
}

// Focusable reports whether bars take focus (always true when bars exist).
func (s *Splitter) Focusable() bool { return s != nil && s.BarCount() > 0 }

// BarFocusable reports per-bar focusability.
func (s *Splitter) BarFocusable(i int) bool {
	return s != nil && i >= 0 && i < s.BarCount()
}

// FocusBar focuses a bar; false on bad index.
func (s *Splitter) FocusBar(i int) bool {
	if !s.BarFocusable(i) {
		return false
	}
	s.focusedBar = i
	s.markPaint()
	return true
}

// BlurBar clears bar focus.
func (s *Splitter) BlurBar() {
	if s == nil || s.focusedBar < 0 {
		return
	}
	s.focusedBar = -1
	s.markPaint()
}

// BarFocused reports whether bar i holds focus.
func (s *Splitter) BarFocused(i int) bool { return s != nil && s.focusedBar == i }

// Focused reports whether any bar holds focus.
func (s *Splitter) Focused() bool { return s != nil && s.focusedBar >= 0 }

// FocusRingVisible reports whether the focus ring paints.
func (s *Splitter) FocusRingVisible(i int) bool { return s.BarFocused(i) }

// BarCount returns the bar number (panels-1, min 0).
func (s *Splitter) BarCount() int {
	if s == nil || len(s.panels) < 2 {
		return 0
	}
	return len(s.panels) - 1
}

// BarNode returns the bar render node for layout/hit assertions.
func (s *Splitter) BarNode(i int) rendering.RenderObject {
	if s == nil || i < 0 || i >= len(s.bars) {
		return nil
	}
	return s.bars[i]
}

// PanelHost returns the panel host node for layout assertions.
func (s *Splitter) PanelHost(i int) rendering.RenderObject {
	if s == nil || i < 0 || i >= len(s.hosts) {
		return nil
	}
	return s.hosts[i]
}

// IsControlled reports controlled path (any panel size set).
func (s *Splitter) IsControlled() bool {
	if s == nil {
		return false
	}
	for _, p := range s.panels {
		if p != nil && p.sizeSet {
			return true
		}
	}
	return false
}

// ContainerLength returns the last main-axis length (0 before layout).
func (s *Splitter) ContainerLength() float64 {
	if s == nil || len(s.lastSizes) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range s.lastSizes {
		sum += v
	}
	return sum
}

// PanelSizes returns current px sizes (copy; needs Layout first for truth).
func (s *Splitter) PanelSizes() []float64 {
	if s == nil {
		return nil
	}
	if len(s.lastSizes) > 0 {
		return append([]float64(nil), s.lastSizes...)
	}
	if len(s.innerSizes) > 0 {
		return append([]float64(nil), s.innerSizes...)
	}
	return nil
}

// SetPanelSizesPx writes back sizes (uncontrolled inner or controlled drive).
func (s *Splitter) SetPanelSizesPx(sizes []float64) {
	if s == nil || len(sizes) != len(s.panels) {
		return
	}
	cp := append([]float64(nil), sizes...)
	if s.IsControlled() {
		s.explicitSizes = cp
		s.hasExplicit = true
	} else {
		s.innerSizes = cp
	}
	s.lastSizes = append([]float64(nil), cp...)
	s.markLayout()
}

// OnResize sets the resize callback.
func (s *Splitter) OnResize(fn func([]float64)) {
	if s == nil {
		return
	}
	s.onResize = fn
}

// OnResizeStart sets the drag-start callback.
func (s *Splitter) OnResizeStart(fn func([]float64)) {
	if s == nil {
		return
	}
	s.onResizeStart = fn
}

// OnResizeEnd sets the drag-end callback.
func (s *Splitter) OnResizeEnd(fn func([]float64)) {
	if s == nil {
		return
	}
	s.onResizeEnd = fn
}

// OnCollapse sets the collapse callback.
func (s *Splitter) OnCollapse(fn func([]bool, []float64)) {
	if s == nil {
		return
	}
	s.onCollapse = fn
}

// OnDraggerDoubleClick sets the bar double-click callback (P1 hook).
func (s *Splitter) OnDraggerDoubleClick(fn func(int)) {
	if s == nil {
		return
	}
	s.onDoubleClick = fn
}

// SetClassName pins one semantic class hook (P1 classNames, paint-only).
func (s *Splitter) SetClassName(k SemanticKey, v string) {
	if s == nil {
		return
	}
	if s.classNames == nil {
		s.classNames = map[SemanticKey]string{}
	}
	if v == "" {
		delete(s.classNames, k)
		return
	}
	s.classNames[k] = v
}

// ClassName reads one semantic class hook.
func (s *Splitter) ClassName(k SemanticKey) string {
	if s == nil || s.classNames == nil {
		return ""
	}
	return s.classNames[k]
}

// SetClassNames replaces all semantic class hooks (nil clears).
func (s *Splitter) SetClassNames(m map[SemanticKey]string) {
	if s == nil {
		return
	}
	if len(m) == 0 {
		s.classNames = nil
		return
	}
	cp := make(map[SemanticKey]string, len(m))
	for k, v := range m {
		if v != "" {
			cp[k] = v
		}
	}
	s.classNames = cp
}

// ClassNames returns a copy of the semantic class hooks.
func (s *Splitter) ClassNames() map[SemanticKey]string {
	if s == nil || s.classNames == nil {
		return nil
	}
	cp := make(map[SemanticKey]string, len(s.classNames))
	for k, v := range s.classNames {
		cp[k] = v
	}
	return cp
}

// SetSemanticStyle pins one semantic inline-style hook (P1 styles).
func (s *Splitter) SetSemanticStyle(k SemanticKey, st Style) {
	if s == nil {
		return
	}
	if s.semanticStyles == nil {
		s.semanticStyles = map[SemanticKey]Style{}
	}
	s.semanticStyles[k] = st
	s.markPaint()
}

// SemanticStyle reads one semantic style hook.
func (s *Splitter) SemanticStyle(k SemanticKey) (Style, bool) {
	if s == nil || s.semanticStyles == nil {
		return Style{}, false
	}
	st, ok := s.semanticStyles[k]
	return st, ok
}

// ClearSemanticStyles removes all semantic style hooks.
func (s *Splitter) ClearSemanticStyles() {
	if s == nil {
		return
	}
	s.semanticStyles = nil
	s.markPaint()
}

// SetDraggerIcon stores a custom bar icon node (P1 customize hook).
func (s *Splitter) SetDraggerIcon(n rendering.RenderObject) {
	if s == nil {
		return
	}
	s.draggerIcon = n
	s.markPaint()
}

// DraggerIcon returns the custom bar icon node (nil when unset).
func (s *Splitter) DraggerIcon() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.draggerIcon
}

// SetCollapsibleIcons stores custom fold icons per side (P1 hook).
func (s *Splitter) SetCollapsibleIcons(start, end rendering.RenderObject) {
	if s == nil {
		return
	}
	s.collapseStartIcon = start
	s.collapseEndIcon = end
	s.markPaint()
}

// CollapsibleIcons returns the custom fold icons (nil when unset).
func (s *Splitter) CollapsibleIcons() (start, end rendering.RenderObject) {
	if s == nil {
		return nil, nil
	}
	return s.collapseStartIcon, s.collapseEndIcon
}

// SetResetOnDoubleClick enables reset.tsx behavior: double-click restores
// initial sizes after firing OnDraggerDoubleClick (P1, default false).
func (s *Splitter) SetResetOnDoubleClick(b bool) {
	if s == nil {
		return
	}
	s.resetOnDoubleClick = b
}

// ResetOnDoubleClick reports the double-click reset flag.
func (s *Splitter) ResetOnDoubleClick() bool { return s != nil && s.resetOnDoubleClick }

// ResetSizes restores initial sizes (clears drag/collapse deltas).
func (s *Splitter) ResetSizes() {
	if s == nil {
		return
	}
	s.innerSizes = nil
	s.explicitSizes = nil
	s.hasExplicit = false
	s.lastSizes = nil
	s.collapsed = make([]bool, len(s.panels))
	s.savedSizes = map[int][]float64{}
	s.previewDelta = 0
	s.dragging = -1
	s.barActive = -1
	s.markLayout()
}

// CollapsedStates returns per-panel collapsed flags.
func (s *Splitter) CollapsedStates() []bool {
	if s == nil {
		return nil
	}
	return append([]bool(nil), s.collapsed...)
}

// IsCollapsed reports panel collapsed state.
func (s *Splitter) IsCollapsed(i int) bool {
	if s == nil || i < 0 || i >= len(s.collapsed) {
		return false
	}
	return s.collapsed[i]
}

// IsContentDestroyed reports destroy-on-hidden outcome for panel i.
func (s *Splitter) IsContentDestroyed(i int) bool {
	if s == nil || i < 0 || i >= len(s.panels) {
		return false
	}
	sizes := s.PanelSizes()
	if len(sizes) != len(s.panels) {
		return false
	}
	if sizes[i] != 0 {
		return false
	}
	p := s.panels[i]
	if p == nil {
		return s.destroyOnHidden
	}
	return p.effectiveDestroy(s.destroyOnHidden)
}

// ContentVisible reports whether panel content paints.
func (s *Splitter) ContentVisible(i int) bool { return !s.IsContentDestroyed(i) }

// SetBarHover sets hover state (paint-only).
func (s *Splitter) SetBarHover(i int, b bool) {
	if s == nil || i < 0 || i >= len(s.hovered) {
		return
	}
	if s.hovered[i] == b {
		return
	}
	s.hovered[i] = b
	s.markPaint()
}

// BarHovered reports hover state.
func (s *Splitter) BarHovered(i int) bool {
	if s == nil || i < 0 || i >= len(s.hovered) {
		return false
	}
	return s.hovered[i]
}

// KeyboardStep returns arrow step (container 1% or 4px).
func (s *Splitter) KeyboardStep() float64 {
	L := s.ContainerLength()
	if L <= 0 {
		if s.lastW > 0 && !s.IsVertical() {
			L = s.lastW
		} else if s.lastH > 0 && s.IsVertical() {
			L = s.lastH
		} else {
			L = DefaultSplitContainerMain
		}
	}
	step := L * 0.01
	if step < DefaultSplitKeyboardStep {
		step = DefaultSplitKeyboardStep
	}
	return step
}

// HandleBarKey handles keyboard on a bar (arrows resize, Enter/Space collapse).
func (s *Splitter) HandleBarKey(bar int, key string) bool {
	if s == nil || bar < 0 || bar >= s.BarCount() {
		return false
	}
	switch key {
	case "Left", "Right", "Up", "Down":
		horizontal := !s.IsVertical()
		var delta float64
		step := s.KeyboardStep()
		if horizontal {
			if key == "Left" {
				delta = -step
			} else if key == "Right" {
				delta = step
			} else {
				return false
			}
		} else {
			if key == "Up" {
				delta = -step
			} else if key == "Down" {
				delta = step
			} else {
				return false
			}
		}
		return s.DragBar(bar, delta)
	case "Enter", "Space", " ":
		// Collapse the first collapsible side of this bar.
		n := len(s.panels)
		if bar < 0 || bar+1 >= n {
			return false
		}
		left := s.panels[bar]
		right := s.panels[bar+1]
		leftOK := left != nil && (left.collapsibleStart || left.collapsible)
		rightOK := right != nil && (right.collapsibleEnd || right.collapsible)
		// Prefer the non-collapsed collapsible side; else start.
		if leftOK && rightOK {
			if s.IsCollapsed(bar) {
				s.CollapseAt(bar, CollapseStart)
				return true
			}
			if s.IsCollapsed(bar + 1) {
				s.CollapseAt(bar, CollapseEnd)
				return true
			}
			s.CollapseAt(bar, CollapseStart)
			return true
		}
		if leftOK {
			s.CollapseAt(bar, CollapseStart)
			return true
		}
		if rightOK {
			s.CollapseAt(bar, CollapseEnd)
			return true
		}
		return false
	}
	return false
}

// KeyActivate handles Enter/Space on the focused bar.
func (s *Splitter) KeyActivate(key string) bool {
	if s == nil || s.focusedBar < 0 {
		return false
	}
	return s.HandleBarKey(s.focusedBar, key)
}

// DragBar drags bar i by delta px (single gesture: start/update/end).
func (s *Splitter) DragBar(bar int, delta float64) bool {
	if s == nil || !s.BeginDrag(bar) {
		return false
	}
	s.UpdateDrag(delta)
	s.EndDrag()
	return true
}

// BeginDrag starts a drag (emits OnResizeStart).
func (s *Splitter) BeginDrag(bar int) bool {
	if s == nil || bar < 0 || bar >= s.BarCount() {
		return false
	}
	if !s.barResizable(bar) {
		return false
	}
	s.dragging = bar
	s.barActive = bar
	s.previewDelta = 0
	s.dragStartSizesLocked()
	s.markPaint()
	if s.onResizeStart != nil {
		s.onResizeStart(s.snapshotSizes())
	}
	return true
}

// UpdateDrag previews/applies delta from drag start.
func (s *Splitter) UpdateDrag(delta float64) {
	if s == nil || s.dragging < 0 {
		return
	}
	bar := s.dragging
	start := s.dragStartSizesLocked()
	L := s.dragLengthLocked()
	clamped := clampDelta(s, bar, start, L, delta)
	if s.lazy {
		s.previewDelta = clamped
		s.markPaint()
		return
	}
	next := applyDelta(start, bar, clamped)
	if s.IsControlled() {
		if s.onResize != nil {
			s.onResize(append([]float64(nil), next...))
		}
		return
	}
	s.innerSizes = next
	s.lastSizes = append([]float64(nil), next...)
	if s.onResize != nil {
		s.onResize(s.snapshotSizes())
	}
	s.markLayout()
}

// EndDrag commits lazy preview and emits OnResizeEnd once.
func (s *Splitter) EndDrag() {
	if s == nil || s.dragging < 0 {
		return
	}
	bar := s.dragging
	var final []float64
	if s.lazy {
		start := s.dragStartSizesLocked()
		next := applyDelta(start, bar, s.previewDelta)
		if s.IsControlled() {
			if s.previewDelta != 0 && s.onResize != nil {
				s.onResize(append([]float64(nil), next...))
			}
			final = s.snapshotSizes()
			// Controlled lazy keeps old geometry; final for End payload
			// is the preview target so callers can回写.
			if s.previewDelta != 0 {
				final = append([]float64(nil), next...)
			}
		} else {
			if s.previewDelta != 0 {
				s.innerSizes = next
				s.lastSizes = append([]float64(nil), next...)
				if s.onResize != nil {
					s.onResize(s.snapshotSizes())
				}
				s.markLayout()
			}
			final = s.snapshotSizes()
		}
	} else {
		final = s.snapshotSizes()
	}
	s.dragging = -1
	s.barActive = -1
	s.previewDelta = 0
	s.markPaint()
	if s.onResizeEnd != nil {
		s.onResizeEnd(append([]float64(nil), final...))
	}
}

// DraggingBar returns the active drag bar (-1 idle).
func (s *Splitter) DraggingBar() int {
	if s == nil {
		return -1
	}
	return s.dragging
}

// PreviewDelta returns the lazy preview offset.
func (s *Splitter) PreviewDelta() float64 {
	if s == nil {
		return 0
	}
	return s.previewDelta
}

// DoubleClickBar fires the double-click hook (P1).
// When ResetOnDoubleClick is set, initial sizes are restored afterwards.
func (s *Splitter) DoubleClickBar(bar int) {
	if s == nil || bar < 0 || bar >= s.BarCount() {
		return
	}
	if s.onDoubleClick != nil {
		s.onDoubleClick(bar)
	}
	if s.resetOnDoubleClick {
		s.ResetSizes()
	}
}

// CollapseAt toggles collapse/expand of one side of a bar.
func (s *Splitter) CollapseAt(bar int, side CollapseSide) {
	if s == nil || bar < 0 || bar >= s.BarCount() {
		return
	}
	n := len(s.panels)
	if bar+1 >= n {
		return
	}
	var target int
	if side == CollapseEnd {
		target = bar + 1
	} else {
		target = bar
	}
	p := s.panels[target]
	if p == nil || !p.collapsible {
		// Check side-specific flags when general flag unset.
		allowed := false
		if p != nil {
			if target == bar && p.collapsibleStart {
				allowed = true
			}
			if target == bar+1 && p.collapsibleEnd {
				allowed = true
			}
		}
		if !allowed {
			return
		}
	}
	cur := s.snapshotSizes()
	L := s.dragLengthLocked()
	if len(cur) != n {
		return
	}
	if s.collapsed[target] {
		s.expandLocked(bar, target, cur, L)
		return
	}
	s.collapseLocked(bar, target, cur, L)
}

func (s *Splitter) collapseLocked(bar, target int, cur []float64, L float64) {
	neighbor := bar
	if target == bar {
		neighbor = bar + 1
	} else {
		neighbor = bar
	}
	saved := append([]float64(nil), cur...)
	s.savedSizes[target] = saved
	amount := cur[target]
	next := append([]float64(nil), cur...)
	next[target] = 0
	next[neighbor] = cur[neighbor] + amount
	// Clamp neighbor max (keep collapsed at 0).
	if n := len(s.panels); neighbor >= 0 && neighbor < n {
		mx := resolveMax(s.panels[neighbor], L)
		if mx >= 0 && next[neighbor] > mx {
			overflow := next[neighbor] - mx
			next[neighbor] = mx
			// Keep sum: collapsed stays 0, overflow dropped (container shrink edge).
			_ = overflow
		}
	}
	if s.IsControlled() {
		s.collapsed[target] = true
		if s.onCollapse != nil {
			s.onCollapse(append([]bool(nil), s.collapsed...), append([]float64(nil), next...))
		}
		return
	}
	s.collapsed[target] = true
	s.innerSizes = next
	s.lastSizes = append([]float64(nil), next...)
	s.markLayout()
	if s.onCollapse != nil {
		s.onCollapse(append([]bool(nil), s.collapsed...), s.snapshotSizes())
	}
}

func (s *Splitter) expandLocked(bar, target int, cur []float64, L float64) {
	saved, ok := s.savedSizes[target]
	want := 0.0
	if ok && target < len(saved) {
		want = saved[target]
	} else {
		// Fallback: even split remainder.
		want = L / float64(len(s.panels))
	}
	neighbor := bar
	if target == bar {
		neighbor = bar + 1
	} else {
		neighbor = bar
	}
	// Clamp restore to min/max and neighbor availability.
	mn := resolveMin(s.panels[target], L)
	mx := resolveMax(s.panels[target], L)
	if mn >= 0 && want < mn {
		want = mn
	}
	if mx >= 0 && want > mx {
		want = mx
	}
	if want > cur[neighbor] {
		want = cur[neighbor]
	}
	if want < 0 {
		want = 0
	}
	next := append([]float64(nil), cur...)
	next[target] = want
	next[neighbor] = cur[neighbor] - want + cur[target]
	if next[neighbor] < 0 {
		next[neighbor] = 0
	}
	delete(s.savedSizes, target)
	if s.IsControlled() {
		nextCollapsed := append([]bool(nil), s.collapsed...)
		nextCollapsed[target] = false
		s.collapsed[target] = false
		if s.onCollapse != nil {
			s.onCollapse(append([]bool(nil), nextCollapsed...), append([]float64(nil), next...))
		}
		return
	}
	s.collapsed[target] = false
	s.innerSizes = next
	s.lastSizes = append([]float64(nil), next...)
	s.markLayout()
	if s.onCollapse != nil {
		s.onCollapse(append([]bool(nil), s.collapsed...), s.snapshotSizes())
	}
}

func (s *Splitter) barResizable(bar int) bool {
	if s == nil || bar < 0 || bar+1 >= len(s.panels) {
		return false
	}
	a, b := s.panels[bar], s.panels[bar+1]
	if a != nil && !a.resizable {
		return false
	}
	if b != nil && !b.resizable {
		return false
	}
	return true
}

// BarResizable reports whether bar i can be dragged.
func (s *Splitter) BarResizable(bar int) bool { return s.barResizable(bar) }

func (s *Splitter) dragStartSizesLocked() []float64 {
	snap := s.snapshotSizes()
	return snap
}

func (s *Splitter) dragLengthLocked() float64 {
	if len(s.lastSizes) > 0 {
		sum := 0.0
		for _, v := range s.lastSizes {
			sum += v
		}
		if sum > 0 {
			return sum
		}
	}
	if s.lastW > 0 && !s.IsVertical() {
		return s.lastW
	}
	if s.lastH > 0 && s.IsVertical() {
		return s.lastH
	}
	if s.width > 0 && !s.IsVertical() {
		return s.width
	}
	if s.height > 0 && s.IsVertical() {
		return s.height
	}
	return DefaultSplitContainerMain
}

func (s *Splitter) snapshotSizes() []float64 {
	if len(s.lastSizes) > 0 {
		return append([]float64(nil), s.lastSizes...)
	}
	if len(s.innerSizes) > 0 {
		return append([]float64(nil), s.innerSizes...)
	}
	return nil
}

func clampDelta(s *Splitter, bar int, cur []float64, L float64, delta float64) float64 {
	n := len(cur)
	if bar < 0 || bar+1 >= n {
		return 0
	}
	left, right := cur[bar], cur[bar+1]
	minL := resolveMin(s.panels[bar], L)
	maxL := resolveMax(s.panels[bar], L)
	minR := resolveMin(s.panels[bar+1], L)
	maxR := resolveMax(s.panels[bar+1], L)
	lo, hi := -1e9, 1e9
	if minL >= 0 {
		if minL-left > lo {
			lo = minL - left
		}
	}
	if maxR >= 0 {
		if right-maxR > lo {
			lo = right - maxR
		}
	}
	if maxL >= 0 {
		if maxL-left < hi {
			hi = maxL - left
		}
	}
	if minR >= 0 {
		if right-minR < hi {
			hi = right - minR
		}
	}
	// Keep sizes non-negative even without explicit min.
	if -left > lo {
		lo = -left
	}
	if right < hi {
		hi = right
	}
	if lo > hi {
		return 0
	}
	if delta < lo {
		return lo
	}
	if delta > hi {
		return hi
	}
	return delta
}

func applyDelta(cur []float64, bar int, delta float64) []float64 {
	next := append([]float64(nil), cur...)
	next[bar] += delta
	next[bar+1] -= delta
	if next[bar] < 0 {
		next[bar] = 0
	}
	if next[bar+1] < 0 {
		next[bar+1] = 0
	}
	return next
}

func resolveMin(p *SplitterPanel, L float64) float64 {
	if p == nil || !p.min.IsSet() {
		return -1
	}
	return p.min.Resolve(L)
}

func resolveMax(p *SplitterPanel, L float64) float64 {
	if p == nil || !p.max.IsSet() {
		return -1
	}
	return p.max.Resolve(L)
}

func (s *Splitter) resolveSizes(L float64) []float64 {
	n := len(s.panels)
	if n == 0 {
		return nil
	}
	if s.IsControlled() {
		if s.hasExplicit && len(s.explicitSizes) == n {
			out := append([]float64(nil), s.explicitSizes...)
			clampAll(s, out, L)
			return out
		}
		out := make([]float64, n)
		fixed := 0.0
		auto := 0
		for i, p := range s.panels {
			if p != nil && p.sizeSet {
				v := p.size.Resolve(L)
				if v < 0 {
					v = 0
				}
				out[i] = v
				fixed += v
			} else {
				auto++
			}
		}
		rest := L - fixed
		if rest < 0 {
			rest = 0
		}
		each := 0.0
		if auto > 0 {
			each = rest / float64(auto)
		}
		for i, p := range s.panels {
			if p == nil || !p.sizeSet {
				out[i] = each
			}
		}
		clampAll(s, out, L)
		fixSum(out, L)
		return out
	}
	if len(s.innerSizes) == n {
		out := append([]float64(nil), s.innerSizes...)
		clampAll(s, out, L)
		return out
	}
	out := make([]float64, n)
	fixed := 0.0
	auto := 0
	for i, p := range s.panels {
		if p != nil && p.defaultSize.IsSet() {
			v := p.defaultSize.Resolve(L)
			if v < 0 {
				v = 0
			}
			out[i] = v
			fixed += v
		} else {
			auto++
		}
	}
	rest := L - fixed
	if rest < 0 {
		rest = 0
	}
	each := 0.0
	if auto > 0 {
		each = rest / float64(auto)
	}
	for i, p := range s.panels {
		if p == nil || !p.defaultSize.IsSet() {
			out[i] = each
		}
	}
	clampAll(s, out, L)
	fixSum(out, L)
	s.innerSizes = append([]float64(nil), out...)
	return out
}

func clampAll(s *Splitter, out []float64, L float64) {
	for i, p := range s.panels {
		if p == nil {
			continue
		}
		if mn := resolveMin(p, L); mn >= 0 && out[i] < mn {
			// Collapsed panels stay 0 even with min (explicit user fold wins).
			if !(s.collapsed[i] && out[i] == 0) {
				out[i] = mn
			}
		}
		if mx := resolveMax(p, L); mx >= 0 && out[i] > mx {
			out[i] = mx
		}
		if out[i] < 0 {
			out[i] = 0
		}
	}
}

func fixSum(out []float64, L float64) {
	sum := 0.0
	for _, v := range out {
		sum += v
	}
	diff := L - sum
	if len(out) == 0 || (diff < 0.0001 && diff > -0.0001) {
		return
	}
	out[len(out)-1] += diff
	if out[len(out)-1] < 0 {
		out[len(out)-1] = 0
	}
}

// Node returns the tree node (layout/paint/hit through it).
func (s *Splitter) Node() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.root
}

// ChromeNode returns the decorated chrome node.
func (s *Splitter) ChromeNode() rendering.RenderObject { return s.Node() }

// Layout sizes the node under constraints.
func (s *Splitter) Layout(c rendering.Constraints) rendering.Size {
	if s == nil || s.root == nil {
		return rendering.Size{}
	}
	s.syncBarsLocked()
	s.ensureNodes()
	s.layoutRoot(c)
	return s.root.Layout(c)
}

func (s *Splitter) rebuild() {
	if s == nil || s.root == nil {
		return
	}
	s.syncBarsLocked()
	s.ensureNodes()
	s.root.MarkNeedsLayout()
}

func (s *Splitter) markLayout() {
	if s == nil || s.root == nil {
		return
	}
	s.syncBarsLocked()
	s.ensureNodes()
	s.root.MarkNeedsLayout()
}

func (s *Splitter) markPaint() {
	if s == nil || s.root == nil {
		return
	}
	s.root.MarkNeedsPaint()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
