//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// P4.3 root-cause pin (2026-11-10): a persistent filter source RT must start
// clean for every filter application. The session keeps a view in
// renderedViews/colorContentViews for the whole frame, so reusing filterSrcView
// across two filter draws used to make the second filter's output contain the
// first one's silhouette; PopLayer then re-composited the first shadow at its
// own position (frame-local stacking). The CPU leg always starts from the clean
// layer pixmap, so GPU must too.
func p43ShadowRect(dc *render.Context, x0, y0 float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(x0, y0)
	p.LineTo(x0+60, y0)
	p.LineTo(x0+60, y0+40)
	p.LineTo(x0, y0+40)
	p.Close()
	return p
}

func p43RenderShadowScene(t *testing.T, both bool) *image.RGBA {
	t.Helper()
	dc := render.NewContext(240, 180)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.DrawShadow(p43ShadowRect(dc, 20, 20), render.RGBA{A: 1}, 10, false)
	if both {
		dc.DrawShadow(p43ShadowRect(dc, 150, 20), render.RGBA{A: 1}, 10, false)
	}
	img := dc.Image()
	st := dc.RenderPathStats()
	if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("shadow scene did not run on GPU: %s", st.LogLine())
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("Image() = %T, want *image.RGBA", img)
	}
	return rgba
}

func p43MaxDiffInRect(a, b *image.RGBA, r image.Rectangle) (int, image.Point) {
	maxD := 0
	var at image.Point
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ai := a.PixOffset(x, y)
			bi := b.PixOffset(x, y)
			for c := 0; c < 4; c++ {
				d := int(a.Pix[ai+c]) - int(b.Pix[bi+c])
				if d < 0 {
					d = -d
				}
				if d > maxD {
					maxD = d
					at = image.Pt(x, y)
				}
			}
		}
	}
	return maxD, at
}

func TestDrawShadowRepeatedFrameDoesNotStack(t *testing.T) {
	requireFilterOpsGPU(t)
	// A's shadow band: body (20,20)-(80,60), lift 10, 3-sigma tail below.
	// The second shadow at (150,20) is far outside this rect's Gaussian tail.
	probe := image.Rect(10, 60, 100, 110)
	want := p43RenderShadowScene(t, false)
	got := p43RenderShadowScene(t, true)
	if d, at := p43MaxDiffInRect(want, got, probe); d != 0 {
		t.Fatalf("second DrawShadow re-composited the first shadow: max diff %d at %v "+
			"(filterSrcView carried the first silhouette into the second filter; "+
			"clean control %v vs repeated %v)", d, at,
			want.RGBAAt(at.X, at.Y), got.RGBAAt(at.X, at.Y))
	}
}

func TestLayerFilterSourceNotReusedAcrossDraws(t *testing.T) {
	requireFilterOpsGPU(t)
	paint := func(dc *render.Context, x0 float64) {
		dc.PushLayerIsolated(1)
		dc.SetRGBA(0, 0, 0, 0.25)
		dc.DrawRectangle(x0, 20, 60, 40)
		if err := dc.Fill(); err != nil {
			t.Fatalf("Fill: %v", err)
		}
		dc.ApplyBlur((800.0 / 600.0) * 10)
		dc.PopLayer()
	}
	run := func(both bool) *image.RGBA {
		dc := render.NewContext(240, 180)
		defer dc.Close()
		dc.ClearWithColor(render.White)
		paint(dc, 20)
		if both {
			paint(dc, 150)
		}
		img := dc.Image()
		st := dc.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("layer blur scene did not run on GPU: %s", st.LogLine())
		}
		rgba, ok := img.(*image.RGBA)
		if !ok {
			t.Fatalf("Image() = %T, want *image.RGBA", img)
		}
		return rgba
	}
	probe := image.Rect(10, 60, 100, 110)
	want := run(false)
	got := run(true)
	if d, at := p43MaxDiffInRect(want, got, probe); d != 0 {
		t.Fatalf("second layer filter re-composited the first silhouette: max diff %d at %v "+
			"(clean control %v vs repeated %v)", d, at,
			want.RGBAAt(at.X, at.Y), got.RGBAAt(at.X, at.Y))
	}
}
