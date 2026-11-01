//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package filter

import (
	"testing"

	"github.com/energye/gpui/render"
	intBlend "github.com/energye/gpui/render/internal/blend"
)

// P3.4 batch-4 value pins for the ColorFilter family.

// TestLightingColorFilterMath pins the multiply-add per channel.
func TestLightingColorFilterMath(t *testing.T) {
	f := NewLightingColorFilter(render.RGBA{R: 0.5, G: 1, B: 2}, render.RGBA{R: 0.1, G: 0, B: 0, A: 0})
	r, g, b, a := f.Filter(1, 0.5, 0, 1)
	if r < 0.59 || r > 0.61 {
		t.Fatalf("r=%f want 0.6 (0.5*1+0.1)", r)
	}
	if g < 0.49 || g > 0.51 {
		t.Fatalf("g=%f want 0.5", g)
	}
	if b != 0 {
		t.Fatalf("b=%f want 0 (2*0+0)", b)
	}
	if a != 1 {
		t.Fatalf("a=%f want passthrough", a)
	}
}

// TestTableColorFilterLUT pins LUT lookup precision.
func TestTableColorFilterLUT(t *testing.T) {
	tab := make([]uint8, 256)
	for i := range tab {
		tab[i] = uint8(255 - i) // invert
	}
	f := NewTableColorFilter(tab, nil, nil, nil)
	r, g, b, a := f.Filter(0.5, 0.25, 0, 1)
	if r < 0.49 || r > 0.51 {
		t.Fatalf("r=%f want ~0.5 (inverted 0.5)", r)
	}
	if g < 0.24 || g > 0.26 {
		t.Fatalf("g=%f want ~0.25 (nil table passthrough)", g)
	}
	_ = b
	_ = a
}

// TestComposeColorFilterOrder pins outer∘inner evaluation order.
func TestComposeColorFilterOrder(t *testing.T) {
	addA := NewLightingColorFilter(render.RGBA{R: 1, G: 1, B: 1}, render.RGBA{R: 0.1, A: 1}) // +0.1 R with identity multiplier
	halve := NewTableColorFilter(scaleHalf(), nil, nil, nil)
	// outer(halve(inner(+0.1))): 0.5 → 0.6 → 0.3
	p := ComposeColorFilter(halve, addA)
	r, _, _, _ := p.Filter(0.5, 0, 0, 1)
	if r < 0.29 || r > 0.31 {
		t.Fatalf("r=%f want ~0.3", r)
	}
	// reversed order: outer(+0.1(inner(halve))): 0.5 → 0.25 → 0.35
	p2 := ComposeColorFilter(addA, halve)
	r2, _, _, _ := p2.Filter(0.5, 0, 0, 1)
	if r2 < 0.34 || r2 > 0.36 {
		t.Fatalf("r2=%f want ~0.35", r2)
	}
}

// TestBlendColorFilterPins pins Blend with SourceOver: 50% red over opaque
// green yields the known straight mix.
func TestBlendColorFilterPins(t *testing.T) {
	f := NewBlendColorFilter(render.RGBA{R: 1, A: 0.5}, intBlend.BlendSourceOver)
	r, g, b, a := f.Filter(0, 1, 0, 1)
	if d2(r, 0.5) > 0.02 || d2(g, 0.5) > 0.02 || d2(a, 1) > 0.02 {
		t.Fatalf("blend=(%f,%f,%f,%f), want ~(0.5,0.5,0,1)", r, g, b, a)
	}
}

func scaleHalf() []uint8 {
	tab := make([]uint8, 256)
	for i := range tab {
		tab[i] = uint8(i / 2)
	}
	return tab
}

func d2(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}
