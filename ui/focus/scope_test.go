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

import "testing"

// node registers a fresh enabled node in the manager (flat mode when
// scope is nil).
func reg(t *testing.T, m *FocusManager, label string) *FocusNode {
	t.Helper()
	n := NewFocusNode(label)
	m.Register(n)
	return n
}

// TestScope_TrapConfinesTab: a trapped modal scope cycles Tab inside its
// own nodes and never reaches the page behind it.
func TestScope_TrapConfinesTab(t *testing.T) {
	m := NewManager()
	root := NewScope(nil, "page")
	pageA := reg(t, m, "a")
	pageB := reg(t, m, "b")
	root.Add(pageA)
	root.Add(pageB)

	modal := NewScope(root, "modal")
	modal.Trap = true
	mA := reg(t, m, "mA")
	mB := reg(t, m, "mB")
	modal.Add(mA)
	modal.Add(mB)

	m.RequestFocus(mA)
	if got := m.FocusNext(); got != mB {
		t.Fatalf("first Tab inside trap = %v, want mB", got.DebugLabel)
	}
	if got := m.FocusNext(); got != mA {
		t.Fatalf("second Tab must cycle back to mA, got %v", got.DebugLabel)
	}
	if got := m.FocusPrevious(); got != mB {
		t.Fatalf("reverse Tab must wrap to mB, got %v", got.DebugLabel)
	}
}

// TestScope_UntrappedFloatsUp: a non-trapped scope's traversal is
// manager-wide — Tab at its tail moves to the parent-level nodes.
func TestScope_UntrappedFloatsUp(t *testing.T) {
	m := NewManager()
	root := NewScope(nil, "page")
	pageA := reg(t, m, "a")
	root.Add(pageA)

	panel := NewScope(root, "panel") // no Trap
	panelA := reg(t, m, "pA")
	panel.Add(panelA)

	m.RequestFocus(panelA)
	if got := m.FocusNext(); got != pageA {
		t.Fatalf("untrapped scope Tab must float to parent-level node, got %v", got.DebugLabel)
	}
	if got := m.FocusNext(); got != panelA {
		t.Fatalf("wrap must return to panel node, got %v", got.DebugLabel)
	}
}

// TestScope_NestedTrapKeepsInner: a trap inside a trap confines to the
// innermost trapped scope (nearest trap wins).
func TestScope_NestedTrapKeepsInner(t *testing.T) {
	m := NewManager()
	root := NewScope(nil, "root")
	outer := NewScope(root, "outer")
	outer.Trap = true
	inner := NewScope(outer, "inner")
	inner.Trap = true

	o := reg(t, m, "o")
	i := reg(t, m, "i")
	outer.Add(o)
	inner.Add(i)

	m.RequestFocus(i)
	if got := m.FocusNext(); got != i {
		t.Fatalf("inner trap must confine Tab to itself, got %v", got.DebugLabel)
	}
}

// TestScope_DetachFocusFallsBack: destroying the scope holding primary
// blurs (the safe floor) and unregisters its nodes.
func TestScope_DetachFocusFallsBack(t *testing.T) {
	m := NewManager()
	root := NewScope(nil, "page")
	a := reg(t, m, "a")
	root.Add(a)

	modal := NewScope(root, "modal")
	modal.Trap = true
	mA := reg(t, m, "mA")
	modal.Add(mA)
	m.RequestFocus(mA)

	modal.Detach()
	if m.Primary() != nil {
		t.Fatal("detach of scope holding primary must blur")
	}
	if m.Count() != 1 {
		t.Fatalf("node count after detach = %d, want 1 (only page node survives)", m.Count())
	}
	if mA.mgr != nil || mA.scope != nil {
		t.Fatal("detached node must be fully unregistered")
	}
	// Focus can restart on the surviving page node.
	if !m.RequestFocus(a) {
		t.Fatal("surviving node cannot take focus after modal detach")
	}
}

// TestScope_MoveNodeReparents: adding a node to another scope moves it
// (no double membership).
func TestScope_MoveNodeReparents(t *testing.T) {
	m := NewManager()
	s1 := NewScope(nil, "s1")
	s2 := NewScope(nil, "s2")
	n := reg(t, m, "n")
	s1.Add(n)
	s2.Add(n)
	if len(s1.Nodes) != 0 {
		t.Fatalf("s1 still holds %d nodes after re-add", len(s1.Nodes))
	}
	if len(s2.Nodes) != 1 || s2.Nodes[0] != n {
		t.Fatal("s2 does not hold the node exactly once")
	}
}

// TestFlatTraversalUnchanged: the plan's regression leg — nodes without
// any scope keep the legacy manager-wide order byte-for-byte.
func TestFlatTraversalUnchanged(t *testing.T) {
	m := NewManager()
	a := reg(t, m, "a")
	b := reg(t, m, "b")
	c := reg(t, m, "c")

	m.RequestFocus(a)
	if got := m.FocusNext(); got != b {
		t.Fatalf("flat Tab = %v, want b", got.DebugLabel)
	}
	if got := m.FocusNext(); got != c {
		t.Fatalf("flat Tab = %v, want c", got.DebugLabel)
	}
	if got := m.FocusNext(); got != a {
		t.Fatalf("flat wrap = %v, want a", got.DebugLabel)
	}
}

// TestFlatScopeAndTraversalMix: scoped and flat nodes coexist; a flat
// primary traverses everything (no trap above it).
func TestFlatScopeAndTraversalMix(t *testing.T) {
	m := NewManager()
	s := NewScope(nil, "s")
	scopedN := reg(t, m, "scoped")
	s.Add(scopedN)
	flatN := reg(t, m, "flat")

	m.RequestFocus(flatN)
	if got := m.FocusNext(); got != scopedN {
		t.Fatalf("flat primary must traverse into scoped nodes, got %v", got.DebugLabel)
	}
}
