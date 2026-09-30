//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package renderconv

import (
	"testing"

	"github.com/energye/gpui/engine/core"
)

// Boundary converts losslessly (moved verbatim from engine/core E01).
// C (GPU vs CPU pixels) does not apply: conversion draws nothing.
func TestBoundaryRenderRoundTrip(t *testing.T) {
	vs := []core.Vec2{{X: 1.5, Y: -2.25}, {}, {X: -1e6, Y: 1e6}}
	for i, v := range vs {
		if got := Vec2FromRenderPoint(Vec2ToRenderPoint(v)); got != v {
			t.Errorf("vec[%d] round trip = %v, want %v", i, got, v)
		}
	}
	cs := []core.Color{{R: 0.1, G: 0.2, B: 0.3, A: 0.4}, core.Black, core.White, core.Transparent}
	for i, col := range cs {
		if got := ColorFromRender(ColorToRender(col)); got != col {
			t.Errorf("color[%d] round trip = %v, want %v", i, got, col)
		}
	}
	ms := []core.Mat2D{core.Identity2D(), core.Translate2D(3, -4), core.Scale2D(2, 0.5), core.Rotate2D(0.7)}
	for i, m := range ms {
		if got := Mat2DFromRenderMatrix(Mat2DToRenderMatrix(m)); got != m {
			t.Errorf("mat[%d] round trip = %v, want %v", i, got, m)
		}
	}
}
