//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

func forceCPU(t *testing.T) {
	t.Helper()
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
}

func solidBrush(c render.RGBA) render.Brush {
	return render.SolidBrush{Color: c}
}

func cpuPixel(t *testing.T, dc *render.Context, x, y int) (r, g, b, a uint8) {
	t.Helper()
	rr, gg, bb, aa := dc.Image().At(x, y).RGBA()
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8), uint8(aa >> 8)
}

func fillRect(dc *render.Context, x, y, w, h float64, brush render.Brush) {
	dc.SetFillBrush(brush)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

func TestDrawShaderMask_SolidHalfAlpha(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	white := render.RGBA{R: 1, G: 1, B: 1, A: 0.5}
	render.DrawShaderMask(dc, 0, 0, 32, 32, solidBrush(white), func() {
		fillRect(dc, 0, 0, 32, 32, solidBrush(render.RGBA{R: 1, A: 1}))
	})

	r, g, b, a := cpuPixel(t, dc, 16, 16)
	if r < 250 {
		t.Fatalf("r=%d want ~255 (content kept)", r)
	}
	if g < 121 || g > 135 || b < 121 || b > 135 {
		t.Fatalf("masked content rgba=%d,%d,%d,%d want g/b ~128 over white", r, g, b, a)
	}
}

func TestDrawShaderMask_LinearFadeAndZeroStop(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	fade := render.NewLinearGradientBrush(0, 0, 32, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, G: 1, B: 1, A: 0})
	red := solidBrush(render.RGBA{R: 1, A: 1})
	render.DrawShaderMask(dc, 0, 0, 32, 32, fade, func() {
		fillRect(dc, 0, 0, 32, 32, red)
	})

	_, g0, _, _ := cpuPixel(t, dc, 2, 16)
	_, g1, _, _ := cpuPixel(t, dc, 16, 16)
	_, g2, _, _ := cpuPixel(t, dc, 29, 16)
	if g0 > 40 {
		t.Fatalf("left edge g=%d want content-dominant (fade start alpha=1)", g0)
	}
	if !(g0 < g1 && g1 < g2) {
		t.Fatalf("fade not monotonic: g=%d,%d,%d", g0, g1, g2)
	}
	if g2 < 225 {
		t.Fatalf("right edge g=%d want background (alpha=0 erased)", g2)
	}

	// A zero-alpha stop must erase, not leave the content untouched. Keep the
	// gradient inside 0..8 so everything to its right is ExtendPad alpha 0.
	dc2 := render.NewContext(32, 32)
	defer dc2.Close()
	dc2.ClearWithColor(render.White)
	hard := render.NewLinearGradientBrush(0, 0, 8, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, G: 1, B: 1, A: 0})
	render.DrawShaderMask(dc2, 0, 0, 32, 32, hard, func() {
		fillRect(dc2, 0, 0, 32, 32, red)
	})
	r, g, b, _ := cpuPixel(t, dc2, 24, 16)
	if r < 250 || g < 250 || b < 250 {
		t.Fatalf("zero-alpha stop did not erase: rgba=%d,%d,%d", r, g, b)
	}
	if r, g, b, _ := cpuPixel(t, dc2, 1, 16); g > 60 || b > 60 {
		t.Fatalf("inside stop-1 region not content-dominant: rgba=%d,%d,%d", r, g, b)
	}
}

func TestDrawShaderMask_BoundsClip(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	white := render.RGBA{R: 1, G: 1, B: 1, A: 1}
	red := solidBrush(render.RGBA{R: 1, A: 1})
	render.DrawShaderMask(dc, 8, 8, 16, 16, solidBrush(white), func() {
		fillRect(dc, 0, 0, 32, 32, red)
	})

	if r, g, b, _ := cpuPixel(t, dc, 2, 2); g < 250 || b < 250 {
		t.Fatalf("content leaked outside mask bounds: rgba=%d,%d,%d", r, g, b)
	}
	if r, g, b, _ := cpuPixel(t, dc, 16, 16); r < 250 || g > 5 || b > 5 {
		t.Fatalf("content missing inside mask bounds: rgba=%d,%d,%d", r, g, b)
	}
	if r, g, b, _ := cpuPixel(t, dc, 30, 30); g < 250 || b < 250 {
		t.Fatalf("content leaked past bounds bottom-right: rgba=%d,%d,%d", r, g, b)
	}
}

func TestDrawShaderMask_RestoresState(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	blue := render.RGBA{B: 1, A: 1}
	dc.Translate(3, 4)
	dc.SetFillBrush(solidBrush(blue))
	dc.SetBlendMode(render.BlendCopy)
	dc.SetAntiAlias(false)
	dc.ClipRect(10, 10, 10, 10)

	render.DrawShaderMask(dc, 0, 0, 32, 32, solidBrush(render.White), func() {
		fillRect(dc, 0, 0, 32, 32, solidBrush(render.RGBA{R: 1, A: 1}))
	})

	if got := dc.BlendMode(); got != render.BlendCopy {
		t.Fatalf("blend not restored: %v", got)
	}
	if dc.AntiAlias() {
		t.Fatalf("anti-alias not restored to false")
	}
	if got := dc.GetTransform(); got != render.Translate(3, 4) {
		t.Fatalf("transform not restored: %+v", got)
	}
	sb, ok := dc.FillBrush().(render.SolidBrush)
	if !ok || sb.Color != blue {
		t.Fatalf("fill brush not restored: %#v", dc.FillBrush())
	}

	// The caller's clip must still be in effect: this fill only lands inside
	// the translated 10,10,10,10 clip.
	fillRect(dc, 0, 0, 32, 32, solidBrush(render.RGBA{G: 1, A: 1}))
	if _, g, _, _ := cpuPixel(t, dc, 2, 2); g < 250 {
		t.Fatalf("clip lost: pixel outside clip repainted")
	}
	if _, g, b, _ := cpuPixel(t, dc, 18, 18); g < 200 || b > 60 {
		t.Fatalf("clip region not painted: g=%d b=%d", g, b)
	}
}

func TestDrawShaderMask_DroppedDanglingPath(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	// Leave a full-surface path pending across the mask call.
	dc.DrawRectangle(0, 0, 32, 32)
	render.DrawShaderMask(dc, 8, 8, 16, 16, solidBrush(render.White), func() {
		fillRect(dc, 8, 8, 16, 16, solidBrush(render.RGBA{R: 1, A: 1}))
	})
	// Filling now must not paint the dangling full-surface rect.
	dc.SetFillBrush(solidBrush(render.RGBA{B: 1, A: 1}))
	_ = dc.Fill()
	if r, g, b, _ := cpuPixel(t, dc, 2, 2); g < 250 || b < 250 {
		t.Fatalf("dangling path filled after mask: rgba=%d,%d,%d", r, g, b)
	}
}

func TestDrawShaderMask_NilMaskDrawsDirect(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	called := false
	render.DrawShaderMask(dc, 0, 0, 16, 16, nil, func() {
		called = true
		fillRect(dc, 0, 0, 16, 16, solidBrush(render.RGBA{R: 1, A: 1}))
	})
	if !called {
		t.Fatal("draw callback not invoked for nil mask")
	}
	if r, g, _, _ := cpuPixel(t, dc, 8, 8); r < 250 || g > 5 {
		t.Fatalf("nil mask must draw unmasked: rgba=%d,%d", r, g)
	}
}

func TestDrawShaderMask_Nested(t *testing.T) {
	forceCPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)

	half := solidBrush(render.RGBA{R: 1, G: 1, B: 1, A: 0.5})
	red := solidBrush(render.RGBA{R: 1, A: 1})
	render.DrawShaderMask(dc, 0, 0, 32, 32, half, func() {
		render.DrawShaderMask(dc, 0, 0, 32, 32, half, func() {
			fillRect(dc, 0, 0, 32, 32, red)
		})
	})

	// 0.5 * 0.5 = 0.25 source alpha over white.
	r, g, b, _ := cpuPixel(t, dc, 16, 16)
	if r < 250 {
		t.Fatalf("r=%d want content kept", r)
	}
	if g < 183 || g > 200 || b < 183 || b > 200 {
		t.Fatalf("nested mask rgba=%d,%d,%d want g/b ~191", r, g, b)
	}
}
