package gestures_test

import (
	"testing"

	"github.com/energye/gpui/ui/gestures"
	"github.com/energye/gpui/ui/platform"
)

func pe(kind platform.PointerKind, x, y float64) gestures.PointerEvent {
	return gestures.PointerEvent{Kind: kind, X: x, Y: y, PointerID: gestures.PrimaryPointerID}
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

	down := pe(platform.PointerDown, 0, 0)
	a := mgr.Arena(gestures.PrimaryPointerID)
	a.Add(tap)
	a.Add(pan)
	tap.AddPointer(gestures.PrimaryPointerID, down)
	pan.AddPointer(gestures.PrimaryPointerID, down)
	mgr.Route(down)
	mgr.CloseArena(gestures.PrimaryPointerID)

	for _, m := range moves {
		mgr.Route(pe(platform.PointerMove, m[0], m[1]))
	}
	mgr.Route(pe(platform.PointerUp, upX, upY))
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
	down := pe(platform.PointerDown, 10, 10)
	a := mgr.Arena(1)
	a.Add(tap)
	a.Add(pan)
	tap.AddPointer(1, down)
	pan.AddPointer(1, down)
	mgr.Route(down)
	mgr.CloseArena(1)
	// Force reject before up — must not fire OnTap.
	a.Reject(tap)
	mgr.Route(pe(platform.PointerUp, 10, 10))
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
	batch := []gestures.PointerEvent{pe(platform.PointerDown, 0, 0)}
	// First move beyond slop + 9 more — coalesced into one move at end before up.
	// HandleBatch coalesces consecutive moves only; inject one move past slop
	// then a block of moves.
	for i := 1; i <= 10; i++ {
		batch = append(batch, pe(platform.PointerMove, 0, float64(i*3))) // 3,6,...,30
	}
	batch = append(batch, pe(platform.PointerUp, 0, 30))
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
		Kind: platform.PointerScroll, ScrollY: 40, PointerID: 1,
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
	down := pe(platform.PointerDown, 0, 0)
	a := mgr.Arena(1)
	a.Add(tap)
	tap.AddPointer(1, down)
	mgr.Route(down)
	mgr.CloseArena(1)
	mgr.Route(pe(platform.PointerUp, 0, 0))
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
	d.HandleDown(pe(platform.PointerDown, 1, 1), []gestures.PointerTarget{leaf, root})
	if len(order) != 2 || order[0] != "leaf" || order[1] != "root" {
		t.Fatalf("join order=%v want [leaf root]", order)
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
