//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

func TestS43_PathGeometryCache_HitMiss(t *testing.T) {
	c := NewPathGeometryCache()
	p := render.NewPath()
	p.MoveTo(10, 10)
	p.LineTo(100, 10)
	p.LineTo(50, 80)
	p.Close()

	v1, cq1, ok := c.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok || len(v1) == 0 {
		t.Fatalf("miss tessellate failed ok=%v len=%d", ok, len(v1))
	}
	hits, misses, entries := c.Stats()
	if misses != 1 || hits != 0 || entries != 1 {
		t.Fatalf("after miss: hits=%d misses=%d entries=%d", hits, misses, entries)
	}

	v2, cq2, ok := c.GetOrTessellate(p, render.FillRuleNonZero, false)
	if !ok || len(v2) != len(v1) {
		t.Fatalf("hit failed ok=%v len %d vs %d", ok, len(v2), len(v1))
	}
	if cq1 != cq2 {
		t.Fatalf("cover quad mismatch")
	}
	hits, misses, entries = c.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("after hit: hits=%d misses=%d", hits, misses)
	}

	// Different fill rule → new entry
	_, _, ok = c.GetOrTessellate(p, render.FillRuleEvenOdd, false)
	if !ok {
		t.Fatal("evenodd tessellate failed")
	}
	_, _, entries = c.Stats()
	if entries != 2 {
		t.Fatalf("want 2 entries, got %d", entries)
	}
}

func TestS43_StrokeGeometryCache_HitMiss(t *testing.T) {
	c := NewStrokeGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(40, 0)
	p.LineTo(40, 40)
	paint := &render.Paint{}
	paint.LineWidth = 3
	key := makeStrokeCacheKey(p, paint, false, 0)
	if _, ok := c.Get(key); ok {
		t.Fatal("expected miss")
	}
	// Store a simple expanded path
	exp := render.NewPath()
	exp.MoveTo(0, -1.5)
	exp.LineTo(40, -1.5)
	exp.LineTo(40, 1.5)
	exp.LineTo(0, 1.5)
	exp.Close()
	c.Put(key, exp)
	got, ok := c.Get(key)
	if !ok || got == nil || got.NumVerbs() == 0 {
		t.Fatalf("expected hit, ok=%v", ok)
	}
	hits, misses, entries := c.Stats()
	if hits != 1 || misses != 1 || entries != 1 {
		t.Fatalf("stats hits=%d misses=%d entries=%d", hits, misses, entries)
	}
}

func TestS43_ImageCache_Stats(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	cmd := &ImageDrawCommand{
		PixelData:    make([]byte, 4*4*4),
		GenerationID: 42,
		ImgWidth:     4,
		ImgHeight:    4,
		ImgStride:    16,
	}
	for i := range cmd.PixelData {
		cmd.PixelData[i] = 255
	}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatalf("hit: %v", err)
	}
	st := ic.Stats()
	if st.Hits != 1 || st.Misses != 1 || st.Uploads != 1 || st.Entries != 1 {
		t.Fatalf("stats hits=%d misses=%d uploads=%d entries=%d", st.Hits, st.Misses, st.Uploads, st.Entries)
	}
}
