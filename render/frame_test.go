package render

import (
	"image"
	"testing"
)

func TestPlanFramePresent_IdleEmpty(t *testing.T) {
	plan := PlanFramePresent(nil, 800, 480)
	if plan.Mode != PresentModeIdle {
		t.Fatalf("mode=%v want idle", plan.Mode)
	}
	plan = PlanFramePresent([]image.Rectangle{}, 800, 480)
	if plan.Mode != PresentModeIdle {
		t.Fatalf("empty mode=%v want idle", plan.Mode)
	}
	plan = PlanFramePresent([]image.Rectangle{image.Rect(0, 0, 10, 10)}, 0, 0)
	if plan.Mode != PresentModeIdle {
		t.Fatalf("zero surface mode=%v want idle", plan.Mode)
	}
}

func TestPlanFramePresent_SingleUnion(t *testing.T) {
	r := image.Rect(10, 20, 100, 80)
	plan := PlanFramePresent([]image.Rectangle{r}, 800, 480)
	if plan.Mode != PresentModeDamageUnion {
		t.Fatalf("mode=%v want damage_union", plan.Mode)
	}
	if plan.Union != r {
		t.Fatalf("union=%v want %v", plan.Union, r)
	}
}

func TestPlanFramePresent_DistantMulti(t *testing.T) {
	// Two small distant widgets: union waste is large → multi.
	a := image.Rect(10, 10, 50, 50)
	b := image.Rect(700, 400, 760, 460)
	plan := PlanFramePresent([]image.Rectangle{a, b}, 800, 480)
	if plan.Mode != PresentModeDamageMulti {
		t.Fatalf("mode=%v want damage_multi (plan=%+v)", plan.Mode, plan)
	}
	if len(plan.Rects) != 2 {
		t.Fatalf("rects=%d want 2", len(plan.Rects))
	}
}

func TestPlanFramePresent_ClusteredUnion(t *testing.T) {
	// Overlapping / tightly clustered rects → union.
	a := image.Rect(100, 100, 180, 140)
	b := image.Rect(120, 110, 200, 150)
	plan := PlanFramePresent([]image.Rectangle{a, b}, 800, 480)
	if plan.Mode != PresentModeDamageUnion {
		t.Fatalf("mode=%v want damage_union (plan=%+v)", plan.Mode, plan)
	}
	if len(plan.Rects) != 1 {
		t.Fatalf("rects=%d want 1 union", len(plan.Rects))
	}
}

func TestPlanFramePresent_HighCoverageFull(t *testing.T) {
	// Nearly the whole surface dirty → full redraw path.
	r := image.Rect(0, 0, 800, 420) // 87.5% of 800×480
	plan := PlanFramePresent([]image.Rectangle{r}, 800, 480)
	if plan.Mode != PresentModeFull {
		t.Fatalf("mode=%v want full (union area ratio high)", plan.Mode)
	}
	if plan.Union != image.Rect(0, 0, 800, 480) {
		t.Fatalf("full union=%v want surface", plan.Union)
	}
}

func TestCoalesceDamageRects_MergesTouching(t *testing.T) {
	a := image.Rect(0, 0, 10, 10)
	b := image.Rect(10, 0, 20, 10) // edge-adjacent
	out := CoalesceDamageRects([]image.Rectangle{a, b}, MaxTrackedDamageRects)
	if len(out) != 1 {
		t.Fatalf("coalesced len=%d want 1: %v", len(out), out)
	}
	if out[0] != image.Rect(0, 0, 20, 10) {
		t.Fatalf("merged=%v", out[0])
	}
}

func TestCoalesceDamageRects_CapsToUnion(t *testing.T) {
	var rects []image.Rectangle
	for i := 0; i < 8; i++ {
		// far apart so they do not touch
		x := i * 40
		rects = append(rects, image.Rect(x, 0, x+5, 5))
	}
	out := CoalesceDamageRects(rects, 3)
	if len(out) != 1 {
		t.Fatalf("cap len=%d want 1 union, got %v", len(out), out)
	}
}

func TestBeginFrame_ClearsDamage(t *testing.T) {
	dc := NewContext(100, 80)
	defer dc.Close()
	dc.Invalidate(image.Rect(1, 2, 30, 40))
	if len(dc.FrameDamage()) == 0 {
		t.Fatal("expected damage before BeginFrame")
	}
	dc.BeginFrame()
	if len(dc.FrameDamage()) != 0 {
		t.Fatalf("BeginFrame should clear damage, got %v", dc.FrameDamage())
	}
}

func TestInvalidate_HiDPI_Physical(t *testing.T) {
	dc := NewContextWithScale(100, 80, 2.0)
	defer dc.Close()
	dc.BeginFrame()
	dc.Invalidate(image.Rect(10, 20, 40, 50))
	rects := dc.FrameDamage()
	if len(rects) != 1 {
		t.Fatalf("rects=%d want 1", len(rects))
	}
	want := image.Rect(20, 40, 80, 100)
	if rects[0] != want {
		t.Fatalf("physical damage=%v want %v", rects[0], want)
	}
}

func TestMarkFullRedraw_LogicalSurface(t *testing.T) {
	dc := NewContext(200, 100)
	defer dc.Close()
	dc.BeginFrame()
	dc.MarkFullRedraw()
	plan := dc.PlanPresent(200, 100)
	if plan.Mode != PresentModeFull {
		t.Fatalf("mode=%v want full after MarkFullRedraw", plan.Mode)
	}
}

func TestPresentMode_String(t *testing.T) {
	if PresentModeIdle.String() != "idle" {
		t.Fatal(PresentModeIdle.String())
	}
	if PresentModeFull.String() != "full" {
		t.Fatal(PresentModeFull.String())
	}
}
