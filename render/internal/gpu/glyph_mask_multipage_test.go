//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func TestSplitGlyphMaskBatchByPage_SingleAndMulti(t *testing.T) {
	base := GlyphMaskBatch{
		Transform:  render.Identity(),
		Color:      [4]float32{1, 1, 1, 1},
		AtlasWidth: 1024, AtlasHeight: 1024,
		AtlasPageIndex: 0,
		Quads: []GlyphMaskQuad{
			{X0: 0, X1: 1, Page: 0},
			{X0: 1, X1: 2, Page: 0},
			{X0: 2, X1: 3, Page: 1},
			{X0: 3, X1: 4, Page: 1},
			{X0: 4, X1: 5, Page: 0},
		},
	}
	parts := SplitGlyphMaskBatchByPage(base)
	if len(parts) != 3 {
		t.Fatalf("parts=%d want 3 (runs 0,1,0)", len(parts))
	}
	if parts[0].AtlasPageIndex != 0 || len(parts[0].Quads) != 2 {
		t.Fatalf("part0=%+v", parts[0])
	}
	if parts[1].AtlasPageIndex != 1 || len(parts[1].Quads) != 2 {
		t.Fatalf("part1=%+v", parts[1])
	}
	if parts[2].AtlasPageIndex != 0 || len(parts[2].Quads) != 1 {
		t.Fatalf("part2=%+v", parts[2])
	}

	// All page 0 with Page field set → single batch.
	one := base
	one.Quads = []GlyphMaskQuad{{Page: 0}, {Page: 0}}
	one.AtlasPageIndex = 0
	parts = SplitGlyphMaskBatchByPage(one)
	if len(parts) != 1 || parts[0].AtlasPageIndex != 0 || len(parts[0].Quads) != 2 {
		t.Fatalf("single page0 parts=%+v", parts)
	}
}

// TestLayoutText_SetsAtlasPageFromRegion forces a tiny atlas so glyphs spill
// onto page 1, then asserts quads carry Page and Split yields page-correct batches.
func TestLayoutText_SetsAtlasPageFromRegion(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		// LayoutText is CPU-side atlas; no GPU required. Keep gate loose.
	}
	// Tiny atlas: first few glyphs fill page 0, later spill to page 1.
	atlas, err := text.NewGlyphMaskAtlas(text.GlyphMaskAtlasConfig{
		Size: 64, Padding: 1, MaxAtlases: 4, MaxEntries: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	eng := &GlyphMaskEngine{
		atlas:       atlas,
		rasterizer:  text.NewGlyphMaskRasterizer(),
		layoutCache: make(map[glyphLayoutTemplateKey]*glyphLayoutTemplateEntry),
	}

	face := glyphMaskTestFont(t, 28)
	// Dense CJK + Latin to force multi-page packing.
	s := "中文渲染测试混合排版ABCDEFGabcdefg0123456789中文渲染测试混合排版"
	batch, err := eng.LayoutText(face, s, 4, 30, render.RGBA{A: 1}, render.Identity(), 1)
	if err != nil {
		t.Fatalf("LayoutText: %v", err)
	}
	if len(batch.Quads) < 8 {
		t.Fatalf("expected many quads, got %d", len(batch.Quads))
	}
	maxPage := 0
	for _, q := range batch.Quads {
		if q.Page > maxPage {
			maxPage = q.Page
		}
		if q.Page < 0 {
			t.Fatalf("negative page %d", q.Page)
		}
	}
	if maxPage < 1 {
		t.Fatalf("expected multi-page packing on 64px atlas, maxPage=%d quads=%d", maxPage, len(batch.Quads))
	}
	parts := SplitGlyphMaskBatchByPage(batch)
	if len(parts) < 2 {
		t.Fatalf("Split parts=%d want ≥2 for multi-page string", len(parts))
	}
	for i, p := range parts {
		if len(p.Quads) == 0 {
			t.Fatalf("empty part %d", i)
		}
		for _, q := range p.Quads {
			if q.Page != p.AtlasPageIndex {
				t.Fatalf("part %d AtlasPageIndex=%d but quad.Page=%d", i, p.AtlasPageIndex, q.Page)
			}
		}
	}
}
