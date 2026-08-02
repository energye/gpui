package rendering

// RenderObject is the L1 layout/paint/hit unit (Flutter RenderObject subset).
type RenderObject interface {
	// Parent / Children form the tree.
	Parent() RenderObject
	Children() []RenderObject

	// Layout computes size under constraints; must set Size and clear needsLayout.
	Layout(c Constraints) Size
	// Paint draws this node and children using logical origin.
	Paint(pc *PaintContext)
	// HitTest returns the deepest object containing p in parent-local logical coords.
	HitTest(p Point) RenderObject

	// MarkNeedsLayout dirties layout and bubbles to relayout boundary / root.
	MarkNeedsLayout()
	// MarkNeedsPaint dirties paint (P1: bubbles to root; P2: stops at RepaintBoundary).
	MarkNeedsPaint()

	NeedsLayout() bool
	NeedsPaint() bool
	Size() Size
	// Offset is position relative to parent (logical).
	Offset() Point
	SetOffset(p Point)

	// IsRelayoutBoundary stops markNeedsLayout bubbling past this node
	// after marking this node dirty (Flutter relayout boundary).
	IsRelayoutBoundary() bool
	// IsRepaintBoundary reserved for P2 paint isolation.
	IsRepaintBoundary() bool

	// attach/detach used by container helpers.
	setParent(p RenderObject)
	clearLayoutDirty()
	clearPaintDirty()
	setSize(s Size)
}

// Base implements shared tree/dirty state. Embed and set Self to the outer object.
type Base struct {
	Self RenderObject

	parent   RenderObject
	children []RenderObject

	offset Point
	size   Size

	needsLayout bool
	needsPaint  bool

	relayoutBoundary bool
	repaintBoundary  bool

	// Compositing bits (F04). alwaysNeedsCompositing forces needsCompositing.
	needsCompositing       bool
	alwaysNeedsCompositing bool

	// needsCompositingBitsUpdate marks this subtree for the R3b compositing-bits
	// flush (Flutter markNeedsCompositingBitsUpdate). Dirty structure/visibility
	// changes set it; PipelineOwner.UpdateCompositingBits clears it.
	needsCompositingBitsUpdate bool

	// cacheID is a stable BoundaryCache map key (assigned lazily).
	cacheID uint64

	// debugName is a stable hit-test identity tag (R13); empty = unnamed.
	debugName string

	// lastConstraints for ShouldRelayout early-out.
	lastConstraints Constraints
	hasLast         bool

	// owner optional pipeline for dirty scheduling.
	owner *PipelineOwner
}

// Init sets Self identity (call from constructor).
func (b *Base) Init(self RenderObject) {
	b.Self = self
	b.needsLayout = true
	b.needsPaint = true
	// Fresh nodes have no computed compositing bits yet: the first
	// UpdateCompositingBits flush must recompute this subtree (R3b).
	b.needsCompositingBitsUpdate = true
}

// SetOwner attaches a pipeline owner (for metrics / future schedule).
func (b *Base) SetOwner(o *PipelineOwner) { b.owner = o }

// Parent implements RenderObject.
func (b *Base) Parent() RenderObject { return b.parent }

// Children implements RenderObject.
func (b *Base) Children() []RenderObject { return b.children }

func (b *Base) setParent(p RenderObject) { b.parent = p }

// Size implements RenderObject.
func (b *Base) Size() Size { return b.size }

func (b *Base) setSize(s Size) { b.size = s }

// Offset implements RenderObject.
func (b *Base) Offset() Point { return b.offset }

// SetOffset implements RenderObject.
func (b *Base) SetOffset(p Point) { b.offset = p }

// NeedsLayout implements RenderObject.
func (b *Base) NeedsLayout() bool { return b.needsLayout }

// NeedsPaint implements RenderObject.
func (b *Base) NeedsPaint() bool { return b.needsPaint }

func (b *Base) clearLayoutDirty() { b.needsLayout = false }
func (b *Base) clearPaintDirty()  { b.needsPaint = false }

// IsRelayoutBoundary implements RenderObject.
func (b *Base) IsRelayoutBoundary() bool { return b.relayoutBoundary }

// IsRepaintBoundary implements RenderObject.
func (b *Base) IsRepaintBoundary() bool { return b.repaintBoundary }

// SetRelayoutBoundary marks layout isolation (F03).
func (b *Base) SetRelayoutBoundary(v bool) { b.relayoutBoundary = v }

// SetRepaintBoundary marks paint isolation (P2).
func (b *Base) SetRepaintBoundary(v bool) {
	if b.repaintBoundary == v {
		return
	}
	b.repaintBoundary = v
	b.MarkNeedsCompositingBitsUpdate()
}

// DebugName returns the hit-test identity tag (R13).
func (b *Base) DebugName() string {
	if b == nil {
		return ""
	}
	return b.debugName
}

// SetDebugName sets the hit-test identity tag (R13).
func (b *Base) SetDebugName(name string) {
	if b == nil {
		return
	}
	b.debugName = name
}

// HitDebugName returns n.DebugName() when n embeds Base, else "".
func HitDebugName(n RenderObject) string {
	if n == nil {
		return ""
	}
	if b, ok := baseOf(n); ok && b != nil {
		return b.debugName
	}
	return ""
}

// NeedsCompositing reports whether this node or descendants require a compositing layer.
func (b *Base) NeedsCompositing() bool { return b.needsCompositing }

// SetAlwaysNeedsCompositing forces compositing (opacity/transform/filter style).
func (b *Base) SetAlwaysNeedsCompositing(v bool) {
	if b.alwaysNeedsCompositing == v {
		return
	}
	b.alwaysNeedsCompositing = v
	if v {
		b.needsCompositing = true
	}
	b.MarkNeedsCompositingBitsUpdate()
}

// UpdateCompositingBits recomputes needsCompositing from children (F04 / R3b).
// Flutter semantics: needsCompositing is a bottom-up union — a node needs a
// compositing layer if it always does (opacity/transform/filter/boundary) OR
// any descendant does. Propagation skips clean subtrees (no needsCompositingBitsUpdate
// marker) so the flush stays O(changed), not O(tree) every frame.
// Returns whether this node needs compositing.
func (b *Base) UpdateCompositingBits() bool {
	need := b.alwaysNeedsCompositing || b.repaintBoundary
	// Recompute only when this subtree was marked dirty (or always) to keep the
	// R3b flush proportional to structural changes.
	if !b.needsCompositingBitsUpdate {
		return b.needsCompositing
	}
	b.needsCompositingBitsUpdate = false
	for _, ch := range b.children {
		if pb, ok := baseOf(ch); ok {
			if pb.needsCompositingBitsUpdate || pb.alwaysNeedsCompositing || pb.repaintBoundary {
				if pb.UpdateCompositingBits() {
					need = true
				}
			} else {
				// Clean child: keep its previously computed bit.
				need = need || pb.needsCompositing
			}
		} else if ch.IsRepaintBoundary() {
			need = true
		}
	}
	b.needsCompositing = need
	return need
}

// MarkNeedsCompositingBitsUpdate dirties this node and all ancestors up to the
// root for the next UpdateCompositingBits flush (Flutter markNeedsCompositingBitsUpdate).
// Call after structural / visibility-affecting changes (opacity, transform,
// filter, clip, boundary add/remove).
func (b *Base) MarkNeedsCompositingBitsUpdate() {
	if b == nil {
		return
	}
	b.needsCompositingBitsUpdate = true
	for p := b.parent; p != nil; p = p.Parent() {
		if pb, ok := baseOf(p); ok {
			pb.needsCompositingBitsUpdate = true
		}
	}
}

// NeedsCompositingBitsUpdate reports whether this subtree is dirty for the
// compositing-bits flush (diagnostics / R3b).
func (b *Base) NeedsCompositingBitsUpdate() bool {
	if b == nil {
		return false
	}
	return b.needsCompositingBitsUpdate
}

// AddChild appends a child and dirties layout + compositing bits.
func (b *Base) AddChild(child RenderObject) {
	if child == nil {
		return
	}
	b.children = append(b.children, child)
	// Parent must be the outer RenderObject (Self), not *Base.
	if b.Self != nil {
		child.setParent(b.Self)
	}
	b.MarkNeedsLayout()
	b.MarkNeedsCompositingBitsUpdate()
}

// RemoveChild detaches child if present.
func (b *Base) RemoveChild(child RenderObject) {
	if child == nil {
		return
	}
	for i, c := range b.children {
		if c == child {
			b.children = append(b.children[:i], b.children[i+1:]...)
			child.setParent(nil)
			b.MarkNeedsLayout()
			b.MarkNeedsCompositingBitsUpdate()
			return
		}
	}
}

func (b *Base) selfOr(fallback RenderObject) RenderObject {
	if b.Self != nil {
		return b.Self
	}
	return fallback
}

// MarkNeedsLayout dirties this node and ancestors until a relayout boundary
// (the boundary itself is marked, then stop). Always marks root path for owner.
func (b *Base) MarkNeedsLayout() {
	b.needsLayout = true
	b.needsPaint = true
	if b.owner != nil {
		b.owner.noteLayoutDirty()
	}
	// Bubble: always mark ancestors needsLayout; stop after marking a boundary parent chain.
	// Flutter: markNeedsLayout on a node inside a boundary dirties up to and including the boundary.
	for p := b.parent; p != nil; p = p.Parent() {
		// Access via interface — use type assert to Base when possible for flags.
		if pb, ok := baseOf(p); ok {
			pb.needsLayout = true
			pb.needsPaint = true
			if pb.owner != nil {
				pb.owner.noteLayoutDirty()
			}
			if pb.relayoutBoundary {
				break
			}
		} else {
			// Fallback: call MarkNeedsLayout would recurse infinitely; set via interface methods.
			// Non-Base nodes must implement mark themselves — P1 all nodes embed Base.
			break
		}
	}
}

// MarkNeedsPaint dirties paint. Flutter-aligned: a node's own paint dirt
// bubbles up through ancestors and STOPS at the nearest RepaintBoundary —
// the boundary itself is marked dirty (its subtree must re-record), but
// nothing above it is. A node that IS a repaint boundary only dirties itself.
//
// Rationale (Flutter PaintingContext): the RepaintBoundary is the isolation
// unit — repainting below it must not invalidate siblings or ancestors above.
// MarkNeedsPaint is idempotent: already-dirty nodes skip redundant marking.
func (b *Base) MarkNeedsPaint() {
	if b.needsPaint {
		return
	}
	b.needsPaint = true
	if b.owner != nil {
		b.owner.notePaintDirty()
	}
	if b.repaintBoundary {
		return
	}
	for p := b.parent; p != nil; p = p.Parent() {
		pb, ok := baseOf(p)
		if !ok {
			break
		}
		if pb.needsPaint {
			// Ancestor already dirty; if it is (or is above) a boundary the
			// remaining chain is already correct — stop walking.
			if pb.repaintBoundary {
				break
			}
			continue
		}
		pb.needsPaint = true
		if pb.owner != nil {
			pb.owner.notePaintDirty()
		}
		if pb.repaintBoundary {
			break
		}
	}
}

// NeedsCompositingOf reports the compositing bit of any render object (R3b
// diagnostics / real-window gates). False when the node has no Base.
func NeedsCompositingOf(n RenderObject) bool {
	if b, ok := baseOf(n); ok && b != nil {
		return b.needsCompositing
	}
	return false
}

func baseOf(n RenderObject) (*Base, bool) {
	type hasBase interface{ base() *Base }
	if h, ok := n.(hasBase); ok {
		return h.base(), true
	}
	// Embed Base in structs — method set includes promoted *Base methods but not base().
	// Use type switch on known types; for P1, RenderBox embeds Base.
	switch t := n.(type) {
	case *RenderBox:
		return &t.Base, true
	case *RenderColorBox:
		return &t.Base, true
	case *AbsoluteBox:
		return &t.Base, true
	case *RenderSpinner:
		return &t.Base, true
	case *RenderViewport:
		return &t.Base, true
	case *VirtualList:
		return &t.Base, true
	case *RenderText:
		return &t.Base, true
	case *RenderImage:
		return &t.Base, true
	default:
		return nil, false
	}
}

// RememberConstraints stores constraints after layout for early-out.
func (b *Base) RememberConstraints(c Constraints) {
	b.lastConstraints = c
	b.hasLast = true
}

// ShouldRelayout is true if dirty or constraints changed.
func (b *Base) ShouldRelayout(c Constraints) bool {
	if b.needsLayout {
		return true
	}
	if !b.hasLast || !b.lastConstraints.Equal(c) {
		return true
	}
	return false
}

// LayoutSkipIfClean returns previous size when layout can be skipped.
func (b *Base) LayoutSkipIfClean(c Constraints) (Size, bool) {
	if !b.ShouldRelayout(c) {
		return b.size, true
	}
	return Size{}, false
}
