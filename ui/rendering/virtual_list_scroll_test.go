package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// mixedExtents returns alternating short/tall heights for variable-list fixtures.
func mixedExtents(i int) float64 {
	if i%2 == 0 {
		return 20
	}
	return 60
}

// sumExtentsBefore returns Σ extent[0..k) — oracle for OffsetOfIndex(k).
func sumExtentsBefore(n int, extentAt func(int) float64, k int) float64 {
	if k < 0 {
		return 0
	}
	if k > n {
		k = n
	}
	var s float64
	for i := 0; i < k; i++ {
		s += extentAt(i)
	}
	return s
}

// TestVirtualList_OffsetOfIndex_VariablePrefix drives the shipped OffsetOfIndex
// path: for mixed heights, offset(k) must equal the sum of extents before k
// (not k×fallback).
func TestVirtualList_OffsetOfIndex_VariablePrefix(t *testing.T) {
	const count = 50
	const fallback = 40.0
	list := rendering.NewVariableVirtualList(count, fallback, mixedExtents, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(100, mixedExtents(i), 0.3, 0.3, 0.35, 1)
	})

	for _, k := range []int{0, 1, 2, 7, 25, 49} {
		got := list.OffsetOfIndex(k)
		want := sumExtentsBefore(count, mixedExtents, k)
		if got != want {
			t.Fatalf("OffsetOfIndex(%d)=%v want %v (fixed would be %v)", k, got, want, float64(k)*fallback)
		}
	}
	// Prove not fixed-extent: index 1 → 20, not 40; index 3 → 20+60+20=100, not 120.
	if list.OffsetOfIndex(1) == fallback {
		t.Fatal("OffsetOfIndex(1) equals fallback fixed step — variable prefix unused")
	}
	if list.OffsetOfIndex(3) == 3*fallback {
		t.Fatal("OffsetOfIndex(3) equals 3×fallback — variable prefix unused")
	}
	// End sentinel.
	if list.OffsetOfIndex(count) != list.ContentHeight() {
		t.Fatalf("OffsetOfIndex(count)=%v want ContentHeight %v", list.OffsetOfIndex(count), list.ContentHeight())
	}
	// Round-trip: IndexAtOffset(OffsetOfIndex(k)) == k for interior tops.
	for _, k := range []int{0, 3, 10, 40} {
		off := list.OffsetOfIndex(k)
		// Slightly inside the row (not on the next boundary).
		got := list.IndexAtOffset(off + 0.5)
		if got != k {
			t.Fatalf("IndexAtOffset(OffsetOfIndex(%d)+0.5)=%d want %d", k, got, k)
		}
	}
}

// TestVirtualList_ScrollToIndex_VariableJump drives Viewport.ScrollToIndex on a
// variable-height list: after jump, scrollY matches prefix offset, the target
// index is in BoundRange, and BindCount stays ≪ ItemCount.
func TestVirtualList_ScrollToIndex_VariableJump(t *testing.T) {
	const (
		count = 400
		vpH   = 300.0
		vpW   = 180.0
		cache = 60.0
	)
	list := rendering.NewVariableVirtualList(count, 40, mixedExtents, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(vpW, mixedExtents(i), 0.25, 0.3, 0.4, 1)
	})
	list.CacheExtent = cache

	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)

	jumps := []int{0, 50, 200, 350, count - 1}
	for _, idx := range jumps {
		ok := vp.ScrollToIndex(idx)
		if !ok {
			t.Fatalf("ScrollToIndex(%d) returned false", idx)
		}
		owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)

		wantY := list.ScrollOffsetForIndex(idx)
		// Viewport may clamp near the end.
		gotY := vp.ScrollOffset().Y
		maxY := vp.MaxScrollY()
		if maxY >= 0 && wantY > maxY {
			wantY = maxY
		}
		if gotY != wantY {
			t.Fatalf("after ScrollToIndex(%d) scrollY=%v want %v (max=%v)", idx, gotY, wantY, maxY)
		}

		first, last := list.BoundRange()
		if list.BindCount >= count {
			t.Fatalf("BindCount=%d mounted all rows after jump to %d", list.BindCount, idx)
		}
		if list.BindCount > 40 {
			t.Fatalf("BindCount=%d too high after jump to %d (virtualization weak)", list.BindCount, idx)
		}
		// Target should be in the mounted window when not clamped past content end.
		// When scroll is clamped to max, the last visible rows are near the end —
		// index may still be in range if it's the last item.
		if gotY == list.ScrollOffsetForIndex(idx) {
			if idx < first || idx >= last {
				t.Fatalf("index %d not in BoundRange [%d,%d) after unclamped jump", idx, first, last)
			}
		}
		// IndexAtOffset at scroll top should be the jumped index when unclamped.
		if gotY == list.ScrollOffsetForIndex(idx) {
			at := list.IndexAtOffset(gotY + 0.5)
			if at != idx {
				t.Fatalf("IndexAtOffset(scrollY+0.5)=%d want jumped %d", at, idx)
			}
		}
	}
	f, l := list.BoundRange()
	t.Logf("final bind=%d range=[%d,%d) contentH=%.0f", list.BindCount, f, l, list.ContentHeight())
}

// TestVirtualList_ScrollToIndex_FixedStillWorks keeps fixed-extent jump green.
func TestVirtualList_ScrollToIndex_FixedStillWorks(t *testing.T) {
	const count, extent, vpH = 200, 40.0, 200.0
	list := rendering.NewVirtualList(count, extent, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(100, extent, 0.2, 0.2, 0.25, 1)
	})
	list.CacheExtent = 40
	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 100, Height: vpH}, true)

	if !vp.ScrollToIndex(25) {
		t.Fatal("ScrollToIndex fixed failed")
	}
	owner.FlushLayout(rendering.Size{Width: 100, Height: vpH}, false)
	if vp.ScrollOffset().Y != 25*extent {
		t.Fatalf("scrollY=%v want %v", vp.ScrollOffset().Y, 25*extent)
	}
	if list.OffsetOfIndex(25) != 25*extent {
		t.Fatalf("OffsetOfIndex fixed=%v", list.OffsetOfIndex(25))
	}
	if list.BindCount >= count || list.BindCount > 20 {
		t.Fatalf("bind=%d after fixed jump", list.BindCount)
	}
	first, last := list.BoundRange()
	if 25 < first || 25 >= last {
		t.Fatalf("25 not in [%d,%d)", first, last)
	}
}
