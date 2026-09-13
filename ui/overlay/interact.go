package overlay

import (
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
)

// HitOutside reports whether p lands outside every overlay entry
// (no entry consumes it). Callers with outside-close semantics close
// their popup when this returns true on pointer-down.
func HitOutside(s *State, p rendering.Point) bool {
	if s == nil || s.Len() == 0 {
		return false
	}
	return !s.HitTest(p).Consumed
}

// FocusLock traps keyboard focus inside an overlay while it is open.
//
// It reuses focus.Manager: Acquire remembers the previous primary and moves
// focus to the first overlay node; Release restores the saved node.
// Tab cycling itself stays in the manager; Contains lets callers reject
// focus escapes without adding a second focus system.
type FocusLock struct {
	mgr   *focus.FocusManager
	nodes []*focus.FocusNode
	saved *focus.FocusNode
	held  bool
}

// NewFocusLock builds a lock over the overlay-owned focus nodes.
func NewFocusLock(mgr *focus.FocusManager, nodes ...*focus.FocusNode) *FocusLock {
	return &FocusLock{mgr: mgr, nodes: nodes}
}

// Acquire holds the lock and focuses the first node. No-op when mgr is nil.
func (l *FocusLock) Acquire() {
	if l == nil || l.mgr == nil || l.held {
		return
	}
	l.saved = l.mgr.Primary()
	l.held = true
	for _, n := range l.nodes {
		if n != nil && n.CanFocus() {
			l.mgr.RequestFocus(n)
			return
		}
	}
}

// Release drops the lock and restores the pre-acquire focus.
func (l *FocusLock) Release() {
	if l == nil || l.mgr == nil || !l.held {
		return
	}
	l.held = false
	if l.saved != nil && l.saved.CanFocus() {
		l.mgr.RequestFocus(l.saved)
	} else {
		l.mgr.Blur()
	}
	l.saved = nil
}

// Contains reports whether n belongs to this lock.
func (l *FocusLock) Contains(n *focus.FocusNode) bool {
	if l == nil || n == nil {
		return false
	}
	for _, x := range l.nodes {
		if x == n {
			return true
		}
	}
	return false
}

// Held reports whether the lock is acquired.
func (l *FocusLock) Held() bool {
	return l != nil && l.held
}
