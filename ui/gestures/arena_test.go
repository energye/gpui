package gestures_test

import (
	"testing"

	"github.com/energye/gpui/ui/gestures"
	"github.com/energye/gpui/ui/input"
)

func pe(kind input.PointerKind, x, y float64) gestures.PointerEvent {
	return gestures.PointerEvent{Kind: kind, X: x, Y: y, ID: gestures.PrimaryPointerID}
}

// runTapPan drives a down → moves → up sequence with both recognizers.
func runTapPan(t *testing.T, moves [][2]float64, upX, upY float64) (taps, panStarts, panUpdates int) {
	t.Helper()
	mgr := gestures.NewManager()
	tap := gestures.NewTap()
	pan := gestures.NewPan()
	tap.OnTap = func(e gestures.PointerEvent) { taps++ }
	pan.OnPanStart = func(e gestures.PointerEvent) { panStarts++ }
	pan.OnPanUpdate = func(e gestures.PointerEvent, dx, dy float64) { panUpdates++ }

	down := pe(input.PointerDown, 0, 0)
	id := gestures.PrimaryPointerID
	a := mgr.Arena(id)
	a.Add(tap)
	a.Add(pan)
	tap.AddPointer(id, down)
	pan.AddPointer(id, down)
	mgr.Route(down)
	mgr.CloseArena(id)

	for _, m := range moves {
		mgr.Route(pe(input.PointerMove, m[0], m[1]))
	}
	mgr.Route(pe(input.PointerUp, upX, upY))
	return taps, panStarts, panUpdates
}

func TestArena_TwoMembers_AcceptRejectsOther(t *testing.T) {
	mgr := gestures.NewManager()
	a := mgr.Arena(1)
	r1 := gestures.NewTap()
	r2 := gestures.NewTap()
	var a1, r1c, a2, r2c int
	r1.OnTap = func(e gestures.PointerEvent) {}
	// count Accept/Reject via wrappers — use pan/tap Accept side effects
	// Direct arena API:
	a.Add(r1)
	a.Add(r2)
	if a.MemberCount() != 2 {
		t.Fatalf("members=%d", a.MemberCount())
	}
	a.Close()
	a.Accept(r1)
	if a.Winner() != r1 {
		t.Fatal("winner not r1")
	}
	if !a.Resolved() {
		t.Fatal("expected resolved")
	}
	// r2 should be rejected: further Accept no-op
	a.Accept(r2)
	if a.Winner() != r1 {
		t.Fatal("winner changed")
	}
	_ = a1
	_ = r1c
	_ = a2
	_ = r2c
}

func TestArena_TapWinsWithinSlop(t *testing.T) {
	// Move 4px < 8 slop, then up → tap only
	taps, panStarts, _ := runTapPan(t, [][2]float64{{2, 2}, {3, 1}}, 3, 1)
	if taps != 1 {
		t.Fatalf("onTap=%d want 1", taps)
	}
	if panStarts != 0 {
		t.Fatalf("onPanStart=%d want 0", panStarts)
	}
	if mgr := gestures.NewManager(); mgr.ActiveCount() != 0 {
		// ensure runTapPan finished arena — separate check:
	}
}

func TestArena_PanWinsBeyondSlop(t *testing.T) {
	// Move to y=20 (>8) → pan wins; up must not tap
	taps, panStarts, panUpdates := runTapPan(t, [][2]float64{{0, 5}, {0, 20}, {0, 30}}, 0, 35)
	if panStarts != 1 {
		t.Fatalf("onPanStart=%d want 1", panStarts)
	}
	if panUpdates < 1 {
		t.Fatalf("onPanUpdate=%d want ≥1", panUpdates)
	}
	if taps != 0 {
		t.Fatalf("onTap=%d want 0", taps)
	}
}

func TestArena_RejectDoesNotFire(t *testing.T) {
	mgr := gestures.NewManager()
	tap := gestures.NewTap()
	// Second member prevents CloseArena from auto-accepting the sole pending tap.
	pan := gestures.NewPan()
	var taps int
	tap.OnTap = func(e gestures.PointerEvent) { taps++ }
	down := pe(input.PointerDown, 10, 10)
	a := mgr.Arena(1)
	a.Add(tap)
	a.Add(pan)
	tap.AddPointer(1, down)
	pan.AddPointer(1, down)
	mgr.Route(down)
	mgr.CloseArena(1)
	// Force reject before up — must not fire OnTap.
	a.Reject(tap)
	mgr.Route(pe(input.PointerUp, 10, 10))
	if taps != 0 {
		t.Fatalf("onTap after reject = %d", taps)
	}
}

func TestArena_LatestMoveOnly(t *testing.T) {
	// Dispatcher coalesces 10 moves → one update reflecting last point.
	d := gestures.NewDispatcher()
	pan := gestures.NewPan()
	var starts, updates int
	var lastY float64
	var lastDY float64
	pan.OnPanStart = func(e gestures.PointerEvent) {
		starts++
		lastY = e.Y
	}
	pan.OnPanUpdate = func(e gestures.PointerEvent, dx, dy float64) {
		updates++
		lastY = e.Y
		lastDY = dy
	}
	path := []gestures.PointerTarget{&gestures.RecognizerTarget{Rec: pan}}
	batch := []gestures.PointerEvent{pe(input.PointerDown, 0, 0)}
	// First move beyond slop + 9 more — coalesced into one move at end before up.
	// HandleBatch coalesces consecutive moves only; inject one move past slop
	// then a block of moves.
	for i := 1; i <= 10; i++ {
		batch = append(batch, pe(input.PointerMove, 0, float64(i*3))) // 3,6,...,30
	}
	batch = append(batch, pe(input.PointerUp, 0, 30))
	d.HandleBatch(batch, func(e gestures.PointerEvent) []gestures.PointerTarget { return path })

	if starts != 1 {
		t.Fatalf("starts=%d want 1", starts)
	}
	// All 10 moves coalesced to the single latest (y=30) when flushed before up.
	// After down, moves are coalesced: only one Move event (y=30) is delivered.
	// That one move exceeds slop, Accept, Start, Update with dy from last(0) to 30.
	if updates != 1 {
		t.Fatalf("updates=%d want 1 (coalesced)", updates)
	}
	if lastY != 30 {
		t.Fatalf("lastY=%v want 30", lastY)
	}
	if lastDY != 30 {
		t.Fatalf("lastDY=%v want 30", lastDY)
	}
}

func TestArena_ScrollWheelBypassesTap(t *testing.T) {
	d := gestures.NewDispatcher()
	var scrollN int
	var sy float64
	d.OnScroll = func(sd gestures.ScrollDelta) {
		scrollN++
		sy += sd.ScrollY
	}
	// Scroll alone (no down/up): never enters arena, never taps.
	d.HandleEvent(gestures.PointerEvent{
		Kind: input.PointerScroll, ScrollY: 40, ID: 1,
	})
	if scrollN != 1 || sy != 40 {
		t.Fatalf("scroll n=%d sy=%v", scrollN, sy)
	}
	if d.Manager.ActiveCount() != 0 {
		t.Fatalf("arena should stay empty for wheel, active=%d", d.Manager.ActiveCount())
	}
}

func TestArena_ManagerNoLeak(t *testing.T) {
	mgr := gestures.NewManager()
	tap := gestures.NewTap()
	id := 1
	down := gestures.PointerEvent{Kind: input.PointerDown, ID: id, X: 0, Y: 0}
	a := mgr.Arena(id)
	a.Add(tap)
	tap.AddPointer(id, down)
	mgr.Route(down)
	mgr.CloseArena(id)
	mgr.Route(gestures.PointerEvent{Kind: input.PointerUp, ID: id, X: 0, Y: 0})
	if mgr.ActiveCount() != 0 {
		t.Fatalf("active arenas=%d want 0", mgr.ActiveCount())
	}
}

func TestArena_HitPathDeepestFirst(t *testing.T) {
	// Join order: leaf then root — both added; leaf pan wins on drag.
	var order []string
	leaf := &joinRecorder{name: "leaf", order: &order}
	root := &joinRecorder{name: "root", order: &order}
	d := gestures.NewDispatcher()
	d.HandleDown(pe(input.PointerDown, 1, 1), []gestures.PointerTarget{leaf, root})
	if len(order) != 2 || order[0] != "leaf" || order[1] != "root" {
		t.Fatalf("join order=%v want [leaf root]", order)
	}
}

// --- multi-touch: parallel arenas per pointer ID ---

// twoFingerTap drives two independent taps on two touch slots and asserts
// both fire — the multi-touch parallel-arena contract.
func twoFingerTap(t *testing.T) (tapA, tapB int) {
	t.Helper()
	mgr := gestures.NewManager()
	ta := gestures.NewTap()
	tb := gestures.NewTap()
	ta.OnTap = func(e gestures.PointerEvent) { tapA++ }
	tb.OnTap = func(e gestures.PointerEvent) { tapB++ }

	// Finger A (ID 1) and finger B (ID 2) down in the same frame.
	for _, f := range []struct {
		id int
		rec *gestures.TapGestureRecognizer
		x, y float64
	}{
		{1, ta, 10, 10},
		{2, tb, 50, 50},
	} {
		a := mgr.Arena(f.id)
		a.Add(f.rec)
		down := gestures.PointerEvent{Kind: input.PointerDown, ID: f.id, X: f.x, Y: f.y}
		f.rec.AddPointer(f.id, down)
		mgr.Route(down)
		mgr.CloseArena(f.id)
	}
	// Both fingers lift within slop.
	mgr.Route(gestures.PointerEvent{Kind: input.PointerUp, ID: 1, X: 10, Y: 10})
	mgr.Route(gestures.PointerEvent{Kind: input.PointerUp, ID: 2, X: 50, Y: 50})
	return tapA, tapB
}

func TestMultiTouch_TwoIndependentTaps(t *testing.T) {
	tapA, tapB := twoFingerTap(t)
	if tapA != 1 {
		t.Fatalf("finger A taps = %d, want 1", tapA)
	}
	if tapB != 1 {
		t.Fatalf("finger B taps = %d, want 1", tapB)
	}
}

func TestMultiTouch_ParallelArenas(t *testing.T) {
	mgr := gestures.NewManager()
	ta := gestures.NewTap()
	tb := gestures.NewPan()
	var tapsA int
	var panStartsB int
	var panUpdatesB int
	ta.OnTap = func(e gestures.PointerEvent) { tapsA++ }
	tb.OnPanStart = func(e gestures.PointerEvent) { panStartsB++ }
	tb.OnPanUpdate = func(e gestures.PointerEvent, dx, dy float64) { panUpdatesB++ }

	// Finger 1 taps; finger 2 drags — both run concurrently on distinct arenas.
	d1 := gestures.PointerEvent{Kind: input.PointerDown, ID: 1, X: 10, Y: 10}
	d2 := gestures.PointerEvent{Kind: input.PointerDown, ID: 2, X: 50, Y: 50}
	a1 := mgr.Arena(1)
	a1.Add(ta)
	ta.AddPointer(1, d1)
	a2 := mgr.Arena(2)
	a2.Add(tb)
	tb.AddPointer(2, d2)
	mgr.Route(d1)
	mgr.CloseArena(1)
	mgr.Route(d2)
	mgr.CloseArena(2)

	// Finger 2 drags past slop → pan fires on arena 2 (independent of 1).
	mgr.Route(gestures.PointerEvent{Kind: input.PointerMove, ID: 2, X: 50, Y: 80})
	if panStartsB != 1 {
		t.Fatalf("finger 2 pan starts = %d, want 1", panStartsB)
	}
	if panUpdatesB < 1 {
		t.Fatalf("finger 2 pan updates = %d, want ≥1", panUpdatesB)
	}
	// Finger 1 hasn't fired yet — its arena is untouched by finger 2's drag.
	if tapsA != 0 {
		t.Fatalf("finger 1 tap fired during finger 2 drag: %d", tapsA)
	}
	// Finger 1 lifts within slop → tap fires.
	mgr.Route(gestures.PointerEvent{Kind: input.PointerUp, ID: 1, X: 10, Y: 10})
	if tapsA != 1 {
		t.Fatalf("finger 1 taps = %d, want 1", tapsA)
	}
	// Finish finger 2.
	mgr.Route(gestures.PointerEvent{Kind: input.PointerUp, ID: 2, X: 50, Y: 85})
	if mgr.ActiveCount() != 0 {
		t.Fatalf("arenas leaked: active=%d", mgr.ActiveCount())
	}
}

func TestFromInput_PointerAndTouch(t *testing.T) {
	// Pointer kind → straight map.
	in := input.Event{Kind: input.KindPointer, Pointer: input.PointerEvent{Kind: input.PointerDown, ID: 0, X: 3, Y: 4}}
	ge, ok := gestures.FromInput(in)
	if !ok || ge.Kind != input.PointerDown || ge.X != 3 || ge.Y != 4 {
		t.Fatalf("pointer FromInput = %+v ok=%v", ge, ok)
	}
	// Touch kind → ID/X/Y carried.
	inT := input.FromTouch(input.TouchEvent{Kind: input.PointerMove, ID: 5, X: 7, Y: 8}, input.Modifiers{})
	geT, okT := gestures.FromInput(inT)
	if !okT || geT.ID != 5 || geT.X != 7 || geT.Y != 8 {
		t.Fatalf("touch FromInput = %+v ok=%v", geT, okT)
	}
	// Non-pointer kinds → ok=false.
	nonPtr := input.FromText("x", input.Modifiers{})
	if _, ok := gestures.FromInput(nonPtr); ok {
		t.Fatal("text event must not map to pointer")
	}
}

type joinRecorder struct {
	name  string
	order *[]string
	rec   *gestures.TapGestureRecognizer
}

func (j *joinRecorder) JoinPointer(m *gestures.GestureArenaManager, pointerID int, e gestures.PointerEvent) {
	*j.order = append(*j.order, j.name)
	if j.rec == nil {
		j.rec = gestures.NewTap()
	}
	a := m.Arena(pointerID)
	a.Add(j.rec)
	j.rec.AddPointer(pointerID, e)
}
