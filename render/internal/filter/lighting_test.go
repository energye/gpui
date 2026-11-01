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
)

// P3.4 batch-2 value pins: point-diffuse lighting geometry (facing the light
// brightens, facing away darkens; light position matters) and tile periodicity.

// TestPointDiffuseLightGeometry pins the lighting field: a flat opaque
// surface lit from directly above the center is brightest there and falls
// off toward the edge; a light below the plane yields near-zero diffuse.
func TestPointDiffuseLightGeometry(t *testing.T) {
	src := newTestPixmap(t, 64, 64)
	src.Clear(render.RGBA{})
	fillRect(src, 0, 0, 64, 64, render.RGBA{R: 1, A: 1}) // flat opaque height=1

	// Light above the center (32,32,80).
	f := NewPointDiffuseFilter([3]float32{32, 32, 80}, render.RGBA{R: 1, G: 1, B: 1}, 1.0, 8.0)
	dst := newTestPixmap(t, 64, 64)
	dst.Clear(render.RGBA{})
	f.Apply(src, dst, fullRect(dst))

	center := pxAt(dst, 32, 32)
	corner := pxAt(dst, 4, 4)
	if center[0] <= corner[0] {
		t.Fatalf("center lum %d must exceed corner %d (falloff)", center[0], corner[0])
	}
	if center[0] == 0 {
		t.Fatal("center must receive light")
	}

	// A light far below the plane: N·L ≤ 0 almost everywhere → near black.
	fb := NewPointDiffuseFilter([3]float32{32, 32, -500}, render.RGBA{R: 1, G: 1, B: 1}, 1.0, 8.0)
	dst2 := newTestPixmap(t, 64, 64)
	dst2.Clear(render.RGBA{})
	fb.Apply(src, dst2, fullRect(dst2))
	if px := pxAt(dst2, 32, 32); px[0] > 30 {
		t.Fatalf("below-plane light center=%d, want near zero", px[0])
	}
}

// TestTilePeriodicity pins tiling: output(x+tw) == output(x) for the tile
// width, and the tile rect itself is reproduced at the origin.
func TestTilePeriodicity(t *testing.T) {
	src := newTestPixmap(t, 32, 32)
	src.Clear(render.RGBA{})
	fillRect(src, 0, 0, 4, 4, render.RGBA{R: 1, A: 1}) // 4x4 white block at origin
	dst := newTestPixmap(t, 32, 32)
	dst.Clear(render.RGBA{})
	NewTileFilter([4]float32{0, 0, 8, 8}).Apply(src, dst, fullRect(dst))

	// The 4x4 white block repeats at multiples of 8.
	if px := pxAt(dst, 1, 1); px[0] != 255 {
		t.Fatalf("tile origin block=%v, want white", px)
	}
	if px := pxAt(dst, 9, 1); px[0] != 255 {
		t.Fatalf("tile repeat at x+8=%v, want white", px)
	}
	if px := pxAt(dst, 5, 1); px[0] != 0 {
		t.Fatalf("tile gap=%v, want transparent", px)
	}
}
