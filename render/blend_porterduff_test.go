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
	"testing"
)

// B.02 Porter-Duff fixed-function modes on the software compositor
// (compositeAdvanced). Before this coverage existed, SrcIn / DstIn / SrcOut /
// DstOver / DstAtop silently fell through to the advanced-blend default and
// produced SourceOver-ish pixels. Each case asserts the exact formula on a
// solid destination; the same expectations hold on GPU (blend factors), so the
// tests run in both render modes (rounding tolerance ±2).
func TestPorterDuffModes_SolidFill(t *testing.T) {
	const size = 64

	dstInCenter := func(t *testing.T) (uint32, uint32, uint32, uint32) {
		t.Helper()
		dc := NewContext(size, size)
		defer func() { _ = dc.Close() }()
		dc.SetRGBA(1, 0, 0, 1)
		dc.DrawRectangle(0, 0, size, size)
		if err := dc.Fill(); err != nil {
			t.Fatalf("base fill: %v", err)
		}
		dc.SetBlendMode(BlendDestinationIn)
		dc.SetRGBA(1, 1, 1, 0.5) // color ignored by DstIn; alpha scales dst
		dc.DrawRectangle(0, 0, size, size)
		if err := dc.Fill(); err != nil {
			t.Fatalf("dstin fill: %v", err)
		}
		return pixelRGBA(dc.Image(), size/2, size/2)
	}

	// DestinationIn: out = dst * srcA = red * 0.5 → (128, 0, 0, 128).
	r, g, b, a := dstInCenter(t)
	if !near8(r, 128) || g > 2 || b > 2 || !near8(a, 128) {
		t.Errorf("DestinationIn: got (%d,%d,%d,%d), want ~(128,0,0,128)", r>>8, g>>8, b>>8, a>>8)
	}

	// SourceOut: out = src * (1 - dstA) = 0 over an opaque destination → cleared.
	dc := NewContext(size, size)
	defer func() { _ = dc.Close() }()
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, size, size)
	_ = dc.Fill()
	dc.SetBlendMode(BlendSourceOut)
	dc.SetRGBA(1, 1, 1, 1)
	dc.DrawRectangle(0, 0, size, size)
	_ = dc.Fill()
	r, g, b, a = pixelRGBA(dc.Image(), size/2, size/2)
	if a != 0 || r>>8 > 2 || g>>8 > 2 || b>>8 > 2 {
		t.Errorf("SourceOut over opaque dst: got (%d,%d,%d,%d), want cleared", r>>8, g>>8, b>>8, a>>8)
	}

	// DestinationAtop: out = D*Sa + S*(1-Da). A semi-transparent destination
	// distinguishes it from DestinationIn (D*Sa): dst red@0.5, src white@0.5
	// → (0.5, 0.25, 0.25, 0.5) ≈ (128, 64, 64, 128).
	dc2 := NewContext(size, size)
	defer func() { _ = dc2.Close() }()
	dc2.SetRGBA(1, 0, 0, 0.5)
	dc2.DrawRectangle(0, 0, size, size)
	_ = dc2.Fill()
	dc2.SetBlendMode(BlendDestinationAtop)
	dc2.SetRGBA(1, 1, 1, 0.5)
	dc2.DrawRectangle(0, 0, size, size)
	_ = dc2.Fill()
	r, g, b, a = pixelRGBA(dc2.Image(), size/2, size/2)
	if !near8(r, 128) || !near8(g, 64) || !near8(b, 64) || !near8(a, 128) {
		t.Errorf("DestinationAtop: got (%d,%d,%d,%d), want ~(128,64,64,128)", r>>8, g>>8, b>>8, a>>8)
	}

	// DestinationOver: out = src*(1-dstA) + dst = dst for an opaque destination.
	dc3 := NewContext(size, size)
	defer func() { _ = dc3.Close() }()
	dc3.SetRGBA(1, 0, 0, 1)
	dc3.DrawRectangle(0, 0, size, size)
	_ = dc3.Fill()
	dc3.SetBlendMode(BlendDestinationOver)
	dc3.SetRGBA(0, 0, 1, 1)
	dc3.DrawRectangle(0, 0, size, size)
	_ = dc3.Fill()
	r, g, b, a = pixelRGBA(dc3.Image(), size/2, size/2)
	if !near8(r, 255) || g>>8 > 2 || b>>8 > 2 || !near8(a, 255) {
		t.Errorf("DestinationOver over opaque dst: got (%d,%d,%d,%d), want red intact",
			r>>8, g>>8, b>>8, a>>8)
	}
}

// SourceIn scales the source by the destination alpha: over an opaque
// destination the source passes through unchanged.
func TestPorterDuffModes_SourceIn(t *testing.T) {
	const size = 64
	dc := NewContext(size, size)
	defer func() { _ = dc.Close() }()
	dc.SetRGBA(0, 0, 1, 1) // opaque blue destination
	dc.DrawRectangle(0, 0, size, size)
	if err := dc.Fill(); err != nil {
		t.Fatalf("base fill: %v", err)
	}
	dc.SetBlendMode(BlendSourceIn)
	dc.SetRGBA(1, 1, 1, 0.5) // src = premul white 0.5
	dc.DrawRectangle(0, 0, size, size)
	if err := dc.Fill(); err != nil {
		t.Fatalf("srcin fill: %v", err)
	}
	r, g, b, a := pixelRGBA(dc.Image(), size/2, size/2)
	if !near8(r, 128) || !near8(g, 128) || !near8(b, 128) || !near8(a, 128) {
		t.Errorf("SourceIn over opaque dst: got (%d,%d,%d,%d), want ~(128,128,128,128)",
			r>>8, g>>8, b>>8, a>>8)
	}
}

// near8 reports whether a 16-bit channel value matches an 8-bit expectation
// within +/-2 (CPU and GPU rounding differ by one step).
func near8(v uint32, want uint8) bool {
	got := int(v >> 8)
	diff := got - int(want)
	return diff >= -2 && diff <= 2
}
