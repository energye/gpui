//go:build !nogpu

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
	_ "github.com/energye/gpui/render/gpu"
)

// Real GPU path: ShaderMask mask fills must composite with DestinationIn on the
// GPU (fixed-function blend + axis-aligned rect), with no CPU fallback.

func drawMaskProbe(t *testing.T, dc *render.Context, mask render.Brush) {
	t.Helper()
	render.DrawShaderMask(dc, 0, 0, float64(dc.Width()), float64(dc.Height()), mask, func() {
		fillRect(dc, 0, 0, float64(dc.Width()), float64(dc.Height()), solidBrush(render.RGBA{R: 1, A: 1}))
	})
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("mask fill did not reach the GPU: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("mask fill fell back to CPU: %s", stats.LogLine())
	}
}

func TestDrawShaderMaskGPU_SolidHalf(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.ResetRenderPathStats()

	drawMaskProbe(t, dc, solidBrush(render.RGBA{R: 1, G: 1, B: 1, A: 0.5}))

	r, g, b, _ := sampleRGBA(dc, 32, 32)
	almostEq(t, "r", r, 255, 3)
	almostEq(t, "g", g, 128, 6)
	almostEq(t, "b", b, 128, 6)
}

func TestDrawShaderMaskGPU_LinearHorizontal(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.ResetRenderPathStats()

	fade := render.NewLinearGradientBrush(0, 0, 64, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, G: 1, B: 1, A: 0})
	drawMaskProbe(t, dc, fade)

	_, gl, _, _ := sampleRGBA(dc, 2, 32)
	_, gm, _, _ := sampleRGBA(dc, 32, 32)
	_, gr, _, _ := sampleRGBA(dc, 61, 32)
	t.Logf("linear g=%d,%d,%d", gl, gm, gr)
	if gl > 40 {
		t.Fatalf("left edge g=%d want content-dominant", gl)
	}
	if !(gl < gm && gm < gr) {
		t.Fatalf("fade not monotonic: g=%d,%d,%d", gl, gm, gr)
	}
	if gr < 215 {
		t.Fatalf("right edge g=%d want background (alpha=0 erased)", gr)
	}
}

func TestDrawShaderMaskGPU_LinearDiagonal(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.ResetRenderPathStats()

	fade := render.NewLinearGradientBrush(0, 0, 64, 64).
		AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, G: 1, B: 1, A: 0})
	drawMaskProbe(t, dc, fade)

	_, g0, _, _ := sampleRGBA(dc, 6, 6)
	_, g1, _, _ := sampleRGBA(dc, 58, 58)
	if g0 > 50 {
		t.Fatalf("diagonal start g=%d want content-dominant", g0)
	}
	if g1 < 215 {
		t.Fatalf("diagonal end g=%d want background (alpha=0 erased)", g1)
	}
}

func TestDrawShaderMaskGPU_RadialVignette(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.ResetRenderPathStats()

	vig := render.NewRadialGradientBrush(32, 32, 0, 32).
		AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
		AddColorStop(1, render.RGBA{R: 1, G: 1, B: 1, A: 0})
	drawMaskProbe(t, dc, vig)

	_, gc, _, _ := sampleRGBA(dc, 32, 32)
	_, ge, _, _ := sampleRGBA(dc, 2, 32)
	if gc > 40 {
		t.Fatalf("radial center g=%d want content-dominant", gc)
	}
	if ge < 215 {
		t.Fatalf("radial edge g=%d want background (alpha=0 erased)", ge)
	}
}
