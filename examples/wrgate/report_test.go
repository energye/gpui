package wrgate_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scheduler"
)

func TestBuildReport_AndSchema(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy:          scheduler.PresentPolicyFullPaint,
		FrameCount:             10,
		PresentCount:           10,
		AvgFrameIntervalMs:     16.6,
		P50FrameIntervalMs:     16.5,
		P95FrameIntervalMs:     17.0,
		P99FrameIntervalMs:     18.0,
		VSyncSource:            "fallback",
		LayoutCount:            1,
		PaintCount:             10,
		PresentMode:            "full",
		CPUPctAvg:              5,
		CPUUIPct:               50,
		CPURasterPct:           50,
		RSSStartKB:             1000,
		RSSEndKB:               1100,
		RSSPeakKB:              1200,
		RSSSlopeKBPerMin:       10,
		RSSSlopeSteadyKBPerMin: 8,
		RSSSlopeValid:          true,
		RSSWarmupSec:           15,
		GPUOps:                 100,
	}
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R0",
		Scenario:      "ui_wr_r0_fullpaint",
		Snap:          snap,
		PresentCount:  60,
		ElapsedSec:    1.0,
		SurfaceAreaPx: 100 * 100,
		Warmup:        true,
	})
	if r.FPSWall < 59 || r.FPSWall > 61 {
		t.Fatalf("fps_wall=%v want ~60", r.FPSWall)
	}
	// No interval samples in this snap path beyond zeros — ok.
	if r.PresentPolicy != "full_paint" {
		t.Fatalf("policy=%q", r.PresentPolicy)
	}
	if r.RSSSlopeSteadyKBPerMin != 8 || !r.RSSSlopeValid || r.RSSWarmupSec != 15 {
		t.Fatalf("steady fields = %+v", r)
	}
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := wrgate.CheckSchema(b); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"rss_slope_steady_kb_per_min":8`, `"rss_slope_valid":true`, `"rss_warmup_sec":15`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("JSON missing %s: %s", key, b)
		}
	}
}

func TestBuildReport_ShortRunSlopeInvalidIsHonest(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy:    scheduler.PresentPolicyFullPaint,
		VSyncSource:      "fallback",
		RSSStartKB:       1000,
		RSSEndKB:         200000,
		RSSPeakKB:        200000,
		RSSSlopeKBPerMin: 500000,
		RSSSlopeValid:    false,
		RSSWarmupSec:     15,
	}
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "B-scene", Scenario: "short", Snap: snap, PresentCount: 30, ElapsedSec: 5,
	})
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"rss_slope_valid":false`) {
		t.Fatalf("short-run validity missing/false not emitted: %s", b)
	}
	if strings.Contains(string(b), `"rss_slope_steady_kb_per_min"`) {
		t.Fatalf("invalid steady slope must not claim a value: %s", b)
	}
	if err := wrgate.CheckSchema(b); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateGates_FailNoPresents(t *testing.T) {
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:    "R0",
		Scenario:     "t",
		Snap:         scheduler.FrameMetrics{PresentPolicy: "full_paint", VSyncSource: "fallback"},
		PresentCount: 0,
		ElapsedSec:   1,
	})
	// Fill required zeros via rebuild with minimal snap fields for schema
	r.PresentPolicy = "full_paint"
	r.VSyncSource = "fallback"
	err := wrgate.EvaluateGates(r, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true})
	if err == nil || !strings.Contains(err.Error(), "FAIL:") {
		t.Fatalf("want FAIL no presents, got %v", err)
	}
}

func TestEvaluateGates_FailWrongPolicy(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy: "retained",
		VSyncSource:   "fallback",
		FrameCount:    5,
	}
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R0", Scenario: "t", Snap: snap, PresentCount: 5, ElapsedSec: 1,
	})
	err := wrgate.EvaluateGates(r, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true})
	if err == nil || !strings.Contains(err.Error(), "present_policy") {
		t.Fatalf("want policy FAIL, got %v", err)
	}
}

func TestEvaluateGates_PassMinimal(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy:      scheduler.PresentPolicyFullPaint,
		VSyncSource:        "fallback",
		FrameCount:         30,
		P50FrameIntervalMs: 16,
		P95FrameIntervalMs: 17,
		CPUPctAvg:          1,
		RSSStartKB:         1,
		RSSEndKB:           1,
		RSSPeakKB:          1,
	}
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R0", Scenario: "t", Snap: snap, PresentCount: 30, ElapsedSec: 0.5, Warmup: true,
	})
	// Short elapsed → FPS hard gate skipped (default MinFPSElapsed=5s / U16)
	if err := wrgate.EvaluateGates(r, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestBuildReport_FirstPresentObservations: R16 H-family — observed
// warmup/time/first-paint from the snapshot must flow into the JSON shell
// (observation wins over the BuildInput legacy fallback).
func TestBuildReport_FirstPresentObservations(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy:          scheduler.PresentPolicyFullPaint,
		VSyncSource:            "fallback",
		Warmup:                 true,
		TimeToFirstPresentMs:   88.4,
		FirstPresentPaintCount: 25,
		PaintCount:             25,
		RSSStartKB:             1,
		RSSEndKB:               1,
		RSSPeakKB:              1,
	}
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R16", Scenario: "ui_wr_r16_warmup", Snap: snap, PresentCount: 1, ElapsedSec: 0.1,
		Warmup: false, // legacy fallback must NOT override the observation
	})
	if !r.Warmup {
		t.Fatal("observed warmup=true lost (legacy false overrode)")
	}
	if r.TimeToFirstPresentMs != 88.4 {
		t.Fatalf("time_to_first_present_ms=%v want 88.4", r.TimeToFirstPresentMs)
	}
	if r.FirstPresentPaintCount != 25 {
		t.Fatalf("first_present_paint_count=%d want 25", r.FirstPresentPaintCount)
	}
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, k := range []string{`"warmup"`, `"time_to_first_present_ms"`, `"first_present_paint_count"`} {
		if !strings.Contains(js, k) {
			t.Fatalf("JSON missing %s: %s", k, js)
		}
	}
}

// TestBuildReport_DirtyLayerIDs: R4b — dirty_layer_ids 列表 + damage_multi_frames
// 从快照流入 JSON 壳（末帧两脏点 → 两 id）。
func TestBuildReport_DirtyLayerIDs(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy:     scheduler.PresentPolicyFullPaint,
		VSyncSource:       "fallback",
		DirtyLayerIDs:     []uint64{11, 22},
		DamageMultiFrames: 4,
		RSSStartKB:        1,
		RSSEndKB:          1,
		RSSPeakKB:         1,
	}
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R4b", Scenario: "ui_wr_r4b_multidamage", Snap: snap, PresentCount: 1, ElapsedSec: 0.1,
	})
	if len(r.DirtyLayerIDs) != 2 || r.DirtyLayerIDs[0] != 11 || r.DirtyLayerIDs[1] != 22 {
		t.Fatalf("dirty_layer_ids=%v want [11 22]", r.DirtyLayerIDs)
	}
	if r.DamageMultiFrames != 4 {
		t.Fatalf("damage_multi_frames=%d want 4", r.DamageMultiFrames)
	}
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, k := range []string{`"dirty_layer_ids"`, `"damage_multi_frames"`} {
		if !strings.Contains(js, k) {
			t.Fatalf("JSON missing %s: %s", k, js)
		}
	}
}

// TestEvaluateRetainedExtras_DirtyLayerIDGates: R4b — dirty_layer_id_max 与
// damage_multi_frames 门禁判定（EvaluateGates 内自动调用）。
func TestEvaluateRetainedExtras_DirtyLayerIDGates(t *testing.T) {
	base := scheduler.FrameMetrics{
		PresentPolicy: scheduler.PresentPolicyRetained,
		VSyncSource:   "fallback",
		DirtyLayerIDs: []uint64{11, 22},
		RSSStartKB:    1,
		RSSEndKB:      1,
		RSSPeakKB:     1,
	}
	mk := func(extra map[string]any) wrgate.Report {
		return wrgate.BuildReport(wrgate.BuildInput{
			AbilityID: "R4b", Scenario: "ui_wr_r4b_multidamage", Snap: base,
			PresentCount: 10, ElapsedSec: 15, Warmup: true, Extra: extra,
		})
	}
	passExtra := map[string]any{"dirty_layer_id_max": int64(2), "damage_multi_frames": int64(5)}
	if err := wrgate.EvaluateGates(mk(passExtra), wrgate.GateOptions{
		MinDirtyLayerIDs: 2, MinDamageMultiFrames: 1,
	}); err != nil {
		t.Fatalf("pass case failed: %v", err)
	}
	if err := wrgate.EvaluateGates(mk(map[string]any{"dirty_layer_id_max": int64(1), "damage_multi_frames": int64(5)}), wrgate.GateOptions{
		MinDirtyLayerIDs: 2, MinDamageMultiFrames: 1,
	}); err == nil {
		t.Fatal("want FAIL for dirty_layer_id_max=1 < 2")
	}
	if err := wrgate.EvaluateGates(mk(map[string]any{"dirty_layer_id_max": int64(2)}), wrgate.GateOptions{
		MinDirtyLayerIDs: 2, MinDamageMultiFrames: 1,
	}); err == nil {
		t.Fatal("want FAIL for missing damage_multi_frames")
	}
	if err := wrgate.EvaluateGates(mk(passExtra), wrgate.GateOptions{}); err != nil {
		t.Fatalf("zero thresholds must be off: %v", err)
	}
}

// TestEvaluateRetainedExtras_CacheInvalidationGate: R11 — ability_extra
// cache_invalidations 门禁判定（EvaluateGates 内自动调用）。
func TestEvaluateRetainedExtras_CacheInvalidationGate(t *testing.T) {
	base := scheduler.FrameMetrics{
		PresentPolicy: scheduler.PresentPolicyRetained,
		VSyncSource:   "fallback",
		RSSStartKB:    1,
		RSSEndKB:      1,
		RSSPeakKB:     1,
	}
	mk := func(extra map[string]any) wrgate.Report {
		return wrgate.BuildReport(wrgate.BuildInput{
			AbilityID: "R11", Scenario: "ui_wr_r11_dpr", Snap: base,
			PresentCount: 10, ElapsedSec: 15, Warmup: true, Extra: extra,
		})
	}
	passExtra := map[string]any{
		"cache_invalidations": int64(2), "rerecord_waves": int64(2),
	}
	if err := wrgate.EvaluateGates(mk(passExtra), wrgate.GateOptions{
		MinCacheInvalidations: 1,
	}); err != nil {
		t.Fatalf("pass case failed: %v", err)
	}
	if err := wrgate.EvaluateGates(mk(map[string]any{"cache_invalidations": int64(0)}), wrgate.GateOptions{
		MinCacheInvalidations: 1,
	}); err == nil {
		t.Fatal("want FAIL for cache_invalidations=0 < 1")
	}
	if err := wrgate.EvaluateGates(mk(nil), wrgate.GateOptions{
		MinCacheInvalidations: 1,
	}); err == nil {
		t.Fatal("want FAIL for missing cache_invalidations")
	}
	if err := wrgate.EvaluateGates(mk(passExtra), wrgate.GateOptions{}); err != nil {
		t.Fatalf("zero thresholds must be off: %v", err)
	}
}

// TestBuildReport_DebugRepaintOffStateExplicit: R12b off state must emit
// debug_repaint_on=false / debug_repaint_draws=0 explicitly. Omitting them
// looks like missing evidence instead of a measured zero overlay.
func TestBuildReport_DebugRepaintOffStateExplicit(t *testing.T) {
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R12b", Scenario: "ui_wr_r12b_debug_repaint",
		Snap: scheduler.FrameMetrics{
			PresentPolicy: scheduler.PresentPolicyFullPaint,
			VSyncSource:   "true",
		},
		PresentCount: 10, ElapsedSec: 8,
	})
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"debug_repaint_on":false`, `"debug_repaint_draws":0`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("off-state field missing %s: %s", key, b)
		}
	}
}

// TestRuler_NullWhenNotRun: 对齐线 fields must exist in every report and stay
// JSON null when no ruler arm was measured ("跑过尺才填，没跑如实 null").
// Null is evidence of "not run"; an omitted key would look like lost data.
func TestRuler_NullWhenNotRun(t *testing.T) {
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R2", Scenario: "ui_wr_r2_spin_idle",
		Snap: scheduler.FrameMetrics{
			PresentPolicy: scheduler.PresentPolicyFullPaint,
			VSyncSource:   "fallback",
			CPUPctAvg:     14.16, RSSEndKB: 450560,
		},
		PresentCount: 60, ElapsedSec: 8,
	})
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"ruler_name":null`, `"ruler_cpu_pct":null`, `"ruler_rss_mb":null`,
		`"ruler_ratio_cpu":null`, `"ruler_ratio_rss":null`,
	} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("ruler field must be explicit null when not run, missing %s:\n%s", key, b)
		}
	}
	if err := wrgate.CheckSchema(b); err != nil {
		t.Fatalf("schema must pass with null ruler fields: %v", err)
	}
}

// TestRuler_FilledAndRatios: with a real RulerInput, name/numbers fill and
// ratio = window value / ruler value (matrix §4 alignment rule: ratio <= 系数).
// CPU numerator/denominator share the /proc utime+stime percent-of-one-core
// scope; RSS pairs the window run-end rss_end_kb with the ruler rss_end_mb.
func TestRuler_FilledAndRatios(t *testing.T) {
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R2", Scenario: "ui_wr_r2_spin_idle",
		Snap: scheduler.FrameMetrics{
			PresentPolicy: scheduler.PresentPolicyFullPaint,
			VSyncSource:   "fallback",
			CPUPctAvg:     14.16, RSSEndKB: 450560, // 440.0 MB end RSS
		},
		PresentCount: 60, ElapsedSec: 8,
		Ruler: &wrgate.RulerInput{Name: "flutter_spin", CPUPct: 25.41, RSSMB: 300.0},
	})
	if r.RulerName == nil || *r.RulerName != "flutter_spin" {
		t.Fatalf("ruler_name=%v", r.RulerName)
	}
	if r.RulerCPUPct == nil || *r.RulerCPUPct != 25.41 {
		t.Fatalf("ruler_cpu_pct=%v", r.RulerCPUPct)
	}
	if r.RulerRSSMB == nil || *r.RulerRSSMB != 300.0 {
		t.Fatalf("ruler_rss_mb=%v", r.RulerRSSMB)
	}
	if r.RulerRatioCPU == nil {
		t.Fatal("ruler_ratio_cpu must be computed when both sides measured")
	}
	wantCPU := 14.16 / 25.41
	if diff := *r.RulerRatioCPU - wantCPU; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("ruler_ratio_cpu=%v want %v", *r.RulerRatioCPU, wantCPU)
	}
	if r.RulerRatioRSS == nil {
		t.Fatal("ruler_ratio_rss must be computed when both sides measured")
	}
	wantRSS := (450560.0 / 1024.0) / 300.0
	if *r.RulerRatioRSS != wantRSS {
		t.Fatalf("ruler_ratio_rss=%v want %v", *r.RulerRatioRSS, wantRSS)
	}
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := wrgate.CheckSchema(b); err != nil {
		t.Fatalf("schema must pass with filled ruler fields: %v", err)
	}
}

// TestRuler_ZeroSideKeepsNullRatio: a ruler arm measured but with an
// unusable value (0) on one side keeps that ratio null — no 0, no +Inf, no
// fabricated number. The measured ruler numbers themselves still fill.
func TestRuler_ZeroSideKeepsNullRatio(t *testing.T) {
	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "R2", Scenario: "ui_wr_r2_spin_idle",
		Snap: scheduler.FrameMetrics{
			PresentPolicy: scheduler.PresentPolicyFullPaint,
			VSyncSource:   "fallback",
			CPUPctAvg:     14.16, // rss_end_kb left 0 → no process sample path
		},
		PresentCount: 60, ElapsedSec: 8,
		Ruler: &wrgate.RulerInput{Name: "flutter_spin", CPUPct: 0, RSSMB: 300.0},
	})
	if r.RulerRatioCPU != nil {
		t.Fatalf("ruler_ratio_cpu=%v want null (ruler CPU unmeasured)", *r.RulerRatioCPU)
	}
	if r.RulerRatioRSS != nil {
		t.Fatalf("ruler_ratio_rss=%v want null (window rss_end_kb=0)", *r.RulerRatioRSS)
	}
	if r.RulerName == nil || *r.RulerName != "flutter_spin" || r.RulerRSSMB == nil || *r.RulerRSSMB != 300.0 {
		t.Fatalf("measured ruler numbers must still fill: %+v", r)
	}
}

// TestRulerFromEnv: P4 runners export GPUI_RULER_* after the ruler median;
// unset → nil (not run); partial/garbage → nil + stderr warning (never half a
// ruler); complete → parsed RulerInput.
func TestRulerFromEnv(t *testing.T) {
	t.Setenv("GPUI_RULER_NAME", "")
	t.Setenv("GPUI_RULER_CPU_PCT", "")
	t.Setenv("GPUI_RULER_RSS_MB", "")
	if got := wrgate.RulerFromEnv(); got != nil {
		t.Fatalf("unset exports must yield nil, got %+v", got)
	}

	t.Setenv("GPUI_RULER_NAME", "flutter_spin")
	if got := wrgate.RulerFromEnv(); got != nil {
		t.Fatalf("partial export (no numbers) must yield nil, got %+v", got)
	}

	t.Setenv("GPUI_RULER_CPU_PCT", "not-a-number")
	t.Setenv("GPUI_RULER_RSS_MB", "300.0")
	if got := wrgate.RulerFromEnv(); got != nil {
		t.Fatalf("garbage export must yield nil, got %+v", got)
	}

	t.Setenv("GPUI_RULER_CPU_PCT", "25.41")
	got := wrgate.RulerFromEnv()
	if got == nil || got.Name != "flutter_spin" || got.CPUPct != 25.41 || got.RSSMB != 300.0 {
		t.Fatalf("complete export must parse, got %+v", got)
	}
}

// P2.4d: the GPU pool-reuse probe rides every report, and the gate fails when a
// window that must reuse same-size textures reports zero hits.
func TestBuildReport_PoolSectionFromProbe(t *testing.T) {
	owner := new(any)
	render.RegisterGPUPoolStatsCollector(owner, func() []render.PoolDomainStats {
		return []render.PoolDomainStats{
			{Device: "device-1", Name: "device-surface", Hits: 40, Misses: 3, Budget: 128 << 20},
			{Device: "device-1", Name: "filter-graph", Hits: 0, Misses: 9},
		}
	})
	t.Cleanup(func() { render.UnregisterGPUPoolStatsCollector(owner) })

	r := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R20",
		Scenario:      "ui_wr_r20_filter",
		Snap:          scheduler.FrameMetrics{PresentPolicy: scheduler.PresentPolicyFullPaint, VSyncSource: "fallback"},
		PresentCount:  10,
		ElapsedSec:    1.0,
		SurfaceAreaPx: 100 * 100,
	})
	if len(r.Pool) != 2 {
		t.Fatalf("pool domains = %d, want the probe's two", len(r.Pool))
	}
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := wrgate.CheckSchema(b); err != nil {
		t.Fatalf("pool section must be part of the required schema: %v", err)
	}
	for _, needle := range []string{`"pool"`, `"device-surface"`, `"filter-graph"`, `"hits":40`} {
		if !strings.Contains(string(b), needle) {
			t.Fatalf("JSON missing %s: %s", needle, b)
		}
	}
}

func TestEvaluatePoolGates(t *testing.T) {
	r := wrgate.Report{Pool: []render.PoolDomainStats{
		{Name: "device-surface", Hits: 12, Misses: 26},
		{Name: "filter-graph", Hits: 0, Misses: 3},
		{Name: "layer-offscreen", Hits: 0, Misses: 1},
	}}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{}); err != nil {
		t.Fatalf("gates off must pass: %v", err)
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{MinPoolHits: 20}); err == nil {
		t.Fatal("total hits 12 < floor 20 must FAIL")
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{MinPoolHits: 10}); err != nil {
		t.Fatalf("total hits 12 >= floor 10 must pass: %v", err)
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{MinPoolMisses: 31}); err == nil {
		t.Fatal("total misses 30 < floor 31 must FAIL")
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{MinPoolMisses: 30}); err != nil {
		t.Fatalf("total misses 30 >= floor 30 must pass: %v", err)
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{RequirePoolHitDomains: []string{"filter-graph"}}); err == nil {
		t.Fatal("a required domain with 0 hits must FAIL (no fake green)")
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{RequirePoolHitDomains: []string{"dualtex-snap"}}); err == nil {
		t.Fatal("a missing required hit domain must FAIL, not read as silence")
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{RequirePoolHitDomains: []string{"device-surface"}}); err != nil {
		t.Fatalf("device-surface reused 12 times must pass: %v", err)
	}
	// Presence-only domains: the workload may hold its entry for the whole run
	// (0 hits) or draw the composited layer only once; the gate must accept
	// that while still rejecting a domain that never appeared.
	for _, name := range []string{"filter-graph", "layer-offscreen", "device-surface"} {
		if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{RequirePoolDomains: []string{name}}); err != nil {
			t.Fatalf("present domain %q must pass presence gate: %v", name, err)
		}
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{RequirePoolDomains: []string{"dualtex-snap"}}); err == nil {
		t.Fatal("a missing presence-only domain must FAIL")
	}
	if err := wrgate.EvaluatePoolGates(r, wrgate.GateOptions{
		MinPoolMisses:         30,
		RequirePoolDomains:    []string{"device-surface", "filter-graph"},
		RequirePoolHitDomains: []string{"device-surface"},
	}); err != nil {
		t.Fatalf("multiwin-shaped gate (misses floor + hit domain) must pass: %v", err)
	}
}

// C5 (evidence/P2.6/c5-rss-slope-gate-assessment.md): the shared gate set owns
// the RSS steady-slope leak verdict. Default 30000 KB/min, but only a >=60s
// verdict window may judge — a 30-35s window still carries warmup cache growth
// (10 historical PASS records: 9/10 did not reproduce at 35s).
func TestEvaluateGates_RSSSteadySlope(t *testing.T) {
	mk := func(valid bool, elapsed, steady float64) wrgate.Report {
		snap := scheduler.FrameMetrics{
			PresentPolicy:          scheduler.PresentPolicyFullPaint,
			VSyncSource:            "fallback",
			FrameCount:             30,
			P50FrameIntervalMs:     16,
			P95FrameIntervalMs:     17,
			CPUPctAvg:              1,
			RSSStartKB:             1,
			RSSEndKB:               1,
			RSSPeakKB:              1,
			RSSSlopeValid:          valid,
			RSSSlopeSteadyKBPerMin: steady,
		}
		return wrgate.BuildReport(wrgate.BuildInput{
			AbilityID: "R0", Scenario: "t", Snap: snap, PresentCount: 30,
			ElapsedSec: elapsed, Warmup: true,
		})
	}
	opt := wrgate.GateOptions{MinPresents: 1}

	// 1) Default gate is ON: 60s + valid + over line must fail.
	if err := wrgate.EvaluateGates(mk(true, 60.5, 35161), opt); err == nil || !strings.Contains(err.Error(), "rss_slope_steady_kb_per_min") {
		t.Fatalf("want RSS leak FAIL, got %v", err)
	}
	// 2) Same over-line slope on a 35s window: observation only, no verdict.
	if err := wrgate.EvaluateGates(mk(true, 35.0, 90000), opt); err != nil {
		t.Fatalf("35s warmup must not be judged, got %v", err)
	}
	// 3) valid=false: cannot conclude, never a verdict.
	if err := wrgate.EvaluateGates(mk(false, 60.5, 90000), opt); err != nil {
		t.Fatalf("invalid slope must not be judged, got %v", err)
	}
	// 4) Under the line at 60s stays green (m4_virtual 60s measured 28586.8).
	if err := wrgate.EvaluateGates(mk(true, 60.5, 28586.8), opt); err != nil {
		t.Fatalf("under-line 60s run must pass, got %v", err)
	}
	// 5) Explicit thresholds are honored (not hardcoded).
	if err := wrgate.EvaluateGates(mk(true, 60.5, 100), wrgate.GateOptions{
		MinPresents: 1, MaxRSSSlopeSteadyKBPerMin: 50, MinRSSVerdictSec: 30,
	}); err == nil || !strings.Contains(err.Error(), "rss_slope_steady_kb_per_min") {
		t.Fatalf("want explicit-threshold FAIL, got %v", err)
	}
}
