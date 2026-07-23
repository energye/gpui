package primitive

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// Principle: scroll must not mutate child layout Offset; AbsoluteBounds track -ScrollY.
func TestScrollIsPaintOffsetNotLayoutOffset(t *testing.T) {
	inner := NewBox()
	inner.Width, inner.Height = 40, 400
	col := Column(inner)
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 100
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewBox(sv)
	host.Width, host.Height = 100, 100
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 100})

	off0 := col.Base().Offset()
	abs0 := core.AbsoluteBounds(col)
	sv.SetScroll(0, 50)
	tree.Layout(core.Size{Width: 100, Height: 100}) // may re-layout Flex
	off1 := col.Base().Offset()
	abs1 := core.AbsoluteBounds(col)

	if off1 != off0 {
		// layout may reset column offset from Flex; must not be forced to -ScrollY
		if off1.Y == -sv.ScrollY && sv.ScrollY != 0 {
			t.Fatalf("scroll stored in layout Offset again: %v", off1)
		}
	}
	dy := abs0.Min.Y - abs1.Min.Y
	if dy < 49 || dy > 51 {
		t.Fatalf("AbsoluteBounds Y delta=%.2f want ~50 (paint offset); abs0=%v abs1=%v paintOff=%v",
			dy, abs0.Min, abs1.Min, sv.ContentPaintOffset())
	}
}

// Track main length equals outer height (flush top/bottom of ScrollViewport).
func TestTrackFlushWithViewportHeight(t *testing.T) {
	col := Column()
	for i := 0; i < 20; i++ {
		b := NewBox()
		b.Height = 40
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 200
	sv.Scrollbar().Horizontal = ScrollbarNever
	_ = sv.Layout(core.Tight(100, 200))
	m := sv.computeBarMetrics(true, sv.Size().Height)
	if m.trackMain != sv.Size().Height {
		t.Fatalf("trackMain=%.1f want outer height %.1f", m.trackMain, sv.Size().Height)
	}
}

// Content AbsoluteBounds.Y must track -ScrollY smoothly (no 窜 jump).
func TestContentAbsYMonotonicDuringDrag(t *testing.T) {
	col := Column()
	for i := 0; i < 40; i++ {
		b := NewBox()
		b.Height = 40
		b.Width = 80
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 120
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewBox(sv)
	host.Width, host.Height = 100, 120
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 120})

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.Dragging() {
		t.Fatal("no drag")
	}

	var contentYs, scrolls []float64
	prevY := core.AbsoluteBounds(col).Min.Y
	prevS := sv.ScrollY
	for i := 0; i <= 40; i++ {
		sv.HandlePointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + float64(i)*10, Button: core.ButtonLeft,
		})
		// adversarial layout
		host.MarkNeedsLayout()
		tree.Frame(&core.PaintContext{}, core.Size{Width: 100, Height: 120})
		y := core.AbsoluteBounds(col).Min.Y
		contentYs = append(contentYs, y)
		scrolls = append(scrolls, sv.ScrollY)
		// When scroll increases, content Y must decrease (or stay) — no jump up.
		if sv.ScrollY >= prevS-0.01 && y > prevY+0.5 {
			t.Fatalf("content 窜 up while scrolling down i=%d y=%.2f→%.2f scroll=%.2f→%.2f",
				i, prevY, y, prevS, sv.ScrollY)
		}
		// Content ΔY should match -ΔScroll (paint offset principle)
		ds := sv.ScrollY - prevS
		dy := prevY - y
		if ds > 0.5 && absF(dy-ds) > 1.0 {
			t.Fatalf("content not tracking ScrollY i=%d Δy=%.2f Δscroll=%.2f", i, dy, ds)
		}
		prevY, prevS = y, sv.ScrollY
	}
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
