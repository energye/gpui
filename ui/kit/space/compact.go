// SpaceCompact widget (docs/antd/space.md §6.10).
//
// Adjacent children overlap by one line width (no doubled 2px border);
// middle items drop corner radius (uniform-radius P0 approximation).
// Size/theme flow down to SpaceAddon children (useCompactItemContext).
package space

import (
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// DefaultLineWidth is the border overlap fallback (theme LineWidth, §6.2).
const DefaultLineWidth = 1.0

// CompactEdgeSetter is implemented by children (SpaceAddon, future Button)
// so Compact can clear middle-item corner radius.
type CompactEdgeSetter interface {
	SetCompactEdges(first, last bool)
}

// SpaceCompact is the Space.Compact widget (docs/antd/space.md §6.10).
type SpaceCompact struct {
	orient    SpaceOrientation
	orientSet bool
	vertical  bool
	block     bool
	size      SpaceSize
	lineWidth float64
	provider  *theme.Provider
	override  *theme.Tokens
	ariaLabel string
	children  []rendering.RenderObject
	node      *compactNode
}

// NewSpaceCompact creates a horizontal middle-size compact row.
func NewSpaceCompact(children ...rendering.RenderObject) *SpaceCompact {
	c := &SpaceCompact{size: SpaceSizeMiddle}
	c.node = newCompactNode(c)
	c.SetChildren(children...)
	return c
}

// SetOrientation sets horizontal/vertical (wins over Vertical).
func (c *SpaceCompact) SetOrientation(o SpaceOrientation) {
	if c == nil {
		return
	}
	if o != SpaceVertical {
		o = SpaceHorizontal
	}
	if c.orientSet && c.orient == o {
		return
	}
	c.orient = o
	c.orientSet = true
	c.node.MarkNeedsLayout()
}

// SetVertical sets the antd vertical sugar (ignored after SetOrientation).
func (c *SpaceCompact) SetVertical(b bool) {
	if c == nil || c.vertical == b {
		return
	}
	c.vertical = b
	c.node.MarkNeedsLayout()
}

// EffectiveOrientation resolves orientation > vertical > horizontal.
func (c *SpaceCompact) EffectiveOrientation() SpaceOrientation {
	if c != nil && c.orientSet {
		return c.orient
	}
	if c != nil && c.vertical {
		return SpaceVertical
	}
	return SpaceHorizontal
}

// IsVertical reports whether the main axis is vertical.
func (c *SpaceCompact) IsVertical() bool { return c.EffectiveOrientation() == SpaceVertical }

// SetBlock fills the parent width.
func (c *SpaceCompact) SetBlock(b bool) {
	if c == nil || c.block == b {
		return
	}
	c.block = b
	c.node.MarkNeedsLayout()
}

// Block reports the fill flag.
func (c *SpaceCompact) Block() bool { return c != nil && c.block }

// SetSize sets the size passed down to compact children.
func (c *SpaceCompact) SetSize(v SpaceSize) {
	if c == nil {
		return
	}
	if v != SpaceSizeSmall && v != SpaceSizeLarge {
		v = SpaceSizeMiddle
	}
	if c.size == v {
		return
	}
	c.size = v
	c.propagate()
	c.node.MarkNeedsLayout()
}

// Size returns the down-propagated size.
func (c *SpaceCompact) Size() SpaceSize {
	if c == nil {
		return SpaceSizeMiddle
	}
	return c.size
}

// SetLineWidth overrides the overlap width (<=0 follows theme LineWidth).
func (c *SpaceCompact) SetLineWidth(v float64) {
	if c == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	c.lineWidth = v
	c.node.MarkNeedsLayout()
}

// Overlap returns the effective border overlap (theme LineWidth fallback).
func (c *SpaceCompact) Overlap() float64 {
	if c == nil {
		return DefaultLineWidth
	}
	if c.lineWidth > 0 {
		return c.lineWidth
	}
	if w := c.themeTokens().LineWidth; w > 0 {
		return w
	}
	return DefaultLineWidth
}

// SetTheme pins exact tokens (nil clears to provider).
func (c *SpaceCompact) SetTheme(tok *theme.Tokens) {
	if c == nil {
		return
	}
	c.override = tok
	c.propagate()
	c.node.MarkNeedsLayout()
}

// SetProvider selects the theme source (nil selects process default).
func (c *SpaceCompact) SetProvider(p *theme.Provider) {
	if c == nil {
		return
	}
	c.provider = p
	c.propagate()
	c.node.MarkNeedsLayout()
}

func (c *SpaceCompact) themeTokens() theme.Tokens {
	if c != nil && c.override != nil {
		return *c.override
	}
	if c != nil && c.provider != nil {
		return c.provider.Current()
	}
	return theme.Default.Current()
}

// SetAriaLabel names the group (paint-only; never takes focus).
func (c *SpaceCompact) SetAriaLabel(v string) {
	if c == nil || c.ariaLabel == v {
		return
	}
	c.ariaLabel = v
	c.node.MarkNeedsPaint()
}

// AriaLabel returns the accessible name.
func (c *SpaceCompact) AriaLabel() string {
	if c == nil {
		return ""
	}
	return c.ariaLabel
}

// Role is "group" when named, "" otherwise.
func (c *SpaceCompact) Role() string {
	if c != nil && c.ariaLabel != "" {
		return "group"
	}
	return ""
}

// Focusable is always false: layout containers never take Tab.
func (c *SpaceCompact) Focusable() bool { return false }

// SetChildren replaces the children.
func (c *SpaceCompact) SetChildren(children ...rendering.RenderObject) {
	if c == nil {
		return
	}
	c.children = c.children[:0]
	for _, ch := range children {
		if ch != nil {
			c.children = append(c.children, ch)
		}
	}
	c.resync()
}

// Add appends children.
func (c *SpaceCompact) Add(children ...rendering.RenderObject) {
	if c == nil {
		return
	}
	for _, ch := range children {
		if ch != nil {
			c.children = append(c.children, ch)
		}
	}
	c.resync()
}

// AddAddon appends an addon with size/theme propagated (antd context).
func (c *SpaceCompact) AddAddon(a *SpaceAddon) {
	if c == nil || a == nil {
		return
	}
	c.pushAddon(a)
	c.children = append(c.children, a.Node())
	c.resync()
}

// Children returns the children in order.
func (c *SpaceCompact) Children() []rendering.RenderObject {
	if c == nil {
		return nil
	}
	return append([]rendering.RenderObject(nil), c.children...)
}

// ChildCount returns the child count.
func (c *SpaceCompact) ChildCount() int {
	if c == nil {
		return 0
	}
	return len(c.children)
}

// IsFirstItem reports the compact first position (radius kept).
func (c *SpaceCompact) IsFirstItem(i int) bool { return c != nil && i == 0 && len(c.children) > 0 }

// IsLastItem reports the compact last position (radius kept).
func (c *SpaceCompact) IsLastItem(i int) bool {
	return c != nil && i == len(c.children)-1 && len(c.children) > 0
}

// Node returns the tree node (layout/paint/hit through it).
func (c *SpaceCompact) Node() rendering.RenderObject {
	if c == nil {
		return nil
	}
	return c.node
}

// ChromeNode mirrors Node (Compact has no separate chrome).
func (c *SpaceCompact) ChromeNode() rendering.RenderObject { return c.Node() }

// Layout sizes the node under constraints.
func (c *SpaceCompact) Layout(cs rendering.Constraints) rendering.Size {
	if c == nil || c.node == nil {
		return rendering.Size{}
	}
	return c.node.Layout(cs)
}

// pushAddon propagates size/theme into one addon.
func (c *SpaceCompact) pushAddon(a *SpaceAddon) {
	if c == nil || a == nil {
		return
	}
	a.SetSize(c.size)
	if c.override != nil {
		a.SetTheme(c.override)
	} else if c.provider != nil {
		a.SetProvider(c.provider)
	}
}

// propagate pushes edge flags to setters and size/theme to addon nodes.
func (c *SpaceCompact) propagate() {
	if c == nil {
		return
	}
	for i, ch := range c.children {
		if es, ok := ch.(CompactEdgeSetter); ok && es != nil {
			es.SetCompactEdges(i == 0, i == len(c.children)-1)
		}
	}
	for _, ch := range c.children {
		if an, ok := ch.(*addonNode); ok && an != nil && an.addon != nil {
			c.pushAddon(an.addon)
		}
	}
}

// resync re-attaches node children in order and propagates context.
func (c *SpaceCompact) resync() {
	if c == nil || c.node == nil {
		return
	}
	for _, ch := range append([]rendering.RenderObject(nil), c.node.Children()...) {
		c.node.RemoveChild(ch)
	}
	for _, ch := range c.children {
		c.node.AddChild(ch)
	}
	c.propagate()
	c.node.MarkNeedsLayout()
}

// compactNode embeds *RenderBox; only Layout is overridden for overlap.
type compactNode struct {
	*rendering.RenderBox
	compact *SpaceCompact
}

func newCompactNode(c *SpaceCompact) *compactNode {
	n := &compactNode{
		RenderBox: rendering.NewRenderBox(),
		compact:   c,
	}
	n.SetRepaintBoundary(true)
	n.SetRelayoutBoundary(true)
	return n
}

// Layout stacks children with -overlap on the main axis, then delegates
// sizing to RenderBox (offsets assigned after the inner layout).
func (n *compactNode) Layout(cs rendering.Constraints) rendering.Size {
	if n == nil || n.compact == nil || n.RenderBox == nil {
		return rendering.Size{}
	}
	c := n.compact
	if !n.ShouldRelayout(cs) {
		return n.Size()
	}
	vertical := c.IsVertical()
	overlap := c.Overlap()
	kids := append([]rendering.RenderObject(nil), c.children...)
	if len(kids) == 0 {
		n.FixedWidth, n.FixedHeight = 0, 0
		return n.RenderBox.Layout(cs)
	}
	inner := rendering.Constraints{MaxWidth: cs.MaxWidth, MaxHeight: cs.MaxHeight}
	sizes := make([]rendering.Size, len(kids))
	for i, ch := range kids {
		if rendering.ManualLayoutOf(ch) {
			sizes[i] = ch.Size()
			continue
		}
		sizes[i] = ch.Layout(inner)
	}
	var contentW, contentH float64
	if !vertical {
		for i, sz := range sizes {
			contentW += sz.Width
			if i > 0 {
				contentW -= overlap
			}
			if sz.Height > contentH {
				contentH = sz.Height
			}
		}
		if c.block && cs.MaxWidth < rendering.Unbounded/2 {
			contentW = cs.MaxWidth
		}
	} else {
		for i, sz := range sizes {
			contentH += sz.Height
			if i > 0 {
				contentH -= overlap
			}
			if sz.Width > contentW {
				contentW = sz.Width
			}
		}
		if c.block && cs.MaxWidth < rendering.Unbounded/2 {
			contentW = cs.MaxWidth
		}
	}
	n.FixedWidth, n.FixedHeight = contentW, contentH
	out := n.RenderBox.Layout(cs)
	cursor := 0.0
	for i, ch := range kids {
		if !vertical {
			ch.SetOffset(rendering.Point{X: cursor, Y: 0})
			cursor += sizes[i].Width - overlap
		} else {
			ch.SetOffset(rendering.Point{X: 0, Y: cursor})
			cursor += sizes[i].Height - overlap
		}
	}
	return out
}
