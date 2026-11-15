//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	_ "github.com/energye/gpui/render/gpu"
)

// Regression (O13 发现第七件): StrokeString on a MultiFace face was always
// blank — MultiFace.Source() is nil by design and textOutlinePathFrom bailed
// on it. The fix outlines per-component via ComponentForRune (the same
// rune→face resolution DrawString uses).
func TestStrokeString_MultiFaceInk(t *testing.T) {
	mf, _, err := text.LoadMultiFace(24, text.FontRoleUI)
	if err != nil {
		t.Skipf("no fonts: %v", err)
	}

	// Pure outline path must be non-nil with contours now.
	p := render.TextOutlinePath(mf, "Stroke", 20, 100)
	if p == nil || p.NumVerbs() == 0 {
		t.Fatal("TextOutlinePath(MultiFace) still nil/empty after fix")
	}

	// Live ink probe: StrokeString on a clean context must leave strokes.
	dc := render.NewContext(320, 240)
	defer dc.Close()
	dc.SetFont(mf)
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.StrokeString("Stroke", 20, 100)
	img := dc.Image()
	ink := 0
	for y := 60; y < 110; y++ {
		for x := 15; x < 120; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r > 60 {
				ink++
			}
		}
	}
	if ink < 30 {
		t.Fatalf("StrokeString(MultiFace) still blank: ink=%d", ink)
	}
}

// The single-source path must stay intact: same ink expectations on a
// single FontSource face (the pre-fix working path).
func TestStrokeString_SingleFaceInk(t *testing.T) {
	src, err := text.NewFontSourceFromFile("text/testdata/goregular.ttf")
	if err != nil {
		t.Skipf("no font: %v", err)
	}
	defer func() { _ = src.Close() }()
	face := src.Face(24)

	p := render.TextOutlinePath(face, "Stroke", 20, 100)
	if p == nil || p.NumVerbs() == 0 {
		t.Fatal("TextOutlinePath(single) nil/empty — regression on working path")
	}

	dc := render.NewContext(320, 240)
	defer dc.Close()
	dc.SetFont(face)
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.StrokeString("Stroke", 20, 100)
	img := dc.Image()
	ink := 0
	for y := 60; y < 110; y++ {
		for x := 15; x < 120; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r > 60 {
				ink++
			}
		}
	}
	if ink < 30 {
		t.Fatalf("StrokeString(single) blank: ink=%d", ink)
	}
}

// Mixed-script text on MultiFace: Latin + CJK outlines both extract.
func TestStrokeString_MultiFaceMixedScript(t *testing.T) {
	text.SetSystemFontPaths(text.FontRoleCJK, "text/testdata/NotoSansSC-VF.otf")
	mf, _, err := text.LoadMultiFace(24, text.FontRoleUI, text.FontRoleCJK)
	if err != nil {
		t.Skipf("no fonts: %v", err)
	}
	p := render.TextOutlinePath(mf, "A中B", 20, 100)
	if p == nil || p.NumVerbs() == 0 {
		t.Fatal("mixed-script outline nil/empty")
	}
	// At least both scripts contribute contours: verb count > single glyph.
	if p.NumVerbs() < 8 {
		t.Fatalf("mixed-script outline suspiciously small: verbs=%d", p.NumVerbs())
	}
}
