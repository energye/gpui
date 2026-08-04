package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

func TestViewport_ScrollDoesNotLayout(t *testing.T) {
	child := rendering.NewRenderColorBox(100, 800, 0.3, 0.3, 0.4, 1)
	vp := rendering.NewRenderViewport(child)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 200}, true)
	n := owner.LayoutCount

	for i := 0; i < 20; i++ {
		vp.SetScrollOffset(0, float64(i*10))
		if owner.FlushLayout(rendering.Size{Width: 100, Height: 200}, false) {
			t.Fatal("scroll must not force layout when content size unchanged")
		}
	}
	if owner.LayoutCount != n {
		t.Fatalf("layout count %d → %d", n, owner.LayoutCount)
	}
	if vp.ScrollOffset().Y != 190 { // last i=19 → 190, clamped by max
		// max = 800-200=600, so 190 ok
		if vp.ScrollOffset().Y < 100 {
			t.Fatalf("scrollY=%v", vp.ScrollOffset().Y)
		}
	}
}

func TestViewport_HitTestRespectsOffset(t *testing.T) {
	// Content: tall box; hit at content y=250 should require scroll.
	inner := rendering.NewRenderColorBox(50, 50, 1, 0, 0, 1)
	// Place inner at y=250 via parent box
	content := rendering.NewRenderBox(inner)
	content.FixedWidth, content.FixedHeight = 50, 400
	// After layout, set inner offset
	vp := rendering.NewRenderViewport(content)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 50, Height: 100}, true)
	inner.SetOffset(rendering.Point{X: 0, Y: 250})

	// Without scroll, y=10 hits viewport/content top, not inner.
	if hit := vp.HitTest(rendering.Point{X: 10, Y: 10}); hit == inner {
		t.Fatal("should not hit inner without scroll")
	}
	// Scroll so content y=250 appears at viewport y=10 → scrollY=240
	vp.SetScrollOffset(0, 240)
	if hit := vp.HitTest(rendering.Point{X: 10, Y: 10}); hit != inner {
		t.Fatalf("hit=%T want inner", hit)
	}
}

func TestS6_DragScroll_NoLayoutStorm(t *testing.T) {
	child := rendering.NewRenderColorBox(100, 2000, 0.2, 0.3, 0.4, 1)
	vp := rendering.NewRenderViewport(child)
	sc := rendering.NewScrollable(vp)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 200}, true)
	layouts := owner.LayoutCount

	sc.HandlePointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, Y: 100})
	for i := 0; i < 50; i++ {
		sc.HandlePointer(platform.Event{
			Type: platform.EventPointer, Pointer: platform.PointerMove, Y: 100 - float64(i),
		})
		_ = owner.FlushLayout(rendering.Size{Width: 100, Height: 200}, false)
	}
	sc.HandlePointer(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, Y: 50})
	if owner.LayoutCount != layouts {
		t.Fatalf("layout %d → %d during drag", layouts, owner.LayoutCount)
	}
	if vp.ScrollOffset().Y <= 0 {
		t.Fatalf("expected scrollY>0 got %v", vp.ScrollOffset().Y)
	}
}

func TestViewport_PaintVisits(t *testing.T) {
	child := rendering.NewRenderColorBox(80, 400, 0.5, 0.5, 0.5, 1)
	vp := rendering.NewRenderViewport(child)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 80, Height: 100}, true)
	var visits int64
	pc := &rendering.PaintContext{PaintVisits: &visits}
	owner.FlushPaint(pc, true)
	if visits < 1 {
		t.Fatal("expected paint visits")
	}
}

// TestViewport_FixedSizeBoundsScrolledContent: a FixedHeight viewport must lay
// out to the fixed height (not the constraint max), clip scrolled content inside
// it, and derive maxScrollY from content minus the fixed height (R21 HUD band
// overlap bug: unset height grew the viewport over the bottom HUD band).
func TestViewport_FixedSizeBoundsScrolledContent(t *testing.T) {
	child := rendering.NewRenderColorBox(100, 2640, 0.3, 0.4, 0.5, 1)
	vp := rendering.NewRenderViewport(child)
	vp.FixedWidth, vp.FixedHeight = 100, 640
	owner := rendering.NewPipelineOwner(vp)
	// Parent constraint would normally allow 800 (full window) — fixed wins.
	owner.FlushLayout(rendering.Size{Width: 100, Height: 800}, true)
	sz := vp.Size()
	if sz.Height != 640 {
		t.Fatalf("viewport height=%v want 640 (constraint max 800 must not apply)", sz.Height)
	}
	if vp.MaxScrollY() != 2640-640 {
		t.Fatalf("maxScrollY=%v want %v", vp.MaxScrollY(), 2640.0-640)
	}
	// Scroll beyond derived max must clamp (content never over-scrolls past
	// the fixed viewport into the band below it).
	vp.SetScrollOffset(0, 5000)
	if y := vp.ScrollOffset().Y; y != 2640-640 {
		t.Fatalf("clamped scrollY=%v want %v", y, 2640.0-640)
	}
	// Default (no fixed) keeps old behavior: constraint-derived size.
	vp2 := rendering.NewRenderViewport(child)
	owner2 := rendering.NewPipelineOwner(vp2)
	owner2.FlushLayout(rendering.Size{Width: 100, Height: 800}, true)
	if vp2.Size().Height != 800 {
		t.Fatalf("default viewport height=%v want 800 (constraint max)", vp2.Size().Height)
	}
}
