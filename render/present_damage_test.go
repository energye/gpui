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
	"image"
	"testing"
)

// TestClipPresentDamage pins the rects handed to the OS present path:
// clipped to the physical surface, empty rects dropped, touching rects
// coalesced (the same policy PlanFramePresent uses).
func TestClipPresentDamage(t *testing.T) {
	got := clipPresentDamage([]image.Rectangle{
		image.Rect(10, 10, 20, 20),
		image.Rect(20, 10, 30, 20), // touches the first → one rect
		image.Rect(-50, -50, -10, -10),
		image.Rect(90, 90, 200, 200), // clipped to 100x100
	}, 100, 100)
	want := []image.Rectangle{
		image.Rect(10, 10, 30, 20),
		image.Rect(90, 90, 100, 100),
	}
	if len(got) != len(want) {
		t.Fatalf("rects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rects = %v, want %v", got, want)
		}
	}
	if clipPresentDamage(nil, 100, 100) != nil {
		t.Fatal("nil input must stay nil")
	}
	if clipPresentDamage([]image.Rectangle{image.Rect(10, 10, 20, 20)}, 0, 100) != nil {
		t.Fatal("zero surface must return nil")
	}
	if clipPresentDamage([]image.Rectangle{image.Rect(-10, -10, 0, 0)}, 100, 100) != nil {
		t.Fatal("fully outside damage must return nil")
	}
}
