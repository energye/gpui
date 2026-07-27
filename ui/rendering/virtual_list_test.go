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
