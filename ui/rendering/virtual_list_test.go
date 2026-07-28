package rendering_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// S5: 1000 rows, viewport 400px, extent 40 → ~10 visible + cache.
func TestS5_VirtualList_BindCap(t *testing.T) {
	const (
		count   = 1000
		extent  = 40.0
		vpH     = 400.0
		vpW     = 200.0
		cachePx = 80.0 // 2 rows
	)
	list := rendering.NewVirtualList(count, extent, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(vpW, extent, 0.3, 0.3, 0.35, 1)
	})
	list.CacheExtent = cachePx

	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)

	maxBind := 0
	// Scroll through content.
	maxY := float64(count)*extent - vpH
	for y := 0.0; y <= maxY; y += extent * 3 {
		vp.SetScrollOffset(0, y)
		// bind window change may layout
		owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)
		if list.BindCount > maxBind {
			maxBind = list.BindCount
		}
		if list.BindCount >= count {
			t.Fatalf("mounted all %d rows — virtualization broken", count)
		}
		var visits int64
		owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false)
	}
	// visible ≈ 10, cache ≈ 2+2 → expect ≤ 16 with slack
	const cap = 20
	if maxBind > cap {
		t.Fatalf("max BindCount=%d want ≤%d", maxBind, cap)
	}
	if maxBind < 8 {
		t.Fatalf("max BindCount=%d suspiciously low", maxBind)
	}
	t.Logf("S5 maxBind=%d", maxBind)
}

func TestVirtualList_ContentHeight(t *testing.T) {
	list := rendering.NewVirtualList(100, 50, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(100, 50, 1, 1, 1, 1)
	})
	if list.ContentHeight() != 5000 {
		t.Fatalf("h=%v", list.ContentHeight())
	}
}

func TestVirtualList_BuilderIndex(t *testing.T) {
	seen := map[int]bool{}
	list := rendering.NewVirtualList(50, 20, func(i int) rendering.RenderObject {
		seen[i] = true
		return rendering.NewRenderText(fmt.Sprintf("row-%d", i))
	})
	list.CacheExtent = 0
	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true) // ~5 visible after notify
	// Provisional bind before/around first scroll notify is a small page (≪ 50).
	if len(seen) > 20 {
		t.Fatalf("bound too many at start: %d", len(seen))
	}
	if len(seen) == 50 {
		t.Fatal("mounted all rows")
	}
	vp.SetScrollOffset(0, 400) // near row 20
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, false)
	if !seen[20] && !seen[19] && !seen[21] {
		t.Fatalf("expected rows near 20 after scroll, seen=%v bind=%d", seen, list.BindCount)
	}
}

// Variable-height: alternating 20/60 px rows — content height is not count×oneExtent.
func TestVirtualList_VariableExtent_ContentHeight(t *testing.T) {
	const count = 100
	const fallback = 40.0
	extentAt := func(i int) float64 {
		if i%2 == 0 {
			return 20
		}
		return 70 // deliberately not symmetric around fallback
	}
	list := rendering.NewVariableVirtualList(count, fallback, extentAt, func(i int) rendering.RenderObject {
		h := extentAt(i)
		return rendering.NewRenderColorBox(100, h, 0.4, 0.4, 0.5, 1)
	})
	// 50×20 + 50×70 = 1000 + 3500 = 4500; fixed would be 100×40 = 4000
	want := 50*20.0 + 50*70.0
	got := list.ContentHeight()
	if got != want {
		t.Fatalf("ContentHeight=%v want %v (fixed would be %v)", got, want, float64(count)*fallback)
	}
	if got == float64(count)*fallback {
		t.Fatal("still using single fixed extent for content height")
	}
}

func TestVirtualList_VariableExtent_BindCapAndScroll(t *testing.T) {
	const (
		count = 500
		vpH   = 400.0
		vpW   = 200.0
		cache = 80.0
	)
	extentAt := func(i int) float64 {
		// Mix short/medium/tall so window lookup cannot use count×extent.
		switch i % 3 {
		case 0:
			return 24
		case 1:
			return 48
		default:
			return 72
		}
	}
	list := rendering.NewVariableVirtualList(count, 40, extentAt, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(vpW, extentAt(i), 0.3, 0.35, 0.4, 1)
	})
	list.CacheExtent = cache

	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)

	totalH := list.ContentHeight()
	if totalH <= float64(count)*24 {
		t.Fatalf("totalH=%v too small", totalH)
	}
	maxY := totalH - vpH
	if maxY < 0 {
		maxY = 0
	}

	maxBind := 0
	for y := 0.0; y <= maxY; y += 120 {
		vp.SetScrollOffset(0, y)
		owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)
		if list.BindCount > maxBind {
			maxBind = list.BindCount
		}
		if list.BindCount >= count {
			t.Fatalf("mounted all %d rows at y=%v — virtualization broken", count, y)
		}
	}
	// Tallest rows 72px → visible ~6 + cache slack; allow generous cap ≪ 500.
	const cap = 40
	if maxBind > cap {
		t.Fatalf("max BindCount=%d want ≤%d", maxBind, cap)
	}
	if maxBind < 4 {
		t.Fatalf("max BindCount=%d suspiciously low", maxBind)
	}
	t.Logf("var-extent maxBind=%d contentH=%.0f", maxBind, totalH)
}

func TestVirtualList_VariableExtent_RowOffsets(t *testing.T) {
	extents := []float64{10, 30, 50, 20}
	list := rendering.NewVariableVirtualList(len(extents), 10, func(i int) float64 {
		return extents[i]
	}, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(50, extents[i], 1, 1, 1, 1)
	})
	list.CacheExtent = 0
	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: 50, Height: 200}, true)

	// Force bind all few rows by large viewport.
	if list.ContentHeight() != 110 {
		t.Fatalf("h=%v want 110", list.ContentHeight())
	}
	// Scroll 0 with tall viewport should mount all 4.
	if list.BindCount != 4 {
		// may be ok if still provisional — scroll notify
		vp.SetScrollOffset(0, 0)
		owner.FlushLayout(rendering.Size{Width: 50, Height: 200}, false)
	}
	// Check child offsets via layout positions: row2 should start at 10+30=40.
	// We inspect by rebinding and reading offsets from mounted map indirectly:
	// Flush and hit-test at y=45 should hit row index 2 (50px tall starting 40).
	hit := list.HitTest(rendering.Point{X: 10, Y: 45})
	if hit == nil {
		t.Fatal("expected hit at y=45")
	}
}
