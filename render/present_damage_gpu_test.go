//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build !nogpu

package render_test

// Dirty-region P2 pixel evidence for the multi-rect scissor present path
// (ADR-028): per-rect scissors must apply new content inside each damage
// rect while the untouched gap between rects keeps the previous frame.

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestPresentDamage_MultiRectScissorPixels is the dirty-region P2 pixel-level
// proof of the multi-rect scissor path (PresentFrameDamageRects, ADR-028):
// two distant damaged regions each get their own scissor — inside every rect
// the new content lands; the untouched gap BETWEEN the rects must keep the
// previous frame's pixels (a union-bbox scissor would have cleared it).
func TestPresentDamage_MultiRectScissorPixels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 200, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	// Frame 1: the whole surface is base blue.
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.15, 0.25, 0.75)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("no offscreen")
	}
	defer rel()
	if err := dc.PresentFrameDamageRects(view, uint32(w), uint32(h), []image.Rectangle{image.Rect(0, 0, w, h)}, nil); err != nil {
		t.Fatalf("base present: %v", err)
	}

	// Frame 2: red content inside rect A (top-left), green content inside
	// rect B (bottom-right); the middle band is touched by neither.
	rectA := image.Rect(10, 10, 60, 40)
	rectB := image.Rect(120, 70, 180, 110)
	dc.SetRGB(0.85, 0.15, 0.15)
	dc.DrawRectangle(float64(rectA.Min.X), float64(rectA.Min.Y), float64(rectA.Dx()), float64(rectA.Dy()))
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.75, 0.25)
	dc.DrawRectangle(float64(rectB.Min.X), float64(rectB.Min.Y), float64(rectB.Dx()), float64(rectB.Dy()))
	_ = dc.Fill()
	if err := dc.PresentFrameDamageRects(view, uint32(w), uint32(h), []image.Rectangle{rectA, rectB}, nil); err != nil {
		t.Fatalf("multi present: %v", err)
	}

	sample := func(x, y int) (uint8, uint8, uint8) {
		r, g, b, _ := p1Sample(dc, x, y)
		return r, g, b
	}
	near := func(got, want, tol uint8) bool { return got <= want+tol && got+tol >= want }

	// Inside rect A: the new red content landed.
	if r, g, b := sample(30, 25); !near(r, 217, 30) || g > 90 || b > 90 {
		t.Fatalf("rect A center rgba=(%d,%d,%d), want the new red content", r, g, b)
	}
	// Inside rect B: the new green content landed.
	if r, g, b := sample(150, 90); r > 90 || !near(g, 191, 30) || b > 90 {
		t.Fatalf("rect B center rgba=(%d,%d,%d), want the new green content", r, g, b)
	}
	// The gap BETWEEN the rects: previous frame's blue must survive — the
	// pixel-level teeth distinguishing per-rect scissors from a union bbox.
	if r, g, b := sample(90, 55); r > 90 || g < 40 || b < 150 {
		t.Fatalf("gap between rects rgba=(%d,%d,%d), want preserved base blue (union scissor would have cleared it)", r, g, b)
	}
	// Outside both rects (far corner): base blue preserved as well.
	if r, g, b := sample(195, 8); r > 90 || b < 150 {
		t.Fatalf("outside corner rgba=(%d,%d,%d), want preserved base blue", r, g, b)
	}
}
