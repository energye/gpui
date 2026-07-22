package core

// HitBehavior controls how a node participates in hit testing.
type HitBehavior int

const (
	// HitTarget participates and can be the hit result (default for interactive).
	HitTarget HitBehavior = iota
	// HitTransparent lets hits fall through to siblings/parent behind it.
	HitTransparent
	// HitDefer tests children first; if none hit, the node itself is not a target.
	HitDefer
	// HitBlock absorbs hits within bounds even when no child is hit (e.g. Box).
	HitBlock
)

// Node is a single-tree element: layout + paint + hit merged (Flutter RO-like).
// Implementations embed NodeBase and set Self via Init.
type Node interface {
	// TypeID is a stable string such as "primitive.Box" for skin/plugin hooks.
	TypeID() string

	// Layout computes size under constraints and positions children.
	// Implementations must set their size via SetSize and child offsets.
	Layout(c Constraints) Size

	// Paint draws this node and children into pc (absolute origin applied).
	Paint(pc *PaintContext)

	// HitTest returns the deepest interactive node under p (parent-local coords).
	HitTest(p Point) Node

	// Structure
	Parent() Node
	Children() []Node
	// Base returns the embedded NodeBase (shared geometry / dirty / tree).
	Base() *NodeBase
}

// FlexFactorNode is implemented by Flexible / Spacer to participate in Flex.
type FlexFactorNode interface {
	Node
	// FlexGrow is the flex grow factor (>= 0). Zero means fixed intrinsic size.
	FlexGrow() float64
	// FlexShrink is reserved for future multi-pass shrink; M0 may ignore.
	FlexShrink() float64
}

// NodeBase provides tree links, geometry, dirty flags, and default hit/paint walk.
// Concrete nodes embed NodeBase and call Init(self) before use.
type NodeBase struct {
	self     Node
	parent   Node
	children []Node

	offset Point
	size   Size

	needsLayout bool
	needsPaint  bool
	mounted     bool
	tree        *Tree

	// lastConstraints caches the most recent Layout input for early-out (Flutter RO).
	lastConstraints Constraints
	hasConstraints  bool

	// isRepaintBoundary stops MarkNeedsPaint bubbling (Flutter RepaintBoundary).
	// Paint still walks this node; implementations may cache an offscreen layer.
	isRepaintBoundary bool

	// Hit controls hit participation; default HitDefer for containers.
	Hit HitBehavior

	// ClipHit when true restricts child hits to local bounds.
	ClipHit bool

	// Key is optional identity for future reconciliation.
	Key string

	// A11y minimal fields (C-A11y). Empty Role/Label means none.
	Role  string // e.g. "button", "dialog", "navigation", "listitem"
	Label string // accessible name
	// Live is aria-live region: "", "polite", "assertive".
	Live string
}

// Init wires the concrete self pointer. Call once after construction.
func (n *NodeBase) Init(self Node) {
	n.self = self
	n.needsLayout = true
	n.needsPaint = true
	if n.Hit == 0 && self != nil {
		// zero value is HitTarget; containers should set HitDefer/HitBlock explicitly.
	}
}

// Self returns the concrete node (or nil if Init was not called).
func (n *NodeBase) Self() Node {
	if n.self != nil {
		return n.self
	}
	return nil
}

// Base implements Node.Base.
func (n *NodeBase) Base() *NodeBase { return n }

// Parent implements Node.Parent.
func (n *NodeBase) Parent() Node { return n.parent }

// Children implements Node.Children.
func (n *NodeBase) Children() []Node { return n.children }

// Offset returns position relative to parent.
func (n *NodeBase) Offset() Point { return n.offset }

// SetOffset sets position relative to parent.
func (n *NodeBase) SetOffset(p Point) { n.offset = p }

// Size returns the laid-out size.
func (n *NodeBase) Size() Size { return n.size }

// SetSize records layout size.
func (n *NodeBase) SetSize(s Size) { n.size = s }

// LocalBounds returns the rect at local origin with laid-out size.
func (n *NodeBase) LocalBounds() Rect { return RectFromSize(n.size) }

// Tree returns the owning tree if mounted.
func (n *NodeBase) Tree() *Tree { return n.tree }

// AddChild appends a child and marks layout dirty.
func (n *NodeBase) AddChild(child Node) {
	if child == nil {
		return
	}
	cb := child.Base()
	if cb.parent != nil {
		cb.parent.Base().RemoveChild(child)
	}
	cb.parent = n.self
	n.children = append(n.children, child)
	if n.mounted && n.tree != nil {
		mountNode(child, n.tree)
	}
	n.MarkNeedsLayout()
}

// RemoveChild detaches a child if present.
func (n *NodeBase) RemoveChild(child Node) {
	if child == nil {
		return
	}
	for i, c := range n.children {
		if c == child {
			n.children = append(n.children[:i], n.children[i+1:]...)
			cb := child.Base()
			cb.parent = nil
			if cb.mounted {
				unmountNode(child)
			}
			n.MarkNeedsLayout()
			return
		}
	}
}

// ClearChildren removes all children.
func (n *NodeBase) ClearChildren() {
	for _, c := range n.children {
		c.Base().parent = nil
		if c.Base().mounted {
			unmountNode(c)
		}
	}
	n.children = nil
	n.MarkNeedsLayout()
}

// MarkNeedsLayout dirties this node and ancestors (always bubbles past boundaries).
func (n *NodeBase) MarkNeedsLayout() {
	n.needsLayout = true
	n.needsPaint = true
	if n.tree != nil {
		n.tree.markDirty()
	}
	for p := n.parent; p != nil; p = p.Parent() {
		b := p.Base()
		if b.needsLayout {
			break
		}
		b.needsLayout = true
		b.needsPaint = true
	}
}

// MarkNeedsPaint dirties paint without forcing full layout.
// Bubbling stops at a parent that is already needsPaint, or at a
// RepaintBoundary (Flutter: layer isolates paint dirty). The tree is still
// marked dirty so a frame is scheduled.
func (n *NodeBase) MarkNeedsPaint() {
	n.needsPaint = true
	if n.tree != nil {
		n.tree.markDirty()
	}
	for p := n.parent; p != nil; p = p.Parent() {
		b := p.Base()
		if b.needsPaint {
			break
		}
		// Paint dirty stops at the boundary: the boundary itself becomes dirty
		// so it re-rasterizes; ancestors above it stay clean.
		if b.isRepaintBoundary {
			b.needsPaint = true
			break
		}
		b.needsPaint = true
	}
}

// SetRepaintBoundary marks this node as a Flutter-style paint isolation root.
func (n *NodeBase) SetRepaintBoundary(v bool) { n.isRepaintBoundary = v }

// IsRepaintBoundary reports paint isolation (Flutter RepaintBoundary).
func (n *NodeBase) IsRepaintBoundary() bool { return n.isRepaintBoundary }

// NeedsLayout reports layout dirtiness.
func (n *NodeBase) NeedsLayout() bool { return n.needsLayout }

// NeedsPaint reports paint dirtiness.
func (n *NodeBase) NeedsPaint() bool { return n.needsPaint }

// ClearDirty clears dirty flags after a successful frame phase.
func (n *NodeBase) ClearDirty() {
	n.needsLayout = false
	n.needsPaint = false
}

// ClearLayoutDirty clears only the layout dirty bit (after a successful Layout).
func (n *NodeBase) ClearLayoutDirty() { n.needsLayout = false }

// ClearPaintDirty clears only the paint dirty bit (after a successful Paint).
func (n *NodeBase) ClearPaintDirty() { n.needsPaint = false }

// ShouldRelayout reports whether Layout must recompute under c.
// Clean nodes with identical constraints skip work (Flutter RO early-out).
func (n *NodeBase) ShouldRelayout(c Constraints) bool {
	if n.needsLayout {
		return true
	}
	if !n.hasConstraints {
		return true
	}
	return !constraintsEqual(n.lastConstraints, c)
}

// RememberConstraints stores c after a successful Layout pass.
func (n *NodeBase) RememberConstraints(c Constraints) {
	n.lastConstraints = c
	n.hasConstraints = true
	n.needsLayout = false
}

// LayoutSkipIfClean returns the cached size when layout can be skipped.
// Call at the start of Layout; ok=true means return size immediately.
func (n *NodeBase) LayoutSkipIfClean(c Constraints) (size Size, ok bool) {
	if n.ShouldRelayout(c) {
		return Size{}, false
	}
	return n.size, true
}

func constraintsEqual(a, b Constraints) bool {
	return a.MinWidth == b.MinWidth && a.MaxWidth == b.MaxWidth &&
		a.MinHeight == b.MinHeight && a.MaxHeight == b.MaxHeight
}

// DefaultHitTest walks children reverse-z then applies Hit behavior.
// Concrete nodes may call this from HitTest.
func (n *NodeBase) DefaultHitTest(p Point) Node {
	if n.ClipHit && !n.LocalBounds().Contains(p) {
		return nil
	}
	// Children are painted in order; last child is topmost.
	for i := len(n.children) - 1; i >= 0; i-- {
		c := n.children[i]
		cb := c.Base()
		local := p.Sub(cb.offset)
		if hit := c.HitTest(local); hit != nil {
			return hit
		}
	}
	switch n.Hit {
	case HitTransparent:
		return nil
	case HitDefer:
		return nil
	case HitBlock, HitTarget:
		if n.LocalBounds().Contains(p) {
			if n.self != nil {
				return n.self
			}
		}
		return nil
	default:
		if n.LocalBounds().Contains(p) && n.self != nil {
			return n.self
		}
		return nil
	}
}

// DefaultPaintChildren paints children with translated origin.
//
// When pc.CompositeOnly is set (retained composite frame after the first full
// paint), children that are not paint-dirty are still visited if they contain
// RepaintBoundary descendants that may blit a cached layer; leaf-like clean
// subtrees without dirty descendants are skipped to cut CPU.
func (n *NodeBase) DefaultPaintChildren(pc *PaintContext) {
	for _, c := range n.children {
		cb := c.Base()
		if pc != nil && pc.CompositeOnly && !cb.needsPaint && !subtreeHasPaintDirty(c) {
			// Still blit clean repaint boundaries so the retained surface stays correct
			// when the parent path is composite-only and this branch only holds layers.
			if !subtreeHasRepaintBoundary(c) {
				continue
			}
		}
		childPC := pc.WithOrigin(pc.Origin.Add(cb.offset))
		c.Paint(childPC)
	}
}

// subtreeHasPaintDirty reports whether n or any descendant needs paint.
func subtreeHasPaintDirty(n Node) bool {
	if n == nil {
		return false
	}
	b := n.Base()
	if b.needsPaint {
		return true
	}
	for _, c := range b.children {
		if subtreeHasPaintDirty(c) {
			return true
		}
	}
	return false
}

// subtreeHasRepaintBoundary reports whether n or any descendant is a boundary.
func subtreeHasRepaintBoundary(n Node) bool {
	if n == nil {
		return false
	}
	b := n.Base()
	if b.isRepaintBoundary {
		return true
	}
	for _, c := range b.children {
		if subtreeHasRepaintBoundary(c) {
			return true
		}
	}
	return false
}

func mountNode(n Node, t *Tree) {
	b := n.Base()
	if b.mounted {
		return
	}
	b.mounted = true
	b.tree = t
	if m, ok := n.(interface{ OnMount() }); ok {
		m.OnMount()
	}
	for _, c := range b.children {
		mountNode(c, t)
	}
}

func unmountNode(n Node) {
	b := n.Base()
	for _, c := range b.children {
		unmountNode(c)
	}
	if m, ok := n.(interface{ OnUnmount() }); ok {
		m.OnUnmount()
	}
	b.mounted = false
	b.tree = nil
}
