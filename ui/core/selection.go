package core

// SelectionMode controls how SelectionModel accepts values.
type SelectionMode int

const (
	SelectSingle SelectionMode = iota
	SelectMultiple
)

// SelectionModel is a reusable single/multi selection bag (C-SelectModel).
type SelectionModel struct {
	Mode     SelectionMode
	selected map[string]struct{}
	// Order preserves multi-select insertion order.
	Order []string
	// OnChange fires after any mutation.
	OnChange func(values []string)
}

// NewSelectionModel creates an empty model.
func NewSelectionModel(mode SelectionMode) *SelectionModel {
	return &SelectionModel{
		Mode:     mode,
		selected: make(map[string]struct{}),
	}
}

// Values returns selected keys (stable order for multi).
func (m *SelectionModel) Values() []string {
	if m == nil {
		return nil
	}
	if m.Mode == SelectSingle {
		if len(m.Order) == 0 {
			return nil
		}
		return []string{m.Order[0]}
	}
	out := make([]string, len(m.Order))
	copy(out, m.Order)
	return out
}

// Value returns the first selected key (single-select convenience).
func (m *SelectionModel) Value() string {
	vs := m.Values()
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// Has reports whether key is selected.
func (m *SelectionModel) Has(key string) bool {
	if m == nil || m.selected == nil {
		return false
	}
	_, ok := m.selected[key]
	return ok
}

// Set replaces selection with the given keys.
func (m *SelectionModel) Set(keys ...string) {
	if m == nil {
		return
	}
	m.selected = make(map[string]struct{})
	m.Order = nil
	if m.Mode == SelectSingle {
		if len(keys) > 0 && keys[0] != "" {
			m.selected[keys[0]] = struct{}{}
			m.Order = []string{keys[0]}
		}
	} else {
		for _, k := range keys {
			if k == "" {
				continue
			}
			if _, ok := m.selected[k]; ok {
				continue
			}
			m.selected[k] = struct{}{}
			m.Order = append(m.Order, k)
		}
	}
	m.fire()
}

// Toggle adds or removes key (multi); for single, sets exclusive.
func (m *SelectionModel) Toggle(key string) {
	if m == nil || key == "" {
		return
	}
	if m.Mode == SelectSingle {
		if m.Has(key) {
			m.Set()
		} else {
			m.Set(key)
		}
		return
	}
	if m.Has(key) {
		delete(m.selected, key)
		for i, k := range m.Order {
			if k == key {
				m.Order = append(m.Order[:i], m.Order[i+1:]...)
				break
			}
		}
	} else {
		m.selected[key] = struct{}{}
		m.Order = append(m.Order, key)
	}
	m.fire()
}

// Clear empties selection.
func (m *SelectionModel) Clear() { m.Set() }

func (m *SelectionModel) fire() {
	if m.OnChange != nil {
		m.OnChange(m.Values())
	}
}

// SelectionScope is a node that owns a SelectionModel for descendants.
type SelectionScope struct {
	NodeBase
	Model *SelectionModel
}

// NewSelectionScope creates a scope with the given mode.
func NewSelectionScope(mode SelectionMode, children ...Node) *SelectionScope {
	s := &SelectionScope{Model: NewSelectionModel(mode)}
	s.Init(s)
	s.Hit = HitDefer
	for _, c := range children {
		s.AddChild(c)
	}
	return s
}

// TypeID implements Node.
func (s *SelectionScope) TypeID() string { return "core.SelectionScope" }

// Layout implements Node.
func (s *SelectionScope) Layout(c Constraints) Size {
	kids := s.Children()
	if len(kids) == 0 {
		out := c.Tighten(Size{})
		s.SetSize(out)
		return out
	}
	// Stack-like: if one child, fill; else max
	if len(kids) == 1 {
		sz := kids[0].Layout(c.Expand())
		kids[0].Base().SetOffset(Point{})
		out := c.Tighten(sz)
		s.SetSize(out)
		return out
	}
	content := Size{}
	for _, child := range kids {
		sz := child.Layout(c.Expand())
		child.Base().SetOffset(Point{})
		content = MaxSize(content, sz)
	}
	out := c.Tighten(content)
	s.SetSize(out)
	return out
}

// Paint implements Node.
func (s *SelectionScope) Paint(pc *PaintContext) { s.DefaultPaintChildren(pc) }

// HitTest implements Node.
func (s *SelectionScope) HitTest(p Point) Node { return s.DefaultHitTest(p) }

// FindSelectionScope walks ancestors for a SelectionScope.
func FindSelectionScope(n Node) *SelectionScope {
	for cur := n; cur != nil; cur = cur.Parent() {
		if s, ok := cur.(*SelectionScope); ok {
			return s
		}
	}
	return nil
}
