//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package stroke

import "testing"

// TestStrokeExpanderRejectsConic pins the boundary guard: the expander has no
// rational-quadratic stroke implementation, so a conic that was not reduced to
// quads first must panic instead of desyncing the coordinate stream.
func TestStrokeExpanderRejectsConic(t *testing.T) {
	e := NewStrokeExpander(Stroke{Width: 2})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expand accepted a conic verb; expected panic")
		}
	}()
	e.Expand(
		[]PathVerb{VerbMoveTo, VerbConicTo},
		[]float64{0, 0, 10, 10, 20, 20, 0.7071067811865476},
	)
}
