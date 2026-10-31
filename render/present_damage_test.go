//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"testing"
)

// TestPlanFramePresentRects pins the rects the single damage decision hands
// to the present path: clipped to the physical surface, empty rects dropped,
// touching rects coalesced (B4/P2.4e — the parallel clipPresentDamage copy
// was removed; plan.Rects feeds both the flush path and the OS compositor).
func TestPlanFramePresentRects(t *testing.T) {
	plan := PlanFramePresent([]image.Rectangle{
		image.Rect(10, 10, 20, 20),
		image.Rect(20, 10, 30, 20), // touches the first → one rect
		image.Rect(-50, -50, -10, -10),
		image.Rect(90, 90, 200, 200), // clipped to 100x100
	}, 100, 100)
	want := []image.Rectangle{
		image.Rect(10, 10, 30, 20),
		image.Rect(90, 90, 100, 100),
	}
	if plan.Mode != PresentModeDamageMulti {
		t.Fatalf("mode = %v, want DamageMulti (300px << 85%% coverage)", plan.Mode)
	}
	if len(plan.Rects) != len(want) {
		t.Fatalf("rects = %v, want %v", plan.Rects, want)
	}
	for i := range want {
		if plan.Rects[i] != want[i] {
			t.Fatalf("rects = %v, want %v", plan.Rects, want)
		}
	}

	if plan := PlanFramePresent(nil, 100, 100); plan.Mode != PresentModeIdle {
		t.Fatalf("nil input mode = %v, want Idle", plan.Mode)
	}
	if plan := PlanFramePresent([]image.Rectangle{image.Rect(10, 10, 20, 20)}, 0, 100); plan.Mode != PresentModeIdle {
		t.Fatalf("zero surface mode = %v, want Idle", plan.Mode)
	}
	if plan := PlanFramePresent([]image.Rectangle{image.Rect(-10, -10, 0, 0)}, 100, 100); plan.Mode != PresentModeIdle {
		t.Fatalf("fully outside damage mode = %v, want Idle", plan.Mode)
	}
}

// TestDamageRectsAreaSum pins the retained-cost metric: scattered dirty
// rects contribute their own pixels, not the untouched gap inside their union.
func TestDamageRectsAreaSum(t *testing.T) {
	rects := []image.Rectangle{
		image.Rect(10, 10, 30, 20),   // 200
		image.Rect(90, 90, 100, 100), // 100
	}
	if got := damageRectsAreaSum(rects); got != 300 {
		t.Fatalf("damageRectsAreaSum = %d, want 300", got)
	}
	if got := damageRectsAreaSum(nil); got != 0 {
		t.Fatalf("damageRectsAreaSum(nil) = %d, want 0", got)
	}
	if got := damageRectsAreaSum([]image.Rectangle{{Min: image.Pt(10, 10), Max: image.Pt(5, 5)}}); got != 0 {
		t.Fatalf("damageRectsAreaSum(inverted rect) = %d, want 0", got)
	}
}
