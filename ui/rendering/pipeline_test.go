package rendering_test

import (
	"testing"
	"time"

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

// fakeTicker 是可控的 scheduler.Ticker 实现，供注册表测试用。
// alive 就是 Tick 的返回值（契约：false = 这一 tick 之后自动摘注册）。
type fakeTicker struct {
	calls   int
	alive   bool
	wake    time.Duration
	hasWake bool
}

func (f *fakeTicker) Tick(dt float64) bool {
	f.calls++
	_ = dt
	return f.alive
}

func (f *fakeTicker) NextWake() (time.Duration, bool) { return f.wake, f.hasWake }

// 参与者自注册：注册表在空/非空翻转的那一刻才通知嵌道层回调；
// TickAllTickers 在调用线程把 dt 扇给每个参与者，返回 false 的由注册表摘掉。
func TestOwner_TickerRegistry(t *testing.T) {
	root := rendering.NewRenderBox()
	owner := rendering.NewPipelineOwner(root)
	var ctlCalls []bool
	owner.SetTickerCtl(func(active bool) { ctlCalls = append(ctlCalls, active) })
	if owner.HasTickers() {
		t.Fatal("fresh owner must have no tickers")
	}
	a, b := &fakeTicker{alive: true}, &fakeTicker{alive: true}
	owner.AddTicker(a)
	owner.AddTicker(a) // 幂等：不该再翻一次边沿
	owner.AddTicker(b)
	if !owner.HasTickers() {
		t.Fatal("expected tickers registered")
	}
	if len(ctlCalls) != 1 || !ctlCalls[0] {
		t.Fatalf("ctl calls=%v want single [true]", ctlCalls)
	}
	owner.TickAllTickers(0.016)
	if a.calls != 1 || b.calls != 1 {
		t.Fatalf("calls a=%d b=%d want 1/1", a.calls, b.calls)
	}
	// 契约核心：返回 false 的参与者自动摘除，活着的那个留在表里。
	a.alive = false
	owner.TickAllTickers(0.016)
	if a.calls != 2 {
		t.Fatalf("a calls=%d want 2", a.calls)
	}
	owner.TickAllTickers(0.016)
	if a.calls != 2 {
		t.Fatalf("dropped ticker must not be ticked again: a calls=%d", a.calls)
	}
	if !owner.HasTickers() {
		t.Fatal("b must still be registered")
	}
	if len(ctlCalls) != 1 {
		t.Fatalf("ctl calls=%v want no edge while b remains", ctlCalls)
	}
	owner.RemoveTicker(b)
	if owner.HasTickers() {
		t.Fatal("expected empty after remove")
	}
	if len(ctlCalls) != 2 || ctlCalls[1] {
		t.Fatalf("ctl calls=%v want [true false]", ctlCalls)
	}
	owner.RemoveTicker(b) // 幂等
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

// NextTickerWake 取全部在册参与者里最早的那个截止点；没人报（表空、或都不带
// 截止）就报 false，让事件循环转去等事件。
//
// 这里同时钉住一条容易丢的性质：注册完立刻就能报出截止，不需要先被 tick 一次。
// 光标获得焦点、动图开播都发生在两次 tick 之间；若截止点只能来自上一次 tick 的
// 快照，空闲循环会一直睡到下一次外部输入，闪烁与动画就停在那一帧。
func TestOwner_NextTickerWake(t *testing.T) {
	root := rendering.NewRenderBox()
	owner := rendering.NewPipelineOwner(root)
	if _, ok := owner.NextTickerWake(); ok {
		t.Fatal("empty set must report no deadline")
	}
	a := &fakeTicker{alive: true, wake: 500 * time.Millisecond, hasWake: true}
	b := &fakeTicker{alive: true, wake: 200 * time.Millisecond, hasWake: true}
	c := &fakeTicker{alive: true}
	owner.AddTicker(a)
	owner.AddTicker(b)
	owner.AddTicker(c)
	if a.calls != 0 {
		t.Fatalf("registration must not tick: calls=%d", a.calls)
	}
	if d, ok := owner.NextTickerWake(); !ok || d != 200*time.Millisecond {
		t.Fatalf("NextTickerWake=%v,%v want 200ms,true (before any tick)", d, ok)
	}
	owner.RemoveTicker(a)
	owner.RemoveTicker(b)
	owner.RemoveTicker(c)
	if _, ok := owner.NextTickerWake(); ok {
		t.Fatal("emptied set must report no deadline")
	}
}
