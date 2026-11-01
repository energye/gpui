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
	"bytes"
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// TestRecorderDrawImageQuadPerspectivePassThrough pins that perspective state
// (P3.5 batch 3) rides the existing DrawImageQuadCommand verbatim: the
// Recorder records Opts as a value, so Perspective and Weights must survive
// record → replay without a new command type.
func TestRecorderDrawImageQuadPerspectivePassThrough(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	img, err := render.NewImageBuf(20, 1, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer img.Dispose()
	for x := 0; x < 20; x++ {
		r, g, b := uint8(255), uint8(0), uint8(0)
		if x >= 16 {
			r, g, b = 0, 0, 255
		}
		if err := img.SetRGBA(x, 0, r, g, b, 255); err != nil {
			t.Fatalf("SetRGBA: %v", err)
		}
	}

	corners := [4]render.Point{
		{X: 0.5, Y: 0.5},
		{X: 240.5, Y: 0.5},
		{X: 240.5, Y: 120.5},
		{X: 0.5, Y: 120.5},
	}
	weights := [4]float64{200, 100, 100, 200}
	opts := render.QuadDrawOptions{
		Perspective:   true,
		Weights:       weights,
		Interpolation: render.InterpNearest,
		Opacity:       1,
	}

	rec := NewRecorder(241, 121)
	if err := rec.DrawImageQuad(img, corners, opts); err != nil {
		t.Fatalf("recorder DrawImageQuad: %v", err)
	}
	var cmd *DrawImageQuadCommand
	for i := range rec.commands {
		if c, ok := rec.commands[i].(DrawImageQuadCommand); ok {
			got := c
			cmd = &got
		}
	}
	if cmd == nil {
		t.Fatal("no DrawImageQuadCommand recorded")
	}
	if !cmd.Opts.Perspective || cmd.Opts.Weights != weights {
		t.Fatalf("recorded opts lost perspective: %+v", cmd.Opts)
	}

	// Live vs replay must be pixel-identical.
	live := render.NewContext(241, 121)
	defer func() { _ = live.Close() }()
	live.ClearWithColor(render.White)
	if err := live.DrawImageQuad(img, corners, opts); err != nil {
		t.Fatalf("live draw: %v", err)
	}
	_ = live.FlushGPU()

	replay := render.NewContext(241, 121)
	defer func() { _ = replay.Close() }()
	replay.ClearWithColor(render.White)
	if err := rec.FinishRecording().Playback(NewCanvasBackend(replay)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	_ = replay.FlushGPU()

	// Live vs replay must be pixel-identical (batch 2 playback-pair pattern).
	liveImg := live.Image().(*image.RGBA)
	replayImg := replay.Image().(*image.RGBA)
	if !bytes.Equal(liveImg.Pix, replayImg.Pix) {
		t.Fatal("replay pixels differ from live perspective quad")
	}
	for _, p := range []struct{ x, y int }{{180, 5}, {60, 5}, {120, 60}, {180, 90}} {
		r, _, b, _ := liveImg.At(p.x, p.y).RGBA()
		rr, bb := int(r>>8), int(b>>8)
		if !(rr > 250 && bb < 5) && !(bb > 250 && rr < 5) {
			t.Fatalf("live probe (%d,%d) neither red nor blue: rgb=(%d,%d)", p.x, p.y, rr, bb)
		}
		r2, _, b2, _ := replayImg.At(p.x, p.y).RGBA()
		if r2 != r || b2 != b {
			t.Fatalf("replay probe (%d,%d) = (%d,%d), live (%d,%d)", p.x, p.y, r2>>8, b2>>8, rr, bb)
		}
	}
}
