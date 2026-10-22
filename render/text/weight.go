//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

// Weight resolution (WithWeight → real family cut, wght variation, or
// synthetic outline embolden).
//
// A requested grade resolves at Face creation time:
//   - FontFamily member → the closest real static cut (Regular/Semibold/
//     Bold/...) is selected first; no synthetic bold is applied to it;
//   - variable font with a wght axis → wght variation set to the grade
//     (real outlines; advances follow the existing HVAR machinery and all
//     variation-keyed caches separate instances with no further changes);
//   - standalone static font, grade >= 600 (SemiBold and up) → synthetic
//     outline embolden (EmboldenOutline, size/24 per side, FreeType/Skia
//     fake bold; advances unchanged, so layout/measure/wrap need no weight
//     keys — only raster caches separate via GlyphMaskFlagBold);
//   - grade < 600 on a static font → as designed, no-op.
//
// An explicit wght axis value in WithVariations takes precedence over
// WithWeight (weight fills the axis only when absent), regardless of
// option order. Weight 0 (unset) means as designed.

// weightWghtTag is the OpenType wght axis tag for direct comparison.
var weightWghtTag = AxisWeight

// hasWeightAxis reports whether the font offers a wght variation axis.
func hasWeightAxis(source *FontSource) bool {
	if source == nil {
		return false
	}
	if !source.IsVariable() {
		return false
	}
	for _, ax := range source.VariationAxes() {
		if ax.Tag == weightWghtTag {
			return true
		}
	}
	return false
}

// hasWghtVariation reports whether vars already carry a wght axis value.
func hasWghtVariation(vars []FontVariation) bool {
	for _, v := range vars {
		if v.Tag == weightWghtTag {
			return true
		}
	}
	return false
}

// resolveWeight applies the requested grade to cfg at Face creation:
// upserts a wght variation when the font has the axis, else records
// synthetic embolden for grades >= 600. Idempotent: re-derivation over
// an already-resolved config keeps the explicit wght value.
func resolveWeight(source *FontSource, cfg *faceConfig) {
	w := cfg.weight
	if w == 0 {
		return
	}
	if hasWghtVariation(cfg.variations) {
		return
	}
	if hasWeightAxis(source) {
		cfg.variations = append(cfg.variations, FontVariation{
			Tag:   weightWghtTag,
			Value: float32(w),
		})
	}
}

// FaceEmbolden is the nil-safe single entry for synthetic-bold checks.
// All Face implementations carry Embolden (see Face interface); this wrapper
// only adds nil safety so GPU/CPU callers never branch on concrete types.
func FaceEmbolden(f Face) bool {
	if f == nil {
		return false
	}
	return f.Embolden()
}
