package gpu

import (
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestGlyphPlacementCJKVerticalSnapsX guards the ADR-027 follow-up fix:
// CJK Vertical-hinted text must snap to integer device X, exactly like Full
// hinting. Fractional X placement splits 1px vertical CJK stems into two
// half-coverage columns in the rasterizer (FreeType shows the same break at
// fracX=0.5), which produced broken/uneven strokes on CJK labels.
func TestGlyphPlacementCJKVerticalSnapsX(t *testing.T) {
	hinting := text.HintingVertical
	absX, absY, fracX, fracY := glyphPlacement(10.4, 3.7, 1.0, 1.0, hinting, 10.0, true)
	if fracX != 0 {
		t.Fatalf("CJK Vertical hint: fracX = %v, want 0 (integer device X)", fracX)
	}
	if absX != 10.0 {
		t.Fatalf("CJK Vertical hint: absX = %v, want snapped 10.0", absX)
	}
	if fracY != 0 {
		t.Fatalf("CJK Vertical hint: fracY = %v, want 0 (grid-fitted Y)", fracY)
	}
	if absY != 4.0 {
		t.Fatalf("CJK Vertical hint: absY = %v, want rounded 4.0", absY)
	}
}

// TestGlyphPlacementFullSnapsX guards the pre-existing Full-hint contract.
func TestGlyphPlacementFullSnapsX(t *testing.T) {
	absX, _, fracX, fracY := glyphPlacement(10.4, 3.7, 1.0, 1.0, text.HintingFull, 10.0, true)
	if fracX != 0 || absX != 10.0 {
		t.Fatalf("Full hint: fracX=%v absX=%v, want 0/10.0", fracX, absX)
	}
	if fracY != 0 {
		t.Fatalf("Full hint: fracY = %v, want 0", fracY)
	}
}

// TestGlyphPlacementNoneKeepsXFraction guards the LCD path (snapX=false):
// the X fraction picks the RGB subpixel phase and must survive even with no
// hinting. Y still snaps to the integer baseline like every other mode.
func TestGlyphPlacementNoneKeepsXFraction(t *testing.T) {
	absX, _, fracX, fracY := glyphPlacement(10.4, 3.7, 1.0, 1.0, text.HintingNone, 0, false)
	if fracX == 0 {
		t.Fatalf("None hint + snapX=false: fracX=0, want fractional X (LCD phase)")
	}
	if fracY != 0 {
		t.Fatalf("None hint: fracY = %v, want 0 (integer baseline)", fracY)
	}
	if absX != 10.4 {
		t.Fatalf("None hint: absX = %v, want unchanged 10.4", absX)
	}
}

// TestGlyphPlacementNoneSnapsX guards the non-LCD integer-grid contract
// (R21): unhinted masks also snap X to the rounded-advance grid, exactly like
// hinted text, so advance spacing never jitters with the origin fraction.
func TestGlyphPlacementNoneSnapsX(t *testing.T) {
	absX, _, fracX, fracY := glyphPlacement(10.4, 3.7, 1.0, 1.0, text.HintingNone, 10.0, true)
	if fracX != 0 || absX != 10.0 {
		t.Fatalf("None hint + snapX: fracX=%v absX=%v, want 0/10.0", fracX, absX)
	}
	if fracY != 0 {
		t.Fatalf("None hint: fracY = %v, want 0", fracY)
	}
}

// TestGlyphPlacementScaledCTMXAxisFraction guards the scaled-CTM contract:
// devScaleX runs at raster resolution (deviceScale*rasterScale) so the
// sub-pixel phase aligns with CPU drawStringScaled's continuous placement,
// while Y keeps the axis deviceScale (integer baseline).
func TestGlyphPlacementScaledCTMXAxisFraction(t *testing.T) {
	absX, absY, fracX, fracY := glyphPlacement(10.4, 3.7, 2.0, 1.0, text.HintingNone, 0, false)
	if fracX < 0.799 || fracX > 0.801 {
		t.Fatalf("scaled CTM: fracX = %v, want ~0.8 (raster-res fraction)", fracX)
	}
	if absX != 10.4 {
		t.Fatalf("scaled CTM: absX = %v, want unchanged 10.4 (continuous user px)", absX)
	}
	if fracY != 0 || absY != 4.0 {
		t.Fatalf("scaled CTM: fracY=%v absY=%v, want 0/4.0 (Y grid-fitted at deviceScale)", fracY, absY)
	}
}

// TestGlyphPlacementLCDKeepsXPhase guards the LCD contract: the X fraction
// selects the RGB subpixel phase and must survive even with hinting.
func TestGlyphPlacementLCDKeepsXPhase(t *testing.T) {
	// snapX=false is what the layout path passes for LCD (snapX excludes LCD).
	_, _, fracX, fracY := glyphPlacement(10.4, 3.7, 1.0, 1.0, text.HintingVertical, 0, false)
	if fracX < 0.399 || fracX > 0.401 {
		t.Fatalf("LCD-phase placement: fracX = %v, want ~0.4 (X phase preserved)", fracX)
	}
	if fracY != 0 {
		t.Fatalf("LCD-phase placement: fracY = %v, want 0 (Y still grid-fitted)", fracY)
	}
}

// TestSnapXGridMonotonic guards the advance grid: snapping must keep glyphs
// ordered and spacing even (per-glyph round of cumulative advance).
func TestSnapXGridMonotonic(t *testing.T) {
	glyphs := []text.ShapedGlyph{
		{X: 0.2}, {X: 15.6}, {X: 31.3}, {X: 46.9},
	}
	grid := snapXGrid(glyphs, 1.3, 1.0)
	if len(grid) != len(glyphs) {
		t.Fatalf("grid len = %d, want %d", len(grid), len(glyphs))
	}
	for i := 1; i < len(grid); i++ {
		if grid[i] < grid[i-1] {
			t.Fatalf("grid not monotonic at %d: %v < %v", i, grid[i], grid[i-1])
		}
	}
	for _, g := range grid {
		if g != float64(int(g)) {
			t.Fatalf("grid value %v not integer device X", g)
		}
	}
}
