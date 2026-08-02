package wrgate_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/ui/scheduler"
)

func TestBuildReport_AndSchema(t *testing.T) {
	snap := scheduler.FrameMetrics{
		PresentPolicy:      scheduler.PresentPolicyFullPaint,
		FrameCount:         10,
		PresentCount:       10,
		AvgFrameIntervalMs: 16.6,
		P50FrameIntervalMs: 16.5,
		P95FrameIntervalMs: 17.0,
		P99FrameIntervalMs: 18.0,
		VSyncSource:        "fallback",
		LayoutCount:        1,
		PaintCount:         10,
		PresentMode:        "full",
		CPUPctAvg:          5,
		CPUUIPct:           50,
		CPURasterPct:       50,
		RSSStartKB:         1000,
		RSSEndKB:           1100,
		RSSPeakKB:          1200,
		RSSSlopeKBPerMin:   10,
		GPUOps:             100,
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
	b, err := wrgate.Marshal(r)
	if err != nil {
		t.Fatal(err)
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
		PresentPolicy:       scheduler.PresentPolicyFullPaint,
		VSyncSource:         "fallback",
		DirtyLayerIDs:       []uint64{11, 22},
		DamageMultiFrames:   4,
		RSSStartKB:          1,
		RSSEndKB:            1,
		RSSPeakKB:           1,
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
