package rendering_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// TestClampingScrollPhysics_AdjustPosition hard-clamps to range.
func TestClampingScrollPhysics_AdjustPosition(t *testing.T) {
	p := rendering.DefaultClampingScrollPhysics()
	if g := p.AdjustPosition(-10, 0, 100); g != 0 {
		t.Fatalf("below min got %v", g)
	}
	if g := p.AdjustPosition(150, 0, 100); g != 100 {
		t.Fatalf("above max got %v", g)
	}
	if g := p.AdjustPosition(40, 0, 100); g != 40 {
		t.Fatalf("interior got %v", g)
	}
}

// TestClampingScrollPhysics_BallisticDeceleratesAndStops: fling velocity decays
// and ends at rest within bounds.
func TestClampingScrollPhysics_BallisticDeceleratesAndStops(t *testing.T) {
	p := &rendering.ClampingScrollPhysics{Deceleration: 2000, Tolerance: 30}
	sim := p.CreateBallistic(800, 50, 0, 500)
	if sim == nil {
		t.Fatal("expected ballistic for v=800")
	}
	pos := 50.0
	steps := 0
	for steps < 500 {
		var done bool
		pos, done = sim.Step(1.0 / 60)
		steps++
		if done {
			break
		}
	}
	if steps >= 500 {
		t.Fatal("ballistic did not finish")
	}
	if pos < 50 {
		t.Fatalf("positive velocity should increase scrollY, pos=%v", pos)
	}
	if pos > 500 {
		t.Fatalf("must stop at max, pos=%v", pos)
	}
	if math.Abs(sim.Velocity()) > 30 {
		t.Fatalf("velocity should be near 0, got %v", sim.Velocity())
	}
}

// TestClampingScrollPhysics_BallisticHitsMaxBoundary.
func TestClampingScrollPhysics_BallisticHitsMaxBoundary(t *testing.T) {
	p := &rendering.ClampingScrollPhysics{Deceleration: 500, Tolerance: 20}
	// Near max with high velocity → should hit 100 quickly.
	sim := p.CreateBallistic(2000, 90, 0, 100)
	if sim == nil {
		t.Fatal("nil sim")
	}
	pos := 90.0
	for i := 0; i < 120; i++ {
		var done bool
		pos, done = sim.Step(1.0 / 60)
		if done {
			break
		}
	}
	if pos != 100 {
		t.Fatalf("pos=%v want 100 (clamped at max)", pos)
	}
}

// TestViewport_PhysicsClampAndFling drives shipped RenderViewport path.
func TestViewport_PhysicsClampAndFling(t *testing.T) {
	content := rendering.NewRenderColorBox(100, 2000, 0.2, 0.3, 0.4, 1)
	vp := rendering.NewRenderViewport(content)
	vp.SetPhysics(rendering.DefaultClampingScrollPhysics())
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 200}, true)

	maxY := vp.MaxScrollY()
	if maxY < 100 {
		t.Fatalf("maxScrollY=%v want large content", maxY)
	}

	// Clamp via SetScrollOffset.
	vp.SetScrollOffset(0, -50)
	if vp.ScrollOffset().Y != 0 {
		t.Fatalf("clamp min got %v", vp.ScrollOffset().Y)
	}
	vp.SetScrollOffset(0, maxY+100)
	if vp.ScrollOffset().Y != maxY {
		t.Fatalf("clamp max got %v want %v", vp.ScrollOffset().Y, maxY)
	}

	// Fling from mid.
	vp.SetScrollOffset(0, 100)
	vp.Fling(1200)
	if !vp.HasBallistic() {
		t.Fatal("Fling should start ballistic")
	}
	start := vp.ScrollOffset().Y
	running := true
	for i := 0; i < 300 && running; i++ {
		running = vp.TickPhysics(1.0 / 60)
	}
	end := vp.ScrollOffset().Y
	if end <= start {
		t.Fatalf("fling should advance scrollY start=%v end=%v", start, end)
	}
	if end > maxY {
		t.Fatalf("fling exceeded max end=%v max=%v", end, maxY)
	}
	if vp.HasBallistic() {
		t.Fatal("ballistic should have finished")
	}
}

// TestViewport_NilPhysicsNoFling: historical hard clamp, Fling is no-op.
func TestViewport_NilPhysicsNoFling(t *testing.T) {
	content := rendering.NewRenderColorBox(50, 500, 0.3, 0.3, 0.3, 1)
	vp := rendering.NewRenderViewport(content)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 50, Height: 100}, true)
	vp.SetScrollOffset(0, 50)
	vp.Fling(9999)
	if vp.HasBallistic() {
		t.Fatal("nil Physics must not fling")
	}
	if vp.ScrollOffset().Y != 50 {
		t.Fatal("offset changed without physics tick")
	}
}

// TestNeverScrollPhysics_NoBallistic.
func TestNeverScrollPhysics_NoBallistic(t *testing.T) {
	var p rendering.NeverScrollPhysics
	if p.CreateBallistic(1000, 0, 0, 100) != nil {
		t.Fatal("NeverScrollPhysics must not fling")
	}
	if p.AdjustPosition(50, 0, 100) != 50 {
		t.Fatal("adjust interior")
	}
}
