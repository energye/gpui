// Indic GSUB feature staging (ENGINE_GAPS G1.c lightweight state machine).
//
// Full Uniscribe/HarfBuzz Indic reordering is out of scope. We apply OpenType
// features in the conventional Indic stage order so fonts that encode behavior
// under rphf/half/vatu/pres/… get a deterministic, script-aware pass instead of
// a single sorted-lookup soup.
//
// Reference: Microsoft OpenType Indic shaping spec (Devanagari / “dev2” model).
package text

// needsIndicShaping reports whether runes need Indic-style staged GSUB.
func needsIndicShaping(runes []rune) bool {
	for _, r := range runes {
		switch DetectScript(r) {
		case ScriptDevanagari, ScriptBengali, ScriptGurmukhi, ScriptGujarati,
			ScriptOriya, ScriptTamil, ScriptTelugu, ScriptKannada,
			ScriptMalayalam, ScriptSinhala, ScriptMyanmar, ScriptKhmer,
			ScriptTibetan:
			return true
		}
		// Devanagari block (covers most Hindi)
		if r >= 0x0900 && r <= 0x097F {
			return true
		}
		if r >= 0x0980 && r <= 0x09FF { // Bengali
			return true
		}
		if r >= 0x0A00 && r <= 0x0A7F { // Gurmukhi
			return true
		}
		if r >= 0x0A80 && r <= 0x0AFF { // Gujarati
			return true
		}
		if r >= 0x0B00 && r <= 0x0B7F { // Oriya
			return true
		}
		if r >= 0x0B80 && r <= 0x0BFF { // Tamil
			return true
		}
		if r >= 0x0C00 && r <= 0x0C7F { // Telugu
			return true
		}
		if r >= 0x0C80 && r <= 0x0CFF { // Kannada
			return true
		}
		if r >= 0x0D00 && r <= 0x0D7F { // Malayalam
			return true
		}
	}
	return false
}

func tag4(a, b, c, d byte) [4]byte { return [4]byte{a, b, c, d} }

// indicGSUBStages returns ordered feature-tag groups for Indic shaping.
// Stages are applied sequentially; within a stage, LangSys feature order is kept
// (collectLookupIndicesOrdered). Tags not in desiredTags are skipped.
func indicGSUBStages() [][][4]byte {
	return [][][4]byte{
		// 0: local forms + composition basics
		{
			tag4('l', 'o', 'c', 'l'),
			tag4('c', 'c', 'm', 'p'),
			tag4('n', 'u', 'k', 't'),
			tag4('a', 'k', 'h', 'n'),
		},
		// 1: reph / rakar / below-base / half / post-base forms
		{
			tag4('r', 'p', 'h', 'f'),
			tag4('r', 'k', 'r', 'f'),
			tag4('p', 'r', 'e', 'f'),
			tag4('b', 'l', 'w', 'f'),
			tag4('a', 'b', 'v', 'f'),
			tag4('h', 'a', 'l', 'f'),
			tag4('p', 's', 't', 'f'),
			tag4('v', 'a', 't', 'u'),
			tag4('c', 'j', 'c', 't'),
			tag4('c', 'f', 'a', 'r'),
		},
		// 2: presentation / above-below substitutions
		{
			tag4('p', 'r', 'e', 's'),
			tag4('a', 'b', 'v', 's'),
			tag4('b', 'l', 'w', 's'),
			tag4('p', 's', 't', 's'),
			tag4('h', 'a', 'l', 'n'),
			tag4('c', 'a', 'l', 't'),
		},
		// 3: remaining (liga etc.) — applied last
	}
}

// splitIndicDesired splits desired tags into staged Indic tags vs leftover general.
func splitIndicDesired(desired [][4]byte) (stages [][][4]byte, rest [][4]byte) {
	staged := make(map[[4]byte]bool)
	for _, st := range indicGSUBStages() {
		var present [][4]byte
		for _, tag := range st {
			for _, d := range desired {
				if d == tag {
					present = append(present, tag)
					staged[tag] = true
					break
				}
			}
		}
		if len(present) > 0 {
			stages = append(stages, present)
		}
	}
	for _, d := range desired {
		if !staged[d] {
			// Skip Arabic form tags in Indic path (harmless if present).
			switch d {
			case tag4('i', 'n', 'i', 't'), tag4('m', 'e', 'd', 'i'),
				tag4('f', 'i', 'n', 'a'), tag4('i', 's', 'o', 'l'):
				continue
			}
			rest = append(rest, d)
		}
	}
	return stages, rest
}

// applyGSUBStagedIndic applies GSUB in Indic stage order then remaining tags.
func (g *gsubTable) applyGSUBStagedIndic(
	glyphs []shapingGlyph,
	scriptTag, langTag [4]byte,
	desiredTags [][4]byte,
) []shapingGlyph {
	if g == nil {
		return glyphs
	}
	stages, rest := splitIndicDesired(desiredTags)
	for _, st := range stages {
		glyphs = g.applyGSUBFeatures(glyphs, scriptTag, langTag, st, nil)
	}
	if len(rest) > 0 {
		glyphs = g.applyGSUBFeatures(glyphs, scriptTag, langTag, rest, nil)
	}
	return glyphs
}
