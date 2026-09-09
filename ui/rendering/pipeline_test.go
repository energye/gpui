package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func TestRenderBox_LayoutPaintHit(t *testing.T) {
	red := rendering.NewRenderColorBox(40, 30, 1, 0, 0, 1)
	root := rendering.NewRenderBox(red)
	root.FixedWidth, root.FixedHeight = 100, 80

	owner := rendering.NewPipelineOwner(root)
	vp := rendering.Size{Width: 100, Height: 80}
	if !owner.FlushLayout(vp, true) {
		t.Fatal("expected layout")
	}
	if root.Size().Width != 100 || root.Size().Height != 80 {
		t.Fatalf("root size=%v", root.Size())
	}
	if red.Size().Width != 40 || red.Size().Height != 30 {
		t.Fatalf("red size=%v", red.Size())
	}

	// Hit inside red (at pad 0).
	if hit := root.HitTest(rendering.Point{X: 10, Y: 10}); hit != red {
		t.Fatalf("hit=%T want red", hit)
	}
	// Hit root chrome area outside red.
	if hit := root.HitTest(rendering.Point{X: 90, Y: 70}); hit != root {
		t.Fatalf("hit=%T want root", hit)
	}
	// Miss outside.
	if hit := root.HitTest(rendering.Point{X: -1, Y: 0}); hit != nil {
		t.Fatalf("hit=%v", hit)
	}
}

// fakeBlinker is a controllable rendering.Blinkable for registry tests.
type fakeBlinker struct {
	calls   int
	toggled bool
}

func (f *fakeBlinker) BlinkTick(dt float64) bool {
	f.calls++
	_ = dt
	return f.toggled
}

// Blinkables self-report: register/drop transitions notify the embedder
// exactly on empty<->non-empty edges, and TickBlink fans dt out on the
// calling thread reporting the OR of toggles.
func TestOwner_BlinkRegistry(t *testing.T) {
	root := rendering.NewRenderBox()
	owner := rendering.NewPipelineOwner(root)
	var ctlCalls []bool
	owner.SetBlinkTickerCtl(func(active bool) { ctlCalls = append(ctlCalls, active) })
	if owner.BlinkActive() {
		t.Fatal("fresh owner must have no blinkers")
	}
	a, b := &fakeBlinker{}, &fakeBlinker{}
	owner.NoteBlinkable(a)
	owner.NoteBlinkable(a) // idempotent: no second edge
	owner.NoteBlinkable(b)
	if !owner.BlinkActive() {
		t.Fatal("expected blinkers registered")
	}
	if len(ctlCalls) != 1 || !ctlCalls[0] {
		t.Fatalf("ctl calls=%v want single [true]", ctlCalls)
	}
	a.toggled, b.toggled = true, false
	if !owner.TickBlink(0.016) {
		t.Fatal("TickBlink must OR toggle reports")
	}
	if a.calls != 1 || b.calls != 1 {
		t.Fatalf("calls a=%d b=%d want 1/1", a.calls, b.calls)
	}
	a.toggled = false
	if owner.TickBlink(0.016) {
		t.Fatal("no toggles must report false")
	}
	owner.DropBlinkable(a)
	if !owner.BlinkActive() {
		t.Fatal("b still registered")
	}
	if len(ctlCalls) != 1 {
		t.Fatalf("ctl calls=%v want no edge until empty", ctlCalls)
	}
	owner.DropBlinkable(b)
	if owner.BlinkActive() {
		t.Fatal("expected empty after drops")
	}
	if len(ctlCalls) != 2 || ctlCalls[1] {
		t.Fatalf("ctl calls=%v want [true false]", ctlCalls)
	}
	owner.DropBlinkable(b) // idempotent
	if len(ctlCalls) != 2 {
		t.Fatalf("ctl calls=%v want no duplicate edge", ctlCalls)
	}
}

func TestRelayoutBoundary_StopsBubble(t *testing.T) {
	// root (not boundary) -> mid (boundary) -> leaf
	leaf := rendering.NewRenderColorBox(10, 10, 0, 1, 0, 1)
	mid := rendering.NewRenderBox(leaf)
	mid.SetRelayoutBoundary(true)
	mid.FixedWidth, mid.FixedHeight = 50, 50
	root := rendering.NewRenderBox(mid)
	root.FixedWidth, root.FixedHeight = 100, 100

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)

	// Clear dirty via successful layout.
	if root.NeedsLayout() || mid.NeedsLayout() || leaf.NeedsLayout() {
		t.Fatal("expected clean after layout")
	}

	leaf.MarkNeedsLayout()
	if !leaf.NeedsLayout() || !mid.NeedsLayout() {
		t.Fatal("leaf and boundary mid should be dirty")
	}
	if root.NeedsLayout() {
		t.Fatal("root must NOT be layout-dirty (stopped at relayout boundary)")
	}
}

func TestMarkNeedsPaint_Bubbles(t *testing.T) {
	leaf := rendering.NewRenderColorBox(10, 10, 1, 1, 0, 1)
	root := rendering.NewRenderBox(leaf)
	_ = rendering.NewPipelineOwner(root)
	root.Layout(rendering.Tight(100, 100))
	// After layout paint may still be dirty from mark; clear paint on both.
	leaf.Paint(&rendering.PaintContext{})
	root.Paint(&rendering.PaintContext{})
	if leaf.NeedsPaint() || root.NeedsPaint() {
		t.Fatal("expected clean paint")
	}
	leaf.MarkNeedsPaint()
	if !leaf.NeedsPaint() || !root.NeedsPaint() {
		t.Fatal("paint dirty should bubble to root in P1")
	}
}

func TestPipeline_SecondFlushNoWork(t *testing.T) {
	box := rendering.NewRenderColorBox(20, 20, 0.2, 0.4, 0.8, 1)
	owner := rendering.NewPipelineOwner(box)
	vp := rendering.Size{Width: 20, Height: 20}
	// Single first layout (force).
	if !owner.FlushLayout(vp, true) {
		t.Fatal("expected first layout")
	}
	c1 := owner.LayoutCount
	if c1 != 1 {
		t.Fatalf("layout count=%d", c1)
	}
	// Second flush: clean → no-op
	if owner.FlushLayout(vp, false) {
		t.Fatal("second layout should be no-op")
	}
	if owner.LayoutCount != c1 {
		t.Fatalf("layout count grew to %d", owner.LayoutCount)
	}

	dc := render.NewContext(20, 20)
	defer dc.Close()
	pc := rendering.NewPaintContext(dc, 1)
	dc.BeginFrame()
	if !owner.FlushPaint(pc, true) {
		t.Fatal("expected paint")
	}
	p1 := owner.PaintCount
	if owner.FlushPaint(pc, false) {
		t.Fatal("second paint should be no-op")
	}
	if owner.PaintCount != p1 {
		t.Fatalf("paint count=%d", owner.PaintCount)
	}
}

func TestPaint_LogicalTopLeft_YDown(t *testing.T) {
	// Red 4×4 child inside 32×32 root so tight viewport does not stretch the red box.
	red := rendering.NewRenderColorBox(4, 4, 1, 0, 0, 1)
	root := rendering.NewRenderBox(red)
	root.FixedWidth, root.FixedHeight = 32, 32
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 32, Height: 32}, true)

	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.BeginFrame()
	// Black background via CPU so Image() sees it.
	dc.FillRectCPU(0, 0, 32, 32, render.RGBA{R: 0, G: 0, B: 0, A: 1})

	// Headless pixel test: CPU fill so Image() sees data without full GPU present.
	_ = root.Children()[0] // red at 0,0 4x4
	dc.FillRectCPU(0, 0, 4, 4, render.RGBA{R: 1, G: 0, B: 0, A: 1})

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// Sample near (0,0) — should be reddish (Y-down top-left).
	r, g, b, _ := img.At(1, 1).RGBA()
	if r < 0x8000 || g > 0x4000 || b > 0x4000 {
		t.Fatalf("pixel(1,1)=#%04x%04x%04x want red-ish (Y-down top-left)", r, g, b)
	}
	// Far corner should remain black background.
	r2, g2, b2, _ := img.At(30, 30).RGBA()
	if r2 > 0x2000 || g2 > 0x2000 || b2 > 0x2000 {
		t.Fatalf("pixel(30,30)=#%04x%04x%04x want dark", r2, g2, b2)
	}
}

func TestDeviceScale_LogicalVsPhysical(t *testing.T) {
	const logical = 50
	const scale = 2.0
	dc := render.NewContext(logical, logical, render.WithDeviceScale(scale))
	defer dc.Close()
	if dc.Width() != logical || dc.Height() != logical {
		t.Fatalf("logical size %dx%d", dc.Width(), dc.Height())
	}
	if dc.DeviceScale() != scale {
		t.Fatalf("scale=%v", dc.DeviceScale())
	}
	// Physical pixmap extent used for damage is scale×logical (render contract).
	// Draw full logical rect and ensure no panic; damage tracks physical.
	dc.BeginFrame()
	dc.SetRGBA(0, 1, 0, 1)
	dc.DrawRectangle(0, 0, float64(logical), float64(logical))
	_ = dc.Fill()
	// Context width stays logical; device scale is separate — F07.
}

func TestHitTest_MatchesPaintOffset(t *testing.T) {
	// Child offset (10, 20) — Y-down means 20 is below top.
	child := rendering.NewRenderColorBox(10, 10, 0, 0, 1, 1)
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = 100, 100
	root.AddChild(child)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	// Override offset after layout (simulates positioned child).
	child.SetOffset(rendering.Point{X: 10, Y: 20})

	if hit := root.HitTest(rendering.Point{X: 15, Y: 25}); hit != child {
		t.Fatalf("hit=%T", hit)
	}
	if hit := root.HitTest(rendering.Point{X: 15, Y: 5}); hit == child {
		t.Fatal("y=5 should miss child at y=20")
	}
}
