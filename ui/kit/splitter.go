package kit

import (
	"math"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Splitter defaults — components/splitter/style prepareComponentToken.
// Product contract: docs/antd/splitter.md §6 (P0 DoD).
// https://ant.design/components/splitter
const (
	TypeSplitter      = "kit.Splitter"
	TypeSplitterPanel = "kit.SplitterPanel"
	TypeSplitterBar   = "kit.SplitterBar"

	// DefaultSplitBarSize is antd splitBarSize (visual rail).
	DefaultSplitBarSize = 2.0
	// DefaultSplitTriggerSize is antd splitTriggerSize (hit box ≥ visual).
	DefaultSplitTriggerSize = 6.0
	// DefaultSplitBarDraggableSize is antd splitBarDraggableSize (spinner mark).
	DefaultSplitBarDraggableSize = 20.0
	// DefaultSplitterFontSize / radius / lineWidth — §6.2 global seeds.
	DefaultSplitterFontSize     = 14.0
	DefaultSplitterBorderRadius = 6.0
	DefaultSplitterLineWidth    = 1.0
	// DefaultSplitterKeyStepPx is keyboard nudge when container unknown.
	DefaultSplitterKeyStepPx = 4.0
)

// SplitterBarStyle customizes the dragger chrome (default / hover / active).
// Zero colors fall back to Theme tokens; zero sizes fall back to DefaultSplit*.
//
//	default → Color / TokenColorBgTextHover
//	hover   → HoverColor / TokenColorBgTextActive
//	active  → ActiveColor / TokenColorBgTextActive (stronger)
//	spinner → SpinnerColor / TokenColorFillSecondary
//	variant → solid (fill rail) | dashed (stroke rail)
type SplitterBarStyle struct {
	Color        render.RGBA
	HoverColor   render.RGBA
	ActiveColor  render.RGBA
	SpinnerColor render.RGBA
	// Size is visual rail thickness (antd splitBarSize, default 2).
	Size float64
	// TriggerSize is hit box (antd splitTriggerSize, default 6; ≥ Size).
	TriggerSize float64
	// Variant: solid fill (default) or dashed stroke along the seam.
	Variant SplitterBarVariant
	// Dash is the stroke dash pattern when Variant=Dashed; empty → [4,4].
	Dash []float64
}

// SplitterBarVariant is the visual style of the split rail.
type SplitterBarVariant int

const (
	// SplitterBarSolid fills a thin rect (antd default).
	SplitterBarSolid SplitterBarVariant = iota
	// SplitterBarDashed strokes a dashed line along the seam.
	SplitterBarDashed
)

// SplitterOrientation is antd orientation: horizontal | vertical.
type SplitterOrientation int

const (
	SplitterHorizontal SplitterOrientation = iota
	SplitterVertical
)

// CollapseSide is which panel a bar collapses toward.
type CollapseSide int

const (
	CollapseStart CollapseSide = iota // toward previous / first side of the bar
	CollapseEnd
)

// CollapsibleIconMode is panel collapsible.showCollapsibleIcon: auto | true | false.
type CollapsibleIconMode int

const (
	CollapsibleIconAuto CollapsibleIconMode = iota
	CollapsibleIconAlways
	CollapsibleIconNever
)

// SplitterDim is antd number | 'xx%' panel size unit.
type SplitterDim struct {
	Px        float64
	Percent   float64 // 0..100 when IsPercent
	IsPercent bool
	Valid     bool
}

// DimPx builds a pixel dimension.
func DimPx(px float64) SplitterDim {
	return SplitterDim{Px: px, Valid: true}
}

// DimPercent builds a percentage dimension (0..100, e.g. 40 → 40%).
func DimPercent(pct float64) SplitterDim {
	return SplitterDim{Percent: pct, IsPercent: true, Valid: true}
}

// ParseDim parses "40%" or a numeric string as px.
func ParseDim(s string) (SplitterDim, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return SplitterDim{}, nil
	}
	if strings.HasSuffix(s, "%") {
		n, err := strconv.ParseFloat(strings.TrimSpace(s[:len(s)-1]), 64)
		if err != nil {
			return SplitterDim{}, err
		}
		return DimPercent(n), nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return SplitterDim{}, err
	}
	return DimPx(n), nil
}

func (d SplitterDim) toPx(container float64) float64 {
	if !d.Valid {
		return 0
	}
	if d.IsPercent {
		return d.Percent / 100 * container
	}
	return d.Px
}

func (d SplitterDim) toPtg(container float64) float64 {
	if !d.Valid {
		return 0
	}
	if d.IsPercent {
		return d.Percent / 100
	}
	if container <= 0 {
		return 0
	}
	return d.Px / container
}

// SplitterPanel is antd Splitter.Panel configuration + child.
type SplitterPanel struct {
	Child core.Node

	Min, Max, Size, DefaultSize             SplitterDim
	minSet, maxSet, sizeSet, defaultSizeSet bool

	Resizable bool
	// collapsible
	CollapsibleStart, CollapsibleEnd bool
	ShowIcon                         CollapsibleIconMode
	// DestroyOnHidden overrides Splitter when set.
	DestroyOnHidden    bool
	destroyOnHiddenSet bool

	// collapsed cache restored on expand (legacy per-panel; prefer Splitter.collapseCache).
	cachedCollapsePx float64
}

// NewSplitterPanel creates a panel wrapping child (may be nil).
func NewSplitterPanel(child core.Node) *SplitterPanel {
	return &SplitterPanel{
		Child:     child,
		Resizable: true,
		ShowIcon:  CollapsibleIconAuto,
	}
}

// SetChild sets panel content.
func (p *SplitterPanel) SetChild(n core.Node) {
	if p == nil {
		return
	}
	p.Child = n
}

// SetMin / SetMax / SetSize / SetDefaultSize — antd min/max/size/defaultSize.
func (p *SplitterPanel) SetMin(d SplitterDim) {
	if p == nil {
		return
	}
	p.Min = d
	p.minSet = d.Valid
}
func (p *SplitterPanel) SetMax(d SplitterDim) {
	if p == nil {
		return
	}
	p.Max = d
	p.maxSet = d.Valid
}
func (p *SplitterPanel) SetSize(d SplitterDim) {
	if p == nil {
		return
	}
	p.Size = d
	p.sizeSet = d.Valid
}
func (p *SplitterPanel) SetDefaultSize(d SplitterDim) {
	if p == nil {
		return
	}
	p.DefaultSize = d
	p.defaultSizeSet = d.Valid
}

// SetMinPx / SetMaxPx / SetSizePx / SetDefaultSizePx convenience.
func (p *SplitterPanel) SetMinPx(px float64)         { p.SetMin(DimPx(px)) }
func (p *SplitterPanel) SetMaxPx(px float64)         { p.SetMax(DimPx(px)) }
func (p *SplitterPanel) SetSizePx(px float64)        { p.SetSize(DimPx(px)) }
func (p *SplitterPanel) SetDefaultSizePx(px float64) { p.SetDefaultSize(DimPx(px)) }
func (p *SplitterPanel) SetMinPercent(pct float64)   { p.SetMin(DimPercent(pct)) }
func (p *SplitterPanel) SetMaxPercent(pct float64)   { p.SetMax(DimPercent(pct)) }
func (p *SplitterPanel) SetSizePercent(pct float64)  { p.SetSize(DimPercent(pct)) }
func (p *SplitterPanel) SetDefaultSizePercent(pct float64) {
	p.SetDefaultSize(DimPercent(pct))
}

// SetResizable toggles drag on the adjacent bar (antd resizable, default true).
func (p *SplitterPanel) SetResizable(v bool) {
	if p == nil {
		return
	}
	p.Resizable = v
}

// SetCollapsible enables both-side collapsible (antd collapsible=true).
func (p *SplitterPanel) SetCollapsible(v bool) {
	if p == nil {
		return
	}
	p.CollapsibleStart = v
	p.CollapsibleEnd = v
}

// SetCollapsibleSides sets start/end independently.
func (p *SplitterPanel) SetCollapsibleSides(start, end bool) {
	if p == nil {
		return
	}
	p.CollapsibleStart = start
	p.CollapsibleEnd = end
}

// SetShowCollapsibleIcon sets icon visibility mode.
func (p *SplitterPanel) SetShowCollapsibleIcon(m CollapsibleIconMode) {
	if p == nil {
		return
	}
	p.ShowIcon = m
}

// SetDestroyOnHidden overrides root destroyOnHidden for this panel.
func (p *SplitterPanel) SetDestroyOnHidden(v bool) {
	if p == nil {
		return
	}
	p.DestroyOnHidden = v
	p.destroyOnHiddenSet = true
}

// Splitter is Ant Design Splitter: multi-panel resizable layout.
//
//	splitterRoot
//	  ├─ panelHost[i]
//	  └─ bar[i] (between panels)
//
// hit == layout == paint. Product: docs/antd/splitter.md §6.
type Splitter struct {
	Root *splitterRoot

	Orientation    SplitterOrientation
	orientationSet bool
	// Vertical is antd vertical sugar; ignored when Orientation was Set.
	Vertical bool
	Lazy     bool
	// DestroyOnHidden applies to all panels unless panel overrides.
	DestroyOnHidden bool
	// CollapsibleMotion: P0 instantaneous; reserved for P1 animation.
	CollapsibleMotion bool

	// Preferred box (0 → fill parent max when bounded).
	Width, Height float64

	// Theme / a11y
	Theme     *core.Theme
	AriaLabel string

	// BarStyle customizes dragger default/hover/active colors and sizes.
	BarStyle SplitterBarStyle

	// Callbacks (sizes in px)
	OnResizeStart func(sizes []float64)
	OnResize      func(sizes []float64)
	OnResizeEnd   func(sizes []float64)
	OnCollapse    func(collapsed []bool, sizes []float64)

	panels []*SplitterPanel

	// innerSizes holds non-controlled sizes (same units as antd: px or %).
	// When any panel has sizeSet, controlled path uses panel.Size.
	innerDims []SplitterDim

	// lastPx is the resolved px sizes after layout (len == panels).
	lastPx []float64
	// lastContainer is main-axis length used for last resolution.
	lastContainer float64
	// collapseCache is per-bar collapsed size (antd cacheCollapsedSizeRef).
	collapseCache []float64

	// drag state
	dragging      bool
	dragBar       int
	dragCachePx   []float64
	lazyPreviewPx float64 // constrained offset for lazy preview line
	lazyOffset    float64

	// keyboard focus bar index (-1 none)
	focusBar int
}

// NewSplitter creates a Splitter with optional panels.
// Breaking: previously NewSplitter(first, second core.Node) with Ratio API.
func NewSplitter(panels ...*SplitterPanel) *Splitter {
	s := &Splitter{
		Orientation: SplitterHorizontal,
		focusBar:    -1,
	}
	s.Root = &splitterRoot{owner: s}
	s.Root.Init(s.Root)
	s.Root.Hit = core.HitDefer
	s.SetPanels(panels...)
	return s
}

// NewSplitterNodes is a convenience: wrap each node as a default panel.
// Prefer NewSplitter + NewSplitterPanel for size/min/max.
func NewSplitterNodes(nodes ...core.Node) *Splitter {
	ps := make([]*SplitterPanel, 0, len(nodes))
	for _, n := range nodes {
		ps = append(ps, NewSplitterPanel(n))
	}
	return NewSplitter(ps...)
}

// Node returns the stable mount root.
func (s *Splitter) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		s.Root = &splitterRoot{owner: s}
		s.Root.Init(s.Root)
		s.Root.Hit = core.HitDefer
		s.rebuild()
	}
	return s.Root
}

// ChromeNode returns the layout root (visual tests).
func (s *Splitter) ChromeNode() core.Node { return s.Node() }

// SetPanels replaces panels and rebuilds chrome.
func (s *Splitter) SetPanels(panels ...*SplitterPanel) {
	if s == nil {
		return
	}
	s.panels = make([]*SplitterPanel, 0, len(panels))
	for _, p := range panels {
		if p != nil {
			s.panels = append(s.panels, p)
		}
	}
	// seed inner dims from defaultSize
	s.innerDims = make([]SplitterDim, len(s.panels))
	for i, p := range s.panels {
		if p.defaultSizeSet {
			s.innerDims[i] = p.DefaultSize
		}
	}
	s.lastPx = make([]float64, len(s.panels))
	s.rebuild()
}

// Panels returns the current panel slice (mutable; call rebuild via setters).
func (s *Splitter) Panels() []*SplitterPanel {
	if s == nil {
		return nil
	}
	return s.panels
}

// SetOrientation sets horizontal/vertical. Takes priority over Vertical sugar.
func (s *Splitter) SetOrientation(o SplitterOrientation) {
	if s == nil {
		return
	}
	s.Orientation = o
	s.orientationSet = true
	s.rebuild()
}

// SetVertical is antd vertical sugar; ignored when Orientation was Set.
func (s *Splitter) SetVertical(v bool) {
	if s == nil {
		return
	}
	s.Vertical = v
	if !s.orientationSet {
		if v {
			s.Orientation = SplitterVertical
		} else {
			s.Orientation = SplitterHorizontal
		}
	}
	s.rebuild()
}

// EffectiveOrientation resolves orientation with Vertical sugar.
func (s *Splitter) EffectiveOrientation() SplitterOrientation {
	if s == nil {
		return SplitterHorizontal
	}
	if s.orientationSet {
		return s.Orientation
	}
	if s.Vertical {
		return SplitterVertical
	}
	return s.Orientation
}

// IsVertical reports effective vertical layout.
func (s *Splitter) IsVertical() bool {
	return s.EffectiveOrientation() == SplitterVertical
}

// SetLazy enables lazy resize (preview only until pointer up).
func (s *Splitter) SetLazy(v bool) {
	if s == nil {
		return
	}
	s.Lazy = v
}

// SetDestroyOnHidden sets root-level destroyOnHidden.
func (s *Splitter) SetDestroyOnHidden(v bool) {
	if s == nil {
		return
	}
	s.DestroyOnHidden = v
	s.rebuild()
}

// SetCollapsibleMotion stores motion flag (P0: no animation).
func (s *Splitter) SetCollapsibleMotion(v bool) {
	if s == nil {
		return
	}
	s.CollapsibleMotion = v
}

// SetWidth / SetHeight optional preferred size.
func (s *Splitter) SetWidth(w float64) {
	if s == nil {
		return
	}
	s.Width = w
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
	}
}
func (s *Splitter) SetHeight(h float64) {
	if s == nil {
		return
	}
	s.Height = h
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
	}
}

// SetTheme stores product theme.
func (s *Splitter) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

// SetAriaLabel sets optional accessible name on root.
func (s *Splitter) SetAriaLabel(label string) {
	if s == nil {
		return
	}
	s.AriaLabel = label
	s.applyA11y()
}

// SetBarStyle sets dragger chrome (default / hover / active colors + sizes).
// Zero color channels (A==0) keep Theme token fallbacks; Size/TriggerSize 0 → defaults.
func (s *Splitter) SetBarStyle(st SplitterBarStyle) {
	if s == nil {
		return
	}
	s.BarStyle = st
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
		s.Root.MarkNeedsPaint()
	}
}

// PanelSizes returns last resolved px sizes (after Layout).
func (s *Splitter) PanelSizes() []float64 {
	if s == nil {
		return nil
	}
	out := make([]float64, len(s.lastPx))
	copy(out, s.lastPx)
	return out
}

// SetPanelSizesPx writes absolute px sizes into the non-controlled inner state
// (or, when controlled, only updates lastPx + fires OnResize if caller wants).
// Sizes are normalized to sum to lastContainer when container known.
func (s *Splitter) SetPanelSizesPx(sizes []float64) {
	if s == nil || len(sizes) == 0 {
		return
	}
	n := len(s.panels)
	if n == 0 {
		return
	}
	px := make([]float64, n)
	for i := 0; i < n && i < len(sizes); i++ {
		px[i] = sizes[i]
	}
	container := s.lastContainer
	if container <= 0 {
		var sum float64
		for _, v := range px {
			sum += v
		}
		container = sum
	}
	if container > 0 {
		// store as percent dims so resize of container preserves proportions
		s.innerDims = make([]SplitterDim, n)
		for i := 0; i < n; i++ {
			s.innerDims[i] = DimPercent(px[i] / container * 100)
		}
	} else {
		s.innerDims = make([]SplitterDim, n)
		for i := 0; i < n; i++ {
			s.innerDims[i] = DimPx(px[i])
		}
	}
	// clear controlled size flags so inner path takes over (callers who want
	// controlled must re-SetSize on panels themselves after OnResize).
	// Do not clear sizeSet here — controlled demos keep size via OnResize.
	s.lastPx = px
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
	}
}

// CollapseAt folds/expands the panel on the given side of barIndex (0..n-2).
// Ports antd useResize.onCollapse:
//
//	start → current=prev, target=next
//	end   → current=next, target=prev
//
// Both non-zero → collapse current into target (cache size on the bar).
// Else → expand using bar cache onto target (the zero-size panel).
func (s *Splitter) CollapseAt(barIndex int, side CollapseSide) {
	if s == nil || barIndex < 0 || barIndex >= len(s.panels)-1 {
		return
	}
	s.ensurePx()
	px := append([]float64(nil), s.lastPx...)
	if len(px) < 2 {
		return
	}
	curIdx := barIndex
	tgtIdx := barIndex + 1
	if side == CollapseEnd {
		curIdx = barIndex + 1
		tgtIdx = barIndex
	}
	cur := px[curIdx]
	tgt := px[tgtIdx]

	// per-bar collapse cache (antd cacheCollapsedSizeRef.current[barIndex])
	if len(s.collapseCache) != len(s.panels)-1 {
		s.collapseCache = make([]float64, len(s.panels)-1)
	}

	if cur > 1e-6 && tgt > 1e-6 {
		// fold current into target
		s.collapseCache[barIndex] = cur
		px[tgtIdx] += cur
		px[curIdx] = 0
	} else {
		// expand: restore collapsed panel (target when the expandable button is used)
		total := cur + tgt
		cache := s.collapseCache[barIndex]
		minC := s.limitPx(curIdx, true)
		maxC := s.limitPx(curIdx, false)
		minT := s.limitPx(tgtIdx, true)
		maxT := s.limitPx(tgtIdx, false)

		// antd shouldUseCache on target
		curCache := total - cache
		useCache := cache > 1e-6 &&
			cache <= maxT+1e-6 && cache >= minT-1e-6 &&
			curCache <= maxC+1e-6 && curCache >= minC-1e-6

		if useCache {
			px[tgtIdx] = cache
			px[curIdx] = curCache
		} else {
			// half of free range (antd halfOffset)
			limitStart := minC
			if total-maxT > limitStart {
				limitStart = total - maxT
			}
			limitEnd := maxC
			if total-minT < limitEnd {
				limitEnd = total - minT
			}
			half := minT
			if half <= 0 {
				half = (limitEnd - limitStart) / 2
			}
			if half < 0 {
				half = 0
			}
			// move half from current (the non-zero side) to target
			if cur > 1e-6 {
				// current has size, target is 0
				if half > cur {
					half = cur
				}
				px[curIdx] = cur - half
				px[tgtIdx] = tgt + half
			} else {
				// current is 0, target has size — move half to current
				if half > tgt {
					half = tgt
				}
				px[curIdx] = cur + half
				px[tgtIdx] = tgt - half
			}
		}
	}
	s.applySizes(px, true)
	if s.OnCollapse != nil {
		collapsed := make([]bool, len(px))
		for i, v := range px {
			collapsed[i] = math.Abs(v) < 1e-6
		}
		s.OnCollapse(collapsed, append([]float64(nil), px...))
	}
	if s.OnResize != nil {
		s.OnResize(append([]float64(nil), px...))
	}
	if s.OnResizeEnd != nil {
		s.OnResizeEnd(append([]float64(nil), px...))
	}
}

// ---- internals ----

func (s *Splitter) theme() *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	return core.ThemeOrDefault(nil)
}

func (s *Splitter) applyA11y() {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.Base().Label = s.AriaLabel
}

func (s *Splitter) barSize() float64 {
	if s != nil && s.BarStyle.Size > 0 {
		return s.BarStyle.Size
	}
	return DefaultSplitBarSize
}

func (s *Splitter) triggerSize() float64 {
	if s != nil && s.BarStyle.TriggerSize > 0 {
		t := s.BarStyle.TriggerSize
		if t < s.barSize() {
			t = s.barSize()
		}
		return t
	}
	return DefaultSplitTriggerSize
}

func (s *Splitter) spinnerSize() float64 {
	return DefaultSplitBarDraggableSize
}

func (s *Splitter) isControlled() bool {
	for _, p := range s.panels {
		if p != nil && p.sizeSet {
			return true
		}
	}
	return false
}

func (s *Splitter) sizeSource() []SplitterDim {
	n := len(s.panels)
	out := make([]SplitterDim, n)
	if s.isControlled() {
		for i, p := range s.panels {
			if p.sizeSet {
				out[i] = p.Size
			}
			// else leave invalid → auto fill
		}
		return out
	}
	// non-controlled: inner (seeded from defaultSize)
	if len(s.innerDims) != n {
		s.innerDims = make([]SplitterDim, n)
		for i, p := range s.panels {
			if p.defaultSizeSet {
				s.innerDims[i] = p.DefaultSize
			}
		}
	}
	copy(out, s.innerDims)
	return out
}

func (s *Splitter) minPtg(i int, container float64) float64 {
	if i < 0 || i >= len(s.panels) || container <= 0 {
		return 0
	}
	p := s.panels[i]
	if p == nil || !p.minSet {
		return 0
	}
	return p.Min.toPtg(container)
}

func (s *Splitter) maxPtg(i int, container float64) float64 {
	if i < 0 || i >= len(s.panels) || container <= 0 {
		return 1
	}
	p := s.panels[i]
	if p == nil || !p.maxSet {
		return 1
	}
	return p.Max.toPtg(container)
}

func (s *Splitter) limitPx(i int, isMin bool) float64 {
	container := s.lastContainer
	if i < 0 || i >= len(s.panels) {
		if isMin {
			return 0
		}
		return container
	}
	p := s.panels[i]
	if p == nil {
		if isMin {
			return 0
		}
		return container
	}
	if isMin {
		if !p.minSet {
			return 0
		}
		return p.Min.toPx(container)
	}
	if !p.maxSet {
		return container
	}
	return p.Max.toPx(container)
}

// resolvePx converts size source → px via autoPtgSizes (antd sizeUtil).
func (s *Splitter) resolvePx(container float64) []float64 {
	n := len(s.panels)
	if n == 0 {
		return nil
	}
	if container < 0 {
		container = 0
	}
	src := s.sizeSource()
	ptg := make([]*float64, n)
	minP := make([]float64, n)
	maxP := make([]float64, n)
	for i := 0; i < n; i++ {
		minP[i] = s.minPtg(i, container)
		maxP[i] = s.maxPtg(i, container)
		if maxP[i] < minP[i] {
			maxP[i] = minP[i]
		}
		if src[i].Valid {
			v := src[i].toPtg(container)
			ptg[i] = &v
		}
	}
	outPtg := autoPtgSizes(ptg, minP, maxP)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = outPtg[i] * container
	}
	return out
}

// autoPtgSizes ports antd sizeUtil.autoPtgSizes.
func autoPtgSizes(ptgSizes []*float64, minPtg, maxPtg []float64) []float64 {
	n := len(ptgSizes)
	result := make([]float64, n)
	var currentTotal float64
	undef := make([]int, 0, n)
	for i, p := range ptgSizes {
		if p == nil {
			undef = append(undef, i)
		} else {
			currentTotal += *p
			result[i] = *p
		}
	}
	rest := 1 - currentTotal
	undefCount := len(undef)

	// all defined but sum != 1 → scale
	if n > 0 && undefCount == 0 && math.Abs(currentTotal-1) > 1e-9 {
		if currentTotal == 0 {
			avg := 1 / float64(n)
			for i := range result {
				result[i] = avg
			}
			return result
		}
		scale := 1 / currentTotal
		for i := range result {
			result[i] *= scale
		}
		return result
	}
	if rest < 0 {
		scale := 1 / currentTotal
		for i, p := range ptgSizes {
			if p == nil {
				result[i] = 0
			} else {
				result[i] = *p * scale
			}
		}
		return result
	}
	if undefCount == 0 {
		return result
	}
	var sumMin, sumMax float64
	limitMin, limitMax := 0.0, 1.0
	for _, index := range undef {
		min := minPtg[index]
		max := maxPtg[index]
		if max <= 0 {
			max = 1
		}
		sumMin += min
		sumMax += max
		if min > limitMin {
			limitMin = min
		}
		if max < limitMax {
			limitMax = max
		}
	}
	if sumMin > 1 && sumMax < 1 {
		avg := 1 / float64(undefCount)
		for _, index := range undef {
			result[index] = avg
		}
		return result
	}
	restAvg := rest / float64(undefCount)
	if limitMin <= restAvg && restAvg <= limitMax {
		for _, index := range undef {
			result[index] = restAvg
		}
		return result
	}
	// greedy
	remain := rest - sumMin
	for _, index := range undef {
		min := minPtg[index]
		max := maxPtg[index]
		if max <= 0 {
			max = 1
		}
		result[index] = min
		canAdd := max - min
		add := canAdd
		if add > remain {
			add = remain
		}
		if add < 0 {
			add = 0
		}
		result[index] += add
		remain -= add
	}
	return result
}

func (s *Splitter) ensurePx() {
	if len(s.lastPx) != len(s.panels) {
		s.lastPx = s.resolvePx(s.lastContainer)
	}
}

func (s *Splitter) applySizes(px []float64, writeInner bool) {
	if s == nil {
		return
	}
	n := len(s.panels)
	if len(px) != n {
		return
	}
	s.lastPx = append([]float64(nil), px...)
	if writeInner && !s.isControlled() {
		container := s.lastContainer
		s.innerDims = make([]SplitterDim, n)
		if container > 0 {
			for i := 0; i < n; i++ {
				s.innerDims[i] = DimPercent(px[i] / container * 100)
			}
		} else {
			for i := 0; i < n; i++ {
				s.innerDims[i] = DimPx(px[i])
			}
		}
	}
	// controlled: caller is expected to SetSize on panels from OnResize
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
	}
}

// barResizeInfo ports antd useResizable for one seam (bar between panels[i] and [i+1]).
// See components/splitter/hooks/useResizable.ts.
type barResizeInfo struct {
	Resizable     bool
	StartCollapse bool // show start-side control (Left/Up)
	EndCollapse   bool // show end-side control (Right/Down)
	ShowStartIcon CollapsibleIconMode
	ShowEndIcon   CollapsibleIconMode
}

func (s *Splitter) barResizable(barIndex int) bool {
	return s.barInfo(barIndex).Resizable
}

func (s *Splitter) barStartCollapsible(barIndex int) bool {
	return s.barInfo(barIndex).StartCollapse
}

// BarStartCollapsible reports whether the start-side collapse control is active (tests / debug).
func (s *Splitter) BarStartCollapsible(barIndex int) bool {
	return s.barStartCollapsible(barIndex)
}

func (s *Splitter) barEndCollapsible(barIndex int) bool {
	return s.barInfo(barIndex).EndCollapse
}

// BarEndCollapsible reports whether the end-side collapse control is active (tests / debug).
func (s *Splitter) BarEndCollapsible(barIndex int) bool {
	return s.barEndCollapsible(barIndex)
}

// barInfo computes per-bar resizable + collapsible icon presence from current sizes.
// Official rules (horizontal LTR):
//
//	startCollapsible = (prev.end && prevSize>0) || (next.start && nextSize==0 && prevSize>0)
//	endCollapsible   = (next.start && nextSize>0) || (prev.end && prevSize==0 && nextSize>0)
//
// So with both open + both collapsible → both arrows; after folding one side,
// only the opposite arrow remains (expand that side). Arrow glyph is fixed
// (start=Left/Up, end=Right/Down) — direction does not flip; presence does.
func (s *Splitter) barInfo(barIndex int) barResizeInfo {
	out := barResizeInfo{
		ShowStartIcon: CollapsibleIconNever,
		ShowEndIcon:   CollapsibleIconNever,
	}
	if s == nil || barIndex < 0 || barIndex >= len(s.panels)-1 {
		return out
	}
	prev := s.panels[barIndex]
	next := s.panels[barIndex+1]
	if prev == nil || next == nil {
		return out
	}
	s.ensurePx()
	var prevSize, nextSize float64
	if barIndex < len(s.lastPx) {
		prevSize = s.lastPx[barIndex]
	}
	if barIndex+1 < len(s.lastPx) {
		nextSize = s.lastPx[barIndex+1]
	}

	// resizable: both allow + collapsed+min edge case from antd
	prevMin := prev.minSet
	nextMin := next.minSet
	out.Resizable = prev.Resizable && next.Resizable &&
		(prevSize != 0 || !prevMin) &&
		(nextSize != 0 || !nextMin)

	// collapsible presence (size-aware)
	prevEndCollapsible := prev.CollapsibleEnd && prevSize > 1e-6
	nextStartExpandable := next.CollapsibleStart && nextSize <= 1e-6 && prevSize > 1e-6
	startCollapsible := prevEndCollapsible || nextStartExpandable

	nextStartCollapsible := next.CollapsibleStart && nextSize > 1e-6
	prevEndExpandable := prev.CollapsibleEnd && prevSize <= 1e-6 && nextSize > 1e-6
	endCollapsible := nextStartCollapsible || prevEndExpandable

	out.StartCollapse = startCollapsible
	out.EndCollapse = endCollapsible

	// showCollapsibleIcon merge (antd getShowCollapsibleIcon)
	out.ShowStartIcon = mergeShowCollapsibleIcon(
		iconOpt{collapsible: prevEndCollapsible, mode: prev.ShowIcon},
		iconOpt{collapsible: nextStartExpandable, mode: next.ShowIcon},
	)
	out.ShowEndIcon = mergeShowCollapsibleIcon(
		iconOpt{collapsible: nextStartCollapsible, mode: next.ShowIcon},
		iconOpt{collapsible: prevEndExpandable, mode: prev.ShowIcon},
	)
	return out
}

type iconOpt struct {
	collapsible bool
	mode        CollapsibleIconMode
}

// mergeShowCollapsibleIcon ports antd getShowCollapsibleIcon.
func mergeShowCollapsibleIcon(prev, next iconOpt) CollapsibleIconMode {
	if prev.collapsible && next.collapsible {
		if prev.mode == CollapsibleIconAlways || next.mode == CollapsibleIconAlways {
			return CollapsibleIconAlways
		}
		if prev.mode == CollapsibleIconAuto || next.mode == CollapsibleIconAuto {
			return CollapsibleIconAuto
		}
		return CollapsibleIconNever
	}
	if prev.collapsible {
		return prev.mode
	}
	if next.collapsible {
		return next.mode
	}
	return CollapsibleIconNever
}

// rebuild rebuilds panel hosts + bars as children of Root.
func (s *Splitter) rebuild() {
	if s == nil {
		return
	}
	if s.Root == nil {
		s.Root = &splitterRoot{owner: s}
		s.Root.Init(s.Root)
		s.Root.Hit = core.HitDefer
	}
	s.Root.ClearChildren()
	n := len(s.panels)
	for i := 0; i < n; i++ {
		ph := &splitterPanelHost{owner: s, index: i}
		ph.Init(ph)
		ph.Hit = core.HitDefer
		s.Root.AddChild(ph)
		if i < n-1 {
			bar := s.newBar(i)
			s.Root.AddChild(bar)
		}
	}
	s.applyA11y()
	s.Root.MarkNeedsLayout()
}

func (s *Splitter) newBar(index int) *splitterBar {
	b := &splitterBar{owner: s, index: index}
	b.Init(b)
	b.Hit = core.HitTarget
	b.Cursor = core.CursorMove
	// Stack above panels so overflow collapse arrows are not covered by the next
	// panel sibling (core.PaintOrder — general z-order, not TypeID special-case).
	b.PaintOrder = 1

	// visual rail
	rail := primitive.NewBox()
	rail.Color = s.railColor(0)
	// spinner mark (antd ::after — thin center grip, not a black block)
	spin := primitive.NewBox()
	spin.Color = s.spinnerColor()

	// collapse buttons: arrow glyphs, hover-only by default (antd showCollapsibleIcon=auto)
	startBtn := newSplitterCollapseBtn(s, index, CollapseStart)
	endBtn := newSplitterCollapseBtn(s, index, CollapseEnd)

	b.rail = rail
	b.spin = spin
	b.startBtn = startBtn
	b.endBtn = endBtn
	b.AddChild(rail)
	b.AddChild(spin)
	b.AddChild(startBtn)
	b.AddChild(endBtn)
	return b
}

func themeColorOr(th *core.Theme, key string, fb render.RGBA) render.RGBA {
	if th == nil {
		return fb
	}
	c := th.Color(key)
	if c.A == 0 && c.R == 0 && c.G == 0 && c.B == 0 {
		return fb
	}
	return c
}

// railState: 0 default, 1 hover, 2 active (dragging).
func (s *Splitter) railColor(state int) render.RGBA {
	th := s.theme()
	switch state {
	case 2: // active
		if s != nil && s.BarStyle.ActiveColor.A > 0 {
			return s.BarStyle.ActiveColor
		}
		return themeColorOr(th, core.TokenColorBgTextActive, render.RGBA{R: 0, G: 0, B: 0, A: 0.15})
	case 1: // hover
		if s != nil && s.BarStyle.HoverColor.A > 0 {
			return s.BarStyle.HoverColor
		}
		return themeColorOr(th, core.TokenColorBgTextActive, render.RGBA{R: 0, G: 0, B: 0, A: 0.15})
	default:
		if s != nil && s.BarStyle.Color.A > 0 {
			return s.BarStyle.Color
		}
		return themeColorOr(th, core.TokenColorBgTextHover, render.RGBA{R: 0, G: 0, B: 0, A: 0.06})
	}
}

func (s *Splitter) spinnerColor() render.RGBA {
	if s != nil && s.BarStyle.SpinnerColor.A > 0 {
		return s.BarStyle.SpinnerColor
	}
	th := s.theme()
	return themeColorOr(th, core.TokenColorFillSecondary, render.RGBA{R: 0, G: 0, B: 0, A: 0.15})
}

func (s *Splitter) primaryColor() render.RGBA {
	if s != nil && s.BarStyle.ActiveColor.A > 0 {
		c := s.BarStyle.ActiveColor
		if c.A > 0.35 {
			c.A = 0.2
		}
		return c
	}
	th := s.theme()
	c := themeColorOr(th, core.TokenColorPrimary, render.Hex("#1677ff"))
	c.A = 0.2
	return c
}

// ---- drag logic ----

func (s *Splitter) onBarDragStart(barIndex int) {
	s.ensurePx()
	s.dragging = true
	s.dragBar = barIndex
	s.dragCachePx = append([]float64(nil), s.lastPx...)
	s.lazyOffset = 0
	s.lazyPreviewPx = 0
	if s.OnResizeStart != nil {
		s.OnResizeStart(append([]float64(nil), s.lastPx...))
	}
}

func (s *Splitter) onBarDrag(barIndex int, offset float64) {
	if !s.dragging || len(s.dragCachePx) == 0 {
		return
	}
	if !s.barResizable(barIndex) {
		return
	}
	px, used := s.offsetSizes(barIndex, offset, s.dragCachePx)
	s.lazyOffset = used
	s.lazyPreviewPx = used
	if s.Lazy {
		if s.Root != nil {
			s.Root.MarkNeedsPaint()
		}
		return
	}
	s.applySizes(px, true)
	if s.OnResize != nil {
		s.OnResize(append([]float64(nil), px...))
	}
}

func (s *Splitter) onBarDragEnd(barIndex int) {
	if !s.dragging {
		return
	}
	s.dragging = false
	if s.Lazy {
		px, _ := s.offsetSizes(barIndex, s.lazyOffset, s.dragCachePx)
		s.applySizes(px, true)
		if s.OnResize != nil {
			s.OnResize(append([]float64(nil), px...))
		}
		if s.OnResizeEnd != nil {
			s.OnResizeEnd(append([]float64(nil), px...))
		}
	} else {
		if s.OnResizeEnd != nil {
			s.OnResizeEnd(append([]float64(nil), s.lastPx...))
		}
	}
	s.lazyOffset = 0
	s.lazyPreviewPx = 0
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

// offsetSizes applies drag offset to cache, clamping min/max. Returns new sizes and used offset.
func (s *Splitter) offsetSizes(index int, offset float64, cache []float64) ([]float64, float64) {
	n := len(cache)
	if index < 0 || index+1 >= n {
		return append([]float64(nil), cache...), 0
	}
	num := append([]float64(nil), cache...)
	startMin := s.limitPx(index, true)
	endMin := s.limitPx(index+1, true)
	startMax := s.limitPx(index, false)
	endMax := s.limitPx(index+1, false)
	container := s.lastContainer

	merged := offset
	if num[index]+merged < startMin {
		merged = startMin - num[index]
	}
	if num[index+1]-merged < endMin {
		merged = num[index+1] - endMin
	}
	if num[index]+merged > startMax {
		merged = startMax - num[index]
	}
	if num[index+1]-merged > endMax {
		merged = num[index+1] - endMax
	}
	// keep non-negative
	if num[index]+merged < 0 {
		merged = -num[index]
	}
	if num[index+1]-merged < 0 {
		merged = num[index+1]
	}
	_ = container
	num[index] += merged
	num[index+1] -= merged
	return num, merged
}

func (s *Splitter) keyboardNudge(barIndex int, dir float64) {
	if !s.barResizable(barIndex) {
		return
	}
	s.ensurePx()
	step := DefaultSplitterKeyStepPx
	if s.lastContainer > 0 {
		pct := s.lastContainer * 0.01
		if pct > step {
			step = pct
		}
	}
	cache := append([]float64(nil), s.lastPx...)
	px, _ := s.offsetSizes(barIndex, dir*step, cache)
	if s.OnResizeStart != nil {
		s.OnResizeStart(append([]float64(nil), s.lastPx...))
	}
	s.applySizes(px, true)
	if s.OnResize != nil {
		s.OnResize(append([]float64(nil), px...))
	}
	if s.OnResizeEnd != nil {
		s.OnResizeEnd(append([]float64(nil), px...))
	}
}

// ---- node types ----

type splitterRoot struct {
	core.NodeBase
	owner *Splitter
}

func (r *splitterRoot) TypeID() string { return TypeSplitter }

func (r *splitterRoot) Layout(c core.Constraints) core.Size {
	if r == nil || r.owner == nil {
		out := c.Tighten(core.Size{})
		if r != nil {
			r.SetSize(out)
		}
		return out
	}
	s := r.owner
	// resolve outer size
	w, h := s.Width, s.Height
	if w <= 0 {
		if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded {
			w = c.MaxWidth
		} else {
			w = 400
		}
	}
	if h <= 0 {
		if c.HasBoundedHeight() && c.MaxHeight < core.Unbounded {
			h = c.MaxHeight
		} else {
			h = 200
		}
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	r.SetSize(out)

	vert := s.IsVertical()
	container := out.Width
	if vert {
		container = out.Height
	}
	s.lastContainer = container
	px := s.resolvePx(container)
	// if dragging non-lazy, prefer lastPx (already updated)
	if s.dragging && !s.Lazy && len(s.lastPx) == len(px) {
		px = s.lastPx
	} else if !s.dragging {
		s.lastPx = px
	} else if s.Lazy && len(s.lastPx) == len(px) {
		// keep frozen geometry while lazy-dragging
		px = s.lastPx
	} else {
		s.lastPx = px
	}

	// place children: panel, bar, panel, bar, ...
	// Bars sit centered on seams; panels take full px sizes (sum == container).
	// Visual bar does not steal main-axis flex basis (antd width:0 + absolute dragger).
	kids := r.Children()
	n := len(s.panels)
	trigger := s.triggerSize()
	halfT := trigger / 2
	cursor := 0.0
	ki := 0
	for i := 0; i < n; i++ {
		if ki >= len(kids) {
			break
		}
		panel := kids[ki]
		ki++
		ps := px[i]
		if ps < 0 {
			ps = 0
		}
		if vert {
			_ = panel.Layout(core.Tight(out.Width, ps))
			panel.Base().SetOffset(core.Point{X: 0, Y: cursor})
		} else {
			_ = panel.Layout(core.Tight(ps, out.Height))
			panel.Base().SetOffset(core.Point{X: cursor, Y: 0})
		}
		cursor += ps

		if i < n-1 {
			if ki >= len(kids) {
				break
			}
			bar := kids[ki]
			ki++
			// center bar on seam at `cursor`
			if vert {
				_ = bar.Layout(core.Tight(out.Width, trigger))
				bar.Base().SetOffset(core.Point{X: 0, Y: cursor - halfT})
			} else {
				_ = bar.Layout(core.Tight(trigger, out.Height))
				bar.Base().SetOffset(core.Point{X: cursor - halfT, Y: 0})
			}
		}
	}
	r.RememberConstraints(c)
	return out
}

func (r *splitterRoot) Paint(pc *core.PaintContext) {
	if r == nil {
		return
	}
	// Children use core.PaintOrder so chrome (bars) stack above panels generically.
	r.DefaultPaintChildren(pc)
	// lazy preview line above everything
	s := r.owner
	if s != nil && s.dragging && s.Lazy && pc != nil {
		vert := s.IsVertical()
		bar := s.dragBar
		seam := 0.0
		for i := 0; i <= bar && i < len(s.lastPx); i++ {
			seam += s.lastPx[i]
		}
		seam += s.lazyPreviewPx
		sz := r.Size()
		barW := s.barSize()
		col := s.primaryColor()
		if vert {
			pc.FillLocalRect(0, seam-barW/2, sz.Width, barW, col)
		} else {
			pc.FillLocalRect(seam-barW/2, 0, barW, sz.Height, col)
		}
	}
	if pc != nil {
		r.ClearPaintDirty()
	}
}

func (r *splitterRoot) HitTest(p core.Point) core.Node {
	// PaintOrder puts bars above panels; DefaultHitTest matches that z-order.
	// Bars still expand hit into overflow collapse arrows via their own HitTest.
	if r == nil {
		return nil
	}
	return r.DefaultHitTest(p)
}

// splitterPanelHost hosts one panel child.
type splitterPanelHost struct {
	core.NodeBase
	owner *Splitter
	index int
}

func (h *splitterPanelHost) TypeID() string { return TypeSplitterPanel }

func (h *splitterPanelHost) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: c.MaxWidth, Height: c.MaxHeight})
	if !c.HasBoundedWidth() {
		out.Width = 0
	}
	if !c.HasBoundedHeight() {
		out.Height = 0
	}
	h.SetSize(out)
	if h.owner == nil || h.index < 0 || h.index >= len(h.owner.panels) {
		h.ClearChildren()
		return out
	}
	p := h.owner.panels[h.index]
	if p == nil {
		h.ClearChildren()
		return out
	}
	sz := 0.0
	if h.index < len(h.owner.lastPx) {
		sz = h.owner.lastPx[h.index]
	}
	destroy := h.owner.DestroyOnHidden
	if p.destroyOnHiddenSet {
		destroy = p.DestroyOnHidden
	}
	if sz <= 1e-6 && destroy {
		h.ClearChildren()
		return out
	}
	if p.Child != nil {
		kids := h.Children()
		if len(kids) != 1 || kids[0] != p.Child {
			h.ClearChildren()
			h.AddChild(p.Child)
		}
		_ = p.Child.Layout(core.Tight(out.Width, out.Height))
		p.Child.Base().SetOffset(core.Point{})
	} else {
		h.ClearChildren()
	}
	return out
}

func (h *splitterPanelHost) Paint(pc *core.PaintContext) {
	if h == nil {
		return
	}
	// clip overflow like antd overflow:auto (we just paint children; no scroll P0)
	h.DefaultPaintChildren(pc)
	if pc != nil {
		h.ClearPaintDirty()
	}
}

func (h *splitterPanelHost) HitTest(p core.Point) core.Node {
	if h == nil {
		return nil
	}
	// zero-size panels don't hit
	if h.Size().Width <= 0 || h.Size().Height <= 0 {
		return nil
	}
	return h.DefaultHitTest(p)
}

// splitterBar is the draggable seam between two panels.
type splitterBar struct {
	core.NodeBase
	owner    *Splitter
	index    int
	rail     *primitive.Box
	spin     *primitive.Box
	startBtn *splitterCollapseBtn
	endBtn   *splitterCollapseBtn

	// drag tracking
	active       bool
	startX       float64
	startY       float64
	hovered      bool
	focused      bool
	focusVisible bool

	// rail paint cache for dashed variant
	railCol render.RGBA
	dashed  bool
}

// splitterCollapseBtn is antd collapse-bar: soft hover chrome + Left/Right/Up/Down chevron.
// Layout stays when collapsible (so hover handoff bar↔btn works); paint respects auto/always/never.
type splitterCollapseBtn struct {
	core.NodeBase
	owner    *Splitter
	bar      int
	side     CollapseSide
	w, h     float64
	layoutOn bool // collapsible && mode != never — keeps hit box
	paintOn  bool // auto: bar/btn hover or focus; always: true
	arrow    collapseArrow
	hovered  bool
}

type collapseArrow int

const (
	arrowLeft collapseArrow = iota
	arrowRight
	arrowUp
	arrowDown
)

func newSplitterCollapseBtn(s *Splitter, bar int, side CollapseSide) *splitterCollapseBtn {
	b := &splitterCollapseBtn{owner: s, bar: bar, side: side}
	b.Init(b)
	b.Hit = core.HitTarget
	b.Cursor = core.CursorPointer
	return b
}

func (b *splitterCollapseBtn) TypeID() string { return "kit.SplitterCollapseBtn" }

func (b *splitterCollapseBtn) Layout(c core.Constraints) core.Size {
	out := core.Size{Width: b.w, Height: b.h}
	if !b.layoutOn {
		out = core.Size{}
	}
	out = c.Tighten(out)
	b.SetSize(out)
	return out
}

func (b *splitterCollapseBtn) Paint(pc *core.PaintContext) {
	if b == nil || !b.layoutOn || !b.paintOn || pc == nil || b.owner == nil {
		if pc != nil {
			b.ClearPaintDirty()
		}
		return
	}
	sz := b.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		b.ClearPaintDirty()
		return
	}
	s := b.owner
	th := s.theme()
	// soft hover bg (antd controlItemBgHover) — never solid black fill as the glyph
	bg := themeColorOr(th, core.TokenColorBgTextHover, render.RGBA{R: 0, G: 0, B: 0, A: 0.06})
	if b.hovered {
		bg = themeColorOr(th, core.TokenColorBgTextActive, render.RGBA{R: 0, G: 0, B: 0, A: 0.15})
	}
	r := th.SizeOr(core.TokenBorderRadiusSM, 4)
	if r > 4 {
		r = 4 // antd borderRadiusXS-ish
	}
	pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, r, bg)

	// chevron stroke in text color (antd Left/Right/Up/DownOutlined)
	col := themeColorOr(th, core.TokenColorText, render.RGBA{R: 0, G: 0, B: 0, A: 0.88})
	lw := 1.6
	padX := sz.Width * 0.28
	padY := sz.Height * 0.28
	cx, cy := sz.Width/2, sz.Height/2
	switch b.arrow {
	case arrowLeft:
		pc.StrokeLocalPolyline([]float64{
			sz.Width - padX, padY,
			padX, cy,
			sz.Width - padX, sz.Height - padY,
		}, lw, col)
	case arrowRight:
		pc.StrokeLocalPolyline([]float64{
			padX, padY,
			sz.Width - padX, cy,
			padX, sz.Height - padY,
		}, lw, col)
	case arrowUp:
		pc.StrokeLocalPolyline([]float64{
			padX, sz.Height - padY,
			cx, padY,
			sz.Width - padX, sz.Height - padY,
		}, lw, col)
	case arrowDown:
		pc.StrokeLocalPolyline([]float64{
			padX, padY,
			cx, sz.Height - padY,
			sz.Width - padX, padY,
		}, lw, col)
	}
	b.ClearPaintDirty()
}

func (b *splitterCollapseBtn) HitTest(p core.Point) core.Node {
	if b == nil || !b.layoutOn {
		return nil
	}
	// auto + not paintOn: still hit so hover can reveal (opacity 0 → 1)
	if b.LocalBounds().Contains(p) {
		return b
	}
	return nil
}

func (b *splitterCollapseBtn) SetHovered(v bool) {
	if b == nil {
		return
	}
	if b.hovered == v {
		return
	}
	b.hovered = v
	b.MarkNeedsPaint()
	// Tree only hover-notifies the leaf hit. Parent bar must track enter/leave
	// of its collapse arrows so auto icons hide when the pointer leaves the bar.
	if p, ok := b.Parent().(*splitterBar); ok {
		if v {
			p.hovered = true
		} else {
			// leaving this arrow — clear bar hover unless the other arrow is hovered
			other := p.startBtn
			if b == p.startBtn {
				other = p.endBtn
			}
			if other == nil || !other.hovered {
				p.hovered = false
			}
		}
		p.syncCollapsePaint()
		p.MarkNeedsPaint()
	}
}

func (b *splitterCollapseBtn) HandlePointer(ev *core.PointerEvent) {
	if b == nil || b.owner == nil || ev == nil || !b.layoutOn {
		return
	}
	// Only interact when painted (auto: after hover reveal). Swallow down so the
	// parent bar does not start a drag; actual toggle is via OnClick (once).
	switch ev.Type {
	case core.PointerDown:
		if ev.Button == core.ButtonLeft && b.paintOn {
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		if ev.Button == core.ButtonLeft {
			// do NOT CollapseAt here — tree also synthesizes OnClick; double-toggle
			// would cancel the fold. Just stop bubble.
			ev.Handled = true
		}
	}
}

func (b *splitterCollapseBtn) OnClick(ev *core.PointerEvent) {
	if b == nil || b.owner == nil || !b.layoutOn || !b.paintOn {
		return
	}
	b.owner.CollapseAt(b.bar, b.side)
	if ev != nil {
		ev.Handled = true
	}
}

func (b *splitterBar) TypeID() string { return TypeSplitterBar }

func (b *splitterBar) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: c.MaxWidth, Height: c.MaxHeight})
	b.SetSize(out)
	if b.owner == nil {
		return out
	}
	s := b.owner
	vert := s.IsVertical()
	barVis := s.barSize()
	spinLen := s.spinnerSize()
	resizable := s.barResizable(b.index)
	active := s.dragging && s.dragBar == b.index

	// rail: visual bar centered; state colors via BarStyle / Theme
	if b.rail != nil {
		railState := 0
		if active {
			railState = 2
		} else if b.hovered {
			railState = 1
		}
		col := s.railColor(railState)
		if !resizable {
			col = s.railColor(0)
		}
		dashed := s != nil && s.BarStyle.Variant == SplitterBarDashed
		if dashed {
			// dashed: rail box is transparent; stroke drawn in Paint
			b.rail.Color = render.RGBA{}
			b.rail.Width, b.rail.Height = 0, 0
			_ = b.rail.Layout(core.Tight(0, 0))
		} else if vert {
			b.rail.Width = out.Width
			b.rail.Height = barVis
			b.rail.Color = col
			_ = b.rail.Layout(core.Tight(out.Width, barVis))
			b.rail.SetOffset(core.Point{X: 0, Y: (out.Height - barVis) / 2})
		} else {
			b.rail.Width = barVis
			b.rail.Height = out.Height
			b.rail.Color = col
			_ = b.rail.Layout(core.Tight(barVis, out.Height))
			b.rail.SetOffset(core.Point{X: (out.Width - barVis) / 2, Y: 0})
		}
		b.railCol = col
		b.dashed = dashed
	}
	// spinner mark
	if b.spin != nil {
		showSpin := resizable
		if showSpin {
			if vert {
				b.spin.Width = spinLen
				b.spin.Height = barVis
				_ = b.spin.Layout(core.Tight(spinLen, barVis))
				b.spin.SetOffset(core.Point{X: (out.Width - spinLen) / 2, Y: (out.Height - barVis) / 2})
			} else {
				b.spin.Width = barVis
				b.spin.Height = spinLen
				_ = b.spin.Layout(core.Tight(barVis, spinLen))
				b.spin.SetOffset(core.Point{X: (out.Width - barVis) / 2, Y: (out.Height - spinLen) / 2})
			}
			b.spin.Color = s.spinnerColor()
		} else {
			b.spin.Width, b.spin.Height = 0, 0
			_ = b.spin.Layout(core.Tight(0, 0))
		}
	}

	// collapse buttons — antd useResizable size-aware presence + fixed arrow direction.
	// start = Left/Up, end = Right/Down (glyphs never flip; which side is shown flips).
	th := s.theme()
	btnMain := th.SizeOr(core.TokenFontSizeSM, 12)       // along thin axis
	btnCross := th.SizeOr(core.TokenControlHeightSM, 24) // along bar length
	info := s.barInfo(b.index)
	placeBtn := func(btn *splitterCollapseBtn, start bool) {
		if btn == nil {
			return
		}
		collapsible := info.EndCollapse
		mode := info.ShowEndIcon
		if start {
			collapsible = info.StartCollapse
			mode = info.ShowStartIcon
		}
		layoutOn := collapsible && mode != CollapsibleIconNever
		paintOn := false
		if layoutOn {
			switch mode {
			case CollapsibleIconAlways:
				paintOn = true
			case CollapsibleIconAuto:
				// hover-only (antd collapse-bar-hover-only).
				// Use focusVisible (keyboard), not mouse focused — pointer click
				// focuses the bar and would otherwise pin icons open after click.
				paintOn = b.hovered || b.focusVisible || b.active ||
					(b.startBtn != nil && b.startBtn.hovered) ||
					(b.endBtn != nil && b.endBtn.hovered)
			}
		}
		btn.layoutOn = layoutOn
		btn.paintOn = paintOn
		if !layoutOn {
			btn.w, btn.h = 0, 0
			_ = btn.Layout(core.Tight(0, 0))
			btn.SetOffset(core.Point{})
			return
		}
		var bw, bh float64
		if vert {
			bw, bh = btnCross, btnMain
		} else {
			bw, bh = btnMain, btnCross
		}
		btn.w, btn.h = bw, bh
		// Fixed glyph per side (antd SplitBar startIcon/endIcon) — does NOT reverse
		// when expanding a collapsed panel; the *other* button appears instead.
		if vert {
			if start {
				btn.arrow = arrowUp
			} else {
				btn.arrow = arrowDown
			}
		} else {
			if start {
				btn.arrow = arrowLeft
			} else {
				btn.arrow = arrowRight
			}
		}
		_ = btn.Layout(core.Tight(bw, bh))
		halfT := out.Width / 2
		if vert {
			halfT = out.Height / 2
		}
		if vert {
			x := (out.Width - bw) / 2
			var y float64
			if start {
				y = halfT - bh - 2
			} else {
				y = halfT + 2
			}
			btn.SetOffset(core.Point{X: x, Y: y})
		} else {
			y := (out.Height - bh) / 2
			var x float64
			if start {
				x = halfT - bw - 2
			} else {
				x = halfT + 2
			}
			btn.SetOffset(core.Point{X: x, Y: y})
		}
	}
	placeBtn(b.startBtn, true)
	placeBtn(b.endBtn, false)

	// cursor: no col/row-resize in core.CursorKind; Move signals drag.
	if resizable {
		b.Cursor = core.CursorMove
	} else {
		b.Cursor = core.CursorDefault
	}
	return out
}

// syncCollapsePaint updates paintOn without full geometry recompute.
// Visibility (layoutOn) still needs Layout after size changes / collapse.
func (b *splitterBar) syncCollapsePaint() {
	if b == nil || b.owner == nil {
		return
	}
	info := b.owner.barInfo(b.index)
	for _, btn := range []*splitterCollapseBtn{b.startBtn, b.endBtn} {
		if btn == nil || !btn.layoutOn {
			continue
		}
		mode := info.ShowEndIcon
		if btn.side == CollapseStart {
			mode = info.ShowStartIcon
		}
		switch mode {
		case CollapsibleIconAlways:
			btn.paintOn = true
		case CollapsibleIconAuto:
			btn.paintOn = b.hovered || b.focusVisible || b.active ||
				(b.startBtn != nil && b.startBtn.hovered) ||
				(b.endBtn != nil && b.endBtn.hovered)
		default:
			btn.paintOn = false
		}
		btn.MarkNeedsPaint()
	}
}

func (b *splitterBar) Paint(pc *core.PaintContext) {
	if b == nil {
		return
	}
	// dashed rail stroke (solid uses child Box)
	if b.dashed && pc != nil && b.owner != nil {
		sz := b.Size()
		lw := b.owner.barSize()
		if lw < 1 {
			lw = 1
		}
		dash := b.owner.BarStyle.Dash
		if len(dash) == 0 {
			dash = []float64{4, 4}
		}
		if pc.DC != nil {
			pc.DC.SetDash(dash...)
		}
		col := b.railCol
		if b.owner.IsVertical() {
			y := sz.Height / 2
			pc.StrokeLocalLine(0, y, sz.Width, y, lw, col)
		} else {
			x := sz.Width / 2
			pc.StrokeLocalLine(x, 0, x, sz.Height, lw, col)
		}
		if pc.DC != nil {
			pc.DC.SetDash() // clear
		}
	}
	b.DefaultPaintChildren(pc)
	// focus ring when keyboard focus-visible
	if b.focusVisible && b.focused && pc != nil && b.owner != nil {
		sz := b.Size()
		th := b.owner.theme()
		ring := themeColorOr(th, core.TokenColorControlOutline, render.RGBA{R: 0.02, G: 0.4, B: 1, A: 0.4})
		outset := 1.5
		pc.FillLocalRect(-outset, -outset, sz.Width+2*outset, 1.5, ring)
		pc.FillLocalRect(-outset, sz.Height, sz.Width+2*outset, 1.5, ring)
		pc.FillLocalRect(-outset, -outset, 1.5, sz.Height+2*outset, ring)
		pc.FillLocalRect(sz.Width, -outset, 1.5, sz.Height+2*outset, ring)
	}
	if pc != nil {
		b.ClearPaintDirty()
	}
}

func (b *splitterBar) HitTest(p core.Point) core.Node {
	if b == nil {
		return nil
	}
	// collapse arrows may overflow the 6px trigger box (antd absolute outside halfTrigger)
	for _, ch := range b.Children() {
		if ch == nil || ch == b.rail || ch == b.spin {
			continue
		}
		off := ch.Base().Offset()
		local := core.Point{X: p.X - off.X, Y: p.Y - off.Y}
		if hit := ch.HitTest(local); hit != nil {
			return hit
		}
	}
	if b.LocalBounds().Contains(p) {
		return b
	}
	return nil
}

func (b *splitterBar) HandlePointer(ev *core.PointerEvent) {
	if b == nil || b.owner == nil || ev == nil {
		return
	}
	s := b.owner
	switch ev.Type {
	case core.PointerMove:
		if b.active {
			var off float64
			if s.IsVertical() {
				off = ev.Y - b.startY
			} else {
				off = ev.X - b.startX
			}
			s.onBarDrag(b.index, off)
			ev.Handled = true
		}
	case core.PointerDown:
		if ev.Button != core.ButtonLeft {
			return
		}
		s.focusBar = b.index
		if !s.barResizable(b.index) {
			ev.Handled = true
			b.MarkNeedsPaint()
			return
		}
		b.active = true
		b.startX, b.startY = ev.X, ev.Y
		s.onBarDragStart(b.index)
		ev.Handled = true
		b.MarkNeedsPaint()
	case core.PointerUp, core.PointerCancel:
		if b.active {
			b.active = false
			s.onBarDragEnd(b.index)
			ev.Handled = true
			b.MarkNeedsPaint()
		}
	}
}

// SetHovered implements tree hoverable.
// Hover reveals auto-mode collapse arrows (antd collapse-bar-hover-only).
func (b *splitterBar) SetHovered(v bool) {
	if b == nil {
		return
	}
	// Moving bar → collapse child: tree calls bar.SetHovered(false) before
	// child.SetHovered(true). Do not force-keep hover here from stale child
	// flags; child enter will set bar.hovered again in the same move event.
	// Leaving bar → outside: children already false (or never hovered).
	if b.hovered == v {
		// still refresh paintOn if child hover changed under us
		if !v {
			b.syncCollapsePaint()
		}
		return
	}
	b.hovered = v
	b.syncCollapsePaint()
	// rail color + auto icons
	b.MarkNeedsLayout()
	b.MarkNeedsPaint()
}

func (b *splitterBar) HandleKey(ev *core.KeyEvent) {
	if b == nil || b.owner == nil || ev == nil {
		return
	}
	s := b.owner
	vert := s.IsVertical()
	if ev.Type != core.KeyDown {
		return
	}
	key := ev.Key
	switch {
	case (!vert && (key == "ArrowLeft" || key == "Left")) ||
		(vert && (key == "ArrowUp" || key == "Up")):
		s.keyboardNudge(b.index, -1)
		ev.Handled = true
	case (!vert && (key == "ArrowRight" || key == "Right")) ||
		(vert && (key == "ArrowDown" || key == "Down")):
		s.keyboardNudge(b.index, 1)
		ev.Handled = true
	}
}

// CanFocus implements core.FocusTarget.
func (b *splitterBar) CanFocus() bool { return true }

// SetFocused implements core.FocusTarget.
func (b *splitterBar) SetFocused(v bool) {
	if b == nil {
		return
	}
	b.focused = v
	if b.owner != nil {
		if v {
			b.owner.focusBar = b.index
		} else if b.owner.focusBar == b.index {
			b.owner.focusBar = -1
		}
	}
	b.syncCollapsePaint()
	b.MarkNeedsLayout()
	b.MarkNeedsPaint()
}

// SetFocusVisible implements core.FocusVisibleTarget.
// Keyboard focus (Tab) shows auto collapse icons; mouse focus does not.
func (b *splitterBar) SetFocusVisible(v bool) {
	if b == nil {
		return
	}
	if b.focusVisible == v {
		return
	}
	b.focusVisible = v
	b.syncCollapsePaint()
	b.MarkNeedsPaint()
}
