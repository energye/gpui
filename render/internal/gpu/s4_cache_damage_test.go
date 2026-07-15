//go:build !nogpu

package gpu

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// TestS44_SharedPathCacheAcrossContexts verifies S4.3 caches live on GPUShared
// and accumulate hits when the same path is tessellated repeatedly (retained).
func TestS44_SharedPathCacheAcrossContexts(t *testing.T) {
	shared := NewGPUShared()
	defer shared.Close()

	p := render.NewPath()
	// Non-trivial path (star-ish) so tessellation is non-empty.
	p.MoveTo(20, 5)
	p.LineTo(25, 18)
	p.LineTo(40, 18)
	p.LineTo(28, 28)
	p.LineTo(32, 42)
	p.LineTo(20, 34)
	p.LineTo(8, 42)
	p.LineTo(12, 28)
	p.LineTo(0, 18)
	p.LineTo(15, 18)
	p.Close()

	c1 := shared.PathGeomCache()
	c2 := shared.PathGeomCache()
	if c1 != c2 {
		t.Fatal("PathGeomCache must be singleton per GPUShared")
	}

	_, _, ok := c1.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok {
		t.Fatal("first tessellate failed")
	}
	// Second context logical "frame" reuses same shared cache.
	_, _, ok = c2.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok {
		t.Fatal("second tessellate failed")
	}
	_, _, ok = c2.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok {
		t.Fatal("third tessellate failed")
	}

	hits, misses, entries := c1.Stats()
	if misses != 1 || hits != 2 || entries != 1 {
		t.Fatalf("want hits=2 misses=1 entries=1, got hits=%d misses=%d entries=%d", hits, misses, entries)
	}
}

// TestS44_StrokeCache_SharedRetainedHit covers stroke expansion reuse (S4.3/S4.4 retained).
func TestS44_StrokeCache_SharedRetainedHit(t *testing.T) {
	shared := NewGPUShared()
	defer shared.Close()

	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(30, 0)
	p.LineTo(30, 20)
	paint := render.NewPaint()
	paint.LineWidth = 2.5
	key := makeStrokeCacheKey(p, paint, false, 0)

	sc := shared.StrokeGeomCache()
	if _, ok := sc.Get(key); ok {
		t.Fatal("expected cold miss")
	}
	exp := render.NewPath()
	exp.MoveTo(0, -1)
	exp.LineTo(30, -1)
	exp.LineTo(30, 1)
	exp.LineTo(0, 1)
	exp.Close()
	sc.Put(key, exp)

	// Retained "frame 2"
	got, ok := shared.StrokeGeomCache().Get(key)
	if !ok || got == nil || got.NumVerbs() == 0 {
		t.Fatalf("retained hit failed ok=%v", ok)
	}
	hits, misses, entries := sc.Stats()
	if hits != 1 || misses != 1 || entries != 1 {
		t.Fatalf("stats hits=%d misses=%d entries=%d", hits, misses, entries)
	}
}

// TestS44_DamageUnion_MultiRegion documents multi-region damage union used by
// applyGroupScissorWithDamage on MSAA + blit paths.
func TestS44_DamageUnion_MultiRegion(t *testing.T) {
	rects := []image.Rectangle{
		image.Rect(10, 10, 40, 40),
		image.Rect(80, 20, 120, 60),
	}
	u := damageRectsUnion(rects)
	if u != image.Rect(10, 10, 120, 60) {
		t.Fatalf("union=%v", u)
	}

	// Group fully outside first panel but inside union still valid via union path.
	group := [4]uint32{90, 30, 20, 20}
	x, y, w, h, valid := computeDamageScissor(&group, 200, 100, u)
	if !valid || w == 0 || h == 0 {
		t.Fatalf("expected valid scissor inside union, got %v %d,%d %dx%d", valid, x, y, w, h)
	}

	// Group outside entire union → skip draw.
	groupOut := [4]uint32{0, 70, 10, 10}
	if _, _, _, _, valid := computeDamageScissor(&groupOut, 200, 100, u); valid {
		t.Fatal("expected skip when group outside damage union")
	}
}
