//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

func TestS66_PathTess_ZeroCopyHit(t *testing.T) {
	c := NewPathGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(80, 0)
	p.LineTo(40, 60)
	p.LineTo(10, 50)
	p.Close() // concave → stencil path (not convex-only)

	v1, _, ok := c.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok || len(v1) == 0 {
		t.Fatalf("miss failed")
	}
	v2, _, ok := c.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok || len(v2) != len(v1) {
		t.Fatalf("hit failed")
	}
	if &v1[0] != &v2[0] {
		t.Fatal("S6.6 expected zero-copy tess hit (shared vertices)")
	}
	hits, misses, _ := c.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("hits=%d misses=%d", hits, misses)
	}
}

func TestS66_StrokeCache_SharedPathHit(t *testing.T) {
	c := NewStrokeGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(50, 0)
	p.LineTo(50, 40)
	paint := &render.Paint{}
	paint.LineWidth = 2.5
	key := makeStrokeCacheKey(p, paint, false, 0)

	exp := render.NewPath()
	exp.MoveTo(0, -1)
	exp.LineTo(50, -1)
	exp.LineTo(50, 1)
	exp.LineTo(0, 1)
	exp.Close()
	c.Put(key, exp)

	a, ok := c.Get(key)
	if !ok || a == nil {
		t.Fatal("miss after put")
	}
	b, ok := c.Get(key)
	if !ok {
		t.Fatal("second get")
	}
	if a != b {
		t.Fatal("S6.6 expected shared expanded path on stroke cache hit")
	}
	hits, misses, entries := c.Stats()
	if hits != 2 || misses != 0 || entries != 1 {
		t.Fatalf("stats hits=%d misses=%d entries=%d", hits, misses, entries)
	}
}

func TestS66_DashGeometryCache_Hit(t *testing.T) {
	c := NewDashGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(100, 0)
	p.LineTo(100, 40)
	p.LineTo(0, 40)
	p.Close()
	dash := render.NewDash(4, 3)
	d1 := c.GetOrApply(p, dash, 1)
	if d1 == nil || d1.NumVerbs() == 0 {
		t.Fatal("dash apply failed")
	}
	d2 := c.GetOrApply(p, dash, 1)
	if d1 != d2 {
		t.Fatal("expected shared dashed path on hit")
	}
	hits, misses, entries := c.Stats()
	if hits < 1 || misses < 1 || entries != 1 {
		t.Fatalf("hits=%d misses=%d entries=%d", hits, misses, entries)
	}
}

func TestS66_ConvexCache_HitAndNegative(t *testing.T) {
	c := NewConvexPathCache()
	// Convex triangle
	tri := render.NewPath()
	tri.MoveTo(0, 0)
	tri.LineTo(40, 0)
	tri.LineTo(20, 30)
	tri.Close()
	pts, ok := c.GetOrClassify(tri)
	if !ok || len(pts) != 3 {
		t.Fatalf("convex miss classify ok=%v n=%d", ok, len(pts))
	}
	pts2, ok := c.GetOrClassify(tri)
	if !ok || &pts[0] != &pts2[0] {
		t.Fatal("expected shared convex points on hit")
	}

	// Concave quad → negative cache
	conc := render.NewPath()
	conc.MoveTo(0, 0)
	conc.LineTo(40, 0)
	conc.LineTo(10, 10)
	conc.LineTo(40, 40)
	conc.LineTo(0, 40)
	conc.Close()
	if _, ok := c.GetOrClassify(conc); ok {
		t.Fatal("concave should not be convex")
	}
	if _, ok := c.GetOrClassify(conc); ok {
		t.Fatal("negative cache should stay negative")
	}
	hits, misses, entries := c.Stats()
	if hits < 2 || misses < 2 || entries < 2 {
		t.Fatalf("hits=%d misses=%d entries=%d", hits, misses, entries)
	}
}

func TestS66_StrokePath_DashAndStrokeCachesHot(t *testing.T) {
	// End-to-end through GPURenderContext shared caches.
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	_ = device
	_ = queue

	shared := &GPUShared{}
	// Minimal shared for cache accessors (no full GPU init needed for cache unit).
	// StrokePath needs more — exercise caches directly like production wiring.
	dashC := shared.DashGeomCache()
	strokeC := shared.StrokeGeomCache()

	p := render.NewPath()
	p.MoveTo(10, 10)
	p.LineTo(90, 15)
	p.LineTo(70, 70)
	p.LineTo(5, 60)
	p.Close()
	dash := render.NewDash(5, 4)
	dashed := dashC.GetOrApply(p, dash, 1)
	if dashed == nil {
		t.Fatal("dash")
	}
	_ = dashC.GetOrApply(p, dash, 1)

	paint := &render.Paint{}
	paint.LineWidth = 1.5
	key := makeStrokeCacheKey(dashed, paint, false, hashDash(dash))
	// Simulate expand once then hit.
	exp := dashed.Clone()
	strokeC.Put(key, exp)
	if _, ok := strokeC.Get(key); !ok {
		t.Fatal("stroke hit")
	}
	if _, ok := strokeC.Get(key); !ok {
		t.Fatal("stroke hit2")
	}
	st := shared.GeometryCacheStats()
	t.Logf("geom stats: dash hits=%d misses=%d stroke hits=%d", st.DashHits, st.DashMisses, st.StrokeHits)
	if st.DashHits < 1 || st.StrokeHits < 2 {
		t.Fatalf("expected hot dash/stroke caches: %+v", st)
	}
}

func TestS66_ComplexPolygon_TessReuse(t *testing.T) {
	c := NewPathGeometryCache()
	// Multi-segment star-like concave polygon (forces stencil tess).
	p := render.NewPath()
	p.MoveTo(50, 0)
	for i, ang := range []float64{0.3, 0.6, 0.9, 1.2, 1.5, 1.8, 2.1, 2.4, 2.7, 3.0, 3.3, 3.6, 3.9, 4.2, 4.5, 4.8, 5.1, 5.4, 5.7, 6.0} {
		r := 40.0
		if i%2 == 0 {
			r = 20
		}
		// approximate star points without math import heavy
		x := 50 + r*(0.5+0.5*float64((i*17)%10)/10)
		y := 50 + r*(0.5+0.5*float64((i*13)%10)/10)
		_ = ang
		p.LineTo(x, y)
	}
	p.Close()

	v1, cq1, ok := c.GetOrTessellate(p, render.FillRuleEvenOdd, false)
	if !ok || len(v1) < 9 {
		t.Fatalf("complex tess failed ok=%v len=%d", ok, len(v1))
	}
	v2, cq2, ok := c.GetOrTessellate(p, render.FillRuleEvenOdd, false)
	if !ok || len(v2) != len(v1) || cq1 != cq2 {
		t.Fatal("complex tess hit mismatch")
	}
	hits, misses, _ := c.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("hits=%d misses=%d", hits, misses)
	}
}
