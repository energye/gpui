//go:build !nogpu

package render_test

// S5.1–S5.4 harness: present-only baseline, frame model, 60fps gate, P0 smoke.
//
// Timed path = draw + PresentFrame/PresentFrameDamageRects (FlushGPUWithView*).
// No ReadPixels in the timed path.
//
// Env:
//   WGPU_NATIVE_PATH   required
//   S5_PERF_WARMUP     default 3
//   S5_PERF_ITERS      default 10
//   S5_PERF_JSON       default tmp/s5_present_baseline.json
//   S5_FPS_BUDGET_MS   default 16.7 (gate uses p50)
//   S5_ALLOW_SLOW=1    soft-pass gate on overloaded hosts

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/render"
)

type s5SceneResult struct {
	Name           string  `json:"name"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Warmup         int     `json:"warmup"`
	Iters          int     `json:"iters"`
	Retained       bool    `json:"retained"`
	Damage         bool    `json:"damage"`
	TotalMsAvg     float64 `json:"total_ms_avg"`
	TotalMsP50     float64 `json:"total_ms_p50"`
	TotalMsP95     float64 `json:"total_ms_p95"`
	TotalMsMin     float64 `json:"total_ms_min"`
	TotalMsMax     float64 `json:"total_ms_max"`
	DrawMsAvg      float64 `json:"draw_ms_avg"`
	PresentMsAvg   float64 `json:"present_ms_avg"`
	FpsEst         float64 `json:"fps_est"`
	FpsP50         float64 `json:"fps_p50"`
	GPUOps         int     `json:"gpu_ops"`
	CPUFallbackOps int     `json:"cpu_fallback_ops"`
	Path           string  `json:"path"`
}

type s5BaselineFile struct {
	Version  string          `json:"version"`
	Date     string          `json:"date"`
	GOOS     string          `json:"goos"`
	GOARCH   string          `json:"goarch"`
	NumCPU   int             `json:"num_cpu"`
	Hostname string          `json:"hostname"`
	WGPUPath string          `json:"wgpu_native_path"`
	Note     string          `json:"note"`
	Scenes   []s5SceneResult `json:"scenes"`
}

type s5Scene struct {
	Name      string
	W, H      int
	NeedFont  bool
	Retained  bool
	Damage    bool
	Draw      func(t *testing.T, dc *render.Context, font string)
	Bootstrap func(t *testing.T, dc *render.Context, font string)
}

func s5EnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n < 0 {
		return def
	}
	return n
}

func s5EnvFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil || f <= 0 {
		return def
	}
	return f
}

func s5Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

func s5Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	return cp[int(float64(len(cp)-1)*p)]
}

func s5Solid(dc *render.Context, r, g, b, a, x, y, w, h float64) {
	dc.SetRGBA(r, g, b, a)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

func s5MeasurePresent(t *testing.T, sc s5Scene, warmup, iters int) s5SceneResult {
	t.Helper()
	font := ""
	if sc.NeedFont {
		font = p1FindFont(t)
	}

	// Shared retained context + offscreen target.
	dc := render.NewContext(sc.W, sc.H)
	defer dc.Close()
	if sc.NeedFont {
		if err := dc.LoadFontFace(font, 13); err != nil {
			t.Fatalf("%s LoadFontFace: %v", sc.Name, err)
		}
	}
	view, rel := dc.CreateOffscreenTexture(sc.W, sc.H)
	if rel == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer rel()

	boot := sc.Bootstrap
	if boot == nil {
		boot = sc.Draw
	}

	// Probe (untimed): full bootstrap present, GPUOps>0.
	dc.ResetRenderPathStats()
	p1White(dc, sc.W, sc.H)
	boot(t, dc, font)
	if err := dc.PresentFrame(view, uint32(sc.W), uint32(sc.H), func() error { return nil }); err != nil {
		t.Fatalf("%s probe PresentFrame: %v", sc.Name, err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatalf("%s probe GPUOps==0: %s", sc.Name, dc.RenderPathStats().LogLine())
	}

	// Fresh context for cold scenes; retained reuses dc after bootstrap above.
	totals := make([]float64, 0, iters)
	draws := make([]float64, 0, iters)
	presents := make([]float64, 0, iters)
	var lastGPU, lastCPU int

	for i := 0; i < warmup+iters; i++ {
		var frameDC *render.Context
		var frameView = view
		var freeView func()
		var closeDC bool

		if sc.Retained {
			frameDC = dc
			// Steady-state: no full wipe (S5.2). Damage draws only issue dirty cmds.
		} else {
			frameDC = render.NewContext(sc.W, sc.H)
			closeDC = true
			if sc.NeedFont {
				_ = frameDC.LoadFontFace(font, 13)
			}
			v, r := frameDC.CreateOffscreenTexture(sc.W, sc.H)
			if r == nil || v.IsNil() {
				frameDC.Close()
				t.Fatalf("%s CreateOffscreenTexture failed", sc.Name)
			}
			frameView = v
			freeView = r
		}

		frameDC.ResetRenderPathStats()
		if sc.Damage {
			frameDC.ResetFrameDamage()
		}

		t0 := time.Now()
		sc.Draw(t, frameDC, font)
		t1 := time.Now()

		var err error
		if sc.Damage {
			rects := frameDC.FrameDamage()
			if len(rects) == 0 {
				rects = []image.Rectangle{{Min: image.Pt(0, 0), Max: image.Pt(sc.W, sc.H)}}
			}
			err = frameDC.PresentFrameDamageRects(frameView, uint32(sc.W), uint32(sc.H), rects, func() error { return nil })
		} else {
			err = frameDC.PresentFrame(frameView, uint32(sc.W), uint32(sc.H), func() error { return nil })
		}
		t2 := time.Now()
		if err != nil {
			if freeView != nil {
				freeView()
			}
			if closeDC {
				frameDC.Close()
			}
			t.Fatalf("%s Present: %v", sc.Name, err)
		}
		st := frameDC.RenderPathStats()
		lastGPU = st.GPUOps
		lastCPU = st.CPUFallbackOps
		if freeView != nil {
			freeView()
		}
		if closeDC {
			frameDC.Close()
		}
		if st.GPUOps == 0 {
			t.Fatalf("%s GPUOps==0: %s", sc.Name, st.LogLine())
		}
		if i < warmup {
			continue
		}
		totals = append(totals, t2.Sub(t0).Seconds()*1000)
		draws = append(draws, t1.Sub(t0).Seconds()*1000)
		presents = append(presents, t2.Sub(t1).Seconds()*1000)
	}

	avg := s5Mean(totals)
	p50 := s5Percentile(totals, 0.50)
	fps, fpsP50 := 0.0, 0.0
	if avg > 0 {
		fps = 1000.0 / avg
	}
	if p50 > 0 {
		fpsP50 = 1000.0 / p50
	}
	return s5SceneResult{
		Name:           sc.Name,
		Width:          sc.W,
		Height:         sc.H,
		Warmup:         warmup,
		Iters:          iters,
		Retained:       sc.Retained,
		Damage:         sc.Damage,
		TotalMsAvg:     avg,
		TotalMsP50:     p50,
		TotalMsP95:     s5Percentile(totals, 0.95),
		TotalMsMin:     s5Percentile(totals, 0),
		TotalMsMax:     s5Percentile(totals, 1),
		DrawMsAvg:      s5Mean(draws),
		PresentMsAvg:   s5Mean(presents),
		FpsEst:         fps,
		FpsP50:         fpsP50,
		GPUOps:         lastGPU,
		CPUFallbackOps: lastCPU,
		Path:           "present-only-offscreen",
	}
}

func s5Scenes() []s5Scene {
	return []s5Scene{
		{
			Name: "P01_SolidPresent",
			W:    640, H: 400,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.12, 0.14, 0.18, 1, 0, 0, 640, 400)
			},
		},
		{
			// Static shell: bootstrap chrome; steady frame refreshes status chip (damage).
			Name:     "U01_StaticShell",
			W:        800,
			H:        480,
			NeedFont: true,
			Retained: true,
			Damage:   true,
			Bootstrap: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.10, 0.11, 0.14, 1, 0, 0, 800, 480)
				s5Solid(dc, 0.16, 0.18, 0.24, 1, 0, 0, 800, 44)
				s5Solid(dc, 0.14, 0.15, 0.20, 1, 0, 44, 180, 436)
				s5Solid(dc, 0.95, 0.96, 0.98, 1, 180, 44, 620, 436)
				dc.SetRGB(0.92, 0.93, 0.95)
				dc.DrawString("App Title", 16, 28)
				for i := 0; i < 5; i++ {
					dc.SetRGB(0.7, 0.72, 0.78)
					dc.DrawString(fmt.Sprintf("nav-%02d", i), 20, 80+float64(i)*48)
				}
				dc.SetRGB(0.2, 0.45, 0.9)
				dc.DrawRoundedRectangle(200, 60, 140, 32, 6)
				_ = dc.Fill()
				dc.SetRGB(1, 1, 1)
				dc.DrawString("Primary", 236, 82)
				for i := 0; i < 3; i++ {
					dc.SetRGB(1, 1, 1)
					dc.DrawRoundedRectangle(200, 110+float64(i)*90, 560, 72, 8)
					_ = dc.Fill()
					dc.SetRGB(0.2, 0.22, 0.26)
					dc.DrawString(fmt.Sprintf("Content card %d", i), 220, 150+float64(i)*90)
				}
			},
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				dc.ClipRect(680, 8, 100, 28)
				s5Solid(dc, 0.16, 0.18, 0.24, 1, 680, 8, 100, 28)
				dc.SetRGB(0.4, 0.85, 0.5)
				dc.DrawCircle(696, 22, 6)
				_ = dc.Fill()
				dc.SetRGB(0.9, 0.92, 0.95)
				dc.DrawString("online", 710, 26)
				dc.ResetClip()
			},
		},
		{
			// Bootstrap full list; steady frames only dirty 3-row band.
			Name: "U02_ListScrollMorph",
			W:    400, H: 560, NeedFont: true, Retained: true, Damage: true,
			Bootstrap: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.97, 0.97, 0.98, 1, 0, 0, 400, 560)
				s5Solid(dc, 0.15, 0.16, 0.2, 1, 0, 0, 400, 40)
				dc.SetRGB(0.95, 0.96, 0.98)
				dc.DrawString("List scroll", 12, 26)
				for i := 0; i < 14; i++ {
					y := 48 + float64(i)*34
					s5Solid(dc, 1, 1, 1, 1, 0, y, 400, 34)
					dc.SetRGB(0.25, 0.55, 0.95)
					dc.DrawCircle(22, y+17, 10)
					_ = dc.Fill()
					dc.SetRGB(0.15, 0.16, 0.2)
					dc.DrawString(fmt.Sprintf("Row item %02d", i), 44, y+22)
				}
			},
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				const y0, bandH = 180.0, 102.0
				dc.ClipRect(0, y0, 400, bandH)
				for i := 0; i < 3; i++ {
					y := y0 + float64(i)*34
					s5Solid(dc, 1, 1, 1, 1, 0, y, 400, 34)
					dc.SetRGB(0.25, 0.55, 0.95)
					dc.DrawCircle(22, y+17, 10)
					_ = dc.Fill()
					dc.SetRGB(0.15, 0.16, 0.2)
					dc.DrawString(fmt.Sprintf("Row scroll %02d", i), 44, y+22)
				}
				dc.ResetClip()
			},
		},
		{
			Name: "U03_FormFieldDamage",
			W:    400, H: 300, NeedFont: true, Retained: true, Damage: true,
			Bootstrap: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.96, 0.97, 0.98, 1, 0, 0, 400, 300)
				s5Solid(dc, 1, 1, 1, 1, 20, 20, 360, 250)
				dc.SetRGB(0.2, 0.22, 0.26)
				dc.DrawString("Username", 36, 52)
				dc.SetRGB(0.9, 0.91, 0.93)
				dc.DrawRoundedRectangle(36, 60, 320, 32, 4)
				_ = dc.Fill()
				dc.SetRGB(0.2, 0.22, 0.26)
				dc.DrawString("Password", 36, 140)
				dc.SetRGB(0.9, 0.91, 0.93)
				dc.DrawRoundedRectangle(36, 148, 320, 32, 4)
				_ = dc.Fill()
			},
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				dc.ClipRect(36, 60, 320, 56)
				s5Solid(dc, 1, 1, 1, 1, 36, 60, 320, 32)
				dc.SetRGB(0.15, 0.16, 0.2)
				dc.DrawString("user@example.com", 48, 82)
				dc.SetRGB(0.85, 0.25, 0.2)
				dc.DrawString("invalid format", 48, 108)
				dc.ResetClip()
			},
		},
		{
			Name: "U04_ModalStatic",
			W:    480, H: 320, NeedFont: true, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.2, 0.22, 0.28, 1, 0, 0, 480, 320)
				dc.SetRGB(1, 1, 1)
				dc.DrawRoundedRectangle(80, 40, 320, 220, 10)
				_ = dc.Fill()
				dc.SetRGB(0.15, 0.16, 0.2)
				dc.DrawString("Confirm action", 110, 90)
				dc.DrawString("Modal body static frame.", 110, 130)
				dc.SetRGB(0.2, 0.45, 0.9)
				dc.DrawRoundedRectangle(240, 200, 100, 32, 6)
				_ = dc.Fill()
				dc.SetRGB(1, 1, 1)
				dc.DrawString("OK", 276, 222)
			},
		},
		{
			Name: "U05_KitchenSinkStress",
			W:    480, H: 320, NeedFont: true, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.97, 0.97, 0.98, 1, 0, 0, 480, 320)
				for i := 0; i < 8; i++ {
					dc.PushLayer(render.BlendNormal, 0.9)
					dc.SetRGB(0.2+float64(i%4)*0.12, 0.35, 0.75)
					dc.DrawRoundedRectangle(16+float64(i%4)*115, 16+float64(i/4)*140, 100, 110, 8)
					_ = dc.Fill()
					dc.SetRGB(1, 1, 1)
					dc.DrawString(fmt.Sprintf("k%d", i), 40+float64(i%4)*115, 70+float64(i/4)*140)
					dc.PopLayer()
				}
				dc.SetBlendMode(render.BlendMultiply)
				dc.SetRGB(1, 0.5, 0.3)
				dc.DrawCircle(240, 160, 60)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)
			},
		},
		{
			Name: "B15like_MultiDamage",
			W:    320, H: 200, NeedFont: true, Retained: true, Damage: true,
			Bootstrap: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.15, 0.16, 0.2, 1, 0, 0, 320, 36)
				dc.SetRGB(0.95, 0.96, 0.98)
				dc.DrawString("damage retained", 12, 24)
				dc.SetRGB(0.88, 0.90, 0.93)
				dc.DrawRoundedRectangle(16, 52, 130, 120, 8)
				_ = dc.Fill()
				dc.DrawRoundedRectangle(174, 52, 130, 120, 8)
				_ = dc.Fill()
			},
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				dc.ClipRect(16, 52, 130, 120)
				s5Solid(dc, 0.20, 0.45, 0.90, 1, 16, 52, 130, 120)
				dc.SetRGB(1, 1, 1)
				dc.DrawString("A*", 66, 120)
				dc.ResetClip()
				dc.ClipRect(174, 52, 130, 120)
				s5Solid(dc, 0.15, 0.65, 0.40, 1, 174, 52, 130, 120)
				dc.SetRGB(1, 1, 1)
				dc.DrawString("B*", 226, 120)
				dc.ResetClip()
			},
		},
	}
}

func TestS5_PresentBaseline_Scenes(t *testing.T) {
	p1RequireGPU(t)
	warmup := s5EnvInt("S5_PERF_WARMUP", 3)
	iters := s5EnvInt("S5_PERF_ITERS", 10)
	if iters < 1 {
		iters = 1
	}

	host, _ := os.Hostname()
	out := s5BaselineFile{
		Version:  "s5.1-present-1",
		Date:     time.Now().Format(time.RFC3339),
		GOOS:     runtime.GOOS,
		GOARCH:   runtime.GOARCH,
		NumCPU:   runtime.NumCPU(),
		Hostname: host,
		WGPUPath: os.Getenv("WGPU_NATIVE_PATH"),
		Note:     "S5.1 present-only offscreen (FlushGPUWithView, no ReadPixels). Gate metric = p50.",
	}

	var results []s5SceneResult
	for _, sc := range s5Scenes() {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			res := s5MeasurePresent(t, sc, warmup, iters)
			results = append(results, res)
			t.Logf("%s p50=%.2f avg=%.2f present=%.2f fps_p50=%.1f gpu=%d cpu_fb=%d retained=%v damage=%v",
				res.Name, res.TotalMsP50, res.TotalMsAvg, res.PresentMsAvg, res.FpsP50, res.GPUOps, res.CPUFallbackOps, res.Retained, res.Damage)
			if res.CPUFallbackOps != 0 {
				t.Fatalf("cpu_fallback_ops=%d", res.CPUFallbackOps)
			}
		})
	}
	out.Scenes = results
	sort.Slice(out.Scenes, func(i, j int) bool { return out.Scenes[i].Name < out.Scenes[j].Name })

	path := os.Getenv("S5_PERF_JSON")
	if path == "" {
		// Prefer repo-root tmp/ even when `go test` cwd is the package dir.
		candidates := []string{
			filepath.Join("tmp", "s5_present_baseline.json"),
			filepath.Join("..", "tmp", "s5_present_baseline.json"),
		}
		path = candidates[0]
		if _, err := os.Stat("go.mod"); err != nil {
			if _, err2 := os.Stat(filepath.Join("..", "go.mod")); err2 == nil {
				path = candidates[1]
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("wrote %s (%d scenes)", path, len(out.Scenes))
}

func TestS52_FrameModel_RetainedDamage(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	// Bootstrap full present.
	p1White(dc, w, h)
	s5Solid(dc, 0.9, 0.91, 0.93, 1, 0, 0, w, h)
	if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Steady: reset damage → local draws → multi-rect present.
	dc.ResetFrameDamage()
	dc.ClipRect(16, 40, 120, 80)
	s5Solid(dc, 0.2, 0.5, 0.9, 1, 16, 40, 120, 80)
	dc.ResetClip()
	dc.ClipRect(180, 40, 120, 80)
	s5Solid(dc, 0.2, 0.7, 0.4, 1, 180, 40, 120, 80)
	dc.ResetClip()
	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected FrameDamage")
	}
	if dc.FrameDamageUnion().Empty() {
		t.Fatal("FrameDamageUnion empty")
	}
	if err := dc.PresentFrameDamageRects(view, uint32(w), uint32(h), rects, nil); err != nil {
		t.Fatalf("damage present: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
	t.Logf("S5.2 ok damage=%v", rects)
}

func TestS53_MainPath60FPS_Gate(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S5_FPS_BUDGET_MS", 16.7)
	warmup := s5EnvInt("S5_PERF_WARMUP", 3)
	iters := s5EnvInt("S5_PERF_ITERS", 10)
	if iters < 5 {
		iters = 5
	}

	gated := []string{"U01_StaticShell", "U02_ListScrollMorph", "U03_FormFieldDamage", "U04_ModalStatic"}
	byName := map[string]s5Scene{}
	for _, sc := range s5Scenes() {
		byName[sc.Name] = sc
	}
	for _, name := range gated {
		sc := byName[name]
		t.Run(name, func(t *testing.T) {
			res := s5MeasurePresent(t, sc, warmup, iters)
			metric := res.TotalMsP50
			t.Logf("%s p50=%.2f avg=%.2f budget=%.2f fps_p50=%.1f", name, metric, res.TotalMsAvg, budget, res.FpsP50)
			if res.CPUFallbackOps != 0 {
				t.Fatalf("cpu_fallback_ops=%d", res.CPUFallbackOps)
			}
			if metric > budget {
				if os.Getenv("S5_ALLOW_SLOW") == "1" {
					t.Logf("OVER budget soft-pass S5_ALLOW_SLOW=1: p50=%.2f > %.2f", metric, budget)
					return
				}
				t.Fatalf("present-only p50 %.2fms exceeds budget %.2fms (avg=%.2f)", metric, budget, res.TotalMsAvg)
			}
		})
	}
}

func TestS54_P0Capability_NoBlockingGap(t *testing.T) {
	p1RequireGPU(t)
	dc := render.NewContext(200, 120)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)
	dc.ResetRenderPathStats()
	p1White(dc, 200, 120)
	dc.SetRGB(0.2, 0.4, 0.9)
	dc.DrawRoundedRectangle(10, 10, 80, 40, 8)
	_ = dc.Fill()
	dc.ClipRect(100, 10, 90, 100)
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.DrawCircle(145, 55, 30)
	_ = dc.Fill()
	dc.ResetClip()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("P0 ok", 20, 90)
	view, rel := dc.CreateOffscreenTexture(200, 120)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()
	if err := dc.PresentFrame(view, 200, 120, nil); err != nil {
		t.Fatalf("PresentFrame: %v", err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
}
