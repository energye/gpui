package text

import (
	"os"
	"testing"
)

func weightTestSource(t *testing.T) *FontSource {
	t.Helper()
	src, err := NewFontSource(requireTestFont(t))
	if err != nil {
		t.Fatalf("failed to load test font: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	return src
}

func loadVariableTestSource(t *testing.T) *FontSource {
	t.Helper()
	data, err := os.ReadFile("testdata/cantarell_vf_trimmed.ttf")
	if err != nil {
		t.Skip("variable test font not available")
	}
	src, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("failed to load variable font: %v", err)
	}
	if !src.IsVariable() {
		_ = src.Close()
		t.Skip("cantarell trimmed is not variable")
	}
	t.Cleanup(func() { _ = src.Close() })
	return src
}

// Static + 700 must embolden (no wght axis on Go Regular).
func TestWithWeight_StaticBoldEmboldens(t *testing.T) {
	src := weightTestSource(t)
	face := src.Face(16, WithWeight(WeightBold))
	if face.Weight() != WeightBold {
		t.Fatalf("Weight()=%v want 700", face.Weight())
	}
	if !FaceEmbolden(face) {
		t.Fatalf("FaceEmbolden=false want true for static 700")
	}
	for _, v := range face.Variations() {
		if v.Tag == AxisWeight {
			t.Fatalf("static 700 must not add wght variation, got %v", face.Variations())
		}
	}
}

// Static + 400 must not embolden.
func TestWithWeight_StaticRegularNoEmbolden(t *testing.T) {
	src := weightTestSource(t)
	face := src.Face(16, WithWeight(WeightRegular))
	if FaceEmbolden(face) {
		t.Fatalf("FaceEmbolden=true want false for static 400")
	}
}

// Explicit wght wins over WithWeight regardless of option order.
func TestWithWeight_ExplicitWghtWins(t *testing.T) {
	src := weightTestSource(t)
	explicit := NewFontVariation("wght", 300)
	a := src.Face(16, WithVariations(explicit), WithWeight(WeightBold))
	b := src.Face(16, WithWeight(WeightBold), WithVariations(explicit))
	for i, face := range []Face{a, b} {
		vars := face.Variations()
		if len(vars) != 1 || vars[0].Tag != AxisWeight || vars[0].Value != 300 {
			t.Fatalf("face %d variations=%v want single wght=300", i, vars)
		}
		if FaceEmbolden(face) {
			t.Fatalf("face %d embolden=true want false (explicit wght suppresses synthetic)", i)
		}
	}
}

// Variable font with wght axis resolves to real variation, never synthetic.
func TestWithWeight_VariableResolvesToWght(t *testing.T) {
	src := loadVariableTestSource(t)
	face := src.Face(16, WithWeight(WeightBold))
	if face.Weight() != WeightBold {
		t.Fatalf("Weight()=%v want 700", face.Weight())
	}
	found := false
	for _, v := range face.Variations() {
		if v.Tag == AxisWeight && v.Value == 700 {
			found = true
		}
	}
	if !found {
		t.Fatalf("variable 700 must carry wght=700 variation, got %v", face.Variations())
	}
	if FaceEmbolden(face) {
		t.Fatalf("FaceEmbolden=true want false for variable wght (real outlines)")
	}
}

// EmboldenResult grows ink, keeps advance.
func TestEmboldenResult_GrowsInkKeepsAdvance(t *testing.T) {
	before := &GlyphMaskResult{
		Mask:     []byte{0, 0, 0, 0, 255, 0, 0, 0, 0},
		Width:    3,
		Height:   3,
		BearingX: 1,
		BearingY: 2,
		Advance:  8,
	}
	after := EmboldenResult(before, 16)
	if after.Advance != before.Advance {
		t.Fatalf("Advance=%v want %v (synthetic bold never moves advance)", after.Advance, before.Advance)
	}
	if after.Width <= before.Width || after.Height <= before.Height {
		t.Fatalf("grown=%dx%d want larger than %dx%d", after.Width, after.Height, before.Width, before.Height)
	}
	ink := 0
	for _, v := range after.Mask {
		if v != 0 {
			ink++
		}
	}
	if ink <= 1 {
		t.Fatalf("ink pixels=%d want >1 (mask must thicken)", ink)
	}
	if after.BearingX != before.BearingX-1 || after.BearingY != before.BearingY+1 {
		t.Fatalf("bearings=(%v,%v) want (%v,%v)", after.BearingX, after.BearingY, before.BearingX-1, before.BearingY+1)
	}
	if got := EmboldenResult(nil, 16); got != nil {
		t.Fatalf("nil must pass through")
	}
}

// Bold and regular masks must not share an atlas key.
func TestGlyphMaskKey_BoldSeparates(t *testing.T) {
	a := MakeGlyphMaskKey(1, 2, 16, 0, 0)
	b := a
	b.Flags |= GlyphMaskFlagBold
	if a == b {
		t.Fatalf("bold key must differ from regular key")
	}
}

// FaceEmbolden handles nil and faces without Embolden method.
func TestFaceEmbolden_NilSafe(t *testing.T) {
	if FaceEmbolden(nil) {
		t.Fatalf("nil face must not embolden")
	}
}

// DeriveFace is the single size+weight entry: size-only preserves grade,
// weight override resolves once, fast path returns identity.
func TestDeriveFace_SingleEntry(t *testing.T) {
	src := weightTestSource(t)
	base := src.Face(16, WithWeight(WeightBold))
	if got := DeriveFace(base, 16, WeightBold); got != base {
		t.Fatalf("fast path must return identity")
	}
	sized := DeriveFace(base, 20, 0)
	if sized.Size() != 20 || sized.Weight() != WeightBold {
		t.Fatalf("size-only must preserve grade: size=%v weight=%v", sized.Size(), sized.Weight())
	}
	// Single derivation equals separate AtSize+WithWeight composition.
	mf, err := NewMultiFace(src.Face(14), src.Face(14))
	if err != nil {
		t.Fatalf("NewMultiFace: %v", err)
	}
	one := DeriveFace(mf, 18, WeightBold)
	if one.Size() != 18 || one.Weight() != WeightBold {
		t.Fatalf("multi derive: size=%v weight=%v want 18/700", one.Size(), one.Weight())
	}
	if !FaceEmbolden(one) {
		t.Fatalf("derived multi 700 on static must embolden")
	}
}
