package focus

// FocusNode is a focusable entity (not necessarily a RenderObject).
//
// Widgets hold a FocusNode and register it with FocusManager. Detach/dispose
// must Unregister to avoid a dangling primary.
type FocusNode struct {
	// DebugLabel optional name for tests/logs.
	DebugLabel string

	// Enabled when false: skipped in Tab order and cannot become primary.
	Enabled bool

	// TabIndex: <0 skip traversal; 0 = natural (registration) order;
	// >0 = explicit order among positive indices (stable by index then regSeq).
	TabIndex int

	// OnKey is invoked for key events when this node is primary.
	// Return true if the key was consumed (stops further default handling
	// except Tab which is handled by the manager before OnKey when navigating).
	OnKey func(e KeyEvent) bool

	// OnFocusChange is called when focus is gained (true) or lost (false).
	// Typical use: MarkNeedsPaint on a repaint boundary — must not force full-tree layout.
	OnFocusChange func(focused bool)

	// OnActivate optional Space/Enter when OnKey is nil or does not consume.
	OnActivate func()

	// Target is the widget/control this node represents (optional payload).
	// The framework reads it to route capability flows — e.g. an
	// embedder.InputRouter resolves the focused control by type-asserting
	// Target to TextEditTarget for IME session management (plan I4/I5).
	Target any

	mgr    *FocusManager
	regSeq int // registration sequence for stable order
}

// NewFocusNode creates an enabled focus node.
func NewFocusNode(label string) *FocusNode {
	return &FocusNode{DebugLabel: label, Enabled: true}
}

// HasFocus reports whether this node is the manager's primary focus.
func (n *FocusNode) HasFocus() bool {
	if n == nil || n.mgr == nil {
		return false
	}
	return n.mgr.Primary() == n
}

// RequestFocus asks the manager to make this node primary.
func (n *FocusNode) RequestFocus() bool {
	if n == nil || n.mgr == nil {
		return false
	}
	return n.mgr.RequestFocus(n)
}

// Unfocus clears primary if this node holds it.
func (n *FocusNode) Unfocus() {
	if n == nil || n.mgr == nil {
		return
	}
	if n.mgr.Primary() == n {
		n.mgr.Blur()
	}
}

// Unregister removes this node from its manager (safe if unregistered).
func (n *FocusNode) Unregister() {
	if n == nil || n.mgr == nil {
		return
	}
	n.mgr.Unregister(n)
}

// CanFocus reports whether the node may receive focus.
func (n *FocusNode) CanFocus() bool {
	return n != nil && n.Enabled && n.TabIndex >= 0 && n.mgr != nil
}
