package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

type countTicker struct {
	n     int
	limit int
	dirty *core.Tree
}

func (c *countTicker) Tick(dt float64) bool {
	c.n++
	if c.dirty != nil {
		c.dirty.MarkDirty()
	}
	return c.n < c.limit
}

func TestNeedsFrameAndFrameIfNeeded(t *testing.T) {
	n := newLeaf("a", 10, 10)
	tree := core.NewTree(n)
	if !tree.Dirty() {
		t.Fatal("new tree should be dirty")
	}
	if !tree.NeedsFrame() {
		t.Fatal("NeedsFrame when dirty")
	}
	tree.Frame(nil, core.Size{Width: 100, Height: 100})
	// Paint with nil skips paint clear path — Layout still ran; dirty cleared only in Paint.
	// Call with empty paint context via Frame again after MarkDirty.
	tree.MarkDirty()
	pc := &core.PaintContext{}
	if !tree.FrameIfNeeded(pc, core.Size{Width: 100, Height: 100}) {
		t.Fatal("FrameIfNeeded should run when dirty")
	}
	if tree.Dirty() {
		t.Fatal("dirty should clear after paint")
	}
	if tree.FrameIfNeeded(pc, core.Size{Width: 100, Height: 100}) {
		t.Fatal("FrameIfNeeded must skip when clean")
	}
	if tree.NeedsFrame() {
		t.Fatal("NeedsFrame false when clean and no tickers")
	}
}

func TestTickerRegistry(t *testing.T) {
	tree := core.NewTree(newLeaf("a", 8, 8))
	tree.Frame(&core.PaintContext{}, core.Size{Width: 40, Height: 40})
	if tree.Dirty() {
		t.Fatal("expected clean after frame")
	}

	tk := &countTicker{limit: 3, dirty: tree}
	tree.AddTicker(tk)
	if !tree.HasActiveTickers() || !tree.NeedsFrame() {
		t.Fatal("ticker should keep NeedsFrame / HasActiveTickers")
	}
	// FrameIfNeeded only paints when Dirty — AddTicker marks dirty once.
	if !tree.Dirty() {
		t.Fatal("AddTicker should mark dirty")
	}
	tree.ClearDirty() // simulate host that only paints on visual dirty from ticker
	// TickActive advances; ticker marks dirty each tick
	if !tree.TickActive(0.016) {
		t.Fatal("still active after first tick")
	}
	if !tree.Dirty() {
		t.Fatal("ticker should mark dirty on Tick")
	}
	tree.FrameIfNeeded(&core.PaintContext{}, core.Size{Width: 40, Height: 40})
	tree.TickActive(0.016)
	tree.TickActive(0.016) // n=3, limit=3 → still false after this when n becomes 3
	// After 3 ticks with limit 3: n=1,2,3 — when n==3, still is n<3 false, removed
	// We called Tick 1 (after ClearDirty), then 2 more = 3 total. Good.
	if tree.HasActiveTickers() {
		// one more if still
		for tree.TickActive(0.016) {
		}
	}
	if tree.HasActiveTickers() {
		t.Fatalf("tickers should finish, n=%d", tk.n)
	}
}

func TestTickClockDoesNotMarkDirty(t *testing.T) {
	tree := core.NewTree(newLeaf("a", 4, 4))
	tree.Frame(&core.PaintContext{}, core.Size{Width: 20, Height: 20})
	if tree.Dirty() {
		t.Fatal("clean expected")
	}
	tree.TickClock(0.016)
	if tree.Dirty() {
		t.Fatal("TickClock must not mark dirty (demand-driven)")
	}
	if tree.Clock().T <= 0 {
		t.Fatal("clock should advance")
	}
}

func TestOnDirtyCallback(t *testing.T) {
	tree := core.NewTree(nil)
	tree.ClearDirty()
	n := 0
	tree.SetOnDirty(func() { n++ })
	tree.MarkDirty()
	if n != 1 {
		t.Fatalf("onDirty calls=%d", n)
	}
	tree.MarkDirty()
	if n != 2 {
		t.Fatalf("onDirty should fire each markDirty, got %d", n)
	}
}
