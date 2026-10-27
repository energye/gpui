//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func shaderMaskDC(t *testing.T, w, h int) *render.Context {
	t.Helper()
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := render.NewContext(w, h)
	t.Cleanup(func() { _ = dc.Close() })
	dc.ClearWithColor(render.White)
	return dc
}

func TestPaintContextDrawShaderMaskPaints(t *testing.T) {
	dc := shaderMaskDC(t, 32, 32)
	pc := rendering.NewPaintContext(dc, 1)

	called := false
	ok := pc.DrawShaderMask(0, 0, 32, 32, render.SolidBrush{Color: render.RGBA{R: 1, G: 1, B: 1, A: 0.5}}, func() {
		called = true
		rendering.FillRect(pc, 0, 0, 32, 32, 1, 0, 0, 1)
	})
	if !ok || !called {
		t.Fatalf("DrawShaderMask ok=%v called=%v, want both true", ok, called)
	}
	r, g, b, _ := dc.Image().At(16, 16).RGBA()
	if r>>8 < 250 || g>>8 < 121 || g>>8 > 135 || b>>8 < 121 || b>>8 > 135 {
		t.Fatalf("masked pixel = %d,%d,%d, want r~255 g/b~128", r>>8, g>>8, b>>8)
	}
	if got := pc.SaveLayerDepth(); got != 0 {
		t.Fatalf("saveLayerDepth = %d, want 0 (the mask layer is internal)", got)
	}
}

func TestPaintContextDrawShaderMaskBudgetAllowReject(t *testing.T) {
	dc := shaderMaskDC(t, 32, 32)
	stats := &rendering.SaveLayerStats{}
	// Area budget only: one 32x32 mask fits, the second crosses 2000.
	pc := rendering.NewPaintContext(dc, 1)
	pc.LayerBudget = &rendering.SaveLayerBudget{MaxArea: 2000}
	pc.LayerStats = stats

	mask := render.SolidBrush{Color: render.White}
	draws := 0
	draw := func() { draws++ }
	if !pc.DrawShaderMask(0, 0, 32, 32, mask, draw) {
		t.Fatal("first mask rejected, want allowed")
	}
	if pc.DrawShaderMask(0, 0, 32, 32, mask, draw) {
		t.Fatal("second mask allowed, want budget rejection")
	}
	if draws != 1 {
		t.Fatalf("draw callback ran %d times, want 1 (rejected mask must not paint)", draws)
	}
	if got := stats.Allow.Load(); got != 1 {
		t.Fatalf("stats.Allow = %d, want 1", got)
	}
	if got := stats.Reject.Load(); got != 1 {
		t.Fatalf("stats.Reject = %d, want 1", got)
	}
}

func TestPaintContextDrawShaderMaskNilMaskPassthrough(t *testing.T) {
	dc := shaderMaskDC(t, 16, 16)
	pc := rendering.NewPaintContext(dc, 1)
	stats := &rendering.SaveLayerStats{}
	pc.LayerBudget = &rendering.SaveLayerBudget{MaxOps: 1}
	pc.LayerStats = stats

	called := false
	if !pc.DrawShaderMask(0, 0, 16, 16, nil, func() {
		called = true
		rendering.FillRect(pc, 0, 0, 16, 16, 1, 0, 0, 1)
	}) {
		t.Fatal("nil mask returned false, want true (direct draw)")
	}
	if !called {
		t.Fatal("nil mask did not run the draw callback")
	}
	if stats.Allow.Load() != 0 || stats.Reject.Load() != 0 {
		t.Fatalf("nil mask consumed budget: allow=%d reject=%d", stats.Allow.Load(), stats.Reject.Load())
	}
	if r, g, _, _ := dc.Image().At(8, 8).RGBA(); r>>8 < 250 || g>>8 > 5 {
		t.Fatalf("nil mask must draw unmasked, pixel g=%d", g>>8)
	}
}

func TestPaintContextDrawShaderMaskKeepsTransformAndDepth(t *testing.T) {
	dc := shaderMaskDC(t, 32, 32)
	pc := rendering.NewPaintContext(dc, 1)
	pc.DC.Translate(10, 0)
	before := dc.GetTransform()

	if !pc.SaveLayer(32, 32, 1) {
		t.Fatal("SaveLayer refused")
	}
	if got := pc.SaveLayerDepth(); got != 1 {
		t.Fatalf("depth after SaveLayer = %d, want 1", got)
	}
	pc.DrawShaderMask(0, 0, 16, 16, render.SolidBrush{Color: render.White}, func() {
		rendering.FillRect(pc, 0, 0, 16, 16, 0, 0, 1, 1)
	})
	if got := pc.SaveLayerDepth(); got != 1 {
		t.Fatalf("depth after mask = %d, want 1 (internal layer must not touch the counter)", got)
	}
	if got := dc.GetTransform(); got != before {
		t.Fatalf("transform = %+v, want %+v (mask must not reset the CTM)", got, before)
	}
	pc.RestoreLayer()
	if got := pc.SaveLayerDepth(); got != 0 {
		t.Fatalf("depth after RestoreLayer = %d, want 0", got)
	}
}
