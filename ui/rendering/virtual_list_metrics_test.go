package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// R7 gate data path: PipelineApp samples LastVirtualBind after present; the
// JSON gate is bind_count ≪ item_count.
func TestVirtualList_BindMetricsSnapshot(t *testing.T) {
	const (
		count  = 1000
		extent = 40.0
		vpH    = 400.0
		vpW    = 200.0
	)
	list := rendering.NewVirtualList(count, extent, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(vpW, extent, 0.3, 0.3, 0.35, 1)
	})
	list.CacheExtent = 80 // 2 rows each side

	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)
	vp.SetScrollOffset(0, 4000) // deep mid-list
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)

	bind, items := rendering.LastVirtualBind()
	if items != count {
		t.Fatalf("item_count=%d want %d", items, count)
	}
	if bind == 0 || bind >= count {
		t.Fatalf("bind_count=%d want 0<bind<%d", bind, count)
	}
	// ~10 visible + 4 cache rows + slack.
	if bind > 20 {
		t.Fatalf("bind_count=%d exceeds viewport+cache bound", bind)
	}
}

// R7b/C3 scroll_rerecord: only cells first-mounted by scrolling count; kept
// cells (cached Picture replays), no-op scrolls and resize rebinds do not.
func TestVirtualList_ScrollRerecord_FreshMountsOnly(t *testing.T) {
	const (
		count  = 200
		extent = 40.0
		vpH    = 400.0 // exactly 10 rows
		vpW    = 100.0
	)
	list := rendering.NewVirtualList(count, extent, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(vpW, extent, 0.4, 0.4, 0.5, 1)
	})
	list.CacheExtent = 0
	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)

	base := rendering.ScrollRerecordTotal()

	// Initial layout bind is not scrolling: no rerecord credit.
	if got := rendering.ScrollRerecordTotal(); got != base {
		t.Fatalf("initial layout counted as scroll rerecord: +%d", got-base)
	}

	// Scroll one row: exactly cell 10 enters the window → +1.
	vp.SetScrollOffset(0, extent)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)
	if got := rendering.ScrollRerecordTotal(); got != base+1 {
		t.Fatalf("after +1 row scroll: total=%d want %d", got, base+1)
	}

	// Same offset again: SetScrollOffset early-returns, nothing bumps.
	vp.SetScrollOffset(0, extent)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)
	if got := rendering.ScrollRerecordTotal(); got != base+1 {
		t.Fatalf("no-op scroll bumped rerecord to +%d", got-base)
	}

	// Viewport growth at constant offset mounts fresh cells but is a resize,
	// not a scroll: window [1..11) → [1..13) must not count.
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH + 80}, false)
	if got := rendering.ScrollRerecordTotal(); got != base+1 {
		t.Fatalf("resize rebind counted as scroll rerecord: +%d", got-base-1)
	}

	// Scrolling back remounts cell 0 as a fresh object: it records once again,
	// so it counts (honest remount accounting).
	vp.SetScrollOffset(0, 0)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH + 80}, false)
	if got := rendering.ScrollRerecordTotal(); got != base+2 {
		t.Fatalf("scroll-back remount not counted: total=%d want %d", got, base+2)
	}
}

// Fresh cells keep their dirty bit (record once on next paint); kept cells are
// cleared so their cached Picture replays instead of re-recording (R7b).
func TestVirtualList_FreshCellKeepsPaintDirty(t *testing.T) {
	const (
		count  = 200
		extent = 40.0
		vpH    = 400.0
		vpW    = 100.0
	)
	list := rendering.NewVirtualList(count, extent, func(i int) rendering.RenderObject {
		return rendering.NewRenderColorBox(vpW, extent, 0.4, 0.4, 0.5, 1)
	})
	list.CacheExtent = 0
	vp := rendering.NewRenderViewport(list)
	owner := rendering.NewPipelineOwner(vp)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)
	vp.SetScrollOffset(0, extent) // +1 row: cell 10 enters, cell 0 leaves
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)

	kids := list.Children()
	// Fixed-path window bound is inclusive of the row straddling endY:
	// [40,440) → rows 1..11 = 11 cells.
	if len(kids) != 11 {
		t.Fatalf("mounted=%d want 11", len(kids))
	}
	// Mount order appends the fresh cell last (cells 1..10 kept).
	for i, ch := range kids {
		want := i == len(kids)-1
		if ch.NeedsPaint() != want {
			t.Fatalf("child %d NeedsPaint=%v want %v", i, ch.NeedsPaint(), want)
		}
	}
}
