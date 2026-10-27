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
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

func shaderMaskFade() render.Brush {
	return render.NewLinearGradientBrush(0, 0, 64, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, G: 1, B: 1, A: 0})
}

// recordShaderMask records a full-surface red fill masked by a horizontal
// white-to-transparent fade, the same shape ui/rendering uses for retained
// layers.
func recordShaderMask(t *testing.T) *Recording {
	t.Helper()
	rec := NewRecorder(64, 64)
	render.DrawShaderMask(rec, 0, 0, 64, 64, shaderMaskFade(), func() {
		rec.SetRGBA(1, 0, 0, 1)
		rec.DrawRectangle(0, 0, 64, 64)
		_ = rec.Fill()
	})
	return rec.FinishRecording()
}

func fillBlends(rec *Recording) []render.BlendMode {
	var out []render.BlendMode
	for _, cmd := range rec.Commands() {
		if c, ok := cmd.(FillPathCommand); ok {
			out = append(out, c.Blend)
		}
	}
	return out
}

// The blend must ride the fill command itself: replay then never depends on
// the target's ambient paint state.
func TestShaderMaskRecordingCarriesBlendPerFill(t *testing.T) {
	rec := recordShaderMask(t)

	blends := fillBlends(rec)
	if len(blends) != 2 {
		t.Fatalf("fill commands = %d, want 2 (content + mask): %v", len(blends), blends)
	}
	if blends[0] != render.BlendNormal {
		t.Fatalf("content fill blend = %v, want Normal", blends[0])
	}
	if blends[1] != render.BlendDestinationIn {
		t.Fatalf("mask fill blend = %v, want DestinationIn", blends[1])
	}
}

func TestShaderMaskRecordingRestoresRecorderBlend(t *testing.T) {
	rec := NewRecorder(16, 16)
	rec.SetBlendMode(render.BlendCopy)
	render.DrawShaderMask(rec, 0, 0, 16, 16, render.SolidBrush{Color: render.White}, func() {})
	if got := rec.BlendMode(); got != render.BlendCopy {
		t.Fatalf("recorder blend after mask = %v, want Copy (state must be restored)", got)
	}
}

func samplePixels(img image.Image, pts [][2]int) [][4]uint8 {
	out := make([][4]uint8, 0, len(pts))
	for _, p := range pts {
		r, g, b, a := img.At(p[0], p[1]).RGBA()
		out = append(out, [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)})
	}
	return out
}

func rgbNear(a, b [4]uint8) bool {
	for i := 0; i < 4; i++ {
		da := int(a[i]) - int(b[i])
		if da < 0 {
			da = -da
		}
		if da > 2 {
			return false
		}
	}
	return true
}

// A recording must rasterize to the same pixels as a live draw, and folding it
// into another recorder must preserve both the pixels and the per-command blend.
func TestShaderMaskRecordingReplayParity(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")

	live := render.NewContext(64, 64)
	defer live.Close()
	live.ClearWithColor(render.White)
	render.DrawShaderMask(live, 0, 0, 64, 64, shaderMaskFade(), func() {
		live.SetFillBrush(render.SolidBrush{Color: render.RGBA{R: 1, A: 1}})
		live.DrawRectangle(0, 0, 64, 64)
		_ = live.Fill()
	})

	rec := recordShaderMask(t)
	replayed := render.NewContext(64, 64)
	defer replayed.Close()
	replayed.ClearWithColor(render.White)
	// A non-Normal ambient mode must not leak into the replayed list or survive
	// it: each fill carries its own mode and restores the target afterwards.
	replayed.SetBlendMode(render.BlendCopy)
	if err := rec.Playback(NewCanvasBackend(replayed)); err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if got := replayed.BlendMode(); got != render.BlendCopy {
		t.Fatalf("ambient blend after playback = %v, want Copy (no leak)", got)
	}

	// Fold into a second recorder; its command stream must keep the modes.
	outerRec := NewRecorder(64, 64)
	if err := rec.Playback(NewCanvasBackend(outerRec)); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	outer := outerRec.FinishRecording()
	outerBlends := fillBlends(outer)
	if len(outerBlends) != 2 || outerBlends[0] != render.BlendNormal || outerBlends[1] != render.BlendDestinationIn {
		t.Fatalf("folded fill blends = %v, want [Normal DestinationIn]", outerBlends)
	}
	folded := render.NewContext(64, 64)
	defer folded.Close()
	folded.ClearWithColor(render.White)
	if err := outer.Playback(NewCanvasBackend(folded)); err != nil {
		t.Fatalf("folded playback: %v", err)
	}

	pts := [][2]int{{2, 32}, {16, 32}, {32, 32}, {48, 32}, {61, 32}, {2, 2}, {61, 61}}
	want := samplePixels(live.Image(), pts)
	for name, got := range map[string][][4]uint8{
		"replayed": samplePixels(replayed.Image(), pts),
		"folded":   samplePixels(folded.Image(), pts),
	} {
		for i := range want {
			if !rgbNear(want[i], got[i]) {
				t.Errorf("%s px %v = %v, want %v", name, pts[i], got[i], want[i])
			}
		}
	}

	// Sanity on the live reference itself: fade start keeps content, fade end
	// shows background.
	if want[0][1] > 40 {
		t.Fatalf("live fade start g=%d want content-dominant", want[0][1])
	}
	if want[4][1] < 215 {
		t.Fatalf("live fade end g=%d want background (alpha=0 erased)", want[4][1])
	}
}
