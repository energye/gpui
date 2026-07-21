package text

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// G1.a product-path soak: Input-style reshape + virtual-list churn must keep
// shape-result cache and glyph-mask atlas within configured soft limits.
//
// This is an engine gate (not a full editor). It does not require GPU.
// Aligns with ENGINE_GAPS G1.a "atlas 上传有界；无 RSS 斜率爆炸".

func TestG1a_InputReshape_ShapeCacheBounded(t *testing.T) {
	src, err := NewFontSource(requireTestFont(t), WithParser("own"))
	if err != nil {
		t.Fatal(err)
	}
	face := src.Face(16)
	ClearShapeResultCache()
	ResetShapeResultCacheStats()

	// Simulate typing into a multi-line "input": each keystroke reshapes the
	// whole field (common naive editor path). Also inject high-churn HUD labels
	// so IsHighChurnLabel path is exercised without filling the soft-LRU.
	var field strings.Builder
	const keystrokes = 400
	base := "Input field sample — 中文混排 office ffi "
	for i := 0; i < keystrokes; i++ {
		field.WriteByte(byte('a' + i%26))
		if i%40 == 39 {
			field.WriteByte('\n')
		}
		text := base + field.String()
		glyphs := Shape(text, face)
		if len(glyphs) == 0 {
			t.Fatalf("keystroke %d: empty shape", i)
		}
		// Ephemeral FPS-like label (should bypass cache).
		_ = Shape(fmt.Sprintf("FPS %d.%d RSS %dMB frame=%d", i%90, i%10, 200+i%50, i), face)
	}

	st := ShapeResultCacheStats()
	t.Logf("shape cache: entries=%d soft=%d hits=%d misses=%d evictions=%d",
		st.Entries, st.SoftLimit, st.Hits, st.Misses, st.Evictions)
	if st.SoftLimit <= 0 {
		t.Fatal("soft limit unset")
	}
	// Eviction keeps entries near softLimit (implementation trims toward 3/4).
	if st.Entries > st.SoftLimit {
		t.Fatalf("shape cache entries %d > softLimit %d — unbounded growth", st.Entries, st.SoftLimit)
	}
	if st.Misses == 0 {
		t.Fatal("expected some shape cache misses during typing")
	}
}

func TestG1a_VirtualList_ShapeAndAtlasBounded(t *testing.T) {
	src, err := NewFontSource(requireTestFont(t), WithParser("own"))
	if err != nil {
		t.Fatal(err)
	}
	face := src.Face(14)
	ClearShapeResultCache()

	// Small atlas forces eviction under list churn.
	cfg := GlyphMaskAtlasConfig{
		Size:       256,
		Padding:    1,
		MaxAtlases: 2,
		MaxEntries: 256,
	}
	atlas, err := NewGlyphMaskAtlas(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rast := NewGlyphMaskRasterizer()
	parsed := src.Parsed()
	fontID := FontSourceID(src)

	const (
		rows    = 500
		window  = 40 // visible rows reshaped each "scroll" tick
		scrolls = 80
		ppem    = 14.0
	)

	// Prebuild row strings (unique content).
	rowsText := make([]string, rows)
	for i := range rows {
		rowsText[i] = fmt.Sprintf("row %04d — 表格单元格 content #%d office", i, i)
	}

	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	heap0 := ms.HeapAlloc

	for s := 0; s < scrolls; s++ {
		start := (s * 7) % (rows - window)
		for r := start; r < start+window; r++ {
			text := rowsText[r]
			glyphs := Shape(text, face)
			if len(glyphs) == 0 {
				t.Fatalf("scroll %d row %d: empty shape", s, r)
			}
			// Rasterize + pack first N outline glyphs into atlas (mask path).
			n := 0
			for _, g := range glyphs {
				if g.GID == 0 {
					continue
				}
				key := MakeGlyphMaskKey(fontID, g.GID, ppem, 0, 0)
				_, err := atlas.GetOrRasterize(key, func() ([]byte, int, int, float32, float32, error) {
					res, rerr := rast.Rasterize(parsed, g.GID, ppem, 0, 0)
					if rerr != nil {
						return nil, 0, 0, 0, 0, rerr
					}
					if res == nil {
						return nil, 0, 0, 0, 0, nil
					}
					return res.Mask, res.Width, res.Height, res.BearingX, res.BearingY, nil
				})
				if err != nil {
					t.Fatalf("atlas put: %v", err)
				}
				n++
				if n >= 8 {
					break
				}
			}
		}
		atlas.AdvanceFrame()

		if atlas.EntryCount() > cfg.MaxEntries {
			t.Fatalf("atlas entries %d > MaxEntries %d", atlas.EntryCount(), cfg.MaxEntries)
		}
		if atlas.PageCount() > cfg.MaxAtlases {
			t.Fatalf("atlas pages %d > MaxAtlases %d", atlas.PageCount(), cfg.MaxAtlases)
		}
	}

	st := ShapeResultCacheStats()
	hits, misses, entries, pages := atlas.Stats()
	mem := atlas.MemoryUsage()
	t.Logf("list soak: shape entries=%d/%d atlas entries=%d pages=%d hits=%d misses=%d mem=%dB",
		st.Entries, st.SoftLimit, entries, pages, hits, misses, mem)

	if st.Entries > st.SoftLimit {
		t.Fatalf("shape cache unbounded: %d > %d", st.Entries, st.SoftLimit)
	}
	if entries > cfg.MaxEntries {
		t.Fatalf("atlas unbounded: %d > %d", entries, cfg.MaxEntries)
	}
	// 2 pages × 256×256 R8 = 128KiB theoretical max for this config.
	maxMem := int64(cfg.MaxAtlases * cfg.Size * cfg.Size)
	if mem > maxMem {
		t.Fatalf("atlas memory %d > hard cap %d", mem, maxMem)
	}

	runtime.GC()
	runtime.ReadMemStats(&ms)
	heap1 := ms.HeapAlloc
	// Soft RSS check: allow growth but flag multi-hundred-MB leaks on this micro soak.
	const maxHeapGrowth = 64 << 20 // 64 MiB
	if heap1 > heap0+maxHeapGrowth {
		t.Fatalf("heap grew too much: before=%d after=%d delta=%d (limit %d)",
			heap0, heap1, heap1-heap0, maxHeapGrowth)
	}
}

// TestG1a_EditingReshape_NoEntryExplosion hammers repeated reshape of nearly
// identical strings (cursor blink / validation re-layout pattern).
func TestG1a_EditingReshape_NoEntryExplosion(t *testing.T) {
	src, err := NewFontSource(requireTestFont(t), WithParser("own"))
	if err != nil {
		t.Fatal(err)
	}
	face := src.Face(18)
	ClearShapeResultCache()
	ResetShapeResultCacheStats()

	const body = "用户输入 User Input 12345 — 编辑中 office"
	for i := 0; i < 1000; i++ {
		// Same text most of the time → should hit shape cache heavily.
		text := body
		if i%50 == 0 {
			text = body + fmt.Sprintf(" %d", i) // occasional unique
		}
		if g := Shape(text, face); len(g) == 0 {
			t.Fatal("empty shape")
		}
	}
	st := ShapeResultCacheStats()
	t.Logf("edit soak: hits=%d misses=%d entries=%d evictions=%d",
		st.Hits, st.Misses, st.Entries, st.Evictions)
	if st.Hits < 800 {
		t.Fatalf("expected strong cache reuse, hits=%d", st.Hits)
	}
	if st.Entries > st.SoftLimit {
		t.Fatalf("entries %d > softLimit %d", st.Entries, st.SoftLimit)
	}
}

// TestG1a_LongReshape_HeapGrowthBounded is a heavier CPU soak for G1.a:
// many reshape iterations + GC samples; heap must not grow unboundedly.
// Complements short Input/list tests; still no GPU/window requirement.
func TestG1a_LongReshape_HeapGrowthBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("long soak")
	}
	src, err := NewFontSource(requireTestFont(t), WithParser("own"))
	if err != nil {
		t.Fatal(err)
	}
	face := src.Face(15)
	ClearShapeResultCache()

	// Warm once so first-parse allocations are excluded from the slope.
	const warm = "warm 中文 ffi office 12345"
	_ = Shape(warm, face)
	runtime.GC()
	var bas runtime.MemStats
	runtime.ReadMemStats(&bas)
	baseHeap := bas.HeapAlloc

	var field strings.Builder
	field.WriteString(warm)
	const iters = 2500
	var maxHeap uint64
	for i := 0; i < iters; i++ {
		field.WriteByte(byte('a' + i%26))
		if i%80 == 79 {
			// Keep field size bounded (editor-like window of text).
			s := field.String()
			if len(s) > 400 {
				field.Reset()
				field.WriteString(s[len(s)-200:])
			}
			field.WriteByte('\n')
		}
		_ = Shape(field.String(), face)
		_ = Shape(fmt.Sprintf("HUD %d", i), face)
		if i%250 == 0 {
			runtime.GC()
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			if ms.HeapAlloc > maxHeap {
				maxHeap = ms.HeapAlloc
			}
		}
	}
	st := ShapeResultCacheStats()
	if st.Entries > st.SoftLimit {
		t.Fatalf("shape cache entries %d > softLimit %d", st.Entries, st.SoftLimit)
	}

	// Soft-LRU intentionally retains entries; clear before residual heap check.
	ClearShapeResultCache()
	runtime.GC()
	runtime.GC()
	var end runtime.MemStats
	runtime.ReadMemStats(&end)

	// Residual after cache clear: font/shaper warm data may remain.
	// 12 MiB headroom flags real leaks without fighting GC noise.
	const maxGrowth = 12 << 20
	growth := int64(end.HeapAlloc) - int64(baseHeap)
	if growth < 0 {
		growth = 0
	}
	t.Logf("heap base=%d residual=%d midMax=%d residualGrowth=%d peakShapeEntries=%d soft=%d",
		baseHeap, end.HeapAlloc, maxHeap, growth, st.Entries, st.SoftLimit)
	if growth > maxGrowth {
		t.Fatalf("residual heap growth %d bytes > %d after cache clear — possible leak", growth, maxGrowth)
	}
}
