//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

import (
	"testing"

	"github.com/energye/gpui/render"
)

// A recording stores brushes in the space the caller drew in. Replaying it onto
// a target with a non-identity CTM (the retained-layer case: record locally,
// replay under a parent transform) must sample the gradient through that CTM
// exactly like a live draw does.

func gradientCTMBrush() render.Brush {
	g := render.NewLinearGradientBrush(0, 0, 100, 0)
	g.AddColorStop(0, render.RGBA{R: 0, G: 0, B: 0, A: 1})
	g.AddColorStop(1, render.RGBA{R: 1, G: 0, B: 0, A: 1})
	return g
}

// gradientCTMRecording records a horizontal black->red gradient fill of the
// rect (0,0)-(100,100). Translating the replay target moves both the rect and
// the gradient.
func gradientCTMRecording(t *testing.T) *Recording {
	t.Helper()
	rec := NewRecorder(100, 100)
	rec.SetFillBrush(gradientCTMBrush())
	rec.DrawRectangle(0, 0, 100, 100)
	if err := rec.Fill(); err != nil {
		t.Fatalf("record Fill: %v", err)
	}
	return rec.FinishRecording()
}

func gradientCTMLive(t *testing.T, w, h int, translateX float64) *render.Context {
	t.Helper()
	dc := render.NewContext(w, h)
	t.Cleanup(func() { _ = dc.Close() })
	if translateX != 0 {
		dc.Translate(translateX, 0)
	}
	dc.SetFillBrush(gradientCTMBrush())
	dc.DrawRectangle(0, 0, 100, 100)
	if err := dc.Fill(); err != nil {
		t.Fatalf("live Fill: %v", err)
	}
	return dc
}

func TestGradientCTMReplayUnderTranslatedTarget(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	rec := gradientCTMRecording(t)
	live := gradientCTMLive(t, 200, 100, 100)

	replayed := render.NewContext(200, 100)
	defer replayed.Close()
	replayed.Translate(100, 0)
	if err := rec.Playback(NewCanvasBackend(replayed)); err != nil {
		t.Fatalf("Playback: %v", err)
	}

	pts := [][2]int{{102, 50}, {125, 50}, {150, 50}, {175, 50}, {198, 50}}
	want := samplePixels(live.Image(), pts)
	got := samplePixels(replayed.Image(), pts)
	for i := range want {
		if !rgbNear(want[i], got[i]) {
			t.Errorf("replayed px %v = %v, want %v", pts[i], got[i], want[i])
		}
	}
	// The left half must stay untouched: the gradient moved with the CTM, it
	// did not stay pinned to the surface origin.
	if _, _, _, a := replayed.Image().At(50, 50).RGBA(); a != 0 {
		t.Errorf("left of translated replay alpha = %d, want transparent", a)
	}
	if want[0][0] >= want[4][0] {
		t.Fatalf("reference gradient is flat (%v): test would not detect a pinned brush", want)
	}
}

func TestGradientCTMReplayMatchesLiveAtRecordedCTM(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	rec := NewRecorder(200, 100)
	rec.Translate(100, 0)
	rec.SetFillBrush(gradientCTMBrush())
	rec.DrawRectangle(0, 0, 100, 100)
	if err := rec.Fill(); err != nil {
		t.Fatalf("record Fill: %v", err)
	}
	recorded := rec.FinishRecording()

	live := gradientCTMLive(t, 200, 100, 100)
	replayed := render.NewContext(200, 100)
	defer replayed.Close()
	if err := recorded.Playback(NewCanvasBackend(replayed)); err != nil {
		t.Fatalf("Playback: %v", err)
	}

	pts := [][2]int{{102, 50}, {125, 50}, {150, 50}, {175, 50}, {198, 50}}
	want := samplePixels(live.Image(), pts)
	got := samplePixels(replayed.Image(), pts)
	for i := range want {
		if !rgbNear(want[i], got[i]) {
			t.Errorf("replayed px %v = %v, want %v", pts[i], got[i], want[i])
		}
	}
}

// Folding a translated recording into another recorder must keep the CTM on the
// command stream so the brush conversion happens once, at the final replay.
func TestGradientCTMFoldedRecordingKeepsCTM(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	inner := NewRecorder(200, 100)
	inner.Translate(100, 0)
	inner.SetFillBrush(gradientCTMBrush())
	inner.DrawRectangle(0, 0, 100, 100)
	if err := inner.Fill(); err != nil {
		t.Fatalf("record Fill: %v", err)
	}
	innerRec := inner.FinishRecording()

	outerRec := NewRecorder(200, 100)
	if err := innerRec.Playback(NewCanvasBackend(outerRec)); err != nil {
		t.Fatalf("fold playback: %v", err)
	}
	folded := outerRec.FinishRecording()

	live := gradientCTMLive(t, 200, 100, 100)
	replayed := render.NewContext(200, 100)
	defer replayed.Close()
	if err := folded.Playback(NewCanvasBackend(replayed)); err != nil {
		t.Fatalf("folded playback: %v", err)
	}

	pts := [][2]int{{102, 50}, {125, 50}, {150, 50}, {175, 50}, {198, 50}}
	want := samplePixels(live.Image(), pts)
	got := samplePixels(replayed.Image(), pts)
	for i := range want {
		if !rgbNear(want[i], got[i]) {
			t.Errorf("folded px %v = %v, want %v", pts[i], got[i], want[i])
		}
	}
}
