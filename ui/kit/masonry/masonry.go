// Package masonry implements the masonry control (docs/antd/masonry.md §6).
//
// Composition over new frameworks: the widget owns a masonryBox node (Node)
// and reuses ui/rendering for layout/paint, ui/theme for tokens.
// No new event or frame system; pure layout container, no chrome.
package masonry

import (
	"strconv"
	"sync"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// DefaultColumns is the fallback when columns are unset or 0 (antd §6.2.1).
const DefaultColumns = 3

// Breakpoint names an antd responsive breakpoint (antd §6.2.1).
type Breakpoint string

const (
	BreakpointXS   Breakpoint = "xs"
	BreakpointSM   Breakpoint = "sm"
	BreakpointMD   Breakpoint = "md"
	BreakpointLG   Breakpoint = "lg"
	BreakpointXL   Breakpoint = "xl"
	BreakpointXXL  Breakpoint = "xxl"
	BreakpointXXXL Breakpoint = "xxxl"
)

// breakpointOrder checks largest first (responsiveArray order).
var breakpointOrder = []Breakpoint{
	BreakpointXXXL, BreakpointXXL, BreakpointXL, BreakpointLG,
	BreakpointMD, BreakpointSM, BreakpointXS,
}

// breakpointMinWidth is the inclusive lower edge per breakpoint.
var breakpointMinWidth = map[Breakpoint]float64{
	BreakpointSM:   576,
	BreakpointMD:   768,
	BreakpointLG:   992,
	BreakpointXL:   1200,
	BreakpointXXL:  1600,
	BreakpointXXXL: 1920,
}

// matchesBreakpoint reports viewport membership (xs is max-width 575).
func matchesBreakpoint(bp Breakpoint, viewportW float64) bool {
	if bp == BreakpointXS {
		return viewportW < 576
	}
	min, ok := breakpointMinWidth[bp]
	if !ok {
		return false
	}
	return viewportW >= min
}

// MasonryItem is one waterfall entry (docs/antd/masonry.md §6.10).
type MasonryItem struct {
	Key     string
	Column  int     // -1 means auto into the shortest column
	Height  float64 // 0 means measure Content
	Data    any
	Content rendering.RenderObject
}

// MasonryColumn is one onLayoutChange entry.
type MasonryColumn struct {
	Key    string
	Column int
}

// ItemRender builds a node for items without Content (Content wins).
type ItemRender func(item MasonryItem, index, column int) rendering.RenderObject

// OnLayoutChange receives column ownership in items order.
type OnLayoutChange func(items []MasonryColumn)

// SemanticKey names the classNames/styles hooks (P1, root/item per _semantic.tsx).
type SemanticKey string

const (
	SemanticRoot SemanticKey = "root"
	SemanticItem SemanticKey = "item"
)

// Style is an opaque semantic inline-style hook (P1 styles).
// Stored only; never moves layout (class strings are not measured).
type Style struct {
	Props map[string]string
}

// GlobalConfig mirrors ConfigProvider masonry defaults (P1, masonry-owned so
// shared packages stay untouched). Unset fields keep NewMasonry baselines.
type GlobalConfig struct {
	Columns          int
	HasColumns       bool
	GutterH          float64
	GutterV          float64
	HasGutter        bool
	Fresh            bool
	HasFresh         bool
	ReducedMotion    bool
	HasReducedMotion bool
}

var globalMu sync.RWMutex
var globalCfg GlobalConfig

// SetGlobalConfig installs ConfigProvider-style masonry defaults (P1).
func SetGlobalConfig(c GlobalConfig) {
	globalMu.Lock()
	globalCfg = c
	globalMu.Unlock()
}

// GetGlobalConfig returns the current global defaults snapshot.
func GetGlobalConfig() GlobalConfig {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalCfg
}

// ResetGlobalConfig clears global defaults to NewMasonry baselines.
func ResetGlobalConfig() {
	globalMu.Lock()
	globalCfg = GlobalConfig{}
	globalMu.Unlock()
}

// Masonry is the masonry widget (docs/antd/masonry.md §6.10).
type Masonry struct {
	items       []MasonryItem
	colFixed    int
	colResp     map[Breakpoint]int
	colIsResp   bool
	gutterH     float64
	gutterV     float64
	gutterRespH map[Breakpoint]float64
	gutterRespV map[Breakpoint]float64
	gutterResp  bool
	viewportW   float64
	render      ItemRender
	fresh       bool
	freshSet    bool
	gutterSet   bool
	onChange    OnLayoutChange
	provider    *theme.Provider
	override    *theme.Tokens
	ariaLabel   string
	classNames  map[SemanticKey]string
	semStyles   map[SemanticKey]Style
	reduced     bool
	reducedSet  bool
	node        *masonryBox

	lastWidth   float64
	lastColumns int
	lastGH      float64
	lastGV      float64
	colOf       map[string]int
	boxes       map[string]rendering.Rect
	colHeights  []float64
	measure     map[string]float64
}

// NewMasonry creates a container with default columns (3) and gutter 0.
func NewMasonry(items ...MasonryItem) *Masonry {
	m := &Masonry{colOf: map[string]int{}, boxes: map[string]rendering.Rect{}, measure: map[string]float64{}}
	m.node = newMasonryBox(m)
	m.SetItems(itemsToSlice(items))
	// Explicit props win over globals; resolve lazily in Effective* so a
	// late SetGlobalConfig still applies to unset instances (button parity).
	return m
}

func itemsToSlice(items []MasonryItem) []MasonryItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]MasonryItem, len(items))
	copy(out, items)
	return out
}

func effectiveKey(item MasonryItem, index int) string {
	if item.Key != "" {
		return item.Key
	}
	return strconv.Itoa(index)
}

// SetColumns sets a fixed column count (<=0 selects DefaultColumns).
func (m *Masonry) SetColumns(n int) {
	if m == nil {
		return
	}
	if m.colFixed == n && !m.colIsResp {
		return
	}
	m.colFixed = n
	m.colIsResp = false
	m.node.MarkNeedsLayout()
}

// SetResponsiveColumns sets per-breakpoint counts (last call wins over SetColumns).
func (m *Masonry) SetResponsiveColumns(mp map[Breakpoint]int) {
	if m == nil {
		return
	}
	cp := map[Breakpoint]int{}
	for k, v := range mp {
		cp[k] = v
	}
	m.colResp = cp
	m.colIsResp = true
	m.node.MarkNeedsLayout()
}

// SetGutter sets horizontal/vertical px (vertical<0 follows horizontal).
func (m *Masonry) SetGutter(horizontal, vertical float64) {
	if m == nil {
		return
	}
	if horizontal < 0 {
		horizontal = 0
	}
	if vertical < 0 {
		vertical = horizontal
	}
	wasSet := m.gutterSet
	m.gutterSet = true
	if wasSet && m.gutterH == horizontal && m.gutterV == vertical && !m.gutterResp {
		return
	}
	if !wasSet && m.gutterH == horizontal && m.gutterV == vertical && !m.gutterResp {
		m.node.MarkNeedsLayout()
		return
	}
	m.gutterH = horizontal
	m.gutterV = vertical
	m.gutterResp = false
	m.node.MarkNeedsLayout()
}

// SetResponsiveGutter sets one responsive gutter for both axes.
func (m *Masonry) SetResponsiveGutter(mp map[Breakpoint]float64) {
	if m == nil {
		return
	}
	cp := map[Breakpoint]float64{}
	for k, v := range mp {
		cp[k] = v
	}
	m.gutterRespH = cp
	m.gutterRespV = nil
	m.gutterResp = true
	m.node.MarkNeedsLayout()
}

// SetResponsiveGutterPair sets horizontal/vertical responsive gutters.
func (m *Masonry) SetResponsiveGutterPair(h, v map[Breakpoint]float64) {
	if m == nil {
		return
	}
	ch := map[Breakpoint]float64{}
	for k, val := range h {
		ch[k] = val
	}
	var cv map[Breakpoint]float64
	if v != nil {
		cv = map[Breakpoint]float64{}
		for k, val := range v {
			cv[k] = val
		}
	}
	m.gutterRespH = ch
	m.gutterRespV = cv
	m.gutterResp = true
	m.node.MarkNeedsLayout()
}

// SetViewportWidth selects the breakpoint input for responsive props.
func (m *Masonry) SetViewportWidth(w float64) {
	if m == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	if m.viewportW == w {
		return
	}
	m.viewportW = w
	m.node.MarkNeedsLayout()
}

// ViewportWidth returns the responsive input width.
func (m *Masonry) ViewportWidth() float64 {
	if m == nil {
		return 0
	}
	return m.viewportW
}

// SetItems replaces all items.
func (m *Masonry) SetItems(items []MasonryItem) {
	if m == nil {
		return
	}
	cp := make([]MasonryItem, len(items))
	copy(cp, items)
	m.items = cp
	m.syncChildren()
	m.node.MarkNeedsLayout()
}

// Items returns a copy of the items.
func (m *Masonry) Items() []MasonryItem {
	if m == nil {
		return nil
	}
	out := make([]MasonryItem, len(m.items))
	copy(out, m.items)
	return out
}

// ItemCount returns the item number.
func (m *Masonry) ItemCount() int {
	if m == nil {
		return 0
	}
	return len(m.items)
}

// SetItemRender sets the fallback renderer (Content wins).
func (m *Masonry) SetItemRender(fn ItemRender) {
	if m == nil {
		return
	}
	m.render = fn
	m.node.MarkNeedsLayout()
}

// ResolveItemNode returns Content when present else ItemRender output.
func (m *Masonry) ResolveItemNode(item MasonryItem, index, column int) rendering.RenderObject {
	if item.Content != nil {
		return item.Content
	}
	if m != nil && m.render != nil {
		return m.render(item, index, column)
	}
	return nil
}

// SetFresh toggles continuous child-size observation (P0 instant relayout).
func (m *Masonry) SetFresh(b bool) {
	if m == nil {
		return
	}
	if m.freshSet && m.fresh == b {
		return
	}
	m.fresh = b
	m.freshSet = true
	m.node.MarkNeedsLayout()
}

// Fresh reports the fresh flag (explicit wins, else global, else false).
func (m *Masonry) Fresh() bool {
	if m != nil && m.freshSet {
		return m.fresh
	}
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalCfg.HasFresh {
		return globalCfg.Fresh
	}
	if m != nil {
		return m.fresh
	}
	return false
}

// SetOnLayoutChange sets the column ownership callback.
func (m *Masonry) SetOnLayoutChange(fn OnLayoutChange) {
	if m == nil {
		return
	}
	m.onChange = fn
}

// EffectiveColumnCount resolves fixed/responsive counts (antd §6.4).
func (m *Masonry) EffectiveColumnCount() int {
	if m != nil && m.colIsResp && len(m.colResp) > 0 {
		for _, bp := range breakpointOrder {
			if !matchesBreakpoint(bp, m.viewportW) {
				continue
			}
			if v, ok := m.colResp[bp]; ok && v > 0 {
				return v
			}
		}
		if v, ok := m.colResp[BreakpointXS]; ok && v > 0 {
			return v
		}
		return 1
	}
	n := 0
	if m != nil {
		n = m.colFixed
	}
	if n <= 0 {
		globalMu.RLock()
		g := globalCfg
		globalMu.RUnlock()
		if g.HasColumns && g.Columns > 0 {
			return g.Columns
		}
		return DefaultColumns
	}
	return n
}

// resolveGutterMap returns the responsive value with xs fallback.
func resolveGutterMap(mp map[Breakpoint]float64, viewportW float64) (float64, bool) {
	for _, bp := range breakpointOrder {
		if !matchesBreakpoint(bp, viewportW) {
			continue
		}
		if v, ok := mp[bp]; ok {
			if v < 0 {
				v = 0
			}
			return v, true
		}
	}
	if v, ok := mp[BreakpointXS]; ok {
		if v < 0 {
			v = 0
		}
		return v, true
	}
	return 0, false
}

// EffectiveGutter resolves horizontal/vertical px (antd §6.4 Row semantics).
func (m *Masonry) EffectiveGutter() (float64, float64) {
	fh, fv := 0.0, 0.0
	if m != nil {
		fh, fv = m.gutterH, m.gutterV
	}
	if fh < 0 {
		fh = 0
	}
	if fv < 0 {
		fv = fh
	}
	if m != nil && !m.gutterResp && !m.gutterSet {
		globalMu.RLock()
		g := globalCfg
		globalMu.RUnlock()
		if g.HasGutter {
			gh, gv := g.GutterH, g.GutterV
			if gh < 0 {
				gh = 0
			}
			if gv < 0 {
				gv = gh
			}
			return gh, gv
		}
	}
	if m == nil || !m.gutterResp {
		return fh, fv
	}
	if len(m.gutterRespH) == 0 && (m.gutterRespV == nil || len(m.gutterRespV) == 0) {
		return fh, fv
	}
	h := fh
	if v, ok := resolveGutterMap(m.gutterRespH, m.viewportW); ok {
		h = v
	} else {
		h = fh
	}
	var v float64
	if m.gutterRespV == nil {
		v = h
	} else if len(m.gutterRespV) > 0 {
		if rv, ok := resolveGutterMap(m.gutterRespV, m.viewportW); ok {
			v = rv
		} else {
			v = fv
		}
	} else {
		v = h
	}
	return h, v
}

// ColumnOf returns the owning column for key (-1 when missing).
func (m *Masonry) ColumnOf(key string) int {
	if m == nil {
		return -1
	}
	if c, ok := m.colOf[key]; ok {
		return c
	}
	return -1
}

// ItemBox returns the laid-out rect for key (zero when missing).
func (m *Masonry) ItemBox(key string) rendering.Rect {
	if m == nil {
		return rendering.Rect{}
	}
	if r, ok := m.boxes[key]; ok {
		return r
	}
	return rendering.Rect{}
}

// ColumnHeights returns a copy of the last column heights.
func (m *Masonry) ColumnHeights() []float64 {
	if m == nil {
		return nil
	}
	out := make([]float64, len(m.colHeights))
	copy(out, m.colHeights)
	return out
}

// SetProvider selects the theme source (nil selects process default).
func (m *Masonry) SetProvider(p *theme.Provider) {
	if m == nil {
		return
	}
	m.provider = p
	m.node.MarkNeedsPaint()
}

// SetTheme pins exact tokens (nil clears to provider).
func (m *Masonry) SetTheme(t *theme.Tokens) {
	if m == nil {
		return
	}
	m.override = t
	m.node.MarkNeedsPaint()
}

func (m *Masonry) themeTokens() theme.Tokens {
	if m != nil && m.override != nil {
		return *m.override
	}
	if m != nil && m.provider != nil {
		return m.provider.Current()
	}
	return theme.Default.Current()
}

// SetAriaLabel names the container landmark (empty keeps it unnamed).
func (m *Masonry) SetAriaLabel(s string) {
	if m == nil || m.ariaLabel == s {
		return
	}
	m.ariaLabel = s
	m.node.MarkNeedsPaint()
}

// AriaLabel returns the accessible name ("" means unnamed).
func (m *Masonry) AriaLabel() string {
	if m == nil {
		return ""
	}
	return m.ariaLabel
}

// Role returns "group" for named containers, "" otherwise.
func (m *Masonry) Role() string {
	if m != nil && m.ariaLabel != "" {
		return "group"
	}
	return ""
}

// Focusable is always false: the container never takes Tab (children do).
func (m *Masonry) Focusable() bool { return false }

// SetClassName pins one semantic class hook (P1 classNames, paint-only).
func (m *Masonry) SetClassName(k SemanticKey, v string) {
	if m == nil {
		return
	}
	if m.classNames == nil {
		m.classNames = map[SemanticKey]string{}
	}
	if m.classNames[k] == v {
		return
	}
	if v == "" {
		delete(m.classNames, k)
	} else {
		m.classNames[k] = v
	}
	m.node.MarkNeedsPaint()
}

// ClassName reads one semantic class hook ("" when unset).
func (m *Masonry) ClassName(k SemanticKey) string {
	if m == nil || m.classNames == nil {
		return ""
	}
	return m.classNames[k]
}

// SetClassNames replaces all semantic class hooks (nil clears, paint-only).
func (m *Masonry) SetClassNames(mp map[SemanticKey]string) {
	if m == nil {
		return
	}
	if len(mp) == 0 {
		m.classNames = nil
		m.node.MarkNeedsPaint()
		return
	}
	cp := make(map[SemanticKey]string, len(mp))
	for k, v := range mp {
		if v != "" {
			cp[k] = v
		}
	}
	m.classNames = cp
	m.node.MarkNeedsPaint()
}

// SetSemanticStyle pins one semantic inline-style hook (P1 styles, paint-only).
func (m *Masonry) SetSemanticStyle(k SemanticKey, s Style) {
	if m == nil {
		return
	}
	if m.semStyles == nil {
		m.semStyles = map[SemanticKey]Style{}
	}
	cp := map[string]string{}
	for kk, vv := range s.Props {
		cp[kk] = vv
	}
	m.semStyles[k] = Style{Props: cp}
	m.node.MarkNeedsPaint()
}

// SemanticStyle reads one semantic style hook.
func (m *Masonry) SemanticStyle(k SemanticKey) (Style, bool) {
	if m == nil || m.semStyles == nil {
		return Style{}, false
	}
	s, ok := m.semStyles[k]
	return s, ok
}

// ClearSemanticStyles removes all semantic style hooks (paint-only).
func (m *Masonry) ClearSemanticStyles() {
	if m == nil {
		return
	}
	m.semStyles = nil
	m.node.MarkNeedsPaint()
}

// SetReducedMotion opts out of item transitions (P0 instant, P1 pixel motion staged).
func (m *Masonry) SetReducedMotion(b bool) {
	if m == nil {
		return
	}
	if m.reducedSet && m.reduced == b {
		return
	}
	m.reduced = b
	m.reducedSet = true
	m.node.MarkNeedsPaint()
}

// ReducedMotion reports the reduced-motion flag (explicit wins, else global).
func (m *Masonry) ReducedMotion() bool {
	if m != nil && m.reducedSet {
		return m.reduced
	}
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalCfg.HasReducedMotion {
		return globalCfg.ReducedMotion
	}
	return false
}

// MotionEnabled reports whether item transitions may run (false under reduced-motion).
func (m *Masonry) MotionEnabled() bool { return !m.ReducedMotion() }

// Node returns the tree node (layout/paint/hit through it).
func (m *Masonry) Node() rendering.RenderObject {
	if m == nil {
		return nil
	}
	return m.node
}

// Layout sizes the node under constraints.
func (m *Masonry) Layout(c rendering.Constraints) rendering.Size {
	if m == nil || m.node == nil {
		return rendering.Size{}
	}
	return m.node.Layout(c)
}

func (m *Masonry) syncChildren() {
	if m == nil || m.node == nil {
		return
	}
	kids := append([]rendering.RenderObject(nil), m.node.Children()...)
	for _, ch := range kids {
		m.node.RemoveChild(ch)
	}
	for _, it := range m.items {
		if it.Content != nil {
			m.node.AddChild(it.Content)
		}
	}
}

// masonryBox is the layout node. It embeds *RenderBox so all
// RenderObject storage (size/dirty/parent, including unexported
// Base bits) lives in the rendering package; this file only
// overrides Layout to arrange children with masonry rules.
type masonryBox struct {
	*rendering.RenderBox
	m *Masonry
}

func newMasonryBox(m *Masonry) *masonryBox {
	b := &masonryBox{RenderBox: rendering.NewRenderBox(), m: m}
	b.SetRepaintBoundary(true)
	b.SetRelayoutBoundary(true)
	return b
}

// Layout implements RenderObject (antd usePositions order, shortest-first).
//
// Measures Height==0 items at the resolved item width first, derives the
// content size, delegates sizing to RenderBox, then tightens children and
// assigns offsets after the inner layout (so the box reset never survives).
func (b *masonryBox) Layout(c rendering.Constraints) rendering.Size {
	if b == nil || b.m == nil || b.RenderBox == nil {
		return rendering.Size{}
	}
	if !b.ShouldRelayout(c) {
		return b.Size()
	}
	m := b.m
	var W float64
	if c.MaxWidth < rendering.Unbounded/2 {
		W = c.MaxWidth
	} else if c.MinWidth > 0 {
		W = c.MinWidth
	} else {
		W = m.lastWidth
	}
	if W < 0 {
		W = 0
	}
	n := m.EffectiveColumnCount()
	if n <= 0 {
		n = DefaultColumns
	}
	h, v := m.EffectiveGutter()
	var colW, itemW float64
	if n > 0 {
		colW = (W + h) / float64(n)
		itemW = colW - h
		if itemW < 0 {
			itemW = 0
		}
	}
	colHeights := make([]float64, n)
	newCol := make(map[string]int, len(m.items))
	newBoxes := make(map[string]rendering.Rect, len(m.items))
	assigns := make([]MasonryColumn, 0, len(m.items))
	if m.measure == nil {
		m.measure = map[string]float64{}
	}
	heights := make([]float64, len(m.items))
	for idx, it := range m.items {
		key := effectiveKey(it, idx)
		var ht float64
		if it.Height > 0 {
			ht = it.Height
		} else {
			if it.Height < 0 {
				ht = 0
			} else if m.fresh {
				if it.Content != nil && !rendering.ManualLayoutOf(it.Content) {
					sz := it.Content.Layout(rendering.Constraints{MinWidth: itemW, MaxWidth: itemW, MaxHeight: rendering.Unbounded})
					ht = sz.Height
				} else if it.Content != nil {
					ht = it.Content.Size().Height
				}
				if ht < 0 {
					ht = 0
				}
				m.measure[key] = ht
			} else {
				if cached, ok := m.measure[key]; ok {
					ht = cached
				} else {
					if it.Content != nil && !rendering.ManualLayoutOf(it.Content) {
						sz := it.Content.Layout(rendering.Constraints{MinWidth: itemW, MaxWidth: itemW, MaxHeight: rendering.Unbounded})
						ht = sz.Height
					} else if it.Content != nil {
						ht = it.Content.Size().Height
					}
					if ht < 0 {
						ht = 0
					}
					m.measure[key] = ht
				}
			}
		}
		var col int
		if it.Column < 0 {
			col = 0
			best := colHeights[0]
			for j := 1; j < n; j++ {
				if colHeights[j] < best-1e-9 {
					best = colHeights[j]
					col = j
				}
			}
		} else {
			col = it.Column
			if col >= n {
				col = n - 1
			}
			if col < 0 {
				col = 0
			}
		}
		top := colHeights[col]
		left := float64(col) * colW
		newCol[key] = col
		newBoxes[key] = rendering.NewRect(left, top, itemW, ht)
		assigns = append(assigns, MasonryColumn{Key: key, Column: col})
		colHeights[col] += ht + v
		heights[idx] = ht
	}
	var rootH float64
	if len(m.items) > 0 && n > 0 {
		maxH := colHeights[0]
		for _, hh := range colHeights[1:] {
			if hh > maxH {
				maxH = hh
			}
		}
		rootH = maxH - v
		if rootH < 0 {
			rootH = 0
		}
	}
	b.FixedWidth, b.FixedHeight = W, rootH
	out := b.RenderBox.Layout(c)
	for idx, it := range m.items {
		if it.Content == nil || rendering.ManualLayoutOf(it.Content) {
			continue
		}
		key := effectiveKey(it, idx)
		bx := newBoxes[key]
		it.Content.Layout(rendering.Tight(bx.Size().Width, heights[idx]))
		it.Content.SetOffset(rendering.Point{X: bx.Min.X, Y: bx.Min.Y})
	}
	m.lastWidth = W
	m.lastColumns = n
	m.lastGH = h
	m.lastGV = v
	m.colOf = newCol
	m.boxes = newBoxes
	m.colHeights = append(m.colHeights[:0], colHeights...)
	if m.onChange != nil {
		cp := append([]MasonryColumn(nil), assigns...)
		m.onChange(cp)
	}
	return out
}
