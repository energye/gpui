package core

// Tree owns the root node, dirty state, pointer capture, and focus.
type Tree struct {
	root Node

	dirty bool

	// Pointer routing
	hover    Node
	capture  Node
	focus    Node
	lastDown Node

	// Viewport used for last layout (logical pixels).
	viewport Size
}

// NewTree creates a tree with the given root (may be nil).
func NewTree(root Node) *Tree {
	t := &Tree{dirty: true}
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

// markDirty flags that a frame phase is needed.
func (t *Tree) markDirty() {
	if t != nil {
		t.dirty = true
	}
}

// Dirty reports whether layout or paint is needed.
func (t *Tree) Dirty() bool {
	return t != nil && t.dirty
}

// Layout runs a single layout pass on the root with tight viewport constraints.
func (t *Tree) Layout(viewport Size) {
	if t == nil || t.root == nil {
		return
	}
	t.viewport = viewport
	_ = t.root.Layout(Tight(viewport.Width, viewport.Height))
	clearLayoutDirty(t.root)
}

// Paint paints the root into pc. Origin should typically be (0,0).
func (t *Tree) Paint(pc *PaintContext) {
	if t == nil || t.root == nil || pc == nil {
		return
	}
	t.root.Paint(pc)
	clearPaintDirty(t.root)
	t.dirty = false
}

// Frame runs layout then paint for the viewport.
func (t *Tree) Frame(pc *PaintContext, viewport Size) {
	t.Layout(viewport)
	if pc != nil {
		t.Paint(pc)
	}
}

// HitTest returns the deepest hit under absolute tree coordinates.
func (t *Tree) HitTest(p Point) Node {
	if t == nil || t.root == nil {
		return nil
	}
	return t.root.HitTest(p)
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
	if ev.Type == PointerUp || ev.Type == PointerCancel {
		if ev.Type == PointerUp && t.capture != nil && t.lastDown != nil {
			upHit := t.HitTest(p)
			if upHit == t.lastDown || t.capture == t.lastDown {
				if ch, ok := t.lastDown.(ClickHandler); ok {
					// Click is a separate callback; do not require !Handled.
					ch.OnClick(ev)
				}
			}
		}
		t.capture = nil
		if ev.Type == PointerCancel {
			t.lastDown = nil
		}
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

// FocusTarget is implemented by focusable primitives (Pressable, Focusable).
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
