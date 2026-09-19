// SpaceAddon widget (docs/antd/space.md §6.10).
//
// Custom cell inside Compact: fixed control height per size档, children
// in a zero-gap centered row. Middle-in-compact clears corner radius.
package space

import (
	"sync/atomic"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Control height fallbacks when theme tokens are zero (antd §6.2.1).
const (
	DefaultControlHeightSM = 24.0
	DefaultControlHeight   = 32.0
	DefaultControlHeightLG = 40.0
	DefaultRadius          = 6.0
)

// SpaceAddon is the Space.Addon widget (docs/antd/space.md §6.10).
type SpaceAddon struct {
	children []rendering.RenderObject
	size     SpaceSize
	disabled bool
	// edges packs compact-edge flags as bits (R2-6): bit0 first, bit1 last,
	// bit2 edgesSet. Layout writes (UI), paintCell reads (raster).
	edges    atomic.Uint32
	provider *theme.Provider
	override *theme.Tokens
	node     *addonNode
}

// NewSpaceAddon creates a middle-size addon cell.
func NewSpaceAddon(children ...rendering.RenderObject) *SpaceAddon {
	a := &SpaceAddon{size: SpaceSizeMiddle}
	a.node = newAddonNode(a)
	a.SetChildren(children...)
	return a
}

// SetChildren replaces the cell content.
func (a *SpaceAddon) SetChildren(children ...rendering.RenderObject) {
	if a == nil {
		return
	}
	a.children = a.children[:0]
	for _, ch := range children {
		if ch != nil {
			a.children = append(a.children, ch)
		}
	}
	a.resync()
}

// SetChild replaces the cell with a single child.
func (a *SpaceAddon) SetChild(ch rendering.RenderObject) {
	if a == nil {
		return
	}
	a.children = a.children[:0]
	if ch != nil {
		a.children = append(a.children, ch)
	}
	a.resync()
}

// Add appends cell content.
func (a *SpaceAddon) Add(children ...rendering.RenderObject) {
	if a == nil {
		return
	}
	for _, ch := range children {
		if ch != nil {
			a.children = append(a.children, ch)
		}
	}
	a.resync()
}

// Children returns the cell content in order.
func (a *SpaceAddon) Children() []rendering.RenderObject {
	if a == nil {
		return nil
	}
	return append([]rendering.RenderObject(nil), a.children...)
}

// ChildCount returns the content count.
func (a *SpaceAddon) ChildCount() int {
	if a == nil {
		return 0
	}
	return len(a.children)
}

// SetSize sets small/middle/large (height/pad/radius source).
func (a *SpaceAddon) SetSize(v SpaceSize) {
	if a == nil {
		return
	}
	if v != SpaceSizeSmall && v != SpaceSizeLarge {
		v = SpaceSizeMiddle
	}
	if a.size == v {
		return
	}
	a.size = v
	a.node.MarkNeedsLayout()
}

// Size returns the size档.
func (a *SpaceAddon) Size() SpaceSize {
	if a == nil {
		return SpaceSizeMiddle
	}
	return a.size
}

// ControlHeight returns the cell height for the size档 (theme-driven).
func (a *SpaceAddon) ControlHeight() float64 {
	if a == nil {
		return DefaultControlHeight
	}
	tok := a.themeTokens()
	switch a.size {
	case SpaceSizeSmall:
		if tok.ControlHeightSM > 0 {
			return tok.ControlHeightSM
		}
		return DefaultControlHeightSM
	case SpaceSizeLarge:
		if tok.ControlHeightLG > 0 {
			return tok.ControlHeightLG
		}
		return DefaultControlHeightLG
	default:
		if tok.ControlHeight > 0 {
			return tok.ControlHeight
		}
		return DefaultControlHeight
	}
}

// SetDisabled stores the disabled flag (content renders itself).
func (a *SpaceAddon) SetDisabled(b bool) {
	if a == nil || a.disabled == b {
		return
	}
	a.disabled = b
	a.node.MarkNeedsPaint()
}

// Disabled reports the flag.
func (a *SpaceAddon) Disabled() bool { return a != nil && a.disabled }

// Focusable is always false: cells never take Tab.
func (a *SpaceAddon) Focusable() bool { return false }

// SetCompactEdges records the compact position (middle clears radius).
func (a *SpaceAddon) SetCompactEdges(first, last bool) {
	if a == nil {
		return
	}
	a.edges.Store(edgeBits(first, last, true))
	a.node.MarkNeedsPaint()
}

// edgeBits packs compact-edge flags: bit0 first, bit1 last, bit2 set.
func edgeBits(first, last, set bool) uint32 {
	var b uint32
	if first {
		b |= 1
	}
	if last {
		b |= 2
	}
	if set {
		b |= 4
	}
	return b
}

// CompactEdges returns the recorded position (ok=false when standalone).
func (a *SpaceAddon) CompactEdges() (first, last, ok bool) {
	if a == nil {
		return false, false, false
	}
	e := a.edges.Load()
	if e&4 == 0 {
		return false, false, false
	}
	return e&1 != 0, e&2 != 0, true
}

// EffectiveRadius is 0 for middle-in-compact, else the theme radius.
func (a *SpaceAddon) EffectiveRadius() float64 {
	e := a.edges.Load()
	if e&4 != 0 && e&1 == 0 && e&2 == 0 {
		return 0
	}
	tok := a.themeTokens()
	if tok.Radius > 0 {
		return tok.Radius
	}
	return DefaultRadius
}

// SetTheme pins exact tokens (nil clears to provider).
func (a *SpaceAddon) SetTheme(tok *theme.Tokens) {
	if a == nil {
		return
	}
	a.override = tok
	a.node.MarkNeedsLayout()
}

// SetProvider selects the theme source (nil selects process default).
func (a *SpaceAddon) SetProvider(p *theme.Provider) {
	if a == nil {
		return
	}
	a.provider = p
	a.node.MarkNeedsLayout()
}

func (a *SpaceAddon) themeTokens() theme.Tokens {
	if a != nil && a.override != nil {
		return *a.override
	}
	if a != nil && a.provider != nil {
		return a.provider.Current()
	}
	return theme.Default.Current()
}

// Node returns the tree node (layout/paint/hit through it).
func (a *SpaceAddon) Node() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.node
}

// ChromeNode mirrors Node (Addon has no separate chrome).
func (a *SpaceAddon) ChromeNode() rendering.RenderObject { return a.Node() }

// Layout sizes the node under constraints.
func (a *SpaceAddon) Layout(c rendering.Constraints) rendering.Size {
	if a == nil || a.node == nil {
		return rendering.Size{}
	}
	return a.node.Layout(c)
}

// resync re-attaches node children in order.
func (a *SpaceAddon) resync() {
	if a == nil || a.node == nil {
		return
	}
	for _, ch := range append([]rendering.RenderObject(nil), a.node.Children()...) {
		a.node.RemoveChild(ch)
	}
	for _, ch := range a.children {
		a.node.AddChild(ch)
	}
	a.node.MarkNeedsLayout()
}

// addonNode embeds *RenderBox; Layout centers content in control height,
// OnPaint fills the bordered cell. Forwards compact edges to the widget
// so Compact sees a CompactEdgeSetter child.
type addonNode struct {
	*rendering.RenderBox
	addon *SpaceAddon
}

func newAddonNode(a *SpaceAddon) *addonNode {
	n := &addonNode{
		RenderBox: rendering.NewRenderBox(),
		addon:     a,
	}
	n.SetRepaintBoundary(true)
	n.SetRelayoutBoundary(true)
	self := n
	n.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paintCell(pc, size)
	}
	return n
}

// SetCompactEdges implements CompactEdgeSetter via the widget.
func (n *addonNode) SetCompactEdges(first, last bool) {
	if n == nil || n.addon == nil {
		return
	}
	n.addon.SetCompactEdges(first, last)
}

// Layout centers children vertically in control height, zero x-gap.
func (n *addonNode) Layout(c rendering.Constraints) rendering.Size {
	if n == nil || n.addon == nil || n.RenderBox == nil {
		return rendering.Size{}
	}
	a := n.addon
	if !n.ShouldRelayout(c) {
		return n.Size()
	}
	h := a.ControlHeight()
	inner := rendering.Constraints{MaxWidth: c.MaxWidth, MaxHeight: h}
	kids := append([]rendering.RenderObject(nil), a.children...)
	sizes := make([]rendering.Size, len(kids))
	var contentW float64
	for i, ch := range kids {
		if rendering.ManualLayoutOf(ch) {
			sizes[i] = ch.Size()
		} else {
			sizes[i] = ch.Layout(inner)
		}
		contentW += sizes[i].Width
	}
	n.FixedWidth, n.FixedHeight = contentW, h
	out := n.RenderBox.Layout(c)
	cursor := 0.0
	for i, ch := range kids {
		y := (out.Height - sizes[i].Height) / 2
		if y < 0 {
			y = 0
		}
		ch.SetOffset(rendering.Point{X: cursor, Y: y})
		cursor += sizes[i].Width
	}
	return out
}

func (n *addonNode) paintCell(pc *rendering.PaintContext, size rendering.Size) {
	if n == nil || n.addon == nil || pc == nil {
		return
	}
	tok := n.addon.themeTokens()
	bg, bd := tok.ColorBgContainer, tok.ColorBorderSecondary
	r := n.addon.EffectiveRadius()
	rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, r, bg.R, bg.G, bg.B, bg.A)
	lw := tok.LineWidth
	if lw <= 0 {
		lw = DefaultLineWidth
	}
	rendering.StrokeRoundRect(pc, 0, 0, size.Width, size.Height, r, lw, bd.R, bd.G, bd.B, bd.A)
}
