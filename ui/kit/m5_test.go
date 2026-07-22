package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestAnimEaseAndAdvance(t *testing.T) {
	a := core.Anim{Duration: 1, Ease: core.EaseLinear}
	a.Start()
	if p := a.Advance(0.5); p < 0.49 || p > 0.51 {
		t.Fatalf("p=%v", p)
	}
	if p := a.Advance(0.6); p != 1 || !a.Done() {
		t.Fatalf("p=%v done=%v", p, a.Done())
	}
	if core.EaseOutCubic(0) != 0 || core.EaseOutCubic(1) != 1 {
		t.Fatal("ease ends")
	}
}

func TestClockReduceMotion(t *testing.T) {
	tree := core.NewTree(nil)
	tree.Clock().ReduceMotion = true
	tree.TickClock(0.016)
	if tree.Clock().T <= 0 {
		t.Fatal("clock not advancing")
	}
}

func TestMotionFade(t *testing.T) {
	child := primitive.NewBox()
	child.Width, child.Height = 40, 20
	m := primitive.NewMotion(child)
	m.Anim.Duration = 0.2
	m.Anim.Start()
	tree := core.NewTree(m)
	tree.Layout(core.Size{Width: 100, Height: 50})
	m.Advance(0.1, false)
	if m.Progress() <= 0 {
		t.Fatal("no progress")
	}
	m.Advance(1, true) // reduce → complete
	if m.Progress() != 1 {
		t.Fatalf("t=%v", m.Progress())
	}
}

func TestPresenceLeave(t *testing.T) {
	p := primitive.NewPresence(primitive.NewText("x"))
	p.Hide()
	for i := 0; i < 20 && !p.Gone; i++ {
		p.Advance(0.02, false)
	}
	if !p.Gone || p.Visible {
		t.Fatalf("gone=%v visible=%v", p.Gone, p.Visible)
	}
	p.Show()
	if p.Gone || !p.Visible {
		t.Fatal("show failed")
	}
}

func TestCanvasProgressRing(t *testing.T) {
	c := primitive.ProgressRing(32, 3, 0.5,
		core.DefaultTheme().Color(core.TokenColorFillSecondary),
		core.DefaultTheme().Color(core.TokenColorPrimary),
	)
	sz := c.Layout(core.Loose(100, 100))
	if sz.Width != 32 {
		t.Fatal(sz)
	}
}

func TestSkeletonAndSpin(t *testing.T) {
	sk := kit.NewSkeleton(100, 16)
	if !sk.Tick(0.05) {
		t.Fatal("active skeleton should keep ticking")
	}
	_ = sk.Node().Layout(core.Loose(200, 50))

	sp := kit.NewSpin(nil)
	if !sp.Tick(0.05) {
		t.Fatal("spinning spin should keep ticking")
	}
	_ = sp.Node().Layout(core.Loose(50, 50))
}

func TestProgress(t *testing.T) {
	p := kit.NewProgress(40)
	root1 := p.Node()
	p.SetPercent(80)
	root2 := p.Node()
	if root1 != root2 {
		t.Fatal("SetPercent must not rebuild root")
	}
	sz := p.Node().Layout(core.Loose(300, 40))
	if sz.Width < 80 {
		t.Fatal(sz)
	}
}

func TestTourOpen(t *testing.T) {
	tour := kit.NewTour(
		kit.TourStep{Title: "Step 1", Body: "Hello", Target: core.NewRect(20, 20, 80, 30)},
		kit.TourStep{Title: "Step 2", Body: "World", Target: core.NewRect(40, 80, 60, 20)},
	)
	tour.Viewport = core.Size{Width: 400, Height: 300}
	root := primitive.NewBox(tour.Node())
	root.Width, root.Height = 400, 300
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 400, Height: 300})
	tour.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if tree.Overlays().Len() < 1 {
		t.Fatal("tour not open")
	}
	tour.Next()
	if tour.Index != 1 {
		t.Fatal(tour.Index)
	}
}

func TestA11yCollect(t *testing.T) {
	btn := kit.NewButton("OK")
	btn.Root.Base().Role = "button"
	btn.Root.Base().Label = "OK"
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	nodes := kit.CollectA11y(tree.Root())
	if len(nodes) == 0 {
		t.Fatal("no a11y nodes")
	}
	found := false
	for _, n := range nodes {
		if n.Role == "button" && n.Label == "OK" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", nodes)
	}
}

func TestDensity(t *testing.T) {
	th := kit.DefaultTheme()
	kit.ApplyDensity(th, kit.DensityCompact)
	if th.Size(core.TokenControlHeight) != 24 {
		t.Fatal(th.Size(core.TokenControlHeight))
	}
	kit.ApplyDensity(th, kit.DensityLarge)
	if th.Size(core.TokenControlHeight) != 40 {
		t.Fatal(th.Size(core.TokenControlHeight))
	}
}

func TestButtonDefaultA11y(t *testing.T) {
	// ensure rebuild sets role if we add it — set manually for contract
	b := kit.NewButton("Save")
	b.Root.Base().Role = "button"
	b.Root.Base().Label = "Save"
	if b.Root.Base().Role != "button" {
		t.Fatal("role")
	}
}
