//go:build !nogpu

package render_test

// S4.0 performance baseline — measure only, no algorithm changes.
// Scenes mirror P1/A pressure (D05/D06/D08/D36 + synthetic stress).
// Exit artifact: docs/S4_PERF_BASELINE.md (written from this harness output).
//
// Env:
//   WGPU_NATIVE_PATH  required for real native path
//   S4_PERF_WARMUP    default 3
//   S4_PERF_ITERS     default 20
//   S4_PERF_JSON      optional path for JSON dump (default <repo>/tmp/s4_baseline.json)

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

type s4SceneResult struct {
	Name           string  `json:"name"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Warmup         int     `json:"warmup"`
	Iters          int     `json:"iters"`
	TotalMsAvg     float64 `json:"total_ms_avg"`
	TotalMsP50     float64 `json:"total_ms_p50"`
	TotalMsP95     float64 `json:"total_ms_p95"`
	TotalMsMin     float64 `json:"total_ms_min"`
	TotalMsMax     float64 `json:"total_ms_max"`
	DrawMsAvg      float64 `json:"draw_ms_avg"`
	FlushMsAvg     float64 `json:"flush_ms_avg"`
	FPS            float64 `json:"fps_est"`
	GPUOps         int     `json:"gpu_ops"`
	CPUFallbackOps int     `json:"cpu_fallback_ops"`
	UploadCount    string  `json:"upload_count"` // N/A until counters exist
	DrawCount      string  `json:"draw_count"`   // N/A until counters exist
}

type s4BaselineReport struct {
	Version  string          `json:"version"`
	Date     string          `json:"date"`
	GOOS     string          `json:"goos"`
	GOARCH   string          `json:"goarch"`
	NumCPU   int             `json:"num_cpu"`
	Hostname string          `json:"hostname,omitempty"`
	WGPUPath string          `json:"wgpu_native_path"`
	Note     string          `json:"note"`
	Scenes   []s4SceneResult `json:"scenes"`
}

func s4EnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func s4RepoTmp(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			out := filepath.Join(dir, "tmp")
			if err := os.MkdirAll(out, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", out, err)
			}
			return out
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	out := filepath.Join(wd, "tmp")
	_ = os.MkdirAll(out, 0o755)
	return out
}

func s4Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func s4Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

type s4Scene struct {
	Name     string
	W, H     int
	NeedFont bool
	// Retained reuses one Context across measured frames (S4.4).
	// First warmup frames still cold-start; measured iters amortize caches.
	Retained bool
	// Draw builds one frame. May call intermediate FlushGPU (backdrop scenes).
	Draw func(t *testing.T, dc *render.Context, font string)
}

func s4Measure(t *testing.T, sc s4Scene, warmup, iters int) s4SceneResult {
	t.Helper()
	font := ""
	if sc.NeedFont {
		font = p1FindFont(t)
	}

	dc := render.NewContext(sc.W, sc.H)
	defer dc.Close()
	if sc.NeedFont {
		if err := dc.LoadFontFace(font, 13); err != nil {
			t.Fatalf("%s LoadFontFace: %v", sc.Name, err)
		}
	}

	// One-shot probe to ensure GPU path works for this scene shape.
	dc.ResetRenderPathStats()
	sc.Draw(t, dc, font)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("%s probe FlushGPU: %v", sc.Name, err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatalf("%s probe requires GPUOps>0: %s", sc.Name, dc.RenderPathStats().LogLine())
	}

	totals := make([]float64, 0, iters)
	draws := make([]float64, 0, iters)
	flushes := make([]float64, 0, iters)
	var lastStats render.RenderPathStats

	var retainedDC *render.Context
	if sc.Retained {
		retainedDC = render.NewContext(sc.W, sc.H)
		defer retainedDC.Close()
		if sc.NeedFont {
			_ = retainedDC.LoadFontFace(font, 13)
		}
	}

	for i := 0; i < warmup+iters; i++ {
		var frameDC *render.Context
		owned := false
		if sc.Retained {
			frameDC = retainedDC
		} else {
			// Fresh context each frame: S4.0 cold rebuild baseline.
			frameDC = render.NewContext(sc.W, sc.H)
			owned = true
			if sc.NeedFont {
				_ = frameDC.LoadFontFace(font, 13)
			}
		}
		frameDC.ResetRenderPathStats()
		if sc.Retained {
			// Clear full canvas between retained frames.
			p1White(frameDC, sc.W, sc.H)
		}

		t0 := time.Now()
		sc.Draw(t, frameDC, font)
		t1 := time.Now()
		if err := frameDC.FlushGPU(); err != nil {
			if owned {
				frameDC.Close()
			}
			t.Fatalf("%s FlushGPU iter=%d: %v", sc.Name, i, err)
		}
		t2 := time.Now()
		stats := frameDC.RenderPathStats()
		if owned {
			frameDC.Close()
		}

		if stats.GPUOps == 0 {
			t.Fatalf("%s iter=%d GPUOps==0: %s", sc.Name, i, stats.LogLine())
		}
		if i < warmup {
			continue
		}
		totals = append(totals, t2.Sub(t0).Seconds()*1000)
		draws = append(draws, t1.Sub(t0).Seconds()*1000)
		flushes = append(flushes, t2.Sub(t1).Seconds()*1000)
		lastStats = stats
	}

	sorted := append([]float64(nil), totals...)
	sort.Float64s(sorted)
	avg := s4Mean(totals)
	fps := 0.0
	if avg > 0 {
		fps = 1000.0 / avg
	}
	minV, maxV := sorted[0], sorted[len(sorted)-1]

	return s4SceneResult{
		Name:           sc.Name,
		Width:          sc.W,
		Height:         sc.H,
		Warmup:         warmup,
		Iters:          iters,
		TotalMsAvg:     avg,
		TotalMsP50:     s4Percentile(sorted, 0.50),
		TotalMsP95:     s4Percentile(sorted, 0.95),
		TotalMsMin:     minV,
		TotalMsMax:     maxV,
		DrawMsAvg:      s4Mean(draws),
		FlushMsAvg:     s4Mean(flushes),
		FPS:            fps,
		GPUOps:         lastStats.GPUOps,
		CPUFallbackOps: lastStats.CPUFallbackOps,
		UploadCount:    "N/A",
		DrawCount:      "N/A",
	}
}

func s4SolidRGBA(dc *render.Context, r, g, b, a float64, x, y, w, h float64) {
	dc.SetRGBA(r, g, b, a)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

func s4Scenes() []s4Scene {
	return []s4Scene{
		{
			Name: "B01_SolidFill",
			W:    800, H: 600,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 800, 600)
				s4SolidRGBA(dc, 0.2, 0.4, 0.8, 1, 0, 0, 800, 600)
			},
		},
		{
			Name: "B02_ManyRects200",
			W:    800, H: 600,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 800, 600)
				for i := 0; i < 200; i++ {
					x := float64((i*37)%760) + 10
					y := float64((i*53)%560) + 10
					s4SolidRGBA(dc, float64(i%5)/5, 0.3, float64(i%7)/7, 0.85, x, y, 28, 18)
				}
			},
		},
		{
			Name: "B03_TextRows40",
			W:    640, H: 480,
			NeedFont: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 640, 480)
				s4SolidRGBA(dc, 0.96, 0.97, 0.98, 1, 0, 0, 640, 480)
				dc.SetRGB(0.12, 0.13, 0.16)
				for i := 0; i < 40; i++ {
					dc.DrawString(fmt.Sprintf("S4 text pressure row %02d — glyph path baseline", i), 16, 18+float64(i)*11)
				}
			},
		},
		{
			Name: "B04_D05_LayerBlendClip",
			W:    280, H: 200,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 280, 200)
				s4SolidRGBA(dc, 0.70, 0.90, 0.95, 1, 0, 0, 280, 200)
				dc.ClipRect(40, 30, 200, 140)
				dc.PushLayer(render.BlendMultiply, 1.0)
				s4SolidRGBA(dc, 1.0, 0.55, 0.55, 1, 40, 30, 200, 140)
				dc.PushLayer(render.BlendNormal, 0.7)
				dc.SetRGB(0.15, 0.25, 0.85)
				dc.DrawRoundedRectangle(70, 60, 120, 70, 12)
				_ = dc.Fill()
				dc.PopLayer()
				dc.PopLayer()
			},
		},
		{
			Name: "B05_D06_ImageTextClipBackdrop",
			W:    360, H: 240,
			NeedFont: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 360, 240)
				s4SolidRGBA(dc, 0.96, 0.97, 0.98, 1, 0, 0, 360, 240)
				img := compMakeImage(t, 48, 48, 40, 140, 220)
				for i := 0; i < 4; i++ {
					x := 24 + float64(i)*80
					dc.ClipRect(x, 40, 64, 64)
					dc.DrawImage(img, x+8, 48)
					dc.ResetClip()
					dc.SetRGB(0.15, 0.16, 0.2)
					dc.DrawString(fmt.Sprintf("tile-%d", i), x+8, 128)
				}
				// intermediate flush like D06
				if err := dc.FlushGPU(); err != nil {
					t.Fatalf("B05 pre-backdrop flush: %v", err)
				}
				dc.PushBackdropLayer(render.BlendNormal, 1)
				s4SolidRGBA(dc, 0, 0, 0, 0.40, 0, 0, 360, 240)
				dc.SetRGB(1, 1, 1)
				dc.DrawRoundedRectangle(80, 50, 200, 140, 12)
				_ = dc.Fill()
				dc.SetRGB(0.12, 0.12, 0.15)
				dc.DrawString("backdrop×image×text", 100, 120)
				dc.PopLayer()
			},
		},
		{
			Name: "B06_D08_MultiRegionRedraw",
			W:    320, H: 200,
			NeedFont: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 320, 200)
				s4SolidRGBA(dc, 0.15, 0.16, 0.2, 1, 0, 0, 320, 36)
				dc.SetRGB(0.95, 0.96, 0.98)
				dc.DrawString("multi-region redraw", 12, 24)
				dc.SetRGB(0.88, 0.90, 0.93)
				dc.DrawRoundedRectangle(16, 52, 130, 120, 8)
				_ = dc.Fill()
				dc.DrawRoundedRectangle(174, 52, 130, 120, 8)
				_ = dc.Fill()
				dc.SetRGB(0.2, 0.22, 0.26)
				dc.DrawString("A", 70, 120)
				dc.DrawString("B", 230, 120)
				if err := dc.FlushGPU(); err != nil {
					t.Fatalf("B06 base flush: %v", err)
				}
				dc.ClipRect(16, 52, 130, 120)
				s4SolidRGBA(dc, 0.20, 0.45, 0.90, 1, 16, 52, 130, 120)
				dc.SetRGB(1, 1, 1)
				dc.DrawString("A*", 66, 120)
				dc.ResetClip()
				dc.ClipRect(174, 52, 130, 120)
				s4SolidRGBA(dc, 0.15, 0.65, 0.40, 1, 174, 52, 130, 120)
				dc.SetRGB(1, 1, 1)
				dc.DrawString("B*", 226, 120)
				dc.ResetClip()
			},
		},
		{
			Name: "B07_D36_KitchenSinkMaxMix",
			W:    560, H: 400,
			NeedFont: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				const w, h = 560, 400
				p1White(dc, w, h)
				dc.Push()
				dc.Translate(10, 10)
				dc.Scale(1.1, 1.1)
				grad := render.NewLinearGradientBrush(0, 0, 200, 0).
					AddColorStop(0, render.RGB(0.2, 0.3, 0.6)).
					AddColorStop(1, render.RGB(0.6, 0.3, 0.5))
				dc.SetFillBrush(grad)
				dc.DrawRoundedRectangle(0, 0, 200, 80, 10)
				_ = dc.Fill()
				dc.Pop()

				mask := render.NewMask(w, h)
				compFillMaskRect(mask, 220, 20, 520, 120, 255)
				dc.SetMask(mask)
				s4SolidRGBA(dc, 0.2, 0.75, 0.55, 1, 220, 20, 300, 100)
				dc.ClearMask()

				dc.DrawMesh(render.Mesh{
					Positions: []render.Point{{X: 30, Y: 120}, {X: 120, Y: 110}, {X: 90, Y: 200}, {X: 20, Y: 190}},
					Colors: []render.RGBA{
						{R: 1, G: 0.3, B: 0.2, A: 1}, {R: 0.3, G: 1, B: 0.3, A: 1},
						{R: 0.2, G: 0.4, B: 1, A: 1}, {R: 1, G: 1, B: 0.2, A: 1},
					},
					Indices: []uint16{0, 1, 2, 0, 2, 3},
				})

				atlas := compMakeImage(t, 48, 24, 0, 0, 0)
				for y := 0; y < 24; y++ {
					for x := 0; x < 24; x++ {
						_ = atlas.SetRGBA(x, y, 240, 80, 40, 255)
						_ = atlas.SetRGBA(x+24, y, 40, 120, 240, 255)
					}
				}
				dc.DrawAtlas(atlas, []render.AtlasSprite{
					{SrcX: 0, SrcY: 0, SrcW: 24, SrcH: 24, DstX: 150, DstY: 130, DstW: 28, DstH: 28},
					{SrcX: 24, SrcY: 0, SrcW: 24, SrcH: 24, DstX: 190, DstY: 140, DstW: 32, DstH: 32},
				})

				p := render.NewPath()
				p.MoveTo(240, 140)
				p.LineTo(360, 130)
				p.LineTo(380, 220)
				p.LineTo(250, 230)
				p.Close()
				dc.SetRGB(0.15, 0.45, 0.9)
				dc.SetLineWidth(2)
				dc.SetDash(6, 4)
				dc.AppendPath(p.WithCorners(10))
				_ = dc.Stroke()
				dc.SetDash()

				dc.ClipRect(20, 250, 300, 130)
				s4SolidRGBA(dc, 0.95, 0.96, 0.98, 1, 20, 250, 300, 130)
				for i := 0; i < 6; i++ {
					dc.SetRGB(0.15, 0.16, 0.2)
					dc.DrawString(fmt.Sprintf("sink-row-%d clip×text×list", i), 32, 270+float64(i)*18)
				}
				dc.ResetClip()

				dc.SetBlendMode(render.BlendMultiply)
				dc.SetRGB(1, 0.6, 0.6)
				dc.DrawCircle(450, 200, 50)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendScreen)
				dc.SetRGB(0.4, 0.5, 1)
				dc.DrawCircle(480, 230, 45)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)

				if err := dc.FlushGPU(); err != nil {
					t.Fatalf("B07 pre-backdrop: %v", err)
				}
				dc.PushBackdropLayer(render.BlendNormal, 1)
				s4SolidRGBA(dc, 0, 0, 0, 0.3, 0, 0, w, h)
				dc.SetRGB(1, 1, 1)
				dc.DrawRoundedRectangle(160, 100, 240, 140, 12)
				_ = dc.Fill()
				dc.SetRGB(0.12, 0.12, 0.15)
				dc.DrawString("kitchen-sink modal", 190, 170)
				dc.PopLayer()
			},
		},
		{
			Name: "B08_ListScrollMorphology",
			W:    480, H: 640,
			NeedFont: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				const w, h = 480, 640
				p1White(dc, w, h)
				s4SolidRGBA(dc, 0.12, 0.13, 0.16, 1, 0, 0, w, 48)
				dc.SetRGB(0.95, 0.96, 0.98)
				dc.DrawString("S4 list morphology", 16, 30)
				dc.ClipRect(0, 48, w, h-48)
				for i := 0; i < 24; i++ {
					y := 56 + float64(i)*24
					if i%2 == 0 {
						s4SolidRGBA(dc, 0.97, 0.98, 0.99, 1, 8, y, w-16, 22)
					} else {
						s4SolidRGBA(dc, 0.93, 0.94, 0.96, 1, 8, y, w-16, 22)
					}
					dc.SetRGB(0.15, 0.16, 0.2)
					dc.DrawString(fmt.Sprintf("row %02d · avatar · title · meta · action", i), 48, y+15)
					dc.SetRGB(0.3, 0.55, 0.95)
					dc.DrawCircle(26, y+11, 8)
					_ = dc.Fill()
				}
				dc.ResetClip()
			},
		},
		{
			Name: "B09_BlendStackPressure",
			W:    400, H: 400,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 400, 400)
				s4SolidRGBA(dc, 0.9, 0.9, 0.92, 1, 0, 0, 400, 400)
				modes := []render.BlendMode{
					render.BlendNormal, render.BlendMultiply, render.BlendScreen,
					render.BlendOverlay, render.BlendDarken, render.BlendLighten,
				}
				for i := 0; i < 36; i++ {
					dc.SetBlendMode(modes[i%len(modes)])
					dc.SetRGBA(float64(i%5)/5, float64(i%3)/3, float64(i%7)/7, 0.55)
					cx := 60 + float64(i%6)*55
					cy := 60 + float64(i/6)*55
					dc.DrawCircle(cx, cy, 28)
					_ = dc.Fill()
				}
				dc.SetBlendMode(render.BlendNormal)
			},
		},
		{
			Name: "B10_ImageTileGrid",
			W:    512, H: 512,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 512, 512)
				img := compMakeImage(t, 32, 32, 30, 140, 220)
				for row := 0; row < 8; row++ {
					for col := 0; col < 8; col++ {
						x := float64(col*60 + 16)
						y := float64(row*60 + 16)
						dc.ClipRect(x, y, 48, 48)
						dc.DrawImage(img, x+8, y+8)
						dc.ResetClip()
						dc.SetRGBA(1, 0.3, 0.2, 0.35)
						dc.DrawRoundedRectangle(x, y, 48, 48, 6)
						_ = dc.Fill()
					}
				}
			},
		},
		{
			Name: "B11_StressNestedClipLayerText",
			W:    640, H: 480,
			NeedFont: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				const w, h = 640, 480
				p1White(dc, w, h)
				s4SolidRGBA(dc, 0.94, 0.95, 0.97, 1, 0, 0, w, h)
				for i := 0; i < 8; i++ {
					inset := float64(i * 18)
					dc.ClipRect(20+inset, 20+inset, w-40-2*inset, h-40-2*inset)
					dc.PushLayer(render.BlendNormal, 0.92)
					s4SolidRGBA(dc, 0.2+float64(i)*0.05, 0.3, 0.7-float64(i)*0.04, 0.5, 20+inset, 20+inset, 120, 40)
					dc.SetRGB(0.1, 0.1, 0.12)
					dc.DrawString(fmt.Sprintf("nested clip/layer %d", i), 28+inset, 44+inset)
					dc.PopLayer()
				}
				for i := 0; i < 8; i++ {
					dc.ResetClip()
				}
			},
		},
		{
			// S4.1 input: same GenerationID tiles WITHOUT per-tile clip so
			// image multi-quad coalescing can collapse N draws → 1.
			Name: "B13_ImageBatchNoClip",
			W:    512, H: 512,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 512, 512)
				img := compMakeImage(t, 32, 32, 30, 140, 220)
				for row := 0; row < 8; row++ {
					for col := 0; col < 8; col++ {
						x := float64(col*60 + 16)
						y := float64(row*60 + 16)
						dc.DrawImage(img, x+8, y+8)
					}
				}
			},
		},
		{
			Name: "B12_PathStrokeDashCloud",
			W:    480, H: 360,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				p1White(dc, 480, 360)
				s4SolidRGBA(dc, 0.98, 0.98, 0.99, 1, 0, 0, 480, 360)
				dc.SetLineWidth(1.5)
				for i := 0; i < 40; i++ {
					p := render.NewPath()
					x0 := 20 + float64(i%8)*55
					y0 := 30 + float64(i/8)*60
					p.MoveTo(x0, y0)
					p.LineTo(x0+40, y0+10)
					p.LineTo(x0+30, y0+40)
					p.LineTo(x0-5, y0+35)
					p.Close()
					dc.SetRGB(float64(i%5)/6, 0.25, float64(i%7)/8)
					dc.SetDash(4, 3)
					dc.AppendPath(p)
					_ = dc.Stroke()
				}
				dc.SetDash()
			},
		},
		{
			// S4.4 retained: reuse Context so path/glyph/image caches hit across frames.
			Name:     "B14_RetainedPathText",
			W:        480,
			H:        360,
			NeedFont: true,
			Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s4SolidRGBA(dc, 0.97, 0.97, 0.98, 1, 0, 0, 480, 360)
				for i := 0; i < 20; i++ {
					p := render.NewPath()
					x0 := 20 + float64(i%5)*90
					y0 := 30 + float64(i/5)*70
					p.MoveTo(x0, y0)
					p.LineTo(x0+50, y0+5)
					p.LineTo(x0+40, y0+45)
					p.Close()
					dc.SetRGB(0.2, 0.35, 0.8)
					dc.SetLineWidth(2)
					dc.AppendPath(p)
					_ = dc.Stroke()
				}
				dc.SetRGB(0.12, 0.12, 0.15)
				for i := 0; i < 12; i++ {
					dc.DrawString(fmt.Sprintf("retained-row-%02d cache", i), 24, 280+float64(i)*6)
				}
			},
		},
		{
			// S4.4 damage: multi-region dirty redraw on retained context.
			Name:     "B15_RetainedMultiDamage",
			W:        320,
			H:        200,
			NeedFont: true,
			Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s4SolidRGBA(dc, 0.15, 0.16, 0.2, 1, 0, 0, 320, 36)
				dc.SetRGB(0.95, 0.96, 0.98)
				dc.DrawString("damage retained", 12, 24)
				dc.SetRGB(0.88, 0.90, 0.93)
				dc.DrawRoundedRectangle(16, 52, 130, 120, 8)
				_ = dc.Fill()
				dc.DrawRoundedRectangle(174, 52, 130, 120, 8)
				_ = dc.Fill()
				if err := dc.FlushGPU(); err != nil {
					t.Fatalf("B15 base flush: %v", err)
				}
				dc.ResetFrameDamage()
				dc.ClipRect(16, 52, 130, 120)
				s4SolidRGBA(dc, 0.20, 0.45, 0.90, 1, 16, 52, 130, 120)
				dc.SetRGB(1, 1, 1)
				dc.DrawString("A*", 66, 120)
				dc.ResetClip()
				dc.ClipRect(174, 52, 130, 120)
				s4SolidRGBA(dc, 0.15, 0.65, 0.40, 1, 174, 52, 130, 120)
				dc.SetRGB(1, 1, 1)
				dc.DrawString("B*", 226, 120)
				dc.ResetClip()
				_ = dc.FrameDamage() // ensure tracking API exercised
			},
		},
	}
}

// TestS4_PerfBaseline_Scenes records S4.0 wall-time + path stats on representative scenes.
// Measure-only; does not change render algorithms.
func TestS4_PerfBaseline_Scenes(t *testing.T) {
	p1RequireGPU(t)

	warmup := s4EnvInt("S4_PERF_WARMUP", 3)
	iters := s4EnvInt("S4_PERF_ITERS", 20)
	if iters < 1 {
		iters = 1
	}

	host, _ := os.Hostname()
	report := s4BaselineReport{
		Version:  "s4.0-baseline-1",
		Date:     time.Now().Format(time.RFC3339),
		GOOS:     runtime.GOOS,
		GOARCH:   runtime.GOARCH,
		NumCPU:   runtime.NumCPU(),
		Hostname: host,
		WGPUPath: os.Getenv("WGPU_NATIVE_PATH"),
		Note:     "S4.0 measure-only. upload/draw counters N/A (only gpu_ops/cpu_fallback_ops). Default: fresh Context per frame; B14/B15 Retained=true reuses Context.",
	}

	scenes := s4Scenes()
	results := make([]s4SceneResult, 0, len(scenes))
	for _, sc := range scenes {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			r := s4Measure(t, sc, warmup, iters)
			results = append(results, r)
			t.Logf("S4.0 %s total_ms avg=%.3f p50=%.3f p95=%.3f fps=%.1f draw_ms=%.3f flush_ms=%.3f %s",
				r.Name, r.TotalMsAvg, r.TotalMsP50, r.TotalMsP95, r.FPS, r.DrawMsAvg, r.FlushMsAvg,
				fmt.Sprintf("gpu_ops=%d cpu_fallback_ops=%d", r.GPUOps, r.CPUFallbackOps))
		})
	}

	// Preserve scene order for report even if subtests ran serially (default).
	// Re-sort by original order if needed.
	order := map[string]int{}
	for i, sc := range scenes {
		order[sc.Name] = i
	}
	sort.SliceStable(results, func(i, j int) bool {
		return order[results[i].Name] < order[results[j].Name]
	})
	report.Scenes = results

	jsonPath := os.Getenv("S4_PERF_JSON")
	if jsonPath == "" {
		jsonPath = filepath.Join(s4RepoTmp(t), "s4_baseline.json")
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", jsonPath, err)
	}
	t.Logf("S4.0 baseline JSON: %s (%d scenes)", jsonPath, len(results))

	// Human table in test log
	t.Logf("--- S4.0 baseline summary ---")
	t.Logf("%-36s %8s %8s %8s %7s %6s %6s", "scene", "avg_ms", "p50_ms", "p95_ms", "fps", "gpu", "cpu_fb")
	for _, r := range results {
		t.Logf("%-36s %8.3f %8.3f %8.3f %7.1f %6d %6d",
			r.Name, r.TotalMsAvg, r.TotalMsP50, r.TotalMsP95, r.FPS, r.GPUOps, r.CPUFallbackOps)
	}
}
