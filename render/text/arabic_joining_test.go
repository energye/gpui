package text

import "testing"

func TestArabicJoining_Forms_BehMeem(t *testing.T) {
	// Beh (dual) + Meem (dual) → init + fina in a two-letter word
	runes := []rune{0x0628, 0x0645} // ب م
	forms := computePresentationForms(runes)
	if len(forms) != 2 {
		t.Fatalf("len=%d", len(forms))
	}
	if forms[0] != formInit {
		t.Fatalf("beh form=%v want init", forms[0])
	}
	if forms[1] != formFina {
		t.Fatalf("meem form=%v want fina", forms[1])
	}
}

func TestArabicJoining_Forms_ThreeLetter(t *testing.T) {
	// Beh + Heh + Meem → init, medi, fina
	runes := []rune{0x0628, 0x0647, 0x0645}
	forms := computePresentationForms(runes)
	if forms[0] != formInit || forms[1] != formMedi || forms[2] != formFina {
		t.Fatalf("forms=%v want init,medi,fina", forms)
	}
}

func TestArabicJoining_Forms_AlefRightOnly(t *testing.T) {
	// Alef is right-joining only: after a dual letter → fina for dual, isol/none for alef
	// ب + ا → beh should be fina (joins prev? no prev; joins next? alef joins right only so
	// dual joins left into alef? joinsLeft(dual)=true, joinsRight(alef)=true → joinNext for beh
	// so beh=init, alef: joinPrev=true (prev dual joins left), joinNext=false → fina
	runes := []rune{0x0628, 0x0627} // ب ا
	forms := computePresentationForms(runes)
	if forms[0] != formInit {
		t.Fatalf("beh=%v want init", forms[0])
	}
	if forms[1] != formFina {
		t.Fatalf("alef=%v want fina", forms[1])
	}
}

func TestArabicJoining_TransparentMark(t *testing.T) {
	// Beh + fatha + Meem — mark transparent, still init/fina on letters
	runes := []rune{0x0628, 0x064E, 0x0645} // ب َ م
	forms := computePresentationForms(runes)
	if forms[0] != formInit || forms[1] != formNone || forms[2] != formFina {
		t.Fatalf("forms=%v want init,none,fina", forms)
	}
}

func TestArabicJoining_Isolated(t *testing.T) {
	runes := []rune{0x0628} // single beh
	forms := computePresentationForms(runes)
	if forms[0] != formIsol {
		t.Fatalf("form=%v want isol", forms[0])
	}
}

func TestNeedsArabicJoining(t *testing.T) {
	if !needsArabicJoining([]rune("سلام")) {
		t.Fatal("expected arabic")
	}
	if needsArabicJoining([]rune("hello")) {
		t.Fatal("latin should not need joining")
	}
}

// TestGSUB_StagedArabic_MaskOnlyFormPositions: init feature only touches init slots.
func TestGSUB_StagedArabic_MaskOnlyFormPositions(t *testing.T) {
	// Single subst: coverage [10], delta +1 → 11
	// Glyph buffer: cluster 0 form init, cluster 1 form fina
	// Mask for init only allows i=0
	nestedCov := buildCoverageFormat1(10)
	nested := make([]byte, 6+len(nestedCov))
	nested[1] = 1
	nested[2], nested[3] = 0, 6
	nested[4], nested[5] = 0, 1 // delta +1
	copy(nested[6:], nestedCov)

	glyphs := []shapingGlyph{{gid: 10, cluster: 0}, {gid: 10, cluster: 1}}
	mask := []bool{true, false}
	out := applySingleSubstMasked(nested, append([]shapingGlyph(nil), glyphs...), mask)
	if out[0].gid != 11 {
		t.Fatalf("masked init slot: gid=%d want 11", out[0].gid)
	}
	if out[1].gid != 10 {
		t.Fatalf("fina slot must not change: gid=%d", out[1].gid)
	}
}
