package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/text"
)

func p11CJKFace(t *testing.T, size float64) text.Face {
	t.Helper()
	candidates := []string{
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		src, err := text.NewFontSource(data)
		if err != nil {
			continue
		}
		return src.Face(size)
	}
	t.Skip("no CJK font installed")
	return nil
}

// TestP11_CJKDrawString_ShapeCacheWarm verifies X.02 / P1-1:
// repeated CJK DrawString hits shape/layout cache and stays on GPU (cpu_fb=0).
func TestP11_CJKDrawString_ShapeCacheWarm(t *testing.T) {
	requireNativeGPU(t)
	face := p11CJKFace(t, 18)

	text.ClearShapeResultCache()
	text.ResetShapeResultCacheStats()

	const w, h = 320, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.SetFont(face)
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	baseGPU := dc.RenderPathStats().GPUOps

	labels := []string{
		"场景：半透明涂层",
		"渲染性能 60fps",
		"网格 混合 变换",
		"场景：半透明涂层", // repeat for shape hit
	}
	for i, s := range labels {
		dc.SetRGB(0.1, 0.1, 0.15)
		dc.DrawString(s, 12, 28+float64(i)*22)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	st := text.ShapeResultCacheStats()
	t.Logf("path %s baseGPU=%d shape=%+v", stats.LogLine(), baseGPU, st)

	if stats.GPUOps <= baseGPU {
		t.Fatalf("CJK DrawString must use GPU: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("CJK text cpu_fb: %s", stats.LogLine())
	}
	// 3 unique + 1 repeat. R7.5/opt24 layout template may short-circuit
	// LayoutGlyphs on the repeat, so shape Hits can stay 0 while Misses
	// remains at the unique-label count (no extra miss for the repeat).
	if st.Misses < 1 {
		t.Fatalf("expected at least one shape miss for unique labels, %+v", st)
	}
	if st.Hits < 1 && st.Misses > 3 {
		t.Fatalf("expected shape/layout reuse for repeated CJK label (hits>=1 or misses<=unique), %+v", st)
	}

	// Pixel ink present (not blank white)
	ink := false
	for y := 10; y < h-10 && !ink; y += 4 {
		for x := 10; x < w-10; x += 4 {
			r, g, b, _ := sampleRGBA(dc, x, y)
			if int(r)+int(g)+int(b) < 700 {
				ink = true
				t.Logf("ink at %d,%d = %d,%d,%d", x, y, r, g, b)
				break
			}
		}
	}
	if !ink {
		t.Fatal("expected CJK ink on canvas")
	}

	// Second frame same labels at new Y → more hits, still GPU
	text.ResetShapeResultCacheStats()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i, s := range labels {
		dc.SetRGB(0.05, 0.05, 0.08)
		dc.DrawString(s, 12, 20+float64(i)*22)
	}
	_ = dc.FlushGPU()
	st2 := text.ShapeResultCacheStats()
	stats2 := dc.RenderPathStats()
	t.Logf("warm2 path %s shape=%+v", stats2.LogLine(), st2)
	// Warm frame: layout template and/or shape cache must prevent reshape.
	// Template hits skip LayoutGlyphs entirely → Hits may be 0 with Misses=0.
	if st2.Misses > 0 {
		t.Fatalf("warm frame should not reshape CJK labels (template/shape reuse), %+v", st2)
	}
	if stats2.CPUFallbackOps > 0 {
		t.Fatalf("warm cpu_fb: %s", stats2.LogLine())
	}
}
