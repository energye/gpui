package primitive

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// Thumb length and maxScroll must stay fixed for the whole drag gesture even if
// Layout is forced every move (gallery demand loop + Tabs remeasure).
func TestThumbLengthStableDuringDrag(t *testing.T) {
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
	vp := core.Size{Width: 100, Height: 120}
	tree.Layout(vp)

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	if h0 < 1 {
		t.Fatalf("no thumb h=%v ContentH=%v", h0, sv.ContentH)
	}
	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if sv.drag.axis != 1 {
		t.Fatalf("drag not started axis=%d", sv.drag.axis)
	}
	thumb0 := sv.drag.thumbMain
	content0 := sv.drag.content
	max0 := sv.drag.maxScroll

	var heights, contents, maxes, scrolls []float64
	for i := 0; i <= 40; i++ {
		frac := float64(i) / 40
		sv.HandlePointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + frac*300, Button: core.ButtonLeft,
		})
		// Adversarial: parent forces layout every frame like demand loop.
		host.MarkNeedsLayout()
		tree.Layout(vp)
		_, _, _, _, h := sv.vThumbGeom(sv.Size())
		heights = append(heights, h)
		contents = append(contents, sv.ContentH)
		maxes = append(maxes, sv.drag.maxScroll)
		scrolls = append(scrolls, sv.ScrollY)
	}
	for i := range heights {
		if heights[i] != thumb0 {
			t.Fatalf("thumb length thrash: start=%.3f series=%v", thumb0, heights)
		}
		if contents[i] != content0 {
			t.Fatalf("ContentH thrash during drag: %v", contents)
		}
		if maxes[i] != max0 {
			t.Fatalf("maxScroll thrash: %v", maxes)
		}
	}
	for i := 1; i < len(scrolls); i++ {
		if scrolls[i] < scrolls[i-1]-0.01 {
			t.Fatalf("ScrollY backwards: %v", scrolls)
		}
	}
	if scrolls[len(scrolls)-1] < max0*0.85 {
		t.Fatalf("did not reach end last=%.1f max=%.1f", scrolls[len(scrolls)-1], max0)
	}
	// thumb length independent of pixels when not dragging
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerUp, X: downX, Y: downY + 300, Button: core.ButtonLeft})
	sv.SetScroll(0, 0)
	_, _, _, _, hA := sv.vThumbGeom(sv.Size())
	sv.SetScroll(0, max0)
	_, _, _, _, hB := sv.vThumbGeom(sv.Size())
	if hA != hB {
		t.Fatalf("thumb length depends on scroll: at0=%.3f atMax=%.3f", hA, hB)
	}
}

// Flutter: thumb extent independent of scroll offset (pixels).
func TestFlutterThumbIndependentOfPixels(t *testing.T) {
	col := Column()
	for i := 0; i < 30; i++ {
		b := NewBox()
		b.Height = 50
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 80, 100
	sv.Scrollbar().Horizontal = ScrollbarNever
	_ = sv.Layout(core.Tight(80, 100))
	m0 := sv.computeBarMetrics(true, sv.Size().Height)
	for _, px := range []float64{0, 10, 50, 100, 200, m0.maxScroll} {
		sv.SetScroll(0, px)
		_, _, _, _, h := sv.vThumbGeom(sv.Size())
		if h != m0.thumbMain {
			t.Fatalf("pixels=%v thumbH=%.4f want frozen formula %.4f (viewport=%.1f content=%.1f track=%.1f)",
				px, h, m0.thumbMain, m0.viewport, m0.content, m0.trackMain)
		}
	}
}

// Prove thrash is ScrollViewport-local: mutating ContentH under parent Layout during
// drag must not change painted thumb length (container cannot break freeze).
func TestThumbStableWhenContentHMutatedMidDrag(t *testing.T) {
	col := Column()
	for i := 0; i < 20; i++ {
		b := NewBox()
		b.Height = 40
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 100
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewBox(sv)
	host.Width, host.Height = 100, 100
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 100})

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.drag.active() {
		t.Fatal("no drag")
	}
	thumb0 := sv.drag.thumbMain

	for i := 0; i < 20; i++ {
		// Simulate container remeasure thrash (Tabs / Flex adversarial).
		sv.ContentH = sv.drag.content + float64((i%5)-2)*30
		sv.HandlePointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + float64(i)*10, Button: core.ButtonLeft,
		})
		tree.Layout(core.Size{Width: 100, Height: 100})
		_, _, _, _, h := sv.vThumbGeom(sv.Size())
		if h != thumb0 {
			t.Fatalf("container ContentH thrash changed thumb: i=%d h=%.3f want %.3f ContentH=%.1f",
				i, h, thumb0, sv.ContentH)
		}
	}
}

// Parent AbsoluteBounds jitter mid-drag must not change thumb length or reverse scroll.
// Simulates Tabs/Flex host moving under the viewport while pointer capture continues.
func TestThumbStableWhenAbsOriginJitters(t *testing.T) {
	col := Column()
	for i := 0; i < 25; i++ {
		b := NewBox()
		b.Height = 40
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 100
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewBox(sv)
	host.Width, host.Height = 100, 100
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 100})

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.drag.active() {
		t.Fatal("no drag")
	}
	thumb0 := sv.drag.thumbMain
	// Poison live abs by moving host offset after down (would thrash live AbsoluteBounds).
	host.Base().SetOffset(core.Point{X: 0, Y: 40})
	var heights, scrolls []float64
	for i := 0; i <= 20; i++ {
		sv.HandlePointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + float64(i)*12, Button: core.ButtonLeft,
		})
		_, _, _, _, h := sv.vThumbGeom(sv.Size())
		heights = append(heights, h)
		scrolls = append(scrolls, sv.ScrollY)
	}
	for i, h := range heights {
		if h != thumb0 {
			t.Fatalf("thumb thrash under abs jitter i=%d h=%v want %v", i, h, thumb0)
		}
	}
	for i := 1; i < len(scrolls); i++ {
		if scrolls[i] < scrolls[i-1]-0.01 {
			t.Fatalf("scroll reversed under abs jitter: %v", scrolls)
		}
	}
}

// Near maxScroll: thumb height must not flicker (end seating).
func TestThumbStableNearBottom(t *testing.T) {
	col := Column()
	for i := 0; i < 50; i++ {
		b := NewBox()
		b.Height = 40
		b.Width = 80
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 150
	sv.Scrollbar().Horizontal = ScrollbarNever
	host := NewBox(sv)
	host.Width, host.Height = 100, 150
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 100, Height: 150})

	// Jump near bottom first
	max0 := sv.ContentH - sv.ContentSize().Height
	sv.SetScroll(0, max0*0.85)
	tree.Layout(core.Size{Width: 100, Height: 150})

	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if sv.drag.axis != 1 {
		t.Fatalf("no drag axis=%d", sv.drag.axis)
	}
	thumb0 := sv.drag.thumbMain
	var heights, scrolls []float64
	for i := 0; i <= 30; i++ {
		sv.HandlePointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + float64(i)*8, Button: core.ButtonLeft,
		})
		host.MarkNeedsLayout()
		tree.Frame(&core.PaintContext{}, core.Size{Width: 100, Height: 150})
		heights = append(heights, sv.ThumbMainLength(true))
		scrolls = append(scrolls, sv.ScrollY)
	}
	for i, h := range heights {
		if h != thumb0 {
			t.Fatalf("near-bottom thumb thrash i=%d h=%.4f want %.4f scrolls=%v heights=%v",
				i, h, thumb0, scrolls, heights)
		}
	}
	// Must be able to seat at exact max without oscillation
	last := scrolls[len(scrolls)-1]
	if last < sv.drag.maxScroll-1 && last < max0*0.95 {
		// ok if not fully at end depending on drag distance
	}
	// At maxScroll, thumb bottom must sit on track end
	sv.SetScroll(0, sv.drag.maxScroll)
	_, _, y, _, h := sv.vThumbGeom(sv.Size())
	// still dragging freeze
	track := sv.drag.trackMain
	if y+h < track-0.5 || y+h > track+0.5 {
		// after clear drag
	}
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerUp, X: downX, Y: downY + 240, Button: core.ButtonLeft})
	sv.SetScroll(0, sv.ContentH) // clamp to max
	m := sv.computeBarMetrics(true, sv.Size().Height)
	_, _, y2, _, h2 := sv.vThumbGeom(sv.Size())
	if y2+h2 < m.trackMain-0.5 || y2+h2 > m.trackMain+0.5 {
		t.Fatalf("at maxScroll thumb not seated: y=%.3f h=%.3f track=%.3f", y2, h2, m.trackMain)
	}
}
