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

// TestPlanFramePresent_WasteRatioBoundary pins the DamageMultiWasteRatio
// decision boundary (dirty-region P2): union area ≥ sum×1.35 → independent
// multi-rect scissors; just below → one union scissor. Exact-equality lands
// on multi (>=). Rects are ≥2px apart so CoalesceDamageRects cannot merge
// them (edge-adjacent/intersecting merge would change the inputs).
func TestPlanFramePresent_WasteRatioBoundary(t *testing.T) {
	// Exact boundary: two 10x10 rects, 7px gap → union box 27x10 = 270 px,
	// sum = 200 px, 200×1.35 = 270 → equality → multi.
	plan := PlanFramePresent([]image.Rectangle{
		image.Rect(0, 0, 10, 10),
		image.Rect(17, 0, 27, 10),
	}, 100, 100)
	if plan.Mode != PresentModeDamageMulti {
		t.Fatalf("mode = %v at union==sum*1.35 (270==270), want DamageMulti (>= boundary)", plan.Mode)
	}
	if len(plan.Rects) != 2 {
		t.Fatalf("rects = %v, want 2 independent scissors", plan.Rects)
	}

	// Just below the boundary: two rects, union 30x13 = 390 px, sum = 290 px,
	// 290×1.35 = 391.5 → 390 < 391.5 → one union scissor.
	plan = PlanFramePresent([]image.Rectangle{
		image.Rect(0, 0, 20, 10),
		image.Rect(21, 3, 30, 13),
	}, 100, 100)
	if plan.Mode != PresentModeDamageUnion {
		t.Fatalf("mode = %v at union 390 < sum*1.35 391.5, want DamageUnion", plan.Mode)
	}
	if len(plan.Rects) != 1 || plan.Rects[0] != image.Rect(0, 0, 30, 13) {
		t.Fatalf("rects = %v, want single union (0,0,30,13)", plan.Rects)
	}

	// Well above the boundary: distant corners stay multi (the R4b posture —
	// two far-apart hot spots must not collapse into a near-full union).
	plan = PlanFramePresent([]image.Rectangle{
		image.Rect(0, 0, 20, 10),
		image.Rect(25, 3, 34, 13),
	}, 100, 100)
	if plan.Mode != PresentModeDamageMulti {
		t.Fatalf("mode = %v at union 442 ≥ sum*1.35 391.5, want DamageMulti", plan.Mode)
	}

	// Coverage boundary on a single rect (DamageFullCoverageThreshold 0.85):
	// exactly 85% of the surface → full; just below → union scissor.
	plan = PlanFramePresent([]image.Rectangle{image.Rect(0, 0, 100, 85)}, 100, 100)
	if plan.Mode != PresentModeFull {
		t.Fatalf("mode = %v at single-rect coverage 85%%, want Full (>= boundary)", plan.Mode)
	}
	plan = PlanFramePresent([]image.Rectangle{image.Rect(0, 0, 100, 84)}, 100, 100)
	if plan.Mode != PresentModeDamageUnion {
		t.Fatalf("mode = %v at single-rect coverage 84%%, want DamageUnion", plan.Mode)
	}
}
