package core

// CollectFocusables returns FocusTarget nodes in tree order (pre-order DFS).
func CollectFocusables(root Node) []Node {
	var out []Node
	var walk func(Node)
	walk = func(n Node) {
		if n == nil {
			return
		}
		if f, ok := n.(FocusTarget); ok && f.CanFocus() {
			out = append(out, n)
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	return out
}

// ActiveFocusScope is implemented by primitive.FocusScope (Active trap root).
type ActiveFocusScope interface {
	Node
	// FocusTrapActive reports whether Tab must stay inside this subtree.
	FocusTrapActive() bool
}

// focusRootForTraversal returns the active focus trap containing focus, or tree root.
func (t *Tree) focusRootForTraversal() Node {
	if t == nil {
		return nil
	}
	if t.focus != nil {
		for x := t.focus; x != nil; x = x.Parent() {
			if s, ok := x.(ActiveFocusScope); ok && s.FocusTrapActive() {
				return x
			}
		}
	}
	// Overlay portals: active trap may not be under focus yet — scan overlays.
	for _, e := range t.Overlays().Entries() {
		if e.Node == nil {
			continue
		}
		if s := findActiveFocusScope(e.Node); s != nil {
			return s
		}
	}
	return t.root
}

func findActiveFocusScope(n Node) Node {
	if n == nil {
		return nil
	}
	if s, ok := n.(ActiveFocusScope); ok && s.FocusTrapActive() {
		return n
	}
	for _, c := range n.Children() {
		if s := findActiveFocusScope(c); s != nil {
			return s
		}
	}
	return nil
}

// FocusNext moves focus to the next focusable node (wraps within active trap).
func (t *Tree) FocusNext() {
	if t == nil {
		return
	}
	root := t.focusRootForTraversal()
	if root == nil {
		return
	}
	list := CollectFocusables(root)
	if len(list) == 0 {
		return
	}
	idx := -1
	for i, n := range list {
		if n == t.focus {
			idx = i
			break
		}
	}
	next := list[0]
	if idx >= 0 && idx+1 < len(list) {
		next = list[idx+1]
	} else if idx >= 0 {
		next = list[0]
	}
	t.SetFocus(next)
}

// FocusPrev moves focus to the previous focusable node (wraps within active trap).
func (t *Tree) FocusPrev() {
	if t == nil {
		return
	}
	root := t.focusRootForTraversal()
	if root == nil {
		return
	}
	list := CollectFocusables(root)
	if len(list) == 0 {
		return
	}
	idx := -1
	for i, n := range list {
		if n == t.focus {
			idx = i
			break
		}
	}
	prev := list[len(list)-1]
	if idx > 0 {
		prev = list[idx-1]
	} else if idx == 0 {
		prev = list[len(list)-1]
	}
	t.SetFocus(prev)
}

// HandleTabKey focuses next/prev on Tab / Shift+Tab.
// Returns true if handled.
func (t *Tree) HandleTabKey(key string, shift bool) bool {
	if key != "Tab" {
		return false
	}
	if shift {
		t.FocusPrev()
	} else {
		t.FocusNext()
	}
	return true
}
