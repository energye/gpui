//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// K2 morphology propagation pin (SDF coverage line, evidence/SDF/coverage-line.md).
//
// The GPU SDF coverage LUT is a single-variable model: one signed distance gets
// one coverage. Its max error against the CPU analytic row integration has a
// mathematical floor of about 5.5/255 (the truth spread at fixed d is
// radius-independent, evidence/SDF/coverage-line.md §前置②). A hard edge can
// therefore never be bit-identical on the two legs; what must hold is:
//   1. every differing pixel sits in the 1px ring around the shape edge —
//      nothing leaks into the interior or beyond the AA band;
//   2. the amplitude stays inside the LUT floor (<=8, the layered quality gate
//      used for r<5 in render/sdf_coverage_quality_test.go).
// Before the LUT the raw edge ring was 18 levels and erode propagated it to
// max 17 over the same 404px band (evidence/P2.6/k2-erode-edge-dissection.md).
func TestK2MorphologyPropagation(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 192, 240
	bg := func(dc *render.Context) {
		dc.ClearWithColor(render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1})
	}
	rect := func(dc *render.Context) {
		dc.SetRGBA(1, 1, 1, 1)
		dc.DrawRectangle(60, 50, 60, 60)
		_ = dc.Fill()
	}
	scenes := []struct {
		name string
		draw func(*render.Context)
	}{
		{"raw", func(dc *render.Context) { bg(dc); rect(dc) }},
		{"erode5", func(dc *render.Context) { bg(dc); rect(dc); dc.ApplyErode(5) }},
		{"dilate5", func(dc *render.Context) { bg(dc); rect(dc); dc.ApplyDilate(5) }},
	}
	for _, sc := range scenes {
		gpu := render.NewContext(w, h)
		sc.draw(gpu)
		gi := gpu.Image()
		_ = gpu.Close()

		cpu := render.NewContext(w, h, render.WithCPUOnly())
		sc.draw(cpu)
		ci := cpu.Image()
		_ = cpu.Close()

		white := func(img interface{ At(int, int) interface{} }) bool { return false }
		_ = white
		isWhite := func(x, y int) bool {
			r, g, b, _ := ci.At(x, y).RGBA()
			return r>>8 > 200 && g>>8 > 200 && b>>8 > 200
		}
		band := make([]bool, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				w0 := isWhite(x, y)
				edge := false
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := x+d[0], y+d[1]
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					if isWhite(nx, ny) != w0 {
						edge = true
					}
				}
				if !edge {
					continue
				}
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := x+dx, y+dy
						if nx >= 0 && ny >= 0 && nx < w && ny < h {
							band[ny*w+nx] = true
						}
					}
				}
			}
		}
		diff, maxd, outside := 0, 0, 0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				ar, ag, ab, aa := gi.At(x, y).RGBA()
				br, bgc, bb, ba := ci.At(x, y).RGBA()
				d := 0
				for _, v := range [4]int{int(ar>>8) - int(br>>8), int(ag>>8) - int(bgc>>8), int(ab>>8) - int(bb>>8), int(aa>>8) - int(ba>>8)} {
					if v < 0 {
						v = -v
					}
					if v > d {
						d = v
					}
				}
				if d == 0 {
					continue
				}
				diff++
				if d > maxd {
					maxd = d
				}
				if !band[y*w+x] {
					outside++
					if outside <= 4 {
						t.Logf("K2 %s: diff %d at (%d,%d) outside the AA band", sc.name, d, x, y)
					}
				}
			}
		}
		t.Logf("K2 %-8s diff_px=%d max=%d outside_band=%d", sc.name, diff, maxd, outside)
		if outside != 0 {
			t.Fatalf("K2 %s: %d differing pixels outside the 1px AA band (morphology propagated a coverage error)", sc.name, outside)
		}
		if maxd > 8 {
			t.Fatalf("K2 %s: AA band max delta %d > 8 (SDF coverage LUT floor regressed)", sc.name, maxd)
		}
	}
}
