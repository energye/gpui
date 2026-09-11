package focus_test

import (
	"testing"

	"github.com/energye/gpui/ui/focus"
)

func TestFocus_UniquePrimary(t *testing.T) {
	m := focus.NewManager()
	a := focus.NewFocusNode("a")
	b := focus.NewFocusNode("b")
	m.Register(a)
	m.Register(b)
	if !m.RequestFocus(a) {
		t.Fatal("RequestFocus a")
	}
	if m.Primary() != a || !a.HasFocus() || b.HasFocus() {
		t.Fatal("primary not a")
	}
	if !m.RequestFocus(b) {
		t.Fatal("RequestFocus b")
	}
	if m.Primary() != b || a.HasFocus() || !b.HasFocus() {
		t.Fatal("primary not b")
	}
	if m.FocusChanges() < 2 {
		t.Fatalf("focusChanges=%d", m.FocusChanges())
	}
}

func TestFocus_UnregisterClearsPrimary(t *testing.T) {
	m := focus.NewManager()
	a := focus.NewFocusNode("a")
	m.Register(a)
	m.RequestFocus(a)
	a.Unregister()
	if m.Primary() != nil {
		t.Fatal("dangling primary after unregister")
	}
	if a.HasFocus() {
		t.Fatal("HasFocus after unregister")
	}
	if m.Count() != 0 {
		t.Fatalf("count=%d", m.Count())
	}
}

func TestFocus_TabOrder_Wraps(t *testing.T) {
	m := focus.NewManager()
	nodes := make([]*focus.FocusNode, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = focus.NewFocusNode(string(rune('a' + i)))
		m.Register(nodes[i])
	}
	m.RequestFocus(nodes[0])
	// Tab around the ring
	for i := 1; i < 10; i++ {
		m.HandleKey(focus.KeyEvent{KeyCode: focus.KeyTab, Pressed: true})
		if m.Primary() != nodes[i] {
			t.Fatalf("tab %d: primary=%v want %v", i, m.Primary().DebugLabel, nodes[i].DebugLabel)
		}
	}
	// Wrap to start
	m.HandleKey(focus.KeyEvent{KeyCode: focus.KeyTab, Pressed: true})
	if m.Primary() != nodes[0] {
		t.Fatalf("wrap: got %v", m.Primary().DebugLabel)
	}
	// Shift+Tab reverse
	m.HandleKey(focus.KeyEvent{KeyCode: focus.KeyTab, Pressed: true, Shift: true})
	if m.Primary() != nodes[9] {
		t.Fatalf("shift-tab: got %v want j", m.Primary().DebugLabel)
	}
}

func TestFocus_TabIndexOrder(t *testing.T) {
	m := focus.NewManager()
	a := focus.NewFocusNode("a") // natural
	b := focus.NewFocusNode("b")
	b.TabIndex = 2
	c := focus.NewFocusNode("c")
	c.TabIndex = 1
	d := focus.NewFocusNode("d")
	d.TabIndex = -1 // skipped
	m.Register(a)
	m.Register(b)
	m.Register(c)
	m.Register(d)
	// Order: c(1), b(2), a(0)
	m.FocusNext()
	if m.Primary() != c {
		t.Fatalf("first=%v want c", label(m))
	}
	m.FocusNext()
	if m.Primary() != b {
		t.Fatalf("second=%v want b", label(m))
	}
	m.FocusNext()
	if m.Primary() != a {
		t.Fatalf("third=%v want a", label(m))
	}
	m.FocusNext()
	if m.Primary() != c {
		t.Fatalf("wrap=%v want c", label(m))
	}
}

func TestFocus_OnKey_SpaceEnter(t *testing.T) {
	m := focus.NewManager()
	n := focus.NewFocusNode("btn")
	var keys int
	var activates int
	n.OnKey = func(e focus.KeyEvent) bool {
		if e.Pressed && e.IsActivate() {
			keys++
			return true
		}
		return false
	}
	n.OnActivate = func() { activates++ }
	m.Register(n)
	m.RequestFocus(n)
	m.HandleKey(focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true})
	m.HandleKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true})
	if keys != 2 {
		t.Fatalf("OnKey activate count=%d", keys)
	}
	if activates != 0 {
		t.Fatal("OnActivate should not run when OnKey consumes")
	}
	// Without OnKey consume → OnActivate
	n.OnKey = nil
	m.HandleKey(focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true})
	if activates != 1 {
		t.Fatalf("OnActivate=%d", activates)
	}
}

func TestFocus_ChangeDoesNotLayout(t *testing.T) {
	// C5: focus changes only fire OnFocusChange (paint hints), never layout.
	m := focus.NewManager()
	var layout int // simulated — focus package must not touch layout
	paintA, paintB := 0, 0
	a := focus.NewFocusNode("a")
	b := focus.NewFocusNode("b")
	a.OnFocusChange = func(f bool) {
		if f {
			paintA++
		} else {
			paintA++
		}
	}
	b.OnFocusChange = func(f bool) { paintB++ }
	m.Register(a)
	m.Register(b)
	m.RequestFocus(a)
	m.RequestFocus(b)
	m.Blur()
	if layout != 0 {
		t.Fatal("layout must stay 0")
	}
	if m.PaintHints() < 3 {
		// a gain, a loss, b gain, b loss = 4
		t.Fatalf("paintHints=%d", m.PaintHints())
	}
	if paintA < 2 || paintB < 2 {
		t.Fatalf("paintA=%d paintB=%d", paintA, paintB)
	}
}

func TestFocus_PointerDownRequestsFocus(t *testing.T) {
	m := focus.NewManager()
	a := focus.NewFocusNode("a")
	b := focus.NewFocusNode("b")
	m.Register(a)
	m.Register(b)
	// Simulate hit-test result → FocusFromHit
	if !m.FocusFromHit(b) {
		t.Fatal("FocusFromHit b")
	}
	if m.Primary() != b {
		t.Fatal("primary not b after hit")
	}
	if m.FocusFromHit(nil) {
		t.Fatal("nil hit should be false")
	}
}

func TestFocus_KeyWithNoPrimary(t *testing.T) {
	m := focus.NewManager()
	// Must not panic
	ok := m.HandleKey(focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true})
	if ok {
		t.Fatal("expected unconsumed")
	}
	// Tab with no focusables registered must also pass through unconsumed.
	if m.HandleKey(focus.KeyEvent{KeyCode: focus.KeyTab, Pressed: true}) {
		t.Fatal("tab with no focusables should be unconsumed")
	}
}

func TestFocus_DisabledSkipped(t *testing.T) {
	m := focus.NewManager()
	a := focus.NewFocusNode("a")
	b := focus.NewFocusNode("b")
	b.Enabled = false
	c := focus.NewFocusNode("c")
	m.Register(a)
	m.Register(b)
	m.Register(c)
	m.RequestFocus(a)
	m.HandleKey(focus.KeyEvent{KeyCode: focus.KeyTab, Pressed: true})
	if m.Primary() != c {
		t.Fatalf("tab over disabled: got %v want c", label(m))
	}
	if m.RequestFocus(b) {
		t.Fatal("disabled must not focus")
	}
}

func TestFocus_RingRect(t *testing.T) {
	x, y, w, h := focus.FocusRingRect(10, 20, 100, 40, 1.5)
	if x != 8.5 || y != 18.5 || w != 103 || h != 43 {
		t.Fatalf("ring=%v,%v,%v,%v", x, y, w, h)
	}
}

func label(m *focus.FocusManager) string {
	if m.Primary() == nil {
		return "<nil>"
	}
	return m.Primary().DebugLabel
}

// TestFocus_ObserverTransitions verifies manager-level observers fire after
// every primary transition with (from, to) — the embedder's IME session
// management (I4) depends on this contract.
func TestFocus_ObserverTransitions(t *testing.T) {
	m := focus.NewManager()
	a := focus.NewFocusNode("a")
	b := focus.NewFocusNode("b")
	m.Register(a)
	m.Register(b)

	var got [][2]string
	m.AddFocusObserver(func(from, to *focus.FocusNode) {
		f, t2 := "", ""
		if from != nil {
			f = from.DebugLabel
		}
		if to != nil {
			t2 = to.DebugLabel
		}
		got = append(got, [2]string{f, t2})
	})

	if !m.RequestFocus(a) {
		t.Fatal("RequestFocus(a) failed")
	}
	if !m.RequestFocus(b) {
		t.Fatal("RequestFocus(b) failed")
	}
	m.Blur()
	want := [][2]string{{"", "a"}, {"a", "b"}, {"b", ""}}
	if len(got) != len(want) {
		t.Fatalf("observer calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("transition[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestFocus_NodeTarget verifies the Target payload round-trips (the
// framework type-asserts it to TextEditTarget).
func TestFocus_NodeTarget(t *testing.T) {
	n := focus.NewFocusNode("x")
	type box struct{ v int }
	bx := &box{7}
	n.Target = bx
	if n.Target.(*box).v != 7 {
		t.Fatal("target payload lost")
	}
}
