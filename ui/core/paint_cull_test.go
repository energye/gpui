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
