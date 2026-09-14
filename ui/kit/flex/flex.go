// Package flex implements the Flex control (docs/antd/flex.md §6).
//
// Composition over new frameworks: the widget owns a flexNode (Node)
// built on ui/rendering AbsoluteBox for layout/paint, ui/theme for gap
// tokens. No new event or frame system; pure layout container, no chrome.
package flex

import (
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Gap fallbacks when theme tokens are zero (antd §6.2.1).
const (
	DefaultFlexGapSmall  = 8.0
	DefaultFlexGapMedium = 16.0
	DefaultFlexGapLarge  = 24.0
)

// FlexOrientation selects the main axis (orientation wins over Vertical).
type FlexOrientation string

const (
	FlexOrientationHorizontal FlexOrientation = "horizontal"
	FlexOrientationVertical   FlexOrientation = "vertical"
)

// FlexJustify selects main-axis distribution.
type FlexJustify string

const (
	FlexJustifyNormal       FlexJustify = "normal"
	FlexJustifyStart        FlexJustify = "start"
	FlexJustifyCenter       FlexJustify = "center"
	FlexJustifyEnd          FlexJustify = "end"
	FlexJustifySpaceBetween FlexJustify = "space-between"
	FlexJustifySpaceAround  FlexJustify = "space-around"
	FlexJustifySpaceEvenly  FlexJustify = "space-evenly"
)

// FlexAlign selects cross-axis alignment (auto follows antd rule).
type FlexAlign string

const (
	FlexAlignAuto    FlexAlign = "auto"
	FlexAlignNormal  FlexAlign = "normal"
	FlexAlignStart   FlexAlign = "start"
	FlexAlignCenter  FlexAlign = "center"
	FlexAlignEnd     FlexAlign = "end"
	FlexAlignStretch FlexAlign = "stretch"
)

// FlexGapSize selects the preset gap (unset = 0).
type FlexGapSize int

const (
	FlexGapUnset FlexGapSize = iota
	FlexGapSmall
	FlexGapMedium
	FlexGapLarge
)

// FlexGapMiddle is the antd middle alias of medium.
const FlexGapMiddle = FlexGapMedium

// Flex is the Flex widget (docs/antd/flex.md §6.10).
//
// Owns a flexNode: put Node() in the tree, drive Layout through it,
// read EffectiveOrientation/ResolvedGap for assertions.
type Flex struct {
	orient    FlexOrientation
	orientSet bool
	vertical  bool
	wrap      bool
	justify   FlexJustify
	align     FlexAlign
	gapSize   FlexGapSize
	gap       float64
	hasGap    bool
	provider  *theme.Provider
	override  *theme.Tokens
	ariaLabel string
	node      *flexNode
}

// NewFlex creates a horizontal start-aligned container with gap 0.
func NewFlex(children ...rendering.RenderObject) *Flex {
	f := &Flex{justify: FlexJustifyStart, align: FlexAlignAuto}
	f.node = newFlexNode(f)
	for _, c := range children {
		if c != nil {
			f.node.AddChild(c)
		}
	}
	return f
}

// SetOrientation sets horizontal/vertical (wins over Vertical).
func (f *Flex) SetOrientation(o FlexOrientation) {
	if f == nil {
		return
	}
	if o != FlexOrientationVertical {
		o = FlexOrientationHorizontal
	}
	if f.orientSet && f.orient == o {
		return
	}
	f.orient = o
	f.orientSet = true
	f.node.MarkNeedsLayout()
}

// SetVertical sets the antd vertical sugar (ignored after SetOrientation).
func (f *Flex) SetVertical(b bool) {
	if f == nil || f.vertical == b {
		return
	}
	f.vertical = b
	f.node.MarkNeedsLayout()
}

// EffectiveOrientation resolves orientation (orientation first).
func (f *Flex) EffectiveOrientation() FlexOrientation {
	if f != nil && f.orientSet {
		return f.orient
	}
	if f != nil && f.vertical {
		return FlexOrientationVertical
	}
	return FlexOrientationHorizontal
}

// IsVertical reports whether the main axis is vertical.
func (f *Flex) IsVertical() bool { return f.EffectiveOrientation() == FlexOrientationVertical }

// SetWrap toggles multi-line layout on a narrow main axis.
func (f *Flex) SetWrap(b bool) {
	if f == nil || f.wrap == b {
		return
	}
	f.wrap = b
	f.node.MarkNeedsLayout()
}

// Wrap reports the wrap flag.
func (f *Flex) Wrap() bool { return f != nil && f.wrap }

// SetJustify sets main-axis distribution (normal maps to start).
func (f *Flex) SetJustify(j FlexJustify) {
	if f == nil {
		return
	}
	if j == "" || j == FlexJustifyNormal {
		j = FlexJustifyStart
	}
	if f.justify == j {
		return
	}
	f.justify = j
	f.node.MarkNeedsLayout()
}

// Justify returns the effective justify (never normal).
func (f *Flex) Justify() FlexJustify {
	if f == nil || f.justify == "" {
		return FlexJustifyStart
	}
	return f.justify
}

// SetAlign sets cross-axis alignment.
func (f *Flex) SetAlign(a FlexAlign) {
	if f == nil {
		return
	}
	if a == "" {
		a = FlexAlignAuto
	}
	if f.align == a {
		return
	}
	f.align = a
	f.node.MarkNeedsLayout()
}

// Align returns the raw align (auto means axis default).
func (f *Flex) Align() FlexAlign {
	if f == nil || f.align == "" {
		return FlexAlignAuto
	}
	return f.align
}

// EffectiveAlign resolves auto: horizontal start, vertical stretch.
func (f *Flex) EffectiveAlign() FlexAlign {
	a := f.Align()
	if a != FlexAlignAuto && a != FlexAlignNormal {
		return a
	}
	if f.IsVertical() {
		return FlexAlignStretch
	}
	return FlexAlignStart
}

// SetGapSize selects unset/small/medium/large (clears explicit gap).
func (f *Flex) SetGapSize(s FlexGapSize) {
	if f == nil {
		return
	}
	f.gapSize = s
	f.hasGap = false
	f.node.MarkNeedsLayout()
}

// GapSize returns the preset.
func (f *Flex) GapSize() FlexGapSize {
	if f == nil {
		return FlexGapUnset
	}
	return f.gapSize
}

// SetGap sets an explicit px gap (wins over the preset).
func (f *Flex) SetGap(px float64) {
	if f == nil {
		return
	}
	if px < 0 {
		px = 0
	}
	if f.hasGap && f.gap == px {
		return
	}
	f.gap = px
	f.hasGap = true
	f.node.MarkNeedsLayout()
}

// ResolvedGap returns the px gap (explicit first, then preset/theme).
func (f *Flex) ResolvedGap() float64 {
	if f != nil && f.hasGap {
		return f.gap
	}
	if f == nil {
		return 0
	}
	switch f.gapSize {
	case FlexGapSmall:
		return DefaultFlexGapSmall
	case FlexGapMedium:
		tok := f.themeTokens()
		if tok.Padding > 0 {
			return tok.Padding
		}
		return DefaultFlexGapMedium
	case FlexGapLarge:
		tok := f.themeTokens()
		if tok.PaddingLG > 0 {
			return tok.PaddingLG
		}
		return DefaultFlexGapLarge
	default:
		return 0
	}
}

// Add appends a child.
func (f *Flex) Add(c rendering.RenderObject) {
	if f == nil || c == nil {
		return
	}
	f.node.AddChild(c)
}

// SetChildren replaces all children.
func (f *Flex) SetChildren(children ...rendering.RenderObject) {
	if f == nil {
		return
	}
	f.ClearChildren()
	for _, c := range children {
		if c != nil {
			f.node.AddChild(c)
		}
	}
}

// ClearChildren removes all children.
func (f *Flex) ClearChildren() {
	if f == nil {
		return
	}
	kids := append([]rendering.RenderObject(nil), f.node.Children()...)
	for _, c := range kids {
		f.node.RemoveChild(c)
	}
}

// Children returns the live child list.
func (f *Flex) Children() []rendering.RenderObject {
	if f == nil || f.node == nil {
		return nil
	}
	return f.node.Children()
}

// ChildCount returns the child number.
func (f *Flex) ChildCount() int { return len(f.Children()) }

// SetProvider selects the theme source (nil selects process default).
func (f *Flex) SetProvider(p *theme.Provider) {
	if f == nil {
		return
	}
	f.provider = p
	f.node.MarkNeedsLayout()
}

// SetTheme pins exact tokens (nil clears to provider).
func (f *Flex) SetTheme(t *theme.Tokens) {
	if f == nil {
		return
	}
	f.override = t
	f.node.MarkNeedsLayout()
}

func (f *Flex) themeTokens() theme.Tokens {
	if f != nil && f.override != nil {
		return *f.override
	}
	if f != nil && f.provider != nil {
		return f.provider.Current()
	}
	return theme.Default.Current()
}

// SetAriaLabel names the container landmark (empty keeps it unnamed).
func (f *Flex) SetAriaLabel(s string) {
	if f == nil || f.ariaLabel == s {
		return
	}
	f.ariaLabel = s
	f.node.MarkNeedsPaint()
}

// AriaLabel returns the accessible name ("" means unnamed).
func (f *Flex) AriaLabel() string {
	if f == nil {
		return ""
	}
	return f.ariaLabel
}

// Role returns "group" for named containers, "" otherwise.
func (f *Flex) Role() string {
	if f != nil && f.ariaLabel != "" {
		return "group"
	}
	return ""
}

// Focusable is always false: the container never takes Tab (children do).
func (f *Flex) Focusable() bool { return false }

// Node returns the tree node (layout/paint/hit through it).
func (f *Flex) Node() rendering.RenderObject {
	if f == nil {
		return nil
	}
	return f.node
}

// Layout sizes the node under constraints.
func (f *Flex) Layout(c rendering.Constraints) rendering.Size {
	if f == nil || f.node == nil {
		return rendering.Size{}
	}
	return f.node.Layout(c)
}

// flexNode is the layout node. It embeds *RenderBox so all
// RenderObject storage (size/dirty/parent, including unexported
// Base bits) lives in the rendering package; this file only
// overrides Layout to arrange children with flex rules.
type flexNode struct {
	*rendering.RenderBox
	flex *Flex
}

func newFlexNode(f *Flex) *flexNode {
	n := &flexNode{
		RenderBox: rendering.NewRenderBox(),
		flex:      f,
	}
	n.SetRepaintBoundary(true)
	n.SetRelayoutBoundary(true)
	return n
}

// Layout arranges children with flex rules, then delegates sizing
// to RenderBox (offsets are assigned after the inner layout, so the
// box reset to Pad never survives).
func (n *flexNode) Layout(c rendering.Constraints) rendering.Size {
	if n == nil || n.flex == nil || n.RenderBox == nil {
		return rendering.Size{}
	}
	f := n.flex
	// Fast path: clean subtree keeps prior size/offsets.
	if !n.ShouldRelayout(c) {
		return n.Size()
	}
	vertical := f.IsVertical()
	gap := f.ResolvedGap()
	justify := f.Justify()
	align := f.EffectiveAlign()
	kids := n.Children()
	count := len(kids)
	if count == 0 {
		n.FixedWidth, n.FixedHeight = 0, 0
		return n.RenderBox.Layout(c)
	}
	inner := rendering.Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
	sizes := make([]rendering.Size, count)
	for i, ch := range kids {
		if rendering.ManualLayoutOf(ch) {
			sizes[i] = ch.Size()
			continue
		}
		sizes[i] = ch.Layout(inner)
	}
	limit := c.MaxWidth
	if vertical {
		limit = c.MaxHeight
	}
	lines := [][]int{}
	if !f.wrap || limit >= rendering.Unbounded/2 {
		line := make([]int, count)
		for i := range line {
			line[i] = i
		}
		lines = append(lines, line)
	} else {
		cur := []int{}
		var curMain float64
		for i := 0; i < count; i++ {
			main := sizes[i].Width
			if vertical {
				main = sizes[i].Height
			}
			need := main
			if len(cur) > 0 {
				need += gap
			}
			if len(cur) > 0 && curMain+need > limit+1e-9 {
				lines = append(lines, cur)
				cur = []int{i}
				curMain = main
				continue
			}
			cur = append(cur, i)
			curMain += need
		}
		if len(cur) > 0 {
			lines = append(lines, cur)
		}
	}
	mainOf := func(s rendering.Size) float64 {
		if vertical {
			return s.Height
		}
		return s.Width
	}
	crossOf := func(s rendering.Size) float64 {
		if vertical {
			return s.Width
		}
		return s.Height
	}
	lineMain := make([]float64, len(lines))
	lineCross := make([]float64, len(lines))
	for li, line := range lines {
		var m, x float64
		for j, idx := range line {
			m += mainOf(sizes[idx])
			if j > 0 {
				m += gap
			}
			if v := crossOf(sizes[idx]); v > x {
				x = v
			}
		}
		lineMain[li] = m
		lineCross[li] = x
	}
	var contentW, contentH float64
	if !vertical {
		for _, m := range lineMain {
			if m > contentW {
				contentW = m
			}
		}
		for j, x := range lineCross {
			contentH += x
			if j > 0 {
				contentH += gap
			}
		}
	} else {
		contentH = 0
		for j, m := range lineMain {
			contentH += m
			if j > 0 {
				contentH += gap
			}
		}
		for _, x := range lineCross {
			if x > contentW {
				contentW = x
			}
		}
	}
	n.FixedWidth, n.FixedHeight = contentW, contentH
	out := n.RenderBox.Layout(c)
	// Justify/align placement on the final box (assigned after the
	// inner layout, overwriting the box Pad reset).
	var crossCursor float64
	stretchIdx := []int{}
	stretchTo := []float64{}
	for li, line := range lines {
		lm := lineMain[li]
		lx := lineCross[li]
		var containerMain, containerCross float64
		if !vertical {
			containerMain, containerCross = out.Width, out.Height
		} else {
			containerMain, containerCross = out.Height, out.Width
		}
		free := containerMain - lm
		if free < 0 {
			free = 0
		}
		k := len(line)
		var start, stepExtra float64
		switch justify {
		case FlexJustifyCenter:
			start = free / 2
		case FlexJustifyEnd:
			start = free
		case FlexJustifySpaceBetween:
			if k > 1 {
				stepExtra = free / float64(k-1)
			}
		case FlexJustifySpaceAround:
			if k > 0 {
				stepExtra = free / float64(k)
				start = stepExtra / 2
			}
		case FlexJustifySpaceEvenly:
			if k > 0 {
				stepExtra = free / float64(k+1)
				start = stepExtra
			}
		default:
			start = 0
		}
		wantStretch := align == FlexAlignStretch
		stretchTarget := lx
		if len(lines) == 1 && containerCross > stretchTarget {
			stretchTarget = containerCross
		}
		cursor := start
		for j, idx := range line {
			ch := kids[idx]
			sz := sizes[idx]
			if j > 0 {
				switch justify {
				case FlexJustifySpaceBetween, FlexJustifySpaceAround, FlexJustifySpaceEvenly:
					cursor += stepExtra
				}
			}
			cross := crossOf(sz)
			var coff float64
			switch align {
			case FlexAlignCenter:
				base := lx
				if len(lines) == 1 && containerCross > base {
					base = containerCross
				}
				coff = (base - cross) / 2
			case FlexAlignEnd:
				base := lx
				if len(lines) == 1 && containerCross > base {
					base = containerCross
				}
				coff = base - cross
			default:
				coff = 0
			}
			if !vertical {
				ch.SetOffset(rendering.Point{X: cursor, Y: crossCursor + coff})
				cursor += mainOf(sz) + gap
			} else {
				ch.SetOffset(rendering.Point{X: crossCursor + coff, Y: cursor})
				cursor += mainOf(sz) + gap
			}
			if wantStretch && cross < stretchTarget {
				stretchIdx = append(stretchIdx, idx)
				stretchTo = append(stretchTo, stretchTarget)
			}
		}
		crossCursor += lx + gap
	}
	// Grow stretch children on the cross axis after the box layout
	// (RenderBox lays children loosely, so tighten here last).
	for s, idx := range stretchIdx {
		ch := kids[idx]
		target := stretchTo[s]
		if !vertical {
			sizes[idx] = ch.Layout(rendering.Constraints{MaxWidth: inner.MaxWidth, MaxHeight: target, MinHeight: target})
		} else {
			sizes[idx] = ch.Layout(rendering.Constraints{MaxWidth: target, MinWidth: target, MaxHeight: inner.MaxHeight})
		}
	}
	return out
}
