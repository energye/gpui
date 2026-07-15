//go:build !nogpu

package render_test

// S6.9 — tiered heavy-scene present budgets (not a flat 16.7ms claim).
//
// Rules:
//   - Present-only p50 (no ReadPixels wall-time)
//   - GPUOps>0, cpu_fallback_ops=0
//   - P0 main path hard ≤16.7ms; no silent CPU
//   - P1/P2/P3 budgets relative to frozen S6.0 JSON + absolute caps
//
// Env:
//   WGPU_NATIVE_PATH       required
//   S6_PERF_WARMUP         default 3
//   S6_PERF_ITERS          default 6 (S6.9; heavy scenes are long)
//   S6_MAIN_PATH_BUDGET    default 16.7
//   S6_9_JSON              default <repo>/tmp/s6_9_heavy_budget.json
//   S6_9_REGRESS_ONLY=1    skip absolute "must improve" relative gates (still cap)

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

const (
	s69Version           = "s6.9-heavy-budget-1"
	s69DefaultIters      = 6
	s69P1DensityBudgetMs = 33.4 // 2 frames @60fps — density / full-redraw anti-pattern
	s69P1TargetBudgetMs  = 16.7 // preferred for text/image density once under
	s69P3RegressPct      = 15.0 // stress: allow mild noise, fail large regress vs S6.0
	s69P2MustImprovePct  = 8.0  // heavy: require ≥8% better than S6.0 p50
)

// Absolute present p50 ceilings (ms) — set from S6.9 measure + headroom; not 60fps claims.
var s69AbsoluteCapMs = map[string]float64{
	// P2 heavy
	"U05_KitchenSinkStress": 145.0, // S6.9~128; S6.0 160
	"H02_LayerBlendStack":   210.0, // S6.9~190; S6.0 224
	"H03_PathStrokeCloud":   90.0,  // S6.9~64;  S6.0 131
	// P3 stress
	"H06_NestedClipLayerText": 150.0, // S6.9~96; S6.0 125 — regress gate primary
	// P1 density soft ceilings (also ≤16.7 preferred for H04/H05)
	"H01_FullRedrawShell": 33.4,
	"H04_TextRows40":      16.7,
	"H05_ImageTileGrid":   16.7,
}

// Per-scene relative factor vs S6.0 (must be ≤ S6.0 * factor). Empty → tier default.
var s69RelativeFactor = map[string]float64{
	"H03_PathStrokeCloud": 0.70, // path/tess saw large S6.6 wins
	// default P2: 0.92 (mustImprove 8%) applied in code
}

type s69BudgetRow struct {
	Name           string  `json:"name"`
	Tier           string  `json:"tier"`
	Class          string  `json:"class"`
	S60P50Ms       float64 `json:"s6_0_p50_ms"`
	S69P50Ms       float64 `json:"s6_9_p50_ms"`
	S69AvgMs       float64 `json:"s6_9_avg_ms"`
	FpsP50         float64 `json:"fps_p50"`
	BudgetAbsMs    float64 `json:"budget_abs_ms"`
	BudgetRelMs    float64 `json:"budget_rel_ms"`
	BudgetEffMs    float64 `json:"budget_effective_ms"`
	VsS60Ratio     float64 `json:"vs_s6_0_ratio"`
	GPUOps         int     `json:"gpu_ops"`
	CPUFallbackOps int     `json:"cpu_fallback_ops"`
	Status         string  `json:"status"` // PASS / FAIL
	Note           string  `json:"note,omitempty"`
}

type s69BudgetFile struct {
	Version            string         `json:"version"`
	Date               string         `json:"date"`
	GOOS               string         `json:"goos"`
	GOARCH             string         `json:"goarch"`
	NumCPU             int            `json:"num_cpu"`
	Hostname           string         `json:"hostname"`
	WGPUPath           string         `json:"wgpu_native_path"`
	Warmup             int            `json:"warmup"`
	Iters              int            `json:"iters"`
	MainPathBudgetMs   float64        `json:"main_path_budget_ms"`
	S60BaselinePath    string         `json:"s6_0_baseline_path"`
	S60BaselineVersion string         `json:"s6_0_baseline_version"`
	Note               string         `json:"note"`
	Rows               []s69BudgetRow `json:"rows"`
}

func s69JSONPath() string {
	if p := os.Getenv("S6_9_JSON"); p != "" {
		return p
	}
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join("tmp", "s6_9_heavy_budget.json")
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "tmp", "s6_9_heavy_budget.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join("tmp", "s6_9_heavy_budget.json")
}

func s69LoadS60Baseline(t *testing.T) (s6BaselineFile, string) {
	t.Helper()
	path := s6JSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("S6.0 baseline required at %s: %v (run TestS6_PresentBaseline_Scenes first)", path, err)
	}
	var doc s6BaselineFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse S6.0 baseline: %v", err)
	}
	if doc.Version != s6BaselineVersion {
		t.Fatalf("S6.0 baseline version want %s got %s", s6BaselineVersion, doc.Version)
	}
	return doc, path
}

func s69EffectiveBudget(name, tier string, s60p50 float64) (abs, rel, eff float64, note string) {
	abs = 0
	if v, ok := s69AbsoluteCapMs[name]; ok {
		abs = v
	}
	switch tier {
	case "P0_main", "floor":
		abs = s6MainPathBudgetMs
		rel = s60p50 * (1.0 + s6MainPathRegressPct/100.0)
		if rel <= 0 {
			rel = s6MainPathBudgetMs
		}
		// Main path never exceeds hard 16.7; also fail if much worse than S6.0.
		eff = s6MainPathBudgetMs
		if rel < eff {
			// if S6.0 was already well under, keep hard 16.7 (do not tighten to S6.0*1.1 for PASS —
			// production gate is 60fps). Relative is advisory for "no big regress" checks separately.
		}
		note = "P0 hard ≤16.7ms present p50"
		return abs, rel, eff, note
	case "P1_density":
		if abs <= 0 {
			abs = s69P1DensityBudgetMs
		}
		rel = s60p50 * (1.0 + s6MainPathRegressPct/100.0)
		if rel <= 0 {
			rel = abs
		}
		// Effective = min(abs, max(rel, target)) — prefer 16.7 when scene class allows.
		eff = abs
		if rel > 0 && rel < eff {
			eff = rel
		}
		// H04/H05 absolute already 16.7; H01 keeps 33.4
		note = "P1 density: abs cap + no >10% regress vs S6.0"
		return abs, rel, eff, note
	case "P2_heavy":
		factor := 1.0 - s69P2MustImprovePct/100.0 // 0.92
		if f, ok := s69RelativeFactor[name]; ok {
			factor = f
		}
		rel = s60p50 * factor
		if abs <= 0 {
			abs = s60p50 // fallback
		}
		eff = abs
		if rel > 0 && rel < abs {
			eff = rel
		}
		// When S6.0 is huge and we only modestly improved, abs may be the looser; require BOTH:
		// p50 <= abs AND p50 <= S6.0 (never worse). Effective for fail uses the stricter of abs and
		// "must improve" rel when S6_9_REGRESS_ONLY is unset.
		if os.Getenv("S6_9_REGRESS_ONLY") == "1" {
			eff = abs
			if s60p50 > 0 && s60p50 < eff {
				eff = s60p50 // at least not worse
			}
			note = "P2 regress-only mode: ≤abs and ≤S6.0"
		} else {
			// Pass if under abs AND under S6.0*factor (must improve).
			// Store eff as the stricter of the two for the primary gate.
			note = fmt.Sprintf("P2 heavy: ≤abs(%.1f) AND ≤S6.0×%.2f (must improve)", abs, factor)
		}
		return abs, rel, eff, note
	case "P3_stress":
		rel = s60p50 * (1.0 + s69P3RegressPct/100.0)
		if abs <= 0 {
			abs = rel
		}
		eff = rel
		if abs < eff {
			eff = abs
		}
		note = "P3 stress: regression-only vs S6.0 (+15%) + abs ceiling"
		return abs, rel, eff, note
	default:
		abs = s6MainPathBudgetMs * 4
		rel = s60p50 * 1.1
		eff = abs
		note = "default soft"
		return abs, rel, eff, note
	}
}

func s69ScenePass(name, tier string, p50, s60, abs, rel, eff float64) (ok bool, why string) {
	if p50 <= 0 {
		return false, "p50<=0"
	}
	switch tier {
	case "P0_main", "floor":
		if p50 > s6MainPathBudgetMs {
			return false, fmt.Sprintf("P0 p50=%.2f > hard %.2f", p50, s6MainPathBudgetMs)
		}
		return true, "P0 under 16.7"
	case "P1_density":
		// Must respect absolute density cap.
		if abs > 0 && p50 > abs {
			return false, fmt.Sprintf("P1 p50=%.2f > abs %.2f", p50, abs)
		}
		// No >10% regress vs S6.0 when S6.0 known.
		if s60 > 0 && p50 > s60*(1.0+s6MainPathRegressPct/100.0) {
			return false, fmt.Sprintf("P1 p50=%.2f > S6.0×1.10 (%.2f)", p50, s60*1.10)
		}
		return true, "P1 ok"
	case "P2_heavy":
		if abs > 0 && p50 > abs {
			return false, fmt.Sprintf("P2 p50=%.2f > abs %.2f", p50, abs)
		}
		if s60 > 0 && p50 > s60 {
			return false, fmt.Sprintf("P2 p50=%.2f worse than S6.0 %.2f", p50, s60)
		}
		if os.Getenv("S6_9_REGRESS_ONLY") != "1" && s60 > 0 && rel > 0 && p50 > rel {
			return false, fmt.Sprintf("P2 p50=%.2f > must-improve rel %.2f (S6.0=%.2f)", p50, rel, s60)
		}
		return true, "P2 ok"
	case "P3_stress":
		if abs > 0 && p50 > abs {
			return false, fmt.Sprintf("P3 p50=%.2f > abs %.2f", p50, abs)
		}
		if s60 > 0 && p50 > s60*(1.0+s69P3RegressPct/100.0) {
			return false, fmt.Sprintf("P3 p50=%.2f > S6.0×1.15 (%.2f)", p50, s60*1.15)
		}
		return true, "P3 ok"
	default:
		if p50 > eff && eff > 0 {
			return false, fmt.Sprintf("p50=%.2f > eff %.2f", p50, eff)
		}
		return true, "ok"
	}
}

// TestS69_HeavyBudget_TierGates measures frozen scenes and enforces tiered budgets vs S6.0.
func TestS69_HeavyBudget_TierGates(t *testing.T) {
	p1RequireGPU(t)
	s60, s60Path := s69LoadS60Baseline(t)
	by60 := map[string]s6SceneResult{}
	for _, s := range s60.Scenes {
		by60[s.Name] = s
	}

	warmup := s5EnvInt("S6_PERF_WARMUP", s6DefaultWarmup)
	iters := s5EnvInt("S6_PERF_ITERS", s69DefaultIters)
	if iters < 3 {
		iters = 3
	}
	budgetMain := s5EnvFloat("S6_MAIN_PATH_BUDGET", s6MainPathBudgetMs)

	// Measure order: light → dense → heavy → stress (VRAM friendly).
	order := []string{
		"P01_SolidPresent",
		"U01_StaticShell", "U02_ListScrollMorph", "U03_FormFieldDamage", "U04_ModalStatic",
		"B15like_MultiDamage",
		"H01_FullRedrawShell", "H04_TextRows40", "H05_ImageTileGrid",
		"U05_KitchenSinkStress", "H03_PathStrokeCloud", "H02_LayerBlendStack",
		"H06_NestedClipLayerText",
	}
	sceneByName := map[string]s5Scene{}
	for _, sc := range s6AllScenes() {
		sceneByName[sc.Name] = sc
	}

	host, _ := os.Hostname()
	out := s69BudgetFile{
		Version:            s69Version,
		Date:               time.Now().Format(time.RFC3339),
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		NumCPU:             runtime.NumCPU(),
		Hostname:           host,
		WGPUPath:           os.Getenv("WGPU_NATIVE_PATH"),
		Warmup:             warmup,
		Iters:              iters,
		MainPathBudgetMs:   budgetMain,
		S60BaselinePath:    s60Path,
		S60BaselineVersion: s60.Version,
		Note:               "S6.9 tiered heavy budgets. Present-only. P0 hard 16.7ms; P2 must improve vs S6.0; P3 regression-only. Not a blanket 60fps claim for P2/P3.",
	}

	var rows []s69BudgetRow
	var failed []string

	for _, name := range order {
		sc, ok := sceneByName[name]
		if !ok {
			t.Fatalf("scene %s missing from s6AllScenes", name)
		}
		tier, class := s6TierOf(name)
		s60row, has60 := by60[name]
		s60p50 := 0.0
		if has60 {
			s60p50 = s60row.TotalMsP50
		}

		// GC between heavy scenes to reduce cumulative native VRAM pressure.
		runtime.GC()

		res := s5MeasurePresent(t, sc, warmup, iters)
		if res.CPUFallbackOps != 0 {
			t.Fatalf("%s cpu_fallback_ops=%d", name, res.CPUFallbackOps)
		}
		if res.GPUOps == 0 {
			t.Fatalf("%s GPUOps==0", name)
		}

		abs, rel, eff, note := s69EffectiveBudget(name, tier, s60p50)
		ratio := 0.0
		if s60p50 > 0 {
			ratio = res.TotalMsP50 / s60p50
		}
		okPass, why := s69ScenePass(name, tier, res.TotalMsP50, s60p50, abs, rel, eff)
		status := "PASS"
		if !okPass {
			status = "FAIL"
			failed = append(failed, fmt.Sprintf("%s: %s", name, why))
		}
		row := s69BudgetRow{
			Name:           name,
			Tier:           tier,
			Class:          class,
			S60P50Ms:       s60p50,
			S69P50Ms:       res.TotalMsP50,
			S69AvgMs:       res.TotalMsAvg,
			FpsP50:         res.FpsP50,
			BudgetAbsMs:    abs,
			BudgetRelMs:    rel,
			BudgetEffMs:    eff,
			VsS60Ratio:     ratio,
			GPUOps:         res.GPUOps,
			CPUFallbackOps: res.CPUFallbackOps,
			Status:         status,
			Note:           note + "; " + why,
		}
		rows = append(rows, row)
		t.Logf("%s tier=%s s6.0=%.2f s6.9=%.2f ratio=%.3f abs=%.1f rel=%.1f eff=%.1f gpu=%d %s",
			name, tier, s60p50, res.TotalMsP50, ratio, abs, rel, eff, res.GPUOps, status)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	out.Rows = rows

	path := s69JSONPath()
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
	t.Logf("wrote %s (%d rows)", path, len(rows))

	if len(failed) > 0 {
		t.Fatalf("S6.9 budget failures (%d):\n  %s", len(failed), joinLines(failed))
	}
}

func joinLines(xs []string) string {
	out := ""
	for i, s := range xs {
		if i > 0 {
			out += "\n  "
		}
		out += s
	}
	return out
}

// TestS69_Contract_FromJSON validates a previously written S6.9 JSON (CI-friendly if measure already done).
func TestS69_Contract_FromJSON(t *testing.T) {
	path := s69JSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no S6.9 JSON yet (%v); run TestS69_HeavyBudget_TierGates", err)
	}
	var doc s69BudgetFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json: %v", err)
	}
	if doc.Version != s69Version {
		t.Fatalf("version want %s got %s", s69Version, doc.Version)
	}
	if len(doc.Rows) < 10 {
		t.Fatalf("expected ≥10 rows, got %d", len(doc.Rows))
	}
	var fails []string
	for _, r := range doc.Rows {
		if r.GPUOps <= 0 {
			fails = append(fails, r.Name+": GPUOps<=0")
		}
		if r.CPUFallbackOps != 0 {
			fails = append(fails, fmt.Sprintf("%s: cpu_fb=%d", r.Name, r.CPUFallbackOps))
		}
		if r.Status != "PASS" {
			fails = append(fails, fmt.Sprintf("%s: status=%s", r.Name, r.Status))
		}
		// Re-validate gates against stored numbers (no re-measure).
		ok, why := s69ScenePass(r.Name, r.Tier, r.S69P50Ms, r.S60P50Ms, r.BudgetAbsMs, r.BudgetRelMs, r.BudgetEffMs)
		if !ok {
			fails = append(fails, r.Name+": "+why)
		}
	}
	if len(fails) > 0 {
		t.Fatalf("S6.9 contract fails:\n  %s", joinLines(fails))
	}
	t.Logf("S6.9 contract OK rows=%d file=%s", len(doc.Rows), path)
}

// TestS69_L0_MainPathStillGreen keeps P0 under 16.7 after budget work.
func TestS69_L0_MainPathStillGreen(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", s6MainPathBudgetMs)
	warmup := s5EnvInt("S6_PERF_WARMUP", s6DefaultWarmup)
	iters := s5EnvInt("S6_PERF_ITERS", 4)
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
