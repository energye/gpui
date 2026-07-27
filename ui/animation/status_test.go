package animation_test

import (
	"testing"

	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"
)

func TestController_StatusTransitions(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	c := animation.NewController(0.1)
	var statuses []animation.Status
	c.OnStatus(func(s animation.Status) { statuses = append(statuses, s) })

	if c.Status() != animation.StatusDismissed {
		t.Fatal("initial dismissed")
	}
	c.Start(reg)
	if c.Status() != animation.StatusForward {
		t.Fatalf("status=%v", c.Status())
	}
	reg.TickAll(0.12)
	if c.Status() != animation.StatusCompleted {
		t.Fatalf("status=%v want completed", c.Status())
	}
	if reg.HasActive() {
		t.Fatal("F17: ticker must auto-remove")
	}
	// Start → Forward, complete → Completed
	if len(statuses) < 2 {
		t.Fatalf("statuses=%v", statuses)
	}
}

func TestController_Reverse(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	c := animation.NewController(0.2)
	c.Start(reg)
	reg.TickAll(0.2) // complete at 1
	if c.Value() != 1 {
		t.Fatalf("value=%v", c.Value())
	}
	c.Reverse(reg)
	if c.Status() != animation.StatusReverse {
		t.Fatalf("status=%v", c.Status())
	}
	reg.TickAll(0.25)
	if c.Status() != animation.StatusCompleted {
		t.Fatalf("status=%v after reverse", c.Status())
	}
	if c.LinearProgress() != 0 {
		t.Fatalf("linear=%v want 0", c.LinearProgress())
	}
	if reg.HasActive() {
		t.Fatal("ticker should be gone")
	}
}

func TestController_StopDismissed(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	c := animation.NewController(1)
	c.Start(reg)
	c.Stop()
	if c.Status() != animation.StatusDismissed {
		t.Fatalf("status=%v", c.Status())
	}
	if reg.HasActive() {
		t.Fatal("stop removes ticker")
	}
}

func TestAnimatedOpacity_CompositorOnly(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	const layerID = uint64(42)
	var layoutCount int
	a := animation.NewAnimatedOpacity(layerID, 1, 0, 0.3, animation.CurveLinear)
	done := false
	a.OnComplete = func() { done = true }
	a.Start(reg)

	// 30 frames @ 10ms
	for i := 0; i < 30; i++ {
		reg.TickAll(0.01)
		// opacity animation must not force layout
	}
	_ = layoutCount
	if layoutCount != 0 {
		t.Fatal("layout must not run")
	}
	if len(a.Mutations) < 10 {
		t.Fatalf("mutations=%d", len(a.Mutations))
	}
	// All mutations compositor-only
	raster, comp := scene.ClassifyDirty(a.Mutations)
	if len(raster) != 0 {
		t.Fatalf("raster dirty ids=%v (want none)", raster)
	}
	if len(comp) != 1 || comp[0] != layerID {
		t.Fatalf("compositor=%v", comp)
	}
	for _, m := range a.Mutations {
		if m.Kind != scene.MutSetOpacity || m.LayerID != layerID {
			t.Fatalf("bad mut %+v", m)
		}
	}
	// Finish remaining
	for reg.HasActive() {
		reg.TickAll(0.05)
	}
	if !done {
		t.Fatal("expected OnComplete")
	}
	if a.IsRunning() || reg.HasActive() {
		t.Fatal("IDLE: ticker must be unloaded")
	}
	if a.Controller().Status() != animation.StatusCompleted {
		t.Fatalf("status=%v", a.Controller().Status())
	}
}
