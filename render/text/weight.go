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

// Weight resolution (WithWeight → wght variation or synthetic embolden).
//
// A requested grade resolves at Face creation time:
//   - variable font with a wght axis → wght variation set to the grade
//     (real outlines; advances follow the existing HVAR machinery and all
//     variation-keyed caches separate instances with no further changes);
//   - otherwise, grade >= 600 (SemiBold and up) → synthetic embolden
//     (EmboldenMask dilation, Skia fakeBold class; advances unchanged, so
//     layout/measure/wrap need no weight keys — only raster caches
//     separate via GlyphMaskFlagBold);
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

// EmboldenRadius returns the mask-dilation radius in pixels for synthetic
// bold at the given raster size (~3% of em, minimum 1px so small sizes
// still gain visible weight).
func EmboldenRadius(sizePx float64) int {
	r := int(sizePx*0.03 + 0.999)
	if r < 1 {
		r = 1
	}
	return r
}

// EmboldenResult returns a dilated copy of a coverage mask (3x3 max-filter
// repeated radius times), thickening stems without changing the advance —
// Skia fakeBold class. Bearings shift so placement math is unchanged for
// callers (same origin, grown canvas). Nil/empty results pass through.
// LCD (3x-wide) masks dilate uniformly in mask space; subpixel asymmetry
// from the 3x packing is accepted (LCD+bold is rare).
func EmboldenResult(res *GlyphMaskResult, sizePx float64) *GlyphMaskResult {
	if res == nil || len(res.Mask) == 0 || res.Width <= 0 || res.Height <= 0 {
		return res
	}
	radius := EmboldenRadius(sizePx)
	grownW, grownH := res.Width+2*radius, res.Height+2*radius
	grown := make([]byte, grownW*grownH)
	for y := 0; y < res.Height; y++ {
		copy(grown[(y+radius)*grownW+radius:], res.Mask[y*res.Width:(y+1)*res.Width])
	}
	dilateMask(grown, grownW, grownH, radius)
	return &GlyphMaskResult{
		Mask:     grown,
		Width:    grownW,
		Height:   grownH,
		BearingX: res.BearingX - float32(radius),
		BearingY: res.BearingY + float32(radius),
		Advance:  res.Advance,
	}
}

// dilateMask max-filters mask in place (radius in pixels).
func dilateMask(mask []byte, width, height, radius int) {
	src := append([]byte(nil), mask...)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			best := src[y*width+x]
			if best == 0xFF {
				continue
			}
			for dy := -radius; dy <= radius && best != 0xFF; dy++ {
				yy := y + dy
				if yy < 0 || yy >= height {
					continue
				}
				for dx := -radius; dx <= radius; dx++ {
					xx := x + dx
					if xx < 0 || xx >= width {
						continue
					}
					if v := src[yy*width+xx]; v > best {
						best = v
					}
				}
			}
			mask[y*width+x] = best
		}
	}
}
