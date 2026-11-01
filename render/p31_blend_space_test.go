//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"math"
	"testing"

	intImage "github.com/energye/gpui/render/internal/image"
)

// P3.1 capability pins for BlendSpace. Reference math: linear-light blending
// of a 25%-alpha black over opaque white re-encodes to ~225 (linear 0.75 ->
// sRGB 0.8808), while the sRGB-domain blend yields 191 — the midtone gap this
// capability exists for. CPU precision envelope: u8 domain end to end,
// ±2/255 against a float reference (evidence/P3.1/assessment.md §2).

// linearBlendReferenceF32 computes straight-alpha source-over in float linear
// space; inputs are normalized sRGB-encoded values.
func linearBlendReferenceF32(srcSRGB, dstSRGB, srcAlpha float64) float64 {
	decode := func(c float64) float64 {
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	encode := func(c float64) float64 {
		if c <= 0.0031308 {
			return c * 12.92
		}
		return 1.055*math.Pow(c, 1.0/2.4) - 0.055
	}
	co := (1-srcAlpha)*decode(dstSRGB) + srcAlpha*decode(srcSRGB)
	return encode(co)
}

// TestBlendSpaceDefaultIsSRGB pins that zero-valued paints keep the historical
// sRGB-encoded output (P2.6 baseline compatibility) and that Clone copies the
// field.
func TestBlendSpaceDefaultIsSRGB(t *testing.T) {
	p := NewPaint()
	if p.BlendSpace != BlendSpaceSRGB {
		t.Fatalf("zero Paint.BlendSpace = %d, want BlendSpaceSRGB(0)", p.BlendSpace)
	}
	if p.Clone().BlendSpace != BlendSpaceSRGB {
		t.Fatalf("Clone lost BlendSpace default")
	}
	p.BlendSpace = BlendSpaceLinear
	if p.Clone().BlendSpace != BlendSpaceLinear {
		t.Fatalf("Clone did not copy BlendSpace=Linear")
	}

	c := NewContext(8, 8)
	t.Cleanup(func() { _ = c.Close() })
	if got := c.BlendSpace(); got != BlendSpaceSRGB {
		t.Fatalf("fresh Context.BlendSpace() = %d, want SRGB", got)
	}
	c.SetBlendSpace(BlendSpaceLinear)
	if c.BlendSpace() != BlendSpaceLinear {
		t.Fatalf("SetBlendSpace/BlendSpace roundtrip failed")
	}
	c.SetBlendSpace(BlendSpaceSRGB)
	if c.BlendSpace() != BlendSpaceSRGB {
		t.Fatalf("SetBlendSpace(SRGB) roundtrip failed")
	}
}

// TestSoftwareFillLinearModeSet pins the P3.1 mode-set contract on the
// software fill path: a linear request on Normal (fixed-function family)
// degrades to the sRGB domain (191 — identical on every backend), while an
// advanced mode (Difference) blends in linear light (~168 vs 76 sRGB-domain).
func TestSoftwareFillLinearModeSet(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	fill := func(t *testing.T, mode BlendMode, space BlendSpace) uint8 {
		t.Helper()
		dc := NewContext(16, 16)
		t.Cleanup(func() { _ = dc.Close() })
		dc.ClearWithColor(RGBA{R: 0.8, G: 0.8, B: 0.8, A: 1}) // enc 204
		dc.SetRGB(0.5, 0.5, 0.5)                              // enc 128
		dc.SetBlendMode(mode)
		dc.SetBlendSpace(space)
		dc.DrawRectangle(0, 0, 16, 16)
		if err := dc.Fill(); err != nil {
			t.Fatalf("fill: %v", err)
		}
		pm := dc.PixmapForTest()
		data := pm.Data()
		return data[(8*pm.Width()+8)*4]
	}

	// Opaque 0.5-gray over 0.8-gray: plain coverage in either domain.
	if got := fill(t, BlendNormal, BlendSpaceLinear); absDiffF(int(got), 128) > 1 {
		t.Fatalf("Normal+linear must degrade to the sRGB-domain result, got R=%d want 128", got)
	}
	if got := fill(t, BlendNormal, BlendSpaceSRGB); absDiffF(int(got), 128) > 1 {
		t.Fatalf("Normal+sRGB default changed, got R=%d want 128", got)
	}
	if got := fill(t, BlendDifference, BlendSpaceSRGB); absDiffF(int(got), 76) > 2 {
		t.Fatalf("Difference sRGB-domain got R=%d want 76±2", got)
	}
	if got := fill(t, BlendDifference, BlendSpaceLinear); absDiffF(int(got), 168) > 3 {
		t.Fatalf("Difference linear got R=%d want 168±3 (float ref 167.6; sRGB-domain is 76)", got)
	}
}

// TestImageDrawLinearDomain pins the straight-domain image path with the
// same mode-set contract: Normal degrades (191 both domains), an advanced
// mode (Multiply) blends in linear light (~224 vs 191).
func TestImageDrawLinearDomain(t *testing.T) {
	fillWhite := func() *intImage.ImageBuf {
		dst, _ := intImage.NewImageBuf(8, 8, intImage.FormatRGBA8)
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				_ = dst.SetRGBA(x, y, 255, 255, 255, 255)
			}
		}
		return dst
	}
	src, _ := intImage.NewImageBuf(8, 8, intImage.FormatRGBA8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			_ = src.SetRGBA(x, y, 0, 0, 0, 64) // ~25% alpha black
		}
	}
	draw := func(dst *intImage.ImageBuf, mode intImage.BlendMode, linear bool) uint8 {
		intImage.DrawImage(dst, src, intImage.DrawParams{
			DstRect:     intImage.Rect{X: 0, Y: 0, Width: 8, Height: 8},
			Interp:      intImage.InterpNearest,
			Opacity:     1.0,
			BlendMode:   mode,
			BlendLinear: linear,
		})
		r, _, _, _ := dst.GetRGBA(4, 4)
		return r
	}

	if got := draw(fillWhite(), intImage.BlendNormal, true); absDiffF(int(got), 191) > 1 {
		t.Fatalf("Normal+linear image draw must degrade, got R=%d want 191", got)
	}
	if got := draw(fillWhite(), intImage.BlendMultiply, true); absDiffF(int(got), 224) > 2 {
		t.Fatalf("Multiply+linear image draw got R=%d want 224±2 (float ref; sRGB-domain is 191)", got)
	}
	if got := draw(fillWhite(), intImage.BlendNormal, false); absDiffF(int(got), 191) > 1 {
		t.Fatalf("sRGB-domain Normal changed, got R=%d want 191", got)
	}
}

func absDiffF(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
