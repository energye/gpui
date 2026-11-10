//go:build !nogpu

// Temporary P4.3 stage-1 diagnostic probe (2026-11-10). Run with
// GPUI_P43_TIMING=1 later for the segmented attribution; this rough probe only
// measures end-to-end cost and which path (GPU graph vs CPU blur) is taken.
// Removed after stage-1 attribution is written to evidence/P4.3/.
package render_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

type p43Case struct {
	Name      string  `json:"name"`
	W         int     `json:"w"`
	H         int     `json:"h"`
	Elevation float64 `json:"elevation"`
}

func TestP43RoughProbe(t *testing.T) {
	requireFilterOpsGPU(t)
	data, err := os.ReadFile("testdata/p43_timing_cases.json")
	if err != nil {
		t.Skipf("testdata/p43_timing_cases.json missing: %v", err)
	}
	var cfg struct {
		Cases []p43Case `json:"cases"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("bad p43 cases: %v", err)
	}
	for _, c := range cfg.Cases {
		for _, mode := range []string{"blur-only", "drawshadow"} {
			t.Run(c.Name+"/"+mode, func(t *testing.T) {
				dc := render.NewContext(c.W, c.H)
				defer dc.Close()
				dc.ClearWithColor(render.White)
				path := render.NewPath()
				path.MoveTo(40, 40)
				path.LineTo(240, 40)
				path.LineTo(240, 160)
				path.LineTo(40, 160)
				path.Close()
				if mode == "blur-only" {
					dc.SetRGBA(0, 0, 0, 1)
					dc.DrawPath(path)
					_ = dc.Fill()
				}
				dc.ResetRenderPathStats()
				t0 := time.Now()
				if mode == "blur-only" {
					dc.ApplyBlur((800.0 / 600.0) * c.Elevation)
				} else {
					dc.DrawShadow(path, render.RGBA{R: 0, G: 0, B: 0, A: 1}, c.Elevation, false)
				}
				_ = dc.Image() // force materialize of any published filter result
				dt := time.Since(t0)
				st := dc.RenderPathStats()
				t.Logf("P43ROUGH %dx%d %s total=%.1fms gpu_ops=%d cpu_fb=%d %s",
					c.W, c.H, mode, float64(dt)/float64(time.Millisecond),
					st.GPUOps, st.CPUFallbackOps, st.LogLine())
			})
		}
	}
}
