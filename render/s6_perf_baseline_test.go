//go:build !nogpu

package render_test

// S6.0 deep performance baseline + regression lock (measure-only).
//
// Frozen present-only scene set = S5 U/P scenes + heavy H* scenes (full redraw /
// layer / path / text / image). Dual track: present-only primary; optional
// single-scene readback contrast for documentation (not a 60fps claim).
//
// Env:
//   WGPU_NATIVE_PATH     required
//   S6_PERF_WARMUP       default 3 (locked in docs/S6_PERF_BASELINE.md)
//   S6_PERF_ITERS        default 10
//   S6_PERF_JSON         default <repo>/tmp/s6_present_baseline.json
//   S6_WRITE_BASELINE=1  overwrite frozen baseline JSON (default: keep freeze; always write s6_present_latest.json)
//   S6_MAIN_PATH_BUDGET  default 16.7 (p50 ms)
//   S6_REGRESS_PCT       default 10 (main-path allowed regression vs budget floor)

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/render"
)

// S6.0 locked defaults (document must match).
const (
	s6DefaultWarmup      = 3
	s6DefaultIters       = 10
	s6MainPathBudgetMs   = 16.7
	s6MainPathRegressPct = 10.0
	s6BaselineVersion    = "s6.0-present-1"
)

// Frozen scene names for S6 — do not rename/remove without bumping baseline version.
var s6FrozenSceneNames = []string{
	"P01_SolidPresent",
	"U01_StaticShell",
	"U02_ListScrollMorph",
	"U03_FormFieldDamage",
	"U04_ModalStatic",
	"U05_KitchenSinkStress",
	"B15like_MultiDamage",
	// Heavy / production stress (present-only)
	"H01_FullRedrawShell",
	"H02_LayerBlendStack",
	"H03_PathStrokeCloud",
	"H04_TextRows40",
	"H05_ImageTileGrid",
	"H06_NestedClipLayerText",
}

var s6MainPathScenes = map[string]bool{
	"U01_StaticShell":     true,
	"U02_ListScrollMorph": true,
	"U03_FormFieldDamage": true,
	"U04_ModalStatic":     true,
}

type s6SceneMeta struct {
	Name  string `json:"name"`
	Tier  string `json:"tier"` // P0_main / P1_density / P2_heavy / P3_stress / floor
	Class string `json:"class"`
}

type s6SceneResult struct {
	s5SceneResult
	Tier  string `json:"tier"`
	Class string `json:"class"`
}

type s6BaselineFile struct {
	Version            string          `json:"version"`
	Date               string          `json:"date"`
	GOOS               string          `json:"goos"`
	GOARCH             string          `json:"goarch"`
	NumCPU             int             `json:"num_cpu"`
	Hostname           string          `json:"hostname"`
	WGPUPath           string          `json:"wgpu_native_path"`
	Warmup             int             `json:"warmup_locked"`
	Iters              int             `json:"iters_locked"`
	MainPathBudgetMs   float64         `json:"main_path_budget_ms"`
	MainPathRegressPct float64         `json:"main_path_regress_pct"`
	Note               string          `json:"note"`
	FrozenScenes       []string        `json:"frozen_scene_names"`
	Scenes             []s6SceneResult `json:"scenes"`
	ReadbackContrast   *s6ReadbackNote `json:"readback_contrast,omitempty"`
}

type s6ReadbackNote struct {
	Scene         string  `json:"scene"`
	PresentP50Ms  float64 `json:"present_p50_ms"`
	ReadbackP50Ms float64 `json:"readback_p50_ms"`
	Note          string  `json:"note"`
}

func s6JSONPath() string {
	if p := os.Getenv("S6_PERF_JSON"); p != "" {
		return p
	}
	// Walk up from cwd to find go.mod (works whether cwd is repo root or package dir).
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join("tmp", "s6_present_baseline.json")
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "tmp", "s6_present_baseline.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join("tmp", "s6_present_baseline.json")
}

func s6TierOf(name string) (tier, class string) {
	switch name {
	case "P01_SolidPresent":
		return "floor", "solid"
	case "U01_StaticShell", "U02_ListScrollMorph", "U03_FormFieldDamage", "U04_ModalStatic":
		return "P0_main", "ui_main_path"
	case "B15like_MultiDamage":
		return "P0_main", "multi_damage"
	case "U05_KitchenSinkStress", "H02_LayerBlendStack":
		return "P2_heavy", "layer_blend"
	case "H01_FullRedrawShell":
		return "P1_density", "full_redraw"
	case "H03_PathStrokeCloud":
		return "P2_heavy", "path_stroke"
	case "H04_TextRows40":
		return "P1_density", "text"
	case "H05_ImageTileGrid":
		return "P1_density", "image"
	case "H06_NestedClipLayerText":
		return "P3_stress", "nested_clip_layer"
	default:
		return "P2_heavy", "other"
	}
}

func s6HeavyScenes() []s5Scene {
	return []s5Scene{
		{
			// Full shell redraw every frame (anti-pattern; production must prefer damage).
			Name: "H01_FullRedrawShell",
			W:    800, H: 480, NeedFont: true, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.10, 0.11, 0.14, 1, 0, 0, 800, 480)
				s5Solid(dc, 0.16, 0.18, 0.24, 1, 0, 0, 800, 44)
				s5Solid(dc, 0.14, 0.15, 0.20, 1, 0, 44, 180, 436)
				s5Solid(dc, 0.95, 0.96, 0.98, 1, 180, 44, 620, 436)
				dc.SetRGB(0.92, 0.93, 0.95)
				dc.DrawString("FullRedraw Shell", 16, 28)
				for i := 0; i < 6; i++ {
					dc.SetRGB(0.7, 0.72, 0.78)
					dc.DrawString(fmt.Sprintf("nav-%02d", i), 20, 80+float64(i)*48)
				}
				for i := 0; i < 4; i++ {
					dc.SetRGB(1, 1, 1)
					dc.DrawRoundedRectangle(200, 70+float64(i)*90, 560, 72, 8)
					_ = dc.Fill()
					dc.SetRGB(0.2, 0.22, 0.26)
					dc.DrawString(fmt.Sprintf("card %d full redraw", i), 220, 110+float64(i)*90)
				}
			},
		},
		{
			Name: "H02_LayerBlendStack",
			W:    480, H: 360, NeedFont: true, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.97, 0.97, 0.98, 1, 0, 0, 480, 360)
				for i := 0; i < 10; i++ {
					dc.PushLayer(render.BlendNormal, 0.85)
					dc.SetRGB(0.15+float64(i%5)*0.1, 0.3, 0.7-float64(i%3)*0.05)
					dc.DrawRoundedRectangle(12+float64(i%5)*90, 12+float64(i/5)*160, 80, 130, 8)
					_ = dc.Fill()
					dc.PopLayer()
				}
				dc.SetBlendMode(render.BlendMultiply)
				dc.SetRGB(1, 0.45, 0.25)
				dc.DrawCircle(240, 180, 70)
				_ = dc.Fill()
				dc.SetBlendMode(render.BlendNormal)
				dc.SetRGB(0.1, 0.1, 0.12)
				dc.DrawString("layer×blend stack", 160, 340)
			},
		},
		{
			Name: "H03_PathStrokeCloud",
			W:    480, H: 360, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.98, 0.98, 0.99, 1, 0, 0, 480, 360)
				dc.SetLineWidth(2)
				dc.SetDash(6, 4)
				for i := 0; i < 24; i++ {
					p := render.NewPath()
					x0 := 20 + float64(i%6)*75
					y0 := 30 + float64(i/6)*80
					p.MoveTo(x0, y0)
					p.LineTo(x0+55, y0+8)
					p.LineTo(x0+40, y0+50)
					p.LineTo(x0+10, y0+45)
					p.Close()
					dc.SetRGB(0.2, 0.35+float64(i%4)*0.1, 0.75)
					dc.AppendPath(p)
					_ = dc.Stroke()
				}
				dc.SetDash()
			},
		},
		{
			Name: "H04_TextRows40",
			W:    640, H: 480, NeedFont: true, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.12, 0.13, 0.16, 1, 0, 0, 640, 480)
				dc.SetRGB(0.92, 0.93, 0.95)
				for i := 0; i < 40; i++ {
					dc.DrawString(fmt.Sprintf("text-row-%02d shape atlas scroll pressure", i), 16, 16+float64(i)*11)
				}
			},
		},
		{
			Name: "H05_ImageTileGrid",
			W:    512, H: 512, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				// Solid tiles stand in for image grid density without external assets.
				s5Solid(dc, 0.2, 0.22, 0.26, 1, 0, 0, 512, 512)
				for y := 0; y < 8; y++ {
					for x := 0; x < 8; x++ {
						s5Solid(dc,
							0.3+float64(x)*0.05, 0.4, 0.5+float64(y)*0.04, 1,
							float64(x*64+2), float64(y*64+2), 60, 60)
					}
				}
			},
		},
		{
			Name: "H06_NestedClipLayerText",
			W:    560, H: 400, NeedFont: true, Retained: true,
			Draw: func(t *testing.T, dc *render.Context, _ string) {
				s5Solid(dc, 0.96, 0.97, 0.98, 1, 0, 0, 560, 400)
				for i := 0; i < 4; i++ {
					dc.ClipRect(20+float64(i)*8, 20+float64(i)*8, 520-float64(i)*16, 360-float64(i)*16)
					dc.PushLayer(render.BlendNormal, 0.92)
					dc.SetRGB(0.85-float64(i)*0.1, 0.88, 0.95)
					dc.DrawRoundedRectangle(30+float64(i)*10, 30+float64(i)*10, 480-float64(i)*20, 320-float64(i)*20, 10)
					_ = dc.Fill()
					dc.SetRGB(0.15, 0.16, 0.2)
					dc.DrawString(fmt.Sprintf("nested clip×layer %d", i), 50+float64(i)*12, 60+float64(i)*40)
					dc.PopLayer()
				}
				for i := 0; i < 4; i++ {
					dc.ResetClip()
				}
			},
		},
	}
}

func s6AllScenes() []s5Scene {
	// Frozen order: S5 scenes then heavy.
	out := append([]s5Scene{}, s5Scenes()...)
	out = append(out, s6HeavyScenes()...)
	return out
}

func TestS6_PresentBaseline_Scenes(t *testing.T) {
	p1RequireGPU(t)
	warmup := s5EnvInt("S6_PERF_WARMUP", s6DefaultWarmup)
	iters := s5EnvInt("S6_PERF_ITERS", s6DefaultIters)
	if iters < 1 {
		iters = 1
	}
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", s6MainPathBudgetMs)

	host, _ := os.Hostname()
	out := s6BaselineFile{
		Version:            s6BaselineVersion,
		Date:               time.Now().Format(time.RFC3339),
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		NumCPU:             runtime.NumCPU(),
		Hostname:           host,
		WGPUPath:           os.Getenv("WGPU_NATIVE_PATH"),
		Warmup:             warmup,
		Iters:              iters,
		MainPathBudgetMs:   budget,
		MainPathRegressPct: s6MainPathRegressPct,
		Note:               "S6.0 frozen present-only deep baseline. No ReadPixels in timed path. Scene names frozen.",
		FrozenScenes:       append([]string{}, s6FrozenSceneNames...),
	}

	scenes := s6AllScenes()
	// Ensure freeze list matches implementation.
	got := map[string]bool{}
	for _, sc := range scenes {
		got[sc.Name] = true
	}
	for _, name := range s6FrozenSceneNames {
		if !got[name] {
			t.Fatalf("frozen scene %q missing from s6AllScenes", name)
		}
	}
	if len(scenes) != len(s6FrozenSceneNames) {
		// allow only exact freeze set
		extra := []string{}
		for n := range got {
			found := false
			for _, f := range s6FrozenSceneNames {
				if f == n {
					found = true
					break
				}
			}
			if !found {
				extra = append(extra, n)
			}
		}
		if len(extra) > 0 {
			t.Fatalf("scenes not in freeze list: %v", extra)
		}
	}

	var results []s6SceneResult
	for _, sc := range scenes {
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			res := s5MeasurePresent(t, sc, warmup, iters)
			tier, class := s6TierOf(sc.Name)
			row := s6SceneResult{s5SceneResult: res, Tier: tier, Class: class}
			results = append(results, row)
			t.Logf("%s tier=%s p50=%.2f avg=%.2f fps_p50=%.1f gpu=%d cpu_fb=%d dmg=%v ret=%v",
				res.Name, tier, res.TotalMsP50, res.TotalMsAvg, res.FpsP50, res.GPUOps, res.CPUFallbackOps, res.Damage, res.Retained)
			if res.CPUFallbackOps != 0 {
				t.Fatalf("cpu_fallback_ops=%d", res.CPUFallbackOps)
			}
			if res.GPUOps == 0 {
				t.Fatal("GPUOps==0")
			}
			if s6MainPathScenes[sc.Name] && res.TotalMsP50 > budget {
				t.Fatalf("main-path %s p50=%.2f exceeds locked budget %.2f", sc.Name, res.TotalMsP50, budget)
			}
		})
	}
	out.Scenes = results
	sort.Slice(out.Scenes, func(i, j int) bool { return out.Scenes[i].Name < out.Scenes[j].Name })

	// Optional dual-track: readback contrast on P01 only (document noise, not gate).
	out.ReadbackContrast = s6MeasureReadbackContrast(t, warmup, iters)

	path := s6JSONPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	// Always write a non-authoritative latest measure for diagnostics.
	latest := filepath.Join(filepath.Dir(path), "s6_present_latest.json")
	if err := os.WriteFile(latest, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", latest, err)
	}
	// Frozen S6.0 baseline is authoritative for S6.9 relative gates. Only refresh
	// when missing or S6_WRITE_BASELINE=1 (explicit re-freeze). Full unit suites must
	// not clobber the freeze with same-day noise and then require "must improve" vs self.
	writeFreeze := os.Getenv("S6_WRITE_BASELINE") == "1"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			writeFreeze = true
		} else {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
	if writeFreeze {
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote frozen baseline %s (%d scenes) latest=%s", path, len(out.Scenes), latest)
	} else {
		t.Logf("kept frozen baseline %s; wrote measure-only %s (%d scenes). Set S6_WRITE_BASELINE=1 to refresh freeze.", path, latest, len(out.Scenes))
	}
}

func s6MeasureReadbackContrast(t *testing.T, warmup, iters int) *s6ReadbackNote {
	t.Helper()
	// Present-only p50 from a quick P01 measure.
	p01 := s5Scene{
		Name: "P01_contrast",
		W:    320, H: 200,
		Draw: func(t *testing.T, dc *render.Context, _ string) {
			s5Solid(dc, 0.2, 0.25, 0.3, 1, 0, 0, 320, 200)
		},
	}
	pres := s5MeasurePresent(t, p01, warmup, iters)

	// Readback path: Draw + FlushGPU (includes readback) — NOT a 60fps claim.
	dc := render.NewContext(320, 200)
	defer dc.Close()
	samples := make([]float64, 0, iters)
	for i := 0; i < warmup+iters; i++ {
		dc.ResetRenderPathStats()
		t0 := time.Now()
		s5Solid(dc, 0.2, 0.25, 0.3, 1, 0, 0, 320, 200)
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		dt := time.Since(t0).Seconds() * 1000
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatal("readback contrast GPUOps==0")
		}
		if i >= warmup {
			samples = append(samples, dt)
		}
	}
	return &s6ReadbackNote{
		Scene:         "P01_Solid_320x200",
		PresentP50Ms:  pres.TotalMsP50,
		ReadbackP50Ms: s5Percentile(samples, 0.50),
		Note:          "Readback wall-time is diagnostic only; never use for 60fps claims (S5/S6 hard rule).",
	}
}

// TestS6_RegressionLock_Contract verifies S6.0 freeze invariants after baseline JSON exists.
// Run after TestS6_PresentBaseline_Scenes in CI, or alone if JSON already present.
func TestS6_RegressionLock_Contract(t *testing.T) {
	p1RequireGPU(t)
	path := s6JSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		// Generate baseline in-process if missing (single-test local runs).
		t.Logf("baseline missing (%v); running measure once", err)
		TestS6_PresentBaseline_Scenes(t)
		raw, err = os.ReadFile(path)
		if err != nil {
			t.Fatalf("baseline still missing: %v", err)
		}
	}
	var doc s6BaselineFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	if doc.Version != s6BaselineVersion {
		t.Fatalf("version want %s got %s", s6BaselineVersion, doc.Version)
	}
	if doc.Warmup != s6DefaultWarmup && os.Getenv("S6_PERF_WARMUP") == "" {
		// allow env override but default lock is 3
		t.Logf("warmup=%d (default lock %d; env override ok)", doc.Warmup, s6DefaultWarmup)
	}
	if len(doc.Scenes) != len(s6FrozenSceneNames) {
		t.Fatalf("scene count want %d got %d", len(s6FrozenSceneNames), len(doc.Scenes))
	}
	byName := map[string]s6SceneResult{}
	for _, s := range doc.Scenes {
		byName[s.Name] = s
	}
	for _, name := range s6FrozenSceneNames {
		s, ok := byName[name]
		if !ok {
			t.Fatalf("missing frozen scene %s", name)
		}
		if s.GPUOps <= 0 {
			t.Fatalf("%s GPUOps<=0", name)
		}
		if s.CPUFallbackOps != 0 {
			t.Fatalf("%s cpu_fallback_ops=%d", name, s.CPUFallbackOps)
		}
		if s6MainPathScenes[name] && s.TotalMsP50 > doc.MainPathBudgetMs {
			t.Fatalf("main-path %s p50=%.2f > budget %.2f", name, s.TotalMsP50, doc.MainPathBudgetMs)
		}
	}
	if doc.ReadbackContrast == nil {
		t.Fatal("expected readback_contrast dual-track note")
	}
	if doc.ReadbackContrast.PresentP50Ms <= 0 || doc.ReadbackContrast.ReadbackP50Ms <= 0 {
		t.Fatalf("contrast times invalid: %+v", doc.ReadbackContrast)
	}
	t.Logf("S6.0 regression lock OK scenes=%d main_budget=%.1f contrast present=%.2f readback=%.2f",
		len(doc.Scenes), doc.MainPathBudgetMs, doc.ReadbackContrast.PresentP50Ms, doc.ReadbackContrast.ReadbackP50Ms)
}

// TestS6_L0_MainPathStillGreen is the L0 smoke required every later S6.x slice.
func TestS6_L0_MainPathStillGreen(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", s6MainPathBudgetMs)
	warmup := s5EnvInt("S6_PERF_WARMUP", s6DefaultWarmup)
	iters := s5EnvInt("S6_PERF_ITERS", 5)
	if iters < 3 {
		iters = 3
	}
	for _, sc := range s6AllScenes() {
		if !s6MainPathScenes[sc.Name] {
			continue
		}
		sc := sc
		t.Run(sc.Name, func(t *testing.T) {
			res := s5MeasurePresent(t, sc, warmup, iters)
			if res.GPUOps == 0 || res.CPUFallbackOps != 0 {
				t.Fatalf("gpu=%d cpu_fb=%d", res.GPUOps, res.CPUFallbackOps)
			}
			if res.TotalMsP50 > budget {
				t.Fatalf("p50=%.2f > budget %.2f", res.TotalMsP50, budget)
			}
		})
	}
}
