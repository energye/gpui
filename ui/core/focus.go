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

// FocusNext moves focus to the next focusable node (wraps).
func (t *Tree) FocusNext() {
	if t == nil || t.root == nil {
		return
	}
	list := CollectFocusables(t.root)
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

// FocusPrev moves focus to the previous focusable node (wraps).
func (t *Tree) FocusPrev() {
	if t == nil || t.root == nil {
		return
	}
	list := CollectFocusables(t.root)
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
