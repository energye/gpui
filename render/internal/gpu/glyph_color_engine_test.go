//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func TestColorEngineLayoutColorRun(t *testing.T) {
	face := colorStubFace(t, 16)
	engine := NewColorGlyphEngine()
	glyphs := []text.ShapedGlyph{{GID: 65, X: 0, XAdvance: 10}}
	batch, err := engine.LayoutColorGlyphs(face, glyphs, 10, 50, render.Black, render.Identity(), 1)
	if err != nil {
		t.Fatalf("LayoutColorGlyphs: %v", err)
	}
	if !batch.IsColor {
		t.Error("batch.IsColor = false, want true")
	}
	if len(batch.Quads) != 1 {
		t.Fatalf("quads = %d, want 1", len(batch.Quads))
	}
	q := batch.Quads[0]
	if q.X0 != 10 || q.Y0 != 42 || q.X1 != 18 || q.Y1 != 50 {
		t.Fatalf("quad = %+v, want (10,42)-(18,50)", q)
	}
	if q.Page != 0 {
		t.Fatalf("quad page = %d, want 0", q.Page)
	}
}

func TestColorEngineSkipsOutline(t *testing.T) {
	face := colorStubFace(t, 16)
	engine := NewColorGlyphEngine()
	glyphs := []text.ShapedGlyph{{GID: 66, X: 0, XAdvance: 10}}
	batch, err := engine.LayoutColorGlyphs(face, glyphs, 10, 50, render.Black, render.Identity(), 1)
	if err != nil {
		t.Fatalf("LayoutColorGlyphs: %v", err)
	}
	if len(batch.Quads) != 0 {
		t.Fatalf("quads = %d, want 0 (outline skipped)", len(batch.Quads))
	}
}

func TestColorBatchDoesNotMergeWithMask(t *testing.T) {
	colorBatch := GlyphMaskBatch{IsColor: true, AtlasPageIndex: 0}
	maskBatch := GlyphMaskBatch{IsColor: false, AtlasPageIndex: 0}
	if colorBatch.CanMerge(maskBatch) {
		t.Error("color batch merged with mask batch; pipelines sample different textures")
	}
	if maskBatch.CanMerge(colorBatch) {
		t.Error("mask batch merged with color batch; pipelines sample different textures")
	}
	split := SplitGlyphMaskBatchByPage(GlyphMaskBatch{
		Quads:   []GlyphMaskQuad{{X0: 0, Page: 0}},
		IsColor: true,
	})
	if len(split) != 1 || !split[0].IsColor {
		t.Fatalf("split lost IsColor: %+v", split)
	}
}

func TestColorEngineRealEmoji(t *testing.T) {
	data, err := os.ReadFile("/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf")
	if err != nil {
		t.Skip("NotoColorEmoji not available")
	}
	src, err := text.NewFontSource(data, text.WithParser("own"))
	if err != nil {
		t.Fatalf("NewFontSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	face := src.Face(16)
	engine := NewColorGlyphEngine()
	gid := text.GlyphID(src.Parsed().GlyphIndex(0x1F600))
	glyphs := []text.ShapedGlyph{{GID: gid, X: 0, XAdvance: 20}}
	batch, err := engine.LayoutColorGlyphs(face, glyphs, 10, 50, render.Black, render.Identity(), 1)
	if err != nil {
		t.Fatalf("LayoutColorGlyphs: %v", err)
	}
	if len(batch.Quads) != 1 {
		t.Fatalf("quads = %d, want 1", len(batch.Quads))
	}
	q := batch.Quads[0]
	if q.X1 <= q.X0 || q.Y1 <= q.Y0 {
		t.Fatalf("quad = %+v, want positive area", q)
	}
}
