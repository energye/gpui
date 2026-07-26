package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Off-screen flex children must not be painted when Clip is set (scroll cull).
func TestPaintCullsOffscreenChildren(t *testing.T) {
	var paints int
	mk := func(h float64) *countingBox {
		b := &countingBox{}
		b.Init(b)
		b.Width, b.Height = 40, h
		b.onPaint = func() { paints++ }
		return b
	}
	top := mk(50)
	mid := mk(50)
	bot := mk(50)
	col := primitive.Column(top, mid, bot)
	col.Gap = 0
	host := primitive.NewBox(col)
	host.Width, host.Height = 40, 150
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 40, Height: 150})

	// Clip only the middle 50px of the column (absolute y 50..100).
	// After layout: top@0, mid@50, bot@100.
	pc := &core.PaintContext{
		Clip:  core.NewRect(0, 50, 40, 50),
		Scale: 1,
	}
	// Origin of host is 0; paint through host so DefaultPaintChildren sees Clip.
	// Force paint by clearing composite-only path.
	host.Paint(pc)
	// Only mid intersects clip; top ends at 50 (empty intersect with [50,100) if
	// edges touch — Rect intersect: top Max.Y=50, clip Min.Y=50 → empty if half-open
	// semantics. Our Intersect uses max/min; if Equal edges give zero height → empty.
	// mid fully in; bot at 100 may be empty at edge.
	if paints < 1 {
		t.Fatalf("expected at least mid painted, paints=%d", paints)
	}
	if paints > 2 {
		t.Fatalf("expected cull of off-screen rows, paints=%d (want ≤2)", paints)
	}
}

type countingBox struct {
	primitive.Box
	onPaint func()
}

func (b *countingBox) Paint(pc *core.PaintContext) {
	if b.onPaint != nil {
		b.onPaint()
	}
	b.Box.Paint(pc)
}

// Deep PushClipLocal must keep updating advisory Clip past the old fixed depth of 6.
func TestPushClipLocal_DeepNestingUpdatesClip(t *testing.T) {
	pc := &core.PaintContext{
		Clip:  core.NewRect(0, 0, 1000, 1000),
		Scale: 1,
	}
	const depth = 12
	for i := 0; i < depth; i++ {
		// Shrink by 1px each side so the final clip is strictly nested.
		pc.PushClipLocal(float64(i), float64(i), 1000-2*float64(i), 1000-2*float64(i))
		if pc.ClipDepth() != i+1 {
			t.Fatalf("depth after push %d: got %d", i+1, pc.ClipDepth())
		}
	}
	want := core.NewRect(float64(depth-1), float64(depth-1), 1000-2*float64(depth-1), 1000-2*float64(depth-1))
	// After depth nested clips from origin 0, Clip Min should be (depth-1, depth-1)
	// only if each push used absolute origin 0 — PushClipLocal is origin-relative,
	// and Origin stays 0, so successive local (i,i) are absolute (i,i). Intersect
	// of [0,1000] ∩ [0,1000] ∩ [1,999] ∩ … ends at last rect when nested properly.
	got := pc.Clip
	if got.Min.X != want.Min.X || got.Min.Y != want.Min.Y {
		t.Fatalf("deep clip Min=%v want %v (stale cull would keep earlier Min)", got.Min, want.Min)
	}
	if got.Width() != want.Width() || got.Height() != want.Height() {
		t.Fatalf("deep clip size=%vx%v want %vx%v", got.Width(), got.Height(), want.Width(), want.Height())
	}

	// Child context must not share clip-stack backing with parent.
	child := pc.WithOrigin(core.Point{X: 10, Y: 10})
	child.PushClipLocal(0, 0, 50, 50)
	if pc.ClipDepth() != depth {
		t.Fatalf("parent depth mutated by child push: %d want %d", pc.ClipDepth(), depth)
	}
	child.Pop()
	if child.ClipDepth() != depth {
		t.Fatalf("child depth after pop: %d want %d", child.ClipDepth(), depth)
	}

	for i := depth; i > 0; i-- {
		pc.Pop()
	}
	if pc.ClipDepth() != 0 {
		t.Fatalf("depth after full pop: %d", pc.ClipDepth())
	}
	// Restored outermost saved clip was the initial 0,0,1000,1000.
	if pc.Clip.Min.X != 0 || pc.Clip.Min.Y != 0 || pc.Clip.Width() != 1000 || pc.Clip.Height() != 1000 {
		t.Fatalf("restored clip=%v want 1000x1000 at origin", pc.Clip)
	}
}
