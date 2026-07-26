package animation_test

import (
	"testing"

	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/scheduler"
)

func TestController_OneShotAutoRemove(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	var last float64
	c := animation.NewController(0.1)
	c.OnValue(func(v float64) { last = v })
	c.Start(reg)
	if !reg.HasActive() {
		t.Fatal("expected active ticker")
	}
	// Two large ticks should complete.
	reg.TickAll(0.06)
	if !reg.HasActive() {
		// may still be active
	}
	reg.TickAll(0.06)
	if reg.HasActive() {
		t.Fatal("expected auto-remove after completion")
	}
	if last < 1 {
		// last tick sets 1
		if c.Value() != 1 && last != 1 {
			t.Fatalf("value=%v last=%v", c.Value(), last)
		}
	}
	if c.IsRunning() {
		t.Fatal("should not be running")
	}
}

func TestController_Repeat(t *testing.T) {
	reg := &scheduler.TickerRegistry{}
	c := animation.NewController(0.05)
	c.SetRepeat(true)
	n := 0
	c.OnValue(func(v float64) { n++ })
	c.Start(reg)
	for i := 0; i < 5; i++ {
		reg.TickAll(0.02)
	}
	if !reg.HasActive() {
		t.Fatal("repeat should keep ticker")
	}
	if n < 5 {
		t.Fatalf("ticks=%d", n)
	}
	c.Stop()
	if reg.HasActive() {
		t.Fatal("stop should remove")
	}
}
