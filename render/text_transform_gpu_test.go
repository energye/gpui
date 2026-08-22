// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package render

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestNeedsOutlineTransform verifies the transform quality gate: rotation,
// shear, and non-uniform scale must route text to vector outlines, while
// identity, translation, and uniform scale keep bitmap/SDF pipelines.
func TestNeedsOutlineTransform(t *testing.T) {
	dc := NewContext(64, 64)
	cases := []struct {
		name string
		fn   func()
		want bool
	}{
		{"identity", func() {}, false},
		{"translate", func() { dc.Translate(10, 20) }, false},
		{"uniform_scale2", func() { dc.Scale(2, 2) }, false},
		{"uniform_scale_down", func() { dc.Scale(0.7, 0.7) }, false},
		{"nonuniform_scale31", func() { dc.Scale(3, 1) }, true},
		{"rotate30", func() { dc.Rotate(math.Pi / 6) }, true},
		{"rotate45", func() { dc.Rotate(math.Pi / 4) }, true},
		{"shear", func() { dc.Shear(-0.3, 0) }, true},
		{"translate_then_rotate", func() { dc.Translate(5, 5); dc.Rotate(math.Pi / 8) }, true},
		{"uniform_scale_then_rotate", func() { dc.Scale(2, 2); dc.Rotate(math.Pi / 8) }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dc.Identity()
			c.fn()
			if got := dc.needsOutlineTransform(); got != c.want {
				t.Errorf("needsOutlineTransform = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTextTransformAutoRoutesOutlines verifies that transformed text
// (rotation / shear / non-uniform scale) in TextModeAuto is routed to the
// same vector-outline path as TextModeVector — the fix for the MSDF
// "blur + ghosting" on transformed text. Uniform/identity transforms are not
// routed (they keep the bitmap/SDF pipelines).
func TestTextTransformAutoRoutesOutlines(t *testing.T) {
	fontPath := findSystemFont(t)
	if fontPath == "" {
		t.Skip("No system font available")
	}
	source, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatalf("Failed to load font: %v", err)
	}
	defer func() { _ = source.Close() }()

	draw := func(dc *Context) {
		dc.ClearWithColor(White)
		dc.SetFont(source.Face(24))
		dc.SetRGB(0, 0, 0)
		dc.Push()
		dc.Translate(60, 60)
		dc.Rotate(math.Pi / 6)
		dc.DrawString("Hello gg!", 0, 0)
		dc.Pop()
	}

	imgAuto := renderToImage(t, draw, TextModeAuto)
	imgVector := renderToImage(t, draw, TextModeVector)
	imgMSDF := renderToImage(t, draw, TextModeMSDF)

	autoVsVector := countDiffPixels(imgAuto, imgVector)
	autoVsMSDF := countDiffPixels(imgAuto, imgMSDF)
	t.Logf("Auto vs Vector diff: %d px | Auto vs MSDF diff: %d px", autoVsVector, autoVsMSDF)

	// When the GPU text path is unavailable, all modes fall back to the same
	// CPU rasterization and are pixel-identical — the routing assertions only
	// apply when the MSDF path actually rendered on the GPU (i.e. MSDF output
	// differs from the vector output).
	if autoVsMSDF == 0 && autoVsVector == 0 {
		t.Skip("GPU text unavailable: all modes rendered via CPU fallback")
	}

	// Auto must equal the vector-outline path for rotated text…
	if autoVsVector > 8 {
		t.Errorf("TextModeAuto rotated text differs from TextModeVector by %d px (want ≤8): auto must route transforms to outlines", autoVsVector)
	}

	// …and must differ from the raw MSDF path (which blurs under rotation).
	if autoVsMSDF < 64 {
		t.Errorf("TextModeAuto rotated text equals raw MSDF output (%d px diff): outline routing not active", autoVsMSDF)
	}
}

// renderToImage renders the scene with the given text mode and returns pixels.
func renderToImage(t *testing.T, draw func(dc *Context), mode TextMode) *image.RGBA {
	t.Helper()
	dc := NewContext(160, 120)
	dc.SetTextMode(mode)
	draw(dc)
	img := dc.Image()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		rgba = cloneToRGBA(img)
	}
	return rgba
}

// cloneToRGBA converts any image to *image.RGBA for uniform diffing.
func cloneToRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}

// countDiffPixels counts pixels whose RGB differs by more than 12 per channel.
func countDiffPixels(a, b *image.RGBA) int {
	if a == nil || b == nil {
		return int(^uint(0) >> 1) //nolint:gosec // sentinel: treat nil as fully different
	}
	ba, bb := a.Bounds(), b.Bounds()
	if ba != bb {
		return int(^uint(0) >> 1) //nolint:gosec // different sizes: fully different
	}
	count := 0
	for y := ba.Min.Y; y < ba.Max.Y; y++ {
		for x := ba.Min.X; x < ba.Max.X; x++ {
			ca := a.RGBAAt(x, y)
			cb := b.RGBAAt(x, y)
			d := abs8(ca.R, cb.R) + abs8(ca.G, cb.G) + abs8(ca.B, cb.B)
			if d > 12 {
				count++
			}
		}
	}
	return count
}

func abs8(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}
