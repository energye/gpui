package app_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/energye/gpui/ui/app"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
)

type leaf struct {
	core.NodeBase
	w, h   float64
	paints int
}

func newLeaf(w, h float64) *leaf {
	n := &leaf{w: w, h: h}
	n.Init(n)
	n.Hit = core.HitTarget
	return n
}

func (n *leaf) TypeID() string { return "test.leaf" }
func (n *leaf) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: n.w, Height: n.h})
	n.SetSize(out)
	return out
}
func (n *leaf) Paint(*core.PaintContext) { n.paints++ }
func (n *leaf) HitTest(p core.Point) core.Node {
	return n.DefaultHitTest(p)
}

type paintTicker struct {
	left int
	tree *core.Tree
}

func (t *paintTicker) Tick(dt float64) bool {
	t.left--
	if t.tree != nil {
		t.tree.MarkDirty()
	}
	return t.left > 0
}

func TestDemandAnimatingWithoutDirtyDoesNotPaint(t *testing.T) {
	host := platform.NewHeadless(100, 80)
	root := newLeaf(100, 80)
	tree := core.NewTree(root)
	a := app.New(app.Options{AnimTick: time.Millisecond})
	defer a.Close()
	a.Attach(host, tree, nil)

	// Initial paint (Attach RequestRedraw).
	if !a.Pulse() {
		t.Fatal("pulse")
	}
	if root.paints < 1 {
		t.Fatalf("expected initial paint, got %d", root.paints)
	}
	before := root.paints

	// ANIMATING without dirty → onUpdate/tick only, no OnDraw (gogpu).
	a.StartAnimation()
	defer a.StopAnimation()
	if tree.Dirty() {
		// ensure clean
		tree.Frame(&core.PaintContext{}, core.Size{Width: 100, Height: 80})
	}
	if !a.Pulse() {
		t.Fatal("pulse2")
	}
	if root.paints != before {
		t.Fatalf("ANIMATING without dirty must not paint: before=%d after=%d", before, root.paints)
	}

	a.RequestRedraw()
	if !a.Pulse() {
		t.Fatal("pulse3")
	}
	if root.paints <= before {
		t.Fatal("RequestRedraw should paint")
	}
}

func TestTickerDrivesRedraw(t *testing.T) {
	host := platform.NewHeadless(64, 64)
	root := newLeaf(64, 64)
	tree := core.NewTree(root)
	a := app.New(app.Options{AnimTick: time.Millisecond})
	defer a.Close()
	a.Attach(host, tree, nil)
	a.Pulse() // initial
	base := root.paints

	tk := &paintTicker{left: 3, tree: tree}
	tree.AddTicker(tk)
	for i := 0; i < 8 && (tree.HasActiveTickers() || tree.Dirty()); i++ {
		if !a.Pulse() {
			t.Fatal("pulse")
		}
	}
	if root.paints <= base {
		t.Fatalf("ticker should have caused paints: base=%d now=%d", base, root.paints)
	}
	if tree.HasActiveTickers() {
		t.Fatal("ticker should finish")
	}
}

func TestEventRedrawMarksDirty(t *testing.T) {
	host := platform.NewHeadless(50, 50)
	root := newLeaf(50, 50)
	tree := core.NewTree(root)
	a := app.New(app.Options{AnimTick: time.Millisecond})
	defer a.Close()
	a.Attach(host, tree, nil)
	a.Pulse()
	base := root.paints
	// Clear dirty without paint path
	if tree.Dirty() {
		tree.Frame(&core.PaintContext{}, core.Size{Width: 50, Height: 50})
	}

	host.RequestRedraw() // EventRedraw via queue
	a.StartAnimation()   // avoid blocking WaitEvents(-1)
	defer a.StopAnimation()
	if !a.Pulse() {
		t.Fatal("pulse")
	}
	if root.paints <= base {
		t.Fatal("EventRedraw should paint")
	}
}

func TestContinuousRenderAlwaysPaints(t *testing.T) {
	host := platform.NewHeadless(40, 40)
	root := newLeaf(40, 40)
	tree := core.NewTree(root)
	a := app.New(app.Options{ContinuousRender: true, AnimTick: time.Millisecond})
	defer a.Close()
	a.Attach(host, tree, nil)
	a.Pulse()
	a.Pulse()
	a.Pulse()
	if root.paints < 3 {
		t.Fatalf("continuous paints=%d", root.paints)
	}
}

func TestQuitUnblocksRun(t *testing.T) {
	host := platform.NewHeadless(32, 32)
	tree := core.NewTree(newLeaf(32, 32))
	a := app.New(app.Options{})
	defer a.Close()
	a.Attach(host, tree, nil)

	var ran atomic.Bool
	go func() {
		ran.Store(true)
		a.Run()
	}()
	time.Sleep(20 * time.Millisecond)
	a.Quit()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !a.Running() && ran.Load() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("Run did not exit after Quit")
}

func TestPresentRunsOnRenderThread(t *testing.T) {
	host := platform.NewHeadless(48, 48)
	tree := core.NewTree(newLeaf(48, 48))
	a := app.New(app.Options{AnimTick: time.Millisecond})
	defer a.Close()

	var presentGID atomic.Uint64
	a.Attach(host, tree, func(s *app.Session) error {
		// Record that we are not on the caller's stack synchronously without hop
		// by checking hop counter and that Frame cleared dirty.
		presentGID.Store(1)
		if s.Tree != nil {
			s.Tree.Frame(&core.PaintContext{}, core.Size{Width: 48, Height: 48})
		}
		return nil
	})
	if a.RenderLoop() == nil || !a.RenderLoop().IsRunning() {
		t.Fatal("render loop should be running by default")
	}
	if !a.Pulse() {
		t.Fatal("pulse")
	}
	if a.RenderThreadHops.Load() < 1 {
		t.Fatal("expected present hop onto render thread")
	}
	if presentGID.Load() != 1 {
		t.Fatal("present not called")
	}

	// DisableRenderThread: no hops
	a2 := app.New(app.Options{DisableRenderThread: true, AnimTick: time.Millisecond})
	defer a2.Close()
	root2 := newLeaf(20, 20)
	a2.Attach(platform.NewHeadless(20, 20), core.NewTree(root2), nil)
	a2.Pulse()
	if a2.RenderThreadHops.Load() != 0 {
		t.Fatal("DisableRenderThread should not hop")
	}
	if root2.paints < 1 {
		t.Fatal("still paints on main")
	}
}

func TestDispatchRedrawDirty(t *testing.T) {
	tree := core.NewTree(newLeaf(10, 10))
	tree.Frame(&core.PaintContext{}, core.Size{Width: 10, Height: 10})
	if tree.Dirty() {
		t.Fatal("expected clean")
	}
	platform.Dispatch(tree, platform.Event{Type: platform.EventRedraw})
	if !tree.Dirty() {
		t.Fatal("EventRedraw must MarkDirty")
	}
}
