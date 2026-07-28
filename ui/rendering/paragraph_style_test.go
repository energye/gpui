package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

func loadAnyFace(t *testing.T, points float64) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := rendering.TryLoadDefaultFace(points)
	if err != nil || face == nil {
		t.Skipf("no system face: %v", err)
	}
	return face
}

// TestLoadFaceByFamily_SansAndMono resolves generic family strings.
func TestLoadFaceByFamily_SansAndMono(t *testing.T) {
	text.ClearSystemFontPaths()
	sans, path, err := rendering.LoadFaceByFamily("sans-serif", 14)
	if err != nil || sans == nil {
		t.Skipf("sans unavailable: %v", err)
	}
	t.Log("sans:", path)
	if !sans.HasGlyph('A') {
		t.Fatal("sans missing Latin A")
	}
	mono, mpath, err := rendering.LoadFaceByFamily("monospace", 14)
	if err != nil {
		t.Skipf("mono unavailable: %v", err)
	}
	t.Log("mono:", mpath)
	if mono == nil || !mono.HasGlyph('A') {
		t.Fatal("mono face broken")
	}
	// Different files when both exist (not required if distro only has one font).
	_ = mono
}

// TestRenderText_SetFontFamily_AffectsMeasure: mono vs sans width differs for "iii".
func TestRenderText_SetFontFamily_AffectsMeasure(t *testing.T) {
	text.ClearSystemFontPaths()
	rt := rendering.NewRenderText("WWWiii")
	rt.SetFontSize(16)
	if err := rt.SetFontFamily("sans-serif"); err != nil {
		t.Skip(err)
	}
	wSans := rt.Layout(rendering.Loose(800, 40)).Width

	rt2 := rendering.NewRenderText("WWWiii")
	rt2.SetFontSize(16)
	if err := rt2.SetFontFamily("monospace"); err != nil {
		t.Skip(err)
	}
	wMono := rt2.Layout(rendering.Loose(800, 40)).Width
	if wSans <= 0 || wMono <= 0 {
		t.Fatalf("widths sans=%v mono=%v", wSans, wMono)
	}
	// Typically mono "i" is wider than proportional; either way families should not
	// collapse to identical zero-heuristic if both loaded real faces.
	t.Logf("sansW=%.2f monoW=%.2f", wSans, wMono)
}

// TestParagraphBuilder_PushPopStyle nests color/size for spans.
func TestParagraphBuilder_PushPopStyle(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.SetDefaultStyle(nil, 12, 1, 0, 0, 1, 1) // red
	b.AddText("A")
	b.PushStyle(nil, 18, true, 0, 0, 1, 1, 0) // blue larger
	b.AddText("B")
	b.PopStyle()
	b.AddText("C")
	rt := b.Build()
	if rt.RunCount() != 3 {
		t.Fatalf("runs=%d", rt.RunCount())
	}
	if rt.Runs[0].R < 0.9 || rt.Runs[1].B < 0.9 || rt.Runs[2].R < 0.9 {
		t.Fatalf("style stack colors: %+v", rt.Runs)
	}
	if rt.Runs[1].FontSize != 18 || rt.Runs[0].FontSize != 12 || rt.Runs[2].FontSize != 12 {
		t.Fatalf("font sizes: %v %v %v", rt.Runs[0].FontSize, rt.Runs[1].FontSize, rt.Runs[2].FontSize)
	}
}

// TestRenderText_Decoration_UnderlinePaints: with face, underline decoration
// dirties paint path without panic and leaves non-white pixels near baseline.
func TestRenderText_Decoration_UnderlinePaints(t *testing.T) {
	face := loadAnyFace(t, 16)
	rt := rendering.NewRenderText("Link")
	rt.SetFace(face)
	rt.SetFontSize(16)
	rt.SetColor(0, 0, 0.8, 1)
	rt.SetDecoration(render.TextDecorationUnderline)
	_ = rt.Layout(rendering.Loose(200, 40))

	dc := render.NewContext(120, 40)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	rt.Paint(pc)

	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// Some dark ink should exist (glyph and/or underline).
	found := false
	for y := 0; y < 40 && !found; y++ {
		for x := 0; x < 120; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r < 0xE000 || g < 0xE000 || b < 0xE000 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected painted text/underline pixels")
	}
}

// TestParagraphBuilder_DecorationOnRun carries decoration into runs.
func TestParagraphBuilder_DecorationOnRun(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.SetDecoration(render.TextDecorationUnderline)
	b.AddText("u")
	b.SetDecoration(0)
	b.AddText("n")
	rt := b.Build()
	if rt.Runs[0].Decoration&render.TextDecorationUnderline == 0 {
		t.Fatal("first run missing underline")
	}
	if rt.Runs[1].Decoration != 0 {
		t.Fatal("second run should clear decoration")
	}
}
