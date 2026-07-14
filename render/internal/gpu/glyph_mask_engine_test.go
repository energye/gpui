//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func TestSelectGlyphMaskLCD(t *testing.T) {
	tests := []struct {
		name     string
		fontSize float64
		matrix   render.Matrix
		want     bool
	}{
		{
			name:     "small_identity",
			fontSize: 12,
			matrix:   render.Identity(),
			want:     true,
		},
		{
			name:     "small_translation",
			fontSize: 16,
			matrix:   render.Matrix{A: 1, B: 0, C: 50, D: 0, E: 1, F: 30},
			want:     true,
		},
		{
			name:     "threshold_48px",
			fontSize: 48,
			matrix:   render.Identity(),
			want:     true,
		},
		{
			name:     "above_threshold",
			fontSize: 49,
			matrix:   render.Identity(),
			want:     false,
		},
		{
			name:     "large_72px",
			fontSize: 72,
			matrix:   render.Identity(),
			want:     false,
		},
		{
			name:     "rotated_small",
			fontSize: 12,
			matrix:   render.Matrix{A: 0.707, B: -0.707, C: 0, D: 0.707, E: 0.707, F: 0},
			want:     false,
		},
		{
			name:     "skewed",
			fontSize: 14,
			matrix:   render.Matrix{A: 1, B: 0.3, C: 0, D: 0, E: 1, F: 0},
			want:     false,
		},
		{
			name:     "uniform_scale_small",
			fontSize: 12,
			matrix:   render.Matrix{A: 2, B: 0, C: 0, D: 0, E: 2, F: 0},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectGlyphMaskLCD(tt.fontSize, tt.matrix)
			if got != tt.want {
				t.Errorf("selectGlyphMaskLCD(%v, %v) = %v, want %v",
					tt.fontSize, tt.matrix, got, tt.want)
			}
		})
	}
}

func TestGlyphMaskEngine_SetLCDLayout(t *testing.T) {
	engine := NewGlyphMaskEngine()

	// Default should be LCDLayoutNone.
	if engine.LCDLayout() != text.LCDLayoutNone {
		t.Errorf("default LCD layout = %v, want LCDLayoutNone", engine.LCDLayout())
	}

	// Set to RGB.
	engine.SetLCDLayout(text.LCDLayoutRGB)
	if engine.LCDLayout() != text.LCDLayoutRGB {
		t.Errorf("after SetLCDLayout(RGB) = %v, want LCDLayoutRGB", engine.LCDLayout())
	}

	// Set to BGR.
	engine.SetLCDLayout(text.LCDLayoutBGR)
	if engine.LCDLayout() != text.LCDLayoutBGR {
		t.Errorf("after SetLCDLayout(BGR) = %v, want LCDLayoutBGR", engine.LCDLayout())
	}

	// Set back to None.
	engine.SetLCDLayout(text.LCDLayoutNone)
	if engine.LCDLayout() != text.LCDLayoutNone {
		t.Errorf("after SetLCDLayout(None) = %v, want LCDLayoutNone", engine.LCDLayout())
	}
}

func TestGlyphMaskEngine_SetLCDFilter(t *testing.T) {
	engine := NewGlyphMaskEngine()

	// Custom filter should not panic.
	custom := text.LCDFilter{Weights: [5]float32{0.1, 0.2, 0.4, 0.2, 0.1}}
	engine.SetLCDFilter(custom)
}

func TestGlyphMaskRasterScaleFromTotalMatrix(t *testing.T) {
	tests := []struct {
		name        string
		matrix      render.Matrix
		deviceScale float64
		want        float64
	}{
		{
			name:        "identity",
			matrix:      render.Identity(),
			deviceScale: 1,
			want:        1,
		},
		{
			name:        "hidpi_without_user_scale",
			matrix:      render.Scale(2, 2),
			deviceScale: 2,
			want:        1,
		},
		{
			name:        "hidpi_with_user_scale",
			matrix:      render.Scale(4, 4),
			deviceScale: 2,
			want:        2,
		},
		{
			name:        "user_scale_down",
			matrix:      render.Scale(0.7, 0.7),
			deviceScale: 1,
			want:        0.7,
		},
		{
			name:        "rotation_not_baked",
			matrix:      render.Rotate(math.Pi / 6),
			deviceScale: 1,
			want:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := glyphMaskRasterScale(tt.matrix, tt.deviceScale); math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("glyphMaskRasterScale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlyphMaskFontSizeIncludesRasterScale(t *testing.T) {
	if got := glyphMaskFontSize(24, 1, 2); got != 48 {
		t.Fatalf("glyphMaskFontSize scale2 = %v, want 48", got)
	}
	if got := glyphMaskFontSize(24, 2, 1); got != 48 {
		t.Fatalf("glyphMaskFontSize hidpi = %v, want 48", got)
	}
	if got := glyphMaskFontSize(24, 1, 0.7); math.Abs(got-16.8) > 1e-9 {
		t.Fatalf("glyphMaskFontSize scale0.7 = %v, want 16.8", got)
	}
}

func TestGlyphMaskLayoutBakesUniformScaleWithoutChangingUserQuadSize(t *testing.T) {
	face := glyphMaskTestFont(t, 24)
	engine := NewGlyphMaskEngine()
	color := render.RGBA{A: 1}

	identity, err := engine.LayoutText(face, "H", 10, 50, color, render.Identity(), 1)
	if err != nil {
		t.Fatalf("LayoutText identity: %v", err)
	}
	scaled, err := engine.LayoutText(face, "H", 10, 50, color, render.Scale(2, 2), 1)
	if err != nil {
		t.Fatalf("LayoutText scale2: %v", err)
	}
	if len(identity.Quads) == 0 || len(scaled.Quads) == 0 {
		t.Fatalf("expected glyph quads: identity=%d scaled=%d", len(identity.Quads), len(scaled.Quads))
	}

	identityW := float64(identity.Quads[0].X1 - identity.Quads[0].X0)
	scaledW := float64(scaled.Quads[0].X1 - scaled.Quads[0].X0)
	ratio := scaledW / identityW
	if ratio < 0.75 || ratio > 1.25 {
		t.Fatalf("scaled user-space quad width ratio = %.3f, want near 1.0 (identity=%.2f scaled=%.2f)",
			ratio, identityW, scaledW)
	}
}

func TestSelectGlyphMaskHinting(t *testing.T) {
	tests := []struct {
		name        string
		fontSize    float64
		matrix      render.Matrix
		isCJK       bool
		deviceScale float64
		want        text.Hinting
	}{
		{
			name: "latin_small_identity", fontSize: 12, matrix: render.Identity(),
			want: text.HintingFull,
		},
		{
			name: "latin_small_translation", fontSize: 16,
			matrix: render.Matrix{A: 1, B: 0, C: 50, D: 0, E: 1, F: 30},
			want:   text.HintingFull,
		},
		{
			name: "latin_threshold_48px", fontSize: 48, matrix: render.Identity(),
			want: text.HintingFull,
		},
		{
			name: "latin_above_threshold", fontSize: 49, matrix: render.Identity(),
			want: text.HintingNone,
		},
		{
			name: "latin_large_72px", fontSize: 72, matrix: render.Identity(),
			want: text.HintingNone,
		},
		{
			name: "rotated_small", fontSize: 12,
			matrix: render.Matrix{A: 0.707, B: -0.707, C: 0, D: 0.707, E: 0.707, F: 0},
			want:   text.HintingNone,
		},
		{
			name: "skewed", fontSize: 14,
			matrix: render.Matrix{A: 1, B: 0.3, C: 0, D: 0, E: 1, F: 0},
			want:   text.HintingNone,
		},
		{
			name: "latin_uniform_scale", fontSize: 12,
			matrix: render.Matrix{A: 2, B: 0, C: 0, D: 0, E: 2, F: 0},
			want:   text.HintingFull,
		},
		// ADR-027: CJK script-aware hinting
		{
			name: "cjk_small_1x_vertical_only", fontSize: 14, matrix: render.Identity(),
			isCJK: true, deviceScale: 1.0,
			want: text.HintingVertical,
		},
		{
			name: "cjk_small_2x_none", fontSize: 14, matrix: render.Identity(),
			isCJK: true, deviceScale: 2.0,
			want: text.HintingNone,
		},
		{
			name: "cjk_large_none", fontSize: 72, matrix: render.Identity(),
			isCJK: true, deviceScale: 1.0,
			want: text.HintingNone,
		},
		{
			name: "cjk_rotated_none", fontSize: 14,
			matrix: render.Matrix{A: 0.707, B: -0.707, C: 0, D: 0.707, E: 0.707, F: 0},
			isCJK:  true, deviceScale: 1.0,
			want: text.HintingNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := tt.deviceScale
			if ds == 0 {
				ds = 1.0
			}
			got := selectGlyphMaskHinting(tt.fontSize, tt.matrix, tt.isCJK, ds)
			if got != tt.want {
				t.Errorf("selectGlyphMaskHinting(%v, %v, cjk=%v, scale=%v) = %v, want %v",
					tt.fontSize, tt.matrix, tt.isCJK, ds, got, tt.want)
			}
		})
	}
}
