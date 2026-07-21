// Arabic joining analysis for presentation-form feature masks (ENGINE_GAPS G1.c).
//
// Assigns each glyph a joining form (isol/init/medi/fina) so GSUB features of
// those tags only apply to the correct positions. This is a lightweight
// state machine over Unicode Arabic joining types — not a full HarfBuzz
// complex shaper, but enough for fonts that encode forms under init/medi/fina/isol.
//
// Reference: Unicode ArabicJoining.txt; OpenType Arabic shaping model.
package text

// joiningType is the Unicode Arabic joining class (simplified).
type joiningType uint8

const (
	joinNonJoining  joiningType = iota // U (does not join)
	joinLeft                           // L (joins left only) — rare
	joinRight                          // R (joins right only)
	joinDual                           // D (joins both sides)
	joinJoinCausing                    // C (like tatweel / zwj)
	joinTransparent                    // T (marks — ignored for joining)
)

// presentationForm is which OT feature should apply at this slot.
type presentationForm uint8

const (
	formNone presentationForm = iota
	formIsol
	formInit
	formMedi
	formFina
)

// arabicJoiningType returns a simplified joining type for r.
// Transparent for Mn marks; dual/right for Arabic letters; non-joining else.
func arabicJoiningType(r rune) joiningType {
	// Combining marks (general) — treat as transparent for joining.
	if r >= 0x0300 && r <= 0x036F {
		return joinTransparent
	}
	if r >= 0x064B && r <= 0x065F { // Arabic tashkeel
		return joinTransparent
	}
	if r >= 0x0610 && r <= 0x061A {
		return joinTransparent
	}
	if r == 0x0670 || (r >= 0x06D6 && r <= 0x06ED) {
		return joinTransparent
	}
	if r == 0x0640 { // tatweel
		return joinJoinCausing
	}
	if r == 0x200D { // ZWJ
		return joinJoinCausing
	}
	if r == 0x200C { // ZWNJ
		return joinNonJoining
	}

	// Non-joining / right-joining Arabic (simplified Unicode ArabicJoining subset).
	switch r {
	case 0x0621: // hamza
		return joinNonJoining
	case 0x0622, 0x0623, 0x0624, 0x0625, 0x0627, 0x0629,
		0x062F, 0x0630, 0x0631, 0x0632, 0x0648,
		0x0671, 0x0672, 0x0673, 0x0675, 0x0676, 0x0677,
		0x0688, 0x0689, 0x068A, 0x068B, 0x068C, 0x068D, 0x068E, 0x068F,
		0x0690, 0x0691, 0x0692, 0x0693, 0x0694, 0x0695, 0x0696, 0x0697, 0x0698, 0x0699,
		0x06C0, 0x06C3, 0x06C4, 0x06C5, 0x06C6, 0x06C7, 0x06C8, 0x06C9, 0x06CA, 0x06CB,
		0x06CF, 0x06D2, 0x06D3, 0x06D5:
		return joinRight
	}

	// Main Arabic letters dual-joining
	if r >= 0x0620 && r <= 0x064A {
		return joinDual
	}
	if r >= 0x066E && r <= 0x06D3 {
		// many dual; right-only already listed
		return joinDual
	}
	if r >= 0x0671 && r <= 0x06D5 {
		return joinDual
	}
	// Arabic Presentation Forms-A/B already shaped — non-joining
	if r >= 0xFB50 && r <= 0xFDFF {
		return joinNonJoining
	}
	if r >= 0xFE70 && r <= 0xFEFF {
		return joinNonJoining
	}
	return joinNonJoining
}

func joinsLeft(t joiningType) bool {
	return t == joinLeft || t == joinDual || t == joinJoinCausing
}

func joinsRight(t joiningType) bool {
	return t == joinRight || t == joinDual || t == joinJoinCausing
}

// computePresentationForms assigns isol/init/medi/fina for each rune.
// Transparent marks inherit neighbors for joining through them.
func computePresentationForms(runes []rune) []presentationForm {
	n := len(runes)
	forms := make([]presentationForm, n)
	if n == 0 {
		return forms
	}
	types := make([]joiningType, n)
	for i, r := range runes {
		types[i] = arabicJoiningType(r)
	}

	// Prev joining-capable index (skip transparent).
	prevJoin := make([]int, n)
	nextJoin := make([]int, n)
	last := -1
	for i := 0; i < n; i++ {
		prevJoin[i] = last
		if types[i] != joinTransparent {
			last = i
		}
	}
	last = -1
	for i := n - 1; i >= 0; i-- {
		nextJoin[i] = last
		if types[i] != joinTransparent {
			last = i
		}
	}

	for i := 0; i < n; i++ {
		jt := types[i]
		if jt == joinTransparent || jt == joinNonJoining {
			forms[i] = formNone
			continue
		}
		// Can we join to previous solid glyph?
		joinPrev := false
		if p := prevJoin[i]; p >= 0 {
			// Previous wants to join "left" (toward us in logical LTR buffer;
			// for Arabic logical order is still stored LTR then RTL-reordered).
			// In logical order: prev is to the right visually for RTL after reorder,
			// but joining analysis is done on logical order before reorder.
			// Standard: join if prev joins-left and we join-right.
			joinPrev = joinsLeft(types[p]) && joinsRight(jt)
		}
		joinNext := false
		if nx := nextJoin[i]; nx >= 0 {
			joinNext = joinsLeft(jt) && joinsRight(types[nx])
		}

		switch {
		case joinPrev && joinNext:
			forms[i] = formMedi
		case joinPrev && !joinNext:
			forms[i] = formFina
		case !joinPrev && joinNext:
			forms[i] = formInit
		default:
			forms[i] = formIsol
		}
	}
	return forms
}

// formFeatureTag maps a presentation form to its OpenType feature tag.
func formFeatureTag(f presentationForm) [4]byte {
	switch f {
	case formIsol:
		return [4]byte{'i', 's', 'o', 'l'}
	case formInit:
		return [4]byte{'i', 'n', 'i', 't'}
	case formMedi:
		return [4]byte{'m', 'e', 'd', 'i'}
	case formFina:
		return [4]byte{'f', 'i', 'n', 'a'}
	default:
		return [4]byte{}
	}
}

// glyphFormMask builds a parallel mask: true if the glyph at i should receive
// the given presentation feature. Marks (transparent) never get form features.
func glyphFormMask(forms []presentationForm, want presentationForm) []bool {
	m := make([]bool, len(forms))
	for i, f := range forms {
		m[i] = f == want
	}
	return m
}

// needsArabicJoining reports whether runes contain Arabic-script material
// that benefits from init/medi/fina/isol staged application.
func needsArabicJoining(runes []rune) bool {
	for _, r := range runes {
		if DetectScript(r) == ScriptArabic {
			return true
		}
		// Arabic block without going through DetectScript
		if r >= 0x0600 && r <= 0x06FF {
			return true
		}
		if r >= 0x0750 && r <= 0x077F {
			return true
		}
	}
	return false
}
