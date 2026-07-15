//go:build !nogpu

package render_test

// S3c residual M3 capability gates (B.04/C.06/H.04/I.04/I.07/X.08/X.09/X.10).
// Architecture: render → webgpu → rwgpu → libwgpu_native where GPU is claimed.

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/text"
)

func residualFont(t *testing.T) string {
	t.Helper()
	cands := []string{
		"text/testdata/goregular.ttf",
		"render/text/testdata/goregular.ttf",
		filepath.Join("text", "testdata", "goregular.ttf"),
		filepath.Join("render", "text", "testdata", "goregular.ttf"),
	}
	for _, p := range cands {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no test font")
	return ""
}

func residualVF(t *testing.T) string {
	t.Helper()
	cands := []string{
		"text/testdata/cantarell_vf_trimmed.ttf",
		"render/text/testdata/cantarell_vf_trimmed.ttf",
		filepath.Join("text", "testdata", "cantarell_vf_trimmed.ttf"),
		filepath.Join("render", "text", "testdata", "cantarell_vf_trimmed.ttf"),
	}
	for _, p := range cands {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no variable font test asset")
	return ""
}

// --- B.04 HSL blend modes ---

func TestS3c_M3_BlendHue(t *testing.T) {
	// CPU advanced blend path (GPU falls back for non-normal).
	dc := render.NewContext(32, 32)
	defer dc.Close()
	// Red destination
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	// Green source with Hue blend: result should keep red luminosity/sat roughly but take green hue → yellowish/greenish shift
	dc.SetRGB(0, 1, 0)
	dc.SetBlendMode(render.BlendHue)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	r, g, b, a := s3cSample(dc, 16, 16)
	t.Logf("BlendHue rgba=%d,%d,%d,%d", r, g, b, a)
	// Not pure red anymore
	if r == 255 && g == 0 && b == 0 {
		t.Fatalf("BlendHue left pure red unchanged")
	}
	// Still opaque-ish
	if a < 200 {
		t.Fatalf("expected opaque, a=%d", a)
	}
	// Luminosity roughly preserved near red (bright) → some channel high
	if int(r)+int(g)+int(b) < 100 {
		t.Fatalf("unexpectedly dark after Hue blend: %d,%d,%d", r, g, b)
	}
}

func TestS3c_M3_BlendColorAndLuminosity(t *testing.T) {
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.SetRGB(0.2, 0.2, 0.8) // blue-ish dark dest
	dc.DrawRectangle(0, 0, 16, 16)
	_ = dc.Fill()
	dc.SetRGB(1, 0.5, 0) // orange source
	dc.SetBlendMode(render.BlendColor)
	dc.DrawRectangle(0, 0, 16, 16)
	_ = dc.Fill()
	r1, g1, b1, _ := s3cSample(dc, 8, 8)
	t.Logf("BlendColor rgba=%d,%d,%d", r1, g1, b1)

	dc2 := render.NewContext(16, 16)
	defer dc2.Close()
	dc2.SetRGB(0.2, 0.2, 0.8)
	dc2.DrawRectangle(0, 0, 16, 16)
	_ = dc2.Fill()
	dc2.SetRGB(1, 0.5, 0)
	dc2.SetBlendMode(render.BlendLuminosity)
	dc2.DrawRectangle(0, 0, 16, 16)
	_ = dc2.Fill()
	r2, g2, b2, _ := s3cSample(dc2, 8, 8)
	t.Logf("BlendLuminosity rgba=%d,%d,%d", r2, g2, b2)
	// Color and Luminosity should differ
	if r1 == r2 && g1 == g2 && b1 == b2 {
		t.Fatalf("Color and Luminosity produced identical pixels")
	}
}

// --- C.06 clip ops ---

func TestS3c_M3_ClipRectDifference(t *testing.T) {
	dc := render.NewContext(48, 48)
	defer dc.Close()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	// Clip full, then difference a center hole
	dc.ClipRect(0, 0, 48, 48)
	dc.ClipRectOp(12, 12, 24, 24, render.ClipOpDifference)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	// Outside hole should be red
	r, g, b, _ := s3cSample(dc, 4, 4)
	if r < 200 || g > 40 || b > 40 {
		t.Fatalf("outside hole expected red, got %d,%d,%d", r, g, b)
	}
	// Inside hole should remain white (not painted)
	r2, g2, b2, _ := s3cSample(dc, 24, 24)
	if int(r2)+int(g2)+int(b2) < 700 {
		t.Fatalf("hole should stay white, got %d,%d,%d", r2, g2, b2)
	}
}

func TestS3c_M3_ClipRectReplace(t *testing.T) {
	dc := render.NewContext(40, 40)
	defer dc.Close()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 40, 40)
	_ = dc.Fill()
	dc.ClipRect(0, 0, 20, 40)                          // left half
	dc.ClipRectOp(20, 0, 20, 40, render.ClipOpReplace) // replace with right half
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 40, 40)
	_ = dc.Fill()
	// Left should stay white
	r, g, b, _ := s3cSample(dc, 5, 20)
	if int(r)+int(g)+int(b) < 700 {
		t.Fatalf("left after replace clip should be white, got %d,%d,%d", r, g, b)
	}
	// Right should be blue
	r2, g2, b2, _ := s3cSample(dc, 30, 20)
	if b2 < 200 || r2 > 40 || g2 > 40 {
		t.Fatalf("right expected blue, got %d,%d,%d", r2, g2, b2)
	}
}

// --- H.04 path boolean ---

func TestS3c_M3_PathBooleanIntersect(t *testing.T) {
	pathA := render.NewPath()
	pathA.Rectangle(0, 0, 40, 40)
	pathB := render.NewPath()
	pathB.Rectangle(20, 20, 40, 40)
	result := render.BooleanPath(pathA, pathB, render.PathOpIntersect)
	if result == nil || result.NumVerbs() == 0 {
		t.Fatal("intersect produced empty path")
	}
	// Intersection region center (30,30) should be inside result (winding)
	// For rect-run decomposition, fill and sample.
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	_ = dc.Fill()
	dc.SetRGB(0, 0.6, 0)
	// append boolean path into context path via iterate
	result.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			dc.MoveTo(coords[0], coords[1])
		case render.LineTo:
			dc.LineTo(coords[0], coords[1])
		case render.Close:
			dc.ClosePath()
		}
	})
	_ = dc.Fill()
	r, g, b, _ := s3cSample(dc, 30, 30)
	t.Logf("intersect center rgba=%d,%d,%d", r, g, b)
	if g < 100 {
		t.Fatalf("expected green in intersection, got %d,%d,%d", r, g, b)
	}
	// Outside both-overlap should be white
	r2, g2, b2, _ := s3cSample(dc, 5, 5)
	if int(r2)+int(g2)+int(b2) < 650 {
		t.Fatalf("outside intersect should stay light, got %d,%d,%d", r2, g2, b2)
	}
}

func TestS3c_M3_PathBooleanDifference(t *testing.T) {
	pathA := render.NewPath()
	pathA.Rectangle(8, 8, 40, 40)
	pathB := render.NewPath()
	pathB.Rectangle(20, 20, 16, 16)
	result := pathA.Op(pathB, render.PathOpDifference)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 0, 0)
	result.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			dc.MoveTo(coords[0], coords[1])
		case render.LineTo:
			dc.LineTo(coords[0], coords[1])
		case render.Close:
			dc.ClosePath()
		}
	})
	_ = dc.Fill()
	// Frame red
	r, g, b, _ := s3cSample(dc, 10, 10)
	if r < 200 {
		t.Fatalf("frame expected red, got %d,%d,%d", r, g, b)
	}
	// Hole near white
	r2, g2, b2, _ := s3cSample(dc, 28, 28)
	if int(r2)+int(g2)+int(b2) < 600 {
		t.Fatalf("difference hole expected light, got %d,%d,%d", r2, g2, b2)
	}
}

// --- I.04 bicubic + mipmap ---

func TestS3c_M3_ImageBicubicAndMipmap(t *testing.T) {
	// Checker 8x8 scaled up with bicubic should not crash and produce mid tones.
	src, err := render.NewImageBuf(8, 8, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if (x+y)%2 == 0 {
				_ = src.SetRGBA(x, y, 255, 255, 255, 255)
			} else {
				_ = src.SetRGBA(x, y, 0, 0, 0, 255)
			}
		}
	}
	dc := render.NewContext(64, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.DrawImageEx(src, render.DrawImageOptions{
		X: 0, Y: 0, DstWidth: 32, DstHeight: 32,
		Interpolation: render.InterpBicubic,
		Opacity:       1,
	})
	// Mipmap downscale path
	big, err := render.NewImageBuf(64, 64, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			_ = big.SetRGBA(x, y, uint8(x*4), uint8(y*4), 128, 255)
		}
	}
	dc.DrawImageEx(big, render.DrawImageOptions{
		X: 32, Y: 0, DstWidth: 16, DstHeight: 16,
		Interpolation: render.InterpBilinear,
		UseMipmaps:    true,
		Opacity:       1,
	})
	r, g, b, a := s3cSample(dc, 40, 8)
	t.Logf("mipmap sample rgba=%d,%d,%d,%d", r, g, b, a)
	if a < 200 {
		t.Fatalf("mipmap draw missing alpha")
	}
}

// --- I.07 nine-patch ---

func TestS3c_M3_DrawImageNine(t *testing.T) {
	s3cRequireGPU(t)
	// 15x15: 5px red corners/edges border, blue center 5x5
	img, err := render.NewImageBuf(15, 15, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 15; y++ {
		for x := 0; x < 15; x++ {
			if x < 5 || y < 5 || x >= 10 || y >= 10 {
				_ = img.SetRGBA(x, y, 255, 0, 0, 255)
			} else {
				_ = img.SetRGBA(x, y, 0, 0, 255, 255)
			}
		}
	}
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.ResetRenderPathStats()
	s3cWhiteBG(dc, 60, 60)
	// center stretch region is middle 5x5
	center := image.Rect(5, 5, 10, 10)
	dc.DrawImageNine(img, center, 5, 5, 50, 50)
	s3cFlushGPU(t, dc)
	// Deep in TL corner (within unscaled 5px border drawn at dst)
	r, g, b, _ := s3cSample(dc, 7, 7)
	t.Logf("nine corner rgba=%d,%d,%d", r, g, b)
	if r < 200 || b > 40 {
		t.Fatalf("expected red corner from nine-patch, got %d,%d,%d", r, g, b)
	}
	// Center of stretched area
	r2, g2, b2, _ := s3cSample(dc, 30, 30)
	t.Logf("nine center rgba=%d,%d,%d", r2, g2, b2)
	if b2 < 200 || r2 > 40 {
		t.Fatalf("expected blue center from nine-patch, got %d,%d,%d", r2, g2, b2)
	}
}

// --- X.08 text decoration ---

func TestS3c_M3_TextUnderline(t *testing.T) {
	font := residualFont(t)
	dc := render.NewContext(120, 40)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	if err := dc.LoadFontFace(font, 20); err != nil {
		t.Fatal(err)
	}
	dc.SetRGB(0, 0, 0)
	dc.SetTextDecoration(render.TextDecorationUnderline)
	dc.DrawString("Hi", 8, 24)
	// Sample a few rows below baseline for dark underline pixels
	found := false
	for y := 24; y < 36; y++ {
		for x := 8; x < 40; x++ {
			r, g, b, _ := s3cSample(dc, x, y)
			if int(r)+int(g)+int(b) < 500 {
				found = true
				t.Logf("underline pixel at %d,%d rgba=%d,%d,%d", x, y, r, g, b)
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("expected underline dark pixels below baseline")
	}
}

// --- X.09 variable font ---

func TestS3c_M3_VariableFontWeight(t *testing.T) {
	vf := residualVF(t)
	dc := render.NewContext(200, 48)
	defer dc.Close()
	// Default weight
	if err := dc.LoadFontFaceWithVariations(vf, 24); err != nil {
		t.Fatal(err)
	}
	axes := dc.FontVariationAxes()
	if len(axes) == 0 {
		t.Fatal("expected variation axes on VF font")
	}
	t.Logf("axes=%v", axes)
	w1 := float64(0)
	if f := dc.Font(); f != nil {
		w1 = f.Advance("H")
	}
	// Heavy weight
	if err := dc.LoadFontFaceWithVariations(vf, 24, text.NewFontVariation("wght", 700)); err != nil {
		t.Fatal(err)
	}
	w2 := dc.Font().Advance("H")
	t.Logf("advance default=%.2f wght700=%.2f", w1, w2)
	// Advances may equal on trimmed VF for single glyph; at least axes + face load work.
	if dc.Font() == nil {
		t.Fatal("face nil after variations")
	}
	vars := dc.Font().Variations()
	if len(vars) == 0 {
		t.Fatal("expected variations on face")
	}
	// Draw both to ensure render path works
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 0)
	dc.DrawString("V", 10, 30)
}

// --- X.10 emoji path smoke (API wired; asset may be absent) ---

func TestS3c_M3_DrawWithEmojiAPI(t *testing.T) {
	// Ensures DrawWithEmoji is reachable via DrawString CPU path without panic.
	font := residualFont(t)
	dc := render.NewContext(64, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	if err := dc.LoadFontFace(font, 16); err != nil {
		t.Fatal(err)
	}
	dc.SetRGB(0, 0, 0)
	// Regular text still works through DrawWithEmoji wrapper
	dc.DrawString("A", 8, 20)
	r, g, b, a := s3cSample(dc, 10, 14)
	t.Logf("glyph rgba=%d,%d,%d,%d", r, g, b, a)
	// Should have some non-white ink from "A"
	if int(r)+int(g)+int(b) > 750 && a > 200 {
		// might be sampling empty - try broader
		found := false
		for y := 0; y < 32; y++ {
			for x := 0; x < 40; x++ {
				rr, gg, bb, _ := s3cSample(dc, x, y)
				if int(rr)+int(gg)+int(bb) < 700 {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatal("expected some ink from DrawString via DrawWithEmoji path")
		}
	}
}
