package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/scheduler"
)

// runGate applies the three-evidence close-out used by the matrix: logical
// probes, semantic pixel probes on the real GPU snapshot, and a zero-tolerance
// golden over the static mask. A seed run is deliberately not a PASS.
func runGate(spec *sceneSpec, snapPath string, snap scheduler.FrameMetrics, presents int64, elapsed float64) bool {
	out := map[string]any{
		"event":       "RULER_GATE",
		"scene":       spec.name,
		"snapshot":    snapPath,
		"elapsed_sec": round2(elapsed),
		"presents":    presents,
	}
	// Static scenes (the text arm, and gate-mode frozen scenes with no
	// ticker) legitimately show paint_count=0: the warm-up/first-present
	// frame is not a scheduler paint. The snapshot pixels are the paint
	// evidence; logic requires a real first present and a clean fallback path.
	logicOK := presents > 0 && snap.TimeToFirstPresentMs > 0 && snap.CPUFallbackOps == 0 && snap.Warmup
	out["logic_ok"] = logicOK
	out["logic"] = fmt.Sprintf("presents=%d frames=%d cpu_fallback=%d warmup=%v first_present_ms=%.2f",
		presents, snap.FrameCount, snap.CPUFallbackOps, snap.Warmup, snap.TimeToFirstPresentMs)

	var pixelErr error
	if _, err := os.Stat(snapPath); err != nil {
		pixelErr = fmt.Errorf("snapshot missing: %w", err)
	} else {
		pixelErr = wrkit.CheckPixelProbes(snapPath, float64(spec.winW), spec.probes)
		if pixelErr == nil && spec.extraCheck != nil {
			pixelErr = spec.extraCheck(snapPath)
		}
	}
	out["pixel_ok"] = pixelErr == nil
	if pixelErr != nil {
		out["pixel_error"] = pixelErr.Error()
	}

	basePath := filepath.Join("examples", "ui_ruler_parity", "testdata", "golden_"+spec.name+".png")
	var diffPct float64
	var totalPx int64
	var seeded bool
	var goldenErr error
	if len(spec.goldenRects) == 0 {
		spec.goldenRects = []wrkit.GoldenRect{{X: 0, Y: 0, W: float64(spec.winW), H: float64(spec.winH)}}
	}
	diffPct, totalPx, seeded, goldenErr = wrkit.GoldenGateW(snapPath, basePath, float64(spec.winW), spec.goldenRects)
	goldenOK := goldenErr == nil && !seeded && diffPct == 0
	out["golden_ok"] = goldenOK
	out["golden_diff_pct"] = diffPct
	out["golden_total_px"] = totalPx
	out["golden_seeded"] = seeded
	if goldenErr != nil {
		out["golden_error"] = goldenErr.Error()
	}
	if seeded {
		out["golden_note"] = "baseline seeded; rerun without GPUI_ACCEPT_GOLDEN and require diff=0"
	}

	pass := logicOK && pixelErr == nil && goldenOK
	out["pass"] = pass
	raw, _ := json.Marshal(out)
	fmt.Println(string(raw))
	if !pass {
		fmt.Fprintln(os.Stderr, "FAIL: parity gate is not green (see JSON above)")
	}
	return pass
}
