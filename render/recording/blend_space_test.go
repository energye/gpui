//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package recording

import (
	"testing"

	"github.com/energye/gpui/render"
)

// P3.1 recording pins: BlendSpace is a draw-time property carried per command
// (like BlendMode), so replay neither depends on nor leaks into the target's
// ambient state.

// recordLinearRedSquare records a red square with Multiply + Linear ambient
// state, plus a second square back in the default domain.
func recordLinearRedSquare(t *testing.T) *Recording {
	t.Helper()
	rec := NewRecorder(120, 120)
	rec.SetBlendMode(render.BlendMultiply)
	rec.SetBlendSpace(render.BlendSpaceLinear)
	rec.SetRGBA(1, 0, 0, 1)
	rec.FillRectangle(0, 0, 20, 20)
	rec.StrokeRectangle(0, 0, 20, 20)
	rec.SetBlendSpace(render.BlendSpaceSRGB)
	rec.Translate(30, 0)
	rec.FillRectangle(0, 0, 20, 20)
	return rec.FinishRecording()
}

// TestRecorderBlendSpaceCarriedPerCommand pins the recorder side: the ambient
// space at record time lands in each fill command; switching the ambient state
// mid-recording switches what later commands carry.
func TestRecorderBlendSpaceCarriedPerCommand(t *testing.T) {
	pic := recordLinearRedSquare(t)
	sawLinearFill, sawLinearStroke, sawSRGBFill := false, false, false
	for _, cmd := range pic.Commands() {
		switch c := cmd.(type) {
		case FillRectCommand:
			if c.BlendSpace == render.BlendSpaceLinear {
				sawLinearFill = true
				if c.Blend != render.BlendMultiply {
					t.Fatalf("linear FillRectCommand carried Blend=%d, want Multiply", c.Blend)
				}
			} else {
				sawSRGBFill = true
			}
		case StrokeRectCommand:
			if c.BlendSpace == render.BlendSpaceLinear {
				sawLinearStroke = true
			}
		}
	}
	if !sawLinearFill || !sawLinearStroke || !sawSRGBFill {
		t.Fatalf("expected linear FillRect+StrokeRect and an sRGB FillRect, got fill=%v stroke=%v srgb=%v", sawLinearFill, sawLinearStroke, sawSRGBFill)
	}
}

// TestRecorderBlendSpaceAmbientState pins the accessor defaults.
func TestRecorderBlendSpaceAmbientState(t *testing.T) {
	rec := NewRecorder(8, 8)
	if rec.BlendSpace() != render.BlendSpaceSRGB {
		t.Fatalf("fresh Recorder.BlendSpace() = %d, want SRGB", rec.BlendSpace())
	}
	rec.SetBlendSpace(render.BlendSpaceLinear)
	if rec.BlendSpace() != render.BlendSpaceLinear {
		t.Fatalf("SetBlendSpace roundtrip failed")
	}
}

// TestCanvasBackendBlendSpaceReplayAndRestore pins the replay side: replaying
// linear commands must not leak the linear space into the target's ambient
// state (display-list players rely on the withBlend restore).
func TestCanvasBackendBlendSpaceReplayAndRestore(t *testing.T) {
	dc := render.NewContext(120, 120)
	t.Cleanup(func() { _ = dc.Close() })
	ambient := dc.BlendSpace()
	if err := recordLinearRedSquare(t).Playback(NewCanvasBackend(dc)); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if got := dc.BlendSpace(); got != ambient {
		t.Fatalf("replay leaked ambient blend space: got %d, want %d", got, ambient)
	}
}
