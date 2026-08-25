package focus

import "sort"

// FocusManager owns the primary focus and the focusable set for one scope
// (typically one window).
type FocusManager struct {
	primary   *FocusNode
	nodes     []*FocusNode
	nextSeq   int
	shiftDown bool

	// focusChanges counts primary transitions (tests / metrics).
	focusChanges int
	// paintHints counts OnFocusChange invocations (gain+loss).
	paintHints int

	// observers are notified on every primary transition (after state
	// update). Add via AddFocusObserver; used by the embedder's IME session
	// management (I4) — keep callbacks fast, they run on the UI thread.
	observers []func(from, to *FocusNode)
}

// NewManager creates an empty focus manager.
func NewManager() *FocusManager {
	return &FocusManager{}
}

// Primary returns the current focus node, or nil.
func (m *FocusManager) Primary() *FocusNode {
	if m == nil {
		return nil
	}
	return m.primary
}

// FocusChanges returns how many times primary changed (including blur-to-nil).
func (m *FocusManager) FocusChanges() int {
	if m == nil {
		return 0
	}
	return m.focusChanges
}

// PaintHints returns OnFocusChange call count (for C5 dirty-path tests).
func (m *FocusManager) PaintHints() int {
	if m == nil {
		return 0
	}
	return m.paintHints
}

// Register adds n to the focusable set. Idempotent if already registered here.
func (m *FocusManager) Register(n *FocusNode) {
	if m == nil || n == nil {
		return
	}
	if n.mgr == m {
		return
	}
	if n.mgr != nil {
		n.mgr.Unregister(n)
	}
	n.mgr = m
	m.nextSeq++
	n.regSeq = m.nextSeq
	m.nodes = append(m.nodes, n)
}

// Unregister removes n. If it was primary, blurs first.
func (m *FocusManager) Unregister(n *FocusNode) {
	if m == nil || n == nil || n.mgr != m {
		return
	}
	if m.primary == n {
		m.Blur()
	}
	for i, x := range m.nodes {
		if x == n {
			m.nodes = append(m.nodes[:i], m.nodes[i+1:]...)
			break
		}
	}
	n.mgr = nil
	n.regSeq = 0
}

// RequestFocus makes n the primary focus. Returns false if n cannot focus.
func (m *FocusManager) RequestFocus(n *FocusNode) bool {
	if m == nil || n == nil || n.mgr != m || !n.Enabled || n.TabIndex < 0 {
		return false
	}
	if m.primary == n {
		return true
	}
	m.setPrimary(n)
	return true
}

// Blur clears primary focus.
func (m *FocusManager) Blur() {
	if m == nil || m.primary == nil {
		return
	}
	m.setPrimary(nil)
}

// AddFocusObserver registers fn to run after every primary transition with
// (previous, current) nodes (either may be nil). Idempotent adds are NOT
// deduplicated — register once at wiring time.
func (m *FocusManager) AddFocusObserver(fn func(from, to *FocusNode)) {
	if m == nil || fn == nil {
		return
	}
	m.observers = append(m.observers, fn)
}

func (m *FocusManager) notifyObservers(from, to *FocusNode) {
	for _, fn := range m.observers {
		fn(from, to)
	}
}

func (m *FocusManager) setPrimary(n *FocusNode) {
	old := m.primary
	if old == n {
		return
	}
	m.primary = n
	m.focusChanges++
	if old != nil {
		m.paintHints++
		if old.OnFocusChange != nil {
			old.OnFocusChange(false)
		}
	}
	if n != nil {
		m.paintHints++
		if n.OnFocusChange != nil {
			n.OnFocusChange(true)
		}
	}
	m.notifyObservers(old, n)
}

// Count returns registered node count.
func (m *FocusManager) Count() int {
	if m == nil {
		return 0
	}
	return len(m.nodes)
}

// focusables returns enabled, non-negative TabIndex nodes in traversal order.
func (m *FocusManager) focusables() []*FocusNode {
	if m == nil {
		return nil
	}
	var list []*FocusNode
	for _, n := range m.nodes {
		if n != nil && n.Enabled && n.TabIndex >= 0 {
			list = append(list, n)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		// Positive TabIndex first, ordered by TabIndex then regSeq.
		// TabIndex 0 after all positives, by regSeq.
		ai, bi := a.TabIndex, b.TabIndex
		if ai == 0 && bi == 0 {
			return a.regSeq < b.regSeq
		}
		if ai == 0 {
			return false
		}
		if bi == 0 {
			return true
		}
		if ai != bi {
			return ai < bi
		}
		return a.regSeq < b.regSeq
	})
	return list
}

// FocusNext moves primary to the next focusable (wraps).
func (m *FocusManager) FocusNext() *FocusNode {
	return m.move(+1)
}

// FocusPrevious moves primary to the previous focusable (wraps).
func (m *FocusManager) FocusPrevious() *FocusNode {
	return m.move(-1)
}

func (m *FocusManager) move(dir int) *FocusNode {
	if m == nil {
		return nil
	}
	list := m.focusables()
	if len(list) == 0 {
		return nil
	}
	idx := -1
	for i, n := range list {
		if n == m.primary {
			idx = i
			break
		}
	}
	if idx < 0 {
		// No primary: dir>0 → first, dir<0 → last
		if dir >= 0 {
			m.setPrimary(list[0])
		} else {
			m.setPrimary(list[len(list)-1])
		}
		return m.primary
	}
	next := (idx + dir) % len(list)
	if next < 0 {
		next += len(list)
	}
	m.setPrimary(list[next])
	return m.primary
}

// HandleKey routes a key event. Safe with nil primary (no panic).
//
// Order:
//  1. Track Shift press/release
//  2. On Tab press: FocusNext / FocusPrevious (consumes)
//  3. Primary.OnKey if set
//  4. Activate keys → OnActivate if not consumed
//
// Returns true if the event was consumed.
func (m *FocusManager) HandleKey(e KeyEvent) bool {
	if m == nil {
		return false
	}
	if e.IsShift() {
		m.shiftDown = e.Pressed
		return false
	}
	if e.Shift {
		m.shiftDown = true
	}
	shift := m.shiftDown || e.Shift

	if e.Pressed && e.IsTab() {
		if shift {
			m.FocusPrevious()
		} else {
			m.FocusNext()
		}
		return true
	}

	p := m.primary
	if p == nil {
		return false
	}
	if p.OnKey != nil {
		// Copy shift into event for handlers.
		e.Shift = shift
		if p.OnKey(e) {
			return true
		}
	}
	if e.Pressed && e.IsActivate() && p.OnActivate != nil {
		p.OnActivate()
		return true
	}
	return false
}

// FocusFromHit requests focus on n when non-nil and focusable (pointer down helper).
func (m *FocusManager) FocusFromHit(n *FocusNode) bool {
	if n == nil {
		return false
	}
	return m.RequestFocus(n)
}
