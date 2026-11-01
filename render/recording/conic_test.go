//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package recording

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// TestRecorderConicPathRoundtrip pins that the recording surface stores the
// conic verb with its weight instead of silently dropping the arc when the
// current path is handed to the resource pool.
func TestRecorderConicPathRoundtrip(t *testing.T) {
	rec := NewRecorder(20, 20)
	rec.MoveTo(10, 0)
	rec.ConicTo(10, 10, 0, 10, math.Sqrt2/2)
	rec.ClosePath()
	if err := rec.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}

	pic := rec.FinishRecording()
	var fill *FillPathCommand
	for _, cmd := range pic.Commands() {
		if c, ok := cmd.(FillPathCommand); ok {
			fill = &c
			break
		}
	}
	if fill == nil {
		t.Fatal("recording has no FillPathCommand")
	}

	path := pic.Resources().GetPath(fill.Path)
	if path == nil {
		t.Fatal("FillPathCommand path resource is missing")
	}
	var conic []float64
	path.Iterate(func(verb render.PathVerb, coords []float64) {
		if verb == render.ConicTo {
			conic = append(conic, coords...)
		}
	})
	want := []float64{10, 10, 0, 10, math.Sqrt2 / 2}
	if len(conic) != len(want) {
		t.Fatalf("recorded conic coords = %v, want %v", conic, want)
	}
	for i := range want {
		if conic[i] != want[i] {
			t.Errorf("recorded conic coord[%d] = %v, want %v", i, conic[i], want[i])
		}
	}
}
