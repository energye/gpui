package animation_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/scheduler"
)

func TestCurve_LinearBounds(t *testing.T) {
	if animation.Linear(0) != 0 || animation.Linear(1) != 1 {
		t.Fatal("linear bounds")
	}
	if animation.Linear(0.5) != 0.5 {
		t.Fatal("linear mid")
	}
	if animation.Linear(-1) != 0 || animation.Linear(2) != 1 {
		t.Fatal("linear clamp")
	}
}

func TestCurve_EaseInOutBounds(t *testing.T) {
	if animation.EaseInOut(0) != 0 || animation.EaseInOut(1) != 1 {
		t.Fatalf("ease bounds %v %v", animation.EaseInOut(0), animation.EaseInOut(1))
	}
	mid := animation.EaseInOut(0.5)
	if math.Abs(mid-0.5) > 1e-9 {
		t.Fatalf("ease mid=%v", mid)
	}
	// smoothstep lags linear near start
	if animation.EaseInOut(0.1) >= animation.Linear(0.1) {
		t.Fatalf("ease-in should lag linear at start: %v", animation.EaseInOut(0.1))
	}
}

func TestController_UsesCurve(t *testing.T) {
	c := animation.NewController(1)
	c.SetCurve(animation.CurveEaseInOut)
	reg := &scheduler.TickerRegistry{}
	c.Start(reg)
	reg.TickAll(0.5)
	v := c.Value()
	if math.Abs(v-0.5) > 1e-6 {
		t.Fatalf("value=%v want 0.5", v)
	}
	if math.Abs(c.LinearProgress()-0.5) > 1e-6 {
		t.Fatalf("linear=%v", c.LinearProgress())
	}
}
