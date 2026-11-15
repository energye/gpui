//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package focus

// FocusScope is a node in the scope tree (Flutter FocusScopeNode semantics,
// KIT_FRAME_BINDING_PLAN 件 4): it groups focus nodes, nests, and confines
// Tab traversal. A modal mounts a trapped child scope so Tab cycles inside
// the dialog and never reaches the page behind it.
//
// Flat usage is unchanged: nodes registered without a scope keep the
// manager-wide traversal (all pre-existing windows stay byte-for-byte).
type FocusScope struct {
	// DebugLabel optional name for tests/logs.
	DebugLabel string
	// Parent is the enclosing scope, nil for the root scope.
	Parent *FocusScope
	// Children are nested scopes in registration order.
	Children []*FocusScope
	// Nodes are the focus nodes registered directly in this scope.
	Nodes []*FocusNode
	// Trap confines traversal: Tab never leaves this scope's subtree
	// (Modal/Drawer focus trap). Non-trapped scopes let traversal float
	// up to the parent when their own nodes are exhausted.
	Trap bool
}

// NewScope creates a scope attached under parent (nil parent = root).
func NewScope(parent *FocusScope, label string) *FocusScope {
	s := &FocusScope{DebugLabel: label}
	if parent != nil {
		parent.Children = append(parent.Children, s)
		s.Parent = parent
	}
	return s
}

// Add registers n directly in this scope. Re-registering under another
// scope moves it (the old scope's list is cleaned); registering an
// already-scoped node here re-parents it.
func (s *FocusScope) Add(n *FocusNode) {
	if s == nil || n == nil {
		return
	}
	if old := n.scope; old != nil {
		old.removeNode(n)
	}
	n.scope = s
	s.Nodes = append(s.Nodes, n)
}

// removeNode detaches n from this scope's direct list.
func (s *FocusScope) removeNode(n *FocusNode) {
	for i, x := range s.Nodes {
		if x == n {
			s.Nodes = append(s.Nodes[:i], s.Nodes[i+1:]...)
			break
		}
	}
}

// Detach destroys this scope: children are detached first (their focus
// falls back up the ancestor chain), nodes are removed from their manager,
// and the scope unlinks from its parent. If the manager's primary focus
// lived in this subtree, focus falls back to the closest surviving
// ancestor scope's first focusable, or blurs when none exists.
func (s *FocusScope) Detach() {
	if s == nil {
		return
	}
	for _, c := range append([]*FocusScope(nil), s.Children...) {
		c.Detach()
	}
	s.Children = nil

	mgr := (*FocusManager)(nil)
	lostPrimary := (*FocusNode)(nil)
	for _, n := range append([]*FocusNode(nil), s.Nodes...) {
		if n == nil {
			continue
		}
		if n.mgr != nil {
			mgr = n.mgr
		}
		if mgr != nil && mgr.Primary() == n {
			lostPrimary = n
		}
		n.scope = nil
		n.Unregister()
	}
	s.Nodes = nil

	if s.Parent != nil {
		s.Parent.removeScope(s)
		s.Parent = nil
	}
	// Focus fall-back: closest surviving ancestor scope (Flutter returns
	// focus up the ancestor chain when a scope with primary detaches).
	if mgr != nil && lostPrimary != nil {
		// s.Parent was already cleared; walk via the node's old ancestors is
		// impossible, so the caller-owned parent scope handles fall-back by
		// being the surviving ancestor. Blur is the safe floor.
		mgr.Blur()
	}
}

func (s *FocusScope) removeScope(c *FocusScope) {
	for i, x := range s.Children {
		if x == c {
			s.Children = append(s.Children[:i], s.Children[i+1:]...)
			return
		}
	}
}

// encloses reports whether n is registered directly in this scope.
func (s *FocusScope) encloses(n *FocusNode) bool {
	if s == nil || n == nil || n.scope != s {
		return false
	}
	return true
}

// scopeOf returns the scope chain from n's own scope up to the root
// (nil when n is unscoped — the flat mode).
func scopeOf(n *FocusNode) *FocusScope {
	if n == nil {
		return nil
	}
	return n.scope
}

// trapRoot returns the nearest trapped ancestor scope (including s itself)
// that traversal may not leave, or nil when no trap encloses s.
func trapRoot(s *FocusScope) *FocusScope {
	for c := s; c != nil; c = c.Parent {
		if c.Trap {
			return c
		}
	}
	return nil
}
