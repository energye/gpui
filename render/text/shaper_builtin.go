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

// BuiltinShaper provides simple left-to-right text shaping.
// It supports Latin, Cyrillic, Greek, CJK, and other scripts that don't
// require complex text shaping (ligatures, contextual forms, etc.).
//
// BuiltinShaper was the default shaper before ADR-048 Phase 6.
// The default is now OwnShaper which provides GSUB/GPOS support.
//
// BuiltinShaper is stateless and safe for concurrent use.
type BuiltinShaper struct{}

// Shape implements the Shaper interface.
// It converts text to positioned glyphs using the font's glyph metrics.
// The font size is obtained from face.Size().
//
// The shaping is simple left-to-right positioning without:
//   - Ligature substitution (fi, fl, etc.)
//   - Kerning pairs
//   - Contextual alternates
//   - Right-to-left reordering
//
// For these features, use OwnShaper (the default) which provides GSUB/GPOS.
func (s *BuiltinShaper) Shape(text string, face Face) []ShapedGlyph {
	if text == "" || face == nil {
		return nil
	}

	// MultiFace has no single FontSource — shape via the face's own rune→
	// component resolution (face.Glyphs iterates per-rune covering faces,
	// the same walk DrawString uses). Positions accumulate per glyph
	// advance; the glyph GIDs come from each component's parsed font.
	if mf, isMulti := face.(*MultiFace); isMulti {
		runes := []rune(text)
		result := make([]ShapedGlyph, 0, len(runes))
		var x float64
		for cluster, r := range runes {
			if r < 0x20 && r != '\t' {
				continue
			}
			for g := range mf.Glyphs(string(r)) {
				result = append(result, ShapedGlyph{
					GID:      g.GID,
					Cluster:  cluster,
					X:        x,
					Y:        0,
					XAdvance: g.Advance,
					YAdvance: 0,
					IsCJK:    cjkRelated(r),
				})
				x += g.Advance
				break
			}
		}
		return result
	}

	source := face.Source()
	if source == nil {
		return nil
	}

	parsed := source.Parsed()
	if parsed == nil {
		return nil
	}

	size := face.Size()
	runes := []rune(text)
	result := make([]ShapedGlyph, 0, len(runes))

	var x float64

	for cluster, r := range runes {
		// Skip control characters (U+0000..U+001F) — no visual representation.
		// Tab (U+0009) is handled below with proper advance width.
		if r < 0x20 && r != '\t' {
			continue
		}

		var gid uint16
		var advance float64

		if r == '\t' {
			// Tab: use space GID (empty outline) with tab-stop advance.
			// Space GID has no contours → correctly skipped by outline renderer.
			gid, advance = tabAdvance(parsed, size)
		} else {
			gid = parsed.GlyphIndex(r)
			advance = parsed.GlyphAdvance(gid, size)
		}

		result = append(result, ShapedGlyph{
			GID:      GlyphID(gid),
			Cluster:  cluster,
			X:        x,
			Y:        0,
			XAdvance: advance,
			YAdvance: 0,
			IsCJK:    IsCJKRune(r),
		})

		x += advance
	}

	return result
}
