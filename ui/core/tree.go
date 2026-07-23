package core

// Tree owns the root node, dirty state, pointer capture, focus, and overlays.
type Tree struct {
	root Node

	dirty bool

	// fullPaintRequired forces the next Frame to fully repaint (resize, expose, first frame).
	fullPaintRequired bool

	// Pointer routing
	hover    Node
	capture  Node
	focus    Node
	lastDown Node

	// Viewport used for last layout (logical pixels).
	viewport Size

	// Overlay stack (portal host).
	overlays *OverlayHost

	// clock drives Motion/Presence (C-Motion).
	clock *Clock

	// tickers drive demand-mode animation (gogpu ANIMATING).
	tickers []tickerEntry

	// onDirty optional wakeup when markDirty flips dirty→true or re-dirties.
	onDirty func()

	// onCursor optional host callback when hover cursor should change.
	onCursor   func(CursorKind)
	lastCursor CursorKind
	hasCursor  bool
}

// NewTree creates a tree with the given root (may be nil).
func NewTree(root Node) *Tree {
	t := &Tree{dirty: true, fullPaintRequired: true, overlays: NewOverlayHost()}
	if root != nil {
		t.SetRoot(root)
	}
	return t
}

// Root returns the root node.
func (t *Tree) Root() Node {
	if t == nil {
		return nil
	}
	return t.root
}

// Overlays returns the portal host (never nil after NewTree).
func (t *Tree) Overlays() *OverlayHost {
	if t == nil {
		return nil
	}
	if t.overlays == nil {
		t.overlays = NewOverlayHost()
	}
	return t.overlays
}

// AttachSubtree mounts n (and descendants) onto this tree so MarkNeedsPaint
// dirties frames. Used for OverlayPortal content which is not in the root hierarchy.
func (t *Tree) AttachSubtree(n Node) {
	if t == nil || n == nil {
		return
	}
	mountNode(n, t)
}

// DetachSubtree unmounts n (and descendants) from this tree.
func (t *Tree) DetachSubtree(n Node) {
	if t == nil || n == nil {
		return
	}
	unmountNode(n)
}

// SetRoot replaces the root and remounts.
func (t *Tree) SetRoot(root Node) {
	if t.root != nil {
		unmountNode(t.root)
	}
	t.root = root
	t.hover = nil
	t.capture = nil
	t.focus = nil
	t.lastDown = nil
	if root != nil {
		mountNode(root, t)
	}
	t.markDirty()
}

// Viewport returns the last layout viewport.
func (t *Tree) Viewport() Size { return t.viewport }

// markDirty flags that a frame phase is needed and notifies onDirty.
func (t *Tree) markDirty() {
	if t == nil {
		return
	}
	t.dirty = true
	if t.onDirty != nil {
		t.onDirty()
	}
}

// MarkDirty requests a layout/paint frame (for portals/setState).
func (t *Tree) MarkDirty() { t.markDirty() }

// MarkFullPaintRequired forces the next Frame to repaint the whole tree
// (resize, expose, device lost). Clears after a successful full Paint.
func (t *Tree) MarkFullPaintRequired() {
	if t == nil {
		return
	}
	t.fullPaintRequired = true
	t.markDirty()
}

// NonBoundaryPaintDirty reports whether any paint-dirty node sits outside a
// RepaintBoundary. When false, only boundary layers need re-rasterize and the
// compositor may keep the base RT (single Spin/Skeleton demand path).
func (t *Tree) NonBoundaryPaintDirty() bool {
	if t == nil {
		return false
	}
	var walk func(n Node, underBoundary bool) bool
	walk = func(n Node, underBoundary bool) bool {
		if n == nil {
			return false
		}
		b := n.Base()
		ub := underBoundary || b.IsRepaintBoundary()
		if b.NeedsPaint() && !ub {
			return true
		}
		for _, c := range b.Children() {
			if walk(c, ub) {
				return true
			}
		}
		return false
	}
	if walk(t.root, false) {
		return true
	}
	for _, e := range t.Overlays().Entries() {
		if walk(e.Node, false) {
			return true
		}
	}
	return false
}

// FullPaintRequired reports whether the next paint must be a full tree paint.
func (t *Tree) FullPaintRequired() bool {
	return t != nil && t.fullPaintRequired
}

// Dirty reports whether layout or paint is needed.
func (t *Tree) Dirty() bool {
	return t != nil && t.dirty
}

// ClearDirty clears the tree-level dirty bit without painting.
// Prefer Paint (which clears after a successful pass). Used by tests/schedulers.
func (t *Tree) ClearDirty() {
	if t != nil {
		t.dirty = false
	}
}

// Layout runs a single layout pass on the root with tight viewport constraints,
// then lays out overlay nodes loosely within the viewport.
// Clean nodes with identical constraints early-out inside each Layout impl via
// NodeBase.LayoutSkipIfClean / ShouldRelayout.
func (t *Tree) Layout(viewport Size) {
	if t == nil {
		return
	}
	t.viewport = viewport
	if t.root != nil {
		_ = t.root.Layout(Tight(viewport.Width, viewport.Height))
	}
	for _, e := range t.Overlays().Entries() {
		if e.Node == nil {
			continue
		}
		_ = e.Node.Layout(Loose(viewport.Width, viewport.Height))
	}
}

// Paint paints the root then overlays (ascending Z).
// When FullPaintRequired is false and pc.CompositeOnly is set by the host,
// clean non-boundary subtrees are skipped (Flutter retained layer path).
func (t *Tree) Paint(pc *PaintContext) {
	if t == nil || pc == nil {
		return
	}
	full := t.fullPaintRequired
	if full {
		pc.CompositeOnly = false
		pc.ForceFullPaint = true
	}
	if t.root != nil {
		t.root.Paint(pc)
		clearPaintDirty(t.root)
	}
	for _, e := range t.Overlays().Entries() {
		if e.Node == nil {
			continue
		}
		// Overlay nodes use their own Offset as absolute position.
		childPC := pc.WithOrigin(pc.Origin.Add(e.Node.Base().Offset()))
		e.Node.Paint(childPC)
		clearPaintDirty(e.Node)
	}
	t.dirty = false
	if full {
		t.fullPaintRequired = false
	}
}

// Frame runs layout (when needed) then paint for the viewport.
//
// Layout is skipped when the viewport is unchanged and no node has needsLayout.
// Paint-only frames (Spin angle, Skeleton shimmer) must not remeasure the tree.
// Callers still gate OnDraw on Dirty()/NeedsFrame().
func (t *Tree) Frame(pc *PaintContext, viewport Size) {
	if t == nil {
		return
	}
	if t.needsLayoutPass(viewport) {
		t.Layout(viewport)
	} else {
		t.viewport = viewport
	}
	if pc != nil {
		t.Paint(pc)
	}
}

func (t *Tree) needsLayoutPass(viewport Size) bool {
	if t == nil {
		return false
	}
	if t.viewport != viewport {
		return true
	}
	// Thumb drag freezes geometry. Animating widgets (Switch MarkNeedsLayout every
	// tick, Progress width, …) must not remeasure the tree mid-drag — that shifts
	// AbsoluteBounds of the scroll rail and the thumb appears to jump vs the pointer.
	if d, ok := t.capture.(interface{ Dragging() bool }); ok && d.Dragging() {
		return false
	}
	if t.root != nil && t.root.Base().NeedsLayout() {
		return true
	}
	for _, e := range t.Overlays().Entries() {
		if e.Node != nil && e.Node.Base().NeedsLayout() {
			return true
		}
	}
	return false
}

// CollectPaintDamage returns absolute logical rects for nodes that need paint.
// Used by hosts to Invalidate / PresentFrameAuto. Empty when full paint is required.
func (t *Tree) CollectPaintDamage() []Rect {
	if t == nil || t.fullPaintRequired {
		return nil
	}
	var out []Rect
	var walk func(n Node)
	walk = func(n Node) {
		if n == nil {
			return
		}
		b := n.Base()
		if b.needsPaint {
			r := AbsoluteBounds(n)
			if !r.Empty() {
				out = append(out, r)
			}
			// Still walk children: nested dirty regions refine multi-rect present.
		}
		for _, c := range b.children {
			walk(c)
		}
	}
	walk(t.root)
	for _, e := range t.Overlays().Entries() {
		walk(e.Node)
	}
	return out
}

// FrameIfNeeded runs Frame only when Dirty() is true.
// Active tickers alone do not paint — tickers must MarkDirty/MarkNeedsPaint
// when visual state changes (gogpu ANIMATING: onUpdate every tick, OnDraw on RequestRedraw).
func (t *Tree) FrameIfNeeded(pc *PaintContext, viewport Size) bool {
	if t == nil || !t.Dirty() {
		return false
	}
	t.Frame(pc, viewport)
	return true
}

// HitTest returns the deepest hit under absolute tree coordinates.
// Overlays are tested top-most first.
func (t *Tree) HitTest(p Point) Node {
	if t == nil {
		return nil
	}
	entries := t.Overlays().Entries()
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Node == nil {
			continue
		}
		off := e.Node.Base().Offset()
		local := p.Sub(off)
		if hit := e.Node.HitTest(local); hit != nil {
			return hit
		}
	}
	if t.root == nil {
		return nil
	}
	return t.root.HitTest(p)
}

// SetOnCursor registers a host callback for mouse cursor changes.
func (t *Tree) SetOnCursor(fn func(CursorKind)) {
	if t == nil {
		return
	}
	t.onCursor = fn
}

func (t *Tree) applyCursor(n Node) {
	if t == nil || t.onCursor == nil {
		return
	}
	k := ResolveCursor(n)
	if t.hasCursor && t.lastCursor == k {
		return
	}
	t.lastCursor = k
	t.hasCursor = true
	t.onCursor(k)
}

// DispatchPointer routes a pointer event: capture → target → bubble.
func (t *Tree) DispatchPointer(ev *PointerEvent) {
	if t == nil || ev == nil {
		return
	}
	p := ev.Pos()

	var target Node
	if t.capture != nil {
		target = t.capture
	} else {
		target = t.HitTest(p)
	}
	ev.Target = target

	// Hover enter/leave (Move only when not captured for simplicity).
	if ev.Type == PointerMove && t.capture == nil {
		if target != t.hover {
			if t.hover != nil {
				if h, ok := t.hover.(hoverable); ok {
					h.SetHovered(false)
				}
			}
			t.hover = target
			if t.hover != nil {
				if h, ok := t.hover.(hoverable); ok {
					h.SetHovered(true)
				}
			}
			t.applyCursor(target)
		}
	}

	switch ev.Type {
	case PointerDown:
		t.lastDown = target
		if target != nil {
			t.capture = target
			if f, ok := target.(FocusTarget); ok && f.CanFocus() {
				t.SetFocus(target)
			}
		}
	}

	// Deliver to target then bubble (pressed/hover state updates first).
	for n := target; n != nil && !ev.Handled; n = n.Parent() {
		if ph, ok := n.(PointerHandler); ok {
			ph.HandlePointer(ev)
		}
	}

	// Click synthesis after handlers so Pressable can clear Pressed on Up.
	// Ant/HTML: fire only if pointer-up is still over the same target (or a
	// descendant). Release outside the control cancels the click.
	// Note: do NOT treat capture==lastDown as success — capture is always the
	// down target until cleared, which incorrectly fired click on outside-up.
	if ev.Type == PointerUp || ev.Type == PointerCancel {
		if ev.Type == PointerUp && t.lastDown != nil {
			upHit := t.HitTest(p)
			if sameOrDescendant(upHit, t.lastDown) {
				if ch, ok := t.lastDown.(ClickHandler); ok {
					ch.OnClick(ev)
				}
			}
		}
		t.capture = nil
		t.lastDown = nil
	}
}

// SetFocus moves keyboard focus.
func (t *Tree) SetFocus(n Node) {
	if t == nil {
		return
	}
	if t.focus == n {
		return
	}
	if t.focus != nil {
		if f, ok := t.focus.(FocusTarget); ok {
			f.SetFocused(false)
		}
	}
	t.focus = n
	if t.focus != nil {
		if f, ok := t.focus.(FocusTarget); ok {
			f.SetFocused(true)
		}
	}
}

// Focus returns the focused node.
func (t *Tree) Focus() Node {
	if t == nil {
		return nil
	}
	return t.focus
}

// DispatchKey delivers a key event to the focused node (bubble to root).
// Unhandled Tab / Shift+Tab traverse focusables.
func (t *Tree) DispatchKey(ev *KeyEvent) {
	if t == nil || ev == nil {
		return
	}
	for n := t.focus; n != nil && !ev.Handled; n = n.Parent() {
		if kh, ok := n.(KeyHandler); ok {
			kh.HandleKey(ev)
		}
	}
	if ev.Handled || ev.Type != KeyDown {
		return
	}
	switch ev.Key {
	case "Shift+Tab", "ISO_Left_Tab":
		t.FocusPrev()
		ev.Handled = true
	case "Tab":
		t.FocusNext()
		ev.Handled = true
	}
}

// DispatchScroll delivers a wheel event: hit-test then bubble for ScrollHandler.
func (t *Tree) DispatchScroll(ev *ScrollEvent) {
	if t == nil || ev == nil {
		return
	}
	target := t.HitTest(Point{X: ev.X, Y: ev.Y})
	for n := target; n != nil && !ev.Handled; n = n.Parent() {
		if sh, ok := n.(ScrollHandler); ok {
			sh.HandleScroll(ev)
		}
	}
}

// DispatchTextInput delivers committed text to the focused editor.
func (t *Tree) DispatchTextInput(ev *TextInputEvent) {
	if t == nil || ev == nil {
		return
	}
	for n := t.focus; n != nil && !ev.Handled; n = n.Parent() {
		if th, ok := n.(TextInputHandler); ok {
			th.HandleTextInput(ev)
		}
	}
}

// DispatchIME delivers composition events to the focused editor.
func (t *Tree) DispatchIME(ev *IMECompositionEvent) {
	if t == nil || ev == nil {
		return
	}
	for n := t.focus; n != nil && !ev.Handled; n = n.Parent() {
		if ih, ok := n.(IMEHandler); ok {
			ih.HandleIME(ev)
		}
	}
}

// Capture returns the current pointer capture node.
func (t *Tree) Capture() Node {
	if t == nil {
		return nil
	}
	return t.capture
}

// Hover returns the current hover node.
func (t *Tree) Hover() Node {
	if t == nil {
		return nil
	}
	return t.hover
}

type hoverable interface {
	SetHovered(bool)
}

// FocusTarget is implemented by focusable primitives (Pressable, Focusable, EditableText).
// Methods must be exported so cross-package types can satisfy the interface.
type FocusTarget interface {
	CanFocus() bool
	SetFocused(bool)
}

func clearLayoutDirty(n Node) {
	if n == nil {
		return
	}
	b := n.Base()
	b.needsLayout = false
	for _, c := range b.children {
		clearLayoutDirty(c)
	}
}

func clearPaintDirty(n Node) {
	if n == nil {
		return
	}
	b := n.Base()
	b.needsPaint = false
	for _, c := range b.children {
		clearPaintDirty(c)
	}
}

// sameOrDescendant reports whether n is ancestor or n itself.
func sameOrDescendant(n, ancestor Node) bool {
	for x := n; x != nil; x = x.Parent() {
		if x == ancestor {
			return true
		}
	}
	return false
}
