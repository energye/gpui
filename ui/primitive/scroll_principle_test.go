package primitive

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/core"
)

// Principle integration: absolute pointer formula + paint offset content tracking.
// No Tree.Layout between moves — pure gesture math (isolates ScrollViewport).
func TestPrincipleDragLinearNoJump(t *testing.T) {
	col := Column()
	for i := 0; i < 40; i++ {
		b := NewBox()
		b.Height = 40
		b.Width = 80
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 150
	sv.Scrollbar().Horizontal = ScrollbarNever
	_ = sv.Layout(core.Tight(100, 150))

	// Synthetic screen coords: widget at (0,0)
	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	downY := y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: 95, Y: downY, Button: core.ButtonLeft})
	if !sv.Dragging() {
		t.Fatal("drag not started")
	}
	travel := sv.drag.trackMain - sv.drag.thumbMain
	maxS := sv.drag.maxScroll
	if travel < 1 || maxS < 1 {
		t.Fatalf("travel=%.1f max=%.1f", travel, maxS)
	}

	var scrolls []float64
	prev := sv.ScrollY
	for i := 0; i <= 50; i++ {
		// Move pointer by exactly i * 4 px down
		y := downY + float64(i)*4
		sv.HandlePointer(&core.PointerEvent{Type: core.PointerMove, X: 95, Y: y, Button: core.ButtonLeft})
		// Expected pure principle
		want := sv.drag.scroll0 + (y-sv.drag.ptr0Abs)/travel*maxS
		if want < 0 {
			want = 0
		}
		if want > maxS {
			want = maxS
		}
		got := sv.ScrollY
		if math.Abs(got-want) > 0.01 {
			t.Fatalf("i=%d ScrollY=%.4f want principle %.4f (Δptr=%.1f)", i, got, want, y-sv.drag.ptr0Abs)
		}
		if got+0.01 < prev && want > 0 {
			t.Fatalf("ScrollY went backwards: %v → %v", prev, got)
		}
		// No discrete jump larger than one step would allow (+ a little float)
		step := 4 / travel * maxS
		if i > 0 && got-prev > step+0.5 && got < maxS-0.5 {
			t.Fatalf("ScrollY jump i=%d Δ=%.3f maxStep=%.3f", i, got-prev, step)
		}
		scrolls = append(scrolls, got)
		prev = got
	}
	_ = scrolls
}

// With adversarial Layout every move, ScrollY must still match absolute-pointer principle.
func TestPrincipleDragLinearUnderLayoutThrash(t *testing.T) {
	col := Column()
	for i := 0; i < 40; i++ {
		b := NewBox()
		b.Height = 40
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 150
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewBox(sv)
	host.Width, host.Height = 100, 150
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 150})

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	abs := core.AbsoluteBounds(sv)
	downY := abs.Min.Y + y0 + h0/2
	downX := abs.Min.X + 95
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.Dragging() {
		t.Fatal("no drag")
	}
	travel := sv.drag.trackMain - sv.drag.thumbMain
	maxS := sv.drag.maxScroll
	ptr0 := sv.drag.ptr0Abs
	scroll0 := sv.drag.scroll0

	for i := 0; i <= 40; i++ {
		y := downY + float64(i)*5
		sv.HandlePointer(&core.PointerEvent{Type: core.PointerMove, X: downX, Y: y, Button: core.ButtonLeft})
		host.MarkNeedsLayout()
		tree.Frame(&core.PaintContext{}, core.Size{Width: 100, Height: 150})
		want := scroll0 + (y-ptr0)/travel*maxS
		if want < 0 {
			want = 0
		}
		if want > maxS {
			want = maxS
		}
		if math.Abs(sv.ScrollY-want) > 0.05 {
			t.Fatalf("under layout thrash i=%d ScrollY=%.3f want %.3f", i, sv.ScrollY, want)
		}
		// content abs tracks -ScrollY
		// col is direct child; AbsoluteBounds includes paint offset
	}
}

// Parent tight height changes mid-drag (Tabs rail StretchChild). Content column
// AbsoluteBounds.Y must stay continuous with ScrollY (no whole-column 窜).
func TestContentNoJumpWhenParentResizesMidDrag(t *testing.T) {
	col := Column()
	for i := 0; i < 40; i++ {
		b := NewBox()
		b.Height = 40
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	// No fixed Height — take parent max (StretchChild-like).
	sv.Width = 100
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewDecorated(sv)
	host.Width = 100
	host.Height = 200
	host.StretchChild = true
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 200})

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	abs := core.AbsoluteBounds(sv)
	downY := abs.Min.Y + y0 + h0/2
	downX := abs.Min.X + 95
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.Dragging() {
		t.Fatal("no drag")
	}

	prevY := core.AbsoluteBounds(col).Min.Y
	prevS := sv.ScrollY
	for i := 0; i <= 30; i++ {
		// Alternate parent height (simulates flex reflow)
		if i%2 == 0 {
			host.Height = 200
		} else {
			host.Height = 210
		}
		sv.HandlePointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + float64(i)*6, Button: core.ButtonLeft,
		})
		tree.Frame(&core.PaintContext{}, core.Size{Width: 100, Height: host.Height})
		y := core.AbsoluteBounds(col).Min.Y
		s := sv.ScrollY
		// content Y change should match -ΔScroll within tolerance (viewport may grow)
		// Forbid large upward jumps while scrolling down.
		if s >= prevS-0.01 && y > prevY+2 {
			t.Fatalf("whole-column 窜 i=%d contentY %.1f→%.1f scroll %.1f→%.1f hostH=%.0f",
				i, prevY, y, prevS, s, host.Height)
		}
		prevY, prevS = y, s
	}
}
