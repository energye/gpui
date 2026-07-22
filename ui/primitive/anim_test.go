package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/primitive"
)

func TestFloatAnimSnapAndEase(t *testing.T) {
	a := &primitive.FloatAnim{Duration: 0.2, Easing: primitive.EaseOutCubic}
	a.Snap(0)
	if a.Current != 0 || a.Active() {
		t.Fatalf("snap: cur=%v active=%v", a.Current, a.Active())
	}
	a.SetTarget(1)
	if !a.Active() {
		t.Fatal("expected active after SetTarget")
	}
	// Advance past duration
	still := a.Tick(0.25)
	if still || a.Current != 1 {
		t.Fatalf("after tick: still=%v cur=%v", still, a.Current)
	}
}

func TestFloatAnimMidpoint(t *testing.T) {
	a := &primitive.FloatAnim{Duration: 1, Easing: primitive.EaseOutCubic}
	a.Snap(0)
	a.SetTarget(10)
	a.Tick(0.5)
	// ease-out cubic at 0.5 is > 0.5 linear → Current > 5
	if a.Current <= 5 || a.Current >= 10 {
		t.Fatalf("mid ease-out cur=%v want (5,10)", a.Current)
	}
}

func TestLerpClamp(t *testing.T) {
	if primitive.Lerp(0, 10, 0.5) != 5 {
		t.Fatal("lerp")
	}
	if primitive.Clamp01(-1) != 0 || primitive.Clamp01(2) != 1 {
		t.Fatal("clamp")
	}
}
