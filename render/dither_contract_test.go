//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"bytes"
	"image"
	"testing"
)

// A12 pin: SetDither/Dither keep their documented readback contract.
//
// The current implementation is a pixmap postprocess applied by Image() /
// FlushGPU() after GPU resolve. It intentionally touches the whole pixmap;
// see RENDER_FEATURE_COVERAGE §6 for the three known limits (whole-pixmap
// scope, cumulative repeated readbacks, and no effect on the
// FlushGPUWithView/PresentWith window-present path). These tests pin the
// readback contract: default off, explicit toggle, deterministic Bayer 4x4,
// gradient diversity, and alpha preservation.

func ditherGradientContext(w, h int, dither bool) *Context {
	dc := NewContext(w, h)
	grad := NewLinearGradientBrush(0, 0, float64(w-1), float64(h-1)).
		AddColorStop(0, RGB(0.18, 0.20, 0.24)).
		AddColorStop(1, RGB(0.82, 0.84, 0.88))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	if dither {
		dc.SetDither(true)
	}
	return dc
}

func ditherImageBytes(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			out = append(out, byte(r>>8), byte(g>>8), byte(bl>>8), byte(a>>8))
		}
	}
	return out
}

func ditherUnique(img image.Image) int {
	seen := map[[3]uint8]struct{}{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			seen[[3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)}] = struct{}{}
		}
	}
	return len(seen)
}

func ditherAlphaUniform(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a>>8 != 255 {
				return false
			}
		}
	}
	return true
}

func TestDitherContractNilAndToggle(t *testing.T) {
	var nilCtx *Context
	nilCtx.SetDither(true) // must not panic
	if nilCtx.Dither() {
		t.Fatal("nil Context Dither() = true, want false")
	}

	dc := NewContext(8, 8)
	defer dc.Close()
	if dc.Dither() {
		t.Fatal("Dither() default = true, want false")
	}
	dc.SetDither(true)
	if !dc.Dither() {
		t.Fatal("Dither() = false after SetDither(true)")
	}
	dc.SetDither(false)
	if dc.Dither() {
		t.Fatal("Dither() = true after SetDither(false)")
	}
}

func TestDitherContractGradientReadback(t *testing.T) {
	const w, h = 64, 16
	off := ditherGradientContext(w, h, false)
	on := ditherGradientContext(w, h, true)
	defer off.Close()
	defer on.Close()

	imgOff := off.Image()
	imgOn := on.Image()
	if bytes.Equal(ditherImageBytes(imgOff), ditherImageBytes(imgOn)) {
		t.Fatal("dither on/off produced identical gradient pixels")
	}

	uOff, uOn := ditherUnique(imgOff), ditherUnique(imgOn)
	if uOn <= uOff {
		t.Fatalf("dither did not increase gradient color diversity: off=%d on=%d", uOff, uOn)
	}
	if !ditherAlphaUniform(imgOn) {
		t.Fatal("dither changed alpha; only RGB low bits must move")
	}
}

func TestDitherContractDeterministic(t *testing.T) {
	const w, h = 40, 12
	a := ditherGradientContext(w, h, true)
	b := ditherGradientContext(w, h, true)
	defer a.Close()
	defer b.Close()

	ia, ib := a.Image(), b.Image()
	if !bytes.Equal(ditherImageBytes(ia), ditherImageBytes(ib)) {
		t.Fatal("same gradient + same dither flag produced different pixels")
	}
}

func TestDitherContractBayer4Period(t *testing.T) {
	// Current postprocess contract: a flat fill under dither repeats its RGB
	// pattern every 4 px in both axes. This is the ordered Bayer 4x4 signature
	// the window's card D exposes at 1:1.
	dc := NewContext(16, 16)
	defer dc.Close()
	dc.SetRGBA(0.5, 0.5, 0.5, 1)
	dc.DrawRectangle(0, 0, 16, 16)
	_ = dc.Fill()
	dc.SetDither(true)
	img := dc.Image()

	// Stay off the AA edges: the GPU solid cover's outermost row/column can
	// carry fractional coverage before the dither postprocess.
	for y := 4; y < 12; y++ {
		for x := 4; x < 8; x++ {
			r0, g0, b0, _ := img.At(x, y).RGBA()
			r1, g1, b1, _ := img.At(x+4, y).RGBA()
			if r0 != r1 || g0 != g1 || b0 != b1 {
				t.Fatalf("x period != 4 at (%d,%d): %v vs (%d,%d): %v", x, y,
					[3]uint32{r0 >> 8, g0 >> 8, b0 >> 8}, x+4, y,
					[3]uint32{r1 >> 8, g1 >> 8, b1 >> 8})
			}
		}
	}
	for y := 4; y < 8; y++ {
		for x := 4; x < 12; x++ {
			r0, g0, b0, _ := img.At(x, y).RGBA()
			r1, g1, b1, _ := img.At(x, y+4).RGBA()
			if r0 != r1 || g0 != g1 || b0 != b1 {
				t.Fatalf("y period != 4 at (%d,%d): %v vs (%d,%d): %v", x, y,
					[3]uint32{r0 >> 8, g0 >> 8, b0 >> 8}, x, y+4,
					[3]uint32{r1 >> 8, g1 >> 8, b1 >> 8})
			}
		}
	}
}
