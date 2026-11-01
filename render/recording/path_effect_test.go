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

// TestRecorderStrokePathEffectPassThrough pins the batch-4 recording contract:
// SetStroke carries PathEffect into every stroke command recorded afterwards
// (reference, per the effect purity contract), and replay rasterizes
// identically to the live Context.
func TestRecorderStrokePathEffectPassThrough(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	zig := render.NewPath()
	zig.MoveTo(15, 90)
	zig.LineTo(60, 20)
	zig.LineTo(105, 90)
	zig.LineTo(150, 20)

	mkStroke := func() render.Stroke {
		return render.DefaultStroke().
			WithWidth(6).
			WithCap(render.LineCapRound).
			WithPathEffect(render.NewCornerPathEffect(12))
	}

	live := render.NewContext(200, 120)
	defer func() { _ = live.Close() }()
	live.ClearWithColor(render.White)
	live.SetRGB(1, 0, 0)
	live.SetStroke(mkStroke())
	live.DrawPath(zig)
	if err := live.Stroke(); err != nil {
		t.Fatalf("live Stroke: %v", err)
	}
	_ = live.FlushGPU()

	rec := NewRecorder(200, 120)
	rec.ClearWithColor(render.White)
	rec.SetRGB(1, 0, 0)
	rec.SetStroke(mkStroke())
	rec.AppendPath(zig)
	if err := rec.Stroke(); err != nil {
		t.Fatalf("recorder Stroke: %v", err)
	}
	var cmd *StrokePathCommand
	for i := range rec.commands {
		if c, ok := rec.commands[i].(StrokePathCommand); ok {
			got := c
			cmd = &got
		}
	}
	if cmd == nil {
		t.Fatal("no StrokePathCommand recorded")
	}
	if cmd.Stroke.PathEffect == nil {
		t.Fatal("recorded stroke lost the PathEffect")
	}

	replay := render.NewContext(200, 120)
	defer func() { _ = replay.Close() }()
	if err := rec.FinishRecording().Playback(NewCanvasBackend(replay)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	_ = replay.FlushGPU()

	liveImg := live.Image().(*image.RGBA)
	replayImg := replay.Image().(*image.RGBA)
	if !bytes.Equal(liveImg.Pix, replayImg.Pix) {
		t.Fatal("replay pixels differ from live path-effected stroke")
	}
}
