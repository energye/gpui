//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func TestGPUTextEngine_CJKAtlasCreated(t *testing.T) {
	engine := NewGPUTextEngine()
	if engine.atlasManager == nil {
		t.Fatal("Latin atlas manager is nil")
	}
	if engine.cjkAtlasManager == nil {
		t.Fatal("CJK atlas manager is nil")
	}
	if engine.msdfSize != 64 {
		t.Errorf("Latin MSDF size = %d, want 64", engine.msdfSize)
	}
	if engine.msdfSizeCJK != 128 {
		t.Errorf("CJK MSDF size = %d, want 128", engine.msdfSizeCJK)
	}
}

func TestGPUTextEngine_CJKAtlasConfig(t *testing.T) {
	engine := NewGPUTextEngine()

	latinCfg := engine.atlasManager.Config()
	cjkCfg := engine.cjkAtlasManager.Config()

	if latinCfg.GlyphSize != 64 {
		t.Errorf("Latin glyph size = %d, want 64", latinCfg.GlyphSize)
	}
	if cjkCfg.GlyphSize != 128 {
		t.Errorf("CJK glyph size = %d, want 128", cjkCfg.GlyphSize)
	}
	if latinCfg.Size != 1024 {
		t.Errorf("Latin atlas size = %d, want 1024", latinCfg.Size)
	}
	if cjkCfg.Size != 2048 {
		t.Errorf("CJK atlas size = %d, want 2048", cjkCfg.Size)
	}
}

func TestGPUTextEngine_CJKAtlasOffset(t *testing.T) {
	if cjkAtlasOffset < 100 {
		t.Errorf("cjkAtlasOffset = %d, should be ≥100 to avoid index collision", cjkAtlasOffset)
	}
}

func TestGPUTextEngine_DirtyAtlasesEmpty(t *testing.T) {
	engine := NewGPUTextEngine()
	dirty := engine.DirtyAtlases()
	if len(dirty) != 0 {
		t.Errorf("new engine should have 0 dirty atlases, got %d", len(dirty))
	}
}

func TestGPUTextEngine_AtlasRGBADataNil(t *testing.T) {
	engine := NewGPUTextEngine()

	data, w, h := engine.AtlasRGBAData(0)
	if data != nil || w != 0 || h != 0 {
		t.Error("empty Latin atlas should return nil")
	}

	data, w, h = engine.AtlasRGBAData(cjkAtlasOffset)
	if data != nil || w != 0 || h != 0 {
		t.Error("empty CJK atlas should return nil")
	}
}

func TestGPUTextEngine_MarkCleanBothAtlases(t *testing.T) {
	engine := NewGPUTextEngine()
	// Should not panic for either atlas type.
	engine.MarkClean(0)
	engine.MarkClean(cjkAtlasOffset)
}

// --- IsCJKRune coverage for text rendering decisions ---

func TestIsCJKRune_TextRendering(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"Han", '中', true},
		{"Han_complex", '龍', true},
		{"Hiragana", 'あ', true},
		{"Katakana", 'ア', true},
		{"Hangul", '한', true},
		{"Fullwidth", 'Ａ', true},
		{"Latin", 'A', false},
		{"Digit", '1', false},
		{"Emoji", '😀', false},
		{"Arabic", 'ع', false},
		{"Cyrillic", 'Д', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := text.IsCJKRune(tt.r); got != tt.want {
				t.Errorf("IsCJKRune(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestSelectGlyphMaskHinting_CJKEnterprise(t *testing.T) {
	// Enterprise validation: CJK hinting matches platform behaviors.
	tests := []struct {
		name        string
		isCJK       bool
		deviceScale float64
		want        text.Hinting
		reason      string
	}{
		{
			name: "cjk_1x_vertical", isCJK: true, deviceScale: 1.0,
			want:   text.HintingVertical,
			reason: "FreeType afcjk: Y-direction only for CJK",
		},
		{
			name: "cjk_1.5x_vertical", isCJK: true, deviceScale: 1.5,
			want:   text.HintingVertical,
			reason: "150% scale: still benefits from vertical hinting",
		},
		{
			name: "cjk_2x_none", isCJK: true, deviceScale: 2.0,
			want:   text.HintingNone,
			reason: "macOS Core Text: ignores hinting on Retina",
		},
		{
			name: "cjk_3x_none", isCJK: true, deviceScale: 3.0,
			want:   text.HintingNone,
			reason: "HiDPI: pixel density makes hinting unnecessary",
		},
		{
			name: "latin_1x_full", isCJK: false, deviceScale: 1.0,
			want:   text.HintingFull,
			reason: "Latin: full grid-fitting for crisp stems (integer rounded-advance placement keeps spacing even)",
		},
		{
			name: "latin_2x_full", isCJK: false, deviceScale: 2.0,
			want:   text.HintingFull,
			reason: "Latin on Retina: full hinting; rounded-advance placement keeps spacing even",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectGlyphMaskHinting(14, identityMatrix(), tt.isCJK, tt.deviceScale)
			if got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

func identityMatrix() render.Matrix {
	return render.Identity()
}

// TestGlyphMaskSkipsNotdef matches CPU text.Draw: GID 0 (.notdef) must not
// produce ink. CJK-only fonts map missing Latin runes to GID 0; drawing tofu
// boxes previously inflated cjk_text body/mixed coverage by 2–3x.
func TestGlyphMaskSkipsNotdef(t *testing.T) {
	fontPath := "/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf"
	src, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font unavailable: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	face := src.Face(12)
	s := "12px: 中"
	var hasNotdef, hasCJK bool
	for g := range face.Glyphs(s) {
		if g.GID == 0 {
			hasNotdef = true
		}
		if text.IsCJKRune(g.Rune) && g.GID != 0 {
			hasCJK = true
		}
	}
	if !hasNotdef || !hasCJK {
		t.Skipf("fixture font does not map Latin to .notdef and CJK to real gids (notdef=%v cjk=%v)", hasNotdef, hasCJK)
	}

	engine := NewGlyphMaskEngine()
	batch, err := engine.LayoutText(face, s, 20, 40, render.Black, render.Identity(), 1.0)
	if err != nil {
		t.Fatalf("LayoutText: %v", err)
	}
	// Only the CJK glyph "中" should produce a quad; Latin .notdef must be skipped.
	if len(batch.Quads) != 1 {
		t.Fatalf("expected 1 quad for CJK glyph only, got %d", len(batch.Quads))
	}
	// Quad should start near 20+advance("12px: ") ≈ 83, not near 20.
	if batch.Quads[0].X0 < 70 {
		t.Fatalf("CJK quad X0=%.1f too far left; .notdef tofu may still be drawn", batch.Quads[0].X0)
	}
}
